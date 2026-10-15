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

package camundacluster

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
)

// A CamundaCluster that sets only presetRef and releaseRef applies those
// two fields and the metadata. The typed encoding of the same object holds
// a status, so the test also shows that the client removed it.
func TestApplyClientSendsOnlyTheSetFields(t *testing.T) {
	t.Parallel()

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
	cluster := &apiv1.CamundaCluster{
		TypeMeta:   metav1.TypeMeta{APIVersion: apiv1.GroupVersion.String(), Kind: "CamundaCluster"},
		ObjectMeta: metav1.ObjectMeta{Name: "orders", Namespace: "team-a"},
		Spec:       apiv1.CamundaClusterSpec{PresetRef: "small", ReleaseRef: "camunda-8-9"},
		Status:     apiv1.CamundaClusterStatus{ObservedGeneration: 1},
	}

	typed, err := json.Marshal(cluster)
	require.NoError(t, err)
	assert.Contains(t, string(typed), `"status":{"observedGeneration":1}`)

	//nolint:staticcheck // ocf applies through the deprecated client.Apply patch
	require.NoError(t, NewApplyClient(recorder).Patch(t.Context(), cluster, client.Apply))

	want := `{
		"apiVersion": "core.camunda.io/v1",
		"kind": "CamundaCluster",
		"metadata": {"name": "orders", "namespace": "team-a"},
		"spec": {"presetRef": "small", "releaseRef": "camunda-8-9"}
	}`
	assert.JSONEq(t, want, string(sent))
}
