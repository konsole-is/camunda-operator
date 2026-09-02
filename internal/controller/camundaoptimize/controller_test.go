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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gomegatypes "github.com/onsi/gomega/types"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
	clustercomponents "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
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

// createBinding creates an Elasticsearch binding in namespace, with its
// credentials Secret. The binding resolves the Secret in its own namespace.
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

// createAuth creates a Management Identity contract whose client Secret lives
// in namespace.
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
	Expect(applyZeebeEnv(cluster, userManager, userEnv)).To(Succeed())
}

// applyZeebeEnv applies one spec.zeebe.extraEnv entry to cluster under the
// given field manager.
func applyZeebeEnv(cluster *v1.CamundaCluster, manager string, env corev1.EnvVar) error {
	patch := &v1.CamundaCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaCluster"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
		},
		Spec: v1.CamundaClusterSpec{
			Zeebe: &v1.ZeebeSpec{WorkloadSpec: v1.WorkloadSpec{ExtraEnv: []corev1.EnvVar{env}}},
		},
	}

	//nolint:staticcheck // the repository applies through the deprecated client.Apply patch
	return k8sClient.Patch(ctx, patch, client.Apply, client.FieldOwner(manager))
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

	return createNamedOptimize("co-"+utilrand.String(8), namespace, cluster, auth, version)
}

// createNamedOptimize is createOptimize with an explicit name. A
// creationTimestamp carries whole seconds, so two CamundaOptimizes that a
// test creates in one go are equally old and the name decides which of them
// holds the attachment.
func createNamedOptimize(
	name, namespace string,
	cluster *v1.CamundaCluster,
	auth *v1.ManagementAuthConfig,
	version string,
) *v1.CamundaOptimize {
	GinkgoHelper()
	optimize := &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
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

// createDanglingOptimize creates a CamundaOptimize whose clusterRef names no
// cluster. The controller reports InvalidReference and builds nothing, so a
// spec can stage workloads under it and they stay as staged.
//
// Each one names a cluster of its own. Two that named the same cluster would
// contend for it, and the controller would park the loser and release the
// workloads that the spec staged under it.
func createDanglingOptimize(name, namespace string) *v1.CamundaOptimize {
	GinkgoHelper()
	optimize := &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1.CamundaOptimizeSpec{
			Version:           "8.9.4",
			ManagementAuthRef: "no-such-auth",
			ClusterRef:        v1.ClusterRef{Name: "no-such-cluster-" + name},
		},
	}
	Expect(k8sClient.Create(ctx, optimize)).To(Succeed())
	DeferCleanup(func() { _ = deleteOptimize(optimize) })

	return optimize
}

// stageWorkload creates the Deployment and the Service that the renderer gives
// a component of named, under the control of owner, and returns their shared
// key. The two are separate so that a spec can stage an object at the name of
// one CamundaOptimize that another one controls.
func stageWorkload(named, owner *v1.CamundaOptimize, comp string) client.ObjectKey {
	GinkgoHelper()
	key := client.ObjectKey{
		Namespace: named.Namespace,
		Name:      components.WorkloadName(named, comp),
	}
	selector := map[string]string{"camunda.io/component": comp, "camunda.io/optimize": named.Name}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selector},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "optimize", Image: "camunda/optimize:8.9.4"}},
				},
			},
		},
	}
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports:    []corev1.ServicePort{{Name: "http", Port: 8090}},
		},
	}

	for _, obj := range []client.Object{deployment, svc} {
		Expect(controllerutil.SetControllerReference(owner, obj, k8sClient.Scheme())).To(Succeed())
		Expect(k8sClient.Create(ctx, obj)).To(Succeed())
	}

	return key
}

// stageMirroredSecret creates the copy of a referenced Secret that optimize
// makes for a purpose, under its control, and returns its key.
func stageMirroredSecret(optimize *v1.CamundaOptimize, purpose components.MirrorPurpose) client.ObjectKey {
	GinkgoHelper()
	mirrored := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      components.MirroredSecretName(optimize, purpose),
			Namespace: optimize.Namespace,
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{"client-secret": []byte("staged")},
	}
	Expect(controllerutil.SetControllerReference(optimize, mirrored, k8sClient.Scheme())).To(Succeed())
	Expect(k8sClient.Create(ctx, mirrored)).To(Succeed())

	return client.ObjectKeyFromObject(mirrored)
}

// stageForeignImporter creates an importer Deployment of a cluster under the
// control of owner, with the managed labels that the renderer gives it. Every
// CamundaOptimize on one cluster renders the same labels, so this is what the
// holder sees when the previous one has not finished going.
func stageForeignImporter(owner *v1.CamundaOptimize, clusterName string) client.ObjectKey {
	GinkgoHelper()
	selector := labels.Discovery(labels.Cluster(clusterName), components.ComponentImporter)
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      components.WorkloadName(owner, components.ComponentImporter),
			Namespace: owner.Namespace,
			Labels:    labels.Managed(labels.Cluster(clusterName), components.ComponentImporter),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: selector},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: selector},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "optimize", Image: "camunda/optimize:8.9.4"}},
				},
			},
		},
	}
	Expect(controllerutil.SetControllerReference(owner, deployment, k8sClient.Scheme())).To(Succeed())
	Expect(k8sClient.Create(ctx, deployment)).To(Succeed())

	return client.ObjectKeyFromObject(deployment)
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

// newAttachedPair builds a scenario whose CamundaOptimize is named so that it
// holds the attachment, plus a second one that must wait for it.
func newAttachedPair(version string) (scenario, *v1.CamundaOptimize) {
	GinkgoHelper()
	ns := newNamespace()
	binding := createBinding(ns)
	cluster := createCluster(ns, binding)
	auth := createAuth(ns, true)

	holder := createNamedOptimize("co-a-holder", ns, cluster, auth, version)
	waiting := createNamedOptimize("co-b-waiting", ns, cluster, auth, version)

	return scenario{namespace: ns, cluster: cluster, binding: binding, auth: auth, optimize: holder}, waiting
}

// expectNotReady polls until the Ready condition of optimize is False with a
// reason that matches. The True case belongs to expectReadyWhileStamping,
// which has to keep the Deployment status current while it waits.
func expectNotReady(optimize *v1.CamundaOptimize, reason string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaOptimize
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(optimize), &latest)).To(Succeed())
		ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		g.Expect(ready.Reason).To(Equal(reason))
	}, timeout, interval).Should(Succeed())
}

// fetchSecret polls until the Secret exists and returns it.
func fetchSecret(key client.ObjectKey) *corev1.Secret {
	GinkgoHelper()
	var secret corev1.Secret
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, key, &secret)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	return &secret
}

// expectCondition polls until the condition of the given type on optimize has
// a reason that matches.
func expectNotReadyCluster(cluster *v1.CamundaCluster, reason string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Reason).To(Equal(reason))
	}, timeout, interval).Should(Succeed())
}

func expectCondition(optimize *v1.CamundaOptimize, conditionType string, reason gomegatypes.GomegaMatcher) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaOptimize
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(optimize), &latest)).To(Succeed())
		cond := meta.FindStatusCondition(latest.Status.Conditions, conditionType)
		g.Expect(cond).NotTo(BeNil())
		g.Expect(cond.Reason).To(reason)
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
func stampDeploymentReady(g Gomega, key client.ObjectKey) {
	var deployment appsv1.Deployment
	g.Expect(k8sClient.Get(ctx, key, &deployment)).To(Succeed())
	replicas := *deployment.Spec.Replicas
	deployment.Status.ObservedGeneration = deployment.Generation
	deployment.Status.Replicas = replicas
	deployment.Status.ReadyReplicas = replicas
	deployment.Status.UpdatedReplicas = replicas
	deployment.Status.AvailableReplicas = replicas
	g.Expect(k8sClient.Status().Update(ctx, &deployment)).To(Succeed())
}

// expectReadyWhileStamping polls until optimize reports Ready=True/Healthy,
// stamping every Deployment again on each attempt. A stamp names the
// generation it saw, so a re-render after a single stamp leaves that stamp
// stale and the components never read the Deployments as up to date. Only a
// real controller keeps up with a rolling generation, and envtest runs none.
func expectReadyWhileStamping(optimize *v1.CamundaOptimize, keys ...client.ObjectKey) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		for _, key := range keys {
			stampDeploymentReady(g, key)
		}

		var latest v1.CamundaOptimize
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(optimize), &latest)).To(Succeed())
		ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		g.Expect(ready.Reason).To(Equal("Healthy"))
	}, timeout, interval).Should(Succeed())
}

// expectStableRender asserts that nothing re-renders the Deployments: their
// generation and their config hash both hold still. A hash that takes an input
// the controller itself writes never settles, so the pods roll on every
// reconcile and on every unrelated edit of the referenced object.
func expectStableRender(keys ...client.ObjectKey) {
	GinkgoHelper()
	before := map[string]string{}
	generations := map[string]int64{}
	for _, key := range keys {
		deployment := fetchDeployment(key)
		before[key.Name] = deployment.Spec.Template.Annotations[components.ConfigHashAnnotation]
		generations[key.Name] = deployment.Generation
		Expect(before[key.Name]).NotTo(BeEmpty(), key.Name)
	}

	Consistently(func(g Gomega) {
		for _, key := range keys {
			var deployment appsv1.Deployment
			g.Expect(k8sClient.Get(ctx, key, &deployment)).To(Succeed())
			g.Expect(deployment.Spec.Template.Annotations[components.ConfigHashAnnotation]).
				To(Equal(before[key.Name]), key.Name)
			g.Expect(deployment.Generation).To(Equal(generations[key.Name]), key.Name)
		}
	}, 3*time.Second, interval).Should(Succeed())
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

// setClusterSuspend writes spec.suspend on the cluster of a scenario.
func setClusterSuspend(cluster *v1.CamundaCluster, suspend bool) {
	GinkgoHelper()
	Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
		var latest v1.CamundaCluster
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest); err != nil {
			return err
		}
		latest.Spec.Suspend = suspend

		return k8sClient.Update(ctx, &latest)
	})).To(Succeed())
}

// expectReplicas polls until every named Deployment carries want replicas.
func expectReplicas(want int32, keys ...client.ObjectKey) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		for _, key := range keys {
			var deployment appsv1.Deployment
			g.Expect(k8sClient.Get(ctx, key, &deployment)).To(Succeed())
			g.Expect(deployment.Spec.Replicas).NotTo(BeNil(), key.Name)
			g.Expect(*deployment.Spec.Replicas).To(Equal(want), key.Name)
		}
	}, timeout, interval).Should(Succeed())
}

// suspensionEvents returns the reasons of the suspension events recorded on
// optimize, in the order the API server returns them.
func suspensionEvents(optimize *v1.CamundaOptimize) []string {
	GinkgoHelper()
	var events corev1.EventList
	Expect(k8sClient.List(ctx, &events, client.InNamespace(optimize.Namespace))).To(Succeed())

	var reasons []string
	for _, event := range events.Items {
		if event.InvolvedObject.Name != optimize.Name || event.Action != eventActionSuspend {
			continue
		}
		for range max(int(event.Count), 1) {
			reasons = append(reasons, event.Reason)
		}
	}

	return reasons
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
			expectReadyWhileStamping(s.optimize, webappKey, importerKey)

			By("rendering the same pod template on every later reconcile")
			expectStableRender(webappKey, importerKey)

			By("holding the render still across an unrelated edit of the cluster")
			Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
				var latest v1.CamundaCluster
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(s.cluster), &latest); err != nil {
					return err
				}
				latest.Spec.PodLabels = map[string]string{"unrelated": "edit"}
				return k8sClient.Update(ctx, &latest)
			})).To(Succeed())
			expectStableRender(webappKey, importerKey)
		})

		It("follows the suspension of its cluster to zero replicas and back", func() {
			s := newScenario("8.9.4")
			webappKey := client.ObjectKey{
				Namespace: s.namespace,
				Name:      components.WorkloadName(s.optimize, components.ComponentWebapp),
			}
			importerKey := client.ObjectKey{
				Namespace: s.namespace,
				Name:      components.WorkloadName(s.optimize, components.ComponentImporter),
			}
			expectReplicas(1, webappKey, importerKey)

			By("scaling both workloads to zero while the cluster is suspended")
			setClusterSuspend(s.cluster, true)
			expectReplicas(0, webappKey, importerKey)

			By("reporting Ready True with reason Suspended, which is not an error")
			Eventually(func(g Gomega) {
				var latest v1.CamundaOptimize
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(s.optimize), &latest)).To(Succeed())
				ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
				g.Expect(ready).NotTo(BeNil())
				g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
				g.Expect(ready.Reason).To(Equal(string(component.Suspended)))
			}, timeout, interval).Should(Succeed())
			expectCondition(s.optimize, v1.ConditionWebappReady, Equal(string(component.Suspended)))
			expectCondition(s.optimize, v1.ConditionImporterReady, Equal(string(component.Suspended)))

			By("keeping the exporter patch, because the suspension is not a detachment")
			expectClusterEnv(s.cluster, ContainElement("CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_CLASSNAME"))

			By("naming the cluster in an event, once for the transition")
			Expect(suspensionEvents(s.optimize)).To(Equal([]string{eventReasonClusterSuspended}))
			Consistently(func(g Gomega) {
				g.Expect(suspensionEvents(s.optimize)).To(Equal([]string{eventReasonClusterSuspended}))
			}, 3*time.Second, interval).Should(Succeed())

			By("starting both workloads again when the suspension is cleared")
			setClusterSuspend(s.cluster, false)
			expectReplicas(1, webappKey, importerKey)
			expectReadyWhileStamping(s.optimize, webappKey, importerKey)
			Expect(suspensionEvents(s.optimize)).To(Equal([]string{
				eventReasonClusterSuspended, eventReasonClusterResumed,
			}))
		})

		It("scales to zero when another cluster holds the storage contract of its cluster", func() {
			ns := newNamespace()
			binding := createBinding(ns)
			auth := createAuth(ns, true)
			holder := createCluster(ns, binding)

			By("waiting until the holder holds the contract")
			Eventually(func(g Gomega) {
				var latest v1.SecondaryStorageConfig
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(binding), &latest)).To(Succeed())
				claim, held := secondarystorageconfig.HolderOf(&latest)
				g.Expect(held).To(BeTrue())
				g.Expect(claim.Cluster).To(Equal(client.ObjectKeyFromObject(holder)))
			}, timeout, interval).Should(Succeed())

			By("parking the second cluster on the same contract")
			parked := createCluster(ns, binding)
			Eventually(func(g Gomega) {
				var latest v1.CamundaCluster
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(parked), &latest)).To(Succeed())
				ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
				g.Expect(ready).NotTo(BeNil())
				g.Expect(ready.Reason).To(Equal(v1.ReasonStorageAlreadyAttached))
			}, timeout, interval).Should(Succeed())

			optimize := createOptimize(ns, parked, auth, "8.9.4")
			webappKey := client.ObjectKey{
				Namespace: ns,
				Name:      components.WorkloadName(optimize, components.ComponentWebapp),
			}
			importerKey := client.ObjectKey{
				Namespace: ns,
				Name:      components.WorkloadName(optimize, components.ComponentImporter),
			}

			By("scaling both workloads to zero, because the parked cluster runs no workloads")
			expectReplicas(0, webappKey, importerKey)
			expectCondition(optimize, v1.ConditionWebappReady, Equal(string(component.Suspended)))
			expectCondition(optimize, v1.ConditionImporterReady, Equal(string(component.Suspended)))
			Eventually(func(g Gomega) {
				var latest v1.CamundaOptimize
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(optimize), &latest)).To(Succeed())
				ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
				g.Expect(ready).NotTo(BeNil())
				g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
				g.Expect(ready.Reason).To(Equal(string(component.Suspended)))
			}, timeout, interval).Should(Succeed())
		})

		// The cluster waits for the pods of the previous holder of its
		// contract and runs no workloads, so the importer has nothing to
		// import and follows the suspension.
		It("scales to zero while its cluster waits for a storage handover", func() {
			ns := newNamespace()
			binding := createBinding(ns)
			auth := createAuth(ns, true)

			By("claiming the contract for a holder that is gone, with a pod it left behind")
			Eventually(func(g Gomega) {
				var latest v1.SecondaryStorageConfig
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(binding), &latest)).To(Succeed())
				if latest.Annotations == nil {
					latest.Annotations = map[string]string{}
				}
				latest.Annotations[secondarystorageconfig.ClaimHolderAnnotation] = ns + "/ghost"
				latest.Annotations[secondarystorageconfig.ClaimHolderUIDAnnotation] = "ghost-uid"
				g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
			}, timeout, interval).Should(Succeed())
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ghost-zeebe-0",
					Namespace: ns,
					Labels:    clustercomponents.StoragePodLabels("ghost", binding.Name),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "camunda", Image: "camunda/camunda:8.9.9"}},
				},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())

			waiting := createCluster(ns, binding)
			expectNotReadyCluster(waiting, v1.ReasonWaitingForHandover)

			optimize := createOptimize(ns, waiting, auth, "8.9.9")
			webappKey := client.ObjectKey{
				Namespace: ns,
				Name:      components.WorkloadName(optimize, components.ComponentWebapp),
			}
			importerKey := client.ObjectKey{
				Namespace: ns,
				Name:      components.WorkloadName(optimize, components.ComponentImporter),
			}

			By("scaling both workloads to zero while the cluster waits")
			expectReplicas(0, webappKey, importerKey)
			expectCondition(optimize, v1.ConditionImporterReady, Equal(string(component.Suspended)))

			By("starting them when the pod of the previous holder goes")
			Expect(k8sClient.Delete(ctx, pod)).To(Succeed())
			expectReplicas(1, webappKey, importerKey)
		})

		// The cluster stops on this reference too, and reports itself
		// suspended. This controller cannot follow that suspension through
		// the render input, because the reference it fails on is the same
		// one, so the workloads stop on the failed pre-check instead.
		It("scales to zero when the storage contract of its cluster is deleted", func() {
			s := newScenario("8.9.4")
			webappKey := client.ObjectKey{
				Namespace: s.namespace,
				Name:      components.WorkloadName(s.optimize, components.ComponentWebapp),
			}
			importerKey := client.ObjectKey{
				Namespace: s.namespace,
				Name:      components.WorkloadName(s.optimize, components.ComponentImporter),
			}
			expectReplicas(1, webappKey, importerKey)

			By("deleting the contract that the cluster and this instance both resolve")
			Expect(k8sClient.Delete(ctx, s.binding)).To(Succeed())

			By("scaling both workloads to zero and naming that in the Ready message")
			expectReplicas(0, webappKey, importerKey)
			Eventually(func(g Gomega) {
				var latest v1.CamundaOptimize
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(s.optimize), &latest)).To(Succeed())
				ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
				g.Expect(ready).NotTo(BeNil())
				g.Expect(ready.Reason).To(Equal(v1.ReasonInvalidReference))
				g.Expect(ready.Message).To(ContainSubstring("scaled to zero"))
			}, timeout, interval).Should(Succeed())
			expectCondition(s.optimize, v1.ConditionImporterReady, Equal(string(component.Suspended)))
		})

		It("withdraws the exporter entries on deletion and keeps the entry of the user", func() {
			s := newScenario("8.9.4")
			expectClusterEnv(s.cluster, ContainElement("CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_CLASSNAME"))

			Expect(deleteOptimize(s.optimize)).To(Succeed())

			expectClusterEnv(s.cluster, ConsistOf(userEnv.Name))
		})
	})

	Context("with more than one CamundaOptimize on one cluster", func() {
		It("lets one hold the cluster and parks the other", func() {
			s, second := newAttachedPair("8.9.4")
			expectClusterEnv(s.cluster, ContainElement("CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_CLASSNAME"))

			By("parking the second CamundaOptimize with no workloads of its own")
			expectNotReady(second, v1.ReasonClusterAlreadyAttached)
			Consistently(func() error {
				var deployment appsv1.Deployment
				return k8sClient.Get(
					ctx, client.ObjectKey{
						Namespace: s.namespace,
						Name:      components.WorkloadName(second, components.ComponentWebapp),
					}, &deployment,
				)
			}, 2*time.Second, interval).Should(WithTransform(apierrors.IsNotFound, BeTrue()))

			By("keeping the exporter entries of the holder when the parked one goes")
			Expect(deleteOptimize(second)).To(Succeed())
			Consistently(func(g Gomega) {
				var latest v1.CamundaCluster
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(s.cluster), &latest)).To(Succeed())
				g.Expect(latest.Spec.Zeebe.ExtraEnv).To(ContainElement(
					HaveField("Name", "CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_CLASSNAME"),
				))
			}, 2*time.Second, interval).Should(Succeed())
		})

		// The attachment moves when a CamundaOptimize created in the same second
		// sorts earlier by name, so a holder that already built its workloads
		// can be deposed and must drop them. Reproducing that ordering races
		// the clock, so the workloads of a deposed holder are staged here and
		// releaseWorkloads is called on them.
		It("releases the workloads that a deposed holder still owns", func() {
			ns := newNamespace()
			deposed := createDanglingOptimize("co-deposed", ns)
			other := createDanglingOptimize("co-other", ns)

			webappKey := stageWorkload(deposed, deposed, components.ComponentWebapp)
			importerKey := stageWorkload(deposed, deposed, components.ComponentImporter)
			otherKey := stageWorkload(other, other, components.ComponentWebapp)
			mirrorKey := stageMirroredSecret(deposed, components.MirrorPurposeAuthClient)

			r := &Reconciler{Client: k8sClient, APIReader: k8sClient, Scheme: k8sClient.Scheme()}
			Expect(r.releaseWorkloads(ctx, deposed)).To(Succeed())

			By("leaving no Deployment and no Service of the deposed holder")
			for _, key := range []client.ObjectKey{webappKey, importerKey} {
				var deployment appsv1.Deployment
				Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key, &deployment))).To(BeTrue(), key.Name)

				var svc corev1.Service
				Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key, &svc))).To(BeTrue(), key.Name)
			}

			By("keeping the workloads of another CamundaOptimize")
			var kept appsv1.Deployment
			Expect(k8sClient.Get(ctx, otherKey, &kept)).To(Succeed())

			By("leaving no copy of a referenced Secret behind")
			var mirrored corev1.Secret
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, mirrorKey, &mirrored))).To(BeTrue())
		})

		// The managed labels of two CamundaOptimizes on one cluster are
		// identical, so only the owner reference tells their objects apart.
		It("keeps an object at its own workload name that another owner controls", func() {
			ns := newNamespace()
			deposed := createDanglingOptimize("co-deposed", ns)
			other := createDanglingOptimize("co-other", ns)

			key := stageWorkload(deposed, other, components.ComponentWebapp)

			r := &Reconciler{Client: k8sClient, APIReader: k8sClient, Scheme: k8sClient.Scheme()}
			Expect(r.releaseWorkloads(ctx, deposed)).To(Succeed())

			var kept appsv1.Deployment
			Expect(k8sClient.Get(ctx, key, &kept)).To(Succeed())
		})

		// The attachment moves before the workloads of the previous holder go.
		// Two importers on the same indices is the state that one Optimize per
		// cluster exists to prevent, so the new holder renders nothing until
		// the importer Deployment of the old one is gone.
		It("waits for the importer of the previous holder before it renders", func() {
			ns := newNamespace()
			cluster := createCluster(ns, createBinding(ns))
			auth := createAuth(ns, true)
			previous := createDanglingOptimize("co-previous", ns)
			foreign := stageForeignImporter(previous, cluster.Name)

			optimize := createOptimize(ns, cluster, auth, "8.9.4")
			expectNotReady(optimize, v1.ReasonWaitingForHandover)

			By("rendering no importer of its own while it waits")
			var mine appsv1.Deployment
			mineKey := client.ObjectKey{
				Namespace: ns,
				Name:      components.WorkloadName(optimize, components.ComponentImporter),
			}
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, mineKey, &mine))).To(BeTrue())

			By("rendering once the importer of the previous holder goes")
			var stale appsv1.Deployment
			Expect(k8sClient.Get(ctx, foreign, &stale)).To(Succeed())
			Expect(k8sClient.Delete(ctx, &stale)).To(Succeed())

			fetchDeployment(mineKey)
		})

		It("hands the cluster to the waiting one when the holder goes", func() {
			s, second := newAttachedPair("8.9.4")
			expectNotReady(second, v1.ReasonClusterAlreadyAttached)

			Expect(deleteOptimize(s.optimize)).To(Succeed())

			fetchDeployment(client.ObjectKey{
				Namespace: s.namespace,
				Name:      components.WorkloadName(second, components.ComponentWebapp),
			})
			expectClusterEnv(s.cluster, ContainElement("CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_CLASSNAME"))
		})
	})

	// ManagementAuthConfig is cluster-scoped, so its clientSecretRef can name
	// a Secret of any namespace and needs a copy in each consuming namespace.
	// The Elasticsearch binding is namespaced, so its credentials always
	// resolve in the namespace of the Optimize instance, with no copy.
	Context("with the auth Secret in another namespace", func() {
		It("copies the auth Secret, and resolves the ES credentials directly", func() {
			source := newNamespace()
			ns := newNamespace()
			binding := createBinding(ns)
			cluster := createCluster(ns, binding)
			auth := createAuth(source, true)
			optimize := createOptimize(ns, cluster, auth, "8.9.4")

			By("applying a copy of the auth Secret alone")
			authCopy := fetchSecret(client.ObjectKey{
				Namespace: ns,
				Name:      components.MirroredSecretName(optimize, components.MirrorPurposeAuthClient),
			})
			Expect(authCopy.Data).To(HaveKeyWithValue("client-secret", []byte("s3cret")))

			By("naming the ES Secret directly, and the copy for auth")
			webapp := fetchDeployment(client.ObjectKey{
				Namespace: ns,
				Name:      components.WorkloadName(optimize, components.ComponentWebapp),
			})
			sources := map[string]string{}
			for _, env := range webapp.Spec.Template.Spec.Containers[0].Env {
				if env.ValueFrom != nil && env.ValueFrom.SecretKeyRef != nil {
					sources[env.Name] = env.ValueFrom.SecretKeyRef.Name
				}
			}
			Expect(sources).To(HaveKeyWithValue(
				"CAMUNDA_OPTIMIZE_ELASTICSEARCH_SECURITY_PASSWORD",
				binding.Spec.Elasticsearch.CredentialsSecretRef.Name,
			))
			Expect(sources).To(HaveKeyWithValue("CAMUNDA_OPTIMIZE_IDENTITY_CLIENTSECRET", authCopy.Name))

			By("taking part in Ready while a copy exists")
			expectCondition(optimize, v1.ConditionMirroredSecretsReady, Equal("Healthy"))

			By("naming the ES Secret directly on the exporter patch too")
			expectClusterEnv(
				cluster,
				ContainElement("CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_ARGS_AUTHENTICATION_PASSWORD"),
			)
			var latest v1.CamundaCluster
			Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			for _, env := range latest.Spec.Zeebe.ExtraEnv {
				if env.Name == "CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_ARGS_AUTHENTICATION_PASSWORD" {
					Expect(env.ValueFrom.SecretKeyRef.Name).To(Equal(
						binding.Spec.Elasticsearch.CredentialsSecretRef.Name,
					))
				}
			}
		})

		It("rolls the pods when the data behind a reference changes", func() {
			s := newScenario("8.9.4")
			webappKey := client.ObjectKey{
				Namespace: s.namespace,
				Name:      components.WorkloadName(s.optimize, components.ComponentWebapp),
			}
			before := fetchDeployment(webappKey).Spec.Template.Annotations[components.ConfigHashAnnotation]
			Expect(before).NotTo(BeEmpty())
			expectStableRender(webappKey)

			Eventually(func(g Gomega) {
				var secret corev1.Secret
				g.Expect(k8sClient.Get(
					ctx, client.ObjectKey{
						Namespace: s.namespace,
						Name:      s.binding.Spec.Elasticsearch.CredentialsSecretRef.Name,
					}, &secret,
				)).To(Succeed())
				secret.StringData = map[string]string{"username": "camunda", "password": "rotated"}
				g.Expect(k8sClient.Update(ctx, &secret)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				var deployment appsv1.Deployment
				g.Expect(k8sClient.Get(ctx, webappKey, &deployment)).To(Succeed())
				g.Expect(deployment.Spec.Template.Annotations[components.ConfigHashAnnotation]).NotTo(Equal(before))
			}, timeout, interval).Should(Succeed())
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

			expectNotReady(optimize, v1.ReasonInvalidReference)
		})

		It("reports StorageTypeMismatch for a cluster on relational storage", func() {
			ns := newNamespace()
			cluster := createCluster(ns, createRDBMSBinding(ns))
			optimize := createOptimize(ns, cluster, createAuth(ns, true), "8.9.4")

			expectNotReady(optimize, v1.ReasonStorageTypeMismatch)
		})

		It("reports VersionMismatch when the minors differ", func() {
			s := newScenario("8.8.1")

			expectNotReady(s.optimize, v1.ReasonVersionMismatch)
		})

		// The version floor is a rule that admission cannot enforce, because a
		// preset can supply the version, so the API server accepts a cluster
		// below it and the cluster controller refuses to reconcile it. Optimize
		// must not attach to such a cluster, and the matching minor of the two
		// is what lets it through without this check.
		It("reports InvalidReference when the cluster is below the version floor", func() {
			ns := newNamespace()
			cluster := createCluster(ns, createBinding(ns))
			Expect(retry.RetryOnConflict(retry.DefaultRetry, func() error {
				var latest v1.CamundaCluster
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest); err != nil {
					return err
				}
				latest.Spec.Version = "8.8.9"
				return k8sClient.Update(ctx, &latest)
			})).To(Succeed())
			optimize := createOptimize(ns, cluster, createAuth(ns, true), "8.8.9")

			expectNotReady(optimize, v1.ReasonInvalidReference)
		})

		It("reports MissingSecret when the client Secret is absent", func() {
			ns := newNamespace()
			cluster := createCluster(ns, createBinding(ns))
			optimize := createOptimize(ns, cluster, createAuth(ns, false), "8.9.4")

			expectNotReady(optimize, v1.ReasonMissingSecret)
		})

		It("reports ExporterConflict when a user entry supplies the other kind of value", func() {
			ns := newNamespace()
			cluster := createCluster(ns, createBinding(ns))

			// The operator sets this one from a Secret, so a literal on the
			// same name would merge into an entry with both.
			Expect(applyZeebeEnv(cluster, "e2e-user", corev1.EnvVar{
				Name:  "CAMUNDA_DATA_EXPORTERS_ELASTICSEARCH_ARGS_AUTHENTICATION_PASSWORD",
				Value: "plain",
			})).To(Succeed())

			optimize := createOptimize(ns, cluster, createAuth(ns, true), "8.9.4")

			expectNotReady(optimize, v1.ReasonExporterConflict)
		})
	})

	Context("on update", func() {
		It("rejects a clusterRef that points at another cluster", func() {
			s := newScenario("8.9.4")

			err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
				var latest v1.CamundaOptimize
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(s.optimize), &latest); err != nil {
					return err
				}
				latest.Spec.ClusterRef.Name = "another-cluster"
				return k8sClient.Update(ctx, &latest)
			})
			Expect(err).To(MatchError(ContainSubstring("clusterRef is immutable")))
		})
	})

	// Server-side apply creates the object it does not find. The withdrawal
	// reads the cluster first, but the cluster can go between that read and
	// the apply, and a CamundaCluster that holds nothing but exporter entries
	// would then take its place. Deleting a namespace removes both resources
	// at once, so the race is ordinary rather than rare.
	Context("with a cluster that goes while the exporter patch is in flight", func() {
		It("does not put the cluster back", func() {
			ns := newNamespace()
			cluster := createCluster(ns, createBinding(ns))
			key := client.ObjectKeyFromObject(cluster)
			staleUID := cluster.UID

			Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
			Eventually(func() bool {
				var gone v1.CamundaCluster
				return apierrors.IsNotFound(k8sClient.Get(ctx, key, &gone))
			}, timeout, interval).Should(BeTrue())

			r := &Reconciler{Client: k8sClient, APIReader: k8sClient, Scheme: k8sClient.Scheme()}
			Expect(r.applyExporterPatch(ctx, key, staleUID, nil)).To(
				WithTransform(apierrors.IsConflict, BeTrue()),
			)

			var after v1.CamundaCluster
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key, &after))).To(BeTrue())
		})

		It("treats the lost cluster as nothing left to withdraw", func() {
			ns := newNamespace()
			cluster := createCluster(ns, createBinding(ns))
			key := client.ObjectKeyFromObject(cluster)

			Expect(k8sClient.Delete(ctx, cluster)).To(Succeed())
			Eventually(func() bool {
				var gone v1.CamundaCluster
				return apierrors.IsNotFound(k8sClient.Get(ctx, key, &gone))
			}, timeout, interval).Should(BeTrue())

			r := &Reconciler{Client: k8sClient, APIReader: k8sClient, Scheme: k8sClient.Scheme()}
			Expect(r.withdrawExporter(ctx, key)).To(Succeed())

			var after v1.CamundaCluster
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key, &after))).To(BeTrue())
		})
	})
})
