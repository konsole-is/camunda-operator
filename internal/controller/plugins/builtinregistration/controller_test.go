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

package builtinregistration

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

var _ = Describe("Built-in registration controller", func() {
	It("serves at least one built-in registration", func() {
		Expect(builtin.Served()).NotTo(BeEmpty())
	})

	It("marks each served built-in registration Accepted and Ready at its generation", func() {
		for _, name := range builtin.Served() {
			Eventually(func(g Gomega) {
				reg := get(g, name)

				g.Expect(reg.Status.ObservedGeneration).To(Equal(reg.Generation))
				expectTrue(g, &reg, pluginsv1alpha1.ConditionAccepted)
				ready := expectTrue(g, &reg, v1.ConditionReady)
				g.Expect(ready.Reason).To(Equal(v1.ReasonHealthy))
			}, timeout, interval).Should(Succeed(), "registration %s", name)
		}
	})

	It("writes no Ready on a built-in registration that the operator does not serve", func() {
		Expect(builtin.Served()).NotTo(ContainElement(pluginstartup.RegistrationDatabaseServer))

		Eventually(func(g Gomega) {
			reg := get(g, pluginstartup.RegistrationDatabaseServer)
			expectTrue(g, &reg, pluginsv1alpha1.ConditionAccepted)
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			reg := get(g, pluginstartup.RegistrationDatabaseServer)
			g.Expect(meta.FindStatusCondition(reg.Status.Conditions, v1.ConditionReady)).To(BeNil())
			g.Expect(reg.Status.ObservedGeneration).To(BeZero())
		}, 3*time.Second, interval).Should(Succeed())
	})

	It("writes Ready again after it is removed", func() {
		name := pluginstartup.RegistrationExistingSecondaryStorage
		eventuallyReady(name)

		// The removal lands only on the copy that the server holds now, so a
		// Ready read after it comes from a reconcile that read the removal.
		var removedAt string
		Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
			var reg pluginsv1alpha1.PluginRegistration
			if err := k8sClient.Get(ctx, client.ObjectKey{Name: name}, &reg); err != nil {
				return err
			}
			if !meta.RemoveStatusCondition(&reg.Status.Conditions, v1.ConditionReady) {
				return nil
			}
			if err := k8sClient.Status().Update(ctx, &reg); err != nil {
				return err
			}
			removedAt = reg.ResourceVersion
			return nil
		})).To(Succeed())
		Expect(removedAt).NotTo(BeEmpty(), "the spec removed no Ready")

		Eventually(func(g Gomega) {
			reg := get(g, name)
			g.Expect(reg.ResourceVersion).NotTo(Equal(removedAt))
			ready := expectTrue(g, &reg, v1.ConditionReady)
			g.Expect(ready.ObservedGeneration).To(Equal(reg.Generation))
		}, timeout, interval).Should(Succeed())
	})

	It("writes Ready at a new generation and keeps the Accepted of the registry controller", func() {
		name := pluginstartup.RegistrationExistingDatabaseConfigs
		eventuallyReady(name)

		reg := get(Default, name)
		before := reg.Generation
		original := reg.Spec.OperatorVersions
		setOperatorVersions(name, ">=1.0.0")
		DeferCleanup(setOperatorVersions, name, original)

		Eventually(func(g Gomega) {
			reg := get(g, name)
			g.Expect(reg.Generation).To(BeNumerically(">", before))
			g.Expect(reg.Status.ObservedGeneration).To(Equal(reg.Generation))

			accepted := meta.FindStatusCondition(reg.Status.Conditions, pluginsv1alpha1.ConditionAccepted)
			g.Expect(accepted).NotTo(BeNil())
			g.Expect(accepted.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(accepted.Reason).To(Equal(pluginsv1alpha1.ReasonOperatorVersionUnsupported))
			g.Expect(accepted.ObservedGeneration).To(Equal(reg.Generation))

			ready := expectTrue(g, &reg, v1.ConditionReady)
			g.Expect(ready.ObservedGeneration).To(Equal(reg.Generation))
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			reg := get(g, name)
			accepted := meta.FindStatusCondition(reg.Status.Conditions, pluginsv1alpha1.ConditionAccepted)
			g.Expect(accepted).NotTo(BeNil())
			g.Expect(accepted.Reason).To(Equal(pluginsv1alpha1.ReasonOperatorVersionUnsupported))
		}, 3*time.Second, interval).Should(Succeed())
	})
})

func get(g Gomega, name string) pluginsv1alpha1.PluginRegistration {
	var reg pluginsv1alpha1.PluginRegistration
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name}, &reg)).To(Succeed())
	return reg
}

func expectTrue(g Gomega, reg *pluginsv1alpha1.PluginRegistration, condType string) *metav1.Condition {
	cond := meta.FindStatusCondition(reg.Status.Conditions, condType)
	g.Expect(cond).NotTo(BeNil(), "condition %s", condType)
	g.Expect(cond.Status).To(Equal(metav1.ConditionTrue), "condition %s: %s", condType, cond.Message)
	return cond
}

func eventuallyReady(name string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		reg := get(g, name)
		expectTrue(g, &reg, v1.ConditionReady)
	}, timeout, interval).Should(Succeed())
}

func setOperatorVersions(name, versions string) {
	GinkgoHelper()

	Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var reg pluginsv1alpha1.PluginRegistration
		if err := k8sClient.Get(ctx, client.ObjectKey{Name: name}, &reg); err != nil {
			return err
		}
		reg.Spec.OperatorVersions = versions
		return k8sClient.Update(ctx, &reg)
	})).To(Succeed())
}
