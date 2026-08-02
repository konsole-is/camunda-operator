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

package controller

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// This spec proves conditions.PatchReady's server-side-apply path against a
// real API server; needsPatch and Ready are covered by unit tests in
// pkg/conditions.
var _ = Describe("conditions.PatchReady", func() {
	fetchReady := func(name string) *metav1.Condition {
		var fetched v1.DatabaseServerConfig
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, &fetched)).To(Succeed())
		return meta.FindStatusCondition(fetched.Status.Conditions, conditions.TypeReady)
	}

	It("applies the Ready condition and observedGeneration to the status subresource", func() {
		resource := validDatabaseServerConfig()
		Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, resource)).To(Succeed()) })

		cond := conditions.Ready(
			metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed", resource.Generation,
		)
		Expect(conditions.PatchReady(ctx, k8sClient, resource, cond)).To(Succeed())

		Eventually(func(g Gomega) {
			var fetched v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resource.Name}, &fetched)).To(Succeed())
			g.Expect(fetched.Status.ObservedGeneration).To(Equal(resource.Generation))

			ready := meta.FindStatusCondition(fetched.Status.Conditions, conditions.TypeReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(ready.Reason).To(Equal(conditions.ReasonHealthy))
			g.Expect(ready.Message).To(Equal("All checks passed"))
			g.Expect(ready.ObservedGeneration).To(Equal(resource.Generation))
			g.Expect(ready.LastTransitionTime.IsZero()).To(BeFalse())
		}, timeout, interval).Should(Succeed())
	})

	It("preserves LastTransitionTime when the condition status is unchanged", func() {
		resource := validDatabaseServerConfig()
		Expect(k8sClient.Create(ctx, resource)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, resource)).To(Succeed()) })

		Expect(conditions.PatchReady(ctx, k8sClient, resource, conditions.Ready(
			metav1.ConditionFalse, conditions.ReasonMissingSecret, `Secret "ns/s" not found`, resource.Generation,
		))).To(Succeed())
		transitioned := fetchReady(resource.Name).LastTransitionTime

		var persisted v1.DatabaseServerConfig
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: resource.Name}, &persisted)).To(Succeed())
		Expect(conditions.PatchReady(ctx, k8sClient, &persisted, conditions.Ready(
			metav1.ConditionFalse, conditions.ReasonMissingSecret, `Secret "ns/s" is missing key "k"`, persisted.Generation,
		))).To(Succeed())

		ready := fetchReady(resource.Name)
		Expect(ready.Message).To(Equal(`Secret "ns/s" is missing key "k"`))
		Expect(ready.LastTransitionTime).To(Equal(transitioned))
	})
})
