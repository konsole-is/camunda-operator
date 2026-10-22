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
	"slices"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	mdb "github.com/konsole-is/camunda-operator/pkg/components/managementdatabases"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// The kinds of the backing resources of management-database-server.
const (
	kindDatabaseServer = "DatabaseServer"
	kindDatabase       = "Database"
)

// managementOutputs are the output keys of the components.
var managementOutputs = map[mdb.Component]string{
	mdb.Keycloak:   pluginstartup.OutputKeycloakDatabaseConfig,
	mdb.Identity:   pluginstartup.OutputIdentityDatabaseConfig,
	mdb.WebModeler: pluginstartup.OutputWebModelerDatabaseConfig,
}

// managementDatabases serves management-database-server. It applies the
// DatabaseServer <plane>-db and one Database for each component that the
// parameters list.
type managementDatabases struct{}

func (managementDatabases) build(request *pluginsv1alpha1.PluginRequest, live liveObjects) (served, error) {
	v, err := managementValuesWithParameters(request)
	if err != nil {
		return served{}, err
	}

	refs := managementContracts(v)
	outputs, _, err := outputsComponent(refs, request.Namespace)
	if err != nil {
		return served{}, err
	}

	backing, err := mdb.NewBacking(v, v1.ConditionReady)
	if err != nil {
		return served{}, err
	}

	kept := keptComponents(request, v, live)

	return served{
		outputs: outputs,
		backing: backing.Component,
		report: func() report {
			rep := report{
				outputs: map[string]string{},
				summary: []pluginsv1alpha1.SummaryItem{serverItem(v.Server, backing.Server)},
				resources: []pluginsv1alpha1.ResourceRef{{
					Kind: kindDatabaseServer, Name: v.Server, OnOrphan: pluginsv1alpha1.OnOrphanKept,
				}},
				podSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{labels.DatabaseServerKey: labels.DatabaseServer(v.Server).Name},
				},
			}
			for i, ref := range refs {
				rep.outputs[ref.output] = ref.name
				rep.summary = append(
					rep.summary,
					summaryItem(kindDatabase, ref.name, backing.Databases[v.Databases[i]]),
				)
				rep.resources = append(rep.resources, pluginsv1alpha1.ResourceRef{
					Kind: kindDatabase, Name: ref.name, OnOrphan: pluginsv1alpha1.OnOrphanKept,
				})
			}
			rep.summary = append(rep.summary, kept...)
			return rep
		},
	}, nil
}

func (managementDatabases) cleanup(
	request *pluginsv1alpha1.PluginRequest,
	policy pluginsv1alpha1.DeletionPolicy,
	_ liveObjects,
) (cleanup, error) {
	v := managementValues(request)
	databases := make([]client.Object, 0, len(mdb.Components))
	for _, c := range mdb.Components {
		databases = append(databases, mdb.Database(v, c))
	}

	if policy == pluginsv1alpha1.DeletionPolicyOrphan {
		orphan, err := mdb.Orphan(v)
		if err != nil {
			return cleanup{}, err
		}
		return cleanup{stages: []cleanupStage{{
			components: []*component.Component{orphan},
			remaining:  append(databases, mdb.Server(v)),
		}}}, nil
	}

	deleteDatabases, err := mdb.DeleteDatabases(v)
	if err != nil {
		return cleanup{}, err
	}
	deleteServer, err := mdb.DeleteServer(v)
	if err != nil {
		return cleanup{}, err
	}

	return cleanup{stages: []cleanupStage{
		{components: []*component.Component{deleteDatabases}, remaining: databases},
		{components: []*component.Component{deleteServer}, remaining: []client.Object{mdb.Server(v)}},
	}}, nil
}

func (managementDatabases) contracts(request *pluginsv1alpha1.PluginRequest) []contractRef {
	v, err := managementValuesWithParameters(request)
	if err != nil {
		return nil
	}

	return managementContracts(v)
}

func (managementDatabases) backing(request *pluginsv1alpha1.PluginRequest) []backingRef {
	v := managementValues(request)
	refs := make([]backingRef, 0, 1+len(mdb.Components))
	refs = append(refs, backingRef{kind: kindDatabaseServer, object: mdb.Server(v)})
	for _, c := range mdb.Components {
		refs = append(refs, backingRef{kind: kindDatabase, object: mdb.Database(v, c)})
	}

	return refs
}

// managementValuesWithParameters returns the values of request with its
// parameters. Parameters that do not decode return a
// *conditions.PreCheckFailure with reason InvalidParameters.
func managementValuesWithParameters(request *pluginsv1alpha1.PluginRequest) (mdb.Values, error) {
	v := managementValues(request)
	var err error
	v.Preset, v.Version, v.Databases, err = managementParameters(request.Spec.Parameters)
	return v, err
}

// managementValues returns the values of request that do not depend on its
// parameters.
func managementValues(request *pluginsv1alpha1.PluginRequest) mdb.Values {
	plane := request.Spec.ClaimRef.Name
	return mdb.Values{
		Namespace:        request.Namespace,
		Plane:            plane,
		Server:           plane + pluginstartup.BackingSuffixManagementDatabaseServer,
		StorageClassName: request.Spec.Inputs[pluginstartup.OutputStorageClassName],
		Labels: map[string]string{
			pluginsv1alpha1.LabelRegistration: request.Spec.Registration,
			pluginsv1alpha1.LabelRequest:      request.Name,
			labels.ManagedByKey:               labels.ManagedBy,
		},
	}
}

// managementParameters returns an error for a field other than preset,
// version, and databases. An absent version is
// pluginstartup.ManagementDatabaseServerVersion. The databases have no
// duplicates and follow the order of mdb.Components.
func managementParameters(parameters *apiextensionsv1.JSON) (string, string, []mdb.Component, error) {
	var top map[string]json.RawMessage
	if err := decodeStrict(parameters, &top); err != nil {
		return "", "", nil, invalidParameter("spec.parameters", err)
	}

	var preset, version string
	var listed []mdb.Component
	if err := decodeFields(top, "spec.parameters.", map[string]any{
		"preset":    &preset,
		"version":   &version,
		"databases": &listed,
	}); err != nil {
		return "", "", nil, err
	}

	if version == "" {
		version = pluginstartup.ManagementDatabaseServerVersion
	}

	var databases []mdb.Component
	for _, c := range mdb.Components {
		if slices.Contains(listed, c) {
			databases = append(databases, c)
		}
	}

	return preset, version, databases, nil
}

// managementContracts returns the DatabaseConfig of each listed component of
// v, in the order of v.Databases.
func managementContracts(v mdb.Values) []contractRef {
	refs := make([]contractRef, 0, len(v.Databases))
	for _, c := range v.Databases {
		refs = append(refs, contractRef{
			output: managementOutputs[c],
			kind:   kindDatabaseConfig,
			name:   mdb.DatabaseName(v.Plane, c),
		})
	}

	return refs
}

func keptComponents(
	request *pluginsv1alpha1.PluginRequest,
	v mdb.Values,
	live liveObjects,
) []pluginsv1alpha1.SummaryItem {
	var items []pluginsv1alpha1.SummaryItem
	for _, c := range mdb.Components {
		name := mdb.DatabaseName(v.Plane, c)
		current, ok := liveAs[*v1.Database](live, kindDatabase, name)
		if slices.Contains(v.Databases, c) || !ok || !ownedBy(current, request.UID) {
			continue
		}

		items = append(items, pluginsv1alpha1.SummaryItem{
			Label:    kindDatabase,
			Value:    name + " removed, the logical database " + mdb.LogicalName(c) + " stays on " + v.Server,
			Severity: pluginsv1alpha1.SeverityWarning,
		})
	}

	return items
}

// serverItem says whether the DatabaseServer server is there and Ready, with
// its PostgreSQL version once the server reports it.
func serverItem(server string, cell *concepts.Data[v1.DatabaseServer]) pluginsv1alpha1.SummaryItem {
	ready := concepts.NewData[metav1.Condition](server)
	live, ok := cell.Get()
	if ok {
		cond := readyOf(live.Status.Conditions)
		if live.Status.ObservedGeneration != live.Generation ||
			cond.Type != "" && cond.ObservedGeneration != live.Generation {
			return pluginsv1alpha1.SummaryItem{
				Label:    kindDatabaseServer,
				Value:    server + ", updating",
				Severity: pluginsv1alpha1.SeverityWarning,
			}
		}
		ready.Set(cond)
	}

	item := summaryItem(kindDatabaseServer, server, ready)
	if item.Severity == pluginsv1alpha1.SeverityInfo && live.Status.Version != "" {
		item.Value = server + ", PostgreSQL " + live.Status.Version + ", Ready"
	}

	return item
}
