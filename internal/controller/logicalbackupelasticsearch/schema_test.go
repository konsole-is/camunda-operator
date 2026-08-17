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

package logicalbackupelasticsearch

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

var _ = Describe("LogicalBackupElasticsearch schema", func() {
	newSchemaBackup := func(namespace string) *v1.LogicalBackupElasticsearch {
		return &v1.LogicalBackupElasticsearch{
			ObjectMeta: metav1.ObjectMeta{Name: "schema-" + utilrand.String(8), Namespace: namespace},
			Spec:       v1.LogicalBackupElasticsearchSpec{ClusterRef: v1.ClusterRef{Name: "some-cluster"}},
		}
	}

	It("rejects a clusterRef without a name", func() {
		backup := newSchemaBackup(newNamespace())
		backup.Spec.ClusterRef.Name = ""
		Expect(k8sClient.Create(ctx, backup)).NotTo(Succeed())
	})

	It("rejects every change to the spec", func() {
		backup := newSchemaBackup(newNamespace())
		Expect(k8sClient.Create(ctx, backup)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, backup) })

		// The controller adds its finalizer concurrently, so a plain update
		// can lose to a conflict; only the CEL rejection ends the retry.
		mutate := func(change func(*v1.LogicalBackupElasticsearch)) error {
			var current v1.LogicalBackupElasticsearch
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &current)).To(Succeed())
			change(&current)
			return k8sClient.Update(ctx, &current)
		}

		Eventually(func(g Gomega) {
			err := mutate(func(b *v1.LogicalBackupElasticsearch) { b.Spec.ClusterRef.Name = "another-cluster" })
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring("immutable"))
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			err := mutate(func(b *v1.LogicalBackupElasticsearch) { b.Spec.ClusterRef.Namespace = "elsewhere" })
			g.Expect(err).To(HaveOccurred())
			g.Expect(err.Error()).To(ContainSubstring("immutable"))
		}, timeout, interval).Should(Succeed())
	})

	It("serves the resource under its short name plural", func() {
		// The RESTMapper of the client resolves the resource; a wrong plural
		// or short name would fail every typed call, so listing is proof
		// enough of the path marker.
		var list v1.LogicalBackupElasticsearchList
		Expect(k8sClient.List(ctx, &list)).To(Succeed())
	})
})
