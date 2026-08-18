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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/discovery"
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
		// can lose to a conflict. Only the CEL rejection ends the retry.
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

		By("pruning a namespace on the reference: the schema has no such field")
		crossNamespace := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "core.camunda.io/v1",
			"kind":       "LogicalBackupElasticsearch",
			"metadata":   map[string]any{"name": "cross-" + utilrand.String(6), "namespace": backup.Namespace},
			"spec": map[string]any{
				"clusterRef": map[string]any{"name": "some-cluster", "namespace": "elsewhere"},
			},
		}}
		Expect(k8sClient.Create(ctx, crossNamespace)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, crossNamespace) })

		var stored unstructured.Unstructured
		stored.SetGroupVersionKind(crossNamespace.GroupVersionKind())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(crossNamespace), &stored)).To(Succeed())
		_, found, err := unstructured.NestedString(stored.Object, "spec", "clusterRef", "namespace")
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse(), "the API server pruned the unknown namespace field")
	})

	It("serves the resource under its plural and short name", func() {
		// The RESTMapper of the client resolves the resource. A wrong plural
		// fails every typed call. A successful list is proof enough of the
		// path marker.
		var list v1.LogicalBackupElasticsearchList
		Expect(k8sClient.List(ctx, &list)).To(Succeed())

		// The short name is what an operator types. Only discovery proves
		// it: the typed client never uses it.
		discovery, err := discovery.NewDiscoveryClientForConfig(env.Cfg)
		Expect(err).NotTo(HaveOccurred())
		resources, err := discovery.ServerResourcesForGroupVersion(v1.GroupVersion.String())
		Expect(err).NotTo(HaveOccurred())
		Expect(resources.APIResources).To(ContainElement(SatisfyAll(
			HaveField("Name", "logicalbackupelasticsearches"),
			HaveField("ShortNames", ConsistOf("lbes")),
		)))
	})
})
