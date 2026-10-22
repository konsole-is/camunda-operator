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
	"strings"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/databaseconfig"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/objectstorageconfig"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

// The kinds of the contracts that the existing registrations read.
const (
	kindObjectStorageConfig    = "ObjectStorageConfig"
	kindSecondaryStorageConfig = "SecondaryStorageConfig"
	kindDatabaseConfig         = "DatabaseConfig"
)

// existing serves a registration that publishes contracts that are already
// in the namespace of the request. It writes nothing, so it has no backing
// component and an empty cleanup.
type existing struct {
	parse func(parameters *apiextensionsv1.JSON) ([]contractRef, error)
}

func (e existing) build(request *pluginsv1alpha1.PluginRequest, _ liveObjects) (served, error) {
	refs, err := e.parse(request.Spec.Parameters)
	if err != nil {
		return served{}, err
	}

	b := component.NewComponentBuilder().
		WithName("outputs").
		WithConditionType(component.ConditionType(pluginsv1alpha1.ConditionOutputsReady))
	cells := make([]*concepts.Data[metav1.Condition], 0, len(refs))
	for _, ref := range refs {
		res, cell, err := readContract(ref, request.Namespace)
		if err != nil {
			return served{}, err
		}
		b.WithResource(res, component.ReadOnly(), component.BlockOnAbsence())
		cells = append(cells, cell)
	}

	outputs, err := b.Build()
	if err != nil {
		return served{}, fmt.Errorf("building the outputs component: %w", err)
	}

	return served{
		outputs: outputs,
		report: func() report {
			rep := report{outputs: map[string]string{}}
			for i, ref := range refs {
				rep.outputs[ref.output] = ref.name
				rep.summary = append(rep.summary, summaryItem(ref, cells[i]))
			}
			return rep
		},
	}, nil
}

func (existing) cleanup(*pluginsv1alpha1.PluginRequest, pluginsv1alpha1.DeletionPolicy, liveObjects) (cleanup, error) {
	return cleanup{}, nil
}

func (e existing) contracts(request *pluginsv1alpha1.PluginRequest) []contractRef {
	refs, err := e.parse(request.Spec.Parameters)
	if err != nil {
		return nil
	}

	return refs
}

func (existing) backing(*pluginsv1alpha1.PluginRequest) []backingRef {
	return nil
}

// oneContract returns a parse function for the parameters {"name": string},
// which name one contract of kind that the request publishes as output.
func oneContract(output, kind string) func(*apiextensionsv1.JSON) ([]contractRef, error) {
	return func(parameters *apiextensionsv1.JSON) ([]contractRef, error) {
		p, err := decodeParameters(parameters, "name")
		if err != nil {
			return nil, err
		}

		return []contractRef{{output: output, kind: kind, name: p["name"]}}, nil
	}
}

// databaseConfigs returns a contract for each DatabaseConfig name that the
// parameters set, in the order keycloak, identity, webModeler. Two fields
// with one name return a *conditions.PreCheckFailure with reason
// InvalidParameters.
func databaseConfigs(parameters *apiextensionsv1.JSON) ([]contractRef, error) {
	p, err := decodeParameters(parameters, "keycloak", "identity", "webModeler")
	if err != nil {
		return nil, err
	}

	fields := []struct{ field, output string }{
		{"keycloak", pluginstartup.OutputKeycloakDatabaseConfig},
		{"identity", pluginstartup.OutputIdentityDatabaseConfig},
		{"webModeler", pluginstartup.OutputWebModelerDatabaseConfig},
	}

	var refs []contractRef
	fieldsOf := map[string][]string{}
	var names []string
	for _, f := range fields {
		name := p[f.field]
		if name == "" {
			continue
		}
		refs = append(refs, contractRef{output: f.output, kind: kindDatabaseConfig, name: name})
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

// readContract returns a read of the contract of ref in namespace, and the
// cell that holds its Ready condition once it is read. The condition is the
// zero value when the contract has none.
func readContract(
	ref contractRef,
	namespace string,
) (component.Resource, *concepts.Data[metav1.Condition], error) {
	cell := concepts.NewData[metav1.Condition](ref.output)
	object := metav1.ObjectMeta{Name: ref.name, Namespace: namespace}

	switch ref.kind {
	case kindObjectStorageConfig:
		b := objectstorageconfig.NewBuilder(&v1.ObjectStorageConfig{ObjectMeta: object})
		objectstorageconfig.ExtractInto(b, cell, func(c v1.ObjectStorageConfig) (metav1.Condition, error) {
			return readyOf(c.Status.Conditions), nil
		})
		res, err := b.Build()
		return res, cell, err
	case kindSecondaryStorageConfig:
		b := secondarystorageconfig.NewBuilder(&v1.SecondaryStorageConfig{ObjectMeta: object})
		secondarystorageconfig.ExtractInto(b, cell, func(c v1.SecondaryStorageConfig) (metav1.Condition, error) {
			return readyOf(c.Status.Conditions), nil
		})
		res, err := b.Build()
		return res, cell, err
	case kindDatabaseConfig:
		b := databaseconfig.NewBuilder(&v1.DatabaseConfig{ObjectMeta: object})
		databaseconfig.ExtractInto(b, cell, func(c v1.DatabaseConfig) (metav1.Condition, error) {
			return readyOf(c.Status.Conditions), nil
		})
		res, err := b.Build()
		return res, cell, err
	default:
		return nil, nil, fmt.Errorf("no contract of kind %q", ref.kind)
	}
}

func readyOf(conds []metav1.Condition) metav1.Condition {
	if ready := meta.FindStatusCondition(conds, v1.ConditionReady); ready != nil {
		return *ready
	}

	return metav1.Condition{}
}

// summaryItem says whether the contract of ref is there and Ready. A cell
// without a value means that the contract, or a contract before it, is
// missing.
func summaryItem(ref contractRef, cell *concepts.Data[metav1.Condition]) pluginsv1alpha1.SummaryItem {
	item := pluginsv1alpha1.SummaryItem{Label: ref.kind, Severity: pluginsv1alpha1.SeverityWarning}
	ready, ok := cell.Get()
	switch {
	case !ok:
		item.Value = ref.name + ", waiting"
	case ready.Status == metav1.ConditionTrue:
		item.Value = ref.name + ", Ready"
		item.Severity = pluginsv1alpha1.SeverityInfo
	case ready.Reason != "":
		item.Value = ref.name + ", not Ready (" + ready.Reason + ")"
	default:
		item.Value = ref.name + ", not Ready"
	}

	return item
}
