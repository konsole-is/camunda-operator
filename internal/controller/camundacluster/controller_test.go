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
	"maps"
	"slices"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

// countEvents returns the number of times an event with the given reason was
// recorded for cluster: the sum of the counts of the matching Event objects,
// because the recorder aggregates repeats of the same event into one object.
func countEvents(cluster *v1.CamundaCluster, reason string) int32 {
	GinkgoHelper()
	var events corev1.EventList
	Expect(k8sClient.List(ctx, &events, client.InNamespace(cluster.Namespace))).To(Succeed())
	var count int32
	for _, event := range events.Items {
		if event.Reason == reason && event.InvolvedObject.Name == cluster.Name {
			count += max(event.Count, 1)
		}
	}
	return count
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

// createExpandableStorageClass creates a StorageClass that allows volume
// expansion and registers its deletion. The resize admission of the API
// server accepts a claim growth only for such a class.
func createExpandableStorageClass() *storagev1.StorageClass {
	GinkgoHelper()
	sc := &storagev1.StorageClass{
		ObjectMeta:           metav1.ObjectMeta{Name: "sc-" + utilrand.String(8)},
		Provisioner:          "test.camunda.io/none",
		AllowVolumeExpansion: new(true),
	}
	Expect(k8sClient.Create(ctx, sc)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, sc) })
	return sc
}

// createBoundBrokerClaim creates a broker claim of cluster with the given
// size, bound with the same capacity, as the StatefulSet controller and a
// provisioner would have left it. envtest runs neither.
func createBoundBrokerClaim(
	cluster *v1.CamundaCluster,
	ordinal, storageClass, size string,
) *corev1.PersistentVolumeClaim {
	GinkgoHelper()
	claim := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      components.DataVolumeName + "-" + cluster.Name + "-zeebe-" + ordinal,
			Namespace: cluster.Namespace,
			Labels:    components.BrokerClaimSelector(cluster),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			StorageClassName: new(storageClass),
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)},
			},
		},
	}
	Expect(k8sClient.Create(ctx, claim)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, claim) })
	stampClaimCapacity(claim, size)
	return claim
}

// stampClaimCapacity marks claim bound with the given capacity.
func stampClaimCapacity(claim *corev1.PersistentVolumeClaim, capacity string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest corev1.PersistentVolumeClaim
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &latest)).To(Succeed())
		latest.Status.Phase = corev1.ClaimBound
		latest.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(capacity)}
		g.Expect(k8sClient.Status().Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// expectVolumes polls until status.volumes of cluster lists exactly the
// given claim names with the given capacities, in name order.
func expectVolumes(cluster *v1.CamundaCluster, want map[string]string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		g.Expect(latest.Status.Volumes).To(HaveLen(len(want)))
		for i, volume := range latest.Status.Volumes {
			g.Expect(volume.Name).To(Equal(slices.Sorted(maps.Keys(want))[i]))
			g.Expect(volume.Capacity.Cmp(resource.MustParse(want[volume.Name]))).To(BeZero(), volume.Name)
		}
	}, timeout, interval).Should(Succeed())
}

// claimTemplateSize returns the storage request of the data volume claim
// template of sts.
func claimTemplateSize(sts *appsv1.StatefulSet) resource.Quantity {
	return sts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]
}

// updatePresetStorageSize sets the zeebe storageSize of preset.
func updatePresetStorageSize(preset *v1.CamundaClusterPreset, size string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaClusterPreset
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(preset), &latest)).To(Succeed())
		latest.Spec.Cluster.Zeebe.StorageSize = new(resource.MustParse(size))
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
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

	It("deletes what a topology or auth change no longer needs and reports Disabled for it", func() {
		cluster := createDefaultCluster()
		operateKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-operate"}
		connectorsKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-connectors"}
		adminKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-camunda-admin"}
		fetchStatefulSet(client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"})
		expectCondition(cluster, v1.ConditionOperateReady, Equal(string(component.Disabled)))
		expectCondition(cluster, v1.ConditionConnectorsReady, Equal(string(component.Disabled)))

		By("running Operate standalone and connectors")
		updateCluster(cluster, func(c *v1.CamundaCluster) {
			c.Spec.Operate = &v1.WebAppSpec{Mode: v1.ComponentModeStandalone}
			c.Spec.Connectors = &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"}
		})
		fetchDeployment(operateKey)
		fetchDeployment(connectorsKey)
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, operateKey, &corev1.Service{})).To(Succeed())
		}, timeout, interval).Should(Succeed())
		expectCondition(cluster, v1.ConditionOperateReady, Not(Equal(string(component.Disabled))))
		expectCondition(cluster, v1.ConditionConnectorsReady, Not(Equal(string(component.Disabled))))

		By("embedding Operate again and disabling connectors")
		updateCluster(cluster, func(c *v1.CamundaCluster) {
			c.Spec.Operate.Mode = v1.ComponentModeEmbedded
			c.Spec.Connectors.Enabled = new(false)
		})
		Eventually(func(g Gomega) {
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, operateKey, &appsv1.Deployment{}))).To(BeTrue())
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, operateKey, &corev1.Service{}))).To(BeTrue())
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, connectorsKey, &appsv1.Deployment{}))).To(BeTrue())
		}, timeout, interval).Should(Succeed())
		expectCondition(cluster, v1.ConditionOperateReady, Equal(string(component.Disabled)))
		expectCondition(cluster, v1.ConditionConnectorsReady, Equal(string(component.Disabled)))

		By("switching the platform config from basic to oidc")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, adminKey, &corev1.Secret{})).To(Succeed())
		}, timeout, interval).Should(Succeed())
		oidcSecret := createSecret(cluster.Namespace, "oidc", map[string]string{"client-secret": "s"})
		Eventually(func(g Gomega) {
			var latest v1.CamundaPlatformConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: cluster.Spec.PlatformConfigRef}, &latest)).To(Succeed())
			latest.Spec.Auth = &v1.PlatformAuthSpec{
				Method: v1.AuthenticationMethodOIDC,
				OIDC: &v1.OIDCSpec{
					IssuerURL: "https://idp.example.com/realms/camunda",
					ClientID:  "platform-client",
					ClientSecretRef: v1.SecretKeyRef{
						Name: oidcSecret.Name, Namespace: cluster.Namespace, Key: "client-secret",
					},
				},
			}
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, adminKey, &corev1.Secret{}))).To(BeTrue())
		}, timeout, interval).Should(Succeed())
		expectCondition(cluster, v1.ConditionAdminSecretReady, Equal(string(component.Disabled)))
		Expect(envValue(zeebeContainer(cluster), "CAMUNDA_SECURITY_AUTHENTICATION_METHOD")).To(Equal("oidc"))
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
		// ocf keeps the True status of a resumed condition and reports
		// Updating until the workload is healthy again.
		expectReady(cluster, metav1.ConditionTrue, Equal(string(component.AliveUpdating)), Not(BeEmpty()))
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

	It("rolls an RDBMS cluster when its DatabaseServerConfig or DatabaseConfig changes", func() {
		ns := newNamespace()
		server := fixtures.DatabaseServerConfig()
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, server) })
		dbConfig := fixtures.DatabaseConfig()
		dbConfig.Namespace = ns
		dbConfig.Spec.ServerRef = server.Name
		dbConfig.Spec.CredentialsSecretRef.Namespace = ns
		Expect(k8sClient.Create(ctx, dbConfig)).To(Succeed())
		createSecret(ns, dbConfig.Spec.CredentialsSecretRef.Name, map[string]string{
			"username": "camunda", "password": "db-password",
		})
		binding := &v1.SecondaryStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "rdbms-" + utilrand.String(8), Namespace: ns},
			Spec: v1.SecondaryStorageConfigSpec{
				Type:  v1.SecondaryStorageTypeRDBMS,
				RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: dbConfig.Name},
			},
		}
		Expect(k8sClient.Create(ctx, binding)).To(Succeed())
		cluster := newCluster(ns, createPlatformConfig(), binding)
		createCluster(cluster)

		hash := configHash(cluster, "")
		Expect(envValue(zeebeContainer(cluster), "CAMUNDA_DATA_SECONDARYSTORAGE_RDBMS_URL")).
			To(Equal("jdbc:postgresql://" + server.Spec.Host + ":5432/camunda"))

		By("editing the DatabaseServerConfig host")
		Eventually(func(g Gomega) {
			var latest v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(server), &latest)).To(Succeed())
			latest.Spec.Host = "other-postgres." + ns + ".svc"
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		hash = configHash(cluster, hash)
		Expect(envValue(zeebeContainer(cluster), "CAMUNDA_DATA_SECONDARYSTORAGE_RDBMS_URL")).
			To(Equal("jdbc:postgresql://other-postgres." + ns + ".svc:5432/camunda"))

		By("editing the DatabaseConfig database name")
		Eventually(func(g Gomega) {
			var latest v1.DatabaseConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(dbConfig), &latest)).To(Succeed())
			latest.Spec.DatabaseName = "camunda2"
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		configHash(cluster, hash)
		Expect(envValue(zeebeContainer(cluster), "CAMUNDA_DATA_SECONDARYSTORAGE_RDBMS_URL")).
			To(Equal("jdbc:postgresql://other-postgres." + ns + ".svc:5432/camunda2"))
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

	It(
		"grows the broker claims in place, keeps the StatefulSet, reports every bound claim, and ignores a shrink once",
		func() {
			ns := newNamespace()
			sc := createExpandableStorageClass()
			preset := minimalPreset()
			Expect(k8sClient.Create(ctx, preset)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, preset) })
			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			cluster.Spec.PresetRef = preset.Name
			cluster.Spec.Zeebe = &v1.ZeebeSpec{StorageClassName: new(sc.Name)}
			createCluster(cluster)
			zeebeKey := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-zeebe"}

			sts := fetchStatefulSet(zeebeKey)
			Expect(claimTemplateSize(sts)).To(Equal(resource.MustParse("10Gi")))
			Expect(sts.Annotations).To(HaveKeyWithValue(components.RequestedStorageSizeAnnotation, "10Gi"))
			claim0 := createBoundBrokerClaim(cluster, "0", sc.Name, "10Gi")
			claim1 := createBoundBrokerClaim(cluster, "1", sc.Name, "10Gi")
			expectVolumes(cluster, map[string]string{claim0.Name: "10Gi", claim1.Name: "10Gi"})

			By("growing the storage size: the claims grow, the StatefulSet and its template stay")
			updatePresetStorageSize(preset, "20Gi")
			for _, claim := range []*corev1.PersistentVolumeClaim{claim0, claim1} {
				Eventually(func(g Gomega) {
					var latest corev1.PersistentVolumeClaim
					g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &latest)).To(Succeed())
					g.Expect(latest.Spec.Resources.Requests[corev1.ResourceStorage]).
						To(Equal(resource.MustParse("20Gi")))
				}, timeout, interval).Should(Succeed())
			}
			Eventually(func(g Gomega) {
				var latest appsv1.StatefulSet
				g.Expect(k8sClient.Get(ctx, zeebeKey, &latest)).To(Succeed())
				g.Expect(latest.Annotations).To(HaveKeyWithValue(components.RequestedStorageSizeAnnotation, "20Gi"))
			}, timeout, interval).Should(Succeed())
			Consistently(func(g Gomega) {
				var latest appsv1.StatefulSet
				g.Expect(k8sClient.Get(ctx, zeebeKey, &latest)).To(Succeed())
				g.Expect(latest.UID).To(Equal(sts.UID))
				g.Expect(latest.DeletionTimestamp).To(BeNil())
				g.Expect(claimTemplateSize(&latest)).To(Equal(resource.MustParse("10Gi")))
			}, 2*time.Second, interval).Should(Succeed())

			By("growing a claim that binds later, as a new replica would")
			claim2 := createBoundBrokerClaim(cluster, "2", sc.Name, "10Gi")
			Eventually(func(g Gomega) {
				var latest corev1.PersistentVolumeClaim
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim2), &latest)).To(Succeed())
				g.Expect(latest.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("20Gi")))
			}, timeout, interval).Should(Succeed())

			By("reporting every bound claim with its own capacity")
			stampClaimCapacity(claim0, "20Gi")
			expectVolumes(cluster, map[string]string{claim0.Name: "20Gi", claim1.Name: "10Gi", claim2.Name: "10Gi"})
			stampClaimCapacity(claim1, "20Gi")
			stampClaimCapacity(claim2, "20Gi")
			expectVolumes(cluster, map[string]string{claim0.Name: "20Gi", claim1.Name: "20Gi", claim2.Name: "20Gi"})

			By("ignoring a preset shrink once: the claims and the template keep their size")
			updatePresetStorageSize(preset, "5Gi")
			expectEvent(cluster, "StorageShrinkIgnored", corev1.EventTypeWarning)
			Eventually(func(g Gomega) {
				var latest appsv1.StatefulSet
				g.Expect(k8sClient.Get(ctx, zeebeKey, &latest)).To(Succeed())
				g.Expect(latest.Annotations).To(HaveKeyWithValue(components.RequestedStorageSizeAnnotation, "5Gi"))
			}, timeout, interval).Should(Succeed())
			// A second reconcile after the annotation is applied records nothing.
			updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.PodLabels = map[string]string{"touch": "1"} })
			Eventually(func(g Gomega) {
				var latest appsv1.StatefulSet
				g.Expect(k8sClient.Get(ctx, zeebeKey, &latest)).To(Succeed())
				g.Expect(latest.Spec.Template.Labels).To(HaveKeyWithValue("touch", "1"))
			}, timeout, interval).Should(Succeed())
			Consistently(func(g Gomega) {
				g.Expect(countEvents(cluster, "StorageShrinkIgnored")).To(Equal(int32(1)))
				var latest appsv1.StatefulSet
				g.Expect(k8sClient.Get(ctx, zeebeKey, &latest)).To(Succeed())
				g.Expect(latest.UID).To(Equal(sts.UID))
				g.Expect(claimTemplateSize(&latest)).To(Equal(resource.MustParse("10Gi")))
				var claim corev1.PersistentVolumeClaim
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim0), &claim)).To(Succeed())
				g.Expect(claim.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("20Gi")))
			}, 2*time.Second, interval).Should(Succeed())
		},
	)

	It("mirrors the OIDC client secret that a preset references and follows its changes", func() {
		ns := newNamespace()
		sourceNamespace := newNamespace()
		platformSecret := createSecret(sourceNamespace, "platform-oidc", map[string]string{"client-secret": "platform"})
		presetSecret := createSecret(sourceNamespace, "preset-oidc", map[string]string{"client-secret": "preset-v1"})
		cfg := createPlatformConfig()
		Eventually(func(g Gomega) {
			var latest v1.CamundaPlatformConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cfg), &latest)).To(Succeed())
			latest.Spec.Auth = &v1.PlatformAuthSpec{
				Method: v1.AuthenticationMethodOIDC,
				OIDC: &v1.OIDCSpec{
					IssuerURL: "https://idp.example.com/realms/camunda",
					ClientID:  "platform-client",
					ClientSecretRef: v1.SecretKeyRef{
						Name: platformSecret.Name, Namespace: sourceNamespace, Key: "client-secret",
					},
				},
			}
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		preset := minimalPreset()
		preset.Spec.Cluster.Auth = &v1.ClusterAuthSpec{
			ClientID: "preset-client",
			ClientSecretRef: &v1.SecretKeyRef{
				Name: presetSecret.Name, Namespace: sourceNamespace, Key: "client-secret",
			},
		}
		Expect(k8sClient.Create(ctx, preset)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, preset) })
		cluster := newCluster(ns, cfg, createBinding(ns, true))
		cluster.Spec.PresetRef = preset.Name
		createCluster(cluster)

		container := zeebeContainer(cluster)
		Expect(envValue(container, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_CLIENTID")).To(Equal("preset-client"))
		ref := secretKeyRef(container, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_CLIENTSECRET")
		Expect(ref).NotTo(BeNil())
		Expect(ref.Name).To(Equal(cluster.Name + "-camunda-auth-client"))
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{Namespace: ns, Name: cluster.Name + "-camunda-admin"}, &corev1.Secret{},
		)).NotTo(Succeed(), "no admin Secret under oidc")
		hash := configHash(cluster, "")

		Eventually(func(g Gomega) {
			var latest corev1.Secret
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(presetSecret), &latest)).To(Succeed())
			latest.StringData = map[string]string{"client-secret": "preset-v2"}
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		configHash(cluster, hash)
		Eventually(func(g Gomega) {
			var mirror corev1.Secret
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: ref.Name}, &mirror)).To(Succeed())
			g.Expect(mirror.Data["client-secret"]).To(Equal([]byte("preset-v2")))
		}, timeout, interval).Should(Succeed())
	})

	It("mirrors the credentials Secret of a binding from another namespace and follows its changes", func() {
		ns := newNamespace()
		sourceNamespace := newNamespace()
		binding := fixtures.SecondaryStorageConfigElasticsearch(ns)
		binding.Spec.Elasticsearch.CredentialsSecretRef.Namespace = sourceNamespace
		Expect(k8sClient.Create(ctx, binding)).To(Succeed())
		source := createSecret(sourceNamespace, binding.Spec.Elasticsearch.CredentialsSecretRef.Name, map[string]string{
			"username": "camunda", "password": "v1",
		})
		cluster := newCluster(ns, createPlatformConfig(), binding)
		createCluster(cluster)

		mirrorKey := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-camunda-es-credentials"}
		Eventually(func(g Gomega) {
			var mirror corev1.Secret
			g.Expect(k8sClient.Get(ctx, mirrorKey, &mirror)).To(Succeed())
			g.Expect(mirror.Data["password"]).To(Equal([]byte("v1")))
		}, timeout, interval).Should(Succeed())
		ref := secretKeyRef(zeebeContainer(cluster), "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_PASSWORD")
		Expect(ref).NotTo(BeNil())
		Expect(ref.Name).To(Equal(mirrorKey.Name))
		hash := configHash(cluster, "")

		Eventually(func(g Gomega) {
			var latest corev1.Secret
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(source), &latest)).To(Succeed())
			latest.StringData = map[string]string{"password": "v2"}
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		configHash(cluster, hash)
		Eventually(func(g Gomega) {
			var mirror corev1.Secret
			g.Expect(k8sClient.Get(ctx, mirrorKey, &mirror)).To(Succeed())
			g.Expect(mirror.Data["password"]).To(Equal([]byte("v2")))
		}, timeout, interval).Should(Succeed())
	})
})
