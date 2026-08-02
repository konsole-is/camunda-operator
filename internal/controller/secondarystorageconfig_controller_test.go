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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// newSecondaryStorageNamespace creates a uniquely named Namespace for one spec
// and registers its deletion.
func newSecondaryStorageNamespace() string {
	GinkgoHelper()
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "ssc-ns-" + utilrand.String(8)},
	}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })
	return ns.Name
}

// createSecondaryStorageConfig creates secondaryStorage and registers its deletion.
func createSecondaryStorageConfig(secondaryStorage *v1.SecondaryStorageConfig) {
	GinkgoHelper()
	Expect(k8sClient.Create(ctx, secondaryStorage)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, secondaryStorage) })
}

// expectSecondaryStorageReady polls until the named SecondaryStorageConfig's
// Ready condition matches the given status, reason, and message.
func expectSecondaryStorageReady(name string, status metav1.ConditionStatus, reason, message string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var secondaryStorage v1.SecondaryStorageConfig
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, &secondaryStorage)).To(Succeed())
		ready := meta.FindStatusCondition(secondaryStorage.Status.Conditions, conditions.TypeReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Status).To(Equal(status))
		g.Expect(ready.Reason).To(Equal(reason))
		g.Expect(ready.Message).To(Equal(message))
	}, timeout, interval).Should(Succeed())
}

var _ = Describe("SecondaryStorageConfig controller", func() {
	Context("with an elasticsearch contract", func() {
		var (
			secondaryStorage *v1.SecondaryStorageConfig
			secret           *corev1.Secret
		)

		BeforeEach(func() {
			namespace := newSecondaryStorageNamespace()
			secondaryStorage = validSecondaryStorageConfigES()
			secondaryStorage.Spec.Elasticsearch.CredentialsSecretRef.Namespace = namespace
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      secondaryStorage.Spec.Elasticsearch.CredentialsSecretRef.Name,
					Namespace: namespace,
				},
				Data: map[string][]byte{"username": []byte("u"), "password": []byte("p")},
			}
		})

		It("reports MissingSecret while the credentials Secret does not exist", func() {
			createSecondaryStorageConfig(secondaryStorage)

			expectSecondaryStorageReady(
				secondaryStorage.Name, metav1.ConditionFalse, conditions.ReasonMissingSecret,
				fmt.Sprintf("Secret \"%s/%s\" not found", secret.Namespace, secret.Name),
			)
		})

		It("flips to Healthy when the credentials Secret is created", func() {
			createSecondaryStorageConfig(secondaryStorage)
			expectSecondaryStorageReady(
				secondaryStorage.Name, metav1.ConditionFalse, conditions.ReasonMissingSecret,
				fmt.Sprintf("Secret \"%s/%s\" not found", secret.Namespace, secret.Name),
			)

			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			expectSecondaryStorageReady(
				secondaryStorage.Name, metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed",
			)
		})

		It("flips back to MissingSecret when the credentials Secret is deleted", func() {
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			createSecondaryStorageConfig(secondaryStorage)
			expectSecondaryStorageReady(
				secondaryStorage.Name, metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed",
			)

			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())

			expectSecondaryStorageReady(
				secondaryStorage.Name, metav1.ConditionFalse, conditions.ReasonMissingSecret,
				fmt.Sprintf("Secret \"%s/%s\" not found", secret.Namespace, secret.Name),
			)
		})
	})

	Context("with an rdbms contract", func() {
		var (
			secondaryStorage *v1.SecondaryStorageConfig
			database         *v1.DatabaseConfig
		)

		BeforeEach(func() {
			database = validDatabaseConfig()
			secondaryStorage = validSecondaryStorageConfigRDBMS()
			secondaryStorage.Spec.RDBMS.DatabaseConfigRef = database.Name
		})

		It("reports InvalidReference while the DatabaseConfig does not exist", func() {
			createSecondaryStorageConfig(secondaryStorage)

			expectSecondaryStorageReady(
				secondaryStorage.Name, metav1.ConditionFalse, conditions.ReasonInvalidReference,
				fmt.Sprintf("DatabaseConfig %q not found", database.Name),
			)
		})

		It("flips to Healthy when the DatabaseConfig is created", func() {
			createSecondaryStorageConfig(secondaryStorage)
			expectSecondaryStorageReady(
				secondaryStorage.Name, metav1.ConditionFalse, conditions.ReasonInvalidReference,
				fmt.Sprintf("DatabaseConfig %q not found", database.Name),
			)

			Expect(k8sClient.Create(ctx, database)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, database) })

			expectSecondaryStorageReady(
				secondaryStorage.Name, metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed",
			)
		})

		It("flips back to InvalidReference when the DatabaseConfig is deleted", func() {
			Expect(k8sClient.Create(ctx, database)).To(Succeed())
			createSecondaryStorageConfig(secondaryStorage)
			expectSecondaryStorageReady(
				secondaryStorage.Name, metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed",
			)

			Expect(k8sClient.Delete(ctx, database)).To(Succeed())

			expectSecondaryStorageReady(
				secondaryStorage.Name, metav1.ConditionFalse, conditions.ReasonInvalidReference,
				fmt.Sprintf("DatabaseConfig %q not found", database.Name),
			)
		})
	})

	It("re-stamps observedGeneration after a spec update", func() {
		database := validDatabaseConfig()
		Expect(k8sClient.Create(ctx, database)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, database) })

		secondaryStorage := validSecondaryStorageConfigRDBMS()
		secondaryStorage.Spec.RDBMS.DatabaseConfigRef = database.Name
		createSecondaryStorageConfig(secondaryStorage)
		expectSecondaryStorageReady(
			secondaryStorage.Name, metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed",
		)

		other := validDatabaseConfig()
		Expect(k8sClient.Create(ctx, other)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, other) })
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: secondaryStorage.Name}, secondaryStorage)).To(Succeed())
		secondaryStorage.Spec.RDBMS.DatabaseConfigRef = other.Name
		Expect(k8sClient.Update(ctx, secondaryStorage)).To(Succeed())

		Eventually(func(g Gomega) {
			var updated v1.SecondaryStorageConfig
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: secondaryStorage.Name}, &updated)).To(Succeed())
			g.Expect(updated.Status.ObservedGeneration).To(Equal(updated.Generation))
			g.Expect(updated.Generation).To(BeNumerically(">", int64(1)))
		}, timeout, interval).Should(Succeed())
	})
})
