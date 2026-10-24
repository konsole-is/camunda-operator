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

package adapter

import (
	"fmt"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/databaseconfig"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/objectstorageconfig"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

// OutputsComponent returns the component with condition type OutputsReady
// that waits for each contract of refs in namespace, and the cells that hold
// their Ready conditions, in the order of refs.
func OutputsComponent(
	refs []ContractRef,
	namespace string,
) (*component.Component, []*concepts.Data[metav1.Condition], error) {
	b := component.NewComponentBuilder().
		WithName("outputs").
		WithConditionType(component.ConditionType(pluginsv1alpha1.ConditionOutputsReady))
	cells := make([]*concepts.Data[metav1.Condition], 0, len(refs))
	for _, ref := range refs {
		res, cell, err := readContract(ref, namespace)
		if err != nil {
			return nil, nil, err
		}
		b.WithResource(res, component.ReadOnly(), component.BlockOnAbsence())
		cells = append(cells, cell)
	}

	outputs, err := b.Build()
	if err != nil {
		return nil, nil, fmt.Errorf("building the outputs component: %w", err)
	}

	return outputs, cells, nil
}

// readContract returns a read of the contract of ref in namespace, and the
// cell that holds its Ready condition once it is read. The condition is the
// zero value when the contract has none.
func readContract(
	ref ContractRef,
	namespace string,
) (component.Resource, *concepts.Data[metav1.Condition], error) {
	cell := concepts.NewData[metav1.Condition](ref.Output)
	object := metav1.ObjectMeta{Name: ref.Name, Namespace: namespace}

	switch ref.Kind {
	case KindObjectStorageConfig:
		b := objectstorageconfig.NewBuilder(&v1.ObjectStorageConfig{ObjectMeta: object})
		objectstorageconfig.ExtractInto(b, cell, func(c v1.ObjectStorageConfig) (metav1.Condition, error) {
			return ReadyOf(c.Status.Conditions), nil
		})
		res, err := b.Build()
		return res, cell, err
	case KindSecondaryStorageConfig:
		b := secondarystorageconfig.NewBuilder(&v1.SecondaryStorageConfig{ObjectMeta: object})
		secondarystorageconfig.ExtractInto(b, cell, func(c v1.SecondaryStorageConfig) (metav1.Condition, error) {
			return ReadyOf(c.Status.Conditions), nil
		})
		res, err := b.Build()
		return res, cell, err
	case KindDatabaseConfig:
		b := databaseconfig.NewBuilder(&v1.DatabaseConfig{ObjectMeta: object})
		databaseconfig.ExtractInto(b, cell, func(c v1.DatabaseConfig) (metav1.Condition, error) {
			return ReadyOf(c.Status.Conditions), nil
		})
		res, err := b.Build()
		return res, cell, err
	default:
		return nil, nil, fmt.Errorf("no contract of kind %q", ref.Kind)
	}
}

// ReadyOf returns the Ready condition of conds, or the zero value when conds
// has none.
func ReadyOf(conds []metav1.Condition) metav1.Condition {
	if ready := meta.FindStatusCondition(conds, v1.ConditionReady); ready != nil {
		return *ready
	}

	return metav1.Condition{}
}

// SummaryItem says whether the object name is there and Ready, with label as
// the label of the item. A cell without a value means that the object, or an
// object before it, is missing.
func SummaryItem(label, name string, cell *concepts.Data[metav1.Condition]) pluginsv1alpha1.SummaryItem {
	item := pluginsv1alpha1.SummaryItem{Label: label, Severity: pluginsv1alpha1.SeverityWarning}
	ready, ok := cell.Get()
	switch {
	case !ok:
		item.Value = name + ", waiting"
	case ready.Status == metav1.ConditionTrue:
		item.Value = name + ", Ready"
		item.Severity = pluginsv1alpha1.SeverityInfo
	case ready.Reason != "":
		item.Value = name + ", not Ready (" + ready.Reason + ")"
	default:
		item.Value = name + ", not Ready"
	}

	return item
}

// ServerItem says whether the DatabaseServer server is there and Ready, with
// its PostgreSQL version once the server reports it.
func ServerItem(server string, cell *concepts.Data[v1.DatabaseServer]) pluginsv1alpha1.SummaryItem {
	ready := concepts.NewData[metav1.Condition](server)
	live, ok := cell.Get()
	if ok {
		cond := ReadyOf(live.Status.Conditions)
		if live.Status.ObservedGeneration != live.Generation ||
			cond.Type != "" && cond.ObservedGeneration != live.Generation {
			return pluginsv1alpha1.SummaryItem{
				Label:    KindDatabaseServer,
				Value:    server + ", updating",
				Severity: pluginsv1alpha1.SeverityWarning,
			}
		}
		ready.Set(cond)
	}

	item := SummaryItem(KindDatabaseServer, server, ready)
	if item.Severity == pluginsv1alpha1.SeverityInfo && live.Status.Version != "" {
		item.Value = server + ", PostgreSQL " + live.Status.Version + ", Ready"
	}

	return item
}
