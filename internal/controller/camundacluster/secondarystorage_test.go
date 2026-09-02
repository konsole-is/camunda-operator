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

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	optimizecomponents "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

// TestStaleHolder covers staleHolder against a fake client, table-driven over
// why a holder is stale or live. A holder in another namespace whose
// storageRef happens to equal the contract's name must not read as live: the
// contract it names is a different object.
func TestStaleHolder(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	contract := &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "storage-contract", Namespace: "team-a"},
	}

	cases := map[string]struct {
		objects []client.Object
		holder  secondarystorageconfig.Holder
		stale   bool
	}{
		"holder does not exist": {
			holder: secondarystorageconfig.Holder{
				Cluster: types.NamespacedName{Namespace: "team-a", Name: "ghost"},
				UID:     "uid-1",
			},
			stale: true,
		},
		"holder exists with another UID": {
			objects: []client.Object{&v1.CamundaCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "holder", Namespace: "team-a", UID: "uid-2"},
				Spec:       v1.CamundaClusterSpec{StorageRef: "storage-contract"},
			}},
			holder: secondarystorageconfig.Holder{
				Cluster: types.NamespacedName{Namespace: "team-a", Name: "holder"},
				UID:     "uid-1",
			},
			stale: true,
		},
		"holder's storageRef names another contract": {
			objects: []client.Object{&v1.CamundaCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "holder", Namespace: "team-a", UID: "uid-1"},
				Spec:       v1.CamundaClusterSpec{StorageRef: "another-contract"},
			}},
			holder: secondarystorageconfig.Holder{
				Cluster: types.NamespacedName{Namespace: "team-a", Name: "holder"},
				UID:     "uid-1",
			},
			stale: true,
		},
		"holder in another namespace whose storageRef equals the contract's name": {
			objects: []client.Object{&v1.CamundaCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "holder", Namespace: "other-ns", UID: "uid-1"},
				Spec:       v1.CamundaClusterSpec{StorageRef: "storage-contract"},
			}},
			holder: secondarystorageconfig.Holder{
				Cluster: types.NamespacedName{Namespace: "other-ns", Name: "holder"},
				UID:     "uid-1",
			},
			stale: true,
		},
		"holder with the same UID and storageRef": {
			objects: []client.Object{&v1.CamundaCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "holder", Namespace: "team-a", UID: "uid-1"},
				Spec:       v1.CamundaClusterSpec{StorageRef: "storage-contract"},
			}},
			holder: secondarystorageconfig.Holder{
				Cluster: types.NamespacedName{Namespace: "team-a", Name: "holder"},
				UID:     "uid-1",
			},
			stale: false,
		},
		"a paused holder that names another contract still holds": {
			objects: []client.Object{&v1.CamundaCluster{
				ObjectMeta: metav1.ObjectMeta{Name: "holder", Namespace: "team-a", UID: "uid-1"},
				Spec:       v1.CamundaClusterSpec{Pause: true, StorageRef: "another-contract"},
			}},
			holder: secondarystorageconfig.Holder{
				Cluster: types.NamespacedName{Namespace: "team-a", Name: "holder"},
				UID:     "uid-1",
			},
			stale: false,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			res := &resolver{
				reader:  fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.objects...).Build(),
				storage: contract,
			}
			stale, err := res.staleHolder(context.Background(), tc.holder)
			require.NoError(t, err)
			assert.Equal(t, tc.stale, stale)
		})
	}
}

// TestHolderPods covers holderPods against a fake client: only a pod that
// carries the storage labels of the holder and the contract counts, the
// importer of an Optimize attached to the holder among them, and a holder in
// another namespace has none, because a same-named contract there is another
// object.
func TestHolderPods(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	contract := &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "storage-contract", Namespace: "team-a"},
	}
	holder := secondarystorageconfig.Holder{
		Cluster: types.NamespacedName{Namespace: "team-a", Name: "holder"},
		UID:     "uid-1",
	}
	pod := func(namespace, name string, podLabels map[string]string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: podLabels}}
	}

	cases := map[string]struct {
		objects []client.Object
		holder  secondarystorageconfig.Holder
		pods    []string
	}{
		"no pods": {holder: holder},
		"pods of the holder on the contract, sorted": {
			objects: []client.Object{
				pod("team-a", "holder-zeebe-1", components.StoragePodLabels("holder", "storage-contract")),
				pod("team-a", "holder-zeebe-0", components.StoragePodLabels("holder", "storage-contract")),
			},
			holder: holder,
			pods:   []string{"holder-zeebe-0", "holder-zeebe-1"},
		},
		"the Optimize importer pod of the holder on the contract": {
			objects: []client.Object{
				pod("team-a", "holder-optimize-importer-0", labels.Merge(
					components.StoragePodLabels("holder", "storage-contract"),
					map[string]string{labels.ComponentKey: optimizecomponents.ComponentImporter},
				)),
			},
			holder: holder,
			pods:   []string{"holder-optimize-importer-0"},
		},
		"pods of the holder on another contract": {
			objects: []client.Object{
				pod("team-a", "holder-zeebe-0", components.StoragePodLabels("holder", "another-contract")),
			},
			holder: holder,
		},
		"pods of another cluster on the contract": {
			objects: []client.Object{
				pod("team-a", "other-zeebe-0", components.StoragePodLabels("other", "storage-contract")),
			},
			holder: holder,
		},
		"pods of a same-named holder in another namespace": {
			objects: []client.Object{
				pod("other-ns", "holder-zeebe-0", components.StoragePodLabels("holder", "storage-contract")),
			},
			holder: secondarystorageconfig.Holder{
				Cluster: types.NamespacedName{Namespace: "other-ns", Name: "holder"},
				UID:     "uid-1",
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			res := &resolver{
				reader:  fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.objects...).Build(),
				storage: contract,
			}
			pods, err := res.holderPods(context.Background(), tc.holder)
			require.NoError(t, err)
			assert.Equal(t, tc.pods, pods)
		})
	}
}

// TestStorageHeld covers the Ready condition storageHeld builds for a
// suspended cluster, with and without an apply error, and that the result
// keeps CamundaCluster.Suspended true either way.
func TestStorageHeld(t *testing.T) {
	holder := &components.StorageHolder{
		Cluster:  types.NamespacedName{Namespace: "ns", Name: "holder"},
		Contract: types.NamespacedName{Namespace: "ns", Name: "storage"},
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
		assert.Contains(t, cond.Message, "ns/storage")
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

// expectClaimedBy polls until the contract carries the claim of cluster.
func expectClaimedBy(binding *v1.SecondaryStorageConfig, cluster *v1.CamundaCluster) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.SecondaryStorageConfig
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(binding), &latest)).To(Succeed())
		holder, held := secondarystorageconfig.HolderOf(&latest)
		g.Expect(held).To(BeTrue())
		g.Expect(holder.Cluster).To(Equal(client.ObjectKeyFromObject(cluster)))
		g.Expect(holder.UID).To(Equal(cluster.UID))
	}, timeout, interval).Should(Succeed())
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
// on binding, the way a rendered pod of that cluster does. envtest runs no
// kubelet, so a pod of the holder must be made by hand. It is deleted with
// the namespace.
func createStoragePod(cluster *v1.CamundaCluster, binding *v1.SecondaryStorageConfig) *corev1.Pod {
	GinkgoHelper()
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + "-zeebe-0",
			Namespace: cluster.Namespace,
			Labels:    components.StoragePodLabels(cluster.Name, binding.Name),
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "camunda", Image: "camunda/camunda:8.9.0"}}},
	}
	Expect(k8sClient.Create(ctx, pod)).To(Succeed())
	return pod
}

// expectWaitingForHandover polls until cluster reports WaitingForHandover
// naming previous and pod, and checks that its broker StatefulSet stays at
// zero replicas meanwhile.
func expectWaitingForHandover(cluster, previous *v1.CamundaCluster, pod *corev1.Pod) {
	GinkgoHelper()
	expectReady(
		cluster,
		metav1.ConditionFalse,
		Equal(v1.ReasonWaitingForHandover),
		And(ContainSubstring(previous.Namespace+"/"+previous.Name), ContainSubstring(pod.Name)),
	)
	zeebeKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"}
	Consistently(func(g Gomega) {
		g.Expect(*fetchStatefulSet(zeebeKey).Spec.Replicas).To(BeZero())
	}, "2s", interval).Should(Succeed(), "a cluster that waits for the handover renders nothing")
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
		expectWaitingForHandover(parked, holder, pod)
		expectClaimedBy(binding, holder)

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
		expectWaitingForHandover(parked, holder, pod)
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

	// A claim whose holder never existed, or was deleted before the operator
	// ran, must not park every later cluster forever.
	It("takes over a claim whose holder does not exist", func() {
		ns := newNamespace()
		binding := fixtures.SecondaryStorageConfigElasticsearch(ns)
		binding.Annotations = map[string]string{
			secondarystorageconfig.ClaimHolderAnnotation:    ns + "/ghost",
			secondarystorageconfig.ClaimHolderUIDAnnotation: "ghost-uid",
		}
		Expect(k8sClient.Create(ctx, binding)).To(Succeed())
		createSecret(ns, binding.Spec.Elasticsearch.CredentialsSecretRef.Name, map[string]string{
			"username": "camunda", "password": "es-password",
		})

		cluster := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(cluster)

		expectHolds(cluster)
		expectClaimedBy(binding, cluster)
	})

	// The producer of a contract applies it with server-side apply and never
	// names the claim annotations, so its field manager must not take them
	// away.
	It("keeps a claim through an apply of the contract by its producer", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(holder)
		expectClaimedBy(binding, holder)

		desired := &v1.SecondaryStorageConfig{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "SecondaryStorageConfig"},
			ObjectMeta: metav1.ObjectMeta{Name: binding.Name, Namespace: binding.Namespace},
			Spec:       *binding.Spec.DeepCopy(),
		}
		Expect(k8sClient.Patch(
			ctx,
			desired,
			//nolint:staticcheck // the producer applies the contract with the deprecated client.Apply patch, as ocf does
			client.Apply,
			client.FieldOwner("elasticsearchcluster"),
			client.ForceOwnership,
		)).To(Succeed())

		var latest v1.SecondaryStorageConfig
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(binding), &latest)).To(Succeed())
		Expect(latest.Annotations[secondarystorageconfig.ClaimHolderAnnotation]).
			To(Equal(holder.Namespace + "/" + holder.Name))
		Expect(latest.Annotations[secondarystorageconfig.ClaimHolderUIDAnnotation]).To(Equal(string(holder.UID)))
		expectHolds(holder)
	})
})
