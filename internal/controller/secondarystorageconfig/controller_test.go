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

package secondarystorageconfig

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
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

// expectSecondaryStorageReady polls until the Ready condition of
// secondaryStorage matches the given status, reason, and message.
func expectSecondaryStorageReady(
	secondaryStorage *v1.SecondaryStorageConfig,
	status metav1.ConditionStatus,
	reason, message string,
) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.SecondaryStorageConfig
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(secondaryStorage), &latest)).To(Succeed())
		ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
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
			secondaryStorage.Namespace = namespace
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
				secondaryStorage, metav1.ConditionFalse, v1.ReasonMissingSecret,
				fmt.Sprintf("Secret %s/%s not found", secret.Namespace, secret.Name),
			)
		})

		It("flips to Healthy when the credentials Secret is created", func() {
			createSecondaryStorageConfig(secondaryStorage)
			expectSecondaryStorageReady(
				secondaryStorage, metav1.ConditionFalse, v1.ReasonMissingSecret,
				fmt.Sprintf("Secret %s/%s not found", secret.Namespace, secret.Name),
			)

			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			expectSecondaryStorageReady(
				secondaryStorage, metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed",
			)
		})

		It("validates the CA Secret when caSecretRef is set", func() {
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			secondaryStorage.Spec.Elasticsearch.CASecretRef = &v1.LocalSecretKeyRef{
				Name: "es-ca", Key: "ca.crt",
			}
			createSecondaryStorageConfig(secondaryStorage)

			expectSecondaryStorageReady(
				secondaryStorage, metav1.ConditionFalse, v1.ReasonMissingSecret,
				fmt.Sprintf("Secret %s/es-ca not found", secret.Namespace),
			)

			caSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{Name: "es-ca", Namespace: secret.Namespace},
				Data:       map[string][]byte{"ca.crt": []byte("pem")},
			}
			Expect(k8sClient.Create(ctx, caSecret)).To(Succeed())

			expectSecondaryStorageReady(
				secondaryStorage, metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed",
			)
		})

		It("flips back to MissingSecret when the credentials Secret is deleted", func() {
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			createSecondaryStorageConfig(secondaryStorage)
			expectSecondaryStorageReady(
				secondaryStorage, metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed",
			)

			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())

			expectSecondaryStorageReady(
				secondaryStorage, metav1.ConditionFalse, v1.ReasonMissingSecret,
				fmt.Sprintf("Secret %s/%s not found", secret.Namespace, secret.Name),
			)
		})
	})

	Context("with an rdbms contract", func() {
		var (
			secondaryStorage *v1.SecondaryStorageConfig
			database         *v1.DatabaseConfig
		)

		BeforeEach(func() {
			namespace := newSecondaryStorageNamespace()
			database = fixtures.DatabaseConfig()
			database.Namespace = namespace
			secondaryStorage = validSecondaryStorageConfigRDBMS()
			secondaryStorage.Namespace = namespace
			secondaryStorage.Spec.RDBMS.DatabaseConfigRef = database.Name
		})

		It("reports InvalidReference while the DatabaseConfig does not exist", func() {
			createSecondaryStorageConfig(secondaryStorage)

			expectSecondaryStorageReady(
				secondaryStorage, metav1.ConditionFalse, v1.ReasonInvalidReference,
				fmt.Sprintf("DatabaseConfig %q not found", database.Name),
			)
		})

		It("flips to Healthy when the DatabaseConfig is created", func() {
			createSecondaryStorageConfig(secondaryStorage)
			expectSecondaryStorageReady(
				secondaryStorage, metav1.ConditionFalse, v1.ReasonInvalidReference,
				fmt.Sprintf("DatabaseConfig %q not found", database.Name),
			)

			Expect(k8sClient.Create(ctx, database)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, database) })

			expectSecondaryStorageReady(
				secondaryStorage, metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed",
			)
		})

		It("flips back to InvalidReference when the DatabaseConfig is deleted", func() {
			Expect(k8sClient.Create(ctx, database)).To(Succeed())
			createSecondaryStorageConfig(secondaryStorage)
			expectSecondaryStorageReady(
				secondaryStorage, metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed",
			)

			Expect(k8sClient.Delete(ctx, database)).To(Succeed())

			expectSecondaryStorageReady(
				secondaryStorage, metav1.ConditionFalse, v1.ReasonInvalidReference,
				fmt.Sprintf("DatabaseConfig %q not found", database.Name),
			)
		})

		It("is not satisfied by a same-named DatabaseConfig in another namespace", func() {
			otherNamespace := newSecondaryStorageNamespace()
			elsewhere := fixtures.DatabaseConfig()
			elsewhere.Name = database.Name
			elsewhere.Namespace = otherNamespace
			Expect(k8sClient.Create(ctx, elsewhere)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, elsewhere) })

			createSecondaryStorageConfig(secondaryStorage)

			expectSecondaryStorageReady(
				secondaryStorage, metav1.ConditionFalse, v1.ReasonInvalidReference,
				fmt.Sprintf("DatabaseConfig %q not found", database.Name),
			)

			Expect(k8sClient.Create(ctx, database)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, database) })

			expectSecondaryStorageReady(
				secondaryStorage, metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed",
			)
		})
	})

	It("re-stamps observedGeneration after a spec update", func() {
		namespace := newSecondaryStorageNamespace()
		database := fixtures.DatabaseConfig()
		database.Namespace = namespace
		Expect(k8sClient.Create(ctx, database)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, database) })

		secondaryStorage := validSecondaryStorageConfigRDBMS()
		secondaryStorage.Namespace = namespace
		secondaryStorage.Spec.RDBMS.DatabaseConfigRef = database.Name
		createSecondaryStorageConfig(secondaryStorage)
		expectSecondaryStorageReady(
			secondaryStorage, metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed",
		)

		other := fixtures.DatabaseConfig()
		other.Namespace = namespace
		Expect(k8sClient.Create(ctx, other)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, other) })
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(secondaryStorage), secondaryStorage)).To(Succeed())
		secondaryStorage.Spec.RDBMS.DatabaseConfigRef = other.Name
		Expect(k8sClient.Update(ctx, secondaryStorage)).To(Succeed())

		Eventually(func(g Gomega) {
			var updated v1.SecondaryStorageConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(secondaryStorage), &updated)).To(Succeed())
			g.Expect(updated.Status.ObservedGeneration).To(Equal(updated.Generation))
			g.Expect(updated.Generation).To(BeNumerically(">", int64(1)))
		}, timeout, interval).Should(Succeed())
	})
})
