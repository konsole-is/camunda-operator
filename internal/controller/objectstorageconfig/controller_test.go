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

package objectstorageconfig

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

var _ = Describe("ObjectStorageConfig controller", func() {
	var storageConfig *v1.ObjectStorageConfig

	// readyCondition fetches the CR and returns its Ready condition. It fails
	// g until status.observedGeneration matches metadata.generation.
	readyCondition := func(g Gomega) *metav1.Condition {
		fetched := &v1.ObjectStorageConfig{}
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(storageConfig), fetched)).To(Succeed())
		g.Expect(fetched.Status.ObservedGeneration).To(Equal(fetched.Generation))
		return meta.FindStatusCondition(fetched.Status.Conditions, v1.ConditionReady)
	}

	expectReady := func(status metav1.ConditionStatus, reason string) {
		GinkgoHelper()
		Eventually(func(g Gomega) {
			cond := readyCondition(g)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(status))
			g.Expect(cond.Reason).To(Equal(reason))
		}, timeout, interval).Should(Succeed())
	}

	Context("with workload identity", func() {
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
				g.Expect(cond.Reason).To(Equal(v1.ReasonHealthy))
				g.Expect(cond.Message).To(Equal("All checks passed"))
				g.Expect(cond.ObservedGeneration).To(Equal(storageConfig.Generation))
			}, timeout, interval).Should(Succeed())
		})

		It("re-stamps observedGeneration after a spec update", func() {
			Eventually(func(g Gomega) {
				g.Expect(readyCondition(g)).NotTo(BeNil())
			}, timeout, interval).Should(Succeed())

			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(storageConfig), storageConfig)).To(Succeed())
			storageConfig.Spec.S3.BasePath = "backups"
			Expect(k8sClient.Update(ctx, storageConfig)).To(Succeed())
			Expect(storageConfig.Generation).To(BeNumerically(">", 1))

			// The name of this test is the assertion: Ready must be
			// re-stamped at the new generation, not left at the old one.
			Eventually(func(g Gomega) {
				cond := readyCondition(g)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
				g.Expect(cond.Reason).To(Equal(v1.ReasonHealthy))
				g.Expect(cond.ObservedGeneration).To(Equal(storageConfig.Generation))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("with static credentials", func() {
		var secret *corev1.Secret

		BeforeEach(func() {
			secret = &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "minio-credentials-" + utilrand.String(8),
					Namespace: "default",
				},
				Data: map[string][]byte{
					"accessKeyId":     []byte("minio"),
					"secretAccessKey": []byte("minio123"),
				},
			}

			storageConfig = validObjectStorageConfig()
			storageConfig.Spec.S3.Endpoint = minioEndpoint
			storageConfig.Spec.S3.Auth = v1.S3StorageAuth{
				Type: v1.ObjectStorageAuthTypeCredentials,
				Credentials: &v1.S3Credentials{
					SecretRef: v1.S3CredentialsSecretRef{
						Name:               secret.Name,
						Namespace:          secret.Namespace,
						AccessKeyIDKey:     "accessKeyId",
						SecretAccessKeyKey: "secretAccessKey",
					},
				},
			}

			Expect(k8sClient.Create(ctx, storageConfig)).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, storageConfig)).To(Succeed()) })
		})

		It("reports MissingSecret until the Secret exists, then Healthy", func() {
			expectReady(metav1.ConditionFalse, v1.ReasonMissingSecret)

			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, secret)).To(Succeed()) })

			expectReady(metav1.ConditionTrue, v1.ReasonHealthy)
		})

		It("reports MissingSecret when a configured key is absent", func() {
			secret.Data = map[string][]byte{"accessKeyId": []byte("minio")}
			Expect(k8sClient.Create(ctx, secret)).To(Succeed())
			DeferCleanup(func() { Expect(k8sClient.Delete(ctx, secret)).To(Succeed()) })

			expectReady(metav1.ConditionFalse, v1.ReasonMissingSecret)

			// A wholly missing Secret reports the same reason, so the reason
			// alone does not prove the key check ran. The message must name
			// the key that is absent.
			Eventually(func(g Gomega) {
				cond := readyCondition(g)
				g.Expect(cond).NotTo(BeNil())
				g.Expect(cond.Message).To(ContainSubstring("secretAccessKey"))
			}, timeout, interval).Should(Succeed())
		})
	})
})
