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

var _ = Describe("ObjectStorageConfig controller", func() {
	var storageConfig *v1.ObjectStorageConfig

	// readyCondition fetches the CR, asserts status.observedGeneration has
	// caught up to metadata.generation (failing g until the controller stamps
	// status), and returns the Ready condition.
	readyCondition := func(g Gomega) *metav1.Condition {
		fetched := &v1.ObjectStorageConfig{}
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: storageConfig.Name}, fetched)).To(Succeed())
		g.Expect(fetched.Status.ObservedGeneration).To(Equal(fetched.Generation))
		return meta.FindStatusCondition(fetched.Status.Conditions, conditions.TypeReady)
	}

	BeforeEach(func() {
		storageConfig = validObjectStorageConfig()
		Expect(k8sClient.Create(ctx, storageConfig)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, storageConfig)).To(Succeed()) })
	})

	It("reports an admitted contract Ready and Healthy at its current generation", func() {
		Eventually(func(g Gomega) {
			cond := readyCondition(g)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(cond.Reason).To(Equal(conditions.ReasonHealthy))
			g.Expect(cond.Message).To(Equal("All checks passed"))
			g.Expect(cond.ObservedGeneration).To(Equal(storageConfig.Generation))
		}, timeout, interval).Should(Succeed())
	})

	It("re-stamps observedGeneration after a spec update", func() {
		Eventually(func(g Gomega) {
			g.Expect(readyCondition(g)).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: storageConfig.Name}, storageConfig)).To(Succeed())
		storageConfig.Spec.BasePath = "backups"
		Expect(k8sClient.Update(ctx, storageConfig)).To(Succeed())
		Expect(storageConfig.Generation).To(BeNumerically(">", 1))

		Eventually(func(g Gomega) {
			cond := readyCondition(g)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(cond.Reason).To(Equal(conditions.ReasonHealthy))
			g.Expect(cond.ObservedGeneration).To(Equal(storageConfig.Generation))
		}, timeout, interval).Should(Succeed())
	})
})
