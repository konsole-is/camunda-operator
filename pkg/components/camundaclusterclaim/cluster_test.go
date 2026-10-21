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

package camundaclusterclaim

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/camundacluster"
)

func TestClusterCarriesTheSlotFieldsFromTheOutputs(t *testing.T) {
	cluster := previewCluster(t, fullInput(t))

	assert.Equal(
		t,
		map[string]string{"eks.amazonaws.com/role-arn": "arn:camunda"},
		cluster.Spec.ServiceAccount.Annotations,
	)
	assert.Equal(
		t,
		map[string]string{"eks.amazonaws.com/role-arn": "arn:connectors"},
		cluster.Spec.Connectors.ServiceAccount.Annotations,
	)
	assert.Equal(t, "orders-es", cluster.Spec.StorageRef)
	assert.Equal(t, "orders-backups", cluster.Spec.BackupStorageRef)
	assert.Equal(t, ptr.To("orders-encrypted"), cluster.Spec.Zeebe.StorageClassName)
	assert.Equal(t, "https://orders.example.com", cluster.Spec.ExternalURL)
}

func TestClusterKeepsTheLiveValueOfASlotFieldWhoseOutputIsMissing(t *testing.T) {
	in := fullInput(t)
	access := slices.IndexFunc(in.Requests, func(r pluginsv1alpha1.PluginRequest) bool {
		return r.Spec.Section == "access"
	})
	in.Requests[access].Status.Conditions = nil

	assert.Empty(t, previewCluster(t, in).Spec.ExternalURL, "no live cluster")

	in.Live = &v1.CamundaCluster{Spec: v1.CamundaClusterSpec{ExternalURL: "https://orders.example.com"}}
	assert.Equal(t, "https://orders.example.com", previewCluster(t, in).Spec.ExternalURL)
}

func TestClusterCarriesTheClaimFields(t *testing.T) {
	in := fullInput(t)
	in.Claim.Spec.Cluster = &v1.CamundaClusterSpec{Zeebe: &v1.ZeebeSpec{Partitions: ptr.To[int32](3)}}

	cluster := previewCluster(t, in)

	assert.Equal(t, "orders", cluster.Name)
	assert.Equal(t, "team-a", cluster.Namespace)
	assert.Equal(t, map[string]string{fleetv1alpha1.LabelClaim: "orders"}, cluster.Labels)
	assert.Equal(t, ptr.To[int32](3), cluster.Spec.Zeebe.Partitions)
	assert.Equal(t, ptr.To("orders-encrypted"), cluster.Spec.Zeebe.StorageClassName)
	assert.Equal(t, "small", cluster.Spec.PresetRef)
	assert.Equal(t, "camunda-8-9", cluster.Spec.ReleaseRef)
	assert.Equal(t, "default", cluster.Spec.PlatformConfigRef)
	assert.Equal(t, "orders-camunda", cluster.Spec.ServiceAccount.Name)
	assert.Equal(t, "orders-connectors", cluster.Spec.Connectors.ServiceAccount.Name)
	assert.Nil(t, in.Claim.Spec.Cluster.ServiceAccount, "the claim is not changed")
}

// The apply of the cluster sends only the fields that the claim and the
// outputs set (E spec "Empty values").
func TestClusterApplySendsOnlyTheClaimAndOutputFields(t *testing.T) {
	var sent []byte
	recorder := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Patch: func(
			_ context.Context, _ client.WithWatch, obj client.Object,
			patch client.Patch, _ ...client.PatchOption,
		) error {
			data, err := patch.Data(obj)
			sent = data

			return err
		},
	}).Build()
	in := fullInput(t)
	in.Claim.Spec.Plugins = []fleetv1alpha1.PluginSelection{
		{Section: "secondary-storage", Registration: "elasticsearch"},
	}
	in.Order = in.Claim.Spec.Plugins

	//nolint:staticcheck // ocf applies through the deprecated client.Apply patch
	require.NoError(t, camundacluster.NewApplyClient(recorder).Patch(t.Context(), previewCluster(t, in), client.Apply))

	var applied struct {
		Spec map[string]json.RawMessage `json:"spec"`
	}
	require.NoError(t, json.Unmarshal(sent, &applied))
	assert.Equal(
		t,
		[]string{"connectors", "platformConfigRef", "presetRef", "releaseRef", "serviceAccount", "storageRef"},
		slices.Sorted(maps.Keys(applied.Spec)),
	)
	assert.JSONEq(t, `{"serviceAccount":{"name":"orders-connectors"}}`, string(applied.Spec["connectors"]))
}
