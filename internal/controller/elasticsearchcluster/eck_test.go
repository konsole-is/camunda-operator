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

package elasticsearchcluster

import (
	"testing"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestECKSupportedFollowsTheRESTMapper(t *testing.T) {
	without := meta.NewDefaultRESTMapper(nil)
	assert.False(t, (&ElasticsearchClusterReconciler{restMapper: without}).eckSupported())

	with := meta.NewDefaultRESTMapper(nil)
	with.Add(esv1.GroupVersion.WithKind("Elasticsearch"), meta.RESTScopeNamespace)
	assert.True(t, (&ElasticsearchClusterReconciler{restMapper: with}).eckSupported())
}

// Without ECK the reconcile must not fail, must not publish anything, and
// must tell the user what to do through Ready.
func TestReconcileWithoutECKReportsECKNotInstalled(t *testing.T) {
	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	require.NoError(t, v1.AddToScheme(s))

	cluster := &v1.ElasticsearchCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "es", Namespace: "ns", Generation: 3},
		Spec:       v1.ElasticsearchClusterSpec{SecondaryStorageConfig: "storage"},
	}
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(cluster).
		WithStatusSubresource(&v1.ElasticsearchCluster{}).Build()
	r := &ElasticsearchClusterReconciler{
		Client:          c,
		APIReader:       c,
		Scheme:          s,
		EventRecorder:   events.NewFakeRecorder(16),
		componentClient: c,
		eckInstalled:    false,
	}

	key := types.NamespacedName{Namespace: "ns", Name: "es"}
	result, err := r.Reconcile(t.Context(), ctrl.Request{NamespacedName: key})
	require.NoError(t, err)
	assert.Equal(t, ctrl.Result{}, result, "nothing changes without a restart, so no timer")

	var got v1.ElasticsearchCluster
	require.NoError(t, c.Get(t.Context(), key, &got))
	ready := meta.FindStatusCondition(got.Status.Conditions, v1.ConditionReady)
	require.NotNil(t, ready)
	assert.Equal(t, metav1.ConditionFalse, ready.Status)
	assert.Equal(t, v1.ReasonECKNotInstalled, ready.Reason)
	assert.Contains(t, ready.Message, "restart the operator")
	assert.Equal(t, int64(3), got.Status.ObservedGeneration)

	var secondary v1.SecondaryStorageConfig
	err = c.Get(t.Context(), types.NamespacedName{Namespace: "ns", Name: "storage"}, &secondary)
	assert.True(t, apierrors.IsNotFound(err), "no contract is published without ECK")
}
