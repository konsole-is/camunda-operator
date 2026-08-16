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
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	// ccNamespace hosts the Elasticsearch flow, ccRDBMSNamespace the RDBMS
	// flow. Each flow brings its own storage backend, so the two are
	// independent of each other and of the storage flows of the suite.
	ccNamespace      = "camunda-e2e"
	ccRDBMSNamespace = "camunda-rdbms-e2e"
	// ccName is the CamundaCluster of both flows; the namespace tells them
	// apart. ccPlatform and ccRDBMSPlatform are the cluster-scoped platform
	// configs.
	ccName          = "camunda"
	ccPlatform      = "camunda-e2e"
	ccRDBMSPlatform = "camunda-rdbms-e2e"
	// ccVersion is the Camunda release under test; ccConnectorsVersion the
	// connectors bundle, which has its own patch line.
	ccVersion           = "8.9.9"
	ccConnectorsVersion = "8.9.7"
	// ccStorageConfig is the SecondaryStorageConfig of the Elasticsearch
	// flow, published by its ElasticsearchCluster.
	ccStorageConfig = "camunda-storage"
	// ccRDBMSServer, ccRDBMSDatabase, and ccRDBMSStorageConfig are the
	// DatabaseServerConfig, the Database, and its SecondaryStorageConfig of
	// the RDBMS flow.
	ccRDBMSServer        = "camunda-rdbms-postgres"
	ccRDBMSDatabase      = "camunda-rdbms"
	ccRDBMSStorageConfig = "camunda-rdbms-storage"
	// processID is the BPMN process id of testdata/process.bpmn.
	processID   = "e2e-process"
	processFile = "process.bpmn"

	ccResource         = "camundaclusters.core.camunda.io"
	ccPlatformResource = "camundaplatformconfigs.core.camunda.io"

	// ccReadyTimeout covers the pulls of the Camunda images (about 2 GB) and
	// the broker and gateway startup on a fresh kind node.
	ccReadyTimeout = 15 * time.Minute
	// ccAPITimeout bounds the wait for a REST answer through the gateway
	// Service, for example the export of an instance to secondary storage.
	ccAPITimeout = 3 * time.Minute
)

// The Orchestration Cluster REST API paths of Camunda 8.9, on the HTTP port
// of the gateway:
//   - GET /v2/topology
//     https://docs.camunda.io/docs/apis-tools/orchestration-cluster-api-rest/specifications/get-topology/
//   - POST /v2/deployments (multipart, form part "resources")
//     https://docs.camunda.io/docs/apis-tools/orchestration-cluster-api-rest/specifications/create-deployment/
//   - POST /v2/process-instances ({"processDefinitionId": ...})
//     https://docs.camunda.io/docs/apis-tools/orchestration-cluster-api-rest/specifications/create-process-instance/
//   - POST /v2/process-instances/search ({"filter": {"processDefinitionId": ...}})
//     https://docs.camunda.io/docs/apis-tools/orchestration-cluster-api-rest/specifications/search-process-instances/
//
// The web applications answer under /operate/, /tasklist/, and /admin/
// (https://docs.camunda.io/docs/reference/announcements-release-notes/890/890-announcements/#identity).
const (
	pathTopology         = "/v2/topology"
	pathDeployments      = "/v2/deployments"
	pathProcessInstances = "/v2/process-instances"
	pathInstanceSearch   = "/v2/process-instances/search"
)

var webAppPaths = []string{"/operate/", "/tasklist/", "/admin/"}

// newCluster returns the CamundaCluster of a flow: the 8.9 default topology
// (one broker, one standalone gateway that hosts the web applications) on
// the named storage binding, sized for a kind node.
func newCluster(namespace, platform, storageRef string, connectors bool) *v1.CamundaCluster {
	cluster := &v1.CamundaCluster{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaCluster"},
		ObjectMeta: metav1.ObjectMeta{Name: ccName, Namespace: namespace},
		Spec: v1.CamundaClusterSpec{
			PlatformConfigRef: platform,
			Version:           ccVersion,
			StorageRef:        storageRef,
			Zeebe: &v1.ZeebeSpec{
				WorkloadSpec: v1.WorkloadSpec{Resources: requests("1", "1.5Gi")},
				StorageSize:  new(resource.MustParse("1Gi")),
			},
			Gateway: &v1.GatewaySpec{WorkloadSpec: v1.WorkloadSpec{Resources: requests("500m", "512Mi")}},
		},
	}

	if connectors {
		cluster.Spec.Connectors = &v1.ConnectorsSpec{
			Enabled:      new(true),
			Version:      ccConnectorsVersion,
			WorkloadSpec: v1.WorkloadSpec{Resources: requests("250m", "512Mi")},
		}
	}

	return cluster
}

func requests(cpu, memory string) *corev1.ResourceRequirements {
	return &corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpu),
			corev1.ResourceMemory: resource.MustParse(memory),
		},
	}
}

// basicPlatform returns a platform config with basic authentication and no
// license: the cluster then runs in non-production mode.
func basicPlatform(name string) *v1.CamundaPlatformConfig {
	return &v1.CamundaPlatformConfig{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaPlatformConfig"},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: v1.CamundaPlatformConfigSpec{
			Auth: &v1.PlatformAuthSpec{Method: v1.AuthenticationMethodBasic},
		},
	}
}

var _ = Describe("CamundaCluster", Ordered, func() {
	var (
		elasticsearch = &v1.ElasticsearchCluster{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "ElasticsearchCluster"},
			ObjectMeta: metav1.ObjectMeta{Name: esName, Namespace: ccNamespace},
			Spec: v1.ElasticsearchClusterSpec{
				Version:                esVersion,
				Replicas:               new(int32(1)),
				StorageSize:            new(resource.MustParse(esStorageSize)),
				Resources:              requests("500m", "1Gi"),
				SecondaryStorageConfig: ccStorageConfig,
			},
		}
		cluster = newCluster(ccNamespace, ccPlatform, ccStorageConfig, true)
		// brokerClaim is the data volume of the single broker, as it was
		// bound before suspension.
		brokerClaim corev1.PersistentVolumeClaim
	)

	BeforeAll(func() {
		By("creating the test namespace")
		_, err := utils.Kubectl("create", "ns", ccNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("creating the ElasticsearchCluster and waiting for Ready Healthy")
		Expect(apply(elasticsearch)).To(Succeed())
		Eventually(func(g Gomega) {
			expectReady(g, esResource, esName, ccNamespace, v1.ReasonHealthy)
			expectReady(g, sscResource, ccStorageConfig, ccNamespace, v1.ReasonHealthy)
		}, esReadyTimeout, 5*time.Second).Should(Succeed())

		By("creating the platform config")
		Expect(apply(basicPlatform(ccPlatform))).To(Succeed())
	})

	AfterAll(func() {
		By("removing the cluster, the platform config, and the test namespace")
		_, _ = utils.Kubectl("delete", ccResource, ccName, "-n", ccNamespace, "--ignore-not-found", "--wait=false")
		_, _ = utils.Kubectl("delete", ccPlatformResource, ccPlatform, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", "ns", ccNamespace, "--wait=false")
	})

	AfterEach(func() {
		dumpDiagnostics(ccNamespace)
	})

	itRunsTheOrchestrationCluster(cluster)

	It("runs Operate standalone and folds it back into the gateway", func() {
		operate := components.WorkloadName(cluster, components.ComponentOperate)

		By("setting spec.operate.mode to Standalone")
		_, err := utils.Kubectl(
			"patch", ccResource, ccName, "-n", ccNamespace,
			"--type=merge", "-p", `{"spec":{"operate":{"mode":"Standalone"}}}`,
		)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the Operate Deployment and OperateReady Healthy")
		Eventually(func(g Gomega) {
			exists, err := utils.Exists("deployment", operate, ccNamespace)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(exists).To(BeTrue(), "deployment %q does not exist yet", operate)
			expectCondition(g, ccResource, ccName, ccNamespace, v1.ConditionOperateReady, v1.ReasonHealthy)
		}, ccReadyTimeout, 5*time.Second).Should(Succeed())

		By("serving Operate on its own Service with the admin credentials")
		Eventually(func(g Gomega) {
			status, body, err := camundaRESTOn(
				cluster,
				components.ComponentOperate,
				"operate",
				http.MethodGet,
				"/operate/",
				nil,
			)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(status).To(Equal(http.StatusOK), body)
		}, ccAPITimeout).Should(Succeed())

		By("setting spec.operate.mode back to Embedded")
		_, err = utils.Kubectl(
			"patch", ccResource, ccName, "-n", ccNamespace,
			"--type=merge", "-p", `{"spec":{"operate":{"mode":"Embedded"}}}`,
		)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the Operate Deployment and Service to be gone and OperateReady Disabled")
		Eventually(func(g Gomega) {
			expectGone(g, "deployment", operate, ccNamespace)
			expectGone(g, "service", operate, ccNamespace)
			expectCondition(g, ccResource, ccName, ccNamespace, v1.ConditionOperateReady, string(component.Disabled))
		}, 5*time.Minute, 5*time.Second).Should(Succeed())

		By("serving Operate on the gateway again")
		Eventually(func(g Gomega) {
			status, body, err := camundaREST(cluster, "webapp", http.MethodGet, "/operate/", nil)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(status).To(Equal(http.StatusOK), body)
		}, ccReadyTimeout, 5*time.Second).Should(Succeed())
	})

	It("runs the connectors runtime against the gateway", func() {
		Eventually(func(g Gomega) {
			expectCondition(g, ccResource, ccName, ccNamespace, v1.ConditionConnectorsReady, v1.ReasonHealthy)
		}, ccReadyTimeout, 5*time.Second).Should(Succeed())
	})

	It("suspends every workload to zero replicas and keeps the broker volume", func() {
		By("recording the bound broker volume")
		var claims corev1.PersistentVolumeClaimList
		Expect(utils.List("pvc", ccNamespace, brokerClaimSelector(cluster), &claims)).To(Succeed())
		Expect(claims.Items).To(HaveLen(1))
		Expect(claims.Items[0].Status.Phase).To(Equal(corev1.ClaimBound))
		brokerClaim = claims.Items[0]

		By("setting spec.suspend")
		_, err := utils.Kubectl(
			"patch", ccResource, ccName, "-n", ccNamespace,
			"--type=merge", "-p", `{"spec":{"suspend":true}}`,
		)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for Ready Suspended and every workload at zero replicas")
		Eventually(func(g Gomega) {
			expectReady(g, ccResource, ccName, ccNamespace, "Suspended")
			expectScaledToZero(
				g,
				"statefulset",
				components.WorkloadName(cluster, components.ComponentZeebe),
				ccNamespace,
			)
			expectScaledToZero(
				g,
				"deployment",
				components.WorkloadName(cluster, components.ComponentGateway),
				ccNamespace,
			)
			expectScaledToZero(
				g, "deployment", components.WorkloadName(cluster, components.ComponentConnectors), ccNamespace,
			)
		}, 5*time.Minute).Should(Succeed())

		By("checking that the broker volume stays bound")
		var claim corev1.PersistentVolumeClaim
		Expect(utils.Get("pvc", brokerClaim.Name, ccNamespace, &claim)).To(Succeed())
		Expect(claim.UID).To(Equal(brokerClaim.UID))
		Expect(claim.Status.Phase).To(Equal(corev1.ClaimBound))
	})

	It("resumes on the same broker volume with the deployed process still present", func() {
		By("clearing spec.suspend")
		_, err := utils.Kubectl(
			"patch", ccResource, ccName, "-n", ccNamespace,
			"--type=merge", "-p", `{"spec":{"suspend":false}}`,
		)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for Ready Healthy")
		Eventually(func(g Gomega) {
			expectReady(g, ccResource, ccName, ccNamespace, v1.ReasonHealthy)
		}, ccReadyTimeout, 5*time.Second).Should(Succeed())

		By("checking that the broker runs on the same volume")
		var claim corev1.PersistentVolumeClaim
		Expect(utils.Get("pvc", brokerClaim.Name, ccNamespace, &claim)).To(Succeed())
		Expect(claim.UID).To(Equal(brokerClaim.UID))

		By("searching the process instance that was started before suspension")
		expectInstanceSearchable(cluster)
	})

	It("garbage-collects the workloads and, by default, the broker volume on deletion", func() {
		_, err := utils.Kubectl("delete", ccResource, ccName, "-n", ccNamespace, "--wait=false")
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the owned resources to be gone")
		Eventually(func(g Gomega) {
			expectGone(g, ccResource, ccName, ccNamespace)
			expectGone(g, "statefulset", components.WorkloadName(cluster, components.ComponentZeebe), ccNamespace)
			expectGone(g, "deployment", components.WorkloadName(cluster, components.ComponentGateway), ccNamespace)
			expectGone(g, "deployment", components.WorkloadName(cluster, components.ComponentConnectors), ccNamespace)
			expectGone(g, "service", components.WorkloadName(cluster, components.ComponentGateway), ccNamespace)
			expectGone(g, "secret", components.AdminSecretName(cluster), ccNamespace)
		}, 5*time.Minute).Should(Succeed())

		By("waiting for the broker volume to be gone")
		Eventually(func(g Gomega) {
			expectGone(g, "pvc", brokerClaim.Name, ccNamespace)
		}, 3*time.Minute).Should(Succeed())
	})
})

var _ = Describe("CamundaCluster on RDBMS", Ordered, func() {
	var (
		server = &v1.DatabaseServerConfig{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "DatabaseServerConfig"},
			ObjectMeta: metav1.ObjectMeta{Name: ccRDBMSServer},
			Spec: v1.DatabaseServerConfigSpec{
				Engine: v1.DatabaseEnginePostgres,
				Host:   postgresService + "." + ccRDBMSNamespace + ".svc",
				Port:   5432,
				AdminCredentialsSecretRef: v1.CredentialsSecretRef{
					Name:        postgresAdminSecret,
					Namespace:   ccRDBMSNamespace,
					UsernameKey: "username",
					PasswordKey: "password",
				},
			},
		}
		database = &v1.Database{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "Database"},
			ObjectMeta: metav1.ObjectMeta{Name: ccRDBMSDatabase},
			Spec: v1.DatabaseSpec{
				ServerRef:              ccRDBMSServer,
				DatabaseName:           dbDatabaseName,
				TargetNamespace:        ccRDBMSNamespace,
				SecondaryStorageConfig: ccRDBMSStorageConfig,
			},
		}
		cluster = newCluster(ccRDBMSNamespace, ccRDBMSPlatform, ccRDBMSStorageConfig, false)
	)

	BeforeAll(func() {
		By("creating the test namespace")
		_, err := utils.Kubectl("create", "ns", ccRDBMSNamespace)
		Expect(err).NotTo(HaveOccurred())

		By("deploying PostgreSQL")
		_, err = utils.Kubectl("apply", "-n", ccRDBMSNamespace, "-f", "test/e2e/testdata/postgres.yaml")
		Expect(err).NotTo(HaveOccurred())
		_, err = utils.Kubectl(
			"rollout", "status", "deployment/postgres", "-n", ccRDBMSNamespace, "--timeout", "5m",
		)
		Expect(err).NotTo(HaveOccurred())

		By("creating the Database and waiting for its binding")
		Expect(apply(server)).To(Succeed())
		Expect(apply(database)).To(Succeed())
		Eventually(func(g Gomega) {
			expectReady(g, dbResource, ccRDBMSDatabase, "", v1.ReasonHealthy)
			expectReady(g, sscResource, ccRDBMSStorageConfig, ccRDBMSNamespace, v1.ReasonHealthy)
		}, 3*time.Minute).Should(Succeed())

		By("creating the platform config")
		Expect(apply(basicPlatform(ccRDBMSPlatform))).To(Succeed())
	})

	AfterAll(func() {
		By("removing the cluster, the database, the platform config, and the test namespace")
		_, _ = utils.Kubectl("delete", ccResource, ccName, "-n", ccRDBMSNamespace, "--ignore-not-found", "--wait=false")
		_, _ = utils.Kubectl("delete", dbResource, ccRDBMSDatabase, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", dbServerResource, ccRDBMSServer, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", ccPlatformResource, ccRDBMSPlatform, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", "ns", ccRDBMSNamespace, "--wait=false")
	})

	AfterEach(func() {
		dumpDiagnostics(ccRDBMSNamespace)
	})

	itRunsTheOrchestrationCluster(cluster)
})

// itRunsTheOrchestrationCluster registers the specs that both flows share:
// the cluster becomes Ready, answers the topology and the web applications
// through the gateway with the admin credentials, and exports a started
// process instance to its secondary storage.
func itRunsTheOrchestrationCluster(cluster *v1.CamundaCluster) {
	It("reaches Ready Healthy", func() {
		By("creating the CamundaCluster")
		Expect(apply(cluster)).To(Succeed())

		By("waiting for Ready Healthy")
		Eventually(func(g Gomega) {
			expectReady(g, ccResource, cluster.Name, cluster.Namespace, v1.ReasonHealthy)
		}, ccReadyTimeout, 5*time.Second).Should(Succeed())
	})

	It("reports one broker and one partition through the gateway with the admin credentials", func() {
		var topology struct {
			Brokers []struct {
				Partitions []struct {
					Health string `json:"health"`
				} `json:"partitions"`
			} `json:"brokers"`
			ClusterSize     int `json:"clusterSize"`
			PartitionsCount int `json:"partitionsCount"`
		}

		Eventually(func(g Gomega) {
			status, body, err := camundaREST(cluster, "topology", http.MethodGet, pathTopology, nil)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(status).To(Equal(http.StatusOK), body)
			g.Expect(json.Unmarshal([]byte(body), &topology)).To(Succeed(), body)
			g.Expect(topology.Brokers).To(HaveLen(1), body)
			g.Expect(topology.Brokers[0].Partitions).To(HaveLen(1), body)
			g.Expect(topology.Brokers[0].Partitions).To(HaveEach(HaveField("Health", "healthy")), body)
			g.Expect(topology.ClusterSize).To(Equal(1), body)
			g.Expect(topology.PartitionsCount).To(Equal(1), body)
		}, ccAPITimeout).Should(Succeed())
	})

	It("serves Operate, Tasklist, and Admin on the gateway", func() {
		for _, path := range webAppPaths {
			Eventually(func(g Gomega) {
				status, body, err := camundaREST(cluster, "webapp", http.MethodGet, path, nil)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(status).To(Equal(http.StatusOK), "%s: %s", path, body)
			}, ccAPITimeout).Should(Succeed())
		}
	})

	It("deploys a process, starts an instance, and finds it in secondary storage", func() {
		By("deploying the process")
		dir, err := utils.GetProjectDir()
		Expect(err).NotTo(HaveOccurred())
		bpmn, err := os.ReadFile(filepath.Join(dir, "test", "e2e", "testdata", processFile))
		Expect(err).NotTo(HaveOccurred())

		status, body, err := camundaREST(
			cluster, "deploy", http.MethodPost, pathDeployments,
			map[string]string{processFile: string(bpmn)},
			"-F", "resources=@/tmp/"+processFile,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal(http.StatusOK), body)
		Expect(body).To(ContainSubstring(`"processDefinitionId":"` + processID + `"`))

		By("starting an instance")
		status, body, err = camundaREST(
			cluster, "start", http.MethodPost, pathProcessInstances, nil,
			"-H", "Content-Type: application/json", "-d", `{"processDefinitionId":"`+processID+`"}`,
		)
		Expect(err).NotTo(HaveOccurred())
		Expect(status).To(Equal(http.StatusOK), body)
		Expect(body).To(ContainSubstring(`"processInstanceKey"`))

		By("searching the instance in secondary storage")
		expectInstanceSearchable(cluster)
	})
}

// expectInstanceSearchable waits until the process instance search returns
// an instance of the e2e process. The search reads secondary storage, so it
// proves the export.
func expectInstanceSearchable(cluster *v1.CamundaCluster) {
	var result struct {
		Items []struct {
			ProcessDefinitionID string `json:"processDefinitionId"`
		} `json:"items"`
	}

	Eventually(func(g Gomega) {
		status, body, err := camundaREST(
			cluster, "search", http.MethodPost, pathInstanceSearch, nil,
			"-H", "Content-Type: application/json", "-d", `{"filter":{"processDefinitionId":"`+processID+`"}}`,
		)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(status).To(Equal(http.StatusOK), body)
		g.Expect(json.Unmarshal([]byte(body), &result)).To(Succeed(), body)
		g.Expect(result.Items).NotTo(BeEmpty(), body)
		g.Expect(result.Items).To(HaveEach(HaveField("ProcessDefinitionID", processID)), body)
	}, ccAPITimeout).Should(Succeed())
}

// camundaREST calls path on the gateway Service of cluster with the
// credentials of the admin Secret and returns the final status code and the
// body. files are uploaded into /tmp of the helper pod; args are extra curl
// arguments.
func camundaREST(
	cluster *v1.CamundaCluster,
	name, method, path string,
	files map[string]string,
	args ...string,
) (int, string, error) {
	return camundaRESTOn(cluster, components.ComponentGateway, name, method, path, files, args...)
}

// camundaRESTOn is camundaREST against the HTTP port of the Service of the
// named component.
func camundaRESTOn(
	cluster *v1.CamundaCluster,
	component, name, method, path string,
	files map[string]string,
	args ...string,
) (int, string, error) {
	return utils.CamundaREST(utils.CamundaRequest{
		Namespace: cluster.Namespace,
		Name:      name,
		Method:    method,
		URL: fmt.Sprintf(
			"http://%s.%s.svc:%d%s",
			components.WorkloadName(cluster, component), cluster.Namespace, components.PortHTTP, path,
		),
		CredentialsSecret: components.AdminSecretName(cluster),
		UsernameKey:       components.AdminUsernameKey,
		PasswordKey:       components.AdminPasswordKey,
		Files:             files,
		Args:              args,
		Timeout:           podTimeout,
	})
}

// brokerClaimSelector is the label selector of the broker volume claims of
// cluster, as kubectl takes it.
func brokerClaimSelector(cluster *v1.CamundaCluster) string {
	return k8slabels.SelectorFromSet(components.BrokerClaimSelector(cluster)).String()
}

// expectScaledToZero asserts that the workload asks for zero replicas and
// runs none. It is written for Eventually.
func expectScaledToZero(g Gomega, resource, name, namespace string) {
	var workload struct {
		Spec struct {
			Replicas *int32 `json:"replicas"`
		} `json:"spec"`
		Status struct {
			Replicas int32 `json:"replicas"`
		} `json:"status"`
	}
	g.Expect(utils.Get(resource, name, namespace, &workload)).To(Succeed())
	g.Expect(workload.Spec.Replicas).To(HaveValue(BeEquivalentTo(0)), "%s %q is not scaled to zero", resource, name)
	g.Expect(workload.Status.Replicas).To(BeZero(), "%s %q still runs pods", resource, name)
}
