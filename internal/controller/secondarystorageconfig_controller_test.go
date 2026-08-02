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
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "ssc-ns-" + utilrand.String(8)},
	}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })
	return ns.Name
}

// createSecondaryStorageConfig creates cfg and registers its deletion.
func createSecondaryStorageConfig(cfg *v1.SecondaryStorageConfig) {
	Expect(k8sClient.Create(ctx, cfg)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, cfg) })
}

// expectSecondaryStorageReady polls until cfg's Ready condition matches the
// given status, reason, and message.
func expectSecondaryStorageReady(name string, status metav1.ConditionStatus, reason, message string) {
	Eventually(func(g Gomega) {
		var cfg v1.SecondaryStorageConfig
		g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name}, &cfg)).To(Succeed())
		ready := meta.FindStatusCondition(cfg.Status.Conditions, conditions.TypeReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Status).To(Equal(status))
		g.Expect(ready.Reason).To(Equal(reason))
		g.Expect(ready.Message).To(Equal(message))
	}, timeout, interval).Should(Succeed())
}

var _ = Describe("SecondaryStorageConfig controller", func() {
	Context("with an elasticsearch contract", func() {
		var (
			cfg    *v1.SecondaryStorageConfig
			secret *corev1.Secret
		)

		BeforeEach(func() {
			namespace := newSecondaryStorageNamespace()
			cfg = validSecondaryStorageConfigES()
			cfg.Spec.Elasticsearch.CredentialsSecretRef.Namespace = namespace
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      cfg.Spec.Elasticsearch.CredentialsSecretRef.Name,
					Namespace: namespace,
				},
				Data: map[string][]byte{"username": []byte("u"), "password": []byte("p")},
			}
		})

		It("reports MissingSecret while the credentials Secret does not exist", func() {
			createSecondaryStorageConfig(cfg)

			expectSecondaryStorageReady(
				cfg.Name, metav1.ConditionFalse, conditions.ReasonMissingSecret,
				fmt.Sprintf("Secret \"%s/%s\" not found", secret.Namespace, secret.Name),
			)
		})

		It("flips to Healthy when the credentials Secret is created", func() {
			createSecondaryStorageConfig(cfg)
			expectSecondaryStorageReady(
				cfg.Name, metav1.ConditionFalse, conditions.ReasonMissingSecret,
				fmt.Sprintf("Secret \"%s/%s\" not found", secret.Namespace, secret.Name),
			)

			Expect(k8sClient.Create(ctx, secret)).To(Succeed())

			expectSecondaryStorageReady(
				cfg.Name, metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed",
			)
		})

		It("flips back to MissingSecret when the credentials Secret is deleted", func() {
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			createSecondaryStorageConfig(cfg)
			expectSecondaryStorageReady(
				cfg.Name, metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed",
			)

			Expect(k8sClient.Delete(ctx, secret)).To(Succeed())

			expectSecondaryStorageReady(
				cfg.Name, metav1.ConditionFalse, conditions.ReasonMissingSecret,
				fmt.Sprintf("Secret \"%s/%s\" not found", secret.Namespace, secret.Name),
			)
		})
	})

	Context("with an rdbms contract", func() {
		var (
			cfg      *v1.SecondaryStorageConfig
			database *v1.DatabaseConfig
		)

		BeforeEach(func() {
			database = validDatabaseConfig()
			cfg = validSecondaryStorageConfigRDBMS()
			cfg.Spec.RDBMS.DatabaseConfigRef = database.Name
		})

		It("reports InvalidReference while the DatabaseConfig does not exist", func() {
			createSecondaryStorageConfig(cfg)

			expectSecondaryStorageReady(
				cfg.Name, metav1.ConditionFalse, conditions.ReasonInvalidReference,
				fmt.Sprintf("DatabaseConfig %q not found", database.Name),
			)
		})

		It("flips to Healthy when the DatabaseConfig is created", func() {
			createSecondaryStorageConfig(cfg)
			expectSecondaryStorageReady(
				cfg.Name, metav1.ConditionFalse, conditions.ReasonInvalidReference,
				fmt.Sprintf("DatabaseConfig %q not found", database.Name),
			)

			Expect(k8sClient.Create(ctx, database)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, database) })

			expectSecondaryStorageReady(
				cfg.Name, metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed",
			)
		})

		It("flips back to InvalidReference when the DatabaseConfig is deleted", func() {
			Expect(k8sClient.Create(ctx, database)).To(Succeed())
			createSecondaryStorageConfig(cfg)
			expectSecondaryStorageReady(
				cfg.Name, metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed",
			)

			Expect(k8sClient.Delete(ctx, database)).To(Succeed())

			expectSecondaryStorageReady(
				cfg.Name, metav1.ConditionFalse, conditions.ReasonInvalidReference,
				fmt.Sprintf("DatabaseConfig %q not found", database.Name),
			)
		})
	})

	It("re-stamps observedGeneration after a spec update", func() {
		database := validDatabaseConfig()
		Expect(k8sClient.Create(ctx, database)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, database) })

		cfg := validSecondaryStorageConfigRDBMS()
		cfg.Spec.RDBMS.DatabaseConfigRef = database.Name
		createSecondaryStorageConfig(cfg)
		expectSecondaryStorageReady(
			cfg.Name, metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed",
		)

		other := validDatabaseConfig()
		Expect(k8sClient.Create(ctx, other)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, other) })
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: cfg.Name}, cfg)).To(Succeed())
		cfg.Spec.RDBMS.DatabaseConfigRef = other.Name
		Expect(k8sClient.Update(ctx, cfg)).To(Succeed())

		Eventually(func(g Gomega) {
			var updated v1.SecondaryStorageConfig
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: cfg.Name}, &updated)).To(Succeed())
			g.Expect(updated.Status.ObservedGeneration).To(Equal(updated.Generation))
			g.Expect(updated.Generation).To(BeNumerically(">", int64(1)))
		}, timeout, interval).Should(Succeed())
	})
})
