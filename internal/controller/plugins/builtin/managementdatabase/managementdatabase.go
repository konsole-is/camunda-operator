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

// Package managementdatabase serves the built-in registration
// management-database-server.
package managementdatabase

import (
	"encoding/json"
	"slices"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/adapter"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	mdb "github.com/konsole-is/camunda-operator/pkg/components/managementdatabases"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// managementOutputs are the output keys of the components.
var managementOutputs = map[mdb.Component]string{
	mdb.Keycloak:   pluginstartup.OutputKeycloakDatabaseConfig,
	mdb.Identity:   pluginstartup.OutputIdentityDatabaseConfig,
	mdb.WebModeler: pluginstartup.OutputWebModelerDatabaseConfig,
}

// Handler serves management-database-server. It applies the
// DatabaseServer <plane>-db and one Database for each component that the
// parameters list.
type Handler struct{}

// Build implements adapter.Handler.
func (Handler) Build(request *pluginsv1alpha1.PluginRequest, live adapter.LiveObjects) (adapter.Served, error) {
	v, err := managementValuesWithParameters(request)
	if err != nil {
		return adapter.Served{}, err
	}

	refs := managementContracts(v)
	outputs, _, err := adapter.OutputsComponent(refs, request.Namespace)
	if err != nil {
		return adapter.Served{}, err
	}

	backing, err := mdb.NewBacking(v, v1.ConditionReady)
	if err != nil {
		return adapter.Served{}, err
	}

	kept := keptComponents(request, v, live)

	return adapter.Served{
		Outputs: outputs,
		Backing: backing.Component,
		Report: func() adapter.Report {
			rep := adapter.Report{
				Outputs: map[string]string{},
				Summary: []pluginsv1alpha1.SummaryItem{adapter.ServerItem(v.Server, backing.Server)},
				Resources: []pluginsv1alpha1.ResourceRef{{
					Kind: adapter.KindDatabaseServer, Name: v.Server, OnOrphan: pluginsv1alpha1.OnOrphanKept,
				}},
				PodSelector: &metav1.LabelSelector{
					MatchLabels: map[string]string{labels.DatabaseServerKey: labels.DatabaseServer(v.Server).Name},
				},
			}
			for i, ref := range refs {
				rep.Outputs[ref.Output] = ref.Name
				rep.Summary = append(
					rep.Summary,
					adapter.SummaryItem(adapter.KindDatabase, ref.Name, backing.Databases[v.Databases[i]]),
				)
				rep.Resources = append(rep.Resources, pluginsv1alpha1.ResourceRef{
					Kind: adapter.KindDatabase, Name: ref.Name, OnOrphan: pluginsv1alpha1.OnOrphanKept,
				})
			}
			rep.Summary = append(rep.Summary, kept...)
			return rep
		},
	}, nil
}

// Cleanup implements adapter.Handler.
func (Handler) Cleanup(
	request *pluginsv1alpha1.PluginRequest,
	policy pluginsv1alpha1.DeletionPolicy,
	_ adapter.LiveObjects,
) (adapter.Cleanup, error) {
	v := managementValues(request)
	databases := make([]client.Object, 0, len(mdb.Components))
	for _, c := range mdb.Components {
		databases = append(databases, mdb.Database(v, c))
	}

	if policy == pluginsv1alpha1.DeletionPolicyOrphan {
		orphan, err := mdb.Orphan(v)
		if err != nil {
			return adapter.Cleanup{}, err
		}
		return adapter.Cleanup{Stages: []adapter.CleanupStage{{
			Components: []*component.Component{orphan},
			Remaining:  append(databases, mdb.Server(v)),
		}}}, nil
	}

	deleteDatabases, err := mdb.DeleteDatabases(v)
	if err != nil {
		return adapter.Cleanup{}, err
	}
	deleteServer, err := mdb.DeleteServer(v)
	if err != nil {
		return adapter.Cleanup{}, err
	}

	return adapter.Cleanup{Stages: []adapter.CleanupStage{
		{Components: []*component.Component{deleteDatabases}, Remaining: databases},
		{Components: []*component.Component{deleteServer}, Remaining: []client.Object{mdb.Server(v)}},
	}}, nil
}

// Contracts implements adapter.Handler.
func (Handler) Contracts(request *pluginsv1alpha1.PluginRequest) []adapter.ContractRef {
	v, err := managementValuesWithParameters(request)
	if err != nil {
		return nil
	}

	return managementContracts(v)
}

// Backing implements adapter.Handler.
func (Handler) Backing(request *pluginsv1alpha1.PluginRequest) []adapter.BackingRef {
	v := managementValues(request)
	refs := make([]adapter.BackingRef, 0, 1+len(mdb.Components))
	refs = append(refs, adapter.BackingRef{Kind: adapter.KindDatabaseServer, Object: mdb.Server(v)})
	for _, c := range mdb.Components {
		refs = append(refs, adapter.BackingRef{Kind: adapter.KindDatabase, Object: mdb.Database(v, c)})
	}

	return refs
}

// managementValuesWithParameters returns the values of request with its
// parameters. Parameters that do not decode return a
// *conditions.PreCheckFailure with reason InvalidParameters.
func managementValuesWithParameters(request *pluginsv1alpha1.PluginRequest) (mdb.Values, error) {
	p, err := decodeManagementParameters(request.Spec.Parameters)
	if err != nil {
		return mdb.Values{}, err
	}

	v := managementValues(request)
	v.Preset, v.Version, v.Databases = p.Preset, p.Version, p.Databases
	return v, nil
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
		Labels:           adapter.BackingLabels(request),
	}
}

// managementParameters are the parameters of a management-database-server
// request.
type managementParameters struct {
	Preset    string
	Version   string
	Databases []mdb.Component
}

// decodeManagementParameters decodes parameters with no unknown field. A
// value that does not decode returns a *conditions.PreCheckFailure with
// reason InvalidParameters, whose message names the field. An absent version
// is pluginstartup.ManagementDatabaseServerVersion. The databases have no
// duplicates and follow the order of mdb.Components.
func decodeManagementParameters(parameters *apiextensionsv1.JSON) (managementParameters, error) {
	var top map[string]json.RawMessage
	if err := adapter.DecodeStrict(parameters, &top); err != nil {
		return managementParameters{}, adapter.InvalidParameter("spec.parameters", err)
	}

	var p managementParameters
	var listed []mdb.Component
	if err := adapter.DecodeFields(top, "spec.parameters.", map[string]any{
		"preset":    &p.Preset,
		"version":   &p.Version,
		"databases": &listed,
	}); err != nil {
		return managementParameters{}, err
	}

	if p.Version == "" {
		p.Version = pluginstartup.ManagementDatabaseServerVersion
	}

	for _, c := range mdb.Components {
		if slices.Contains(listed, c) {
			p.Databases = append(p.Databases, c)
		}
	}

	return p, nil
}

// managementContracts returns the DatabaseConfig of each listed component of
// v, in the order of v.Databases.
func managementContracts(v mdb.Values) []adapter.ContractRef {
	refs := make([]adapter.ContractRef, 0, len(v.Databases))
	for _, c := range v.Databases {
		refs = append(refs, adapter.ContractRef{
			Output: managementOutputs[c],
			Kind:   adapter.KindDatabaseConfig,
			Name:   mdb.DatabaseName(v.Plane, c),
		})
	}

	return refs
}

func keptComponents(
	request *pluginsv1alpha1.PluginRequest,
	v mdb.Values,
	live adapter.LiveObjects,
) []pluginsv1alpha1.SummaryItem {
	var items []pluginsv1alpha1.SummaryItem
	for _, c := range mdb.Components {
		name := mdb.DatabaseName(v.Plane, c)
		current, ok := adapter.LiveAs[*v1.Database](live, adapter.KindDatabase, name)
		if slices.Contains(v.Databases, c) || !ok || !adapter.OwnedBy(current, request.UID) {
			continue
		}

		items = append(items, pluginsv1alpha1.SummaryItem{
			Label:    adapter.KindDatabase,
			Value:    name + " removed, the logical database " + mdb.LogicalName(c) + " stays on " + v.Server,
			Severity: pluginsv1alpha1.SeverityWarning,
		})
	}

	return items
}
