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

package camundaoptimize

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
)

// userManager is the field manager of the entry that a user owns on
// spec.zeebe.extraEnv, next to the entries of the operator.
const userManager = "e2e-user"

// userEnv is the entry that the user owns. The exporter patch must never take
// it away.
var userEnv = corev1.EnvVar{Name: "USER_MARKER", Value: "keep-me"}

// scenario is a resolved set of fixtures: a cluster on Elasticsearch, a
// Management Identity contract, and a CamundaOptimize attached to both.
type scenario struct {
	namespace string
	cluster   *v1.CamundaCluster
	binding   *v1.SecondaryStorageConfig
	auth      *v1.ManagementAuthConfig
	optimize  *v1.CamundaOptimize
}

// newNamespace creates a uniquely named Namespace and registers its deletion.
func newNamespace() string {
	GinkgoHelper()
	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "co-ns-" + utilrand.String(8)}}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })

	return ns.Name
}

// createSecret creates a Secret with the given string data and registers its
// deletion.
func createSecret(namespace, name string, data map[string]string) {
	GinkgoHelper()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		StringData: data,
	}
	Expect(k8sClient.Create(ctx, secret)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })
}

// createBinding creates an Elasticsearch binding with its credentials Secret.
func createBinding(namespace string) *v1.SecondaryStorageConfig {
	GinkgoHelper()
	binding := fixtures.SecondaryStorageConfigElasticsearch(namespace)
	Expect(k8sClient.Create(ctx, binding)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, binding) })
	createSecret(namespace, binding.Spec.Elasticsearch.CredentialsSecretRef.Name, map[string]string{
		"username": "camunda", "password": "es-password",
	})

	return binding
}

// createRDBMSBinding creates a relational binding, which Optimize cannot read.
func createRDBMSBinding(namespace string) *v1.SecondaryStorageConfig {
	GinkgoHelper()
	binding := &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "ssc-" + utilrand.String(8), Namespace: namespace},
		Spec: v1.SecondaryStorageConfigSpec{
			Type:  v1.SecondaryStorageTypeRDBMS,
			RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: "my-database"},
		},
	}
	Expect(k8sClient.Create(ctx, binding)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, binding) })

	return binding
}

// createAuth creates a Management Identity contract with its client Secret in
// namespace.
func createAuth(namespace string, withSecret bool) *v1.ManagementAuthConfig {
	GinkgoHelper()
	name := "mac-" + utilrand.String(8)
	issuer := "https://identity.example.com/realms/camunda"
	auth := &v1.ManagementAuthConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.ManagementAuthConfigSpec{
			BaseURL:   "http://identity." + namespace + ".svc:8080",
			IssuerURL: issuer,
			AuthURL:   issuer + "/protocol/openid-connect/auth",
			TokenURL:  issuer + "/protocol/openid-connect/token",
			JwksURL:   issuer + "/protocol/openid-connect/certs",
			ClientID:  "optimize",
			Audience:  "optimize-api",
			ClientSecretRef: v1.SecretKeyRef{
				Name: name + "-client", Namespace: namespace, Key: "client-secret",
			},
		},
	}
	Expect(k8sClient.Create(ctx, auth)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, auth) })
	if withSecret {
		createSecret(namespace, name+"-client", map[string]string{"client-secret": "s3cret"})
	}

	return auth
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

// createCluster creates a CamundaCluster on binding, with the user entry
// already applied to spec.zeebe.extraEnv under its own field manager.
func createCluster(namespace string, binding *v1.SecondaryStorageConfig) *v1.CamundaCluster {
	GinkgoHelper()
	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cc-" + utilrand.String(8), Namespace: namespace},
		Spec: v1.CamundaClusterSpec{
			PlatformConfigRef: createPlatformConfig().Name,
			Version:           "8.9.9",
			StorageRef:        binding.Name,
		},
	}
	Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, cluster) })
	applyUserEnv(cluster)

	return cluster
}

// applyUserEnv applies the user entry to spec.zeebe.extraEnv under its own
// field manager, the way a user or a GitOps tool would.
func applyUserEnv(cluster *v1.CamundaCluster) {
	GinkgoHelper()
	patch := &v1.CamundaCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaCluster"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		},
		Spec: v1.CamundaClusterSpec{
			Zeebe: &v1.ZeebeSpec{WorkloadSpec: v1.WorkloadSpec{ExtraEnv: []corev1.EnvVar{userEnv}}},
		},
	}

	//nolint:staticcheck // the repository applies through the deprecated client.Apply patch
	Expect(k8sClient.Patch(ctx, patch, client.Apply, client.FieldOwner(userManager))).To(Succeed())
}

// createOptimize creates a CamundaOptimize attached to cluster and auth, and
// registers its deletion.
func createOptimize(
	namespace string,
	cluster *v1.CamundaCluster,
	auth *v1.ManagementAuthConfig,
	version string,
) *v1.CamundaOptimize {
	GinkgoHelper()
	optimize := &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{Name: "co-" + utilrand.String(8), Namespace: namespace},
		Spec: v1.CamundaOptimizeSpec{
			Version:           version,
			ManagementAuthRef: auth.Name,
			ClusterRef:        v1.ClusterRef{Name: cluster.Name},
		},
	}
	Expect(k8sClient.Create(ctx, optimize)).To(Succeed())
	DeferCleanup(func() { _ = deleteOptimize(optimize) })

	return optimize
}

// deleteOptimize deletes a CamundaOptimize and waits for the finalizer to let
// it go, so the namespace cleanup of a spec never blocks.
func deleteOptimize(optimize *v1.CamundaOptimize) error {
	if err := k8sClient.Delete(ctx, optimize); err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	Eventually(func() bool {
		var latest v1.CamundaOptimize
		return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(optimize), &latest))
	}, timeout, interval).Should(BeTrue())

	return nil
}

// newScenario builds a full set of fixtures in a fresh namespace.
func newScenario(version string) scenario {
	GinkgoHelper()
	ns := newNamespace()
	binding := createBinding(ns)
	cluster := createCluster(ns, binding)
	auth := createAuth(ns, true)

	return scenario{
		namespace: ns,
		cluster:   cluster,
		binding:   binding,
		auth:      auth,
		optimize:  createOptimize(ns, cluster, auth, version),
	}
}

// expectReady polls until the Ready condition of optimize has the given status
// and its reason matches.
func expectReady(optimize *v1.CamundaOptimize, status metav1.ConditionStatus, reason gomegatypes.GomegaMatcher) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaOptimize
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(optimize), &latest)).To(Succeed())
		ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Status).To(Equal(status))
		g.Expect(ready.Reason).To(reason)
	}, timeout, interval).Should(Succeed())
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

// stampDeploymentReady writes the status a Deployment controller would write
// when every replica is ready. envtest runs no such controller.
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

// clusterEnvNames polls until the exporter entries of the cluster satisfy
// match, and reports the names on the list.
func expectClusterEnv(cluster *v1.CamundaCluster, match gomegatypes.GomegaMatcher) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		names := []string{}
		if latest.Spec.Zeebe != nil {
			for _, env := range latest.Spec.Zeebe.ExtraEnv {
				names = append(names, env.Name)
			}
		}
		g.Expect(names).To(match)
	}, timeout, interval).Should(Succeed())
}

var _ = Describe("CamundaOptimize controller", func() {
	Context("with every reference resolved", func() {
		It("deploys the workloads, patches the exporter, and reports Ready", func() {
			s := newScenario("8.9.4")

			webappKey := client.ObjectKey{
				Namespace: s.namespace,
				Name:      components.WorkloadName(s.optimize, components.ComponentWebapp),
			}
			importerKey := client.ObjectKey{
				Namespace: s.namespace,
				Name:      components.WorkloadName(s.optimize, components.ComponentImporter),
			}

			By("rendering both Deployments with the Optimize image and the cluster label")
			webapp := fetchDeployment(webappKey)
			Expect(webapp.Spec.Template.Spec.Containers[0].Image).To(Equal("camunda/optimize:8.9.4"))
			Expect(webapp.Labels).To(HaveKeyWithValue("camunda.io/cluster", s.cluster.Name))
			Expect(webapp.Labels).To(HaveKeyWithValue("camunda.io/component", "optimize-webapp"))
			Expect(metav1.GetControllerOf(webapp).Kind).To(Equal("CamundaOptimize"))

			importer := fetchDeployment(importerKey)
			Expect(importer.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
				corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_ZEEBE_ENABLED", Value: "true"},
			))
			Expect(webapp.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
				corev1.EnvVar{Name: "CAMUNDA_OPTIMIZE_ZEEBE_ENABLED", Value: "false"},
			))

			By("adding the exporter entries next to the entry of the user")
			expectClusterEnv(s.cluster, ConsistOf(
				userEnv.Name,
				"CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_CLASSNAME",
				"CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_ARGS_URL",
				"CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_ARGS_INDEX_PREFIX",
				"CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_ARGS_AUTHENTICATION_USERNAME",
				"CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_ARGS_AUTHENTICATION_PASSWORD",
			))

			By("reporting Ready once both Deployments report their replicas ready")
			stampDeploymentReady(webappKey)
			stampDeploymentReady(importerKey)
			expectReady(s.optimize, metav1.ConditionTrue, Equal("Healthy"))
		})

		It("withdraws the exporter entries on deletion and keeps the entry of the user", func() {
			s := newScenario("8.9.4")
			expectClusterEnv(s.cluster, ContainElement("CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_CLASSNAME"))

			Expect(deleteOptimize(s.optimize)).To(Succeed())

			expectClusterEnv(s.cluster, ConsistOf(userEnv.Name))
		})
	})

	Context("with a broken reference", func() {
		It("reports InvalidReference for a cluster that does not exist", func() {
			ns := newNamespace()
			auth := createAuth(ns, true)
			optimize := createOptimize(
				ns, &v1.CamundaCluster{
					ObjectMeta: metav1.ObjectMeta{Name: "missing", Namespace: ns},
				}, auth, "8.9.4",
			)

			expectReady(optimize, metav1.ConditionFalse, Equal(v1.ReasonInvalidReference))
		})

		It("reports StorageTypeMismatch for a cluster on relational storage", func() {
			ns := newNamespace()
			cluster := createCluster(ns, createRDBMSBinding(ns))
			optimize := createOptimize(ns, cluster, createAuth(ns, true), "8.9.4")

			expectReady(optimize, metav1.ConditionFalse, Equal(v1.ReasonStorageTypeMismatch))
		})

		It("reports VersionMismatch when the minors differ", func() {
			s := newScenario("8.8.1")

			expectReady(s.optimize, metav1.ConditionFalse, Equal(v1.ReasonVersionMismatch))
		})

		It("reports MissingSecret when the client Secret is absent", func() {
			ns := newNamespace()
			cluster := createCluster(ns, createBinding(ns))
			optimize := createOptimize(ns, cluster, createAuth(ns, false), "8.9.4")

			expectReady(optimize, metav1.ConditionFalse, Equal(v1.ReasonMissingSecret))
		})
	})
})
