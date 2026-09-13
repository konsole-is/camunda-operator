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
	"errors"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	optimizecomponents "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// TestOtherPodsOnClaim covers otherPodsOnClaim against a fake client: every
// pod that carries the storage claim and another cluster UID counts, whatever
// its namespace, the importer of an Optimize attached to such a cluster among
// them.
func TestOtherPodsOnClaim(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	const claim = "camunda-storage-0123456789abcdef0123456789abcdef01234567"
	self := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "holder", UID: "uid-1"},
	}
	pod := func(namespace, name string, podLabels map[string]string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: podLabels}}
	}

	cases := map[string]struct {
		objects []client.Object
		pods    []string
	}{
		"no pods": {},
		"pods of this cluster": {
			objects: []client.Object{
				pod("team-a", "holder-zeebe-0", components.StoragePodLabels("holder", "uid-1", claim)),
			},
		},
		"pods of a previous holder on the claim, sorted": {
			objects: []client.Object{
				pod("team-a", "old-zeebe-1", components.StoragePodLabels("old", "uid-old", claim)),
				pod("team-a", "old-zeebe-0", components.StoragePodLabels("old", "uid-old", claim)),
			},
			pods: []string{"team-a/old-zeebe-0", "team-a/old-zeebe-1"},
		},
		"the Optimize importer pod of a previous holder": {
			objects: []client.Object{
				pod("team-a", "old-optimize-importer-0", labels.Merge(
					components.StoragePodLabels("old", "uid-old", claim),
					map[string]string{labels.ComponentKey: optimizecomponents.ComponentImporter},
				)),
			},
			pods: []string{"team-a/old-optimize-importer-0"},
		},
		"pods of a same-named earlier cluster": {
			objects: []client.Object{
				pod("team-a", "holder-zeebe-0", components.StoragePodLabels("holder", "uid-0", claim)),
			},
			pods: []string{"team-a/holder-zeebe-0"},
		},
		"pods on another claim": {
			objects: []client.Object{
				pod("team-a", "old-zeebe-0", components.StoragePodLabels("old", "uid-old", "camunda-storage-other")),
			},
		},
		// Two clusters of two namespaces meet on one Lease, so a holder
		// elsewhere leaves pods that this cluster must wait for.
		"pods of a previous holder in another namespace": {
			objects: []client.Object{
				pod("team-b", "old-zeebe-0", components.StoragePodLabels("old", "uid-old", claim)),
			},
			pods: []string{"team-b/old-zeebe-0"},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			res := &resolver{
				reader:  storageClaimPodClient(t, scheme, tc.objects...),
				cluster: self,
			}
			pods, err := res.otherPodsOnClaim(context.Background(), claim)
			require.NoError(t, err)
			assert.Equal(t, tc.pods, pods)
		})
	}
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

// A Lease that carries the name of a storage claim and no holder annotations
// is somebody else's. No cluster holds the backend, so the failure is a report
// and not a suspension. Nothing watches that Lease for this cluster, so the
// failure must be unwatched: its deletion is picked up by the retry timer.
func TestClaimStorageReportsAForeignLeaseAsUnwatched(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	in := &components.Input{Storage: components.Storage{
		Type:          v1.SecondaryStorageTypeElasticsearch,
		Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "https://es.data.svc:9200"},
	}}
	key, err := components.StorageClaimKey(in.Storage)
	require.NoError(t, err)
	foreign := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{
		Namespace: "camunda-system",
		Name:      components.StorageClaimSchema().LeaseName(key),
	}}
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(foreign).Build()

	res := &resolver{
		reader:  c,
		claims:  components.StorageClaimSchema().NewClaim(c, c, "camunda-system"),
		cluster: &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "orders", UID: "uid-1"}},
		storage: &v1.SecondaryStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: "storage"},
		},
	}

	err = res.claimStorage(context.Background(), in)

	var unwatched *conditions.UnwatchedPreCheckFailure
	require.ErrorAs(t, err, &unwatched)
	assert.Equal(t, v1.ReasonInvalidReference, unwatched.Failure.Reason)
	assert.Contains(t, unwatched.Failure.Message, foreign.Name)
	assert.Nil(t, in.Storage.Holder, "no cluster holds the backend, so the cluster is not suspended")
}

// TestStorageHeld covers the Ready condition storageHeld builds for a
// suspended cluster, with and without an apply error, and that the result
// keeps CamundaCluster.Suspended true either way.
func TestStorageHeld(t *testing.T) {
	holder := &components.StorageHolder{
		Cluster: types.NamespacedName{Namespace: "ns", Name: "holder"},
		Backend: "elasticsearch|https://es:9200",
	}
	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Generation: 3},
	}

	t.Run("without an apply error", func(t *testing.T) {
		cond := storageHeld(cluster, holder, nil)
		assert.Equal(t, metav1.ConditionFalse, cond.Status)
		assert.Equal(t, v1.ReasonStorageAlreadyAttached, cond.Reason)
		assert.Equal(t, cluster.Generation, cond.ObservedGeneration)
		assert.Contains(t, cond.Message, "ns/holder")
		assert.Contains(t, cond.Message, "elasticsearch|https://es:9200")
		assert.NotContains(t, cond.Message, "failed")
	})

	t.Run("with an apply error", func(t *testing.T) {
		cond := storageHeld(cluster, holder, errors.New("apply exploded"))
		assert.Equal(t, metav1.ConditionFalse, cond.Status)
		assert.Equal(t, v1.ReasonStorageAlreadyAttached, cond.Reason)
		assert.Contains(t, cond.Message, "The last apply of the suspended workloads failed: apply exploded")
	})

	t.Run("the gate", func(t *testing.T) {
		for _, applyErr := range []error{nil, errors.New("apply exploded")} {
			gated := &v1.CamundaCluster{
				ObjectMeta: metav1.ObjectMeta{Generation: 3},
				Spec:       v1.CamundaClusterSpec{Suspend: false},
			}
			cond := storageHeld(gated, holder, applyErr)
			meta.SetStatusCondition(&gated.Status.Conditions, cond)
			assert.True(t, gated.Suspended())
		}
	})
}

// TestStorageHandover covers the Ready condition storageHandover builds for a
// cluster that holds the claim and waits for the pods of another cluster, with
// and without an apply error, and that the result keeps
// CamundaCluster.Suspended true either way.
func TestStorageHandover(t *testing.T) {
	handover := &components.StorageHandover{
		Backend: "elasticsearch|https://es:9200",
		Pods:    []string{"ns/old-zeebe-0", "ns/old-zeebe-1"},
	}
	cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Generation: 3}}

	t.Run("without an apply error", func(t *testing.T) {
		cond := storageHandover(cluster, handover, nil)
		assert.Equal(t, metav1.ConditionFalse, cond.Status)
		assert.Equal(t, v1.ReasonWaitingForHandover, cond.Reason)
		assert.Equal(t, cluster.Generation, cond.ObservedGeneration)
		assert.Contains(t, cond.Message, "elasticsearch|https://es:9200")
		assert.Contains(t, cond.Message, "ns/old-zeebe-0, ns/old-zeebe-1")
		assert.NotContains(t, cond.Message, "failed")
	})

	t.Run("with an apply error", func(t *testing.T) {
		cond := storageHandover(cluster, handover, errors.New("apply exploded"))
		assert.Equal(t, v1.ReasonWaitingForHandover, cond.Reason)
		assert.Contains(t, cond.Message, "The last apply of the suspended workloads failed: apply exploded")
	})

	t.Run("the gate", func(t *testing.T) {
		for _, applyErr := range []error{nil, errors.New("apply exploded")} {
			gated := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Generation: 3}}
			cond := storageHandover(gated, handover, applyErr)
			meta.SetStatusCondition(&gated.Status.Conditions, cond)
			assert.True(t, gated.Suspended())
		}
	})
}

// newNamedCluster is newCluster with a name prefix, so a spec that creates
// two clusters on one contract can tell them apart in the output.
func newNamedCluster(
	prefix, namespace string,
	cfg *v1.CamundaPlatformConfig,
	binding *v1.SecondaryStorageConfig,
) *v1.CamundaCluster {
	cluster := newCluster(namespace, cfg, binding)
	cluster.Name = prefix + utilrand.String(8)
	return cluster
}

// expectClaimedBy polls until the storage claim of the backend that binding
// names records cluster.
func expectClaimedBy(binding *v1.SecondaryStorageConfig, cluster *v1.CamundaCluster) {
	GinkgoHelper()
	key := storageKeyOf(binding)
	Eventually(func(g Gomega) {
		var lease coordinationv1.Lease
		name := client.ObjectKey{
			Namespace: testClaimNamespace,
			Name:      components.StorageClaimSchema().LeaseName(key),
		}
		g.Expect(k8sClient.Get(ctx, name, &lease)).To(Succeed())
		holder, ours := components.StorageClaimSchema().HolderOf(&lease)
		g.Expect(ours).To(BeTrue())
		g.Expect(holder.NamespacedName).To(Equal(client.ObjectKeyFromObject(cluster)))
		g.Expect(holder.UID).To(Equal(cluster.UID))
	}, timeout, interval).Should(Succeed())
}

// storageKeyOf returns the claim key of the backend that binding names. The
// test bindings are Elasticsearch ones.
func storageKeyOf(binding *v1.SecondaryStorageConfig) string {
	GinkgoHelper()
	key, err := components.StorageClaimKey(components.Storage{
		Type:          binding.Spec.Type,
		Elasticsearch: binding.Spec.Elasticsearch,
	})
	Expect(err).NotTo(HaveOccurred())

	return key
}

// expectParked polls until cluster reports StorageAlreadyAttached naming
// holder, and its broker StatefulSet exists with zero replicas.
func expectParked(cluster, holder *v1.CamundaCluster) {
	GinkgoHelper()
	expectReady(
		cluster,
		metav1.ConditionFalse,
		Equal(v1.ReasonStorageAlreadyAttached),
		ContainSubstring(holder.Namespace+"/"+holder.Name),
	)
	zeebeKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"}
	Eventually(func(g Gomega) {
		g.Expect(*fetchStatefulSet(zeebeKey).Spec.Replicas).To(BeZero())
	}, timeout, interval).Should(Succeed())
}

// createStoragePod creates a pod that carries the storage labels of cluster
// on the backend of binding, the way a rendered pod of that cluster does.
// envtest runs no kubelet, so a pod of the holder must be made by hand. It is
// deleted with the namespace.
func createStoragePod(cluster *v1.CamundaCluster, binding *v1.SecondaryStorageConfig) *corev1.Pod {
	GinkgoHelper()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + "-zeebe-0",
			Namespace: cluster.Namespace,
			Labels: components.StoragePodLabels(
				cluster.Name,
				cluster.UID,
				components.StorageClaimSchema().LeaseName(storageKeyOf(binding)),
			),
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "camunda", Image: "camunda/camunda:8.9.0"}}},
	}
	Expect(k8sClient.Create(ctx, pod)).To(Succeed())
	return pod
}

// expectWaitingForHandover polls until cluster reports WaitingForHandover
// naming the backend of binding and pod, and checks that its broker
// StatefulSet stays at zero replicas meanwhile.
func expectWaitingForHandover(cluster *v1.CamundaCluster, binding *v1.SecondaryStorageConfig, pod *corev1.Pod) {
	GinkgoHelper()
	expectReady(
		cluster,
		metav1.ConditionFalse,
		Equal(v1.ReasonWaitingForHandover),
		And(ContainSubstring(storageKeyOf(binding)), ContainSubstring(pod.Name)),
	)
	zeebeKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"}
	Consistently(func(g Gomega) {
		g.Expect(*fetchStatefulSet(zeebeKey).Spec.Replicas).To(BeZero())
	}, "2s", interval).Should(Succeed(), "a cluster that waits for the handover renders nothing")
}

// createBindingAt creates an Elasticsearch contract in namespace that names
// endpoint, so two contracts can name one backend. createBinding gives every
// contract an endpoint of its own.
func createBindingAt(namespace, endpoint string) *v1.SecondaryStorageConfig {
	GinkgoHelper()
	binding := fixtures.SecondaryStorageConfigElasticsearch(namespace)
	binding.Spec.Elasticsearch.Endpoint = endpoint
	Expect(k8sClient.Create(ctx, binding)).To(Succeed())
	createSecret(namespace, binding.Spec.Elasticsearch.CredentialsSecretRef.Name, map[string]string{
		"username": "camunda", "password": "es-password",
	})

	return binding
}

// createStorageLeaseFor writes a storage claim Lease that records holder, the
// way a cluster that is gone leaves one behind.
func createStorageLeaseFor(key string, holder *v1.CamundaCluster) {
	GinkgoHelper()
	Expect(k8sClient.Create(
		ctx, components.StorageClaimSchema().NewLease(testClaimNamespace, key, holder),
	)).To(Succeed())
}

// expectHolds polls until cluster reports a Ready reason other than
// StorageAlreadyAttached, and its broker StatefulSet asks for one replica.
func expectHolds(cluster *v1.CamundaCluster) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Reason).NotTo(Equal(v1.ReasonStorageAlreadyAttached))
	}, timeout, interval).Should(Succeed())
	zeebeKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"}
	Eventually(func(g Gomega) {
		g.Expect(*fetchStatefulSet(zeebeKey).Spec.Replicas).To(Equal(int32(1)))
	}, timeout, interval).Should(Succeed())
}

var _ = Describe("CamundaCluster secondary storage contract", func() {
	// One CamundaCluster uses one contract, so the first cluster that claims
	// it holds it and the other is suspended, with its volumes, until the
	// holder releases it.
	It("lets the first cluster hold a contract and suspends the second", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)

		first := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(first)
		expectClaimedBy(binding, first)

		second := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(second)

		expectParked(second, first)
		expectReady(
			second,
			metav1.ConditionFalse,
			Equal(v1.ReasonStorageAlreadyAttached),
			ContainSubstring(binding.Name),
		)
		expectHolds(first)
	})

	// Two contracts that name one address are one backend. The second cluster
	// parks on the claim of the first.
	It("parks the second cluster when two contracts name one address", func() {
		ns := newNamespace()
		first := newNamedCluster(
			"cc-a-", ns, createPlatformConfig(), createBindingAt(ns, "https://shared.es:9200"),
		)
		createCluster(first)
		expectHolds(first)

		binding := createBindingAt(ns, "https://SHARED.es:9200/")
		second := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(second)
		expectParked(second, first)
		expectReady(
			second,
			metav1.ConditionFalse,
			Equal(v1.ReasonStorageAlreadyAttached),
			ContainSubstring("elasticsearch|https://shared.es:9200"),
		)
	})

	It("lets two clusters on two contracts both run", func() {
		ns := newNamespace()
		first := newNamedCluster("cc-a-", ns, createPlatformConfig(), createBinding(ns, true))
		createCluster(first)
		second := newNamedCluster("cc-b-", ns, createPlatformConfig(), createBinding(ns, true))
		createCluster(second)

		expectHolds(first)
		expectHolds(second)
	})

	// Nothing watches the holder for the parked cluster, so the takeover
	// happens on the retry timer.
	It("resumes the parked cluster when the holder is deleted", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		Expect(k8sClient.Create(ctx, holder)).To(Succeed())
		expectClaimedBy(binding, holder)

		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)
		expectParked(parked, holder)

		Expect(k8sClient.Delete(ctx, holder)).To(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(holder), &v1.CamundaCluster{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		expectHolds(parked)
		expectClaimedBy(binding, parked)
	})

	// The pods of a deleted holder go through garbage collection after the
	// cluster is gone. The parked cluster waits for them, so the two never
	// write the backend at once.
	It("waits for the pods of a deleted holder before it resumes", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		Expect(k8sClient.Create(ctx, holder)).To(Succeed())
		expectClaimedBy(binding, holder)
		pod := createStoragePod(holder, binding)

		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)
		expectParked(parked, holder)

		Expect(k8sClient.Delete(ctx, holder)).To(Succeed())
		// The claim of a holder that is gone is taken over at once. The pods
		// it left behind hold the render, not the claim.
		expectClaimedBy(binding, parked)
		expectWaitingForHandover(parked, binding, pod)

		Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
		expectHolds(parked)
		expectClaimedBy(binding, parked)
	})

	// The pods of a repointed holder write the old backend until its rollout
	// replaces them. The parked cluster waits for them.
	It("waits for the pods of a repointed holder before it resumes", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(holder)
		expectClaimedBy(binding, holder)
		pod := createStoragePod(holder, binding)

		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)
		expectParked(parked, holder)

		other := createBinding(ns, true)
		updateCluster(holder, func(c *v1.CamundaCluster) { c.Spec.StorageRef = other.Name })
		expectWaitingForHandover(parked, binding, pod)
		expectHolds(holder)
		expectClaimedBy(other, holder)

		Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
		expectHolds(parked)
		expectClaimedBy(binding, parked)
	})

	// A holder that names another contract releases this one, so the parked
	// cluster takes the claim over and both clusters run.
	It("resumes the parked cluster when the holder repoints its storageRef", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(holder)
		expectClaimedBy(binding, holder)

		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)
		expectParked(parked, holder)

		other := createBinding(ns, true)
		updateCluster(holder, func(c *v1.CamundaCluster) { c.Spec.StorageRef = other.Name })

		expectHolds(parked)
		expectClaimedBy(binding, parked)
		expectHolds(holder)
		expectClaimedBy(other, holder)
	})

	// One edit that repoints a cluster to a held contract and lowers its
	// version parks it on the version it runs. The refusal of the downgrade
	// waits until the contract is released, so the parking never applies the
	// lower image.
	It("parks a repointed cluster whose version is also refused, and refuses it again once released", func() {
		ns := newNamespace()
		own := createBinding(ns, true)
		repointed := newNamedCluster("cc-a-", ns, createPlatformConfig(), own)
		createCluster(repointed)
		expectClaimedBy(own, repointed)
		Expect(zeebeContainer(repointed).Image).To(HaveSuffix(":" + runningVersion))

		held := createBinding(ns, true)
		holder := newNamedCluster("cc-b-", ns, createPlatformConfig(), held)
		createCluster(holder)
		expectClaimedBy(held, holder)

		By("repointing to the held contract and lowering the version in one edit")
		updateCluster(repointed, func(c *v1.CamundaCluster) {
			c.Spec.StorageRef = held.Name
			c.Spec.Version = lowerVersion
		})
		expectParked(repointed, holder)
		Consistently(func() string { return zeebeContainer(repointed).Image }, "2s", interval).Should(
			HaveSuffix(":"+runningVersion),
			"the parking keeps the running image. A lower image means the parking applied it",
		)
		Consistently(func(g Gomega) {
			g.Expect(countEvents(g, repointed, v1.ReasonVersionDowngradeRefused)).To(BeZero())
		}, "2s", interval).Should(Succeed(), "a parked cluster does not report the refusal yet")

		By("releasing the contract")
		Expect(k8sClient.Delete(ctx, holder)).To(Succeed())
		expectReady(
			repointed, metav1.ConditionFalse,
			Equal(v1.ReasonVersionDowngradeRefused),
			ContainSubstring(lowerVersion+" is below the running version "+runningVersion),
		)
		expectEvent(repointed, v1.ReasonVersionDowngradeRefused, corev1.EventTypeWarning)
		Expect(zeebeContainer(repointed).Image).To(HaveSuffix(":" + runningVersion))

		By("setting the version forward again")
		updateCluster(repointed, func(c *v1.CamundaCluster) { c.Spec.Version = runningVersion })
		expectHolds(repointed)
		expectClaimedBy(held, repointed)
	})

	// Two contracts can name one backend, and the operator compares
	// contracts, not endpoints. When the holder of one contract loses it and
	// keeps running, both clusters write the backend. A cluster whose
	// pre-check fails therefore scales to zero, so the two never both run.
	It("scales a running cluster to zero when its contract goes away, while the other keeps running", func() {
		ns := newNamespace()
		first := newNamedCluster("cc-a-", ns, createPlatformConfig(), createBinding(ns, true))
		createCluster(first)
		binding := createBinding(ns, true)
		second := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(second)
		expectHolds(first)
		expectHolds(second)

		By("deleting the contract of the second cluster")
		Expect(k8sClient.Delete(ctx, binding)).To(Succeed())
		expectReady(
			second,
			metav1.ConditionFalse,
			Equal(v1.ReasonInvalidReference),
			And(
				ContainSubstring(binding.Name),
				ContainSubstring("The workloads are scaled to zero, with the volumes kept"),
			),
		)
		zeebeKey := client.ObjectKey{Namespace: ns, Name: second.Name + "-zeebe"}
		Eventually(func(g Gomega) {
			g.Expect(*fetchStatefulSet(zeebeKey).Spec.Replicas).To(BeZero())
		}, timeout, interval).Should(Succeed(), "the broker StatefulSet is scaled, not deleted")
		Eventually(func(g Gomega) {
			var latest v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(second), &latest)).To(Succeed())
			zeebe := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionZeebeReady)
			g.Expect(zeebe).NotTo(BeNil())
			g.Expect(zeebe.Reason).To(Equal("Suspended"))
		}, timeout, interval).Should(Succeed(), "the broker condition reports the suspension")
		expectHolds(first)

		By("recreating the contract")
		recreated := &v1.SecondaryStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: binding.Name, Namespace: ns},
			Spec:       *binding.Spec.DeepCopy(),
		}
		Expect(k8sClient.Create(ctx, recreated)).To(Succeed())
		expectHolds(second)
		expectClaimedBy(recreated, second)
	})

	// A running cluster that repoints into a backend whose previous holder
	// left pods behind must stop. It holds the claim of that backend, and its
	// own workloads still write the backend it left.
	It("scales a running cluster to zero while it waits for a handover", func() {
		ns := newNamespace()
		own := createBinding(ns, true)
		cluster := newNamedCluster("cc-a-", ns, createPlatformConfig(), own)
		createCluster(cluster)
		expectHolds(cluster)

		target := createBinding(ns, true)
		ghost := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "ghost", UID: "ghost-uid"},
		}
		createStorageLeaseFor(storageKeyOf(target), ghost)
		pod := createStoragePod(ghost, target)

		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.StorageRef = target.Name })
		expectWaitingForHandover(cluster, target, pod)

		Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
		expectHolds(cluster)
		expectClaimedBy(target, cluster)
	})

	// A repoint holds two claims for one pass. The old one must go, or the
	// next cluster on that backend parks on a claim nothing writes.
	It("releases the old backend when the holder repoints and the new one resolves", func() {
		ns := newNamespace()
		old := createBinding(ns, true)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), old)
		createCluster(holder)
		expectClaimedBy(old, holder)

		other := createBinding(ns, true)
		updateCluster(holder, func(c *v1.CamundaCluster) { c.Spec.StorageRef = other.Name })
		expectClaimedBy(other, holder)
		Eventually(func(g Gomega) {
			var lease coordinationv1.Lease
			name := client.ObjectKey{
				Namespace: testClaimNamespace,
				Name:      components.StorageClaimSchema().LeaseName(storageKeyOf(old)),
			}
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, name, &lease))).To(BeTrue())
		}, timeout, interval).Should(Succeed(), "the claim of the old backend is released")
	})

	// Two clusters that swap backends in one step meet each other's claim. A
	// parked cluster writes nothing, so it gives its previous backend back,
	// and the two never block each other for good.
	It("lets two clusters swap their backends in one step", func() {
		ns := newNamespace()
		first := createBinding(ns, true)
		second := createBinding(ns, true)
		a := newNamedCluster("cc-a-", ns, createPlatformConfig(), first)
		createCluster(a)
		b := newNamedCluster("cc-b-", ns, createPlatformConfig(), second)
		createCluster(b)
		expectClaimedBy(first, a)
		expectClaimedBy(second, b)

		updateCluster(a, func(c *v1.CamundaCluster) { c.Spec.StorageRef = second.Name })
		updateCluster(b, func(c *v1.CamundaCluster) { c.Spec.StorageRef = first.Name })

		expectClaimedBy(second, a)
		expectClaimedBy(first, b)
		expectHolds(a)
		expectHolds(b)
	})

	// An evicted pod of a previous holder keeps its object under a ReplicaSet
	// that nobody deleted. It writes nothing, so it must not hold the backend
	// of the next cluster for good.
	It("ignores a pod of a previous holder that ended", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		ghost := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "ghost", UID: "ghost-uid"},
		}
		createStorageLeaseFor(storageKeyOf(binding), ghost)
		pod := createStoragePod(ghost, binding)

		By("failing the pod the way an eviction does")
		pod.Status.Phase = corev1.PodFailed
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())

		cluster := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(cluster)

		expectHolds(cluster)
		expectClaimedBy(binding, cluster)
	})

	// A claim whose holder never existed, or was deleted before the operator
	// ran, must not park every later cluster forever.
	It("takes over a claim whose holder does not exist", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		ghost := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "ghost", UID: "ghost-uid"},
		}
		createStorageLeaseFor(storageKeyOf(binding), ghost)

		cluster := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(cluster)

		expectHolds(cluster)
		expectClaimedBy(binding, cluster)
	})

})
