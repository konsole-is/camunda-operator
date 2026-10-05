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

package databaseserver

import (
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

var _ = Describe("DatabaseServer grace period", func() {
	It("comes back when the grace period of a cluster that is not ready runs out", func() {
		server := serverInNamespace(nil)
		// Without this Secret, the contract asks for a short retry that hides the grace wait.
		writeSuperuserSecret(server)
		expectCondition(server, v1.ConditionContractReady, metav1.ConditionTrue)
		expectCondition(server, v1.ConditionClusterReady, metav1.ConditionFalse)

		Eventually(func(g Gomega) {
			result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKeyFromObject(server)})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(result.RequeueAfter).To(BeNumerically(">", suiteGracePeriods.Datastore-time.Minute))
			g.Expect(result.RequeueAfter).To(BeNumerically("<=", suiteGracePeriods.Datastore+time.Second))
		}, timeout, interval).Should(Succeed())
	})

	It("starts a new grace period when a cluster name that another owner held is free", func() {
		namespace := "dbs-" + utilrand.String(8)
		Expect(k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: namespace},
		})).To(Succeed())

		occupant := &cnpgv1.Cluster{
			ObjectMeta: metav1.ObjectMeta{Name: "camunda", Namespace: namespace},
			Spec: cnpgv1.ClusterSpec{
				Instances:            1,
				StorageConfiguration: cnpgv1.StorageConfiguration{Size: "1Gi"},
			},
		}
		Expect(k8sClient.Create(ctx, occupant)).To(Succeed())

		server := serverNamed(namespace, "camunda", "camunda", nil)
		expectConditionReason(server, v1.ConditionClusterReady, metav1.ConditionFalse, v1.ReasonClusterTaken)

		// lastTransitionTime keeps whole seconds, so the release must fall in
		// a later second than the ClusterTaken condition.
		time.Sleep(1100 * time.Millisecond)
		released := metav1.NewTime(time.Now().Truncate(time.Second))
		Expect(k8sClient.Delete(ctx, occupant)).To(Succeed())

		Eventually(func(g Gomega) {
			cond := conditionOf(server, v1.ConditionClusterReady)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Reason).NotTo(Equal(v1.ReasonClusterTaken))
			g.Expect(cond.LastTransitionTime.Before(&released)).To(BeFalse(), cond.LastTransitionTime.String())
		}, timeout, interval).Should(Succeed())
	})
})
