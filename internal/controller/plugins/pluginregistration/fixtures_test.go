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

package pluginregistration

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// newRegistration returns a valid registration with a random name that
// serves sections in mode.
func newRegistration(mode pluginsv1alpha1.Mode, sections ...string) *pluginsv1alpha1.PluginRegistration {
	return &pluginsv1alpha1.PluginRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: "registration-" + utilrand.String(5)},
		Spec: pluginsv1alpha1.PluginRegistrationSpec{
			Sections:         sections,
			Label:            "Test option",
			Mode:             mode,
			ServiceAccount:   pluginsv1alpha1.ServiceAccountRef{Namespace: "camunda-plugins", Name: "test-controller"},
			DeletionPolicies: []pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete},
		},
	}
}

// createRegistration creates reg and registers its deletion.
func createRegistration(reg *pluginsv1alpha1.PluginRegistration) {
	GinkgoHelper()
	Expect(k8sClient.Create(ctx, reg)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, reg) })
}

// updateRegistration applies change to the live copy of reg, retrying on a
// conflict.
func updateRegistration(reg *pluginsv1alpha1.PluginRegistration, change func(*pluginsv1alpha1.PluginRegistration)) {
	GinkgoHelper()
	Eventually(func() error {
		var latest pluginsv1alpha1.PluginRegistration
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(reg), &latest); err != nil {
			return err
		}
		change(&latest)
		return k8sClient.Update(ctx, &latest)
	}, timeout, interval).Should(Succeed())
}

// createSection creates a section named name and registers its deletion.
func createSection(name string) *pluginsv1alpha1.PluginSection {
	GinkgoHelper()
	section := &pluginsv1alpha1.PluginSection{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: pluginsv1alpha1.PluginSectionSpec{
			Label:     "Test section",
			Selection: pluginsv1alpha1.SelectionOne,
			AppliesTo: []pluginsv1alpha1.ClaimKind{pluginsv1alpha1.ClaimKindCamundaClusterClaim},
		},
	}
	Expect(k8sClient.Create(ctx, section)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, section) })
	return section
}

// acceptedOf reads the live Accepted condition of the registration name.
func acceptedOf(g Gomega, name string) *metav1.Condition {
	var reg pluginsv1alpha1.PluginRegistration
	g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name}, &reg)).To(Succeed())
	return meta.FindStatusCondition(reg.Status.Conditions, pluginsv1alpha1.ConditionAccepted)
}

// expectAccepted waits until the registration name reports Accepted with
// status and reason, at its current generation, and returns the condition.
func expectAccepted(name string, status metav1.ConditionStatus, reason string) *metav1.Condition {
	GinkgoHelper()
	var cond *metav1.Condition
	Eventually(func(g Gomega) {
		var reg pluginsv1alpha1.PluginRegistration
		g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name}, &reg)).To(Succeed())
		cond = meta.FindStatusCondition(reg.Status.Conditions, pluginsv1alpha1.ConditionAccepted)
		g.Expect(cond).NotTo(BeNil())
		g.Expect(cond.Status).To(Equal(status))
		g.Expect(cond.Reason).To(Equal(reason))
		g.Expect(cond.ObservedGeneration).To(Equal(reg.Generation))
	}, timeout, interval).Should(Succeed())
	return cond
}
