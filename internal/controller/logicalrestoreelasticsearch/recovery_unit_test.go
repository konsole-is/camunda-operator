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

package logicalrestoreelasticsearch

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/storagewriter"
)

// A held restore whose registration is gone registers again even when the
// read of its recovery fails, so no other cluster takes the backend meanwhile.
func TestHoldRecoveryRegistersWhenTheReadFails(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	base := fake.NewClientBuilder().WithScheme(scheme).Build()
	outage := errors.New("the API server did not answer")
	reader := interceptor.NewClient(base, interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*v1.CamundaCluster); ok {
				return outage
			}

			return c.Get(ctx, key, obj, opts...)
		},
	})
	r := New(base, reader, scheme, Options{ClaimNamespace: "default"})

	lres := &v1.LogicalRestoreElasticsearch{
		TypeMeta:   metav1.TypeMeta{Kind: "LogicalRestoreElasticsearch"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "my-cluster-restore", UID: "restore-uid"},
		Spec: v1.LogicalRestoreElasticsearchSpec{
			TargetClusterRef: v1.ClusterRef{Name: "my-cluster"},
		},
	}
	lres.Status.RecoveryHeld = new(true)
	lres.Status.Backend = "https://elasticsearch:9200"
	lres.Status.TargetClusterUID = "cluster-uid"

	_, err := r.holdRecovery(context.Background(), lres)

	require.ErrorIs(t, err, outage)
	var leases coordinationv1.LeaseList
	require.NoError(t, base.List(
		context.Background(),
		&leases,
		client.InNamespace("default"),
		client.MatchingLabels{labels.ComponentKey: storagewriter.Component},
	))
	assert.Len(t, leases.Items, 1)
	assert.True(t, *lres.Status.RecoveryHeld)
}
