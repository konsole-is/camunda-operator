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
	"github.com/onsi/gomega/types"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
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

// smallClusterSpec is a complete single-node baseline used as a preset's
// cluster block.
func smallClusterSpec() v1.ElasticsearchClusterSpec {
	return v1.ElasticsearchClusterSpec{
		Version:     "9.2.4",
		Replicas:    new(int32(1)),
		StorageSize: new(resource.MustParse("1Gi")),
	}
}

// createElasticsearchClusterPreset creates a uniquely named preset with the
// given cluster baseline and registers its deletion.
func createElasticsearchClusterPreset(spec v1.ElasticsearchClusterSpec) *v1.ElasticsearchClusterPreset {
	GinkgoHelper()
	preset := &v1.ElasticsearchClusterPreset{
		ObjectMeta: metav1.ObjectMeta{Name: "escp-" + utilrand.String(8)},
		Spec:       v1.ElasticsearchClusterPresetSpec{Cluster: spec},
	}
	Expect(k8sClient.Create(ctx, preset)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, preset) })
	return preset
}

// createElasticsearchCluster creates cluster in its own fresh namespace and
// registers its deletion.
func createElasticsearchCluster(cluster *v1.ElasticsearchCluster) {
	GinkgoHelper()
	cluster.Namespace = newElasticsearchClusterNamespace()
	Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, cluster) })
}

// expectElasticsearchClusterReady polls until cluster's Ready condition
// matches the given status and reason, and its message the given matcher.
func expectElasticsearchClusterReady(
	cluster *v1.ElasticsearchCluster,
	status metav1.ConditionStatus,
	reason string,
	message types.GomegaMatcher,
) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.ElasticsearchCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		ready := meta.FindStatusCondition(latest.Status.Conditions, conditions.TypeReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Status).To(Equal(status))
		g.Expect(ready.Reason).To(Equal(reason))
		g.Expect(ready.Message).To(message)
	}, timeout, interval).Should(Succeed())
}

// fetchOwnedElasticsearch polls until the ECK CR named after cluster exists
// and returns it.
func fetchOwnedElasticsearch(cluster *v1.ElasticsearchCluster) *esv1.Elasticsearch {
	GinkgoHelper()
	var es esv1.Elasticsearch
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).To(Succeed())
	}, timeout, interval).Should(Succeed())
	return &es
}

// updateECKStatus patches the ECK CR's status subresource, always stamping
// status.observedGeneration with the CR's current generation so the wrapper's
// handlers trust the reported state. envtest runs no ECK operator, so specs
// drive health transitions this way.
func updateECKStatus(cluster *v1.ElasticsearchCluster, mutate func(*esv1.Elasticsearch)) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var es esv1.Elasticsearch
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).To(Succeed())
		es.Status.ObservedGeneration = es.Generation
		mutate(&es)
		g.Expect(k8sClient.Status().Update(ctx, &es)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// expectControlledBy asserts obj carries a controller owner reference to
// cluster, so deletion garbage-collects it without a finalizer.
func expectControlledBy(obj client.Object, cluster *v1.ElasticsearchCluster) {
	GinkgoHelper()
	controller := metav1.GetControllerOf(obj)
	Expect(controller).NotTo(BeNil())
	Expect(controller.Kind).To(Equal("ElasticsearchCluster"))
	Expect(controller.Name).To(Equal(cluster.Name))
}

var _ = Describe("ElasticsearchCluster controller", func() {
	It("publishes the ECK CR, user Secret and storage contract and reports Progressing", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)

		es := fetchOwnedElasticsearch(cluster)
		Expect(es.Spec.Version).To(Equal("9.2.4"))
		Expect(es.Spec.Auth.FileRealm).To(HaveLen(1))
		Expect(es.Spec.Auth.FileRealm[0].SecretName).To(Equal(cluster.Name + "-es-user"))
		Expect(es.Spec.NodeSets).To(HaveLen(1))
		Expect(es.Spec.NodeSets[0].Count).To(Equal(int32(1)))
		Expect(es.Spec.NodeSets[0].PodTemplate.Labels).To(HaveKeyWithValue("camunda.io/cluster", cluster.Name))
		Expect(es.Spec.NodeSets[0].PodTemplate.Labels).To(HaveKeyWithValue("camunda.io/component", "elasticsearch"))
		Expect(es.Spec.NodeSets[0].VolumeClaimTemplates).To(HaveLen(1))
		claim := es.Spec.NodeSets[0].VolumeClaimTemplates[0]
		Expect(claim.Name).To(Equal("elasticsearch-data"))
		Expect(claim.Labels).To(HaveKeyWithValue("camunda.io/cluster", cluster.Name))
		Expect(claim.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("1Gi")))
		expectControlledBy(es, cluster)

		var secret corev1.Secret
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: cluster.Namespace, Name: cluster.Name + "-es-user",
			}, &secret)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Expect(secret.Data["username"]).To(Equal([]byte("camunda")))
		Expect(secret.Data["roles"]).To(Equal([]byte("superuser")))
		Expect(secret.Data["password"]).To(HaveLen(32))
		expectControlledBy(&secret, cluster)

		var contract v1.SecondaryStorageConfig
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{
				Namespace: cluster.Namespace, Name: cluster.Spec.SecondaryStorageConfig,
			}, &contract)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Expect(contract.Spec.Type).To(Equal(v1.SecondaryStorageTypeElasticsearch))
		Expect(contract.Spec.Elasticsearch).NotTo(BeNil())
		Expect(contract.Spec.Elasticsearch.Endpoint).To(Equal(
			"https://" + cluster.Name + "-es-http." + cluster.Namespace + ".svc:9200",
		))
		Expect(contract.Spec.Elasticsearch.CredentialsSecretRef).To(Equal(v1.CredentialsSecretRef{
			Name: cluster.Name + "-es-user", Namespace: cluster.Namespace,
			UsernameKey: "username", PasswordKey: "password",
		}))
		Expect(contract.Spec.Elasticsearch.CASecretRef).To(Equal(&v1.SecretKeyRef{
			Name: cluster.Name + "-es-http-certs-public", Namespace: cluster.Namespace, Key: "ca.crt",
		}))
		expectControlledBy(&contract, cluster)

		expectElasticsearchClusterReady(cluster, metav1.ConditionFalse,
			conditions.ReasonProgressing, ContainSubstring("ElasticsearchReady"))
	})

	It("flips Ready between Progressing and Healthy as ECK health transitions", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		fetchOwnedElasticsearch(cluster)

		updateECKStatus(cluster, func(es *esv1.Elasticsearch) {
			es.Status.Health = esv1.ElasticsearchGreenHealth
			es.Status.AvailableNodes = 1
		})
		expectElasticsearchClusterReady(cluster, metav1.ConditionTrue,
			conditions.ReasonHealthy, Equal("All components ready"))

		updateECKStatus(cluster, func(es *esv1.Elasticsearch) {
			es.Status.Health = esv1.ElasticsearchRedHealth
		})
		expectElasticsearchClusterReady(cluster, metav1.ConditionFalse,
			conditions.ReasonProgressing, ContainSubstring("ElasticsearchReady"))
	})

	It("reports InvalidReference for a dangling presetRef and applies nothing", func() {
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = "does-not-exist-" + utilrand.String(8)
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(cluster, metav1.ConditionFalse,
			conditions.ReasonInvalidReference, ContainSubstring(cluster.Spec.PresetRef))

		var es esv1.Elasticsearch
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).NotTo(Succeed())
	})

	It("names every missing field of an incomplete merge", func() {
		preset := createElasticsearchClusterPreset(v1.ElasticsearchClusterSpec{Version: "9.2.4"})
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(cluster, metav1.ConditionFalse,
			conditions.ReasonInvalidReference,
			And(ContainSubstring("replicas"), ContainSubstring("storageSize")))
	})

	It("enforces the version floor on the merged result", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.Version = versionBelowFloor
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(cluster, metav1.ConditionFalse,
			conditions.ReasonInvalidReference, ContainSubstring(versionBelowFloor))
	})

	It("scales the node set to zero on suspend and reports Suspended", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		fetchOwnedElasticsearch(cluster)

		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			latest.Spec.Suspend = true
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			var es esv1.Elasticsearch
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).To(Succeed())
			g.Expect(es.Spec.NodeSets[0].Count).To(Equal(int32(0)))
		}, timeout, interval).Should(Succeed())

		// ECK confirms the scale-down: no available nodes at the current
		// generation.
		updateECKStatus(cluster, func(es *esv1.Elasticsearch) {
			es.Status.AvailableNodes = 0
		})

		expectElasticsearchClusterReady(cluster, metav1.ConditionFalse,
			conditions.ReasonSuspended, Equal("Suspended by spec.suspend"))
		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			suspended := meta.FindStatusCondition(latest.Status.Conditions, conditions.TypeSuspended)
			g.Expect(suspended).NotTo(BeNil())
			g.Expect(suspended.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(suspended.Reason).To(Equal(conditions.ReasonSuspended))
		}, timeout, interval).Should(Succeed())
	})

	It("regenerates the password when the credentials Secret is deleted", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)

		secretKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-es-user"}
		var secret corev1.Secret
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, secretKey, &secret)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		oldPassword := string(secret.Data["password"])

		Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())

		Eventually(func(g Gomega) {
			var recreated corev1.Secret
			g.Expect(k8sClient.Get(ctx, secretKey, &recreated)).To(Succeed())
			g.Expect(recreated.UID).NotTo(Equal(secret.UID))
			g.Expect(recreated.Data["password"]).To(HaveLen(32))
			g.Expect(string(recreated.Data["password"])).NotTo(Equal(oldPassword))
		}, timeout, interval).Should(Succeed())

		var contract v1.SecondaryStorageConfig
		Expect(k8sClient.Get(ctx, client.ObjectKey{
			Namespace: cluster.Namespace, Name: cluster.Spec.SecondaryStorageConfig,
		}, &contract)).To(Succeed())
		Expect(contract.Spec.Elasticsearch.CredentialsSecretRef.Name).To(Equal(secretKey.Name))
	})

	It("flows a preset edit to referencing clusters without touching the CR", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		fetchOwnedElasticsearch(cluster)

		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchClusterPreset
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(preset), &latest)).To(Succeed())
			latest.Spec.Cluster.Replicas = new(int32(2))
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			var es esv1.Elasticsearch
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).To(Succeed())
			g.Expect(es.Spec.NodeSets[0].Count).To(Equal(int32(2)))
		}, timeout, interval).Should(Succeed())
	})

	It("refuses a preset-driven storageSize shrink and keeps the applied size", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		fetchOwnedElasticsearch(cluster)

		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchClusterPreset
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(preset), &latest)).To(Succeed())
			latest.Spec.Cluster.StorageSize = new(resource.MustParse("512Mi"))
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		expectElasticsearchClusterReady(cluster, metav1.ConditionFalse,
			conditions.ReasonInvalidReference, ContainSubstring("shrink"))

		var es esv1.Elasticsearch
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).To(Succeed())
		Expect(es.Spec.NodeSets[0].VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]).
			To(Equal(resource.MustParse("1Gi")))
	})

	It("refuses an inline storageSize below the applied preset baseline", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		fetchOwnedElasticsearch(cluster)

		// Admission cannot catch this shrink: storageSize was previously
		// unset inline, so the CEL transition rule does not fire.
		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			latest.Spec.StorageSize = new(resource.MustParse("512Mi"))
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		expectElasticsearchClusterReady(cluster, metav1.ConditionFalse,
			conditions.ReasonInvalidReference, ContainSubstring("shrink"))

		var es esv1.Elasticsearch
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).To(Succeed())
		Expect(es.Spec.NodeSets[0].VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]).
			To(Equal(resource.MustParse("1Gi")))
	})

	It("records the reconciled generation in status.observedGeneration", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)

		expectObservedGeneration := func() {
			GinkgoHelper()
			Eventually(func(g Gomega) {
				var latest v1.ElasticsearchCluster
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
				g.Expect(latest.Status.ObservedGeneration).To(Equal(latest.Generation))
			}, timeout, interval).Should(Succeed())
		}
		expectObservedGeneration()

		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			latest.Spec.PodLabels = map[string]string{"team": "platform"}
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
			g.Expect(latest.Generation).To(BeNumerically(">", 1))
		}, timeout, interval).Should(Succeed())
		expectObservedGeneration()
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
