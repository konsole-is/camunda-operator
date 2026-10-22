/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package builtin

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	dbwrapper "github.com/konsole-is/camunda-operator/pkg/wrappers/database"
	serverwrapper "github.com/konsole-is/camunda-operator/pkg/wrappers/databaseserver"
)

// kindDatabaseServerPreset is the kind of the preset that a database-server
// request reads.
const kindDatabaseServerPreset = "DatabaseServerPreset"

// postgresDatabaseName is the logical database on the server of a
// database-server request.
const postgresDatabaseName = "camunda"

// postgresql serves the built-in registration database-server. It applies
// the DatabaseServer <claim>-pg and the Database <claim> on it, and publishes
// the SecondaryStorageConfig <claim>-pg that the Database writes. An override
// never lowers the live storage size.
type postgresql struct{}

// postgresParameters are the parameters of a database-server request.
type postgresParameters struct {
	Preset     string
	ReleaseRef string
	Overrides  postgresOverrides
}

type postgresOverrides struct {
	Instances   *int32
	Resources   *corev1.ResourceRequirements
	StorageSize *resource.Quantity
	Scheduling  *v1.SchedulingSpec
	PodLabels   map[string]string
}

func (postgresql) build(request *pluginsv1alpha1.PluginRequest, live liveObjects) (served, error) {
	p, err := decodePostgresParameters(request.Spec.Parameters)
	if err != nil {
		return served{}, err
	}

	server, warnings, err := postgresServer(request, p, live)
	if err != nil {
		return served{}, err
	}
	database := postgresDatabase(request)

	serverCell := concepts.NewData[v1.DatabaseServer]("server")
	sb := serverwrapper.NewBuilder(server)
	serverwrapper.ExtractInto(sb, serverCell, func(s v1.DatabaseServer) (v1.DatabaseServer, error) {
		return s, nil
	})
	serverRes, err := sb.Build()
	if err != nil {
		return served{}, fmt.Errorf("building the DatabaseServer: %w", err)
	}

	databaseCell := concepts.NewData[metav1.Condition]("database")
	db := dbwrapper.NewBuilder(database)
	dbwrapper.ExtractInto(db, databaseCell, func(d v1.Database) (metav1.Condition, error) {
		return readyOf(d.Status.Conditions), nil
	})
	databaseRes, err := db.Build()
	if err != nil {
		return served{}, fmt.Errorf("building the Database: %w", err)
	}

	backing, err := component.NewComponentBuilder().
		WithName("backing").
		WithConditionType(component.ConditionType(v1.ConditionReady)).
		WithResource(serverRes, component.DeleteOnlyIfOwned()).
		WithResource(databaseRes, component.DeleteOnlyIfOwned()).
		Build()
	if err != nil {
		return served{}, fmt.Errorf("building the backing component: %w", err)
	}

	contract := postgresContract(request)
	outputs, _, err := outputsComponent([]contractRef{contract}, request.Namespace)
	if err != nil {
		return served{}, err
	}

	return served{
		outputs: outputs,
		backing: backing,
		report: func() report {
			summary := []pluginsv1alpha1.SummaryItem{
				serverItem(server.Name, serverCell),
				summaryItem(kindDatabase, database.Name, databaseCell),
			}
			return report{
				outputs: map[string]string{contract.output: contract.name},
				summary: append(summary, warnings...),
				resources: []pluginsv1alpha1.ResourceRef{
					{Kind: kindDatabaseServer, Name: server.Name, OnOrphan: pluginsv1alpha1.OnOrphanKept},
					{Kind: kindDatabase, Name: database.Name, OnOrphan: pluginsv1alpha1.OnOrphanKept},
				},
				podSelector: &metav1.LabelSelector{MatchLabels: map[string]string{
					labels.DatabaseServerKey: labels.DatabaseServer(server.Name).Name,
				}},
			}
		},
	}, nil
}

// decodePostgresParameters decodes parameters with no unknown field. A value
// that does not decode returns a *conditions.PreCheckFailure with reason
// InvalidParameters, whose message names the field.
func decodePostgresParameters(parameters *apiextensionsv1.JSON) (postgresParameters, error) {
	var top map[string]json.RawMessage
	if err := decodeStrict(parameters, &top); err != nil {
		return postgresParameters{}, invalidParameter("spec.parameters", err)
	}

	var p postgresParameters
	var overrides map[string]json.RawMessage
	if err := decodeFields(top, "spec.parameters.", map[string]any{
		"preset":     &p.Preset,
		"releaseRef": &p.ReleaseRef,
		"overrides":  &overrides,
	}); err != nil {
		return postgresParameters{}, err
	}

	if err := decodeFields(overrides, "spec.parameters.overrides.", map[string]any{
		"instances":   &p.Overrides.Instances,
		"resources":   &p.Overrides.Resources,
		"storageSize": &p.Overrides.StorageSize,
		"scheduling":  &p.Overrides.Scheduling,
		"podLabels":   &p.Overrides.PodLabels,
	}); err != nil {
		return postgresParameters{}, err
	}

	return p, nil
}

// postgresServer returns the DatabaseServer to apply for request, and the
// summary items that warn about an input or an override that it does not
// apply.
func postgresServer(
	request *pluginsv1alpha1.PluginRequest,
	p postgresParameters,
	live liveObjects,
) (*v1.DatabaseServer, []pluginsv1alpha1.SummaryItem, error) {
	name := postgresServerName(request)
	server := &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: request.Namespace,
			Labels:    postgresLabels(request),
		},
		Spec: v1.DatabaseServerSpec{
			PresetRef:            p.Preset,
			ReleaseRef:           p.ReleaseRef,
			DatabaseServerConfig: name,
			Instances:            p.Overrides.Instances,
			Resources:            p.Overrides.Resources,
			StorageSize:          p.Overrides.StorageSize,
			Scheduling:           p.Overrides.Scheduling,
			PodLabels:            p.Overrides.PodLabels,
		},
	}

	if class := request.Spec.Inputs[pluginstartup.OutputStorageClassName]; class != "" {
		server.Spec.StorageClassName = &class
	}

	annotations, err := decodeAnnotations(request.Spec.Inputs)
	if err != nil {
		return nil, nil, err
	}
	if annotations != nil {
		server.Spec.ServiceAccount = &v1.DatabaseServerServiceAccountSpec{Annotations: annotations}
	}

	var warnings []pluginsv1alpha1.SummaryItem
	if current, ok := liveAs[*v1.DatabaseServer](live, kindDatabaseServer, name); ok {
		var shrink *pluginsv1alpha1.SummaryItem
		server.Spec.StorageSize, shrink = keepLiveStorageSize(
			server.Spec.StorageSize, current.Spec.StorageSize, dataVolumes(current.Status.Volumes),
		)
		if shrink != nil {
			warnings = append(warnings, *shrink)
		}
	}

	var skipped *pluginsv1alpha1.SummaryItem
	server.Spec.Archive, skipped = postgresArchive(
		request.Spec.Inputs[pluginstartup.OutputObjectStorageConfig],
		p.Preset,
		live,
	)
	if skipped != nil {
		warnings = append(warnings, *skipped)
	}

	return server, warnings, nil
}

// dataVolumes returns volumes without the write-ahead log volumes, which
// spec.walStorageSize sizes.
func dataVolumes(volumes []v1.VolumeStatus) []v1.VolumeStatus {
	return slices.DeleteFunc(slices.Clone(volumes), func(volume v1.VolumeStatus) bool {
		return strings.HasSuffix(volume.Name, cnpgv1.WalArchiveVolumeSuffix)
	})
}

func postgresServerName(request *pluginsv1alpha1.PluginRequest) string {
	return request.Spec.ClaimRef.Name + pluginstartup.BackingSuffixDatabaseServer
}

func postgresLabels(request *pluginsv1alpha1.PluginRequest) map[string]string {
	return map[string]string{
		pluginsv1alpha1.LabelRegistration: pluginstartup.RegistrationDatabaseServer,
		pluginsv1alpha1.LabelRequest:      request.Name,
	}
}

// postgresArchive returns the archive of the server for the bucket input,
// with the retention and the schedule of the archive of the preset. The
// archive replaces the archive of the preset whole, and its retention is
// required. It returns nil without the input. It returns nil and the
// summary item that says why when the preset is absent or has no archive.
func postgresArchive(
	bucket, preset string,
	live liveObjects,
) (*v1.DatabaseServerArchiveSpec, *pluginsv1alpha1.SummaryItem) {
	if bucket == "" {
		return nil, nil
	}

	skipped := &pluginsv1alpha1.SummaryItem{
		Label:    "Backup storage",
		Severity: pluginsv1alpha1.SeverityWarning,
	}
	current, ok := liveAs[*v1.DatabaseServerPreset](live, kindDatabaseServerPreset, preset)
	switch {
	case !ok:
		skipped.Value = bucket + " is not used: the " + kindDatabaseServerPreset + " " + preset + " does not exist"
		return nil, skipped
	case current.Spec.Server.Archive == nil:
		skipped.Value = bucket + " is not used: the " + kindDatabaseServerPreset + " " + preset +
			" sets no archive retention"
		return nil, skipped
	}

	return &v1.DatabaseServerArchiveSpec{
		ObjectStorageRef:    bucket,
		RetentionPeriodDays: current.Spec.Server.Archive.RetentionPeriodDays,
		BaseBackupSchedule:  current.Spec.Server.Archive.BaseBackupSchedule,
	}, nil
}

// postgresDatabase returns the Database of request. It publishes the
// DatabaseConfig <claim> and the SecondaryStorageConfig <claim>-pg.
func postgresDatabase(request *pluginsv1alpha1.PluginRequest) *v1.Database {
	return &v1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      request.Spec.ClaimRef.Name,
			Namespace: request.Namespace,
			Labels:    postgresLabels(request),
		},
		Spec: v1.DatabaseSpec{
			ServerRef:              postgresServerName(request),
			DatabaseName:           postgresDatabaseName,
			DatabaseConfig:         request.Spec.ClaimRef.Name,
			SecondaryStorageConfig: postgresServerName(request),
		},
	}
}

// postgresContract returns the SecondaryStorageConfig that the Database of
// request publishes.
func postgresContract(request *pluginsv1alpha1.PluginRequest) contractRef {
	return contractRef{
		output: pluginstartup.OutputSecondaryStorageConfig,
		kind:   kindSecondaryStorageConfig,
		name:   postgresServerName(request),
	}
}

// cleanup deletes the Database and then the DatabaseServer with the Delete
// policy, each only when the request owns it. With the Orphan policy it
// keeps both without the owner reference.
func (postgresql) cleanup(
	request *pluginsv1alpha1.PluginRequest,
	policy pluginsv1alpha1.DeletionPolicy,
	_ liveObjects,
) (cleanup, error) {
	server := &v1.DatabaseServer{ObjectMeta: metav1.ObjectMeta{
		Name: postgresServerName(request), Namespace: request.Namespace,
	}}
	database := &v1.Database{ObjectMeta: metav1.ObjectMeta{
		Name: request.Spec.ClaimRef.Name, Namespace: request.Namespace,
	}}
	serverRes, err := serverwrapper.NewBuilder(server.DeepCopy()).Build()
	if err != nil {
		return cleanup{}, fmt.Errorf("building the DatabaseServer: %w", err)
	}
	databaseRes, err := dbwrapper.NewBuilder(database.DeepCopy()).Build()
	if err != nil {
		return cleanup{}, fmt.Errorf("building the Database: %w", err)
	}

	if policy == pluginsv1alpha1.DeletionPolicyOrphan {
		orphan, err := component.NewComponentBuilder().
			WithName("orphan").
			WithConditionType(component.ConditionType(v1.ConditionReady)).
			WithResource(databaseRes, component.OrphanWhen(true)).
			WithResource(serverRes, component.OrphanWhen(true)).
			Build()
		if err != nil {
			return cleanup{}, fmt.Errorf("building the orphan component: %w", err)
		}
		return cleanup{stages: []cleanupStage{{
			components: []*component.Component{orphan},
			remaining:  []client.Object{database, server},
		}}}, nil
	}

	deleteDatabase, err := component.NewComponentBuilder().
		WithName("database").
		WithConditionType(component.ConditionType(v1.ConditionReady)).
		WithResource(databaseRes, component.Delete(), component.DeleteOnlyIfOwned()).
		Build()
	if err != nil {
		return cleanup{}, fmt.Errorf("building the database component: %w", err)
	}
	deleteServer, err := component.NewComponentBuilder().
		WithName("server").
		WithConditionType(component.ConditionType(v1.ConditionReady)).
		WithResource(serverRes, component.Delete(), component.DeleteOnlyIfOwned()).
		Build()
	if err != nil {
		return cleanup{}, fmt.Errorf("building the server component: %w", err)
	}

	return cleanup{stages: []cleanupStage{
		{components: []*component.Component{deleteDatabase}, remaining: []client.Object{database}},
		{components: []*component.Component{deleteServer}, remaining: []client.Object{server}},
	}}, nil
}

// contracts returns the SecondaryStorageConfig that the Database publishes.
func (postgresql) contracts(request *pluginsv1alpha1.PluginRequest) []contractRef {
	if _, err := decodePostgresParameters(request.Spec.Parameters); err != nil {
		return nil
	}

	return []contractRef{postgresContract(request)}
}

// backing returns the DatabaseServer, the Database, and the
// DatabaseServerPreset of the parameters, which gives the retention of the
// archive.
func (postgresql) backing(request *pluginsv1alpha1.PluginRequest) []backingRef {
	refs := []backingRef{
		{kind: kindDatabaseServer, object: &v1.DatabaseServer{ObjectMeta: metav1.ObjectMeta{
			Name: postgresServerName(request), Namespace: request.Namespace,
		}}},
		{kind: kindDatabase, object: &v1.Database{ObjectMeta: metav1.ObjectMeta{
			Name: request.Spec.ClaimRef.Name, Namespace: request.Namespace,
		}}},
	}

	p, err := decodePostgresParameters(request.Spec.Parameters)
	if err != nil || p.Preset == "" {
		return refs
	}

	return append(refs, backingRef{
		kind:   kindDatabaseServerPreset,
		object: &v1.DatabaseServerPreset{ObjectMeta: metav1.ObjectMeta{Name: p.Preset}},
	})
}
