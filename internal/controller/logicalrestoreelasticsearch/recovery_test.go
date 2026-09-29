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
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/esadmin/esadmintest"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	restorepkg "github.com/konsole-is/camunda-operator/pkg/restore"
)

// A restore keeps its backend while Elasticsearch still recovers snapshots that it asked for.
var _ = Describe("LogicalRestoreElasticsearch after Elasticsearch accepted its snapshots", func() {
	It("keeps its backend after it fails until Elasticsearch finishes the recovery", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)
		w.search.SetRecoveryActive(true)

		failRestore(restore)

		Expect(latestOf(restore).Status.RecoveryHeld).To(HaveValue(BeTrue()))
		Consistently(func() []string {
			return writersNaming(restore)
		}, "2s", interval).Should(HaveLen(1))

		By("giving the backend back once the recovery ends")
		w.search.SetRecoveryActive(false)
		Eventually(func(g Gomega) {
			g.Expect(writersNaming(restore)).To(BeEmpty())
			g.Expect(latest(g, restore).Status.RecoveryHeld).To(HaveValue(BeFalse()))
		}, timeout, interval).Should(Succeed())
	})

	It("keeps its claim on the target after it fails until the recovery ends", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)
		w.search.SetRecoveryActive(true)
		failRestore(restore)

		retry := createRestore(w, backup.Name)

		reached := expectReason(retry, v1.LogicalRestorePending, v1.ReasonClusterClaimed)
		Expect(readyCondition(reached).Message).To(ContainSubstring(restore.Name))

		By("starting the retry once the recovery ends")
		w.search.SetRecoveryActive(false)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, retry).Status.Phase).NotTo(Equal(v1.LogicalRestorePending))
		}, timeout, interval).Should(Succeed())
	})

	// The Lease reads as renewed a year ago with a one-second duration, so an
	// expiry that a reader measured would have freed the backend long ago.
	It("stays with its backend and its hold after it is deleted until the recovery ends", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)
		w.search.SetRecoveryActive(true)
		ageWriterLeases(latestOf(restore).UID)

		Expect(k8sClient.Delete(ctx, restore)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.RecoveryHeld).To(HaveValue(BeTrue()))
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
			g.Expect(held.Status.RecoveryHeld).To(HaveValue(BeTrue()))
			g.Expect(held.Status.RecoveryUnknownSince).NotTo(BeNil())
			g.Expect(writersNaming(restore)).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		expectGone(w, restore)
	})

	It("does not hold again after the grace while its Job pods keep it from going", func() {
		// outage is a failure count that outlasts the spec.
		const outage = 1000

		w := newWorld()
		collectDeletedJobs(w.namespace)
		backup := createBackup(w)
		restore := startedRestore(w, backup)
		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.PrimaryJobNames).NotTo(BeEmpty())
		}, timeout, interval).Should(Succeed())
		jobName := restorepkg.JobName(labels.LogicalRestoreElasticsearch(restore.Name), 0)
		stuckPod(w, jobName)
		w.search.SetRecoveryActive(true)
		w.search.FailNext("recovery", outage)

		Expect(k8sClient.Delete(ctx, restore)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(latest(g, restore).Status.RecoveryHeld).To(HaveValue(BeFalse()))
		}, midRunGrace+timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(latest(g, restore).Status.RecoveryHeld).To(HaveValue(BeFalse()))
			g.Expect(writersNaming(restore)).To(BeEmpty())
		}, "2s", interval).Should(Succeed())

		By("going once the Job pod is gone")
		pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Namespace: w.namespace, Name: jobName + "-stuck"}}
		Expect(k8sClient.Delete(ctx, pod, client.GracePeriodSeconds(0))).To(Succeed())
		expectGone(w, restore)
	})

	It("does not read the recovery from another Elasticsearch that its target moved to", func() {
		w := newWorld()
		backup := createBackup(w)
		restore := startedRestore(w, backup)
		w.search.SetRecoveryActive(true)
		repointStorage(w)

		Expect(k8sClient.Delete(ctx, restore)).To(Succeed())

		Eventually(func(g Gomega) {
			held := latest(g, restore)
			g.Expect(held.Status.RecoveryHeld).To(HaveValue(BeTrue()))
			g.Expect(held.Status.RecoveryUnknownSince).NotTo(BeNil())
			g.Expect(writersNaming(restore)).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		expectGone(w, restore)
	})
})

// repointStorage moves the storage contract of the target to a second fake
// Elasticsearch, which recovers nothing.
func repointStorage(w *world) {
	GinkgoHelper()
	other := esadmintest.NewTLS()
	DeferCleanup(other.Close)

	ca := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "es-ca-other", Namespace: w.namespace},
		Data:       map[string][]byte{"ca.crt": other.CertificatePEM()},
	}
	Expect(k8sClient.Create(ctx, ca)).To(Succeed())

	Eventually(func(g Gomega) {
		var storage v1.SecondaryStorageConfig
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.storage), &storage)).To(Succeed())
		storage.Spec.Elasticsearch.Endpoint = other.URL()
		storage.Spec.Elasticsearch.CASecretRef = &v1.LocalSecretKeyRef{Name: ca.Name, Key: "ca.crt"}
		g.Expect(k8sClient.Update(ctx, &storage)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

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
