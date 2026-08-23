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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

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
	It("keeps the claim through an apply of the contract by its producer", func() {
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
			client.Apply, //nolint:staticcheck // ocf applies through the deprecated client.Apply patch
			client.FieldOwner("elasticsearchcluster"),
			client.ForceOwnership,
		)).To(Succeed())

		Consistently(func(g Gomega) {
			var latest v1.SecondaryStorageConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(binding), &latest)).To(Succeed())
			_, held := secondarystorageconfig.HolderOf(&latest)
			g.Expect(held).To(BeTrue())
		}, 3*time.Second, interval).Should(Succeed())
		expectHolds(holder)
	})
})
