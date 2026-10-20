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

package builtin

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

var _ = Describe("Built-in requests and the Ready state of their registration", func() {
	const registration = pluginstartup.RegistrationExistingSecondaryStorage

	BeforeEach(func() {
		DeferCleanup(setRegistrationReady, registration, metav1.ConditionTrue)
	})

	It("serves a request only once its registration is Ready", func() {
		setRegistrationReady(registration, metav1.ConditionFalse)
		contract := newSecondaryStorageConfig()
		createContract(contract)
		request := newRequest(registration, `{"name":"`+contract.Name+`"}`, pluginsv1alpha1.DeletionPolicyDelete)

		expectProbeServed()
		Consistently(func(g Gomega) {
			live := liveRequest(g, request)
			g.Expect(live.Finalizers).To(BeEmpty())
			g.Expect(live.Status.Conditions).To(BeEmpty())
			g.Expect(live.Status.Outputs).To(BeEmpty())
		}, 2*time.Second, interval).Should(Succeed())

		setRegistrationReady(registration, metav1.ConditionTrue)

		live := expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionTrue, v1.ReasonHealthy)
		Expect(live.Finalizers).To(ContainElement(pluginsv1alpha1.FinalizerCleanup))
		Expect(live.Status.Outputs).To(Equal(secondaryOutput(contract.Name)))
	})

	It("holds the status of a served request while Ready is behind the generation of the registration", func() {
		first, second := newSecondaryStorageConfig(), newSecondaryStorageConfig()
		createContract(first)
		createContract(second)
		request := newRequest(registration, `{"name":"`+first.Name+`"}`, pluginsv1alpha1.DeletionPolicyDelete)
		served := expectCondition(
			request,
			pluginsv1alpha1.ConditionOutputsReady,
			metav1.ConditionTrue,
			v1.ReasonHealthy,
		)

		bumpRegistration(registration)
		Eventually(func() error {
			live := &pluginsv1alpha1.PluginRequest{}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(request), live); err != nil {
				return err
			}
			live.Spec.Parameters = &apiextensionsv1.JSON{Raw: []byte(`{"name":"` + second.Name + `"}`)}
			return k8sClient.Update(ctx, live)
		}, timeout, interval).Should(Succeed())

		expectProbeServed()
		held := liveRequest(Default, request)
		Expect(held.Generation).To(BeNumerically(">", served.Generation))
		outputsReady := meta.FindStatusCondition(held.Status.Conditions, pluginsv1alpha1.ConditionOutputsReady)
		Expect(outputsReady.ObservedGeneration).To(Equal(served.Generation))
		Expect(held.Status.Outputs).To(Equal(secondaryOutput(first.Name)))
		Consistently(func(g Gomega) {
			g.Expect(liveRequest(g, request).Status).To(Equal(held.Status))
		}, 2*time.Second, interval).Should(Succeed())

		setRegistrationReady(registration, metav1.ConditionTrue)

		live := expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionTrue, v1.ReasonHealthy)
		Expect(live.Status.Outputs).To(Equal(secondaryOutput(second.Name)))
	})

	It("resumes a served request whose label changed once its registration is Ready", func() {
		first, second := newSecondaryStorageConfig(), newSecondaryStorageConfig()
		createContract(first)
		createContract(second)
		request := newRequest(registration, `{"name":"`+first.Name+`"}`, pluginsv1alpha1.DeletionPolicyDelete)
		expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionTrue, v1.ReasonHealthy)

		setRegistrationReady(registration, metav1.ConditionFalse)
		Eventually(func() error {
			live := &pluginsv1alpha1.PluginRequest{}
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(request), live); err != nil {
				return err
			}
			live.Labels[pluginsv1alpha1.LabelRegistration] = "s3-backup-bucket"
			live.Spec.Parameters = &apiextensionsv1.JSON{Raw: []byte(`{"name":"` + second.Name + `"}`)}
			return k8sClient.Update(ctx, live)
		}, timeout, interval).Should(Succeed())
		expectProbeServed()
		Expect(liveRequest(Default, request).Status.Outputs).To(Equal(secondaryOutput(first.Name)))

		setRegistrationReady(registration, metav1.ConditionTrue)

		live := expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionTrue, v1.ReasonHealthy)
		Expect(live.Status.Outputs).To(Equal(secondaryOutput(second.Name)))
	})

	It("finishes the deletion of a served request while its registration is not Ready", func() {
		contract := newSecondaryStorageConfig()
		createContract(contract)
		request := newRequest(registration, `{"name":"`+contract.Name+`"}`, pluginsv1alpha1.DeletionPolicyDelete)
		expectCondition(request, pluginsv1alpha1.ConditionOutputsReady, metav1.ConditionTrue, v1.ReasonHealthy)

		setRegistrationReady(registration, metav1.ConditionFalse)
		Expect(k8sClient.Delete(ctx, request)).To(Succeed())

		expectGone(request)
	})
})

// bumpRegistration changes the description of the registration name, which
// raises its generation.
func bumpRegistration(name string) {
	GinkgoHelper()
	Eventually(func() error {
		var reg pluginsv1alpha1.PluginRegistration
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: name}, &reg); err != nil {
			return err
		}
		reg.Spec.Description += " Changed by the spec."
		return k8sClient.Update(ctx, &reg)
	}, timeout, interval).Should(Succeed())
}

// secondaryOutput returns the outputs of an existing-secondary-storage
// request for the contract name.
func secondaryOutput(name string) map[string]string {
	return map[string]string{pluginstartup.OutputSecondaryStorageConfig: name}
}
