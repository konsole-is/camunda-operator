//go:build e2e
// +build e2e

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

package e2e

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	// optimizeNamespace hosts the Optimize flow: its own Elasticsearch, its
	// own Keycloak, the cluster under test, and the Optimize instance.
	optimizeNamespace = "camunda-optimize-e2e"
	// optimizeName is the CamundaOptimize under test, optimizePlatform the
	// cluster-scoped platform config of its cluster, and optimizeAuthConfig
	// the cluster-scoped Management Identity contract it authenticates
	// through.
	optimizeName       = "camunda-optimize"
	optimizePlatform   = "camunda-optimize-e2e"
	optimizeAuthConfig = "camunda-optimize-e2e-auth"
	// optimizeVersion is the Optimize release under test. Optimize has its own
	// patch line, so it is not ccVersion. Only the minor has to match, and
	// this is the newest 8.9 tag of camunda/optimize.
	optimizeVersion = "8.9.16"
	// optimizeStorage is the SecondaryStorageConfig of the flow.
	optimizeStorage = "camunda-optimize-storage"
	// optimizeSecretName is the Secret of the namespace that holds the client
	// secret of the realm, and optimizeClientKey the key inside it.
	optimizeSecretName = "camunda-optimize-client"

	// userEnvName and userEnvValue are the entry that the flow puts on
	// spec.zeebe.extraEnv of the cluster before Optimize attaches. It proves
	// the co-ownership of that list: the exporter settings arrive next to it,
	// and it outlives their withdrawal.
	userEnvName  = "E2E_USER_MARKER"
	userEnvValue = "keep-me"

	optimizeResource   = "camundaoptimizes.core.camunda.io"
	authConfigResource = "managementauthconfigs.core.camunda.io"

	// optimizeReadyTimeout covers the pull of the Optimize image and the
	// schema that a first start writes into Elasticsearch.
	optimizeReadyTimeout = 15 * time.Minute
	// optimizeImportTimeout bounds the wait for records to travel: the broker
	// exports them, and the importer reads them into the Optimize indices.
	optimizeImportTimeout = 10 * time.Minute
)

// The Elasticsearch of this flow. It is a plain single-node Deployment that
// serves HTTP, not an ElasticsearchCluster, and the flow writes the storage
// contract of it by hand. The legacy Zeebe Elasticsearch exporter carries no
// TLS settings and trusts what the JVM trusts, so it cannot reach an
// ElasticsearchCluster: that one serves HTTPS with the private CA of ECK, and
// nothing puts that CA in the trust store of the broker. An Elasticsearch on
// HTTP is the shape the contract documents for a cluster this operator does
// not manage.
const (
	esFixtureName        = "elasticsearch"
	esFixtureSecret      = "elasticsearch-user"
	esFixtureUsername    = "elastic"
	esFixturePassword    = "camunda-optimize-e2e"
	esFixtureUsernameKey = "username"
	esFixturePasswordKey = "password"
	esFixturePort        = 9200
)

// esFixtureLabels select the pods of the Elasticsearch Deployment.
var esFixtureLabels = map[string]string{"e2e": esFixtureName}

// esFixtureEndpoint is the address of the Elasticsearch of this flow.
func esFixtureEndpoint() string {
	return fmt.Sprintf("http://%s.%s.svc:%d", esFixtureName, optimizeNamespace, esFixturePort)
}

// optimizeIssuerURL is the issuer of the realm that testdata/keycloak.yaml
// serves, reachable from the pods of this namespace.
func optimizeIssuerURL() string {
	return fmt.Sprintf("http://keycloak.%s.svc:8080/realms/%s", optimizeNamespace, ccOIDCRealm)
}

// esFixtureCredentials returns the Secret that the storage contract names. It
// holds the superuser of the Elasticsearch Deployment, which starts with the
// password of ELASTIC_PASSWORD.
func esFixtureCredentials() *corev1.Secret {
	return &corev1.Secret{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{Name: esFixtureSecret, Namespace: optimizeNamespace},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{
			esFixtureUsernameKey: esFixtureUsername,
			esFixturePasswordKey: esFixturePassword,
		},
	}
}

// esFixtureDeployment returns the single-node Elasticsearch of the flow. A
// single-node discovery skips the bootstrap checks, so the node starts on a
// kind node without a sysctl change, and the data lives in the container: the
// flow indexes what it needs from scratch.
func esFixtureDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
		ObjectMeta: metav1.ObjectMeta{Name: esFixtureName, Namespace: optimizeNamespace},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{MatchLabels: esFixtureLabels},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: esFixtureLabels},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  esFixtureName,
						Image: "docker.elastic.co/elasticsearch/elasticsearch:" + esVersion,
						Env: []corev1.EnvVar{
							{Name: "discovery.type", Value: "single-node"},
							{Name: "xpack.security.enabled", Value: "true"},
							{Name: "xpack.security.http.ssl.enabled", Value: "false"},
							{Name: "xpack.security.transport.ssl.enabled", Value: "false"},
							{Name: "ELASTIC_PASSWORD", Value: esFixturePassword},
							{Name: "ES_JAVA_OPTS", Value: "-Xms512m -Xmx512m"},
							// The runner shares one disk with every image that
							// the suite pulls. Above the flood-stage watermark
							// Elasticsearch turns its indices read-only, and
							// the exporter of the broker then fails for a
							// reason outside this flow.
							{Name: "cluster.routing.allocation.disk.threshold_enabled", Value: "false"},
						},
						Ports:     []corev1.ContainerPort{{Name: "http", ContainerPort: esFixturePort}},
						Resources: *requests("250m", "1Gi"),
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								TCPSocket: &corev1.TCPSocketAction{Port: intstr.FromInt32(esFixturePort)},
							},
							InitialDelaySeconds: 10,
							PeriodSeconds:       5,
						},
					}},
				},
			},
		},
	}
}

// esFixtureService returns the Service that the storage contract points at.
func esFixtureService() *corev1.Service {
	return &corev1.Service{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
		ObjectMeta: metav1.ObjectMeta{Name: esFixtureName, Namespace: optimizeNamespace},
		Spec: corev1.ServiceSpec{
			Selector: esFixtureLabels,
			Ports: []corev1.ServicePort{{
				Name:       "http",
				Port:       esFixturePort,
				TargetPort: intstr.FromInt32(esFixturePort),
			}},
		},
	}
}

// optimizeStorageContract returns the secondary storage of the flow: the
// Elasticsearch Deployment, its superuser, and no CA, because the endpoint is
// HTTP.
func optimizeStorageContract() *v1.SecondaryStorageConfig {
	return &v1.SecondaryStorageConfig{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "SecondaryStorageConfig"},
		ObjectMeta: metav1.ObjectMeta{Name: optimizeStorage, Namespace: optimizeNamespace},
		Spec: v1.SecondaryStorageConfigSpec{
			Type: v1.SecondaryStorageTypeElasticsearch,
			Elasticsearch: &v1.ElasticsearchStorage{
				Endpoint: esFixtureEndpoint(),
				CredentialsSecretRef: v1.CredentialsSecretRef{
					Name:        esFixtureSecret,
					Namespace:   optimizeNamespace,
					UsernameKey: esFixtureUsernameKey,
					PasswordKey: esFixturePasswordKey,
				},
			},
		},
	}
}

// optimizeClientSecret returns the Secret that holds the client secret of the
// realm, in the namespace of the flow.
func optimizeClientSecret() *corev1.Secret {
	return &corev1.Secret{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{Name: optimizeSecretName, Namespace: optimizeNamespace},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{ccOIDCSecretKey: ccOIDCClientSecret},
	}
}

// optimizeManagementAuth returns the Management Identity contract of the flow.
// Optimize authenticates against Management Identity, so the realm of
// testdata/keycloak.yaml stands in for it: the endpoints are those of that
// realm, and the client is the confidential client it defines.
func optimizeManagementAuth() *v1.ManagementAuthConfig {
	issuer := optimizeIssuerURL()

	return &v1.ManagementAuthConfig{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "ManagementAuthConfig"},
		ObjectMeta: metav1.ObjectMeta{Name: optimizeAuthConfig},
		Spec: v1.ManagementAuthConfigSpec{
			BaseURL:   issuer,
			IssuerURL: issuer,
			AuthURL:   issuer + "/protocol/openid-connect/auth",
			TokenURL:  issuer + "/protocol/openid-connect/token",
			JwksURL:   issuer + "/protocol/openid-connect/certs",
			ClientID:  ccOIDCClientID,
			Audience:  ccOIDCClientID,
			ClientSecretRef: v1.SecretKeyRef{
				Name:      optimizeSecretName,
				Namespace: optimizeNamespace,
				Key:       ccOIDCSecretKey,
			},
		},
	}
}

// newOptimize returns the CamundaOptimize of the flow, sized for a kind node.
func newOptimize() *v1.CamundaOptimize {
	return &v1.CamundaOptimize{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaOptimize"},
		ObjectMeta: metav1.ObjectMeta{Name: optimizeName, Namespace: optimizeNamespace},
		Spec: v1.CamundaOptimizeSpec{
			Version:           optimizeVersion,
			ManagementAuthRef: optimizeAuthConfig,
			ClusterRef:        v1.ClusterRef{Name: ccName},
			Webapp:            &v1.WorkloadSpec{Resources: requests("250m", "1Gi")},
			Importer:          &v1.WorkloadSpec{Resources: requests("250m", "1Gi")},
		},
	}
}

var _ = Describe("CamundaOptimize", Ordered, func() {
	var (
		cluster  = newCluster(optimizeNamespace, optimizePlatform, optimizeStorage, "", false)
		optimize = newOptimize()
		// exporterEnv are the entries that the operator applies to
		// spec.zeebe.extraEnv of the cluster. The credentials of the contract
		// live in the namespace of the cluster, so the broker reads them
		// directly and the applied entries are these.
		exporterEnv = components.ExporterEnv(*optimizeStorageContract().Spec.Elasticsearch)
	)

	BeforeAll(func() {
		By("creating the test namespace")
		_, err := utils.Kubectl("create", "ns", optimizeNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("deploying Keycloak with the e2e realm")
		_, err = utils.Kubectl("apply", "-n", optimizeNamespace, "-f", "test/e2e/testdata/keycloak.yaml")
		Expect(err).NotTo(HaveOccurred())

		By("deploying Elasticsearch")
		Expect(apply(esFixtureCredentials())).To(Succeed())
		Expect(apply(esFixtureService())).To(Succeed())
		Expect(apply(esFixtureDeployment())).To(Succeed())
		_, err = utils.Kubectl(
			"rollout", "status", "deployment/"+esFixtureName, "-n", optimizeNamespace, "--timeout", "6m",
		)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for Elasticsearch to answer with the credentials of the contract")
		Eventually(func(g Gomega) {
			out, err := curlOptimizeElasticsearch("health", "/_cluster/health")
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(out).To(ContainSubstring(`"status":"green"`))
		}, 5*time.Minute).Should(Succeed())

		By("creating the storage contract and waiting for Ready Healthy")
		Expect(apply(optimizeStorageContract())).To(Succeed())
		Eventually(func(g Gomega) {
			expectReady(g, sscResource, optimizeStorage, optimizeNamespace, v1.ReasonHealthy)
		}, 3*time.Minute).Should(Succeed())

		By("creating the platform config")
		Expect(apply(basicPlatform(optimizePlatform))).To(Succeed())

		By("creating the CamundaCluster with an extraEnv entry of its own")
		cluster.Spec.Zeebe.ExtraEnv = []corev1.EnvVar{{Name: userEnvName, Value: userEnvValue}}
		Expect(apply(cluster)).To(Succeed())
		Eventually(func(g Gomega) {
			expectReady(g, ccResource, ccName, optimizeNamespace, v1.ReasonHealthy)
		}, ccReadyTimeout, 5*time.Second).Should(Succeed())

		By("waiting for the Keycloak realm to be served")
		_, err = utils.Kubectl(
			"rollout", "status", "deployment/keycloak", "-n", optimizeNamespace, "--timeout", "6m",
		)
		Expect(err).NotTo(HaveOccurred())

		By("creating the client secret and the Management Identity contract")
		Expect(apply(optimizeClientSecret())).To(Succeed())
		Expect(apply(optimizeManagementAuth())).To(Succeed())
	})

	AfterAll(func() {
		By("removing the Optimize instance, the cluster, the cluster-scoped contracts, and the namespace")
		_, _ = utils.Kubectl(
			"delete", optimizeResource, optimizeName, "-n", optimizeNamespace, "--ignore-not-found", "--wait=false",
		)
		_, _ = utils.Kubectl(
			"delete", ccResource, ccName, "-n", optimizeNamespace, "--ignore-not-found", "--wait=false",
		)
		_, _ = utils.Kubectl("delete", authConfigResource, optimizeAuthConfig, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", ccPlatformResource, optimizePlatform, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", "ns", optimizeNamespace, "--wait=false")
	})

	AfterEach(func() {
		dumpDiagnostics(optimizeNamespace)
	})

	It("reaches Ready Healthy", func() {
		By("creating the CamundaOptimize")
		Expect(apply(optimize)).To(Succeed())

		By("waiting for Ready Healthy")
		Eventually(func(g Gomega) {
			expectReady(g, optimizeResource, optimizeName, optimizeNamespace, v1.ReasonHealthy)
		}, optimizeReadyTimeout, 5*time.Second).Should(Succeed())
	})

	It("runs a webapp and an importer Deployment that carry the labels of the cluster", func() {
		for component, name := range map[string]string{
			components.ComponentWebapp:   components.WorkloadName(optimize, components.ComponentWebapp),
			components.ComponentImporter: components.WorkloadName(optimize, components.ComponentImporter),
		} {
			var deployment appsv1.Deployment
			Expect(utils.Get("deployment", name, optimizeNamespace, &deployment)).To(Succeed())
			Expect(deployment.Labels).To(HaveKeyWithValue(labels.ClusterKey, ccName))
			Expect(deployment.Labels).To(HaveKeyWithValue(labels.ComponentKey, component))
			Expect(deployment.Status.AvailableReplicas).To(
				BeNumerically(">=", 1), "Deployment %q runs no ready pod", name,
			)
		}
	})

	// This is the co-ownership of spec.zeebe.extraEnv: the operator owns the
	// exporter entries under its own field manager, and the entry of the user
	// stands untouched next to them.
	It("adds the exporter settings to the cluster and keeps the entry of the user", func() {
		var got v1.CamundaCluster
		Expect(utils.Get(ccResource, ccName, optimizeNamespace, &got)).To(Succeed())
		Expect(got.Spec.Zeebe).NotTo(BeNil())

		for _, entry := range exporterEnv {
			Expect(got.Spec.Zeebe.ExtraEnv).To(ContainElement(entry), "exporter entry %q", entry.Name)
		}
		Expect(got.Spec.Zeebe.ExtraEnv).To(ContainElement(corev1.EnvVar{Name: userEnvName, Value: userEnvValue}))
	})

	It("deploys a process, starts an instance, and exports the records to Elasticsearch", func() {
		By("deploying the process")
		dir, err := utils.GetProjectDir()
		Expect(err).NotTo(HaveOccurred())
		bpmn, err := os.ReadFile(filepath.Join(dir, "test", "e2e", "testdata", processFile))
		Expect(err).NotTo(HaveOccurred())

		resp, err := camundaREST(
			cluster, "deploy", http.MethodPost, pathDeployments,
			map[string]string{processFile: string(bpmn)},
			"-F", "resources=@/tmp/"+processFile,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Status).To(Equal(http.StatusOK), resp.Body)
		Expect(resp.Body).To(ContainSubstring(`"processDefinitionId":"` + processID + `"`))

		By("starting an instance")
		resp, err = camundaREST(
			cluster, "start", http.MethodPost, pathProcessInstances, nil,
			"-H", "Content-Type: application/json", "-d", `{"processDefinitionId":"`+processID+`"}`,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Status).To(Equal(http.StatusOK), resp.Body)

		By("waiting for the exporter to write the record indices")
		Eventually(func(g Gomega) {
			g.Expect(elasticsearchIndices(g, "records", components.ZeebeRecordPrefix+"*")).NotTo(BeEmpty())
		}, optimizeImportTimeout, 10*time.Second).Should(Succeed())
	})

	It("imports the exported records into the Optimize indices", func() {
		By("waiting for the Optimize indices")
		Eventually(func(g Gomega) {
			g.Expect(elasticsearchIndices(g, "optimize", "*optimize*")).NotTo(BeEmpty())
		}, optimizeImportTimeout, 10*time.Second).Should(Succeed())

		By("waiting for the deployed process definition to arrive in them")
		Eventually(func(g Gomega) {
			out, err := curlOptimizeElasticsearch("definitions", "/*optimize*process-definition*/_count")
			g.Expect(err).NotTo(HaveOccurred())

			var result struct {
				Count int `json:"count"`
			}
			g.Expect(json.Unmarshal([]byte(out), &result)).To(Succeed(), out)
			g.Expect(result.Count).To(BeNumerically(">=", 1), out)
		}, optimizeImportTimeout, 10*time.Second).Should(Succeed())
	})

	It("withdraws the exporter settings on deletion and keeps the entry of the user", func() {
		_, err := utils.Kubectl("delete", optimizeResource, optimizeName, "-n", optimizeNamespace, "--wait=false")
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the CamundaOptimize and its workloads to be gone")
		Eventually(func(g Gomega) {
			expectGone(g, optimizeResource, optimizeName, optimizeNamespace)
			expectGone(
				g, "deployment", components.WorkloadName(optimize, components.ComponentWebapp), optimizeNamespace,
			)
			expectGone(
				g, "deployment", components.WorkloadName(optimize, components.ComponentImporter), optimizeNamespace,
			)
		}, 5*time.Minute).Should(Succeed())

		By("reading spec.zeebe.extraEnv of the cluster")
		Eventually(func(g Gomega) {
			var got v1.CamundaCluster
			g.Expect(utils.Get(ccResource, ccName, optimizeNamespace, &got)).To(Succeed())
			g.Expect(got.Spec.Zeebe).NotTo(BeNil())

			for _, entry := range exporterEnv {
				g.Expect(envNames(got.Spec.Zeebe.ExtraEnv)).NotTo(
					ContainElement(entry.Name), "exporter entry %q is still there", entry.Name,
				)
			}
			g.Expect(got.Spec.Zeebe.ExtraEnv).To(ContainElement(corev1.EnvVar{Name: userEnvName, Value: userEnvValue}))
		}, 3*time.Minute).Should(Succeed())
	})
})

// envNames returns the names of env, in order.
func envNames(env []corev1.EnvVar) []string {
	names := make([]string, 0, len(env))
	for _, entry := range env {
		names = append(names, entry.Name)
	}

	return names
}

// elasticsearchIndices returns the indices of the Elasticsearch of this flow
// that match pattern. It is written for Eventually.
func elasticsearchIndices(g Gomega, name, pattern string) []string {
	out, err := curlOptimizeElasticsearch(name, "/_cat/indices/"+pattern+"?format=json")
	g.Expect(err).NotTo(HaveOccurred())

	var indices []struct {
		Index string `json:"index"`
	}
	g.Expect(json.Unmarshal([]byte(out), &indices)).To(Succeed(), out)

	names := make([]string, 0, len(indices))
	for _, index := range indices {
		names = append(names, index.Index)
	}

	return names
}

// curlOptimizeElasticsearch runs curl against path on the Elasticsearch of
// this flow, with the credentials that its storage contract names. extra are
// further curl arguments. It returns the response body.
//
// The flow has its own helper because curlElasticsearch always mounts the CA
// of the contract, and the contract of an HTTP endpoint names none.
//
// The pod name carries a random suffix, for the reason that
// curlElasticsearch documents: a left-behind pod of a failed cleanup must not
// fail the next call with AlreadyExists.
func curlOptimizeElasticsearch(name, path string, extra ...string) (string, error) {
	env := []corev1.EnvVar{
		utils.SecretEnv("ES_USERNAME", esFixtureSecret, esFixtureUsernameKey),
		utils.SecretEnv("ES_PASSWORD", esFixtureSecret, esFixturePasswordKey),
	}

	// $0 is "curl", the remaining arguments are the curl arguments.
	script := `exec curl -fsS -u "$ES_USERNAME:$ES_PASSWORD" "$@"`
	args := append([]string{"-ec", script, "curl", esFixtureEndpoint() + path}, extra...)

	return utils.RunPod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "curl-" + name + "-" + utilrand.String(5),
			Namespace: optimizeNamespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "curl",
				Image:   utils.CurlImage,
				Command: []string{"sh"},
				Args:    args,
				Env:     env,
			}},
		},
	}, podTimeout)
}
