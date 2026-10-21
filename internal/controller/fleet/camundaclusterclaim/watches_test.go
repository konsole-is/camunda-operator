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

package camundaclusterclaim

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

var _ = Describe("CamundaClusterClaim watches", func() {
	It("leaves NoEnvironment when the namespace gets its label", func() {
		environment := allowingEnvironment(map[string][]string{
			pluginstartup.SectionSecondaryStorage: {pluginstartup.RegistrationExistingSecondaryStorage},
		})
		createEnvironment(environment)
		claim := newClaim(newNamespace(nil))
		create(claim)
		expectCondition(claim, v1.ConditionReady, metav1.ConditionFalse, fleetv1alpha1.ReasonNoEnvironment, Not(BeEmpty()))

		var namespace corev1.Namespace
		Expect(k8sClient.Get(ctx, client.ObjectKey{Name: claim.Namespace}, &namespace)).To(Succeed())
		namespace.Labels = map[string]string{fleetv1alpha1.LabelEnvironment: environment.Name}
		Expect(k8sClient.Update(ctx, &namespace)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(latest(g, claim).Status.ResolvedPolicy).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
	})

	It("leaves InvalidReference when the preset appears", func() {
		claim := newClaim(newNamespace(nil))
		claim.Spec.PresetRef = uniqueName("late-preset")
		create(claim)
		expectCondition(claim, v1.ConditionReady, metav1.ConditionFalse, v1.ReasonInvalidReference, Not(BeEmpty()))

		create(&v1.CamundaClusterPreset{ObjectMeta: metav1.ObjectMeta{Name: claim.Spec.PresetRef}})

		Eventually(func(g Gomega) {
			g.Expect(latest(g, claim).Status.ResolvedPolicy).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
	})
})
