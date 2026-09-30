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

package pointintimerestore

import (
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	coordinationv1 "k8s.io/api/coordination/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	clustercomponents "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/restore"
	"github.com/konsole-is/camunda-operator/pkg/storagewriter"
)

// recoveredIdentifier is the identity that the contract publishes after its
// producer rolled the server back. A physical recovery restores the pg_control
// of the base backup it reads, so the recovered instance reports the identity
// the restore pinned at admission. Only the endpoint moves.
const recoveredIdentifier = worldSystemIdentifier

// recoveredHost is the endpoint that the contract names once its producer
// rolled the server back.
const recoveredHost = "postgres-r1.databases.svc"

// replacedIdentifier is the identity of another PostgreSQL instance. A
// contract that reports it after a rollback reaches a server the restore never
// validated.
const replacedIdentifier = "7000000000000000099"

// operatorRecoveryWorld is a world whose contract declares that its producer
// rolls the server back on request.
func operatorRecoveryWorld() *world {
	GinkgoHelper()

	return createWorld(func(w *world) {
		w.server.Spec.PITR = &v1.PITRCapability{
			Enabled:             true,
			RetentionPeriodDays: new(int32(7)),
			Recovery:            v1.RecoveryModeOperator,
		}
	})
}

// expectRecoveryRequest waits until the restore wrote its request on the
// contract, and returns it.
func expectRecoveryRequest(w *world) *v1.RecoveryRequest {
	GinkgoHelper()

	var request *v1.RecoveryRequest
	Eventually(func(g Gomega) {
		var contract v1.DatabaseServerConfig
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.server), &contract)).To(Succeed())
		g.Expect(contract.Spec.Recovery).NotTo(BeNil())
		request = contract.Spec.Recovery
	}, timeout, interval).Should(Succeed())

	return request
}

// answerRecovery publishes an outcome on the contract, the way its producer
// does when it has finished with the request.
func answerRecovery(w *world, result v1.RecoveryResult, message string) {
	GinkgoHelper()

	request := expectRecoveryRequest(w)
	Eventually(func(g Gomega) {
		var contract v1.DatabaseServerConfig
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.server), &contract)).To(Succeed())
		contract.Spec.PITR.LastRecovery = &v1.RecoveryOutcome{
			RequestID:   request.RequestID,
			RequestedBy: request.RequestedBy,
			TargetTime:  request.TargetTime,
			CompletedAt: metav1.Now(),
			Result:      result,
			Message:     message,
		}
		g.Expect(k8sClient.Update(ctx, &contract)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// repointContract moves the contract to the recovered server, and claims that
// endpoint for the cluster as its controller does.
func repointContract(w *world) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var contract v1.DatabaseServerConfig
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.server), &contract)).To(Succeed())
		contract.Spec.Host = recoveredHost
		g.Expect(k8sClient.Update(ctx, &contract)).To(Succeed())
	}, timeout, interval).Should(Succeed())
	holdBackend(w.cluster)
}

// publishContractReady records the probe that the contract controller writes
// once it reached the server the contract names now.
func publishContractReady(w *world, identifier string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var contract v1.DatabaseServerConfig
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.server), &contract)).To(Succeed())
		contract.Status.SystemIdentifier = identifier
		contract.Status.ObservedGeneration = contract.Generation
		meta.SetStatusCondition(&contract.Status.Conditions, metav1.Condition{
			Type:               v1.ConditionReady,
			Status:             metav1.ConditionTrue,
			Reason:             v1.ReasonHealthy,
			Message:            "Reached the server",
			ObservedGeneration: contract.Generation,
		})
		g.Expect(k8sClient.Status().Update(ctx, &contract)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// publishStaleContractReady records a probe that answered for the spec of
// before the last change, the way the contract reads between a repoint and the
// probe that follows it.
func publishStaleContractReady(w *world, identifier string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var contract v1.DatabaseServerConfig
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.server), &contract)).To(Succeed())
		contract.Status.SystemIdentifier = identifier
		contract.Status.ObservedGeneration = contract.Generation - 1
		meta.SetStatusCondition(&contract.Status.Conditions, metav1.Condition{
			Type:               v1.ConditionReady,
			Status:             metav1.ConditionTrue,
			Reason:             v1.ReasonHealthy,
			Message:            "Reached the server",
			ObservedGeneration: contract.Generation - 1,
		})
		g.Expect(k8sClient.Status().Update(ctx, &contract)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// expectRecovering asserts that the restore waits in RestoringDatabase and
// reports a message that holds every given substring.
//
// The phase reports two messages in turn. Admission writes the first one when
// it hands the restore over, and the phase itself writes the second one once
// the request stands on the contract. A caller that names a substring of the
// second message therefore waits for that message, instead of reading whichever
// of the two the first look catches.
func expectRecovering(pitr *v1.PointInTimeRestore, substrings ...string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		current := readRestore(g, pitr)
		g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreRestoringDatabase))
		condition := ready(current)
		g.Expect(condition).NotTo(BeNil())
		for _, substring := range substrings {
			g.Expect(condition.Message).To(ContainSubstring(substring))
		}
	}, timeout, interval).Should(Succeed())
}

// expectFailed asserts that the restore ended with the given reason, and
// returns the message it reported.
func expectFailed(pitr *v1.PointInTimeRestore, reason string) string {
	GinkgoHelper()

	var message string
	Eventually(func(g Gomega) {
		current := readRestore(g, pitr)
		g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreFailed))
		condition := ready(current)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(reason))
		message = condition.Message
	}, timeout, interval).Should(Succeed())

	return message
}

// contractOf returns the contract of the database of the world.
func contractOf(w *world) string {
	return restore.DatabaseContract(client.ObjectKeyFromObject(w.server), w.dbConfig.Spec.DatabaseName)
}

// backendsHeldBy returns the backends of the writer Leases that name the
// restore.
func backendsHeldBy(pitr *v1.PointInTimeRestore) []string {
	GinkgoHelper()

	var leases coordinationv1.LeaseList
	Expect(k8sClient.List(
		ctx,
		&leases,
		client.InNamespace(testClaimNamespace),
		client.MatchingLabels{labels.ComponentKey: storagewriter.Component},
	)).To(Succeed())

	var backends []string
	for i := range leases.Items {
		annotations := leases.Items[i].Annotations
		if annotations[storagewriter.WriterAnnotation] == "PointInTimeRestore "+pitr.Namespace+"/"+pitr.Name {
			backends = append(backends, annotations[storagewriter.KeyAnnotation])
		}
	}

	return backends
}

// writersSeenByAnotherCluster returns the writers that a cluster other than
// the target of the restore waits for on backend, or on any backend when a
// writer names contract. An empty contract matches on backend alone.
func writersSeenByAnotherCluster(backend, contract string) []string {
	GinkgoHelper()

	writers, err := storagewriter.Live(
		ctx,
		k8sClient,
		testClaimNamespace,
		backend,
		clustercomponents.StorageClaimSchema().LeaseName(backend),
		contract,
		"uid-of-another-cluster",
	)
	Expect(err).NotTo(HaveOccurred())

	return writers
}

// expectBackendHeld waits until the restore holds exactly one backend, and
// returns it.
func expectBackendHeld(pitr *v1.PointInTimeRestore) string {
	GinkgoHelper()

	var backend string
	Eventually(func(g Gomega) {
		current := readRestore(g, pitr)
		g.Expect(current.Status.Backend).NotTo(BeEmpty())
		g.Expect(backendsHeldBy(pitr)).To(Equal([]string{current.Status.Backend}))
		backend = current.Status.Backend
	}, timeout, interval).Should(Succeed())

	return backend
}

var _ = Describe("PointInTimeRestore database recovery", func() {
	It("asks the contract to roll its server back, and waits for the answer", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)

		expectRecovering(pitr, w.namespace+"/"+w.server.Name, w.namespace+"/"+pitr.Name)

		request := expectRecoveryRequest(w)
		Expect(request.RequestID).To(Equal(string(readRestore(Default, pitr).UID)))
		Expect(request.RequestedBy).To(Equal(w.namespace + "/" + pitr.Name))
		Expect(request.TargetTime).To(Equal(restorePoint().UTC().Format(time.RFC3339)))
	})

	It("asks for the point the cluster holds, to the precision the spec keeps", func() {
		w := operatorRecoveryWorld()
		fractional := metav1.NewTime(restorePoint().Add(1500 * time.Millisecond))
		pitr := createRestore(w, func(p *v1.PointInTimeRestore) { p.Spec.Timestamp = fractional })

		expectRecovering(pitr)

		// The request carries the point the resource holds, whatever was
		// written into it.
		stored := readRestore(Default, pitr).Spec.Timestamp
		request := expectRecoveryRequest(w)
		Expect(request.TargetTime).To(Equal(stored.UTC().Format(time.RFC3339Nano)))
	})

	It("fails when the contract is replaced under its name while it waits", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)

		// Only the endpoint and the identity of the server are allowed to
		// move while the rollback runs. A contract that is deleted and created
		// again under one name is another contract, and nothing of what the
		// restore validated pairs with it.
		Expect(k8sClient.Delete(ctx, w.server)).To(Succeed())
		replacement := w.server.DeepCopy()
		replacement.ObjectMeta = metav1.ObjectMeta{Name: w.server.Name, Namespace: w.namespace}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Create(ctx, replacement)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		message := expectFailed(pitr, v1.ReasonFailed)
		Expect(message).To(ContainSubstring("no DatabaseServerConfig can answer"))
	})

	It("fails and gives its database back when its contract is deleted while it waits", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)
		expectRecoveryRequest(w)
		backend := expectBackendHeld(pitr)

		Expect(k8sClient.Delete(ctx, w.server)).To(Succeed())

		Expect(expectFailed(pitr, v1.ReasonFailed)).To(ContainSubstring(w.server.Name))
		Eventually(func() []string {
			return writersSeenByAnotherCluster(backend, "")
		}, timeout, interval).Should(BeEmpty())
	})

	It("keeps its database held for the grace when its cluster and its contract are both deleted", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)
		expectRecoveryRequest(w)
		backend := expectBackendHeld(pitr)

		Expect(k8sClient.Delete(ctx, w.server)).To(Succeed())
		Expect(k8sClient.Delete(ctx, w.cluster)).To(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(readRestore(g, pitr).Status.Phase).To(Equal(v1.PointInTimeRestoreRestoringDatabase))
			g.Expect(writersSeenByAnotherCluster(backend, "")).NotTo(BeEmpty())
		}, time.Second, interval).Should(Succeed())

		Expect(expectFailed(pitr, v1.ReasonFailed)).To(ContainSubstring("no DatabaseServerConfig can answer"))
		Eventually(func() []string {
			return writersSeenByAnotherCluster(backend, "")
		}, timeout, interval).Should(BeEmpty())
	})

	// The server keeps running a rollback that it accepted. The hold outlasts
	// the mid-run grace, and the Lease reads as renewed a year ago with a
	// one-second duration, so neither a bound nor an expiry frees the database.
	It("keeps its database held after it is deleted until the server answers, then gives it back", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)
		expectRecoveryRequest(w)
		expectBackendHeld(pitr)
		ageWriterLeases(readRestore(Default, pitr).UID)

		Expect(k8sClient.Delete(ctx, pitr)).To(Succeed())

		expectRecovering(pitr, "The restore was deleted", w.server.Name)
		repointContract(w)
		var backend string
		Eventually(func(g Gomega) {
			current := readRestore(g, pitr)
			g.Expect(current.Status.Backend).To(ContainSubstring(recoveredHost))
			g.Expect(backendsHeldBy(pitr)).To(Equal([]string{current.Status.Backend}))
			backend = current.Status.Backend
		}, timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(readRestore(g, pitr).DeletionTimestamp).NotTo(BeNil())
			g.Expect(writersSeenByAnotherCluster(backend, "")).To(HaveLen(1))
		}, midRunGrace+2*time.Second, interval).Should(Succeed())

		answerRecovery(w, v1.RecoveryResultCompleted, "")
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pitr), &v1.PointInTimeRestore{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			g.Expect(writersSeenByAnotherCluster(backend, "")).To(BeEmpty())
		}, timeout, interval).Should(Succeed())
	})

	// Without a contract, nothing reports the end of the rollback, so the
	// hold ends with the mid-run grace.
	It("keeps its database held after it is deleted for the grace when no contract can answer", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)
		expectRecoveryRequest(w)
		backend := expectBackendHeld(pitr)

		Expect(k8sClient.Delete(ctx, pitr)).To(Succeed())
		expectRecovering(pitr, "The restore was deleted")
		Expect(k8sClient.Delete(ctx, w.server)).To(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(readRestore(g, pitr).DeletionTimestamp).NotTo(BeNil())
			g.Expect(writersSeenByAnotherCluster(backend, "")).NotTo(BeEmpty())
		}, time.Second, interval).Should(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(pitr), &v1.PointInTimeRestore{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
			g.Expect(writersSeenByAnotherCluster(backend, "")).To(BeEmpty())
		}, midRunGrace+timeout, interval).Should(Succeed())
	})

	It("asks again when a restore of its name and its point was answered before it", func() {
		w := operatorRecoveryWorld()
		first := createRestore(w)
		expectRecovering(first)
		answerRecovery(w, v1.RecoveryResultCompleted, "")
		answered := expectRecoveryRequest(w)

		// The same name and the same point, and a different resource. The
		// standing answer belongs to the restore that is gone.
		Expect(k8sClient.Delete(ctx, first)).To(Succeed())
		second := &v1.PointInTimeRestore{
			ObjectMeta: metav1.ObjectMeta{Name: first.Name, Namespace: w.namespace},
			Spec:       first.Spec,
		}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Create(ctx, second)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		deleteAtSpecEnd(second)

		expectRecovering(second)
		Eventually(func(g Gomega) {
			request := expectRecoveryRequest(w)
			g.Expect(request.RequestID).NotTo(Equal(answered.RequestID))
			g.Expect(request.RequestID).To(Equal(string(readRestore(g, second).UID)))
			g.Expect(request.TargetTime).To(Equal(answered.TargetTime))
		}, timeout, interval).Should(Succeed())
	})

	It("reads the database once the contract answers and reaches its new server", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)

		repointContract(w)
		answerRecovery(w, v1.RecoveryResultCompleted, "")
		publishContractReady(w, recoveredIdentifier)

		Eventually(func(g Gomega) {
			current := readRestore(g, pitr)
			g.Expect(current.Status.Phase).NotTo(Equal(v1.PointInTimeRestoreRestoringDatabase))
			g.Expect(current.Status.Phase).NotTo(Equal(v1.PointInTimeRestoreFailed))
			g.Expect(current.Status.Storage).NotTo(BeNil())
			g.Expect(current.Status.Storage.SystemIdentifier).To(Equal(recoveredIdentifier))
			g.Expect(current.Status.Storage.Endpoint).To(Equal(recoveredHost + ":5432"))
		}, timeout, interval).Should(Succeed())
	})

	It("ends the restore when the rolled-back server reports another identity", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)

		repointContract(w)
		answerRecovery(w, v1.RecoveryResultCompleted, "")

		// The endpoint moves across a rollback and the identity does not, so
		// another identity behind it is another instance. The rules of
		// admission were read against the one the restore pinned.
		publishContractReady(w, replacedIdentifier)

		Eventually(func(g Gomega) {
			current := readRestore(g, pitr)
			g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreFailed))
			g.Expect(current.Status.FailureMessage).To(ContainSubstring(worldSystemIdentifier))
			g.Expect(current.Status.FailureMessage).To(ContainSubstring(replacedIdentifier))
		}, timeout, interval).Should(Succeed())
	})

	It("keeps waiting while the contract publishes no identity", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)

		repointContract(w)
		answerRecovery(w, v1.RecoveryResultCompleted, "")

		// The contract controller clears the identity when the endpoint moves
		// and publishes it again once it reached the server. The look in
		// between reads no identity at all. That states nothing about the
		// instance, so the pin of the restore holds instead of ending it.
		publishContractReady(w, "")
		Consistently(func(g Gomega) v1.PointInTimeRestorePhase {
			return readRestore(g, pitr).Status.Phase
		}, "2s", interval).Should(Equal(v1.PointInTimeRestoreRestoringDatabase))

		publishContractReady(w, recoveredIdentifier)

		Eventually(func(g Gomega) {
			current := readRestore(g, pitr)
			g.Expect(current.Status.Phase).NotTo(Equal(v1.PointInTimeRestoreRestoringDatabase))
			g.Expect(current.Status.Phase).NotTo(Equal(v1.PointInTimeRestoreFailed))
		}, timeout, interval).Should(Succeed())
	})

	It("waits while the contract has not reached the server it now names", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)

		// The producer repointed the endpoint and answered, and nothing has
		// probed the new server yet. The identity of the old one says nothing
		// about the new one, so the restore holds instead of pinning it.
		repointContract(w)
		answerRecovery(w, v1.RecoveryResultCompleted, "")

		// A Ready that answers the spec of before the endpoint moved says
		// nothing about the server the contract names now.
		publishStaleContractReady(w, worldSystemIdentifier)
		Consistently(func(g Gomega) v1.PointInTimeRestorePhase {
			return readRestore(g, pitr).Status.Phase
		}, "2s", interval).Should(Equal(v1.PointInTimeRestoreRestoringDatabase))

		publishContractReady(w, recoveredIdentifier)

		Eventually(func(g Gomega) {
			current := readRestore(g, pitr)
			g.Expect(current.Status.Phase).NotTo(Equal(v1.PointInTimeRestoreRestoringDatabase))
			g.Expect(current.Status.Phase).NotTo(Equal(v1.PointInTimeRestoreFailed))
		}, timeout, interval).Should(Succeed())
	})

	It("fails a point that the server never held", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)

		answerRecovery(w, v1.RecoveryResultUnavailable, "no archive of the server holds that point")

		message := expectFailed(pitr, v1.ReasonPitrUnavailable)
		Expect(message).To(ContainSubstring("no archive of the server holds that point"))
	})

	It("fails a rollback that the server started and did not finish", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)

		answerRecovery(w, v1.RecoveryResultFailed, "the server is suspended")

		message := expectFailed(pitr, v1.ReasonFailed)
		Expect(message).To(ContainSubstring("the server is suspended"))
	})

	It("holds its database against other clusters while the server rolls back", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)

		backend := expectBackendHeld(pitr)
		Expect(writersSeenByAnotherCluster(backend, "")).To(
			Equal([]string{"PointInTimeRestore " + w.namespace + "/" + pitr.Name}),
		)
		elsewhere := "rdbms|elsewhere." + w.namespace + ".svc:5432/" + w.dbConfig.Spec.DatabaseName
		Expect(writersSeenByAnotherCluster(elsewhere, contractOf(w))).To(
			Equal([]string{"PointInTimeRestore " + w.namespace + "/" + pitr.Name}),
			"a cluster on the contract waits at any address",
		)

		answerRecovery(w, v1.RecoveryResultFailed, "the server is suspended")
		expectFailed(pitr, v1.ReasonFailed)
		Eventually(func() []string {
			return backendsHeldBy(pitr)
		}, timeout, interval).Should(BeEmpty(), "a terminal restore gives its database back")
		Expect(writersSeenByAnotherCluster(backend, "")).To(BeEmpty())
		Expect(writersSeenByAnotherCluster(elsewhere, contractOf(w))).To(BeEmpty())
	})

	It("keeps its database held for the grace when its request is cleared and its cluster is deleted", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)
		expectRecoveryRequest(w)
		backend := expectBackendHeld(pitr)

		Eventually(func(g Gomega) {
			var contract v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.server), &contract)).To(Succeed())
			contract.Spec.Recovery = nil
			g.Expect(k8sClient.Update(ctx, &contract)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Expect(k8sClient.Delete(ctx, w.cluster)).To(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(readRestore(g, pitr).Status.Phase).To(Equal(v1.PointInTimeRestoreRestoringDatabase))
			g.Expect(writersSeenByAnotherCluster(backend, "")).NotTo(BeEmpty())
		}, time.Second, interval).Should(Succeed())
		expectFailed(pitr, v1.ReasonFailed)
	})

	It("keeps its database held when its cluster is deleted, until the server answers", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)
		expectRecoveryRequest(w)
		backend := expectBackendHeld(pitr)

		Expect(k8sClient.Delete(ctx, w.cluster)).To(Succeed())
		expectRecovering(pitr, "was deleted", w.server.Name)

		repointContract(w)
		Eventually(func(g Gomega) {
			current := readRestore(g, pitr)
			g.Expect(current.Status.Backend).To(ContainSubstring(recoveredHost))
			g.Expect(backendsHeldBy(pitr)).To(Equal([]string{current.Status.Backend}))
			backend = current.Status.Backend
		}, timeout, interval).Should(Succeed())
		Consistently(func() []string {
			return writersSeenByAnotherCluster(backend, "")
		}, "2s", interval).Should(HaveLen(1))

		answerRecovery(w, v1.RecoveryResultCompleted, "")
		message := expectFailed(pitr, v1.ReasonFailed)
		Expect(message).To(ContainSubstring("was deleted"))
		Eventually(func() []string {
			return writersSeenByAnotherCluster(backend, "")
		}, timeout, interval).Should(BeEmpty(), "the restore gives the database back once the rollback ended")
	})

	It("follows a contract that moves and stops recovering while its cluster is gone", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)
		expectRecoveryRequest(w)
		expectBackendHeld(pitr)

		Expect(k8sClient.Delete(ctx, w.cluster)).To(Succeed())
		expectRecovering(pitr, "was deleted", w.server.Name)

		Eventually(func(g Gomega) {
			var contract v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.server), &contract)).To(Succeed())
			contract.Spec.Host = recoveredHost
			contract.Spec.PITR.Recovery = v1.RecoveryModeExternal
			g.Expect(k8sClient.Update(ctx, &contract)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(readRestore(g, pitr).Status.Backend).To(ContainSubstring(recoveredHost))
		}, timeout, interval).Should(Succeed())
	})

	It("follows a moved contract without waiting for its retry timer while its cluster is gone", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)
		expectRecoveryRequest(w)
		expectBackendHeld(pitr)

		Expect(k8sClient.Delete(ctx, w.cluster)).To(Succeed())
		expectRecovering(pitr, "was deleted", w.server.Name)

		repointContract(w)
		Eventually(func(g Gomega) {
			current := readRestore(g, pitr)
			g.Expect(current.Status.Backend).To(ContainSubstring(recoveredHost))
			g.Expect(backendsHeldBy(pitr)).To(ContainElement(current.Status.Backend))
		}, watchWindow, interval).Should(Succeed())
	})

	It("keeps its database held when its cluster is still being deleted, until the server answers", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)
		expectRecoveryRequest(w)
		backend := expectBackendHeld(pitr)

		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &cluster)).To(Succeed())
			cluster.Finalizers = append(cluster.Finalizers, "test.camunda.io/keep")
			g.Expect(k8sClient.Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		DeferCleanup(func() {
			var cluster v1.CamundaCluster
			if k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &cluster) == nil {
				cluster.Finalizers = nil
				_ = k8sClient.Update(ctx, &cluster)
			}
		})
		Expect(k8sClient.Delete(ctx, w.cluster)).To(Succeed())
		expectRecovering(pitr, "was deleted", w.server.Name)

		answerRecovery(w, v1.RecoveryResultCompleted, "")
		Expect(expectFailed(pitr, v1.ReasonFailed)).To(ContainSubstring("was deleted"))
		Eventually(func() []string {
			return writersSeenByAnotherCluster(backend, "")
		}, timeout, interval).Should(BeEmpty())
	})

	It("keeps its database held when its cluster is repointed, until the server answers", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)
		expectRecoveryRequest(w)
		backend := expectBackendHeld(pitr)

		other := &v1.DatabaseConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "other-config", Namespace: w.namespace},
			Spec: v1.DatabaseConfigSpec{
				ServerRef:            w.server.Name,
				DatabaseName:         "other_database",
				CredentialsSecretRef: w.dbConfig.Spec.CredentialsSecretRef,
			},
		}
		Expect(k8sClient.Create(ctx, other)).To(Succeed())
		otherStorage := &v1.SecondaryStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "other-storage", Namespace: w.namespace},
			Spec: v1.SecondaryStorageConfigSpec{
				Type:  v1.SecondaryStorageTypeRDBMS,
				RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: other.Name},
			},
		}
		Expect(k8sClient.Create(ctx, otherStorage)).To(Succeed())
		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &cluster)).To(Succeed())
			cluster.Spec.StorageRef = otherStorage.Name
			g.Expect(k8sClient.Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		expectRecovering(pitr, "storage chain", w.server.Name)
		Expect(writersSeenByAnotherCluster(backend, "")).To(HaveLen(1))

		answerRecovery(w, v1.RecoveryResultCompleted, "")
		Expect(expectFailed(pitr, v1.ReasonFailed)).To(ContainSubstring("other-storage"))
		Eventually(func() []string {
			return writersSeenByAnotherCluster(backend, "")
		}, timeout, interval).Should(BeEmpty())
	})

	It("keeps its database held when its cluster runs again, until the server answers", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)
		expectRecoveryRequest(w)
		backend := expectBackendHeld(pitr)

		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &cluster)).To(Succeed())
			cluster.Spec.Suspend = false
			for key := range cluster.Annotations {
				if strings.HasPrefix(key, v1.SuspensionHoldPrefix) {
					delete(cluster.Annotations, key)
				}
			}
			g.Expect(k8sClient.Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		expectRecovering(pitr, "started running again", w.server.Name)

		By("following the endpoint that the contract names while it holds")
		repointContract(w)
		Eventually(func(g Gomega) {
			current := readRestore(g, pitr)
			g.Expect(current.Status.Backend).To(ContainSubstring(recoveredHost))
			g.Expect(backendsHeldBy(pitr)).To(Equal([]string{current.Status.Backend}))
			backend = current.Status.Backend
		}, timeout, interval).Should(Succeed())

		By("holding past the mid-run grace while the request is unanswered")
		Consistently(func(g Gomega) {
			current := readRestore(g, pitr)
			g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreRestoringDatabase))
			g.Expect(ready(current).Reason).To(Equal(v1.ReasonClusterNotSuspended))
			g.Expect(writersSeenByAnotherCluster(backend, "")).To(HaveLen(1))
		}, 5*time.Second, interval).Should(Succeed())

		answerRecovery(w, v1.RecoveryResultCompleted, "")
		Expect(expectFailed(pitr, v1.ReasonFailed)).To(ContainSubstring("is not suspended"))
		Eventually(func() []string {
			return writersSeenByAnotherCluster(backend, "")
		}, timeout, interval).Should(BeEmpty())
	})

	It("gives its database back when the chain stops resolving after the server answered", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)
		expectRecoveryRequest(w)
		backend := expectBackendHeld(pitr)
		answerRecovery(w, v1.RecoveryResultCompleted, "")

		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &cluster)).To(Succeed())
			cluster.Spec.StorageRef = "no-such-storage"
			g.Expect(k8sClient.Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Expect(expectFailed(pitr, v1.ReasonFailed)).To(ContainSubstring("no-such-storage"))
		Eventually(func() []string {
			return writersSeenByAnotherCluster(backend, "")
		}, timeout, interval).Should(BeEmpty())
	})

	It("holds the endpoint that the contract names before the contract reaches it", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)
		before := expectBackendHeld(pitr)

		repointContract(w)
		recovered := w.server.DeepCopy()
		recovered.Spec.Host = recoveredHost
		moved, failure := restore.DatabaseBackend(w.storage, w.dbConfig, recovered)
		Expect(failure).NotTo(HaveOccurred())
		Expect(writersSeenByAnotherCluster(moved, contractOf(w))).To(
			Equal([]string{"PointInTimeRestore " + w.namespace + "/" + pitr.Name}),
			"the restore holds the new endpoint whether or not it followed the move yet",
		)

		var named string
		Eventually(func(g Gomega) {
			current := readRestore(g, pitr)
			g.Expect(current.Status.Backend).NotTo(Equal(before))
			g.Expect(backendsHeldBy(pitr)).To(Equal([]string{current.Status.Backend}))
			named = current.Status.Backend
		}, timeout, interval).Should(Succeed())
		Expect(named).To(ContainSubstring(recoveredHost))
		Expect(writersSeenByAnotherCluster(named, "")).To(
			Equal([]string{"PointInTimeRestore " + w.namespace + "/" + pitr.Name}),
		)
		Expect(readRestore(Default, pitr).Status.Phase).To(Equal(v1.PointInTimeRestoreRestoringDatabase))

		answerRecovery(w, v1.RecoveryResultCompleted, "")
		Consistently(func() []string {
			return backendsHeldBy(pitr)
		}, time.Second, interval).Should(Equal([]string{named}))

		publishContractReady(w, recoveredIdentifier)
		Eventually(func(g Gomega) {
			current := readRestore(g, pitr)
			g.Expect(current.Status.Phase).NotTo(Equal(v1.PointInTimeRestoreRestoringDatabase))
			g.Expect(backendsHeldBy(pitr)).To(Equal([]string{named}))
		}, timeout, interval).Should(Succeed())
	})

	It("moves its hold to the endpoint of the recovered server", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)
		before := expectBackendHeld(pitr)

		repointContract(w)
		answerRecovery(w, v1.RecoveryResultCompleted, "")
		publishContractReady(w, recoveredIdentifier)

		Eventually(func(g Gomega) {
			current := readRestore(g, pitr)
			g.Expect(current.Status.Phase).NotTo(Equal(v1.PointInTimeRestoreRestoringDatabase))
			g.Expect(current.Status.Backend).NotTo(Equal(before))
		}, timeout, interval).Should(Succeed())
		after := expectBackendHeld(pitr)
		Expect(strings.Contains(after, recoveredHost)).To(BeTrue(), after)
		Expect(writersSeenByAnotherCluster(before, "")).To(BeEmpty())
	})

	It("holds after the answer while another cluster holds the endpoint the contract moved to", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)
		expectBackendHeld(pitr)

		repointContract(w)
		other := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "other-" + w.cluster.Name, Namespace: w.namespace},
			Spec:       *w.cluster.Spec.DeepCopy(),
		}
		Expect(k8sClient.Create(ctx, other)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, other) })
		holdBackend(other)
		answerRecovery(w, v1.RecoveryResultCompleted, "")
		publishContractReady(w, recoveredIdentifier)

		Eventually(func(g Gomega) {
			current := readRestore(g, pitr)
			g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreRestoringDatabase))
			condition := ready(current)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Reason).To(Equal(v1.ReasonStorageAlreadyAttached))
			g.Expect(condition.Message).To(ContainSubstring(other.Name))
		}, timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(readRestore(g, pitr).Status.Phase).To(Equal(v1.PointInTimeRestoreRestoringDatabase))
		}, time.Second, interval).Should(Succeed())

		By("going on once its own cluster holds the moved endpoint")
		holdBackend(w.cluster)
		wake(w.cluster)
		Eventually(func(g Gomega) {
			g.Expect(readRestore(g, pitr).Status.Phase).NotTo(Equal(v1.PointInTimeRestoreRestoringDatabase))
		}, timeout, interval).Should(Succeed())
	})

	It("asks nothing of a contract that nobody rolls back", func() {
		w := createWorld()
		pitr := createRestore(w)

		expectAdmitted(pitr, w)
		Expect(readRestore(Default, pitr).Status.Phase).NotTo(Equal(v1.PointInTimeRestoreRestoringDatabase))

		var contract v1.DatabaseServerConfig
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.server), &contract)).To(Succeed())
		Expect(contract.Spec.Recovery).To(BeNil())
		Expect(readRestore(Default, pitr).Status.Backend).To(BeEmpty())
		Expect(backendsHeldBy(pitr)).To(BeEmpty(), "a restore that writes no database holds none")
	})
})

// ageWriterLeases makes every writer Lease of the writer with UID uid read as
// renewed a year ago with a one-second duration.
func ageWriterLeases(uid types.UID) {
	GinkgoHelper()
	old := metav1.NewMicroTime(time.Now().AddDate(-1, 0, 0))
	Eventually(func(g Gomega) {
		var leases coordinationv1.LeaseList
		g.Expect(k8sClient.List(
			ctx,
			&leases,
			client.InNamespace(testClaimNamespace),
			client.MatchingLabels{labels.WriterUIDKey: string(uid)},
		)).To(Succeed())
		g.Expect(leases.Items).NotTo(BeEmpty())
		for i := range leases.Items {
			lease := &leases.Items[i]
			lease.Spec.AcquireTime = &old
			lease.Spec.RenewTime = &old
			lease.Spec.LeaseDurationSeconds = new(int32(1))
			g.Expect(k8sClient.Update(ctx, lease)).To(Succeed())
		}
	}, timeout, interval).Should(Succeed())
}
