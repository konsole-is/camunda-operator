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

// Package existing serves the registrations that publish contracts that are
// already in the namespace of the request.
package existing

import (
	"encoding/json"
	"fmt"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/adapter"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// Handler serves a registration that publishes contracts that are already
// in the namespace of the request. It writes nothing, so it has no backing
// component and an empty cleanup.
type Handler struct {
	parse func(parameters *apiextensionsv1.JSON) ([]adapter.ContractRef, error)
}

// OneContract returns the handler of a registration whose parameters
// {"name": string} name one contract of kind that the request publishes as
// output.
func OneContract(output, kind string) Handler {
	return Handler{parse: oneContract(output, kind)}
}

// DatabaseConfigs returns the handler of existing-database-configs, which
// publishes a DatabaseConfig for each of keycloak, identity, and webModeler
// that the parameters name.
func DatabaseConfigs() Handler {
	return Handler{parse: databaseConfigs}
}

// Build implements adapter.Handler.
func (e Handler) Build(request *pluginsv1alpha1.PluginRequest, _ adapter.LiveObjects) (adapter.Served, error) {
	refs, err := e.parse(request.Spec.Parameters)
	if err != nil {
		return adapter.Served{}, err
	}

	outputs, cells, err := adapter.OutputsComponent(refs, request.Namespace)
	if err != nil {
		return adapter.Served{}, err
	}

	return adapter.Served{
		Outputs: outputs,
		Report: func() adapter.Report {
			rep := adapter.Report{Outputs: map[string]string{}}
			for i, ref := range refs {
				rep.Outputs[ref.Output] = ref.Name
				rep.Summary = append(rep.Summary, adapter.SummaryItem(ref.Kind, ref.Name, cells[i]))
			}
			return rep
		},
	}, nil
}

// Cleanup implements adapter.Handler.
func (Handler) Cleanup(
	*pluginsv1alpha1.PluginRequest,
	pluginsv1alpha1.DeletionPolicy,
	adapter.LiveObjects,
) (adapter.Cleanup, error) {
	return adapter.Cleanup{}, nil
}

// Contracts implements adapter.Handler.
func (e Handler) Contracts(request *pluginsv1alpha1.PluginRequest) []adapter.ContractRef {
	refs, err := e.parse(request.Spec.Parameters)
	if err != nil {
		return nil
	}

	return refs
}

// Backing implements adapter.Handler.
func (Handler) Backing(*pluginsv1alpha1.PluginRequest) []adapter.BackingRef {
	return nil
}

// oneContract returns a parse function for the parameters {"name": string},
// which name one contract of kind that the request publishes as output.
func oneContract(output, kind string) func(*apiextensionsv1.JSON) ([]adapter.ContractRef, error) {
	return func(parameters *apiextensionsv1.JSON) ([]adapter.ContractRef, error) {
		p, err := decodeParameters(parameters, "name")
		if err != nil {
			return nil, err
		}

		return []adapter.ContractRef{{Output: output, Kind: kind, Name: p["name"]}}, nil
	}
}

// databaseConfigs returns a contract for each DatabaseConfig name that the
// parameters set, in the order keycloak, identity, webModeler. Two fields
// with one name return a *conditions.PreCheckFailure with reason
// InvalidParameters.
func databaseConfigs(parameters *apiextensionsv1.JSON) ([]adapter.ContractRef, error) {
	p, err := decodeParameters(parameters, "keycloak", "identity", "webModeler")
	if err != nil {
		return nil, err
	}

	fields := []struct{ field, output string }{
		{"keycloak", pluginstartup.OutputKeycloakDatabaseConfig},
		{"identity", pluginstartup.OutputIdentityDatabaseConfig},
		{"webModeler", pluginstartup.OutputWebModelerDatabaseConfig},
	}

	var refs []adapter.ContractRef
	fieldsOf := map[string][]string{}
	var names []string
	for _, f := range fields {
		name := p[f.field]
		if name == "" {
			continue
		}
		refs = append(refs, adapter.ContractRef{Output: f.output, Kind: adapter.KindDatabaseConfig, Name: name})
		if len(fieldsOf[name]) == 0 {
			names = append(names, name)
		}
		fieldsOf[name] = append(fieldsOf[name], "spec.parameters."+f.field)
	}

	var shared []string
	for _, name := range names {
		if len(fieldsOf[name]) > 1 {
			shared = append(shared, strings.Join(fieldsOf[name], ", ")+" name the same DatabaseConfig "+name)
		}
	}
	if len(shared) > 0 {
		return nil, &conditions.PreCheckFailure{
			Reason:  pluginsv1alpha1.ReasonInvalidParameters,
			Message: strings.Join(shared, ". "),
		}
	}

	return refs, nil
}

// decodeParameters returns the string value of each of keys in parameters,
// matched by exact name. A key that is absent is missing from the result. A
// key whose value is not a string is an error.
func decodeParameters(parameters *apiextensionsv1.JSON, keys ...string) (map[string]string, error) {
	values := map[string]string{}
	if parameters == nil || len(parameters.Raw) == 0 {
		return values, nil
	}

	var object map[string]any
	if err := json.Unmarshal(parameters.Raw, &object); err != nil {
		return nil, fmt.Errorf("decoding the parameters: %w", err)
	}

	for _, key := range keys {
		value, ok := object[key]
		if !ok {
			continue
		}
		s, ok := value.(string)
		if !ok {
			return nil, fmt.Errorf("decoding the parameters: spec.parameters.%s is not a string", key)
		}
		values[key] = s
	}

	return values, nil
}
