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

	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	clustercomponents "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
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
	assert.Equal(t, new("orders-encrypted"), cluster.Spec.Zeebe.StorageClassName)
	assert.Equal(t, "https://orders.example.com", cluster.Spec.ExternalURL)
}

func TestClusterKeepsTheLiveValueOfASlotFieldWhoseOutputIsMissing(t *testing.T) {
	in := fullInput(t)
	access := slices.IndexFunc(in.Requests, func(r pluginsv1alpha1.PluginRequest) bool {
		return r.Spec.Section == accessSection
	})
	in.Requests[access].Status.Conditions = nil

	assert.Empty(t, previewCluster(t, in).Spec.ExternalURL, "no live cluster")

	in.Live = &v1.CamundaCluster{Spec: v1.CamundaClusterSpec{ExternalURL: "https://orders.example.com"}}
	assert.Equal(t, "https://orders.example.com", previewCluster(t, in).Spec.ExternalURL)
}

// RequestsReady can read newer outputs than the cluster was rendered from,
// so the first apply also waits for every slot output of the render.
func TestClusterWaitsForTheSlotOutputsOfTheRenderBeforeTheFirstApply(t *testing.T) {
	tests := []struct {
		name       string
		live       *v1.CamundaCluster
		missing    bool
		wantStatus concepts.GuardStatus
		wantReason string
	}{
		{name: "every output is there", wantStatus: concepts.GuardStatusUnblocked},
		{
			name:       "an output is missing before the cluster exists",
			missing:    true,
			wantStatus: concepts.GuardStatusBlocked,
			wantReason: "waiting for the outputs of access",
		},
		{
			name:       "an output is missing after the cluster exists",
			live:       &v1.CamundaCluster{},
			missing:    true,
			wantStatus: concepts.GuardStatusUnblocked,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := fullInput(t)
			in.Live = tt.live
			if tt.missing {
				access := slices.IndexFunc(in.Requests, func(r pluginsv1alpha1.PluginRequest) bool {
					return r.Spec.Section == accessSection
				})
				in.Requests[access].Status.Conditions = nil
			}

			_, waiting, err := cluster(in)
			require.NoError(t, err)
			status, err := waitFor[v1.CamundaCluster](waiting)(v1.CamundaCluster{})
			require.NoError(t, err)

			assert.Equal(t, tt.wantStatus, status.Status)
			assert.Equal(t, tt.wantReason, status.Reason)
		})
	}
}

// A missing output keeps the annotations that the claim applied, and not
// the keys of another writer.
func TestClusterKeepsOnlyItsOwnAnnotationsWhenTheOutputIsMissing(t *testing.T) {
	in := fullInput(t)
	in.Claim.UID = "claim-uid"
	identity := slices.IndexFunc(in.Requests, func(r pluginsv1alpha1.PluginRequest) bool {
		return r.Spec.Section == "cloud-identity"
	})
	in.Requests[identity].Status.Conditions = nil
	in.Live = &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{ManagedFields: []metav1.ManagedFieldsEntry{
			{
				Manager:   "CamundaClusterClaim/cluster/claim-uid",
				Operation: metav1.ManagedFieldsOperationApply,
				FieldsV1: &metav1.FieldsV1{
					Raw: []byte(
						`{"f:spec":{"f:serviceAccount":{"f:annotations":{"f:eks.amazonaws.com/role-arn":{}}}}}`,
					),
				},
			},
			{
				Manager:   "other-writer",
				Operation: metav1.ManagedFieldsOperationApply,
				FieldsV1: &metav1.FieldsV1{
					Raw: []byte(`{"f:spec":{"f:serviceAccount":{"f:annotations":{"f:team":{}}}}}`),
				},
			},
		}},
		Spec: v1.CamundaClusterSpec{ServiceAccount: &v1.ServiceAccountSpec{Annotations: map[string]string{
			"eks.amazonaws.com/role-arn": "arn:camunda",
			"team":                       "payments",
		}}},
	}

	cluster := previewCluster(t, in)

	assert.Equal(
		t,
		map[string]string{"eks.amazonaws.com/role-arn": "arn:camunda"},
		cluster.Spec.ServiceAccount.Annotations,
	)
	assert.Contains(t, in.Live.Spec.ServiceAccount.Annotations, "team", "the live cluster is not changed")
}

func TestClusterCarriesSuspend(t *testing.T) {
	in := fullInput(t)
	in.Claim.Spec.Suspend = true

	assert.True(t, previewCluster(t, in).Spec.Suspend)
}

func TestClusterDowngradeAnnotation(t *testing.T) {
	const key = clustercomponents.AllowVersionDowngradeAnnotation
	managed := func(version string) *v1.ManagementBinding { return &v1.ManagementBinding{Version: version} }
	refused := func(generation int64) []metav1.Condition {
		return []metav1.Condition{{
			Type:               v1.ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             v1.ReasonVersionDowngradeRefused,
			ObservedGeneration: generation,
		}}
	}

	tests := []struct {
		name  string
		field string
		// release is spec.version of the release of the claim.
		release    string
		live       bool
		management *v1.ManagementBinding
		// annotations are the annotations of the live cluster.
		annotations map[string]string
		claimOwns   bool
		// version is spec.version of the live cluster, set by another writer.
		version    string
		conditions []metav1.Condition
		want       map[string]string
	}{
		{name: "no live cluster", field: "8.9.3", release: "8.9.3"},
		{
			name: "cluster does not run the target", field: "8.9.3", release: "8.9.3", live: true,
			management: managed("8.9.9"), want: map[string]string{key: "8.9.3"},
		},
		{name: "cluster runs the target", field: "8.9.3", release: "8.9.3", live: true, management: managed("8.9.3")},
		{
			name: "management reports the target, the claim wrote the annotation", field: "8.9.3", release: "8.9.3",
			live: true, management: managed("8.9.3"), annotations: map[string]string{key: "8.9.3"}, claimOwns: true,
			want: map[string]string{key: "8.9.3"},
		},
		{
			name: "field names another version than the release", field: "8.9.3", release: "8.9.9", live: true,
			management: managed("8.9.9"),
		},
		{
			name: "field empty", release: "8.9.3", live: true, management: managed("8.9.9"),
			annotations: map[string]string{key: "8.9.3"}, claimOwns: true,
		},
		{
			name: "suspended, the claim wrote it", field: "8.9.3", release: "8.9.3", live: true,
			annotations: map[string]string{key: "8.9.3"}, claimOwns: true, want: map[string]string{key: "8.9.3"},
		},
		{
			name: "suspended, field cleared, the claim wrote it", release: "8.9.3", live: true,
			annotations: map[string]string{key: "8.9.3"}, claimOwns: true, want: map[string]string{key: "8.9.3"},
		},
		{
			name: "suspended, another writer set it", field: "8.9.3", release: "8.9.3", live: true,
			annotations: map[string]string{key: "8.9.3"},
		},
		{name: "suspended, sanction spent", field: "8.9.3", release: "8.9.3", live: true, claimOwns: true},
		{
			name: "resumed, the cluster refuses the downgrade", field: "8.9.3", release: "8.9.3", live: true,
			conditions: refused(2), want: map[string]string{key: "8.9.3"},
		},
		{
			name: "a refusal of an older generation", field: "8.9.3", release: "8.9.3", live: true,
			conditions: refused(1),
		},
		{
			name: "the management version moved, the cluster refuses the downgrade", field: "8.9.3", release: "8.9.3",
			live: true, management: managed("8.9.3"), conditions: refused(2), want: map[string]string{key: "8.9.3"},
		},
		{
			name: "another writer sets the version", field: "8.9.3", release: "8.9.3", live: true,
			management: managed("8.9.9"), version: "8.9.9",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			in := fullInput(t)
			in.Claim.UID = "claim-uid"
			in.Claim.Spec.AllowVersionDowngrade = tt.field
			in.ReleaseVersion = tt.release
			if tt.live {
				in.Live = &v1.CamundaCluster{
					ObjectMeta: metav1.ObjectMeta{Annotations: tt.annotations, Generation: 2},
					Spec:       v1.CamundaClusterSpec{Version: tt.version},
				}
				in.Live.Status.Management = tt.management
				in.Live.Status.Conditions = tt.conditions
				if tt.claimOwns {
					in.Live.ManagedFields = []metav1.ManagedFieldsEntry{ownedDowngrade("claim-uid")}
				}
			}

			assert.Equal(t, tt.want, previewCluster(t, in).Annotations)
		})
	}
}

// ownedDowngrade is the managed fields entry of the cluster component of the
// claim with uid that owns the downgrade annotation.
func ownedDowngrade(uid string) metav1.ManagedFieldsEntry {
	return metav1.ManagedFieldsEntry{
		Manager:   "CamundaClusterClaim/cluster/" + uid,
		Operation: metav1.ManagedFieldsOperationApply,
		FieldsV1: &metav1.FieldsV1{
			Raw: []byte(
				`{"f:metadata":{"f:annotations":{"f:` + clustercomponents.AllowVersionDowngradeAnnotation + `":{}}}}`,
			),
		},
	}
}

func TestClusterCarriesTheClaimFields(t *testing.T) {
	in := fullInput(t)
	in.Claim.Spec.Cluster = &v1.CamundaClusterSpec{Zeebe: &v1.ZeebeSpec{Partitions: ptr.To[int32](3)}}

	cluster := previewCluster(t, in)

	assert.Equal(t, "orders", cluster.Name)
	assert.Equal(t, "team-a", cluster.Namespace)
	assert.Equal(t, map[string]string{fleetv1alpha1.LabelClaim: "orders"}, cluster.Labels)
	assert.Equal(t, ptr.To[int32](3), cluster.Spec.Zeebe.Partitions)
	assert.Equal(t, new("orders-encrypted"), cluster.Spec.Zeebe.StorageClassName)
	assert.Equal(t, "small", cluster.Spec.PresetRef)
	assert.Equal(t, "camunda-8-9", cluster.Spec.ReleaseRef)
	assert.Equal(t, "default", cluster.Spec.PlatformConfigRef)
	assert.Equal(t, "orders-camunda", cluster.Spec.ServiceAccount.Name)
	assert.Equal(t, "orders-connectors", cluster.Spec.Connectors.ServiceAccount.Name)
	assert.Nil(t, in.Claim.Spec.Cluster.ServiceAccount, "the claim is not changed")
}

// The apply of the cluster sends only the fields that the claim and the
// outputs set.
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
