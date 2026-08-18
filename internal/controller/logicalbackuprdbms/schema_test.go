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

package logicalbackuprdbms

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The admission schema of the LogicalBackupRDBMS CRD.
var _ = Describe("LogicalBackupRDBMS schema", func() {
	valid := func() *v1.LogicalBackupRDBMS {
		return &v1.LogicalBackupRDBMS{
			ObjectMeta: metav1.ObjectMeta{
				Name: "backup-" + utilrand.String(6), Namespace: "default",
			},
			Spec: v1.LogicalBackupRDBMSSpec{
				ClusterRef: v1.ClusterRef{Name: "my-cluster"},
			},
		}
	}

	// The controller adds its finalizer concurrently, so a plain update of
	// the created object can lose to a conflict instead of meeting the CEL
	// rule. mutate updates the live object. Only the CEL rejection ends the
	// retry.
	mutate := func(backup *v1.LogicalBackupRDBMS, change func(*v1.LogicalBackupRDBMS)) error {
		var current v1.LogicalBackupRDBMS
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &current)).To(Succeed())
		change(&current)
		return k8sClient.Update(ctx, &current)
	}

	It("accepts the minimal example and rejects a spec change", func() {
		backup := valid()
		Expect(k8sClient.Create(ctx, backup)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, backup) })

		Eventually(func(g Gomega) {
			err := mutate(backup, func(b *v1.LogicalBackupRDBMS) { b.Spec.ClusterRef.Name = "another-cluster" })
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring("immutable"))
		}, timeout, interval).Should(Succeed())
	})

	It("rejects a spec.dump change too: a retry is a new CR", func() {
		backup := valid()
		Expect(k8sClient.Create(ctx, backup)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, backup) })

		Eventually(func(g Gomega) {
			err := mutate(backup, func(b *v1.LogicalBackupRDBMS) {
				b.Spec.Dump = &v1.DumpPodSpec{
					PodAnnotations: map[string]string{"sidecar.istio.io/inject": "false"},
				}
			})
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring("immutable"))
		}, timeout, interval).Should(Succeed())
	})

	// The schema itself bounds the extraEnvFrom of a backup. Without a safe
	// prefix, a source can supply PGHOSTADDR and redirect the dump.
	It("rejects a backup extraEnvFrom source without a safe prefix", func() {
		backup := valid()
		backup.Spec.Dump = &v1.DumpPodSpec{ExtraEnvFrom: []corev1.EnvFromSource{{
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "extras"},
			},
		}}}
		err := k8sClient.Create(ctx, backup)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("prefix"))

		backup = valid()
		backup.Spec.Dump = &v1.DumpPodSpec{ExtraEnvFrom: []corev1.EnvFromSource{{
			Prefix: "P",
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "extras"},
			},
		}}}
		Expect(k8sClient.Create(ctx, backup)).To(HaveOccurred(), "P plus GHOST would spell PGHOST")
	})

	It("accepts a backup extraEnvFrom source with a safe prefix", func() {
		backup := valid()
		backup.Spec.Dump = &v1.DumpPodSpec{ExtraEnvFrom: []corev1.EnvFromSource{{
			Prefix: "X_",
			ConfigMapRef: &corev1.ConfigMapEnvSource{
				LocalObjectReference: corev1.LocalObjectReference{Name: "extras"},
			},
		}}}
		Expect(k8sClient.Create(ctx, backup)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, backup) })
	})

	It("requires the cluster name", func() {
		backup := valid()
		backup.Spec.ClusterRef.Name = ""
		Expect(k8sClient.Create(ctx, backup)).To(HaveOccurred())
	})
})
