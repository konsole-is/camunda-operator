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
	"net/url"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	// ccOIDCNamespace hosts the OIDC flow: its own Elasticsearch, its own
	// Keycloak, and the cluster under test.
	ccOIDCNamespace = "camunda-oidc-e2e"
	// ccOIDCPlatform is the cluster-scoped platform config of the flow, and
	// ccOIDCStorageConfig the SecondaryStorageConfig its Elasticsearch
	// publishes.
	ccOIDCPlatform      = "camunda-oidc-e2e"
	ccOIDCStorageConfig = "camunda-oidc-storage"

	// The realm of testdata/keycloak.yaml: one confidential client with
	// service accounts, and the static secret the flow authenticates with.
	ccOIDCRealm        = "camunda"
	ccOIDCClientID     = "camunda"
	ccOIDCClientSecret = "camunda-e2e-secret"
	// ccOIDCSecretName is the Secret that holds the client secret. It lives
	// in the manager namespace, so the flow also proves the cross-namespace
	// mirror.
	ccOIDCSecretName = "camunda-oidc-client"
	ccOIDCSecretKey  = "client-secret"
	// ccOIDCUsernameClaim and ccOIDCClientIDClaim are the claims the realm
	// puts in an access token. The client id claim comes from a hardcoded
	// claim mapper, because Keycloak names the client in "azp", which the
	// tokens of persons carry as well.
	ccOIDCUsernameClaim = "preferred_username"
	ccOIDCClientIDClaim = "client_id"

	// ccOIDCReadyTimeout adds the Keycloak startup to the Camunda one.
	ccOIDCReadyTimeout = 15 * time.Minute
)

// keycloakBaseURL is the issuer host of the flow, reachable from the pods of
// the namespace. Nothing resolves it from outside, because the flow never
// completes a browser login.
func keycloakBaseURL() string {
	return fmt.Sprintf("http://keycloak.%s.svc:8080", ccOIDCNamespace)
}

func keycloakIssuerURL() string {
	return keycloakBaseURL() + "/realms/" + ccOIDCRealm
}

func keycloakTokenURL() string {
	return keycloakIssuerURL() + "/protocol/openid-connect/token"
}

// oidcClientSecret returns the Secret that holds the client secret of the
// realm. It lives in the manager namespace, which outlives the namespace of
// the flow, so the suite applies it rather than creating it: a run that ends
// before its cleanup must not stop the next one.
func oidcClientSecret() *corev1.Secret {
	return &corev1.Secret{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{Name: ccOIDCSecretName, Namespace: namespace},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{ccOIDCSecretKey: ccOIDCClientSecret},
	}
}

// oidcPlatformConfig returns the platform config of the flow: the realm of
// the in-cluster Keycloak, the client of that realm, and the two claim names
// that turn a token into a caller.
func oidcPlatformConfig() *v1.CamundaPlatformConfig {
	return &v1.CamundaPlatformConfig{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaPlatformConfig"},
		ObjectMeta: metav1.ObjectMeta{Name: ccOIDCPlatform},
		Spec: v1.CamundaPlatformConfigSpec{
			Auth: &v1.PlatformAuthSpec{
				Method: v1.AuthenticationMethodOIDC,
				OIDC: &v1.OIDCSpec{
					IssuerURL:     keycloakIssuerURL(),
					ClientID:      ccOIDCClientID,
					UsernameClaim: ccOIDCUsernameClaim,
					ClientIDClaim: ccOIDCClientIDClaim,
					ClientSecretRef: v1.SecretKeyRef{
						Name:      ccOIDCSecretName,
						Namespace: namespace,
						Key:       ccOIDCSecretKey,
					},
				},
			},
		},
	}
}

var _ = Describe("CamundaCluster with OIDC", Ordered, Label(utils.LabelCamundaClusterOIDC), func() {
	var (
		elasticsearch = &v1.ElasticsearchCluster{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "ElasticsearchCluster"},
			ObjectMeta: metav1.ObjectMeta{Name: esName, Namespace: ccOIDCNamespace},
			Spec: v1.ElasticsearchClusterSpec{
				Version:                os.Getenv(envElasticsearchVersion),
				Replicas:               new(int32(1)),
				StorageSize:            new(resource.MustParse(esStorageSize)),
				Resources:              requests("500m", "1Gi"),
				SecondaryStorageConfig: ccOIDCStorageConfig,
			},
		}
		cluster *v1.CamundaCluster
	)

	BeforeAll(func() {
		By("creating the test namespace")
		_, err := utils.Kubectl("create", "ns", ccOIDCNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("deploying Keycloak with the e2e realm")
		_, err = utils.Kubectl("apply", "-n", ccOIDCNamespace, "-f", "test/e2e/testdata/keycloak.yaml")
		Expect(err).NotTo(HaveOccurred())

		By("creating the client secret in the manager namespace")
		Expect(apply(oidcClientSecret())).To(Succeed())

		By("creating the ElasticsearchCluster and waiting for Ready Healthy")
		Expect(apply(elasticsearch)).To(Succeed())
		Eventually(func(g Gomega) {
			expectReady(g, esResource, esName, ccOIDCNamespace, v1.ReasonHealthy)
			expectReady(g, sscResource, ccOIDCStorageConfig, ccOIDCNamespace, v1.ReasonHealthy)
		}, esReadyTimeout, 5*time.Second).Should(Succeed())

		By("waiting for the Keycloak realm to be served")
		_, err = utils.Kubectl(
			"rollout", "status", "deployment/keycloak", "-n", ccOIDCNamespace, "--timeout", "6m",
		)
		Expect(err).NotTo(HaveOccurred())

		By("creating the platform config")
		Expect(apply(oidcPlatformConfig())).To(Succeed())

		// The OIDC flow takes no backups: the runner is short of CPU, and the
		// backup path is proven on the basic-auth flows.
		cluster = newCluster(ccOIDCNamespace, ccOIDCPlatform, ccOIDCStorageConfig, "", true)
		// The redirect URI is derived from externalUrl, and the flow asserts
		// it, so it points at the gateway Service of this cluster.
		cluster.Spec.ExternalURL = fmt.Sprintf(
			"http://%s.%s.svc:%d",
			components.WorkloadName(cluster, components.ComponentGateway), ccOIDCNamespace, components.PortHTTP,
		)
		// Camunda seeds no administrator under OIDC. The client of the realm
		// is the caller of every spec below, so it is the administrator.
		cluster.Spec.Auth = &v1.ClusterAuthSpec{
			Admin: &v1.ClusterAdminSpec{Clients: []string{ccOIDCClientID}},
		}
	})

	AfterAll(func() {
		By("removing the cluster, the platform config, the client secret, and the test namespace")
		_, _ = utils.Kubectl(
			"delete", ccResource, ccName, "-n", ccOIDCNamespace, "--ignore-not-found", "--wait=false",
		)
		_, _ = utils.Kubectl("delete", ccPlatformResource, ccOIDCPlatform, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", "secret", ccOIDCSecretName, "-n", namespace, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", "ns", ccOIDCNamespace, "--wait=false")
	})

	AfterEach(func() {
		dumpDiagnostics(ccOIDCNamespace)
	})

	It("reaches Ready Healthy with the client secret mirrored from the manager namespace", func() {
		By("creating the CamundaCluster")
		Expect(apply(cluster)).To(Succeed())

		By("waiting for Ready Healthy")
		Eventually(func(g Gomega) {
			expectReady(g, ccResource, ccName, ccOIDCNamespace, v1.ReasonHealthy)
			expectCondition(
				g, ccResource, ccName, ccOIDCNamespace, v1.ConditionMirroredSecretsReady, v1.ReasonHealthy,
			)
		}, ccOIDCReadyTimeout, 5*time.Second).Should(Succeed())

		By("checking that the mirrored Secret exists next to the workloads")
		exists, err := utils.Exists(
			"secret", components.MirroredSecretName(cluster, components.MirrorPurposeOIDCClient), ccOIDCNamespace,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeTrue())
	})

	It("accepts a client-credentials token on the topology endpoint", func() {
		var topology struct {
			Brokers []struct {
				Partitions []struct {
					Health string `json:"health"`
				} `json:"partitions"`
			} `json:"brokers"`
		}

		Eventually(func(g Gomega) {
			resp, err := oidcREST(cluster, "topology", http.MethodGet, pathTopology, nil)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resp.Status).To(Equal(http.StatusOK), resp.Body)
			g.Expect(json.Unmarshal([]byte(resp.Body), &topology)).To(Succeed(), resp.Body)
			g.Expect(topology.Brokers).To(HaveLen(1), resp.Body)
			g.Expect(topology.Brokers[0].Partitions).To(HaveEach(HaveField("Health", "healthy")), resp.Body)
		}, ccAPITimeout).Should(Succeed())
	})

	It("refuses the topology endpoint without credentials", func() {
		resp, err := anonymousREST(cluster, "anonymous", http.MethodGet, pathTopology)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Status).To(Equal(http.StatusUnauthorized), resp.Body)
	})

	// The gateway serves the web applications and the API on one filter
	// chain, and it answers an unauthenticated request in two ways. A caller
	// that asks for HTML gets the login redirect; every other caller gets the
	// 401 of the spec above. The Accept header is therefore what makes this a
	// browser login rather than an API call.
	It("redirects a browser login to the identity provider", func() {
		resp, err := anonymousREST(
			cluster, "login", http.MethodGet, "/operate/", "-H", "Accept: text/html",
		)
		Expect(err).NotTo(HaveOccurred())

		By("landing on the authorization endpoint of the realm")
		Expect(resp.URL).To(
			HavePrefix(keycloakIssuerURL()+"/protocol/openid-connect/auth"),
			"status %d, body %s", resp.Status, resp.Body,
		)

		By("carrying the redirect URI that the operator derived from externalUrl")
		final, err := url.Parse(resp.URL)
		Expect(err).NotTo(HaveOccurred())
		Expect(final.Query().Get("redirect_uri")).To(Equal(cluster.Spec.ExternalURL + "/sso-callback"))
		Expect(final.Query().Get("client_id")).To(Equal(ccOIDCClientID))
	})

	It("runs the connectors runtime under OIDC", func() {
		Eventually(func(g Gomega) {
			expectCondition(g, ccResource, ccName, ccOIDCNamespace, v1.ConditionConnectorsReady, v1.ReasonHealthy)
		}, ccOIDCReadyTimeout, 5*time.Second).Should(Succeed())
	})

	// This spec is what proves the admin bootstrap: a deployment needs the
	// admin role, which only spec.auth.admin grants under OIDC.
	It("deploys a process, starts an instance, and finds it in secondary storage", func() {
		By("deploying the process")
		dir, err := utils.GetProjectDir()
		Expect(err).NotTo(HaveOccurred())
		bpmn, err := os.ReadFile(filepath.Join(dir, "test", "e2e", "testdata", processFile))
		Expect(err).NotTo(HaveOccurred())

		resp, err := oidcREST(
			cluster, "deploy", http.MethodPost, pathDeployments,
			map[string]string{processFile: string(bpmn)},
			"-F", "resources=@/tmp/"+processFile,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Status).To(Equal(http.StatusOK), resp.Body)
		Expect(resp.Body).To(ContainSubstring(`"processDefinitionId":"` + processID + `"`))

		By("starting an instance")
		resp, err = oidcREST(
			cluster, "start", http.MethodPost, pathProcessInstances, nil,
			"-H", "Content-Type: application/json", "-d", `{"processDefinitionId":"`+processID+`"}`,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(resp.Status).To(Equal(http.StatusOK), resp.Body)

		By("searching the instance in secondary storage")
		var result struct {
			Items []struct {
				ProcessDefinitionID string `json:"processDefinitionId"`
			} `json:"items"`
		}
		Eventually(func(g Gomega) {
			resp, err := oidcREST(
				cluster, "search", http.MethodPost, pathInstanceSearch, nil,
				"-H", "Content-Type: application/json",
				"-d", `{"filter":{"processDefinitionId":"`+processID+`"}}`,
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resp.Status).To(Equal(http.StatusOK), resp.Body)
			g.Expect(json.Unmarshal([]byte(resp.Body), &result)).To(Succeed(), resp.Body)
			g.Expect(result.Items).NotTo(BeEmpty(), resp.Body)
			g.Expect(result.Items).To(HaveEach(HaveField("ProcessDefinitionID", processID)), resp.Body)
		}, ccAPITimeout).Should(Succeed())
	})
})

// gatewayURL is the URL of path on the HTTP port of the gateway Service of
// cluster.
func gatewayURL(cluster *v1.CamundaCluster, path string) string {
	return fmt.Sprintf(
		"http://%s.%s.svc:%d%s",
		components.WorkloadName(cluster, components.ComponentGateway), cluster.Namespace,
		components.PortHTTP, path,
	)
}

// oidcREST calls path on the gateway of cluster with a client-credentials
// token of the e2e realm. The helper pod reads the token itself, so it never
// reaches the output of the suite.
func oidcREST(
	cluster *v1.CamundaCluster,
	name, method, path string,
	files map[string]string,
	args ...string,
) (utils.CamundaResponse, error) {
	return utils.CamundaREST(utils.CamundaRequest{
		Namespace: cluster.Namespace,
		Name:      name,
		Method:    method,
		URL:       gatewayURL(cluster, path),
		Auth: utils.ClientCredentials{
			TokenURL:  keycloakTokenURL(),
			ClientID:  ccOIDCClientID,
			Audience:  ccOIDCClientID,
			Secret:    components.MirroredSecretName(cluster, components.MirrorPurposeOIDCClient),
			SecretKey: ccOIDCSecretKey,
		},
		Files:   files,
		Args:    args,
		Timeout: podTimeout,
	})
}

// anonymousREST calls path on the gateway of cluster with no credentials.
// args are extra curl arguments.
func anonymousREST(
	cluster *v1.CamundaCluster,
	name, method, path string,
	args ...string,
) (utils.CamundaResponse, error) {
	return utils.CamundaREST(utils.CamundaRequest{
		Namespace: cluster.Namespace,
		Name:      name,
		Method:    method,
		URL:       gatewayURL(cluster, path),
		Args:      args,
		Timeout:   podTimeout,
	})
}
