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
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// expectKept checks that the object named by key stays the object with uid,
// and that it carries no owner reference.
func expectKept(key client.ObjectKey, obj client.Object, uid types.UID) {
	GinkgoHelper()
	Consistently(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, key, obj)).To(Succeed())
		g.Expect(obj.GetUID()).To(Equal(uid))
		g.Expect(obj.GetOwnerReferences()).To(BeEmpty())
	}, 2*time.Second, interval).Should(Succeed())
}

// The operator derives these names from the cluster, and a user can create an
// object of the same name. An object that the cluster does not own is not the
// operator's to delete.
var _ = Describe("ElasticsearchCluster objects of the user", func() {
	It("deletes the account that it created once the cluster no longer uses it", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{}
		createElasticsearchCluster(cluster)
		key := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-es"}
		expectElasticsearchClusterAccount(key, func(g Gomega, account *corev1.ServiceAccount) {
			g.Expect(ownedBy(account, cluster.Name)).To(BeTrue())
		})

		updateElasticsearchCluster(cluster, func(c *v1.ElasticsearchCluster) { c.Spec.ServiceAccount = nil })

		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, key, &corev1.ServiceAccount{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())
	})

	It("keeps an account of the user with the derived name while the cluster uses none", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Namespace = newElasticsearchClusterNamespace()
		key := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-es"}
		account := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name}}
		Expect(k8sClient.Create(ctx, account)).To(Succeed())
		createElasticsearchCluster(cluster)

		// The ECK CR is applied in the same pass that deletes the account
		// that the cluster does not use.
		fetchOwnedElasticsearch(cluster)
		expectKept(key, &corev1.ServiceAccount{}, account.UID)
	})

	// Without monitoring the metrics component is off and deletes what it
	// owns.
	It("keeps a Service of the user with the derived metrics name while monitoring is off", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Namespace = newElasticsearchClusterNamespace()
		key := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-es-metrics"}
		svc := &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
			Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "http", Port: 80}}},
		}
		Expect(k8sClient.Create(ctx, svc)).To(Succeed())
		createElasticsearchCluster(cluster)

		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			g.Expect(latest.Status.Conditions).To(ContainElement(And(
				HaveField("Type", v1.ConditionMetricsReady),
				HaveField("Reason", string(component.Disabled)),
			)))
		}, timeout, interval).Should(Succeed())
		expectKept(key, &corev1.Service{}, svc.UID)
	})

	// A suspension deletes the ECK CR that the cluster owns. One that the
	// cluster never applied is not suspended and not deleted.
	It("keeps an ECK CR of the user with the name of a suspended cluster", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.Suspend = true
		cluster.Namespace = newElasticsearchClusterNamespace()
		es := &esv1.Elasticsearch{
			ObjectMeta: metav1.ObjectMeta{Namespace: cluster.Namespace, Name: cluster.Name},
			Spec: esv1.ElasticsearchSpec{
				Version:  "9.2.4",
				NodeSets: []esv1.NodeSet{{Name: "default", Count: 1}},
			},
		}
		Expect(k8sClient.Create(ctx, es)).To(Succeed())
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(
			cluster, metav1.ConditionTrue,
			Equal(string(component.Suspended)), HavePrefix("elasticsearch: "),
		)
		expectKept(client.ObjectKeyFromObject(es), &esv1.Elasticsearch{}, es.UID)
	})
})
