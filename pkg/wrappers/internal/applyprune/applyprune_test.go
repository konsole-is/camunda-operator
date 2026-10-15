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

package applyprune

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The pruned content holds only what the object sets. The status of the
// object never joins an apply.
func TestPruneDropsStatusAndEmptyValues(t *testing.T) {
	t.Parallel()

	cluster := &apiv1.CamundaCluster{
		TypeMeta:   metav1.TypeMeta{APIVersion: apiv1.GroupVersion.String(), Kind: "CamundaCluster"},
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "team-a"},
		Spec:       apiv1.CamundaClusterSpec{PresetRef: "small"},
		Status:     apiv1.CamundaClusterStatus{ObservedGeneration: 2},
	}

	got, err := Prune(cluster)
	require.NoError(t, err)

	assert.Equal(
		t, map[string]any{
			"apiVersion": "core.camunda.io/v1",
			"kind":       "CamundaCluster",
			"metadata":   map[string]any{"name": "orders", "namespace": "team-a"},
			"spec":       map[string]any{"presetRef": "small"},
		}, got.Object,
	)
}

// A struct field whose fields all hold nothing is dropped, also when it is
// required in the schema: the writer did not set it.
func TestPruneDropsAStructWithoutASetField(t *testing.T) {
	t.Parallel()

	plane := &apiv1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "plane", Namespace: "team-a"},
		Spec:       apiv1.CamundaManagementClusterSpec{PlatformConfigRef: "default"},
	}

	got, err := Prune(plane)
	require.NoError(t, err)

	assert.Equal(t, map[string]any{"platformConfigRef": "default"}, got.Object["spec"])
}

// identityProvider.oidc: {} selects a mode. A set pointer is a value, even
// when it points to a struct without fields.
func TestPruneKeepsASetPointerToAnEmptyStruct(t *testing.T) {
	t.Parallel()

	plane := &apiv1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "plane", Namespace: "team-a"},
		Spec: apiv1.CamundaManagementClusterSpec{
			IdentityProvider: apiv1.IdentityProviderSpec{OIDC: &apiv1.ManagementOIDCSpec{}},
		},
	}

	got, err := Prune(plane)
	require.NoError(t, err)

	assert.Equal(
		t, map[string]any{
			"identityProvider": map[string]any{"oidc": map[string]any{}},
		}, got.Object["spec"],
	)
}

// A key of a map is a value, so a label with an empty value stays. An item
// of a list stays too.
func TestPruneKeepsMapEntriesAndListItems(t *testing.T) {
	t.Parallel()

	request := &pluginsv1alpha1.PluginRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "orders-elasticsearch",
			Namespace: "team-a",
			Labels:    map[string]string{"team": ""},
		},
		Spec: pluginsv1alpha1.PluginRequestSpec{
			Inputs: map[string]string{"bucket": ""},
			Target: pluginsv1alpha1.TargetRef{AdditionalServiceAccounts: []string{""}},
		},
	}

	got, err := Prune(request)
	require.NoError(t, err)

	labels, _, err := unstructured.NestedStringMap(got.Object, "metadata", "labels")
	require.NoError(t, err)
	assert.Equal(t, map[string]string{"team": ""}, labels)
	assert.Equal(
		t, map[string]any{
			"inputs": map[string]any{"bucket": ""},
			"target": map[string]any{"additionalServiceAccounts": []any{""}},
		}, got.Object["spec"],
	)
}

// Parameters are free-form JSON. Their Go type names no field, so their
// content stays as the writer set it.
func TestPruneKeepsFreeFormJSONAsItIs(t *testing.T) {
	t.Parallel()

	request := &pluginsv1alpha1.PluginRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "orders-access", Namespace: "team-a"},
		Spec: pluginsv1alpha1.PluginRequestSpec{
			Parameters: &apiextensionsv1.JSON{Raw: []byte(`{"host":"","tls":{}}`)},
		},
	}

	got, err := Prune(request)
	require.NoError(t, err)

	params, _, err := unstructured.NestedMap(got.Object, "spec", "parameters")
	require.NoError(t, err)
	assert.Equal(t, map[string]any{"host": "", "tls": map[string]any{}}, params)
}

// A number of zero is a value. Only strings, nulls, lists, and maps can be
// empty.
func TestPruneKeepsZeroNumbers(t *testing.T) {
	t.Parallel()

	schedule := &apiv1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: "orders-backups", Namespace: "team-a"},
		Spec: apiv1.BackupScheduleSpec{
			Retained: &apiv1.RetainedBackups{Completed: new(int32(0))},
		},
	}

	got, err := Prune(schedule)
	require.NoError(t, err)

	completed, found, err := unstructured.NestedInt64(got.Object, "spec", "retained", "completed")
	require.NoError(t, err)
	assert.True(t, found)
	assert.Zero(t, completed)
}

// patchRecorder returns a client that records the object and the patch
// type of every Patch call, and runs mutate on the object when it is set,
// the way the API server answers with the stored object.
func patchRecorder(mutate func(client.Object)) (client.Client, *[]client.Object, *[]types.PatchType) {
	var objects []client.Object
	var patchTypes []types.PatchType
	recorder := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Patch: func(
			_ context.Context, _ client.WithWatch, obj client.Object,
			patch client.Patch, _ ...client.PatchOption,
		) error {
			objects = append(objects, obj)
			patchTypes = append(patchTypes, patch.Type())
			if mutate != nil {
				mutate(obj)
			}

			return nil
		},
	}).Build()

	return recorder, &objects, &patchTypes
}

func TestClientAppliesThePrunedContent(t *testing.T) {
	t.Parallel()

	recorder, objects, _ := patchRecorder(nil)
	cluster := &apiv1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "team-a"},
		Spec:       apiv1.CamundaClusterSpec{PresetRef: "small"},
	}

	//nolint:staticcheck // ocf applies through the deprecated client.Apply patch
	require.NoError(t, NewClient[*apiv1.CamundaCluster](recorder).Patch(t.Context(), cluster, client.Apply))

	require.Len(t, *objects, 1)
	sent, ok := (*objects)[0].(*unstructured.Unstructured)
	require.True(t, ok, "the apply must send the pruned content, got %T", (*objects)[0])
	assert.NotContains(t, sent.Object, "status")
}

// The handlers of a component read the object after the apply, so the
// answer of the API server must reach the typed object.
func TestClientDecodesTheAnswerIntoTheObject(t *testing.T) {
	t.Parallel()

	recorder, _, _ := patchRecorder(func(obj client.Object) {
		obj.SetUID("uid-1")
		obj.SetGeneration(3)
	})
	cluster := &apiv1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "team-a"},
		Spec:       apiv1.CamundaClusterSpec{PresetRef: "small"},
	}

	//nolint:staticcheck // ocf applies through the deprecated client.Apply patch
	require.NoError(t, NewClient[*apiv1.CamundaCluster](recorder).Patch(t.Context(), cluster, client.Apply))

	assert.Equal(t, types.UID("uid-1"), cluster.UID)
	assert.Equal(t, int64(3), cluster.Generation)
	assert.Equal(t, "small", cluster.Spec.PresetRef)
}

// Another kind, and a patch that is not an apply, reach the inner client
// unchanged.
func TestClientPassesOtherCallsThrough(t *testing.T) {
	t.Parallel()

	recorder, objects, patchTypes := patchRecorder(nil)
	c := NewClient[*apiv1.CamundaCluster](recorder)
	configMap := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "settings", Namespace: "team-a"}}
	cluster := &apiv1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "team-a"}}

	//nolint:staticcheck // ocf applies through the deprecated client.Apply patch
	require.NoError(t, c.Patch(t.Context(), configMap, client.Apply))
	require.NoError(t, c.Patch(t.Context(), cluster, client.MergeFrom(cluster.DeepCopy())))

	require.Len(t, *objects, 2)
	assert.Same(t, configMap, (*objects)[0])
	assert.Same(t, cluster, (*objects)[1])
	assert.Equal(t, []types.PatchType{types.ApplyPatchType, types.MergePatchType}, *patchTypes)
}
