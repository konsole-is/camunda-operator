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
	"errors"
	"fmt"
	"slices"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/adapter"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/planedatabases"
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
// publishes a DatabaseConfig for each component of planedatabases.All that
// the parameters name. When the parameters hold databases, it publishes only
// the components that databases lists.
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
// parameters set, in the order of planedatabases.All. When the parameters hold
// databases, the name of a component that databases does not list is left
// out. Two contracts with one name return a *conditions.PreCheckFailure with
// reason InvalidParameters. A databases value that is not a list of strings
// is an error.
func databaseConfigs(parameters *apiextensionsv1.JSON) ([]adapter.ContractRef, error) {
	fields := make([]string, 0, len(planedatabases.All))
	for _, c := range planedatabases.Components() {
		fields = append(fields, string(c))
	}

	p, err := decodeParameters(parameters, fields...)
	if err != nil {
		return nil, err
	}

	listed, err := decodeDatabases(parameters)
	if err != nil {
		return nil, err
	}

	var refs []adapter.ContractRef
	fieldsOf := map[string][]string{}
	var names []string
	for _, d := range planedatabases.All {
		field := string(d.Component)
		name := p[field]
		if name == "" || (listed != nil && !slices.Contains(listed, field)) {
			continue
		}
		refs = append(refs, adapter.ContractRef{Output: d.Output, Kind: adapter.KindDatabaseConfig, Name: name})
		if len(fieldsOf[name]) == 0 {
			names = append(names, name)
		}
		fieldsOf[name] = append(fieldsOf[name], "spec.parameters."+field)
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

// decodeDatabases returns the databases list of parameters, matched by exact
// name. It is nil when the parameters do not hold databases, and empty, not
// nil, for an empty list.
func decodeDatabases(parameters *apiextensionsv1.JSON) ([]string, error) {
	if parameters == nil || len(parameters.Raw) == 0 {
		return nil, nil
	}

	var object map[string]json.RawMessage
	if err := json.Unmarshal(parameters.Raw, &object); err != nil {
		return nil, fmt.Errorf("decoding the parameters: %w", err)
	}

	var listed []string
	if raw, ok := object["databases"]; ok && json.Unmarshal(raw, &listed) != nil {
		return nil, errors.New("decoding the parameters: spec.parameters.databases is not a list of strings")
	}

	return listed, nil
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
