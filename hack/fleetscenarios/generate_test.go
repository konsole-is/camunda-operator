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
	"context"
	"encoding/base64"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/konsole-is/camunda-operator/internal/manager"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
)

func TestGenerate(t *testing.T) {
	sc, err := readScenario("testdata/one-broker/scenario.yaml")
	require.NoError(t, err)

	first, err := generate(t.Context(), sc)
	require.NoError(t, err, "the base state is a cluster that waits out its grace period, and must settle")
	second, err := generate(t.Context(), sc)
	require.NoError(t, err)

	t.Run("writes the same bytes on a second run", func(t *testing.T) {
		require.ElementsMatch(t, slices.Collect(maps.Keys(first)), slices.Collect(maps.Keys(second)))
		for name := range first {
			a, err := encode(first[name])
			require.NoError(t, err)
			b, err := encode(second[name])
			require.NoError(t, err)
			assert.Equal(t, string(a), string(b), name)
		}
	})

	t.Run("shows the workload step on the StatefulSet and on the cluster", func(t *testing.T) {
		sts := find(t, first[objectsFile], "StatefulSet", "team-b", "orders-zeebe")
		ready, _, _ := unstructured.NestedInt64(sts.Object, "status", "readyReplicas")
		assert.Equal(t, int64(1), ready)

		cluster := find(t, first[objectsFile], "CamundaCluster", "team-b", "orders")
		zeebe := condition(t, cluster, "ZeebeReady")
		assert.Equal(t, "False", zeebe["status"])
		assert.Contains(t, zeebe["message"], "1/3 ready")
		assert.Equal(t, "False", condition(t, cluster, "Ready")["status"])
	})

	t.Run("shows the named state after its step", func(t *testing.T) {
		objs := first["brokers-ready.yaml"]
		sts := find(t, objs, "StatefulSet", "team-b", "orders-zeebe")
		ready, _, _ := unstructured.NestedInt64(sts.Object, "status", "readyReplicas")
		assert.Equal(t, int64(3), ready)

		readyCond := condition(t, find(t, objs, "CamundaCluster", "team-b", "orders"), "Ready")
		assert.Equal(t, "True", readyCond["status"])
		assert.Equal(t, "Healthy", readyCond["reason"])
	})

	t.Run("shows the overlay in every state, written by the overlay manager", func(t *testing.T) {
		for _, name := range []string{objectsFile, "brokers-ready.yaml"} {
			cluster := find(t, first[name], "CamundaCluster", "team-b", "orders")
			rotation, _, _ := unstructured.NestedString(cluster.Object, "status", "adminPassword", "rotation")
			assert.Equal(t, "2026-01", rotation, name)
			assert.Contains(t, managers(cluster), overlayManager, name)
		}
	})

	t.Run("names the field managers of the reconcilers", func(t *testing.T) {
		clusterUID := scenarioUID("CamundaCluster", "team-b", "orders")
		sts := find(t, first[objectsFile], "StatefulSet", "team-b", "orders-zeebe")
		assert.Contains(t, managers(sts), "CamundaCluster/"+components.ComponentZeebe+"/"+clusterUID)
		assert.Contains(t, managers(sts), kubeControllerManager)

		cluster := find(t, first[objectsFile], "CamundaCluster", "team-b", "orders")
		assert.Contains(t, managers(cluster), seedManager)
		assert.Contains(t, managers(cluster), deployedManager, "a status write names no field manager")
	})

	t.Run("writes one value for every key of a Secret", func(t *testing.T) {
		for _, obj := range first[objectsFile] {
			if obj.GetKind() != "Secret" {
				continue
			}
			data, _, _ := unstructured.NestedStringMap(obj.Object, "data")
			for key, value := range data {
				assert.Equal(t, base64.StdEncoding.EncodeToString([]byte(secretValue)), value, describe(&obj)+" "+key)
			}
		}
	})

	t.Run("keeps the Events of the objects of the scenario", func(t *testing.T) {
		var events int
		for _, obj := range first[objectsFile] {
			if isEvent(&obj) {
				events++
			}
		}
		assert.Positive(t, events)
	})
}

func TestApplyOverlaysNamesAMissingObject(t *testing.T) {
	s := runtime.NewScheme()
	require.NoError(t, manager.AddToScheme(s))
	c := fake.NewClientBuilder().WithScheme(s).Build()

	err := applyOverlays(context.Background(), c, []overlay{{
		Reason: "test",
		Object: objectRef{
			APIVersion: "core.camunda.io/v1",
			Kind:       "CamundaCluster",
			Namespace:  "team-b",
			Name:       "missing",
		},
		Status: map[string]any{"adminPassword": map[string]any{"rotation": "2026-01"}},
	}})
	assert.EqualError(t, err, "overlay names CamundaCluster team-b/missing, which does not exist")
}

// condition returns the condition of obj with type conditionType, and fails
// the test when obj has none.
func condition(t *testing.T, obj unstructured.Unstructured, conditionType string) map[string]any {
	t.Helper()

	conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	for _, c := range conditions {
		if m, ok := c.(map[string]any); ok && m["type"] == conditionType {
			return m
		}
	}
	require.Failf(t, "condition not found", "%s has no condition %s", describe(&obj), conditionType)
	return nil
}

// managers returns the field managers of obj.
func managers(obj unstructured.Unstructured) []string {
	names := make([]string, 0, len(obj.GetManagedFields()))
	for _, f := range obj.GetManagedFields() {
		names = append(names, f.Manager)
	}
	return names
}
