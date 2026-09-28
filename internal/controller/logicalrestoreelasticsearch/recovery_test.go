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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// Elasticsearch recovers the snapshots it accepted whatever happens to the
// restore that asked for them. Another cluster that starts on the backend
// meanwhile writes beside that recovery.
var _ = Describe("LogicalRestoreElasticsearch after Elasticsearch accepted its snapshots", func() {
	It("keeps its backend after it fails until Elasticsearch finishes the recovery", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)
		w.search.SetRecoveryActive(true)

		failRestore(restore)

		Expect(latestOf(restore).Status.RecoveryHeld).To(BeTrue())
		Consistently(func() []string {
			return writersNaming(restore)
		}, "2s", interval).Should(HaveLen(1))

		By("giving the backend back once the recovery ends")
		w.search.SetRecoveryActive(false)
		Eventually(func(g Gomega) {
			g.Expect(writersNaming(restore)).To(BeEmpty())
			g.Expect(latest(g, restore).Status.RecoveryHeld).To(BeFalse())
		}, timeout, interval).Should(Succeed())
	})

	It("stays with its backend and its hold after it is deleted until the recovery ends", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)
		w.search.SetRecoveryActive(true)

		Expect(k8sClient.Delete(ctx, restore)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.RecoveryHeld).To(BeTrue())
		}, timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(restore), restore)).To(Succeed())
			g.Expect(writersNaming(restore)).To(HaveLen(1))
			g.Expect(holdsOf(g, w)).To(HaveLen(1))
		}, "2s", interval).Should(Succeed())

		By("going, with its backend and its hold, once the recovery ends")
		w.search.SetRecoveryActive(false)
		expectGone(w, restore)
	})

	It("gives its backend back once it cannot read the recovery for the grace", func() {
		// outage is a failure count that outlasts the spec.
		const outage = 1000

		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)
		w.search.SetRecoveryActive(true)
		w.search.FailNext("recovery", outage)

		Expect(k8sClient.Delete(ctx, restore)).To(Succeed())

		Eventually(func(g Gomega) {
			held := latest(g, restore)
			g.Expect(held.Status.RecoveryHeld).To(BeTrue())
			g.Expect(held.Status.RecoveryUnknownSince).NotTo(BeNil())
			g.Expect(writersNaming(restore)).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		expectGone(w, restore)
	})
})

// holdsOf returns the suspension holds that the target carries.
func holdsOf(g Gomega, w *world) []string {
	var cluster v1.CamundaCluster
	g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &cluster)).To(Succeed())

	var holds []string
	for key := range cluster.Annotations {
		if strings.HasPrefix(key, v1.SuspensionHoldPrefix) {
			holds = append(holds, key)
		}
	}

	return holds
}

// expectGone waits until the deleted restore is gone, together with its
// writer registration and its suspension hold.
func expectGone(w *world, restore *v1.LogicalRestoreElasticsearch) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(restore), &v1.LogicalRestoreElasticsearch{})
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "the restore is still there: %v", err)
		g.Expect(writersNaming(restore)).To(BeEmpty())
		g.Expect(holdsOf(g, w)).To(BeEmpty())
	}, midRunGrace+timeout, interval).Should(Succeed())
}
