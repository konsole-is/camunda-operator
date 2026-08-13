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
	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// newElasticsearchClusterNamespace creates a uniquely named Namespace for one
// spec and registers its deletion.
func newElasticsearchClusterNamespace() string {
	GinkgoHelper()
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "esc-ns-" + utilrand.String(8)},
	}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })
	return ns.Name
}

var _ = Describe("ElasticsearchCluster controller", func() {
	// Scaffold smoke spec: the reconciler is registered but reconciles nothing
	// yet; the real reconciliation specs land with the controller (#37).
	It("reconciles a valid resource without error", func() {
		cluster := validElasticsearchCluster()
		cluster.Namespace = newElasticsearchClusterNamespace()
		Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, cluster) })

		reconciler := &ElasticsearchClusterReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err := reconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: client.ObjectKeyFromObject(cluster),
		})
		Expect(err).NotTo(HaveOccurred())
	})

	// The rendered ECK CR must actually apply against the API server in
	// envtest, so the suite loads the ECK CRDs from the resolved module.
	It("accepts an ECK Elasticsearch resource in the test environment", func() {
		es := &esv1.Elasticsearch{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eck-smoke-" + utilrand.String(8),
				Namespace: newElasticsearchClusterNamespace(),
			},
			Spec: esv1.ElasticsearchSpec{
				Version:  "9.2.4",
				NodeSets: []esv1.NodeSet{{Name: "default", Count: 1}},
			},
		}

		Expect(k8sClient.Create(ctx, es)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, es) })

		var fetched esv1.Elasticsearch
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(es), &fetched)).To(Succeed())
		Expect(fetched.Spec.Version).To(Equal("9.2.4"))
	})
})
