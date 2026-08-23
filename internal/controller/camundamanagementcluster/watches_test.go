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

package camundamanagementcluster

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
)

// enqueueNamespace is the namespace of every management cluster of these
// tests.
const enqueueNamespace = "camunda"

// The watch on the orchestration clusters follows the same selector rule as
// the reconcile: an unset selector matches no cluster and an empty one matches
// every cluster. A management cluster that is enqueued for a cluster it can
// never serve reconciles for nothing on every event of that cluster.
func TestEnqueueForCluster(t *testing.T) {
	scheme, err := v1.SchemeBuilder.Build()
	require.NoError(t, err)

	unset := managementClusterWithSelector("unset", nil)
	every := managementClusterWithSelector("every", &metav1.LabelSelector{})
	production := managementClusterWithSelector(
		"production", &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "production"}},
	)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(unset, every, production).Build()
	handle := (&Reconciler{Client: c}).enqueueForCluster()

	t.Run("an empty selector is enqueued and an unset one is not", func(t *testing.T) {
		assert.Equal(
			t,
			[]reconcile.Request{request("every"), request("production")},
			enqueued(t, handle, orchestrationCluster(map[string]string{"tier": "production"}, "")),
		)
	})

	t.Run("a selector that does not match is not enqueued", func(t *testing.T) {
		assert.Equal(
			t,
			[]reconcile.Request{request("every")},
			enqueued(t, handle, orchestrationCluster(map[string]string{"tier": "staging"}, "")),
		)
	})

	// The holder has to withdraw the claim, and its selector no longer
	// reaches the cluster to tell it that.
	t.Run("the holder of the claim is enqueued whatever its selector", func(t *testing.T) {
		assert.Equal(
			t,
			[]reconcile.Request{request("every"), request("unset")},
			enqueued(t, handle, orchestrationCluster(nil, enqueueNamespace+"/unset")),
		)
	})
}

// A Secret that the spec names itself may live in any namespace. The
// namespace enqueue never reaches one outside the management namespace, so
// without this index a rotation of it refreshes neither the copy nor the pods
// that read it.
func TestSecretRefsIndexesEveryReferenceOfTheSpec(t *testing.T) {
	t.Parallel()

	mc := &v1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "management", Namespace: enqueueNamespace},
		Spec: v1.CamundaManagementClusterSpec{
			IdentityProvider: v1.IdentityProviderSpec{ExternalKeycloak: &v1.ExternalKeycloakSpec{
				AdminCredentialsSecretRef: v1.CredentialsSecretRef{
					Name: "keycloak-admin", Namespace: "identity",
				},
			}},
			Identity: v1.IdentitySpec{Admin: v1.IdentityAdminSpec{
				Username:          "admin",
				PasswordSecretRef: &v1.SecretKeyRef{Name: "admin-password", Namespace: "secrets"},
			}},
			WebModeler: &v1.WebModelerSpec{Mail: v1.WebModelerMailSpec{
				CredentialsSecretRef: &v1.CredentialsSecretRef{Name: "smtp", Namespace: "mail"},
			}},
		},
	}

	assert.Equal(
		t,
		[]string{"identity/keycloak-admin", "secrets/admin-password", "mail/smtp"},
		secretRefs(mc),
	)
	assert.Empty(t, secretRefs(managementClusterWithSelector("bare", nil)))
}

// managementClusterWithSelector returns a management cluster whose
// spec.clusterSelector is the given selector.
func managementClusterWithSelector(
	name string,
	selector *metav1.LabelSelector,
) *v1.CamundaManagementCluster {
	return &v1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: enqueueNamespace},
		Spec:       v1.CamundaManagementClusterSpec{ClusterSelector: selector},
	}
}

// orchestrationCluster returns a CamundaCluster with the given labels, held by
// the management cluster that holder names.
func orchestrationCluster(clusterLabels map[string]string, holder string) *v1.CamundaCluster {
	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "orchestration",
			Namespace: "clusters",
			Labels:    clusterLabels,
		},
	}
	if holder != "" {
		cluster.Annotations = map[string]string{components.ClaimAnnotation: holder}
	}

	return cluster
}

// enqueued drives one create event through the handler and returns the
// requests it queued, ordered by name.
func enqueued(t *testing.T, handle handler.EventHandler, cluster *v1.CamundaCluster) []reconcile.Request {
	t.Helper()

	queue := workqueue.NewTypedRateLimitingQueue(
		workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
	)
	handle.Create(context.Background(), event.CreateEvent{Object: cluster}, queue)

	var requests []reconcile.Request
	for queue.Len() > 0 {
		req, _ := queue.Get()
		requests = append(requests, req)
		queue.Done(req)
	}
	slices.SortFunc(requests, func(a, b reconcile.Request) int {
		return strings.Compare(a.Name, b.Name)
	})

	return requests
}

// request is the reconcile request of one management cluster of these tests.
func request(name string) reconcile.Request {
	return reconcile.Request{
		NamespacedName: types.NamespacedName{Namespace: enqueueNamespace, Name: name},
	}
}
