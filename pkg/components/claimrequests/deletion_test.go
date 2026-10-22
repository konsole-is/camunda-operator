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

package claimrequests

import (
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

func TestDeletionPolicyApplyHoldsOnlyThePolicyAndTheUID(t *testing.T) {
	request := &pluginsv1alpha1.PluginRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "orders-backup-storage",
			Namespace: "orders-ns",
			UID:       "uid-2",
			Labels:    map[string]string{"a": "b"},
		},
		Spec: pluginsv1alpha1.PluginRequestSpec{
			Section:        "backup-storage",
			Registration:   "s3",
			DeletionPolicy: pluginsv1alpha1.DeletionPolicyDelete,
		},
	}

	got := DeletionPolicyApply(request, pluginsv1alpha1.DeletionPolicyOrphan)

	want := map[string]any{
		"apiVersion": "plugins.camunda.io/v1alpha1",
		"kind":       "PluginRequest",
		"metadata":   map[string]any{"name": "orders-backup-storage", "namespace": "orders-ns", "uid": "uid-2"},
		"spec":       map[string]any{"deletionPolicy": "Orphan"},
	}
	assert.Equal(t, want, got.Object)
}

func TestDeletionPolicyFieldManager(t *testing.T) {
	assert.Equal(
		t,
		"CamundaClusterClaim/deletionpolicy/uid-1",
		DeletionPolicyFieldManager(fleetv1alpha1.FieldManagerPrefixClusterClaim, "uid-1"),
	)
}

func TestWaitingMessage(t *testing.T) {
	t.Run("names each object", func(t *testing.T) {
		msg := WaitingMessage([]string{"CamundaCluster orders", "PluginRequest orders-backup-storage"})

		assert.Equal(
			t,
			"The claim is deleted after these objects are gone: CamundaCluster orders, PluginRequest orders-backup-storage",
			msg,
		)
	})

	t.Run("bounds a long list", func(t *testing.T) {
		names := make([]string, 32)
		for i := range names {
			names[i] = "PluginRequest " + strings.Repeat("x", 250) + strconv.Itoa(i)
		}

		msg := WaitingMessage(names)

		assert.LessOrEqual(t, len(msg), conditions.MaxMessageLength)
		assert.Contains(t, msg, names[0])
	})
}
