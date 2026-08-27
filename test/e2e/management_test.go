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
	"fmt"
	"net/http"
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	optimizecomponents "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	// mcKeycloakNamespace hosts the keycloak flow: the Keycloak Operator, the
	// PostgreSQL server of the three management databases, the SMTP sink, the
	// management plane, the orchestration cluster it serves, and the Optimize
	// that reads its contract. The suite creates it, because the Keycloak
	// Operator watches its own namespace and has to run before the manager
	// starts.
	mcKeycloakNamespace = "camunda-management-e2e"
	// mcOIDCNamespace hosts the oidc flow: the Keycloak of
	// testdata/keycloak.yaml, its own PostgreSQL, the management plane, and a
	// basic-auth orchestration cluster.
	mcOIDCNamespace = "camunda-management-oidc-e2e"

	// The CamundaManagementCluster of each flow. The two names differ,
	// because each writes a cluster-scoped ManagementAuthConfig that takes
	// the name of its management cluster.
	mcKeycloakName = "camunda-management"
	mcOIDCName     = "camunda-management-oidc"

	// mcFlowLabel is the label that a management plane selects its
	// orchestration clusters by. Each flow uses a value of its own, so
	// neither management plane selects the cluster of the other.
	mcFlowLabel         = "e2e-flow"
	mcKeycloakFlowValue = "management-keycloak"
	mcOIDCFlowValue     = "management-oidc"

	// mcRealm is the realm that Management Identity creates and administers
	// in the two Keycloak modes. It is the documented default of Management
	// Identity, and the operator sets no other.
	mcRealm = "camunda-platform"

	// The first administrator of the management plane in the keycloak flow.
	// Management Identity creates the Keycloak user on its first start.
	mcAdminUsername = "admin"
	mcAdminEmail    = "admin@example.com"

	// mcMailFromAddress is the sender of the Web Modeler notifications.
	// RFC 2606 reserves the domain for documentation, so the flows claim no
	// address that somebody owns.
	mcMailFromAddress = "web-modeler@example.com"
	// smtpPort is the port of testdata/smtp.yaml.
	smtpPort int32 = 1025

	mcResource = "camundamanagementclusters.core.camunda.io"

	// mcReadyTimeout covers the pulls of the management plane images, the
	// bootstrap of Keycloak on an empty database, and the realm that
	// Management Identity creates on its first start.
	mcReadyTimeout = 20 * time.Minute
)

// The cluster-scoped resources of the keycloak flow: the platform config of
// the management plane, the platform config of the orchestration cluster on
// the realm, the PostgreSQL server, and the three databases. Management
// Identity, Keycloak, and Web Modeler each own every table of their database,
// so none of them can share one.
const (
	mcKeycloakPlatform        = "camunda-management-e2e"
	mcKeycloakClusterPlatform = "camunda-management-cluster-e2e"
	mcKeycloakServer          = "camunda-management-postgres"
	mcKeycloakDatabase        = "management-keycloak-db"
	mcKeycloakIdentityDB      = "management-identity-db"
	mcKeycloakWebModelerDB    = "management-web-modeler-db"
	// mcKeycloakStorage is the SecondaryStorageConfig that the
	// ElasticsearchCluster of the flow publishes. Optimize reads the
	// exported records from it.
	mcKeycloakStorage = "camunda-management-storage"
	// mcOptimizeName is the CamundaOptimize that attaches through the
	// contract.
	mcOptimizeName = "camunda-management-optimize"

	// The client that the orchestration cluster of the keycloak flow
	// authenticates with. Management Identity creates a client for every
	// component of the management plane and none for an orchestration
	// cluster, so the flow registers this one in the realm itself.
	mcClusterClientID     = "camunda-core"
	mcClusterClientSecret = "camunda-core-e2e-secret"
	mcClusterSecretName   = "camunda-core-client"
)

// The cluster-scoped resources of the oidc flow. The orchestration cluster
// runs on a logical database of the same PostgreSQL server, so the flow needs
// no Elasticsearch.
const (
	mcOIDCPlatform        = "camunda-management-oidc-e2e"
	mcOIDCClusterPlatform = "camunda-management-oidc-cluster-e2e"
	mcOIDCServer          = "camunda-management-oidc-postgres"
	mcOIDCIdentityDB      = "management-oidc-identity-db"
	mcOIDCWebModelerDB    = "management-oidc-web-modeler-db"
	mcOIDCClusterDB       = "management-oidc-cluster-db"
	mcOIDCStorage         = "camunda-management-oidc-storage"

	// The clients of testdata/keycloak.yaml that a management plane in the
	// oidc mode names. Every confidential one takes the client secret of the
	// realm, so one Secret of the namespace serves all of them.
	mcOIDCIdentityClient      = "management-identity"
	mcOIDCOptimizeClient      = "management-optimize"
	mcOIDCWebModelerClient    = "management-web-modeler"
	mcOIDCWebModelerAPIClient = "management-web-modeler-api"
	// mcOIDCClientSecretName is the Secret that holds that client secret. It
	// lives in the manager namespace, so the flow also proves the
	// cross-namespace mirror of the management plane.
	mcOIDCClientSecretName = "camunda-management-clients"
	// The administrator of the management plane in the oidc mode: the claim
	// of the user that testdata/keycloak.yaml defines.
	mcOIDCAdminClaimName  = ccOIDCUsernameClaim
	mcOIDCAdminClaimValue = "ada@example.com"
)

// managementKeycloakNamespaceObject returns the namespace of the keycloak
// flow. The suite applies it before the manager is deployed, so that the
// Keycloak Operator has a namespace to run in.
func managementKeycloakNamespaceObject() *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
		ObjectMeta: metav1.ObjectMeta{Name: mcKeycloakNamespace},
	}
}

var _ = Describe("CamundaManagementCluster with Keycloak", Ordered, Label(utils.LabelManagementKeycloak), func() {
	var (
		mc            = keycloakManagementCluster()
		elasticsearch = &v1.ElasticsearchCluster{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "ElasticsearchCluster"},
			ObjectMeta: metav1.ObjectMeta{Name: esName, Namespace: mcKeycloakNamespace},
			Spec: v1.ElasticsearchClusterSpec{
				Version:     os.Getenv(envElasticsearchVersion),
				Replicas:    new(int32(1)),
				StorageSize: new(resource.MustParse(esStorageSize)),
				Resources:   capped("200m", "1Gi", "1536Mi"),
				// ES_JAVA_OPTS fixes the heap, and the limit above bounds
				// what Elasticsearch adds around it. With neither,
				// Elasticsearch gives the heap half of the memory it sees,
				// and it sees the whole node: that heap was the largest
				// single claim on the runner. The flow exports the records
				// of one process instance, so 512 MB is enough.
				ExtraEnv:               []corev1.EnvVar{{Name: "ES_JAVA_OPTS", Value: "-Xms512m -Xmx512m"}},
				SecondaryStorageConfig: mcKeycloakStorage,
			},
		}
		cluster = newCluster(mcKeycloakNamespace, mcKeycloakClusterPlatform, mcKeycloakStorage, "", false)
		// installedKeycloakOperator records that this flow installed the
		// Keycloak Operator, and with it that the namespace is its own to
		// remove. KEYCLOAK_OPERATOR_INSTALL_SKIP=true names a cluster that
		// already serves one in this namespace.
		installedKeycloakOperator = false
	)

	// The flows run one at a time on one node, so what a flow reserves has to
	// fit beside the system pods, the manager, and the operators on a
	// four-vCPU runner. A CPU request is scheduling room, and it is also the
	// share a container gets when every process asks for the CPU at once.
	// The kind control plane keeps a larger share when the flow asks for
	// less. The Keycloak Operator adds 300m of its own.
	cluster.Spec.Zeebe.Resources = capped("400m", "1Gi", "2Gi")
	cluster.Spec.Gateway.Resources = capped("150m", "512Mi", "1280Mi")

	BeforeAll(func() {
		By("creating the namespace of the flow")
		Expect(apply(managementKeycloakNamespaceObject())).To(Succeed())

		if os.Getenv("KEYCLOAK_OPERATOR_INSTALL_SKIP") != "true" &&
			!utils.IsKeycloakOperatorInstalled(mcKeycloakNamespace) {
			version, err := utils.KeycloakOperatorVersion()
			Expect(err).NotTo(HaveOccurred())

			// Recorded before the install, so that a Deployment the apply
			// created and the rollout never finished is removed all the same.
			installedKeycloakOperator = true
			By(fmt.Sprintf("installing the Keycloak Operator %s", version))
			Expect(utils.InstallKeycloakOperator(mcKeycloakNamespace)).To(Succeed())
		}

		By("deploying PostgreSQL and the SMTP sink")
		deployBackingServices(mcKeycloakNamespace)

		By("describing the PostgreSQL server and creating the three management databases")
		Expect(apply(databaseServer(mcKeycloakServer, mcKeycloakNamespace))).To(Succeed())
		for _, database := range []*v1.Database{
			managementDatabase(mcKeycloakDatabase, mcKeycloakServer, mcKeycloakNamespace, "keycloak", ""),
			managementDatabase(mcKeycloakIdentityDB, mcKeycloakServer, mcKeycloakNamespace, "identity", ""),
			managementDatabase(mcKeycloakWebModelerDB, mcKeycloakServer, mcKeycloakNamespace, "web_modeler", ""),
		} {
			Expect(apply(database)).To(Succeed())
		}
		Eventually(func(g Gomega) {
			for _, name := range []string{mcKeycloakDatabase, mcKeycloakIdentityDB, mcKeycloakWebModelerDB} {
				expectReady(g, dbResource, name, mcKeycloakNamespace, v1.ReasonHealthy)
				expectReady(g, dbConfigResource, name, mcKeycloakNamespace, v1.ReasonHealthy)
			}
		}, 5*time.Minute, 5*time.Second).Should(Succeed())

		By("creating the ElasticsearchCluster and waiting for Ready Healthy")
		Expect(apply(elasticsearch)).To(Succeed())
		Eventually(func(g Gomega) {
			expectReady(g, esResource, esName, mcKeycloakNamespace, v1.ReasonHealthy)
			expectReady(g, sscResource, mcKeycloakStorage, mcKeycloakNamespace, v1.ReasonHealthy)
		}, esReadyTimeout, 5*time.Second).Should(Succeed())

		By("creating the platform config of the management plane")
		Expect(apply(basicPlatform(mcKeycloakPlatform))).To(Succeed())

		// The management plane runs the identity provider of the
		// orchestration cluster, so the cluster carries the selector label,
		// the realm as its issuer, and the client that the first spec
		// registers there.
		cluster.Labels = map[string]string{mcFlowLabel: mcKeycloakFlowValue}
		cluster.Spec.ExternalURL = gatewayURL(cluster, "")
		cluster.Spec.Auth = &v1.ClusterAuthSpec{
			Admin: &v1.ClusterAdminSpec{Clients: []string{mcClusterClientID}},
		}
	})

	AfterAll(func() {
		By("removing the Optimize instance, the cluster, the management plane, and the cluster-scoped resources")
		_, _ = utils.Kubectl(
			"delete", optimizeResource, mcOptimizeName, "-n", mcKeycloakNamespace,
			"--ignore-not-found", "--wait=false",
		)
		_, _ = utils.Kubectl(
			"delete", ccResource, ccName, "-n", mcKeycloakNamespace, "--ignore-not-found", "--wait=false",
		)
		_, _ = utils.Kubectl(
			"delete", mcResource, mcKeycloakName, "-n", mcKeycloakNamespace, "--ignore-not-found",
		)
		_, _ = utils.Kubectl(
			"delete", ccPlatformResource, mcKeycloakPlatform, mcKeycloakClusterPlatform, "--ignore-not-found",
		)
		_, _ = utils.Kubectl(
			"delete", dbResource, mcKeycloakDatabase, mcKeycloakIdentityDB, mcKeycloakWebModelerDB,
			"-n", mcKeycloakNamespace, "--ignore-not-found", "--wait=false",
		)
		_, _ = utils.Kubectl(
			"delete", dbServerResource, mcKeycloakServer, "-n", mcKeycloakNamespace, "--ignore-not-found",
		)

		// A Keycloak Operator that this flow installed goes with the
		// namespace. One that was there before the flow keeps its namespace,
		// and the flow removes its own fixtures instead.
		if installedKeycloakOperator {
			By("uninstalling the Keycloak Operator with the namespace of the flow")
			utils.UninstallKeycloakOperator(mcKeycloakNamespace)
			_, _ = utils.Kubectl("delete", "ns", mcKeycloakNamespace, "--wait=false")
			return
		}
		_, _ = utils.Kubectl(
			"delete", esResource, esName, "-n", mcKeycloakNamespace, "--ignore-not-found", "--wait=false",
		)
		_, _ = utils.Kubectl(
			"delete", "secret", mcClusterSecretName, "-n", mcKeycloakNamespace, "--ignore-not-found",
		)
		for _, manifest := range []string{"postgres.yaml", "smtp.yaml"} {
			_, _ = utils.Kubectl(
				"delete", "-n", mcKeycloakNamespace, "-f", "test/e2e/testdata/"+manifest,
				"--ignore-not-found", "--wait=false",
			)
		}
	})

	AfterEach(func() {
		dumpDiagnostics(mcKeycloakNamespace)
	})

	It("runs Keycloak, Management Identity, Console, and Web Modeler and publishes the contract", func() {
		By("creating the CamundaManagementCluster")
		Expect(apply(mc)).To(Succeed())

		By("waiting for every component to report Healthy")
		Eventually(func(g Gomega) {
			for _, condition := range []string{
				v1.ConditionSecretsReady,
				v1.ConditionKeycloakReady,
				v1.ConditionIdentityReady,
				v1.ConditionConsoleReady,
				v1.ConditionWebModelerReady,
				v1.ConditionManagementAuthReady,
			} {
				expectCondition(g, mcResource, mcKeycloakName, mcKeycloakNamespace, condition, v1.ReasonHealthy)
			}
			// No CamundaOptimize exists yet, so the realm holds no login
			// callback of this operator and none is missing.
			expectCondition(
				g, mcResource, mcKeycloakName, mcKeycloakNamespace,
				v1.ConditionOptimizeCallbacksReady, v1.ReasonNoCallbacks,
			)
			expectReady(g, mcResource, mcKeycloakName, mcKeycloakNamespace, v1.ReasonHealthy)
		}, mcReadyTimeout, 10*time.Second).Should(Succeed())

		By("publishing the generated credentials")
		for _, name := range []string{
			components.OptimizeClientSecretName(mc),
			components.IdentityAdminSecretName(mc),
			components.PusherSecretName(mc),
		} {
			exists, err := utils.Exists("secret", name, mcKeycloakNamespace)
			Expect(err).NotTo(HaveOccurred())
			Expect(exists).To(BeTrue(), "Secret %q does not exist", name)
		}

		By("publishing a Ready ManagementAuthConfig with the Optimize client of the realm")
		var status v1.CamundaManagementCluster
		Expect(utils.Get(mcResource, mcKeycloakName, mcKeycloakNamespace, &status)).To(Succeed())
		Expect(status.Status.ManagementAuthConfig).To(Equal(mcKeycloakName))

		var contract v1.ManagementAuthConfig
		Expect(utils.Get(authConfigResource, mcKeycloakName, "", &contract)).To(Succeed())
		Expect(contract.Spec.IssuerURL).To(Equal(keycloakRealmURL(mc)))
		Expect(contract.Spec.ClientSecretRef.Name).To(Equal(components.OptimizeClientSecretName(mc)))
		Eventually(func(g Gomega) {
			expectReady(g, authConfigResource, mcKeycloakName, "", v1.ReasonHealthy)
		}, 2*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("attaches an orchestration cluster on the realm and points it at Console", func() {
		By("registering the client of the orchestration cluster in the realm")
		Expect(registerRealmClient(mc, clusterClientRepresentation())).To(Succeed())

		By("creating the client secret and the platform config of the orchestration cluster")
		Expect(apply(clusterClientSecret())).To(Succeed())
		Expect(apply(clusterPlatformConfig(mc))).To(Succeed())

		By("creating the CamundaCluster and waiting for Ready Healthy")
		Expect(apply(cluster)).To(Succeed())
		Eventually(func(g Gomega) {
			expectReady(g, ccResource, ccName, mcKeycloakNamespace, v1.ReasonHealthy)
		}, ccReadyTimeout, 10*time.Second).Should(Succeed())

		By("claiming the cluster and reporting it attached")
		Eventually(func(g Gomega) {
			var attached v1.CamundaCluster
			g.Expect(utils.Get(ccResource, ccName, mcKeycloakNamespace, &attached)).To(Succeed())
			g.Expect(attached.Annotations).To(HaveKeyWithValue(
				components.ClaimAnnotation, mcKeycloakNamespace+"/"+mcKeycloakName,
			))

			var owner v1.CamundaManagementCluster
			g.Expect(utils.Get(mcResource, mcKeycloakName, mcKeycloakNamespace, &owner)).To(Succeed())
			g.Expect(owner.Status.Clusters).To(ContainElement(SatisfyAll(
				HaveField("Name", ccName),
				HaveField("Namespace", mcKeycloakNamespace),
				HaveField("Attached", true),
			)))
		}, 3*time.Minute, 5*time.Second).Should(Succeed())

		By("writing the Console ping into spec.extraEnv of the cluster")
		Eventually(func(g Gomega) {
			var attached v1.CamundaCluster
			g.Expect(utils.Get(ccResource, ccName, mcKeycloakNamespace, &attached)).To(Succeed())

			env := envByName(attached.Spec.ExtraEnv)
			g.Expect(env).To(HaveKeyWithValue("CAMUNDA_CONSOLE_PING_ENABLED", "true"))
			g.Expect(env).To(HaveKeyWithValue("CAMUNDA_CONSOLE_PING_ENDPOINT", components.ConsoleServiceURL(mc)))
			g.Expect(env).To(HaveKeyWithValue("CAMUNDA_CONSOLE_PING_CLUSTERNAME", ccName))
			g.Expect(env).To(HaveKey("CAMUNDA_CONSOLE_PING_PINGPERIOD"))
		}, 3*time.Minute, 5*time.Second).Should(Succeed())

		By("listing the cluster in the Web Modeler restapi environment")
		Eventually(func(g Gomega) {
			var attached v1.CamundaCluster
			g.Expect(utils.Get(ccResource, ccName, mcKeycloakNamespace, &attached)).To(Succeed())

			env, err := deploymentEnv(components.WebModelerRestapiName(mc), mcKeycloakNamespace)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(env).To(HaveKeyWithValue("CAMUNDA_MODELER_CLUSTERS_0_ID", string(attached.UID)))
			g.Expect(env).To(HaveKeyWithValue(
				"CAMUNDA_MODELER_CLUSTERS_0_NAME", mcKeycloakNamespace+"/"+ccName,
			))
			g.Expect(env).To(HaveKeyWithValue("CAMUNDA_MODELER_CLUSTERS_0_AUTHENTICATION", "BEARER_TOKEN"))
			g.Expect(attached.Status.Gateway).NotTo(BeNil(), "the cluster publishes no gateway endpoints")
			g.Expect(env).To(HaveKeyWithValue(
				"CAMUNDA_MODELER_CLUSTERS_0_URL_REST", attached.Status.Gateway.RESTEndpoint,
			))
		}, 5*time.Minute, 5*time.Second).Should(Succeed())
	})

	It("answers the cluster API with a token of the realm", func() {
		Eventually(func(g Gomega) {
			resp, err := utils.CamundaREST(utils.CamundaRequest{
				Namespace: mcKeycloakNamespace,
				Name:      "topology",
				Method:    http.MethodGet,
				URL:       gatewayURL(cluster, pathTopology),
				Auth: utils.ClientCredentials{
					TokenURL:  keycloakRealmURL(mc) + "/protocol/openid-connect/token",
					ClientID:  mcClusterClientID,
					Audience:  mcClusterClientID,
					Secret:    mcClusterSecretName,
					SecretKey: ccOIDCSecretKey,
				},
				Timeout: podTimeout,
			})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resp.Status).To(Equal(http.StatusOK), resp.Body)
		}, ccAPITimeout).Should(Succeed())
	})

	It("runs Optimize against the contract of the management plane", func() {
		By("creating the CamundaOptimize")
		Expect(apply(managementOptimize())).To(Succeed())

		By("registering its login callback on the Optimize client of the realm")
		Eventually(func(g Gomega) {
			var status v1.CamundaManagementCluster
			g.Expect(utils.Get(mcResource, mcKeycloakName, mcKeycloakNamespace, &status)).To(Succeed())
			g.Expect(status.Status.Optimize).To(ContainElement(v1.AttachedOptimizeStatus{
				Namespace:   mcKeycloakNamespace,
				Name:        mcOptimizeName,
				ExternalURL: optimizeWebappURL(),
			}))

			expectCondition(
				g, mcResource, mcKeycloakName, mcKeycloakNamespace,
				v1.ConditionOptimizeCallbacksReady, v1.ReasonHealthy,
			)

			// The realm is the proof: Management Identity created the client
			// from the URL the management plane discovered, and the client
			// carries the login callback under it.
			client, err := realmOptimizeClient(mc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(client).To(ContainSubstring(
				optimizeWebappURL() + components.OptimizeCallbackPath,
			))

			// The preset switched on with the first Optimize, so the realm
			// carries the Optimize role too. Management Identity assigns it to
			// the first administrator on its very first start only, so an
			// administrator from before this point grants it to themselves.
			roles, err := realmRoles(mc)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(roles).To(ContainSubstring(`"name":"Optimize"`))
		}, mcReadyTimeout, 10*time.Second).Should(Succeed())

		By("waiting for Ready Healthy")
		Eventually(func(g Gomega) {
			expectReady(g, optimizeResource, mcOptimizeName, mcKeycloakNamespace, v1.ReasonHealthy)
		}, optimizeReadyTimeout, 10*time.Second).Should(Succeed())
	})
})

var _ = Describe("CamundaManagementCluster with OIDC", Ordered, Label(utils.LabelManagementOIDC), func() {
	var (
		mc      = oidcManagementCluster()
		cluster = newCluster(mcOIDCNamespace, mcOIDCClusterPlatform, mcOIDCStorage, "", false)
	)

	// The same four-vCPU room as the Keycloak flow.
	cluster.Spec.Zeebe.Resources = capped("400m", "1Gi", "2Gi")
	cluster.Spec.Gateway.Resources = capped("150m", "512Mi", "1280Mi")

	BeforeAll(func() {
		By("creating the test namespace")
		_, err := utils.Kubectl("create", "ns", mcOIDCNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("deploying Keycloak with the e2e realm")
		_, err = utils.Kubectl("apply", "-n", mcOIDCNamespace, "-f", "test/e2e/testdata/keycloak.yaml")
		Expect(err).NotTo(HaveOccurred())

		By("deploying PostgreSQL and the SMTP sink")
		deployBackingServices(mcOIDCNamespace)

		By("describing the PostgreSQL server and creating the three databases")
		Expect(apply(databaseServer(mcOIDCServer, mcOIDCNamespace))).To(Succeed())
		for _, database := range []*v1.Database{
			managementDatabase(mcOIDCIdentityDB, mcOIDCServer, mcOIDCNamespace, "identity", ""),
			managementDatabase(mcOIDCWebModelerDB, mcOIDCServer, mcOIDCNamespace, "web_modeler", ""),
			managementDatabase(mcOIDCClusterDB, mcOIDCServer, mcOIDCNamespace, "camunda", mcOIDCStorage),
		} {
			Expect(apply(database)).To(Succeed())
		}
		Eventually(func(g Gomega) {
			for _, name := range []string{mcOIDCIdentityDB, mcOIDCWebModelerDB, mcOIDCClusterDB} {
				expectReady(g, dbResource, name, mcOIDCNamespace, v1.ReasonHealthy)
				expectReady(g, dbConfigResource, name, mcOIDCNamespace, v1.ReasonHealthy)
			}
			expectReady(g, sscResource, mcOIDCStorage, mcOIDCNamespace, v1.ReasonHealthy)
		}, 5*time.Minute, 5*time.Second).Should(Succeed())

		By("waiting for the Keycloak realm to be served")
		_, err = utils.Kubectl(
			"rollout", "status", "deployment/keycloak", "-n", mcOIDCNamespace, "--timeout", "6m",
		)
		Expect(err).NotTo(HaveOccurred())

		By("creating the client secret and the two platform configs")
		Expect(apply(oidcManagementClientSecret())).To(Succeed())
		Expect(apply(oidcManagementPlatformConfig())).To(Succeed())
		Expect(apply(basicPlatform(mcOIDCClusterPlatform))).To(Succeed())

		cluster.Labels = map[string]string{mcFlowLabel: mcOIDCFlowValue}
		cluster.Spec.ExternalURL = gatewayURL(cluster, "")
	})

	AfterAll(func() {
		By("removing the cluster, the management plane, and the cluster-scoped resources")
		_, _ = utils.Kubectl(
			"delete", ccResource, ccName, "-n", mcOIDCNamespace, "--ignore-not-found", "--wait=false",
		)
		_, _ = utils.Kubectl("delete", mcResource, mcOIDCName, "-n", mcOIDCNamespace, "--ignore-not-found")
		_, _ = utils.Kubectl(
			"delete", ccPlatformResource, mcOIDCPlatform, mcOIDCClusterPlatform, "--ignore-not-found",
		)
		_, _ = utils.Kubectl(
			"delete", dbResource, mcOIDCIdentityDB, mcOIDCWebModelerDB, mcOIDCClusterDB,
			"-n", mcOIDCNamespace, "--ignore-not-found", "--wait=false",
		)
		_, _ = utils.Kubectl(
			"delete", dbServerResource, mcOIDCServer, "-n", mcOIDCNamespace, "--ignore-not-found",
		)
		_, _ = utils.Kubectl(
			"delete", "secret", mcOIDCClientSecretName, "-n", namespace, "--ignore-not-found",
		)
		_, _ = utils.Kubectl("delete", "ns", mcOIDCNamespace, "--wait=false")
	})

	AfterEach(func() {
		dumpDiagnostics(mcOIDCNamespace)
	})

	It("runs Management Identity and Web Modeler on the identity provider of the platform config", func() {
		By("creating the CamundaManagementCluster")
		Expect(apply(mc)).To(Succeed())

		By("waiting for every component to report Healthy")
		Eventually(func(g Gomega) {
			// SecretsReady is absent from this list on purpose: the oidc
			// mode generates no credential of its own, so its component is
			// gated off and reports Disabled.
			for _, condition := range []string{
				v1.ConditionMirroredSecretsReady,
				v1.ConditionIdentityReady,
				v1.ConditionWebModelerReady,
				v1.ConditionManagementAuthReady,
			} {
				expectCondition(g, mcResource, mcOIDCName, mcOIDCNamespace, condition, v1.ReasonHealthy)
			}
			expectReady(g, mcResource, mcOIDCName, mcOIDCNamespace, v1.ReasonHealthy)
		}, mcReadyTimeout, 10*time.Second).Should(Succeed())

		By("publishing the contract with the Optimize client of the platform config")
		var contract v1.ManagementAuthConfig
		Expect(utils.Get(authConfigResource, mcOIDCName, "", &contract)).To(Succeed())
		Expect(contract.Spec.ClientID).To(Equal(mcOIDCOptimizeClient))
		Expect(contract.Spec.IssuerURL).To(Equal(oidcIssuerURL()))
	})

	It("gives Web Modeler a user of its own on the basic-auth cluster", func() {
		By("creating the CamundaCluster and waiting for Ready Healthy")
		Expect(apply(cluster)).To(Succeed())
		Eventually(func(g Gomega) {
			expectReady(g, ccResource, ccName, mcOIDCNamespace, v1.ReasonHealthy)
		}, ccReadyTimeout, 10*time.Second).Should(Succeed())

		By("listing the cluster for Web Modeler with basic authentication")
		var attached v1.CamundaCluster
		Eventually(func(g Gomega) {
			g.Expect(utils.Get(ccResource, ccName, mcOIDCNamespace, &attached)).To(Succeed())
			g.Expect(attached.Annotations).To(HaveKeyWithValue(
				components.ClaimAnnotation, mcOIDCNamespace+"/"+mcOIDCName,
			))

			env, err := deploymentEnv(components.WebModelerRestapiName(mc), mcOIDCNamespace)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(env).To(HaveKeyWithValue("CAMUNDA_MODELER_CLUSTERS_0_AUTHENTICATION", "BASIC"))
			g.Expect(env).To(HaveKeyWithValue("CAMUNDA_MODELER_CLUSTERS_0_NAME", mcOIDCNamespace+"/"+ccName))
		}, 5*time.Minute, 5*time.Second).Should(Succeed())

		By("publishing the password of the Web Modeler user")
		userSecret := components.WebModelerClusterUserSecretName(mc, attached.UID)
		Eventually(func(g Gomega) {
			applied, err := utils.SecretValue(
				mcOIDCNamespace, userSecret, components.WebModelerClusterUserAppliedKey,
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(applied).To(Equal("true"))
		}, 5*time.Minute, 5*time.Second).Should(Succeed())

		By("signing in to the cluster API as that user")
		Eventually(func(g Gomega) {
			resp, err := utils.CamundaREST(utils.CamundaRequest{
				Namespace: mcOIDCNamespace,
				Name:      "web-modeler-user",
				Method:    http.MethodGet,
				URL:       gatewayURL(cluster, pathTopology),
				Auth: utils.BasicAuth{
					Username:    components.WebModelerClusterUsername,
					Secret:      userSecret,
					PasswordKey: components.WebModelerClusterUserPasswordKey,
				},
				Timeout: podTimeout,
			})
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(resp.Status).To(Equal(http.StatusOK), resp.Body)
		}, ccAPITimeout).Should(Succeed())
	})
})

// keycloakManagementCluster returns the management plane of the keycloak
// flow: Keycloak through the Keycloak Operator, Management Identity, Console,
// and Web Modeler, sized for a kind node.
//
// Every external URL is the in-cluster Service of the component it names. Only
// the Keycloak one is dialed — Management Identity reads the front-channel
// issuer of the realm from it — and the rest reach no browser in this flow.
func keycloakManagementCluster() *v1.CamundaManagementCluster {
	mc := &v1.CamundaManagementCluster{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaManagementCluster"},
		ObjectMeta: metav1.ObjectMeta{Name: mcKeycloakName, Namespace: mcKeycloakNamespace},
	}
	mc.Spec = v1.CamundaManagementClusterSpec{
		PlatformConfigRef: mcKeycloakPlatform,
		ClusterSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{mcFlowLabel: mcKeycloakFlowValue},
		},
		IdentityProvider: v1.IdentityProviderSpec{
			Keycloak: &v1.ManagedKeycloakSpec{
				Version:           os.Getenv(envKeycloakVersion),
				ExternalURL:       keycloakServiceURL(mc),
				DatabaseConfigRef: mcKeycloakDatabase,
				Replicas:          new(int32(1)),
				Resources:         capped("150m", "512Mi", "1280Mi"),
			},
		},
		Identity: v1.IdentitySpec{
			Version:           os.Getenv(envIdentityVersion),
			ExternalURL:       components.IdentityServiceURL(mc),
			DatabaseConfigRef: mcKeycloakIdentityDB,
			Admin:             v1.IdentityAdminSpec{Username: mcAdminUsername, Email: mcAdminEmail},
			WorkloadSpec:      v1.WorkloadSpec{Resources: capped("150m", "512Mi", "1280Mi")},
		},
		Console: &v1.ConsoleSpec{
			Version:      os.Getenv(envConsoleVersion),
			ExternalURL:  components.ConsoleServiceURL(mc),
			WorkloadSpec: v1.WorkloadSpec{Resources: capped("50m", "64Mi", "256Mi")},
		},
		WebModeler: webModelerSpec(mc, mcKeycloakWebModelerDB),
	}

	return mc
}

// oidcManagementCluster returns the management plane of the oidc flow:
// Management Identity and Web Modeler on the identity provider of the
// platform config. It deploys no Console, so nothing pings and the flow costs
// one workload less.
func oidcManagementCluster() *v1.CamundaManagementCluster {
	mc := &v1.CamundaManagementCluster{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaManagementCluster"},
		ObjectMeta: metav1.ObjectMeta{Name: mcOIDCName, Namespace: mcOIDCNamespace},
	}
	mc.Spec = v1.CamundaManagementClusterSpec{
		PlatformConfigRef: mcOIDCPlatform,
		ClusterSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{mcFlowLabel: mcOIDCFlowValue},
		},
		IdentityProvider: v1.IdentityProviderSpec{OIDC: &v1.ManagementOIDCSpec{}},
		Identity: v1.IdentitySpec{
			Version:           os.Getenv(envIdentityVersion),
			ExternalURL:       components.IdentityServiceURL(mc),
			DatabaseConfigRef: mcOIDCIdentityDB,
			Admin: v1.IdentityAdminSpec{
				ClaimName:  mcOIDCAdminClaimName,
				ClaimValue: mcOIDCAdminClaimValue,
			},
			WorkloadSpec: v1.WorkloadSpec{Resources: capped("150m", "512Mi", "1280Mi")},
		},
		WebModeler: webModelerSpec(mc, mcOIDCWebModelerDB),
	}

	return mc
}

// webModelerSpec returns the Web Modeler of a flow: both processes on the
// named database, sending their notifications through the SMTP sink of the
// namespace.
func webModelerSpec(mc *v1.CamundaManagementCluster, databaseRef string) *v1.WebModelerSpec {
	return &v1.WebModelerSpec{
		Version:               os.Getenv(envWebModelerVersion),
		ExternalURL:           webModelerRestapiURL(mc),
		WebsocketsExternalURL: webModelerWebsocketsURL(mc),
		DatabaseConfigRef:     databaseRef,
		Mail: v1.WebModelerMailSpec{
			SMTPHost:    "smtp." + mc.Namespace + ".svc",
			SMTPPort:    smtpPort,
			FromAddress: mcMailFromAddress,
			// The SMTP sink speaks no STARTTLS.
			TLS: new(false),
		},
		Restapi:    &v1.WorkloadSpec{Resources: capped("150m", "512Mi", "1280Mi")},
		Websockets: &v1.WorkloadSpec{Resources: capped("50m", "128Mi", "256Mi")},
	}
}

// keycloakServiceURL is the URL of the Keycloak that the operator runs, the
// /auth base path included, as every pod of the Kubernetes cluster reaches
// it. The flow completes no browser login, so the address that a browser
// would use is the in-cluster one too.
func keycloakServiceURL(mc *v1.CamundaManagementCluster) string {
	return fmt.Sprintf(
		"http://%s.%s.svc:%d/auth",
		components.KeycloakServiceName(mc), mc.Namespace, components.KeycloakServicePort,
	)
}

// keycloakRealmURL is the issuer of the realm that Management Identity
// creates.
func keycloakRealmURL(mc *v1.CamundaManagementCluster) string {
	return keycloakServiceURL(mc) + "/realms/" + mcRealm
}

// webModelerRestapiURL and webModelerWebsocketsURL are the addresses that Web
// Modeler publishes to a browser. No browser reaches this flow, and Web
// Modeler refuses to start without them.
func webModelerRestapiURL(mc *v1.CamundaManagementCluster) string {
	return fmt.Sprintf(
		"http://%s.%s.svc:%d",
		components.WebModelerRestapiName(mc), mc.Namespace, components.WebModelerRestapiServicePortHTTP,
	)
}

func webModelerWebsocketsURL(mc *v1.CamundaManagementCluster) string {
	return fmt.Sprintf(
		"http://%s.%s.svc:%d",
		components.WebModelerWebsocketsName(mc), mc.Namespace, components.WebModelerWebsocketsServicePortHTTP,
	)
}

// optimizeWebappURL is the address that a browser reaches the Optimize of
// this flow at, and the one that spec.externalUrl of its CamundaOptimize
// carries. The management plane finds it there and registers the login
// callback under it.
func optimizeWebappURL() string {
	optimize := &v1.CamundaOptimize{ObjectMeta: metav1.ObjectMeta{Name: mcOptimizeName}}

	return fmt.Sprintf(
		"http://%s.%s.svc:%d",
		optimizecomponents.WorkloadName(optimize, optimizecomponents.ComponentWebapp),
		mcKeycloakNamespace, optimizecomponents.PortHTTP,
	)
}

// managementOptimize returns the CamundaOptimize that authenticates through
// the contract of the management plane, sized for a kind node.
func managementOptimize() *v1.CamundaOptimize {
	// Optimize starts last, when the node already carries Keycloak,
	// Management Identity, Console, both Web Modeler processes, the
	// orchestration cluster, and an Elasticsearch. optimize-startup.sh of
	// camunda/camunda 8.9.9 starts each process with -Xms1024m -Xmx1024m
	// -XX:MetaspaceSize=256m -XX:MaxMetaspaceSize=256m when
	// OPTIMIZE_JAVA_OPTS is unset, and it reads no container limit, so this
	// variable is the only way to make the heap smaller.
	javaOpts := []corev1.EnvVar{
		{Name: "OPTIMIZE_JAVA_OPTS", Value: "-Xms256m -Xmx512m -XX:MaxMetaspaceSize=256m"},
	}

	return &v1.CamundaOptimize{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaOptimize"},
		ObjectMeta: metav1.ObjectMeta{Name: mcOptimizeName, Namespace: mcKeycloakNamespace},
		Spec: v1.CamundaOptimizeSpec{
			Version:           os.Getenv(envOptimizeVersion),
			ManagementAuthRef: mcKeycloakName,
			ExternalURL:       optimizeWebappURL(),
			ClusterRef:        v1.ClusterRef{Name: ccName},
			Webapp: &v1.WorkloadSpec{
				Resources: capped("150m", "768Mi", "1280Mi"),
				ExtraEnv:  javaOpts,
			},
			Importer: &v1.WorkloadSpec{
				Resources: capped("150m", "768Mi", "1280Mi"),
				ExtraEnv:  javaOpts,
			},
		},
	}
}

// databaseServer returns the DatabaseServerConfig of the PostgreSQL that
// testdata/postgres.yaml runs in namespace.
func databaseServer(name, namespace string) *v1.DatabaseServerConfig {
	return &v1.DatabaseServerConfig{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "DatabaseServerConfig"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1.DatabaseServerConfigSpec{
			Engine: v1.DatabaseEnginePostgres,
			Host:   postgresService + "." + namespace + ".svc",
			Port:   5432,
			AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
				Name:        postgresAdminSecret,
				UsernameKey: "username",
				PasswordKey: "password",
			},
		},
	}
}

// managementDatabase returns one logical database of a flow. storageRef names
// the SecondaryStorageConfig it publishes, and is empty for a database that
// no orchestration cluster stores its records in.
func managementDatabase(name, serverRef, namespace, databaseName, storageRef string) *v1.Database {
	return &v1.Database{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "Database"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1.DatabaseSpec{
			ServerRef:              serverRef,
			DatabaseName:           databaseName,
			SecondaryStorageConfig: storageRef,
		},
	}
}

// deployBackingServices runs the PostgreSQL server and the SMTP sink of a
// management flow in namespace and waits for both.
func deployBackingServices(namespace string) {
	for _, manifest := range []string{"postgres.yaml", "smtp.yaml"} {
		_, err := utils.Kubectl("apply", "-n", namespace, "-f", "test/e2e/testdata/"+manifest)
		Expect(err).NotTo(HaveOccurred())
	}

	for _, deployment := range []string{"postgres", "smtp"} {
		_, err := utils.Kubectl(
			"rollout", "status", "deployment/"+deployment, "-n", namespace, "--timeout", "5m",
		)
		Expect(err).NotTo(HaveOccurred())
	}
}

// clusterClientSecret returns the Secret that holds the client secret of the
// orchestration cluster of the keycloak flow.
func clusterClientSecret() *corev1.Secret {
	return &corev1.Secret{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{Name: mcClusterSecretName, Namespace: mcKeycloakNamespace},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{ccOIDCSecretKey: mcClusterClientSecret},
	}
}

// clusterPlatformConfig returns the platform config of the orchestration
// cluster of the keycloak flow: the realm that Management Identity created,
// and the client that the flow registered there.
func clusterPlatformConfig(mc *v1.CamundaManagementCluster) *v1.CamundaPlatformConfig {
	return &v1.CamundaPlatformConfig{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaPlatformConfig"},
		ObjectMeta: metav1.ObjectMeta{Name: mcKeycloakClusterPlatform},
		Spec: v1.CamundaPlatformConfigSpec{
			Auth: &v1.PlatformAuthSpec{
				Method: v1.AuthenticationMethodOIDC,
				OIDC: &v1.OIDCSpec{
					IssuerURL:     keycloakRealmURL(mc),
					ClientID:      mcClusterClientID,
					UsernameClaim: ccOIDCUsernameClaim,
					ClientIDClaim: ccOIDCClientIDClaim,
					ClientSecretRef: v1.SecretKeyRef{
						Name:      mcClusterSecretName,
						Namespace: mcKeycloakNamespace,
						Key:       ccOIDCSecretKey,
					},
				},
			},
		},
	}
}

// clusterClientRepresentation is the Keycloak client of the orchestration
// cluster of the keycloak flow, in the shape the admin API takes.
//
// The two protocol mappers are the ones that testdata/keycloak.yaml bakes into
// its own realm, and the flow does not work without either. The audience
// mapper puts the client id in "aud", because Keycloak otherwise issues
// "aud": "account" and the Camunda audiences check refuses the token. The
// hardcoded-claim mapper puts "client_id" in the access token, because a
// Keycloak token names its client in "azp", which the tokens of persons carry
// as well.
func clusterClientRepresentation() string {
	return `{
  "clientId": "` + mcClusterClientID + `",
  "enabled": true,
  "protocol": "openid-connect",
  "publicClient": false,
  "clientAuthenticatorType": "client-secret",
  "secret": "` + mcClusterClientSecret + `",
  "standardFlowEnabled": true,
  "serviceAccountsEnabled": true,
  "directAccessGrantsEnabled": true,
  "redirectUris": ["*"],
  "webOrigins": ["*"],
  "protocolMappers": [
    {
      "name": "` + mcClusterClientID + `-audience",
      "protocol": "openid-connect",
      "protocolMapper": "oidc-audience-mapper",
      "config": {
        "included.client.audience": "` + mcClusterClientID + `",
        "access.token.claim": "true",
        "id.token.claim": "false"
      }
    },
    {
      "name": "` + mcClusterClientID + `-client-id",
      "protocol": "openid-connect",
      "protocolMapper": "oidc-hardcoded-claim-mapper",
      "config": {
        "claim.name": "` + ccOIDCClientIDClaim + `",
        "claim.value": "` + mcClusterClientID + `",
        "jsonType.label": "String",
        "access.token.claim": "true",
        "id.token.claim": "false",
        "userinfo.token.claim": "false"
      }
    }
  ]
}`
}

// registerRealmClient creates representation in the realm of mc, through the
// Keycloak admin API, as the administrator that the Keycloak Operator
// published next to the Keycloak.
//
// Management Identity creates a client for every component of the management
// plane and none for an orchestration cluster, so a cluster on this realm
// needs the client that an administrator registers for it. A client that the
// realm already holds is left alone, which makes a repeated call harmless.
func registerRealmClient(mc *v1.CamundaManagementCluster, representation string) error {
	adminSecret := components.KeycloakInitialAdminSecretName(mc)

	_, err := utils.RunPod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keycloak-client-" + utilrand.String(5),
			Namespace: mc.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "curl",
				Image:   utils.CurlImage,
				Command: []string{"sh"},
				Args:    []string{"-ec", registerClientScript},
				Env: []corev1.EnvVar{
					{Name: "KC_URL", Value: keycloakServiceURL(mc)},
					{Name: "KC_REALM", Value: mcRealm},
					{Name: "KC_CLIENT", Value: representation},
					utils.SecretEnv("KC_USER", adminSecret, components.KeycloakAdminUsernameKey),
					utils.SecretEnv("KC_PASSWORD", adminSecret, components.KeycloakAdminPasswordKey),
				},
			}},
		},
	}, podTimeout)

	return err
}

// realmOptimizeClient returns the JSON representation of the optimize client
// of the realm of mc, read through the Keycloak admin API as the administrator
// that the Keycloak Operator published next to the Keycloak.
func realmOptimizeClient(mc *v1.CamundaManagementCluster) (string, error) {
	adminSecret := components.KeycloakInitialAdminSecretName(mc)

	return utils.RunPod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keycloak-read-" + utilrand.String(5),
			Namespace: mc.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "curl",
				Image:   utils.CurlImage,
				Command: []string{"sh"},
				Args:    []string{"-ec", readClientScript},
				Env: []corev1.EnvVar{
					{Name: "KC_URL", Value: keycloakServiceURL(mc)},
					{Name: "KC_REALM", Value: mcRealm},
					{Name: "KC_CLIENT_ID", Value: "optimize"},
					utils.SecretEnv("KC_USER", adminSecret, components.KeycloakAdminUsernameKey),
					utils.SecretEnv("KC_PASSWORD", adminSecret, components.KeycloakAdminPasswordKey),
				},
			}},
		},
	}, podTimeout)
}

// realmRoles returns the JSON list of the realm roles of mc, read through the
// Keycloak admin API.
func realmRoles(mc *v1.CamundaManagementCluster) (string, error) {
	adminSecret := components.KeycloakInitialAdminSecretName(mc)

	return utils.RunPod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keycloak-roles-" + utilrand.String(5),
			Namespace: mc.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "curl",
				Image:   utils.CurlImage,
				Command: []string{"sh"},
				Args:    []string{"-ec", readRolesScript},
				Env: []corev1.EnvVar{
					{Name: "KC_URL", Value: keycloakServiceURL(mc)},
					{Name: "KC_REALM", Value: mcRealm},
					utils.SecretEnv("KC_USER", adminSecret, components.KeycloakAdminUsernameKey),
					utils.SecretEnv("KC_PASSWORD", adminSecret, components.KeycloakAdminPasswordKey),
				},
			}},
		},
	}, podTimeout)
}

// readRolesScript reads an administrator token from the master realm and
// prints the realm roles.
const readRolesScript = `KC_TOKEN=$(curl -sS ` +
	`-d grant_type=password -d client_id=admin-cli ` +
	`--data-urlencode "username=$KC_USER" --data-urlencode "password=$KC_PASSWORD" ` +
	`"$KC_URL/realms/master/protocol/openid-connect/token" | ` +
	`sed -n 's/.*"access_token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
if [ -z "$KC_TOKEN" ]; then echo "no access_token from $KC_URL" >&2; exit 1; fi
curl -sS -f -H "Authorization: Bearer $KC_TOKEN" ` +
	`"$KC_URL/admin/realms/$KC_REALM/roles" | tr -d ' '`

// readClientScript reads an administrator token from the master realm and
// prints the clients of the realm whose client id is KC_CLIENT_ID.
const readClientScript = `KC_TOKEN=$(curl -sS ` +
	`-d grant_type=password -d client_id=admin-cli ` +
	`--data-urlencode "username=$KC_USER" --data-urlencode "password=$KC_PASSWORD" ` +
	`"$KC_URL/realms/master/protocol/openid-connect/token" | ` +
	`sed -n 's/.*"access_token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
if [ -z "$KC_TOKEN" ]; then echo "no access_token from $KC_URL" >&2; exit 1; fi
curl -sS -f -H "Authorization: Bearer $KC_TOKEN" ` +
	`"$KC_URL/admin/realms/$KC_REALM/clients?clientId=$KC_CLIENT_ID"`

// registerClientScript reads an administrator token from the master realm and
// posts one client to the realm. The curl image carries no jq, so the token
// is cut out with sed, as the client-credentials helper of the suite does. A
// 409 is a client the realm already holds.
const registerClientScript = `KC_TOKEN=$(curl -sS ` +
	`-d grant_type=password -d client_id=admin-cli ` +
	`--data-urlencode "username=$KC_USER" --data-urlencode "password=$KC_PASSWORD" ` +
	`"$KC_URL/realms/master/protocol/openid-connect/token" | ` +
	`sed -n 's/.*"access_token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
if [ -z "$KC_TOKEN" ]; then echo "no access_token from $KC_URL" >&2; exit 1; fi
KC_CODE=$(printf '%s' "$KC_CLIENT" | curl -sS -o /tmp/response -w '%{http_code}' ` +
	`-H "Authorization: Bearer $KC_TOKEN" -H 'Content-Type: application/json' ` +
	`--data-binary @- "$KC_URL/admin/realms/$KC_REALM/clients")
echo "$KC_CODE"
cat /tmp/response
case "$KC_CODE" in 201|409) ;; *) exit 1 ;; esac`

// oidcManagementClientSecret returns the Secret that holds the client secret
// of every confidential management client of the realm of
// testdata/keycloak.yaml.
//
// It lives in the manager namespace, which outlives the namespace of the flow,
// so the suite applies it rather than creating it: a run that ends before its
// cleanup must not stop the next one. Management Identity reads it through the
// copy that the management plane makes in its own namespace.
func oidcManagementClientSecret() *corev1.Secret {
	return &corev1.Secret{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{Name: mcOIDCClientSecretName, Namespace: namespace},
		Type:       corev1.SecretTypeOpaque,
		StringData: map[string]string{ccOIDCSecretKey: ccOIDCClientSecret},
	}
}

// oidcIssuerURL is the issuer of the realm that testdata/keycloak.yaml serves
// in the namespace of the oidc flow.
func oidcIssuerURL() string {
	return fmt.Sprintf("http://keycloak.%s.svc:8080/realms/%s", mcOIDCNamespace, ccOIDCRealm)
}

// oidcManagementPlatformConfig returns the platform config of the management
// plane of the oidc flow: the realm of testdata/keycloak.yaml and the client
// of every component that authenticates through it.
//
// The three endpoints are set by hand. The operator makes no request to the
// identity provider, and the ManagementAuthConfig carries all three.
func oidcManagementPlatformConfig() *v1.CamundaPlatformConfig {
	issuer := oidcIssuerURL()
	secretRef := v1.SecretKeyRef{
		Name:      mcOIDCClientSecretName,
		Namespace: namespace,
		Key:       ccOIDCSecretKey,
	}

	return &v1.CamundaPlatformConfig{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaPlatformConfig"},
		ObjectMeta: metav1.ObjectMeta{Name: mcOIDCPlatform},
		Spec: v1.CamundaPlatformConfigSpec{
			Auth: &v1.PlatformAuthSpec{
				Method: v1.AuthenticationMethodOIDC,
				OIDC: &v1.OIDCSpec{
					IssuerURL:       issuer,
					AuthURL:         issuer + "/protocol/openid-connect/auth",
					TokenURL:        issuer + "/protocol/openid-connect/token",
					JWKSURL:         issuer + "/protocol/openid-connect/certs",
					ClientID:        ccOIDCClientID,
					UsernameClaim:   ccOIDCUsernameClaim,
					ClientIDClaim:   ccOIDCClientIDClaim,
					ClientSecretRef: secretRef,
					Management: &v1.ManagementOIDCClientsSpec{
						Clients: v1.ManagementClients{
							Identity: &v1.ConfidentialClientSpec{
								ClientID:        mcOIDCIdentityClient,
								ClientSecretRef: secretRef,
							},
							Optimize: &v1.ConfidentialClientSpec{
								ClientID:        mcOIDCOptimizeClient,
								ClientSecretRef: secretRef,
							},
							WebModeler: &v1.PublicClientSpec{ClientID: mcOIDCWebModelerClient},
							WebModelerAPI: &v1.WebModelerAPIClientSpec{
								ConfidentialClientSpec: v1.ConfidentialClientSpec{
									ClientID:        mcOIDCWebModelerAPIClient,
									ClientSecretRef: secretRef,
								},
							},
						},
					},
				},
			},
		},
	}
}

// deploymentEnv returns the plain environment entries of the container of a
// Deployment, by name. Every Deployment of the management plane runs one
// container, and an entry that reads a Secret carries no value of its own, so
// it is absent from the result.
func deploymentEnv(name, namespace string) (map[string]string, error) {
	var deployment appsv1.Deployment
	if err := utils.Get("deployment", name, namespace, &deployment); err != nil {
		return nil, err
	}

	containers := deployment.Spec.Template.Spec.Containers
	if len(containers) != 1 {
		return nil, fmt.Errorf(
			"deployment %s/%s runs %d containers", namespace, name, len(containers),
		)
	}

	return envByName(containers[0].Env), nil
}

// envByName returns the plain entries of env, by name.
func envByName(env []corev1.EnvVar) map[string]string {
	values := make(map[string]string, len(env))
	for _, entry := range env {
		if entry.ValueFrom == nil {
			values[entry.Name] = entry.Value
		}
	}

	return values
}
