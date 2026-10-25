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

package camundamanagementcluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
)

var _ = Describe("CamundaManagementCluster ServiceAccounts", func() {
	It("creates, owns, and annotates the account of the plane, and names it in the status", func() {
		s := newScenario(func(f *fixture) {
			f.mc.Spec.ServiceAccount = &v1.ServiceAccountSpec{
				Annotations: map[string]string{"eks.amazonaws.com/role-arn": "arn:aws:iam::1:role/plane"},
			}
		})
		name := s.mc.Name + "-management"

		Eventually(func(g Gomega) {
			var account corev1.ServiceAccount
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: s.namespace, Name: name}, &account)).
				To(Succeed())
			g.Expect(account.Annotations).
				To(HaveKeyWithValue("eks.amazonaws.com/role-arn", "arn:aws:iam::1:role/plane"))
			g.Expect(metav1.IsControlledBy(&account, readManagementCluster(g, s.mc))).To(BeTrue())
			g.Expect(readManagementCluster(g, s.mc).Status.ServiceAccountName).To(Equal(name))
			g.Expect(identityServiceAccount(g, s)).To(Equal(name))
		}, timeout, interval).Should(Succeed())
	})

	It("runs a component with an account of its own under that account", func() {
		s := newScenario(func(f *fixture) {
			f.mc.Spec.Identity.ServiceAccount = &v1.ServiceAccountSpec{}
		})
		own := s.mc.Name + "-identity"

		Eventually(func(g Gomega) {
			var account corev1.ServiceAccount
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: s.namespace, Name: own}, &account)).
				To(Succeed())
			g.Expect(identityServiceAccount(g, s)).To(Equal(own))
			g.Expect(readManagementCluster(g, s.mc).Status.ServiceAccountName).To(Equal(s.mc.Name + "-management"))
		}, timeout, interval).Should(Succeed())
	})

	It("reports InvalidReference while an account with create false is missing", func() {
		s := newScenario(func(f *fixture) {
			f.mc.Spec.ServiceAccount = &v1.ServiceAccountSpec{Name: "existing", Create: new(false)}
		})

		expectReadyReason(s.mc, v1.ReasonInvalidReference)
		Expect(conditionOf(Default, s.mc, v1.ConditionReady).Message).
			To(ContainSubstring("spec.serviceAccount.create is false"))

		existing := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: s.namespace, Name: "existing"}}
		Expect(k8sClient.Create(ctx, existing)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(identityServiceAccount(g, s)).To(Equal("existing"))
			g.Expect(readManagementCluster(g, s.mc).Status.ServiceAccountName).To(Equal("existing"))
		}, timeout, interval).Should(Succeed())

		var account corev1.ServiceAccount
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(existing), &account)).To(Succeed())
		Expect(account.OwnerReferences).To(BeEmpty())

		Expect(k8sClient.Delete(ctx, existing)).To(Succeed())
		expectReadyReason(s.mc, v1.ReasonInvalidReference)
	})
})

// identityServiceAccount returns the account that the Management Identity
// pods of s run under.
func identityServiceAccount(g Gomega, s scenario) string {
	var workload appsv1.Deployment
	key := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
	g.Expect(k8sClient.Get(ctx, key, &workload)).To(Succeed())

	return workload.Spec.Template.Spec.ServiceAccountName
}
