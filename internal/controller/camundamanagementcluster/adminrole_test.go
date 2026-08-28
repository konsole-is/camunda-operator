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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

var _ = Describe("CamundaManagementCluster controller and the Optimize role of the administrator", func() {
	// A management plane that starts before its first Optimize bootstraps its
	// administrator while the realm holds no Optimize role. Management
	// Identity gives the roles of the realm to that user on its very first
	// start and never again, so nothing else puts the role there.
	It("gives the first administrator the Optimize role when the first Optimize arrives", func() {
		keycloak := startFakeKeycloak(nil)
		s := newScenario(withFakeKeycloak(keycloak))

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady).Reason).To(
				Equal(v1.ReasonNoCallbacks),
			)
		}, timeout, interval).Should(Succeed())

		Expect(keycloak.adminRealmRoles()).To(BeEmpty())

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)
		keycloak.runOptimizePreset()

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.adminRealmRoles()).To(ContainElement(optimizeRealmRole))

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(v1.ReasonHealthy))
		}, timeout, interval).Should(Succeed())
	})

	// Nothing watches the realm, so a role that somebody took away there is
	// found by the next converge and by nothing else.
	It("gives the role back after somebody takes it away in Keycloak", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.adminRealmRoles()).To(ContainElement(optimizeRealmRole))
		}, timeout, interval).Should(Succeed())

		keycloak.revokeAdminRealmRoles()

		Eventually(func(g Gomega) {
			g.Expect(keycloak.adminRealmRoles()).To(ContainElement(optimizeRealmRole))
		}, timeout, interval).Should(Succeed())
	})

	// The role is granted once. A converge that finds the administrator
	// holding it writes nothing, so the realm keeps one mapping and not one
	// for every reconcile.
	It("grants the role once and leaves it alone after that", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.adminRealmRoles()).To(Equal([]string{optimizeRealmRole}))
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(keycloak.adminRealmRoles()).To(Equal([]string{optimizeRealmRole}))
		}, "3s", interval).Should(Succeed())
	})

	// A group of the realm carries the role to its members. The administrator
	// holds it already, so a direct mapping of the same role adds nothing and
	// the operator writes none.
	It("leaves the realm alone while the administrator holds the role through a group", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		keycloak.putAdminInAnOptimizeGroup()
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(v1.ReasonHealthy))
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(keycloak.adminRealmRoles()).To(BeEmpty())
		}, "3s", interval).Should(Succeed())
	})

	// Only Management Identity creates the first administrator. A realm
	// without that user is one somebody edited, and the operator says so
	// rather than granting the role to nobody.
	It("reports AdminRoleGrantFailed while the realm holds no such user", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		keycloak.removeAdminUser()
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(v1.ReasonAdminRoleGrantFailed))
			g.Expect(condition.Message).To(ContainSubstring(adminUsername))

			ready := conditionOf(g, s.mc, v1.ConditionReady)
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(v1.ReasonAdminRoleGrantFailed))
		}, timeout, interval).Should(Succeed())
	})

	// The realm holds the Optimize client, so Management Identity ran the
	// preset and the role went with it. A realm without the role is one
	// somebody edited.
	It("reports AdminRoleGrantFailed while the realm holds no Optimize role", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		keycloak.removeRealmRole(optimizeRealmRole)
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(v1.ReasonAdminRoleGrantFailed))
			g.Expect(condition.Message).To(ContainSubstring(optimizeRealmRole))
		}, timeout, interval).Should(Succeed())
	})
})
