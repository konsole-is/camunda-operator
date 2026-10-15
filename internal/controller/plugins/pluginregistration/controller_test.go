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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

var _ = Describe("PluginRegistration Accepted", func() {
	It("reports SectionMissing until the section exists", func() {
		sectionName := "section-" + utilrand.String(5)
		reg := newRegistration(pluginsv1alpha1.ModeOptional, sectionName)
		createRegistration(reg)

		cond := expectAccepted(reg.Name, metav1.ConditionFalse, pluginsv1alpha1.ReasonSectionMissing)
		Expect(cond.Message).To(ContainSubstring(sectionName))

		createSection(sectionName)

		expectAccepted(reg.Name, metav1.ConditionTrue, v1.ReasonHealthy)
	})

	It("reports SectionMissing again when the section is deleted", func() {
		section := createSection("section-" + utilrand.String(5))
		reg := newRegistration(pluginsv1alpha1.ModeOptional, section.Name)
		createRegistration(reg)
		expectAccepted(reg.Name, metav1.ConditionTrue, v1.ReasonHealthy)

		Expect(k8sClient.Delete(ctx, section)).To(Succeed())

		cond := expectAccepted(reg.Name, metav1.ConditionFalse, pluginsv1alpha1.ReasonSectionMissing)
		Expect(cond.Message).To(ContainSubstring(section.Name))
	})

	It("checks operatorVersions against the release version of the operator", func() {
		section := createSection("section-" + utilrand.String(5))
		reg := newRegistration(pluginsv1alpha1.ModeOptional, section.Name)
		reg.Spec.OperatorVersions = ">=0.4.0 <1.0.0"
		createRegistration(reg)

		cond := expectAccepted(reg.Name, metav1.ConditionFalse, pluginsv1alpha1.ReasonOperatorVersionUnsupported)
		Expect(cond.Message).To(ContainSubstring(operatorVersion))
		Expect(cond.Message).To(ContainSubstring(">=0.4.0 <1.0.0"))

		updateRegistration(reg, func(latest *pluginsv1alpha1.PluginRegistration) {
			latest.Spec.OperatorVersions = ">=0.3.0 <0.4.0"
		})

		expectAccepted(reg.Name, metav1.ConditionTrue, v1.ReasonHealthy)
	})

	It("accepts a registration without operatorVersions on a release version", func() {
		section := createSection("section-" + utilrand.String(5))
		reg := newRegistration(pluginsv1alpha1.ModeOptional, section.Name)
		createRegistration(reg)

		expectAccepted(reg.Name, metav1.ConditionTrue, v1.ReasonHealthy)
	})
})
