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

package managementauthconfig

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
)

var _ = Describe("ManagementAuthConfig controller", func() {
	var (
		namespace  string
		authConfig *v1.ManagementAuthConfig
	)

	BeforeEach(func() {
		namespace = "mac-ns-" + utilrand.String(8)
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })

		authConfig = validManagementAuthConfig()
		authConfig.Spec.ClientSecretRef.Namespace = namespace
		Expect(k8sClient.Create(ctx, authConfig)).To(Succeed())
		DeferCleanup(func() { Expect(k8sClient.Delete(ctx, authConfig)).To(Succeed()) })
	})

	// expectReady polls until the Ready condition and status.observedGeneration
	// match the expectation.
	expectReady := func(status metav1.ConditionStatus, reason, message string) {
		GinkgoHelper()
		Eventually(func(g Gomega) {
			var current v1.ManagementAuthConfig
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: authConfig.Name}, &current)).To(Succeed())
			cond := meta.FindStatusCondition(current.Status.Conditions, v1.ConditionReady)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(status))
			g.Expect(cond.Reason).To(Equal(reason))
			g.Expect(cond.Message).To(Equal(message))
			g.Expect(current.Status.ObservedGeneration).To(Equal(current.Generation))
		}, timeout, interval).Should(Succeed())
	}

	// createClientSecret creates the Secret that the clientSecretRef of the CR
	// names, with the configured key present.
	createClientSecret := func() *corev1.Secret {
		GinkgoHelper()
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      authConfig.Spec.ClientSecretRef.Name,
				Namespace: namespace,
			},
			Data: map[string][]byte{authConfig.Spec.ClientSecretRef.Key: []byte("s3cr3t")},
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })
		return secret
	}

	notFoundMessage := func() string {
		return fmt.Sprintf("Secret %s not found", namespace+"/"+authConfig.Spec.ClientSecretRef.Name)
	}

	It("reports MissingSecret when the referenced Secret does not exist", func() {
		expectReady(metav1.ConditionFalse, v1.ReasonMissingSecret, notFoundMessage())
	})

	It("flips to Healthy when the Secret with the configured key appears, without touching the CR", func() {
		expectReady(metav1.ConditionFalse, v1.ReasonMissingSecret, notFoundMessage())

		createClientSecret()

		expectReady(metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed")
	})

	It("flips back to MissingSecret naming the key when the key is removed from the Secret", func() {
		secret := createClientSecret()
		expectReady(metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed")

		secret.Data = map[string][]byte{"unrelated": []byte("x")}
		Expect(k8sClient.Update(ctx, secret)).To(Succeed())

		expectReady(
			metav1.ConditionFalse,
			v1.ReasonMissingSecret,
			fmt.Sprintf(
				"Secret %s is missing key %q",
				namespace+"/"+authConfig.Spec.ClientSecretRef.Name, authConfig.Spec.ClientSecretRef.Key,
			),
		)
	})

	It("flips back to MissingSecret when the Secret is deleted", func() {
		secret := createClientSecret()
		expectReady(metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed")

		Expect(k8sClient.Delete(ctx, secret)).To(Succeed())

		expectReady(metav1.ConditionFalse, v1.ReasonMissingSecret, notFoundMessage())
	})

	It("keeps status.observedGeneration in step with spec updates", func() {
		createClientSecret()
		expectReady(metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed")

		var current v1.ManagementAuthConfig
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: authConfig.Name}, &current)).To(Succeed())
		current.Spec.Audience = "camunda-management-updated"
		Expect(k8sClient.Update(ctx, &current)).To(Succeed())
		Expect(current.Generation).To(BeNumerically(">", int64(1)))

		expectReady(metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed")
	})
})
