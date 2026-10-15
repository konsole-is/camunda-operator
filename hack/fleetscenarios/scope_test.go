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

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestUnobserved(t *testing.T) {
	kind := schema.GroupKind{Group: "plugins.camunda.io", Kind: "PluginRegistration"}
	sc := &scope{observed: map[schema.GroupKind]bool{kind: true}}

	withStatus := func(name string, status map[string]any) unstructured.Unstructured {
		obj := object("plugins.camunda.io/v1alpha1", "PluginRegistration", "", name, name)
		obj.SetGeneration(2)
		if status != nil {
			require.NoError(t, unstructured.SetNestedField(obj.Object, status, "status"))
		}
		return obj
	}
	condition := func(conditionType string, generation int64) any {
		return map[string]any{"type": conditionType, "status": "True", "observedGeneration": generation}
	}

	objs := []unstructured.Unstructured{
		withStatus("no-status", nil),
		withStatus("observed-field", map[string]any{"observedGeneration": int64(2)}),
		withStatus("behind-field", map[string]any{
			"observedGeneration": int64(1),
			"conditions":         []any{condition("Accepted", 2)},
		}),
		withStatus("conditions-current", map[string]any{"conditions": []any{condition("Accepted", 2)}}),
		withStatus("a-condition-behind", map[string]any{
			"conditions": []any{condition("Accepted", 2), condition("Ready", 1)},
		}),
		withStatus("a-condition-not-an-object", map[string]any{
			"conditions": []any{condition("Accepted", 2), "Ready"},
		}),
	}

	got := sc.unobserved(objs)

	assert.Len(t, got, 4)
	assert.Contains(t, got[0], "no-status")
	assert.Contains(t, got[1], "behind-field")
	assert.Contains(t, got[2], "a-condition-behind")
	assert.Contains(t, got[3], "a-condition-not-an-object")
}
