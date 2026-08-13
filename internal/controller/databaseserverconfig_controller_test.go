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

var _ = Describe("DatabaseServerConfig controller", func() {
	var (
		namespace    string
		serverConfig *v1.DatabaseServerConfig
	)

	BeforeEach(func() {
		namespace = "dbsc-ns-" + utilrand.String(8)
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })

		serverConfig = validDatabaseServerConfig()
		serverConfig.Spec.AdminCredentialsSecretRef.Namespace = namespace
	})

	// createServerConfig submits the fixture CR and registers its deletion.
	createServerConfig := func() {
		Expect(k8sClient.Create(ctx, serverConfig)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, serverConfig) })
	}

	// adminSecret builds the referenced admin-creds Secret holding the given keys.
	adminSecret := func(keys ...string) *corev1.Secret {
		data := map[string][]byte{}
		for _, key := range keys {
			data[key] = []byte("value")
		}
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serverConfig.Spec.AdminCredentialsSecretRef.Name,
				Namespace: namespace,
			},
			Data: data,
		}
	}

	// expectReady polls until the CR's Ready condition matches exactly.
	expectReady := func(status metav1.ConditionStatus, reason, message string) {
		GinkgoHelper()
		Eventually(func(g Gomega) {
			var got v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: serverConfig.Name}, &got)).To(Succeed())
			cond := meta.FindStatusCondition(got.Status.Conditions, conditions.TypeReady)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(status))
			g.Expect(cond.Reason).To(Equal(reason))
			g.Expect(cond.Message).To(Equal(message))
		}, timeout, interval).Should(Succeed())
	}

	notFoundMessage := func() string {
		return fmt.Sprintf("Secret %q not found", namespace+"/"+serverConfig.Spec.AdminCredentialsSecretRef.Name)
	}

	It("reports MissingSecret when the admin credentials Secret does not exist", func() {
		createServerConfig()

		expectReady(metav1.ConditionFalse, conditions.ReasonMissingSecret, notFoundMessage())
	})

	It("flips to Healthy when the Secret appears, without the CR being touched", func() {
		createServerConfig()
		expectReady(metav1.ConditionFalse, conditions.ReasonMissingSecret, notFoundMessage())

		secret := adminSecret("username", "password")
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		expectReady(metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed")
	})

	It("flips back to MissingSecret when the Secret is deleted", func() {
		secret := adminSecret("username", "password")
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())

		createServerConfig()
		expectReady(metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed")

		Expect(k8sClient.Delete(ctx, secret)).To(Succeed())

		expectReady(metav1.ConditionFalse, conditions.ReasonMissingSecret, notFoundMessage())
	})

	It("reports MissingSecret naming the key when the Secret lacks the password key", func() {
		secret := adminSecret("username")
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		createServerConfig()

		expectReady(metav1.ConditionFalse, conditions.ReasonMissingSecret, fmt.Sprintf(
			"Secret %q is missing key %q",
			namespace+"/"+serverConfig.Spec.AdminCredentialsSecretRef.Name,
			serverConfig.Spec.AdminCredentialsSecretRef.PasswordKey,
		))
	})

	It("catches status.observedGeneration up to metadata.generation after a spec update", func() {
		secret := adminSecret("username", "password")
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		createServerConfig()
		expectReady(metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed")

		var fetched v1.DatabaseServerConfig
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: serverConfig.Name}, &fetched)).To(Succeed())
		fetched.Spec.Host = "replica.camunda-system.svc.cluster.local"
		Expect(k8sClient.Update(ctx, &fetched)).To(Succeed())
		Expect(fetched.Generation).To(BeNumerically(">", 1))

		Eventually(func(g Gomega) {
			var got v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: serverConfig.Name}, &got)).To(Succeed())
			g.Expect(got.Status.ObservedGeneration).To(Equal(fetched.Generation))
		}, timeout, interval).Should(Succeed())
	})
})
