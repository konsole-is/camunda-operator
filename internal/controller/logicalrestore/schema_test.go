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

package logicalrestore

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

var _ = Describe("LogicalRestore schema", func() {
	newSchemaRestore := func(namespace string) *v1.LogicalRestore {
		return &v1.LogicalRestore{
			ObjectMeta: metav1.ObjectMeta{Name: "schema-" + utilrand.String(8), Namespace: namespace},
			Spec: v1.LogicalRestoreSpec{
				BackupRef: v1.LogicalBackupRef{
					Kind: v1.LogicalBackupKindElasticsearch, Name: "some-backup",
				},
				TargetClusterRef: v1.ClusterRef{Name: "some-cluster"},
			},
		}
	}

	It("rejects a backupRef of an unknown kind", func() {
		restore := newSchemaRestore(newNamespace())
		restore.Spec.BackupRef.Kind = "LogicalBackupOpenSearch"
		Expect(k8sClient.Create(ctx, restore)).NotTo(Succeed())
	})

	It("rejects a reference without a name", func() {
		namespace := newNamespace()

		noBackup := newSchemaRestore(namespace)
		noBackup.Spec.BackupRef.Name = ""
		Expect(k8sClient.Create(ctx, noBackup)).NotTo(Succeed())

		noCluster := newSchemaRestore(namespace)
		noCluster.Spec.TargetClusterRef.Name = ""
		Expect(k8sClient.Create(ctx, noCluster)).NotTo(Succeed())
	})

	It("rejects every change to the spec", func() {
		restore := newSchemaRestore(newNamespace())
		Expect(k8sClient.Create(ctx, restore)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, restore) })

		var current v1.LogicalRestore
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(restore), &current)).To(Succeed())
		current.Spec.BackupRef.Name = "another-backup"
		err := k8sClient.Update(ctx, &current)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("immutable"))
	})

	It("prunes a namespace on a reference: the schema has no such field", func() {
		namespace := newNamespace()
		crossNamespace := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "core.camunda.io/v1",
			"kind":       "LogicalRestore",
			"metadata":   map[string]any{"name": "cross-" + utilrand.String(6), "namespace": namespace},
			"spec": map[string]any{
				"backupRef": map[string]any{
					"kind": "LogicalBackupRDBMS", "name": "some-backup", "namespace": "elsewhere",
				},
				"targetClusterRef": map[string]any{"name": "some-cluster", "namespace": "elsewhere"},
			},
		}}
		Expect(k8sClient.Create(ctx, crossNamespace)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, crossNamespace) })

		var stored unstructured.Unstructured
		stored.SetGroupVersionKind(crossNamespace.GroupVersionKind())
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(crossNamespace), &stored)).To(Succeed())
		for _, reference := range []string{"backupRef", "targetClusterRef"} {
			_, found, err := unstructured.NestedString(stored.Object, "spec", reference, "namespace")
			Expect(err).NotTo(HaveOccurred())
			Expect(found).To(BeFalse(), "the API server pruned the unknown namespace field")
		}
	})

	It("serves the resource under its plural and short name", func() {
		// The RESTMapper of the client resolves the resource. A wrong plural
		// fails every typed call. A successful list is proof enough of the
		// path marker.
		var list v1.LogicalRestoreList
		Expect(k8sClient.List(ctx, &list)).To(Succeed())

		// The short name is what an operator types. Only discovery proves it:
		// the typed client never uses it.
		client, err := discovery.NewDiscoveryClientForConfig(env.Cfg)
		Expect(err).NotTo(HaveOccurred())
		resources, err := client.ServerResourcesForGroupVersion(v1.GroupVersion.String())
		Expect(err).NotTo(HaveOccurred())
		Expect(resources.APIResources).To(ContainElement(SatisfyAll(
			HaveField("Name", "logicalrestores"),
			HaveField("ShortNames", ConsistOf("lr")),
		)))
	})
})
