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
	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
)

// newElasticsearchClusterNamespace creates a uniquely named Namespace for one
// spec and registers its deletion.
// versionBelowFloor is an Elasticsearch version below the Camunda 8.9 floor.
const versionBelowFloor = "8.18.0"

func newElasticsearchClusterNamespace() string {
	GinkgoHelper()
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "esc-ns-" + utilrand.String(8)},
	}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })
	return ns.Name
}

// smallClusterSpec is a complete single-node baseline that presets use as
// their cluster block.
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

// expectElasticsearchClusterReady polls until the Ready condition of cluster
// has the given status and its reason and message match the given matchers.
func expectElasticsearchClusterReady(
	cluster *v1.ElasticsearchCluster,
	status metav1.ConditionStatus,
	reason, message types.GomegaMatcher,
) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.ElasticsearchCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Status).To(Equal(status))
		g.Expect(ready.Reason).To(reason)
		g.Expect(ready.Message).To(message)
	}, timeout, interval).Should(Succeed())
}

// fetchOwnedElasticsearch polls until the ECK CR with the name of cluster
// exists, and returns it.
func fetchOwnedElasticsearch(cluster *v1.ElasticsearchCluster) *esv1.Elasticsearch {
	GinkgoHelper()
	var es esv1.Elasticsearch
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).To(Succeed())
	}, timeout, interval).Should(Succeed())
	return &es
}

// updateECKStatus patches the status subresource of the ECK CR. It always
// stamps status.observedGeneration with the current generation of the CR, so
// the handlers of the wrapper trust the reported state. envtest runs no ECK
// operator, so the specs drive health transitions this way.
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

// expectStorageShrinkIgnored polls until the controller has recorded the
// StorageShrinkIgnored event for cluster, then asserts that the applied ECK
// data volume claim still requests applied and that Ready does not report
// InvalidReference.
func expectStorageShrinkIgnored(cluster *v1.ElasticsearchCluster, applied string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var events corev1.EventList
		g.Expect(k8sClient.List(ctx, &events, client.InNamespace(cluster.Namespace))).To(Succeed())
		g.Expect(events.Items).To(ContainElement(SatisfyAll(
			HaveField("Reason", "StorageShrinkIgnored"),
			HaveField("InvolvedObject.Name", cluster.Name),
			HaveField("Type", corev1.EventTypeWarning),
		)))
	}, timeout, interval).Should(Succeed())

	var es esv1.Elasticsearch
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).To(Succeed())
	Expect(es.Spec.NodeSets[0].VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]).
		To(Equal(resource.MustParse(applied)))

	var latest v1.ElasticsearchCluster
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
	ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
	Expect(ready).NotTo(BeNil())
	Expect(ready.Reason).NotTo(Equal(v1.ReasonInvalidReference))
}

// expectStorageSize polls until status.storageSize of cluster equals want.
func expectStorageSize(cluster *v1.ElasticsearchCluster, want string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.ElasticsearchCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		g.Expect(latest.Status.StorageSize).NotTo(BeNil())
		g.Expect(latest.Status.StorageSize.Cmp(resource.MustParse(want))).To(BeZero())
	}, timeout, interval).Should(Succeed())
}

// expectControlledBy asserts that obj carries a controller owner reference to
// cluster. Deletion then garbage-collects it without a finalizer.
func expectControlledBy(obj client.Object, cluster *v1.ElasticsearchCluster) {
	GinkgoHelper()
	controller := metav1.GetControllerOf(obj)
	Expect(controller).NotTo(BeNil())
	Expect(controller.Kind).To(Equal("ElasticsearchCluster"))
	Expect(controller.Name).To(Equal(cluster.Name))
}

var _ = Describe("ElasticsearchCluster controller", func() {
	It("publishes the ECK CR, user Secret and storage contract and mirrors the creating component", func() {
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
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{
					Namespace: cluster.Namespace, Name: cluster.Name + "-es-user",
				}, &secret,
			)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Expect(secret.Data["username"]).To(Equal([]byte("camunda")))
		Expect(secret.Data["roles"]).To(Equal([]byte("superuser")))
		Expect(secret.Data["password"]).To(HaveLen(32))
		expectControlledBy(&secret, cluster)

		var contract v1.SecondaryStorageConfig
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{
					Namespace: cluster.Namespace, Name: cluster.Spec.SecondaryStorageConfig,
				}, &contract,
			)).To(Succeed())
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

		expectElasticsearchClusterReady(
			cluster, metav1.ConditionFalse,
			// Before ECK reports health, the first reconcile sees Creating and
			// any later reconcile sees Failing (health unreported past the
			// first apply). envtest runs no ECK, so both are valid here.
			SatisfyAny(Equal(string(component.AliveCreating)), Equal(string(component.AliveFailing))),
			HavePrefix("ElasticsearchReady: "),
		)
	})

	It("mirrors the Elasticsearch component onto Ready as ECK health transitions", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		fetchOwnedElasticsearch(cluster)

		updateECKStatus(cluster, func(es *esv1.Elasticsearch) {
			es.Status.Health = esv1.ElasticsearchGreenHealth
			es.Status.AvailableNodes = 1
		})
		expectElasticsearchClusterReady(
			cluster, metav1.ConditionTrue,
			Equal(v1.ReasonHealthy), HaveSuffix(": Component is healthy."),
		)

		updateECKStatus(cluster, func(es *esv1.Elasticsearch) {
			es.Status.Health = esv1.ElasticsearchRedHealth
		})
		expectElasticsearchClusterReady(
			cluster, metav1.ConditionFalse,
			Equal(string(component.AliveFailing)), Equal("ElasticsearchReady: Elasticsearch reports red health"),
		)
	})

	It("reports InvalidReference for a dangling presetRef and applies nothing", func() {
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = "does-not-exist-" + utilrand.String(8)
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonInvalidReference), ContainSubstring(cluster.Spec.PresetRef),
		)

		var es esv1.Elasticsearch
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).NotTo(Succeed())
	})

	It("names every missing field of an incomplete merge", func() {
		preset := createElasticsearchClusterPreset(v1.ElasticsearchClusterSpec{Version: "9.2.4"})
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonInvalidReference),
			And(ContainSubstring("replicas"), ContainSubstring("storageSize")),
		)
	})

	It("enforces the version floor on the merged result", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.Version = versionBelowFloor
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonInvalidReference), ContainSubstring(versionBelowFloor),
		)
	})

	It("suspends by deleting the ECK CR with retained volumes, and recreates it on resume", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		es := fetchOwnedElasticsearch(cluster)
		Expect(es.Spec.VolumeClaimDeletePolicy).To(Equal(esv1.DeleteOnScaledownOnlyPolicy))

		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			latest.Spec.Suspend = true
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// The CR is deleted only once ECK has observed the retaining policy.
		// envtest runs no ECK, so the spec stamps the observed generation.
		updateECKStatus(cluster, func(es *esv1.Elasticsearch) {
			es.Status.Phase = esv1.ElasticsearchReadyPhase
		})

		Eventually(func(g Gomega) {
			var latest esv1.Elasticsearch
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "the Elasticsearch CR must be deleted on suspension")
		}, timeout, interval).Should(Succeed())

		// Suspended is Ready=True: the cluster is in its desired state.
		expectElasticsearchClusterReady(
			cluster, metav1.ConditionTrue,
			Equal(string(component.Suspended)), HavePrefix("ElasticsearchReady: "),
		)

		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			latest.Spec.Suspend = false
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// Resume recreates the CR; ECK reattaches the retained volumes by name.
		fetchOwnedElasticsearch(cluster)
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
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{
				Namespace: cluster.Namespace, Name: cluster.Spec.SecondaryStorageConfig,
			}, &contract,
		)).To(Succeed())
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

	It("ignores a preset-driven storageSize shrink, keeps the applied size, and records an event", func() {
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

		expectStorageShrinkIgnored(cluster, "1Gi")
	})

	It("ignores an inline storageSize below the applied preset baseline", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		fetchOwnedElasticsearch(cluster)

		// Admission cannot catch this shrink. storageSize was unset inline
		// before, so the CEL transition rule does not fire.
		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			latest.Spec.StorageSize = new(resource.MustParse("512Mi"))
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		expectStorageShrinkIgnored(cluster, "1Gi")
	})

	It("reports the data volume size in status.storageSize", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		fetchOwnedElasticsearch(cluster)

		// Before any claim reports a capacity, the applied claim size is reported.
		expectStorageSize(cluster, "1Gi")

		// A data claim that ECK labels with the cluster name reports its
		// capacity, for example after a resize outside the spec. envtest runs
		// no ECK, so the spec creates the claim and stamps its capacity.
		claim := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      components.DataVolumeClaimName + "-" + cluster.Name + "-es-default-0",
				Namespace: cluster.Namespace,
				Labels:    map[string]string{components.ECKClusterNameLabel: cluster.Name},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")},
				},
			},
		}
		Expect(k8sClient.Create(ctx, claim)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, claim) })
		claim.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("2Gi")}
		claim.Status.Phase = corev1.ClaimBound
		Expect(k8sClient.Status().Update(ctx, claim)).To(Succeed())

		expectStorageSize(cluster, "2Gi")
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

	// The rendered ECK CR must apply against the API server in envtest, so
	// the suite loads the ECK CRDs from the resolved module.
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
