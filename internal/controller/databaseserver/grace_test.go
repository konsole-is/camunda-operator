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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
})
