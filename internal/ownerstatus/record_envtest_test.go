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

package ownerstatus

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/test/envtest"
)

var (
	env       *envtest.Env
	envCtx    context.Context
	k8sClient client.Client
)

// TestRecordAgainstTheAPIServer runs Record against a real API server, because
// the fake client does not show what a status patch keeps of the metadata.
func TestRecordAgainstTheAPIServer(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "ownerstatus Record Suite")
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	// No controller runs, so only Record writes the status.
	env = envtest.Start(func(ctrl.Manager) error { return nil })

	envCtx, k8sClient = env.Ctx, env.Client
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	Eventually(env.Stop, time.Minute, time.Second).Should(Succeed())
})

var _ = Describe("Record", func() {
	It("refuses a receipt for an object that replaced the owner under its name", func() {
		namespace := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "record-" + utilrand.String(6)}}
		Expect(k8sClient.Create(envCtx, namespace)).To(Succeed())
		original := bucket(namespace.Name)
		Expect(k8sClient.Create(envCtx, original)).To(Succeed())

		var read v1.ObjectStorageConfig
		Expect(k8sClient.Get(envCtx, client.ObjectKeyFromObject(original), &read)).To(Succeed())

		By("replacing the object under its name")
		Expect(k8sClient.Delete(envCtx, original)).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(
				k8sClient.Get(envCtx, client.ObjectKeyFromObject(original), &v1.ObjectStorageConfig{}),
			)
		}, time.Minute, 100*time.Millisecond).Should(BeTrue())
		replacement := bucket(namespace.Name)
		Expect(k8sClient.Create(envCtx, replacement)).To(Succeed())

		By("recording a field of the object as it was read")
		owner := read.DeepCopy()
		owner.Status.ObservedGeneration = 42
		Expect(Record(envCtx, k8sClient, k8sClient, &read, owner)).NotTo(Succeed())

		var stored v1.ObjectStorageConfig
		Expect(k8sClient.Get(envCtx, client.ObjectKeyFromObject(original), &stored)).To(Succeed())
		Expect(stored.UID).NotTo(Equal(read.UID))
		Expect(stored.Status.ObservedGeneration).NotTo(Equal(int64(42)), "the receipt landed on the replacement")

		By("recording a field of the replacement itself")
		mine := stored.DeepCopy()
		mine.Status.ObservedGeneration = 42
		Expect(Record(envCtx, k8sClient, k8sClient, &stored, mine)).To(Succeed())
		Expect(k8sClient.Get(envCtx, client.ObjectKeyFromObject(original), &stored)).To(Succeed())
		Expect(stored.Status.ObservedGeneration).To(Equal(int64(42)))
	})
})

// bucket returns an ObjectStorageConfig that the API server admits.
func bucket(namespace string) *v1.ObjectStorageConfig {
	return &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "bucket", Namespace: namespace},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "backups",
				Region:     "eu-west-1",
				Auth:       v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
			},
		},
	}
}
