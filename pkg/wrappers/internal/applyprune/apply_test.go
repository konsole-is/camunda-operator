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

package applyprune_test

import (
	"encoding/json"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/editors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/backupschedule"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/camundaoptimize"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/pluginrequest"
	testenvtest "github.com/konsole-is/camunda-operator/test/envtest"
)

// fieldManager names this test as the owner of the applied fields.
const fieldManager = "claim-wrappers-apply-test"

// testLabel is the label that the mutation of each wrapper sets.
const testLabel = "camunda.io/test"

func TestWrappersApplyOnlyWhatTheySet(t *testing.T) {
	apiClient, scheme := startControlPlane(t)
	c := camundacluster.NewApplyClient(
		camundamanagementcluster.NewApplyClient(
			camundaoptimize.NewApplyClient(
				backupschedule.NewApplyClient(
					pluginrequest.NewApplyClient(apiClient),
				),
			),
		),
	)
	label := func(e *editors.ObjectMetaEditor) error {
		e.EnsureLabel(testLabel, "applied")
		return nil
	}

	cluster, err := camundacluster.NewBuilder(&apiv1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "default"},
		Spec: apiv1.CamundaClusterSpec{
			PlatformConfigRef: "default", StorageRef: "orders-storage", PresetRef: "small", ReleaseRef: "camunda-8-9",
		},
	}).WithMutation(camundacluster.Mutation{Name: "label", Mutate: func(m *camundacluster.Mutator) error {
		m.EditObjectMetadata(label)
		return nil
	}}).Build()
	require.NoError(t, err)

	plane, err := camundamanagementcluster.NewBuilder(&apiv1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "plane", Namespace: "default"},
		Spec: apiv1.CamundaManagementClusterSpec{
			PlatformConfigRef: "default",
			IdentityProvider:  apiv1.IdentityProviderSpec{OIDC: &apiv1.ManagementOIDCSpec{}},
			Identity: apiv1.IdentitySpec{
				Version:           "8.9.0",
				ExternalURL:       "https://plane.example.com/identity",
				DatabaseConfigRef: "plane-identity",
				Admin:             apiv1.IdentityAdminSpec{ClaimName: "email", ClaimValue: "admin@example.com"},
			},
		},
	}).WithMutation(camundamanagementcluster.Mutation{
		Name: "label", Mutate: func(m *camundamanagementcluster.Mutator) error {
			m.EditObjectMetadata(label)
			return nil
		},
	}).Build()
	require.NoError(t, err)

	optimize, err := camundaoptimize.NewBuilder(&apiv1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{Name: "orders-optimize", Namespace: "default"},
		Spec: apiv1.CamundaOptimizeSpec{
			Version: "8.9.0", ManagementAuthRef: "plane", ClusterRef: apiv1.ClusterRef{Name: "orders"},
		},
	}).WithMutation(camundaoptimize.Mutation{Name: "label", Mutate: func(m *camundaoptimize.Mutator) error {
		m.EditObjectMetadata(label)
		return nil
	}}).Build()
	require.NoError(t, err)

	schedule, err := backupschedule.NewBuilder(&apiv1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "orders-backups", Namespace: "default"},
		Spec:       apiv1.BackupScheduleSpec{ClusterRef: apiv1.ClusterRef{Name: "orders"}, Schedule: "0 2 * * *"},
	}).WithMutation(backupschedule.Mutation{Name: "label", Mutate: func(m *backupschedule.Mutator) error {
		m.EditObjectMetadata(label)
		return nil
	}}).Build()
	require.NoError(t, err)

	request, err := pluginrequest.NewBuilder(&pluginsv1alpha1.PluginRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "orders-elasticsearch", Namespace: "default"},
		Spec: pluginsv1alpha1.PluginRequestSpec{
			Section:      "elasticsearch",
			Registration: "elasticsearch",
			ClaimRef:     pluginsv1alpha1.ClaimRef{Kind: pluginsv1alpha1.ClaimKindCamundaClusterClaim, Name: "orders"},
			Target: pluginsv1alpha1.TargetRef{
				Kind: "CamundaCluster", Name: "orders", ServiceAccountName: "orders-camunda",
			},
			DeletionPolicy: pluginsv1alpha1.DeletionPolicyDelete,
		},
	}).WithMutation(pluginrequest.Mutation{Name: "label", Mutate: func(m *pluginrequest.Mutator) error {
		m.EditObjectMetadata(label)
		return nil
	}}).Build()
	require.NoError(t, err)

	tests := []struct {
		name     string
		resource interface{ Preview() (client.Object, error) }
		readBack client.Object
		spec     []string
	}{
		{
			name: "CamundaCluster", resource: cluster, readBack: &apiv1.CamundaCluster{},
			spec: []string{"f:platformConfigRef", "f:presetRef", "f:releaseRef", "f:storageRef"},
		},
		{
			name: "CamundaManagementCluster", resource: plane, readBack: &apiv1.CamundaManagementCluster{},
			spec: []string{"f:identity", "f:identityProvider", "f:platformConfigRef"},
		},
		{
			name: "CamundaOptimize", resource: optimize, readBack: &apiv1.CamundaOptimize{},
			spec: []string{"f:clusterRef", "f:managementAuthRef", "f:version"},
		},
		{
			name: "BackupSchedule", resource: schedule, readBack: &apiv1.BackupSchedule{},
			spec: []string{"f:clusterRef", "f:schedule"},
		},
		{
			name: "PluginRequest", resource: request, readBack: &pluginsv1alpha1.PluginRequest{},
			spec: []string{"f:claimRef", "f:deletionPolicy", "f:registration", "f:section", "f:target"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			desired := applyPreview(t, c, scheme, tt.resource)
			assert.NotEmpty(t, desired.GetUID(), "the answer of the API server reaches the typed object")

			require.NoError(t, apiClient.Get(t.Context(), client.ObjectKeyFromObject(desired), tt.readBack))
			assert.Equal(t, "applied", tt.readBack.GetLabels()[testLabel])
			assert.ElementsMatch(t, tt.spec, ownedSpecFields(t, tt.readBack))
		})
	}

	var storedPlane apiv1.CamundaManagementCluster
	require.NoError(t, apiClient.Get(t.Context(), client.ObjectKey{Namespace: "default", Name: "plane"}, &storedPlane))
	assert.NotNil(t, storedPlane.Spec.IdentityProvider.OIDC, "identityProvider.oidc: {} survives the pruning")

	// The typed encoding of a required field that the wrapper leaves empty
	// is "". An apply of it would take the field from its writer.
	t.Run("a field that another writer set keeps its value", func(t *testing.T) {
		shared := &apiv1.CamundaManagementCluster{
			TypeMeta:   metav1.TypeMeta{APIVersion: apiv1.GroupVersion.String(), Kind: "CamundaManagementCluster"},
			ObjectMeta: metav1.ObjectMeta{Name: "shared", Namespace: "default"},
			Spec: apiv1.CamundaManagementClusterSpec{
				PlatformConfigRef: "default",
				IdentityProvider:  apiv1.IdentityProviderSpec{OIDC: &apiv1.ManagementOIDCSpec{}},
				Identity: apiv1.IdentitySpec{
					Version:           "8.9.0",
					ExternalURL:       "https://shared.example.com/identity",
					DatabaseConfigRef: "shared-identity",
					Admin:             apiv1.IdentityAdminSpec{ClaimName: "email", ClaimValue: "admin@example.com"},
				},
			},
		}
		require.NoError(t, apiClient.Patch(
			t.Context(),
			shared,
			client.Apply, //nolint:staticcheck // the other writer applies the typed object as it is
			client.FieldOwner("other-writer"),
		))

		partial, err := camundamanagementcluster.NewBuilder(&apiv1.CamundaManagementCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "shared", Namespace: "default"},
			Spec:       apiv1.CamundaManagementClusterSpec{PlatformConfigRef: "default"},
		}).Build()
		require.NoError(t, err)
		applyPreview(t, c, scheme, partial)

		var stored apiv1.CamundaManagementCluster
		require.NoError(t, apiClient.Get(t.Context(), client.ObjectKeyFromObject(shared), &stored))
		assert.Equal(t, "https://shared.example.com/identity", stored.Spec.Identity.ExternalURL)
		assert.ElementsMatch(t, []string{"f:platformConfigRef"}, ownedSpecFields(t, &stored))
	})

	t.Run("ClusterReady carries the Ready of the CamundaCluster", func(t *testing.T) {
		var stored apiv1.CamundaCluster
		require.NoError(t, apiClient.Get(t.Context(), client.ObjectKey{Namespace: "default", Name: "orders"}, &stored))
		stored.Status.ObservedGeneration = stored.Generation
		stored.Status.Conditions = []metav1.Condition{{
			Type:               apiv1.ConditionReady,
			Status:             metav1.ConditionFalse,
			Reason:             apiv1.ReasonInvalidReference,
			Message:            "CamundaRelease camunda-8-9 does not exist",
			LastTransitionTime: metav1.Now(),
		}}
		require.NoError(t, apiClient.Status().Update(t.Context(), &stored))

		comp, err := component.NewComponentBuilder().
			WithName("cluster").
			WithConditionType("ClusterReady").
			WithResource(cluster).
			Build()
		require.NoError(t, err)
		owner := &apiv1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "owner", Namespace: "default", UID: "owner-uid", Generation: 1},
		}
		require.NoError(t, comp.Reconcile(t.Context(), component.ReconcileContext{
			Client:        c,
			APIReader:     apiClient,
			Scheme:        scheme,
			EventRecorder: events.NewFakeRecorder(10),
			Owner:         owner,
		}))

		got := comp.GetCondition(owner)
		assert.Equal(t, metav1.ConditionFalse, got.Status)
		assert.Equal(t, "InvalidReference: CamundaRelease camunda-8-9 does not exist", got.Message)
	})
}

// applyPreview applies the desired object of resource through c as
// fieldManager, and returns the object with the answer of the API server.
func applyPreview(
	t *testing.T,
	c client.Client,
	scheme *runtime.Scheme,
	resource interface{ Preview() (client.Object, error) },
) client.Object {
	t.Helper()

	desired, err := resource.Preview()
	require.NoError(t, err)

	// The framework stamps the kind on the object before it patches,
	// because server-side apply reads it from the body.
	gvks, _, err := scheme.ObjectKinds(desired)
	require.NoError(t, err)
	desired.GetObjectKind().SetGroupVersionKind(gvks[0])

	require.NoError(t, c.Patch(
		t.Context(),
		desired,
		client.Apply, //nolint:staticcheck // ocf applies through the deprecated client.Apply patch
		client.FieldOwner(fieldManager),
		client.ForceOwnership,
	))

	return desired
}

// ownedSpecFields returns the spec fields of the first level that
// fieldManager owns on obj.
func ownedSpecFields(t *testing.T, obj client.Object) []string {
	t.Helper()

	for _, entry := range obj.GetManagedFields() {
		if entry.Manager != fieldManager || entry.Operation != metav1.ManagedFieldsOperationApply {
			continue
		}

		var fields map[string]map[string]any
		require.NoError(t, json.Unmarshal(entry.FieldsV1.Raw, &fields))
		assert.NotContains(t, fields, "f:status")

		owned := make([]string, 0, len(fields["f:spec"]))
		for key := range fields["f:spec"] {
			owned = append(owned, key)
		}

		return owned
	}

	require.Fail(t, "no managed fields entry for "+fieldManager)

	return nil
}

// startControlPlane boots an envtest control plane with the CRDs of the
// operator only, and returns a client and the scheme it reads through.
func startControlPlane(t *testing.T) (client.Client, *runtime.Scheme) {
	t.Helper()

	control, err := testenvtest.ControlPlane(testenvtest.Options{
		WithoutECK: true, WithoutKeycloak: true, WithoutCNPG: true,
	})
	require.NoError(t, err)

	cfg, err := control.Start()
	require.NoError(t, err)
	t.Cleanup(func() {
		assert.NoError(t, control.Stop())
	})

	// The scheme is private to this test. Registering into the global
	// scheme.Scheme would leak the kinds into every other test binary.
	scheme := runtime.NewScheme()
	require.NoError(t, apiv1.AddToScheme(scheme))
	require.NoError(t, pluginsv1alpha1.AddToScheme(scheme))

	apiClient, err := client.New(cfg, client.Options{Scheme: scheme})
	require.NoError(t, err)

	return apiClient, scheme
}
