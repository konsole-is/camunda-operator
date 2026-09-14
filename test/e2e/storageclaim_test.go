//go:build e2e
// +build e2e

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

package e2e

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	// scHolder takes the backend first, scWaiting waits for it. Both run in
	// the namespace of the ElasticsearchCluster flow, against the
	// Elasticsearch that flow provisions.
	scHolder  = "storage-claim-a"
	scWaiting = "storage-claim-b"
	// scContract is a second SecondaryStorageConfig on the endpoint that the
	// ElasticsearchCluster publishes. Two contracts on one address are what
	// the claim has to see through.
	scContract = "camunda-es-storage-shared"
	// scPlatform is the platform config of both clusters.
	scPlatform = "storage-claim-e2e"
	// scHandoverTimeout bounds the wait for the pods of the deleted holder.
	// They go under the default grace period of thirty seconds, and the
	// waiting cluster reports the wait for as long as they are there.
	scHandoverTimeout = 3 * time.Minute
	// scHoldInterval is how long a hold of the operator is sampled, and
	// scHoldSample is how often. A hold that the operator must keep reads the
	// same at every sample.
	scHoldInterval = 30 * time.Second
	scHoldSample   = 10 * time.Second
)

// itHandsTheStorageBackendOver runs the journey of the storage claim inside
// the ElasticsearchCluster flow: two CamundaClusters on two contracts that
// name one Elasticsearch. The first one writes it, the second one waits, and
// the backend moves to the second one when the first is deleted. It reuses
// the Elasticsearch of that flow, so it needs no backend of its own.
func itHandsTheStorageBackendOver() {
	It("holds the Elasticsearch backend for one cluster and hands it to the next", func() {
		By("reading the published contract and the backend it addresses")
		var published v1.SecondaryStorageConfig
		Expect(utils.Get(sscResource, esStorageConfig, esNamespace, &published)).To(Succeed())
		backend, err := components.StorageClaimKey(components.Storage{
			Type:          published.Spec.Type,
			Namespace:     published.Namespace,
			Elasticsearch: published.Spec.Elasticsearch,
		})
		Expect(err).NotTo(HaveOccurred())
		claim := components.StorageClaimSchema().LeaseName(backend)

		By("creating the platform config of both clusters")
		Expect(apply(basicPlatform(scPlatform))).To(Succeed())
		DeferCleanup(func() {
			_, _ = utils.Kubectl("delete", ccPlatformResource, scPlatform, "--ignore-not-found")
			_, _ = utils.Kubectl("delete", sscResource, scContract, "-n", esNamespace, "--ignore-not-found")
		})

		holder := newCluster(esNamespace, scPlatform, esStorageConfig, "", false)
		holder.Name = scHolder
		waiting := newCluster(esNamespace, scPlatform, scContract, "", false)
		waiting.Name = scWaiting
		// The one broker pod of the holder is ordinal zero of its StatefulSet.
		brokerPodOfHolder := components.WorkloadName(holder, components.ComponentZeebe) + "-0"
		brokerOfWaiting := components.WorkloadName(waiting, components.ComponentZeebe)
		gatewayOfWaiting := components.WorkloadName(waiting, components.ComponentGateway)

		// The spec that follows deletes the Elasticsearch these clusters
		// write, so neither of them may outlive this one. A cluster is gone
		// before its pods are: the garbage collector removes the workloads
		// after the finalizer cleared, so the cleanup waits for the pods too.
		DeferCleanup(func() {
			for _, name := range []string{scHolder, scWaiting} {
				_, _ = utils.Kubectl(
					"delete", ccResource, name, "-n", esNamespace, "--ignore-not-found", "--wait=false",
				)
			}
			Eventually(func(g Gomega) {
				expectGone(g, ccResource, scHolder, esNamespace)
				expectGone(g, ccResource, scWaiting, esNamespace)

				var pods corev1.PodList
				g.Expect(utils.List("pods", esNamespace, ownPodsSelector(scHolder, scWaiting), &pods)).To(Succeed())
				g.Expect(pods.Items).To(BeEmpty(), "a pod of a deleted cluster still runs")
			}, 5*time.Minute, 5*time.Second).Should(Succeed())
		})

		By("creating the first cluster and waiting for Ready Healthy")
		Expect(apply(holder)).To(Succeed())
		Eventually(func(g Gomega) {
			expectReady(g, ccResource, scHolder, esNamespace, v1.ReasonHealthy)
		}, ccReadyTimeout, 5*time.Second).Should(Succeed())

		By("creating a second contract on the same endpoint, and a cluster on it")
		Expect(apply(&v1.SecondaryStorageConfig{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "SecondaryStorageConfig"},
			ObjectMeta: metav1.ObjectMeta{Name: scContract, Namespace: esNamespace},
			Spec:       *published.Spec.DeepCopy(),
		})).To(Succeed())
		Expect(apply(waiting)).To(Succeed())

		By("parking the second cluster and naming the holder and the backend")
		Eventually(func(g Gomega) {
			expectReadyFailure(
				g, scWaiting, v1.ReasonStorageAlreadyAttached, esNamespace+"/"+scHolder, backend,
			)
		}, 5*time.Minute, 5*time.Second).Should(Succeed())

		By("keeping it at zero while the pods of the holder write the backend")
		Consistently(func(g Gomega) {
			expectReadyFailure(g, scWaiting, v1.ReasonStorageAlreadyAttached)
			expectScaledToZero(g, "statefulset", brokerOfWaiting, esNamespace)
			expectScaledToZero(g, "deployment", gatewayOfWaiting, esNamespace)
			expectReady(g, ccResource, scHolder, esNamespace, v1.ReasonHealthy)
		}, scHoldInterval, scHoldSample).Should(Succeed())

		By("deleting the holder")
		_, err = utils.Kubectl("delete", ccResource, scHolder, "-n", esNamespace, "--wait=false")
		Expect(err).NotTo(HaveOccurred())

		By("waiting for its pods to go, at zero and naming them")
		Eventually(func(g Gomega) {
			expectReadyFailure(
				g, scWaiting, v1.ReasonWaitingForHandover, esNamespace+"/"+brokerPodOfHolder, backend,
			)
			expectScaledToZero(g, "statefulset", brokerOfWaiting, esNamespace)
			expectScaledToZero(g, "deployment", gatewayOfWaiting, esNamespace)
		}, scHandoverTimeout, time.Second).Should(Succeed())

		By("starting the second cluster on the backend it took over")
		Eventually(func(g Gomega) {
			expectReady(g, ccResource, scWaiting, esNamespace, v1.ReasonHealthy)
		}, ccReadyTimeout, 5*time.Second).Should(Succeed())

		By("carrying the storage claim of the backend on its broker pods")
		var pods corev1.PodList
		Expect(utils.List("pods", esNamespace, brokerClaimSelector(waiting), &pods)).To(Succeed())
		Expect(pods.Items).NotTo(BeEmpty(), "the second cluster runs no broker pod")
		for i := range pods.Items {
			Expect(pods.Items[i].Labels).To(HaveKeyWithValue(labels.StorageClaimKey, claim))
		}

		By("recording it as the holder of the claim and leaving no Lease of the deleted cluster")
		Eventually(func(g Gomega) {
			expectGone(g, ccResource, scHolder, esNamespace)

			var lease coordinationv1.Lease
			g.Expect(utils.Get("lease", claim, namespace, &lease)).To(Succeed())
			g.Expect(lease.Annotations).To(HaveKeyWithValue(
				components.StorageClaimHolderNamespaceAnnotation, esNamespace,
			))
			g.Expect(lease.Annotations).To(HaveKeyWithValue(
				components.StorageClaimHolderNameAnnotation, scWaiting,
			))

			var leases coordinationv1.LeaseList
			g.Expect(utils.List(
				"leases", namespace,
				k8slabels.SelectorFromSet(components.StorageClaimLeaseLabels(scHolder)).String(),
				&leases,
			)).To(Succeed())
			g.Expect(leases.Items).To(BeEmpty(), "the deleted cluster left a storage claim Lease behind")
		}, 3*time.Minute, 5*time.Second).Should(Succeed())

		By("deleting the contract of the running cluster")
		_, err = utils.Kubectl("delete", sscResource, scContract, "-n", esNamespace, "--wait=false")
		Expect(err).NotTo(HaveOccurred())

		By("reporting the dangling reference and keeping the brokers")
		Eventually(func(g Gomega) {
			expectReadyFailure(g, scWaiting, v1.ReasonInvalidReference, esNamespace+"/"+scContract)
		}, 3*time.Minute, 5*time.Second).Should(Succeed())
		Consistently(func(g Gomega) {
			expectReadyFailure(g, scWaiting, v1.ReasonInvalidReference, esNamespace+"/"+scContract)
			expectBrokerReplicas(g, brokerOfWaiting, 1)
		}, scHoldInterval, scHoldSample).Should(Succeed())
	})
}

// expectReadyFailure asserts that the CamundaCluster name of the
// ElasticsearchCluster flow reports Ready False with reason, and that its
// message holds every part. It is written for Eventually and for Consistently.
func expectReadyFailure(g Gomega, name, reason string, parts ...string) {
	expectConditionFalse(g, ccResource, name, esNamespace, v1.ConditionReady, reason)

	cond, err := utils.Condition(ccResource, name, esNamespace, v1.ConditionReady)
	g.Expect(err).NotTo(HaveOccurred())
	g.Expect(cond).NotTo(BeNil(), "CamundaCluster %q reports no %s yet", name, v1.ConditionReady)
	for _, part := range parts {
		g.Expect(cond.Message).To(ContainSubstring(part), cond.Message)
	}
}

// expectBrokerReplicas asserts that the broker StatefulSet name of the
// ElasticsearchCluster flow asks for replicas and serves them. It reads the
// ready count, because the pod count also counts a pod that is pending or
// terminating. It is the read of a cluster that keeps its workloads, as
// expectScaledToZero is the read of one that stopped. It is written for
// Consistently.
func expectBrokerReplicas(g Gomega, name string, replicas int32) {
	var broker appsv1.StatefulSet
	g.Expect(utils.Get("statefulset", name, esNamespace, &broker)).To(Succeed())
	g.Expect(broker.Spec.Replicas).To(
		HaveValue(Equal(replicas)), "statefulset %q does not ask for %d replicas", name, replicas,
	)
	g.Expect(broker.Status.ReadyReplicas).To(
		Equal(replicas), "statefulset %q does not serve %d ready pods", name, replicas,
	)
}

// ownPodsSelector selects the pods that the named CamundaClusters own, by the
// cluster label every process pod carries.
func ownPodsSelector(clusters ...string) string {
	requirement, err := k8slabels.NewRequirement(labels.ClusterKey, selection.In, clusters)
	Expect(err).NotTo(HaveOccurred())

	return k8slabels.NewSelector().Add(*requirement).String()
}
