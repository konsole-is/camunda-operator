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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// recoveredIdentifier is the identity that the contract publishes after its
// producer rolled the server back. A recovery starts a new PostgreSQL
// instance, so the identity is never the one the restore pinned at admission.
const recoveredIdentifier = "7000000000000000099"

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
			RequestedBy: request.RequestedBy,
			TargetTime:  request.TargetTime,
			CompletedAt: metav1.Now(),
			Result:      result,
			Message:     message,
		}
		g.Expect(k8sClient.Update(ctx, &contract)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// repointContract moves the endpoint of the contract to the server that the
// recovery built.
func repointContract(w *world, host string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var contract v1.DatabaseServerConfig
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.server), &contract)).To(Succeed())
		contract.Spec.Host = host
		g.Expect(k8sClient.Update(ctx, &contract)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// publishContractReady records the probe that the contract controller writes
// once it reached the server the contract names now.
func publishContractReady(w *world, identifier string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var contract v1.DatabaseServerConfig
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.server), &contract)).To(Succeed())
		contract.Status.SystemIdentifier = identifier
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

// expectRecovering asserts that the restore waits in RestoringDatabase, and
// returns the message it reported.
func expectRecovering(pitr *v1.PointInTimeRestore) string {
	GinkgoHelper()

	var message string
	Eventually(func(g Gomega) {
		current := readRestore(pitr)
		g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreRestoringDatabase))
		condition := ready(current)
		g.Expect(condition).NotTo(BeNil())
		message = condition.Message
	}, timeout, interval).Should(Succeed())

	return message
}

// expectFailed asserts that the restore ended with the given reason, and
// returns the message it reported.
func expectFailed(pitr *v1.PointInTimeRestore, reason string) string {
	GinkgoHelper()

	var message string
	Eventually(func(g Gomega) {
		current := readRestore(pitr)
		g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreFailed))
		condition := ready(current)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(reason))
		message = condition.Message
	}, timeout, interval).Should(Succeed())

	return message
}

var _ = Describe("PointInTimeRestore database recovery", func() {
	It("asks the contract to roll its server back, and waits for the answer", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)

		message := expectRecovering(pitr)
		Expect(message).To(ContainSubstring(w.namespace + "/" + w.server.Name))
		Expect(message).To(ContainSubstring(w.namespace + "/" + pitr.Name))

		request := expectRecoveryRequest(w)
		Expect(request.RequestedBy).To(Equal(w.namespace + "/" + pitr.Name))
		Expect(request.TargetTime).To(Equal(restorePoint().UTC().Format(time.RFC3339)))
	})

	It("reads the database once the contract answers and reaches its new server", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)

		repointContract(w, "postgres-r1.databases.svc")
		publishContractReady(w, recoveredIdentifier)
		answerRecovery(w, v1.RecoveryResultCompleted, "")

		Eventually(func(g Gomega) {
			current := readRestore(pitr)
			g.Expect(current.Status.Phase).NotTo(Equal(v1.PointInTimeRestoreRestoringDatabase))
			g.Expect(current.Status.Phase).NotTo(Equal(v1.PointInTimeRestoreFailed))
			g.Expect(current.Status.Storage).NotTo(BeNil())
			g.Expect(current.Status.Storage.SystemIdentifier).To(Equal(recoveredIdentifier))
			g.Expect(current.Status.Storage.Endpoint).To(Equal("postgres-r1.databases.svc:5432"))
		}, timeout, interval).Should(Succeed())
	})

	It("waits while the contract has not reached the server it now names", func() {
		w := operatorRecoveryWorld()
		pitr := createRestore(w)
		expectRecovering(pitr)

		// The producer repointed the endpoint and answered, and nothing has
		// probed the new server yet. The identity of the old one says nothing
		// about the new one, so the restore holds instead of pinning it.
		repointContract(w, "postgres-r1.databases.svc")
		answerRecovery(w, v1.RecoveryResultCompleted, "")

		Consistently(func() v1.PointInTimeRestorePhase {
			return readRestore(pitr).Status.Phase
		}, "2s", interval).Should(Equal(v1.PointInTimeRestoreRestoringDatabase))

		publishContractReady(w, recoveredIdentifier)

		Eventually(func(g Gomega) {
			current := readRestore(pitr)
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

	It("asks nothing of a contract that nobody rolls back", func() {
		w := createWorld()
		pitr := createRestore(w)

		expectAdmitted(pitr, w)
		Expect(readRestore(pitr).Status.Phase).NotTo(Equal(v1.PointInTimeRestoreRestoringDatabase))

		var contract v1.DatabaseServerConfig
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.server), &contract)).To(Succeed())
		Expect(contract.Spec.Recovery).To(BeNil())
	})
})
