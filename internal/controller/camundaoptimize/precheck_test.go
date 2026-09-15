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

package camundaoptimize

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	clustercomponents "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// The importer writes the analytics indices of the backend of its cluster. A
// cluster that does not hold the storage claim of that backend is parked or
// mid-handover, and its Ready still carries the last state either way, so the
// claim is what decides whether the importer may run.
func TestPreCheckSuspendsWhileTheClusterDoesNotHoldItsBackend(t *testing.T) {
	const (
		namespace  = gateNamespace
		claimSpace = "camunda-system"
		endpoint   = gateEndpoint
	)

	cases := map[string]struct {
		holderUID types.UID
		suspended bool
	}{
		"the cluster holds the claim":     {holderUID: "cluster-uid", suspended: false},
		"another cluster holds the claim": {holderUID: "other-uid", suspended: true},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			scheme, cluster, objects := storageClaimGateFixture(t)
			holder := cluster.DeepCopy()
			holder.UID = tc.holderUID
			key, err := clustercomponents.StorageClaimKey(clustercomponents.Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{Endpoint: endpoint},
			})
			require.NoError(t, err)
			objects = append(objects, clustercomponents.StorageClaimSchema().NewLease(claimSpace, key, holder))

			c := storageClaimPodClient(t, scheme, objects...)
			r := &Reconciler{
				Client:         c,
				APIReader:      c,
				Scheme:         scheme,
				ClaimNamespace: claimSpace,
			}

			var optimize v1.CamundaOptimize
			require.NoError(t, c.Get(
				context.Background(), client.ObjectKey{Namespace: namespace, Name: "my-optimize"}, &optimize,
			))

			out, err := r.preCheck(context.Background(), &optimize)

			require.NoError(t, err)
			assert.Equal(
				t, clustercomponents.StorageClaimSchema().LeaseName(key), out.Input.StorageClaim,
				"the claim of the backend is what the pods carry, so the gate ran",
			)
			assert.Equal(t, tc.suspended, out.Input.Suspended)
			assert.Equal(
				t, tc.suspended, out.AwaitsBackendClaim,
				"the Lease can come back without a status change of the cluster, so the reconcile requeues",
			)
		})
	}
}

// The cluster takes the claim of a new backend and records the handover it
// must wait for in one pass, and its Ready reaches the API server at the end
// of that pass. This controller can read the claim in between, so the pods on
// the backend decide as well: one pod of another cluster keeps the importer
// off the indices of that cluster.
func TestPreCheckSuspendsWhileAnotherClusterWritesTheBackend(t *testing.T) {
	const (
		namespace  = gateNamespace
		claimSpace = "camunda-system"
		endpoint   = gateEndpoint
	)

	cases := map[string]struct {
		podUID    types.UID
		podSpace  string
		suspended bool
	}{
		"a pod of the cluster itself":                   {podUID: "cluster-uid", podSpace: namespace},
		"a pod of another cluster":                      {podUID: "other-uid", podSpace: namespace, suspended: true},
		"a pod of another cluster in another namespace": {podUID: "other-uid", podSpace: "team-b", suspended: true},
	}

	// The importer of this instance on that backend says the gate passed
	// before, so the pods of every namespace stay unread. A webapp pod says
	// less: it can be up while the importer is still pending. The importer of
	// a deleted instance of the same cluster carries the cluster UID too and
	// may still be stopping, so the instance UID tells it apart and the gate
	// waits for it.
	fastPath := map[string]struct {
		component string
		instance  types.UID
		suspended bool
	}{
		"an importer pod of this instance": {component: components.ComponentImporter, instance: "optimize-uid"},
		"a webapp pod of this instance": {
			component: components.ComponentWebapp, instance: "optimize-uid", suspended: true,
		},
		"an importer pod of a deleted instance": {
			component: components.ComponentImporter, instance: "gone-uid", suspended: true,
		},
		"a webapp pod of a deleted instance": {
			component: components.ComponentWebapp, instance: "gone-uid", suspended: true,
		},
	}

	for name, tc := range fastPath {
		t.Run(name, func(t *testing.T) {
			scheme, cluster, objects := storageClaimGateFixture(t)
			key, err := clustercomponents.StorageClaimKey(clustercomponents.Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{Endpoint: endpoint},
			})
			require.NoError(t, err)
			claim := clustercomponents.StorageClaimSchema().LeaseName(key)
			objects = append(
				objects,
				clustercomponents.StorageClaimSchema().NewLease(claimSpace, key, cluster),
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      "own-" + tc.component,
					Labels: labels.Merge(
						clustercomponents.StoragePodLabels("holder", cluster.UID, claim),
						map[string]string{
							labels.ComponentKey:   tc.component,
							labels.OptimizeUIDKey: string(tc.instance),
						},
					),
				}},
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
					Namespace: namespace,
					Name:      "other-zeebe-0",
					Labels:    clustercomponents.StoragePodLabels("other", "other-uid", claim),
				}},
			)

			c := storageClaimPodClient(t, scheme, objects...)
			r := &Reconciler{Client: c, APIReader: c, Scheme: scheme, ClaimNamespace: claimSpace}

			var optimize v1.CamundaOptimize
			require.NoError(t, c.Get(
				context.Background(), client.ObjectKey{Namespace: namespace, Name: "my-optimize"}, &optimize,
			))

			out, err := r.preCheck(context.Background(), &optimize)

			require.NoError(t, err)
			assert.Equal(t, tc.suspended, out.Input.Suspended)
			assert.Equal(t, tc.suspended, out.AwaitsBackendClaim)
		})
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			scheme, cluster, objects := storageClaimGateFixture(t)
			key, err := clustercomponents.StorageClaimKey(clustercomponents.Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{Endpoint: endpoint},
			})
			require.NoError(t, err)
			claim := clustercomponents.StorageClaimSchema().LeaseName(key)
			objects = append(
				objects,
				clustercomponents.StorageClaimSchema().NewLease(claimSpace, key, cluster),
				&corev1.Pod{ObjectMeta: metav1.ObjectMeta{
					Namespace: tc.podSpace,
					Name:      "zeebe-0",
					Labels:    clustercomponents.StoragePodLabels("holder", tc.podUID, claim),
				}},
			)

			c := storageClaimPodClient(t, scheme, objects...)
			r := &Reconciler{Client: c, APIReader: c, Scheme: scheme, ClaimNamespace: claimSpace}

			var optimize v1.CamundaOptimize
			require.NoError(t, c.Get(
				context.Background(), client.ObjectKey{Namespace: namespace, Name: "my-optimize"}, &optimize,
			))

			out, err := r.preCheck(context.Background(), &optimize)

			require.NoError(t, err)
			assert.Equal(t, claim, out.Input.StorageClaim, "the gate ran")
			assert.Equal(t, tc.suspended, out.Input.Suspended)
			assert.Equal(
				t, tc.suspended, out.AwaitsBackendClaim,
				"nothing wakes this instance when those pods go, so the reconcile requeues on its timer",
			)
		})
	}
}

// A cluster that reports itself suspended keeps the workloads at zero, so the
// claim cannot change the answer and is not read. The fixture writes no Lease:
// a read would find none, park the workloads for that reason, and ask for a
// pass on the timer that this instance does not need.
func TestPreCheckLeavesTheClaimUnreadForASuspendedCluster(t *testing.T) {
	const (
		namespace = gateNamespace
		endpoint  = gateEndpoint
	)

	scheme, cluster, objects := storageClaimGateFixture(t)
	cluster.Spec.Suspend = true
	c := storageClaimPodClient(t, scheme, objects...)
	r := &Reconciler{Client: c, APIReader: c, Scheme: scheme, ClaimNamespace: "camunda-system"}

	var optimize v1.CamundaOptimize
	require.NoError(t, c.Get(
		context.Background(), client.ObjectKey{Namespace: namespace, Name: "my-optimize"}, &optimize,
	))

	out, err := r.preCheck(context.Background(), &optimize)

	require.NoError(t, err)
	assert.True(t, out.Input.Suspended, "the cluster is suspended, so the workloads are at zero")
	assert.False(t, out.AwaitsBackendClaim, "no claim was read, so nothing waits on one")
}

// storageClaimPodClient builds a fake client for the handover gate. The fake
// client refuses a "!=" field selector, which the API server serves, so the
// interceptor asserts that the phase selector reached it and then drops it. An
// envtest spec covers the filtering itself.
func storageClaimPodClient(t *testing.T, scheme *runtime.Scheme, objects ...client.Object) client.Client {
	t.Helper()

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithInterceptorFuncs(interceptor.Funcs{
			List: func(
				ctx context.Context,
				c client.WithWatch,
				list client.ObjectList,
				opts ...client.ListOption,
			) error {
				if _, ours := list.(*metav1.PartialObjectMetadataList); !ours {
					return c.List(ctx, list, opts...)
				}

				kept := make([]client.ListOption, 0, len(opts))
				var phases string
				for _, opt := range opts {
					if selector, ok := opt.(client.MatchingFieldsSelector); ok {
						phases = selector.String()

						continue
					}
					kept = append(kept, opt)
				}
				assert.Equal(t, "status.phase!=Failed,status.phase!=Succeeded", phases)

				return c.List(ctx, list, kept...)
			},
		}).
		Build()
}

// The cluster of the gate fixtures, and the backend it resolves.
const (
	gateNamespace = "apps"
	gateEndpoint  = "https://es-http.apps.svc:9200"
)

// storageClaimGateFixture returns the scheme, the cluster, and every object
// that preCheck reads on the path to the storage claim gate: a healthy cluster
// on an Elasticsearch contract, the Optimize of that cluster, and the auth
// config and Secrets they name.
func storageClaimGateFixture(t *testing.T) (*runtime.Scheme, *v1.CamundaCluster, []client.Object) {
	t.Helper()

	const (
		namespace = gateNamespace
		endpoint  = gateEndpoint
	)

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "my-cluster", UID: "cluster-uid"},
		Spec: v1.CamundaClusterSpec{
			PlatformConfigRef: "my-platform-config",
			Version:           "8.9.4",
			StorageRef:        "my-storage-config",
		},
	}
	meta.SetStatusCondition(cluster.GetStatusConditions(), metav1.Condition{
		Type:   v1.ConditionReady,
		Status: metav1.ConditionTrue,
		Reason: v1.ReasonHealthy,
	})

	optimize := &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "my-optimize", UID: "optimize-uid"},
		Spec: v1.CamundaOptimizeSpec{
			Version:           "8.9.4",
			ManagementAuthRef: "my-auth",
			ClusterRef:        v1.ClusterRef{Name: cluster.Name},
		},
	}

	issuer := "https://identity.example.com/realms/camunda"
	auth := &v1.ManagementAuthConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-auth"},
		Spec: v1.ManagementAuthConfigSpec{
			BaseURL:   "http://identity." + namespace + ".svc:8080",
			IssuerURL: issuer,
			AuthURL:   issuer + "/protocol/openid-connect/auth",
			TokenURL:  issuer + "/protocol/openid-connect/token",
			JwksURL:   issuer + "/protocol/openid-connect/certs",
			ClientID:  "optimize",
			Audience:  "optimize-api",
			ClientSecretRef: v1.SecretKeyRef{
				Name: "my-auth-client", Namespace: namespace, Key: "client-secret",
			},
		},
	}

	secret := func(name string, data map[string][]byte) *corev1.Secret {
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
			Data:       data,
		}
	}
	binding := &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: "my-storage-config"},
		Spec: v1.SecondaryStorageConfigSpec{
			Type: v1.SecondaryStorageTypeElasticsearch,
			Elasticsearch: &v1.ElasticsearchStorage{
				Endpoint: endpoint,
				CredentialsSecretRef: v1.LocalCredentialsSecretRef{
					Name: "es-credentials", UsernameKey: "username", PasswordKey: "password",
				},
			},
		},
	}

	return scheme, cluster, []client.Object{
		cluster,
		optimize,
		auth,
		binding,
		&v1.CamundaPlatformConfig{ObjectMeta: metav1.ObjectMeta{Name: "my-platform-config"}},
		secret("my-auth-client", map[string][]byte{"client-secret": []byte("s3cret")}),
		secret("es-credentials", map[string][]byte{
			"username": []byte("camunda"), "password": []byte("es-password"),
		}),
	}
}
