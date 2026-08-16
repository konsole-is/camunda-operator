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

package camundacluster

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// newNamespace creates a uniquely named Namespace and registers its deletion.
func newNamespace() string {
	GinkgoHelper()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "cc-ns-" + utilrand.String(8)}}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })
	return ns.Name
}

// createPlatformConfig creates the basic-auth platform config fixture and
// registers its deletion.
func createPlatformConfig() *v1.CamundaPlatformConfig {
	GinkgoHelper()
	cfg := fixtures.CamundaPlatformConfigBasic()
	Expect(k8sClient.Create(ctx, cfg)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, cfg) })
	return cfg
}

// createSecret creates a Secret with the given string data and registers its
// deletion.
func createSecret(namespace, name string, data map[string]string) *corev1.Secret {
	GinkgoHelper()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		StringData: data,
	}
	Expect(k8sClient.Create(ctx, secret)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })
	return secret
}

// createBinding creates the Elasticsearch binding fixture in namespace, with
// its credentials Secret when withCredentials is set.
func createBinding(namespace string, withCredentials bool) *v1.SecondaryStorageConfig {
	GinkgoHelper()
	binding := fixtures.SecondaryStorageConfigElasticsearch(namespace)
	Expect(k8sClient.Create(ctx, binding)).To(Succeed())
	if withCredentials {
		createSecret(namespace, binding.Spec.Elasticsearch.CredentialsSecretRef.Name, map[string]string{
			"username": "camunda", "password": "es-password",
		})
	}
	return binding
}

// newCluster returns a minimal cluster in namespace that references cfg and
// binding. It is not created.
func newCluster(
	namespace string,
	cfg *v1.CamundaPlatformConfig,
	binding *v1.SecondaryStorageConfig,
) *v1.CamundaCluster {
	return &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cc-" + utilrand.String(8), Namespace: namespace},
		Spec: v1.CamundaClusterSpec{
			PlatformConfigRef: cfg.Name,
			Version:           "8.9.9",
			StorageRef:        binding.Name,
		},
	}
}

// createCluster creates cluster and registers its deletion.
func createCluster(cluster *v1.CamundaCluster) {
	GinkgoHelper()
	Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, cluster) })
}

// createDefaultCluster creates a platform config, a binding with credentials,
// and a minimal cluster in a fresh namespace.
func createDefaultCluster() *v1.CamundaCluster {
	GinkgoHelper()
	ns := newNamespace()
	cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
	createCluster(cluster)
	return cluster
}

// updateCluster applies mutate to the latest revision of cluster, retrying on
// conflicts.
func updateCluster(cluster *v1.CamundaCluster, mutate func(*v1.CamundaCluster)) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		mutate(&latest)
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// expectReady polls until the Ready condition of cluster has the given status
// and its reason and message match.
func expectReady(cluster *v1.CamundaCluster, status metav1.ConditionStatus, reason, message types.GomegaMatcher) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Status).To(Equal(status))
		g.Expect(ready.Reason).To(reason)
		g.Expect(ready.Message).To(message)
	}, timeout, interval).Should(Succeed())
}

// expectCondition polls until the condition of the given type on cluster has
// a reason that matches.
func expectCondition(cluster *v1.CamundaCluster, conditionType string, reason types.GomegaMatcher) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		cond := meta.FindStatusCondition(latest.Status.Conditions, conditionType)
		g.Expect(cond).NotTo(BeNil())
		g.Expect(cond.Reason).To(reason)
	}, timeout, interval).Should(Succeed())
}

// fetchStatefulSet polls until the StatefulSet exists and returns it.
func fetchStatefulSet(key client.ObjectKey) *appsv1.StatefulSet {
	GinkgoHelper()
	var sts appsv1.StatefulSet
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, key, &sts)).To(Succeed())
	}, timeout, interval).Should(Succeed())
	return &sts
}

// fetchDeployment polls until the Deployment exists and returns it.
func fetchDeployment(key client.ObjectKey) *appsv1.Deployment {
	GinkgoHelper()
	var deployment appsv1.Deployment
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, key, &deployment)).To(Succeed())
	}, timeout, interval).Should(Succeed())
	return &deployment
}

// stampStatefulSetReady writes the status a StatefulSet controller would
// write when every replica is ready. envtest runs no such controller.
func stampStatefulSetReady(key client.ObjectKey) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var sts appsv1.StatefulSet
		g.Expect(k8sClient.Get(ctx, key, &sts)).To(Succeed())
		replicas := *sts.Spec.Replicas
		sts.Status.ObservedGeneration = sts.Generation
		sts.Status.Replicas = replicas
		sts.Status.ReadyReplicas = replicas
		sts.Status.UpdatedReplicas = replicas
		sts.Status.CurrentReplicas = replicas
		sts.Status.AvailableReplicas = replicas
		g.Expect(k8sClient.Status().Update(ctx, &sts)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// stampDeploymentReady writes the status a Deployment controller would write
// when every replica is ready.
func stampDeploymentReady(key client.ObjectKey) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var deployment appsv1.Deployment
		g.Expect(k8sClient.Get(ctx, key, &deployment)).To(Succeed())
		replicas := *deployment.Spec.Replicas
		deployment.Status.ObservedGeneration = deployment.Generation
		deployment.Status.Replicas = replicas
		deployment.Status.ReadyReplicas = replicas
		deployment.Status.UpdatedReplicas = replicas
		deployment.Status.AvailableReplicas = replicas
		g.Expect(k8sClient.Status().Update(ctx, &deployment)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// expectControlledBy asserts that obj carries a controller owner reference to
// cluster.
func expectControlledBy(obj client.Object, cluster *v1.CamundaCluster) {
	GinkgoHelper()
	controller := metav1.GetControllerOf(obj)
	Expect(controller).NotTo(BeNil())
	Expect(controller.Kind).To(Equal("CamundaCluster"))
	Expect(controller.Name).To(Equal(cluster.Name))
}

// expectEvent polls until an event with the given reason and type exists for
// cluster.
func expectEvent(cluster *v1.CamundaCluster, reason, eventType string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var events corev1.EventList
		g.Expect(k8sClient.List(ctx, &events, client.InNamespace(cluster.Namespace))).To(Succeed())
		g.Expect(events.Items).To(ContainElement(SatisfyAll(
			HaveField("Reason", reason),
			HaveField("InvolvedObject.Name", cluster.Name),
			HaveField("Type", eventType),
		)))
	}, timeout, interval).Should(Succeed())
}

// configHash polls until the zeebe StatefulSet of cluster carries a config
// hash that differs from previous, and returns it.
func configHash(cluster *v1.CamundaCluster, previous string) string {
	GinkgoHelper()
	var hash string
	Eventually(func(g Gomega) {
		var sts appsv1.StatefulSet
		g.Expect(k8sClient.Get(
			ctx, client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"}, &sts,
		)).To(Succeed())
		hash = sts.Spec.Template.Annotations[components.ConfigHashAnnotation]
		g.Expect(hash).NotTo(BeEmpty())
		g.Expect(hash).NotTo(Equal(previous))
	}, timeout, interval).Should(Succeed())
	return hash
}

// zeebeContainer returns the camunda container of the zeebe StatefulSet.
func zeebeContainer(cluster *v1.CamundaCluster) corev1.Container {
	GinkgoHelper()
	return fetchStatefulSet(
		client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"},
	).Spec.Template.Spec.Containers[0]
}

// secretKeyRef returns the Secret reference of the named variable of
// container, or nil.
func secretKeyRef(container corev1.Container, name string) *corev1.SecretKeySelector {
	for _, e := range container.Env {
		if e.Name == name && e.ValueFrom != nil {
			return e.ValueFrom.SecretKeyRef
		}
	}
	return nil
}

// envValue returns the plain value of the named variable of container.
func envValue(container corev1.Container, name string) string {
	for _, e := range container.Env {
		if e.Name == name {
			return e.Value
		}
	}
	return ""
}

var _ = Describe("CamundaCluster controller", func() {
	It("renders the default topology, mirrors workload readiness onto per-process conditions and Ready", func() {
		cluster := createDefaultCluster()
		zeebeKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"}
		gatewayKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-gateway"}

		sts := fetchStatefulSet(zeebeKey)
		expectControlledBy(sts, cluster)
		Expect(sts.Labels).To(HaveKeyWithValue(labels.ClusterKey, cluster.Name))
		Expect(sts.Labels).To(HaveKeyWithValue(labels.ComponentKey, components.ComponentZeebe))
		Expect(*sts.Spec.Replicas).To(Equal(int32(1)))
		Expect(sts.Spec.Template.Annotations).To(HaveKey(components.ConfigHashAnnotation))
		Expect(sts.Spec.Template.Spec.Containers).To(HaveLen(1))
		container := sts.Spec.Template.Spec.Containers[0]
		Expect(container.Name).To(Equal("camunda"))
		Expect(container.Image).To(Equal("camunda/camunda:8.9.9"))
		Expect(envValue(container, "SPRING_PROFILES_ACTIVE")).To(Equal("broker,consolidated-auth"))

		deployment := fetchDeployment(gatewayKey)
		expectControlledBy(deployment, cluster)
		Expect(deployment.Labels).To(HaveKeyWithValue(labels.ComponentKey, components.ComponentGateway))

		var zeebeService, gatewayService corev1.Service
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, zeebeKey, &zeebeService)).To(Succeed())
			g.Expect(k8sClient.Get(ctx, gatewayKey, &gatewayService)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Expect(zeebeService.Spec.ClusterIP).To(Equal(corev1.ClusterIPNone))
		expectControlledBy(&gatewayService, cluster)

		var admin corev1.Secret
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-camunda-admin"}, &admin,
			)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		expectControlledBy(&admin, cluster)
		Expect(admin.Data["username"]).To(Equal([]byte("admin")))
		Expect(admin.Data["password"]).To(HaveLen(32))

		expectCondition(cluster, v1.ConditionZeebeReady, Not(Equal(v1.ReasonHealthy)))
		expectCondition(cluster, v1.ConditionGatewayReady, Not(Equal(v1.ReasonHealthy)))
		expectReady(cluster, metav1.ConditionFalse, Not(Equal(v1.ReasonHealthy)), Not(BeEmpty()))

		stampStatefulSetReady(zeebeKey)
		expectCondition(cluster, v1.ConditionZeebeReady, Equal(v1.ReasonHealthy))
		expectReady(cluster, metav1.ConditionFalse, Not(Equal(v1.ReasonHealthy)), HavePrefix("GatewayReady: "))

		stampDeploymentReady(gatewayKey)
		expectCondition(cluster, v1.ConditionGatewayReady, Equal(v1.ReasonHealthy))
		expectReady(cluster, metav1.ConditionTrue, Equal(v1.ReasonHealthy), Not(BeEmpty()))
	})

	It(
		"reports InvalidReference for a missing preset, platform config, binding, DatabaseConfig, DatabaseServerConfig, ObjectStorageConfig",
		func() {
			ns := newNamespace()
			cfg := createPlatformConfig()
			binding := createBinding(ns, true)
			missing := "missing-" + utilrand.String(8)

			rdbmsBinding := &v1.SecondaryStorageConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "rdbms-" + utilrand.String(8), Namespace: ns},
				Spec: v1.SecondaryStorageConfigSpec{
					Type:  v1.SecondaryStorageTypeRDBMS,
					RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: missing},
				},
			}
			Expect(k8sClient.Create(ctx, rdbmsBinding)).To(Succeed())

			dbConfig := fixtures.DatabaseConfig()
			dbConfig.Namespace = ns
			dbConfig.Spec.ServerRef = missing
			Expect(k8sClient.Create(ctx, dbConfig)).To(Succeed())
			serverlessBinding := &v1.SecondaryStorageConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "rdbms-" + utilrand.String(8), Namespace: ns},
				Spec: v1.SecondaryStorageConfigSpec{
					Type:  v1.SecondaryStorageTypeRDBMS,
					RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: dbConfig.Name},
				},
			}
			Expect(k8sClient.Create(ctx, serverlessBinding)).To(Succeed())

			cases := map[string]func(*v1.CamundaCluster){
				"CamundaClusterPreset":  func(c *v1.CamundaCluster) { c.Spec.PresetRef = missing },
				"CamundaPlatformConfig": func(c *v1.CamundaCluster) { c.Spec.PlatformConfigRef = missing },
				"SecondaryStorageConfig": func(c *v1.CamundaCluster) {
					c.Spec.StorageRef = missing
				},
				"DatabaseConfig": func(c *v1.CamundaCluster) { c.Spec.StorageRef = rdbmsBinding.Name },
				"DatabaseServerConfig": func(c *v1.CamundaCluster) {
					c.Spec.StorageRef = serverlessBinding.Name
				},
				"ObjectStorageConfig": func(c *v1.CamundaCluster) { c.Spec.BackupStorageRef = missing },
			}
			for kind, mutate := range cases {
				cluster := newCluster(ns, cfg, binding)
				mutate(cluster)
				createCluster(cluster)

				expectReady(
					cluster, metav1.ConditionFalse,
					Equal(v1.ReasonInvalidReference),
					And(ContainSubstring(kind), ContainSubstring(missing)),
				)
				Expect(k8sClient.Get(
					ctx, client.ObjectKey{Namespace: ns, Name: cluster.Name + "-zeebe"}, &appsv1.StatefulSet{},
				)).NotTo(Succeed(), kind)
			}
		},
	)

	It("reports InvalidReference with the field names when the merged spec is invalid", func() {
		ns := newNamespace()
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		// Admission sees no replicas, so it cannot compare. The merged spec
		// defaults replicas to 1.
		cluster.Spec.Zeebe = &v1.ZeebeSpec{ReplicationFactor: new(int32(3))}
		createCluster(cluster)

		expectReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonInvalidReference),
			And(HavePrefix("invalid effective spec: "), ContainSubstring("zeebe.replicationFactor 3")),
		)
	})

	It("reports MissingSecret for a missing storage credentials Secret and recovers when it appears", func() {
		ns := newNamespace()
		binding := createBinding(ns, false)
		cluster := newCluster(ns, createPlatformConfig(), binding)
		createCluster(cluster)

		expectReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonMissingSecret),
			ContainSubstring(binding.Spec.Elasticsearch.CredentialsSecretRef.Name),
		)

		createSecret(ns, binding.Spec.Elasticsearch.CredentialsSecretRef.Name, map[string]string{
			"username": "camunda", "password": "es-password",
		})
		fetchStatefulSet(client.ObjectKey{Namespace: ns, Name: cluster.Name + "-zeebe"})
		expectReady(cluster, metav1.ConditionFalse, Not(Equal(v1.ReasonMissingSecret)), Not(BeEmpty()))
	})

	It("keeps the admin password stable across reconciles and regenerates it when the Secret is deleted", func() {
		cluster := createDefaultCluster()
		adminKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-camunda-admin"}

		var admin corev1.Secret
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, adminKey, &admin)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		password := string(admin.Data["password"])

		// Another reconcile through a spec change: the password is kept.
		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.PodLabels = map[string]string{"touch": "1"} })
		Eventually(func(g Gomega) {
			var sts appsv1.StatefulSet
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"}, &sts,
			)).To(Succeed())
			g.Expect(sts.Spec.Template.Labels).To(HaveKeyWithValue("touch", "1"))
		}, timeout, interval).Should(Succeed())
		Expect(k8sClient.Get(ctx, adminKey, &admin)).To(Succeed())
		Expect(string(admin.Data["password"])).To(Equal(password))

		Expect(k8sClient.Delete(ctx, &admin)).To(Succeed())
		Eventually(func(g Gomega) {
			var recreated corev1.Secret
			g.Expect(k8sClient.Get(ctx, adminKey, &recreated)).To(Succeed())
			g.Expect(recreated.UID).NotTo(Equal(admin.UID))
			g.Expect(recreated.Data["password"]).To(HaveLen(32))
			g.Expect(string(recreated.Data["password"])).NotTo(Equal(password))
		}, timeout, interval).Should(Succeed())
	})

	It("does nothing while paused, not even status", func() {
		ns := newNamespace()
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		cluster.Spec.Pause = true
		createCluster(cluster)
		zeebeKey := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-zeebe"}

		expectEvent(cluster, "Paused", corev1.EventTypeNormal)
		Consistently(func(g Gomega) {
			var latest v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			g.Expect(latest.Status.Conditions).To(BeEmpty())
			g.Expect(latest.Status.ObservedGeneration).To(BeZero())
			g.Expect(k8sClient.Get(ctx, zeebeKey, &appsv1.StatefulSet{})).NotTo(Succeed())
		}, 2*time.Second, interval).Should(Succeed())

		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.Pause = false })
		fetchStatefulSet(zeebeKey)
	})

	It("suspends every workload to zero replicas and reports Ready True Suspended, then resumes", func() {
		cluster := createDefaultCluster()
		zeebeKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"}
		gatewayKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-gateway"}
		fetchStatefulSet(zeebeKey)
		fetchDeployment(gatewayKey)

		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.Suspend = true })
		Eventually(func(g Gomega) {
			g.Expect(*fetchStatefulSet(zeebeKey).Spec.Replicas).To(BeZero())
			g.Expect(*fetchDeployment(gatewayKey).Spec.Replicas).To(BeZero())
		}, timeout, interval).Should(Succeed())
		expectReady(cluster, metav1.ConditionTrue, Equal(string(component.Suspended)), Not(BeEmpty()))

		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.Suspend = false })
		Eventually(func(g Gomega) {
			g.Expect(*fetchStatefulSet(zeebeKey).Spec.Replicas).To(Equal(int32(1)))
			g.Expect(*fetchDeployment(gatewayKey).Spec.Replicas).To(Equal(int32(1)))
		}, timeout, interval).Should(Succeed())
		// ocf keeps the status of a resumed condition until the workload is
		// healthy again, so only the reason is asserted before stamping.
		expectCondition(cluster, v1.ConditionReady, Not(Equal(string(component.Suspended))))
		stampStatefulSetReady(zeebeKey)
		stampDeploymentReady(gatewayKey)
		expectReady(cluster, metav1.ConditionTrue, Equal(v1.ReasonHealthy), Not(BeEmpty()))
	})

	It("rolls the workloads when the platform config, the preset, the binding, or a referenced Secret changes", func() {
		ns := newNamespace()
		cfg := createPlatformConfig()
		binding := createBinding(ns, true)
		preset := minimalPreset()
		Expect(k8sClient.Create(ctx, preset)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, preset) })
		cluster := newCluster(ns, cfg, binding)
		cluster.Spec.PresetRef = preset.Name
		createCluster(cluster)
		hash := configHash(cluster, "")

		By("editing the platform config")
		Eventually(func(g Gomega) {
			var latest v1.CamundaPlatformConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cfg), &latest)).To(Succeed())
			latest.Spec.ImageRegistry = "registry.example.com"
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		hash = configHash(cluster, hash)
		Expect(zeebeContainer(cluster).Image).To(Equal("registry.example.com/camunda/camunda:8.9.9"))

		By("editing the preset")
		Eventually(func(g Gomega) {
			var latest v1.CamundaClusterPreset
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(preset), &latest)).To(Succeed())
			latest.Spec.Cluster.Zeebe.Resources = &corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceMemory: resource.MustParse("2Gi")},
			}
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		hash = configHash(cluster, hash)
		Expect(zeebeContainer(cluster).Resources.Requests[corev1.ResourceMemory]).To(Equal(resource.MustParse("2Gi")))

		By("editing the binding")
		Eventually(func(g Gomega) {
			var latest v1.SecondaryStorageConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(binding), &latest)).To(Succeed())
			latest.Spec.Elasticsearch.Endpoint = "https://other-es." + ns + ".svc:9200"
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		hash = configHash(cluster, hash)
		Expect(envValue(zeebeContainer(cluster), "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_URL")).
			To(Equal("https://other-es." + ns + ".svc:9200"))

		By("editing the credentials Secret of the binding")
		Eventually(func(g Gomega) {
			var latest corev1.Secret
			g.Expect(k8sClient.Get(
				ctx,
				client.ObjectKey{Namespace: ns, Name: binding.Spec.Elasticsearch.CredentialsSecretRef.Name},
				&latest,
			)).To(Succeed())
			latest.StringData = map[string]string{"password": "rotated"}
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		configHash(cluster, hash)
	})

	It("mirrors a referenced Secret from another namespace and follows its changes", func() {
		ns := newNamespace()
		sourceNamespace := newNamespace()
		source := createSecret(sourceNamespace, "license", map[string]string{"key": "license-v1", "other": "x"})
		cfg := createPlatformConfig()
		Eventually(func(g Gomega) {
			var latest v1.CamundaPlatformConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cfg), &latest)).To(Succeed())
			latest.Spec.LicenseSecretRef = &v1.SecretKeyRef{Name: source.Name, Namespace: sourceNamespace, Key: "key"}
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		cluster := newCluster(ns, cfg, createBinding(ns, true))
		createCluster(cluster)

		mirrorKey := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-camunda-license"}
		var mirror corev1.Secret
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, mirrorKey, &mirror)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		expectControlledBy(&mirror, cluster)
		Expect(mirror.Data).To(Equal(map[string][]byte{"key": []byte("license-v1")}))
		expectCondition(cluster, v1.ConditionMirroredSecretsReady, Not(BeEmpty()))

		ref := secretKeyRef(zeebeContainer(cluster), "CAMUNDA_LICENSE_KEY")
		Expect(ref).NotTo(BeNil())
		Expect(ref.Name).To(Equal(mirrorKey.Name))
		Expect(ref.Key).To(Equal("key"))
		hash := configHash(cluster, "")

		Eventually(func(g Gomega) {
			var latest corev1.Secret
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(source), &latest)).To(Succeed())
			latest.StringData = map[string]string{"key": "license-v2"}
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		configHash(cluster, hash)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, mirrorKey, &mirror)).To(Succeed())
			g.Expect(mirror.Data["key"]).To(Equal([]byte("license-v2")))
		}, timeout, interval).Should(Succeed())
	})
})
