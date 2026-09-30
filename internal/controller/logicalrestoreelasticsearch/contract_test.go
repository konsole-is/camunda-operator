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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	coordinationv1 "k8s.io/api/coordination/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/storagewriter"
)

var _ = Describe("LogicalRestoreElasticsearch hold on its contract", func() {
	It("pins its SecondaryStorageConfig and holds it at every endpoint", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)
		contract := contractOf(w.storage)
		var backend string
		Eventually(func(g Gomega) {
			current := latest(g, restore)
			g.Expect(current.Status.Backend).NotTo(BeEmpty())
			g.Expect(current.Status.Contract).To(Equal(contract))
			backend = current.Status.Backend
		}, timeout, interval).Should(Succeed())
		elsewhere := "elasticsearch|https://elsewhere.example.svc:9200"
		Expect(writersSeenByAnotherCluster(elsewhere, contract)).To(HaveLen(1))

		By("stripping the contract from its Lease")
		key := client.ObjectKey{Namespace: claimNamespace, Name: storagewriter.LeaseName(backend, restore.UID)}
		Eventually(func(g Gomega) {
			var lease coordinationv1.Lease
			g.Expect(k8sClient.Get(ctx, key, &lease)).To(Succeed())
			delete(lease.Annotations, storagewriter.ContractAnnotation)
			g.Expect(k8sClient.Update(ctx, &lease)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Eventually(func() []string {
			return writersSeenByAnotherCluster(elsewhere, contract)
		}, timeout, interval).Should(HaveLen(1), "a later pass restores the pinned contract")
	})

	// The restore does not follow a move of the endpoint: it fails. Its
	// contract holds the moved endpoint until it gives its backend back.
	It("holds the moved endpoint until it gives its backend back", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)
		contract := contractOf(w.storage)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.Contract).To(Equal(contract))
		}, timeout, interval).Should(Succeed())
		endpoint := "https://moved-es." + w.namespace + ".svc:9200"
		moved := keyAt(w, endpoint)
		Expect(writersSeenByAnotherCluster(moved, contract)).To(HaveLen(1))

		moveEndpoint(w, endpoint)
		// A restore gives its backend back only after its terminal phase is
		// written, so a read of the writers before a read of a running
		// restore must find the writer, whatever the timing.
		freedWhileRunning := false
		Eventually(func(g Gomega) {
			writers := writersSeenByAnotherCluster(moved, contract)
			current := latest(g, restore)
			if current.Status.Phase != v1.LogicalRestoreFailed && len(writers) == 0 {
				freedWhileRunning = true
			}
			g.Expect(current.Status.Phase).To(Equal(v1.LogicalRestoreFailed))
			g.Expect(writers).To(BeEmpty())
		}, 3*timeout, interval).Should(Succeed(), "the failure and the recovery hold each take one grace")
		Expect(freedWhileRunning).To(BeFalse(), "a running restore holds the moved endpoint")
	})
})

// contractOf returns the contract of the Elasticsearch that storage names.
func contractOf(storage *v1.SecondaryStorageConfig) string {
	return camundacluster.StorageContract(camundacluster.Storage{
		Type:      storage.Spec.Type,
		Namespace: storage.Namespace,
		Name:      storage.Name,
	})
}

// moveEndpoint points the storage contract of w at endpoint.
func moveEndpoint(w *world, endpoint string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.SecondaryStorageConfig
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.storage), &latest)).To(Succeed())
		latest.Spec.Elasticsearch.Endpoint = endpoint
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
	w.storage.Spec.Elasticsearch.Endpoint = endpoint
}

// keyAt returns the claim key of the storage contract of w at endpoint.
func keyAt(w *world, endpoint string) string {
	GinkgoHelper()
	elasticsearch := w.storage.Spec.Elasticsearch.DeepCopy()
	elasticsearch.Endpoint = endpoint
	key, err := camundacluster.StorageClaimKey(camundacluster.Storage{
		Type:          w.storage.Spec.Type,
		Namespace:     w.storage.Namespace,
		Elasticsearch: elasticsearch,
	})
	Expect(err).NotTo(HaveOccurred())

	return key
}

// writersSeenByAnotherCluster returns the writers that a cluster other than
// the target of a restore waits for on backend, or on any backend when a
// writer names contract.
func writersSeenByAnotherCluster(backend, contract string) []string {
	GinkgoHelper()
	writers, err := storagewriter.Live(
		ctx,
		k8sClient,
		claimNamespace,
		backend,
		camundacluster.StorageClaimSchema().LeaseName(backend),
		contract,
		"uid-of-another-cluster",
	)
	Expect(err).NotTo(HaveOccurred())

	return writers
}
