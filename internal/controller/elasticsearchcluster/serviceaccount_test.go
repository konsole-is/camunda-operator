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

package elasticsearchcluster

import (
	"time"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/accounthash"
)

// updateElasticsearchCluster applies mutate to the latest copy of cluster and
// polls until the update lands.
func updateElasticsearchCluster(cluster *v1.ElasticsearchCluster, mutate func(*v1.ElasticsearchCluster)) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.ElasticsearchCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		mutate(&latest)
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// expectElasticsearchClusterAccount polls until the account named key
// satisfies match.
func expectElasticsearchClusterAccount(key client.ObjectKey, match func(Gomega, *corev1.ServiceAccount)) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var account corev1.ServiceAccount
		g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
		match(g, &account)
	}, timeout, interval).Should(Succeed())
}

// ownedBy reports whether account carries an owner reference to the
// ElasticsearchCluster named name.
func ownedBy(account *corev1.ServiceAccount, name string) bool {
	for _, ref := range account.OwnerReferences {
		if ref.Kind == "ElasticsearchCluster" && ref.Name == name {
			return true
		}
	}
	return false
}

// nodeTemplate polls until the ECK CR of cluster has a pod template that
// satisfies match, and returns that template.
func nodeTemplate(
	cluster *v1.ElasticsearchCluster,
	match func(Gomega, *corev1.PodTemplateSpec),
) corev1.PodTemplateSpec {
	GinkgoHelper()
	var template corev1.PodTemplateSpec
	Eventually(func(g Gomega) {
		var es esv1.Elasticsearch
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).To(Succeed())
		g.Expect(es.Spec.NodeSets).NotTo(BeEmpty())
		template = es.Spec.NodeSets[0].PodTemplate
		match(g, &template)
	}, timeout, interval).Should(Succeed())
	return template
}

var _ = Describe("ElasticsearchCluster ServiceAccount roll", func() {
	// Workload identity reads the annotations of the account when a pod
	// starts, so ECK must restart the nodes when they change.
	It("rolls the nodes when the account annotations change, and only then", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{
			Annotations: map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/old"},
		}
		createElasticsearchCluster(cluster)
		first := nodeTemplate(cluster, func(g Gomega, template *corev1.PodTemplateSpec) {
			g.Expect(template.Annotations).To(HaveKey(accounthash.Annotation))
		})
		hash := first.Annotations[accounthash.Annotation]

		updateElasticsearchCluster(cluster, func(c *v1.ElasticsearchCluster) {
			c.Spec.ServiceAccount.Annotations[v1.IRSARoleARNAnnotation] = "arn:aws:iam::123456789012:role/new"
		})
		changed := nodeTemplate(cluster, func(g Gomega, template *corev1.PodTemplateSpec) {
			g.Expect(template.Annotations).To(HaveKey(accounthash.Annotation))
			g.Expect(template.Annotations[accounthash.Annotation]).NotTo(Equal(hash))
		})

		// The new label proves that the operator applied the edit.
		updateElasticsearchCluster(cluster, func(c *v1.ElasticsearchCluster) {
			c.Spec.PodLabels = map[string]string{"team": "platform"}
		})
		labeled := nodeTemplate(cluster, func(g Gomega, template *corev1.PodTemplateSpec) {
			g.Expect(template.Labels).To(HaveKeyWithValue("team", "platform"))
		})
		Expect(labeled.Annotations).To(Equal(changed.Annotations))
	})
})

var _ = Describe("ElasticsearchCluster ServiceAccount release", func() {
	// create: false says that the user owns the account. An owner reference
	// that stays makes the garbage collector delete the account with the
	// cluster.
	It("releases the account when create switches to false, and adopts it back", func() {
		const (
			roleFirst = "arn:aws:iam::123456789012:role/es"
			roleLater = "arn:aws:iam::123456789012:role/later"
		)
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{
			Annotations: map[string]string{v1.IRSARoleARNAnnotation: roleFirst},
		}
		createElasticsearchCluster(cluster)
		key := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-es"}
		expectElasticsearchClusterAccount(key, func(g Gomega, account *corev1.ServiceAccount) {
			g.Expect(metav1.GetControllerOf(account)).NotTo(BeNil())
			g.Expect(account.Annotations).To(HaveKeyWithValue(v1.IRSARoleARNAnnotation, roleFirst))
		})

		updateElasticsearchCluster(cluster, func(c *v1.ElasticsearchCluster) {
			c.Spec.ServiceAccount.Create = new(false)
		})
		expectElasticsearchClusterAccount(key, func(g Gomega, account *corev1.ServiceAccount) {
			g.Expect(ownedBy(account, cluster.Name)).To(BeFalse())
		})

		// The user takes the account over. A spec change that the operator
		// processed proves that it read the spec and left the account alone.
		Eventually(func(g Gomega) {
			var account corev1.ServiceAccount
			g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
			account.Annotations = map[string]string{"example.com/team": "user"}
			g.Expect(k8sClient.Update(ctx, &account)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		updateElasticsearchCluster(cluster, func(c *v1.ElasticsearchCluster) {
			c.Spec.ServiceAccount.Annotations[v1.IRSARoleARNAnnotation] = roleLater
		})
		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			g.Expect(latest.Status.ObservedGeneration).To(Equal(latest.Generation))
		}, timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			var account corev1.ServiceAccount
			g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
			g.Expect(ownedBy(&account, cluster.Name)).To(BeFalse())
			g.Expect(account.Annotations).To(Equal(map[string]string{"example.com/team": "user"}))
		}, 2*time.Second, interval).Should(Succeed())

		updateElasticsearchCluster(cluster, func(c *v1.ElasticsearchCluster) {
			c.Spec.ServiceAccount.Create = nil
		})
		expectElasticsearchClusterAccount(key, func(g Gomega, account *corev1.ServiceAccount) {
			g.Expect(metav1.GetControllerOf(account)).NotTo(BeNil())
			g.Expect(account.Annotations).To(HaveKeyWithValue(v1.IRSARoleARNAnnotation, roleLater))
		})
	})
})
