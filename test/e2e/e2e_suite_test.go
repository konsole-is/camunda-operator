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
	"os"
	"os/exec"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"

	"github.com/konsole-is/camunda-operator/test/utils"
)

var (
	// managerImage is the manager image to be built and loaded for testing.
	managerImage = "example.com/camunda-operator:v0.0.1"
	// cliImage is the camunda-operator-cli image to be built and loaded for
	// testing. The manager requires it, and the backup Jobs run it, so the
	// suite builds and loads it exactly like the manager image. The default
	// of the Makefile names a published image that a Kind node cannot pull.
	cliImage = "example.com/camunda-operator-cli:v0.0.1"
	// shouldCleanupCertManager tracks whether CertManager was installed by this suite.
	shouldCleanupCertManager = false
	// shouldCleanupECK tracks whether the ECK operator was installed by this suite.
	shouldCleanupECK = false
	// shouldCleanupCNPG tracks whether CloudNativePG and the Barman Cloud
	// plugin were installed by this suite.
	shouldCleanupCNPG = false
	// shouldCleanupKeycloakCRDs tracks whether the Keycloak CRDs were
	// installed by this suite.
	shouldCleanupKeycloakCRDs = false
)

// TestE2E runs the e2e test suite to validate the solution in an isolated environment.
// The default setup requires Kind, CertManager, CloudNativePG with the Barman
// Cloud plugin, the ECK operator, and the Keycloak Operator. The suite
// installs all of them when the cluster does not serve them.
//
// To skip CertManager installation, set: CERT_MANAGER_INSTALL_SKIP=true
// To skip the CloudNativePG and Barman Cloud plugin installation, set: CNPG_INSTALL_SKIP=true
// To skip ECK installation, set: ECK_INSTALL_SKIP=true
// To install a different ECK release, set: ECK_VERSION=<version>
// To skip the Keycloak CRD and Keycloak Operator installation, set: KEYCLOAK_OPERATOR_INSTALL_SKIP=true
// To install a different Keycloak Operator release, set: KEYCLOAK_OPERATOR_VERSION=<version>
// To run against another Camunda minor: make test-e2e E2E_CAMUNDA_MINOR=<minor>
// That picks the file of test/e2e/matrix/ that holds the image versions and
// the label list of the run.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting camunda-operator e2e test suite\n")

	// -ginkgo.label-filter on the command line wins over the list of the
	// matrix entry.
	suiteConfig, reporterConfig := GinkgoConfiguration()
	if suiteConfig.LabelFilter == "" {
		filter, err := utils.LabelFilter(os.Getenv(envLabels), utils.AllLabels)
		require.NoError(t, err, envLabels)
		suiteConfig.LabelFilter = filter
	}

	RunSpecs(t, "e2e suite", suiteConfig, reporterConfig)
}

// The suite deploys the manager once, after CloudNativePG, ECK, and the
// Keycloak CRDs. Each controller attaches its watch on the kind it finds at
// startup: the DatabaseServer controller on the CloudNativePG Cluster and the
// ObjectStore kinds, the ElasticsearchCluster controller on the ECK
// Elasticsearch kind, and the CamundaManagementCluster controller on the
// Keycloak kind. A run that skips one of these installs runs no flow of that
// kind, and the controller reports the kind as not installed. The Keycloak
// Operator itself is installed by the flow that creates a Keycloak, because
// it reserves CPU for as long as it runs.
var _ = BeforeSuite(func() {
	By("checking the image versions of the run")
	for _, name := range versionEnv {
		Expect(os.Getenv(name)).NotTo(
			BeEmpty(),
			"%s is not set. Run the suite through make test-e2e, which exports it from test/e2e/matrix/<minor>.env",
			name,
		)
	}

	By("building the manager image")
	cmd := exec.Command("make", "docker-build", fmt.Sprintf("IMG=%s", managerImage))
	_, err := utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the manager image")

	// TODO(user): If you want to change the e2e test vendor from Kind,
	// ensure the image is built and available, then remove the following block.
	By("loading the manager image on Kind")
	err = utils.LoadImageToKindClusterWithName(managerImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the manager image into Kind")

	By("building the camunda-operator-cli image")
	cmd = exec.Command("make", "docker-build-cli", fmt.Sprintf("CLI_IMG=%s", cliImage))
	_, err = utils.Run(cmd)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to build the camunda-operator-cli image")

	By("loading the camunda-operator-cli image on Kind")
	err = utils.LoadImageToKindClusterWithName(cliImage)
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "Failed to load the camunda-operator-cli image into Kind")

	setupCertManager()
	setupCNPG()
	setupECK()
	setupKeycloakCRDs()
	deployManager()
	setupMinIO()
})

var _ = AfterSuite(func() {
	teardownMinIO()
	undeployManager()
	teardownKeycloakCRDs()
	teardownECK()
	teardownCNPG()
	teardownCertManager()
})

// A spec without a label never runs under a label filter, so a container
// that forgot its Label would drop its flow from every matrix entry that
// selects by name. The report lists every spec, selected or not.
var _ = ReportAfterSuite("every spec carries a label", func(report Report) {
	for _, spec := range report.SpecReports {
		if spec.LeafNodeType != types.NodeTypeIt {
			continue
		}
		Expect(spec.Labels()).NotTo(BeEmpty(), "%s has no label", spec.FullText())
	}
})

// deployManager creates the manager namespace with the restricted security
// policy, installs the CRDs, and deploys the controller-manager.
func deployManager() {
	By("creating manager namespace")
	cmd := exec.Command("kubectl", "create", "ns", namespace)
	_, err := utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

	By("labeling the namespace to enforce the restricted security policy")
	cmd = exec.Command(
		"kubectl", "label", "--overwrite", "ns", namespace,
		"pod-security.kubernetes.io/enforce=restricted",
	)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with restricted policy")

	By("installing CRDs")
	cmd = exec.Command("make", "install")
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to install CRDs")

	By("deploying the controller-manager")
	cmd = exec.Command(
		"make", "deploy",
		fmt.Sprintf("IMG=%s", managerImage),
		fmt.Sprintf("CLI_IMG=%s", cliImage),
	)
	_, err = utils.Run(cmd)
	Expect(err).NotTo(HaveOccurred(), "Failed to deploy the controller-manager")
}

// undeployManager removes the controller-manager, the CRDs, and the manager
// namespace.
func undeployManager() {
	By("undeploying the controller-manager")
	cmd := exec.Command("make", "undeploy")
	_, _ = utils.Run(cmd)

	By("uninstalling CRDs")
	cmd = exec.Command("make", "uninstall")
	_, _ = utils.Run(cmd)

	By("removing manager namespace")
	cmd = exec.Command("kubectl", "delete", "ns", namespace)
	_, _ = utils.Run(cmd)
}

// setupMinIO deploys the object store of the backup flows. Each flow creates
// the bucket contract in its own namespace with createBackupStorage.
func setupMinIO() {
	By("creating the MinIO namespace")
	_, err := utils.Kubectl("create", "ns", minioNamespace)
	Expect(err).NotTo(HaveOccurred(), "Failed to create the MinIO namespace")

	By("deploying MinIO and creating the bucket")
	Expect(utils.InstallMinIO(minioNamespace)).To(Succeed(), "Failed to install MinIO")
}

// teardownMinIO removes the MinIO namespace.
func teardownMinIO() {
	By("removing the MinIO namespace")
	_, _ = utils.Kubectl("delete", "ns", minioNamespace, "--wait=false")
}

// setupCertManager installs CertManager if needed for webhook tests.
// Skips installation if CERT_MANAGER_INSTALL_SKIP=true or if already present.
func setupCertManager() {
	if os.Getenv("CERT_MANAGER_INSTALL_SKIP") == "true" {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping CertManager installation (CERT_MANAGER_INSTALL_SKIP=true)\n")
		return
	}

	By("checking if CertManager is already installed")
	if utils.IsCertManagerCRDsInstalled() {
		_, _ = fmt.Fprintf(GinkgoWriter, "CertManager is already installed. Skipping installation.\n")
		return
	}

	// Mark for cleanup before installation to handle interruptions and partial installs.
	shouldCleanupCertManager = true

	By("installing CertManager")
	Expect(utils.InstallCertManager()).To(Succeed(), "Failed to install CertManager")
}

// teardownCertManager uninstalls CertManager if it was installed by setupCertManager.
// This ensures we only remove what we installed.
func teardownCertManager() {
	if !shouldCleanupCertManager {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping CertManager cleanup (not installed by this suite)\n")
		return
	}

	By("uninstalling CertManager")
	utils.UninstallCertManager()
}

// setupCNPG installs CloudNativePG and the Barman Cloud plugin that the
// DatabaseServer flow drives. Skips installation if CNPG_INSTALL_SKIP=true or
// if already present. It runs after CertManager, which issues the certificate
// the plugin serves its CNPG-I endpoint with.
func setupCNPG() {
	if os.Getenv("CNPG_INSTALL_SKIP") == "true" {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping CloudNativePG installation (CNPG_INSTALL_SKIP=true)\n")
		return
	}

	By("checking if CloudNativePG and the Barman Cloud plugin are already installed")
	if utils.IsCNPGInstalled() {
		_, _ = fmt.Fprintf(GinkgoWriter, "CloudNativePG is already installed. Skipping installation.\n")
		return
	}

	shouldCleanupCNPG = true

	By(fmt.Sprintf("installing CloudNativePG %s", utils.CNPGVersion()))
	Expect(utils.InstallCNPG()).To(Succeed(), "Failed to install CloudNativePG")

	By(fmt.Sprintf("installing the Barman Cloud plugin %s", utils.BarmanPluginVersion()))
	Expect(utils.InstallBarmanPlugin()).To(Succeed(), "Failed to install the Barman Cloud plugin")
}

// teardownCNPG uninstalls CloudNativePG and the Barman Cloud plugin if
// setupCNPG installed them.
func teardownCNPG() {
	if !shouldCleanupCNPG {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping CloudNativePG cleanup (not installed by this suite)\n")
		return
	}

	By("uninstalling CloudNativePG and the Barman Cloud plugin")
	utils.UninstallCNPG()
}

// setupECK installs the ECK operator that the ElasticsearchCluster flow
// drives. Skips installation if ECK_INSTALL_SKIP=true or if already present.
func setupECK() {
	if os.Getenv("ECK_INSTALL_SKIP") == "true" {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping ECK installation (ECK_INSTALL_SKIP=true)\n")
		return
	}

	By("checking if ECK is already installed")
	if utils.IsECKInstalled() {
		_, _ = fmt.Fprintf(GinkgoWriter, "ECK is already installed. Skipping installation.\n")
		return
	}

	shouldCleanupECK = true

	By(fmt.Sprintf("installing ECK %s", utils.ECKVersion()))
	Expect(utils.InstallECK()).To(Succeed(), "Failed to install ECK")
}

// teardownECK uninstalls ECK if it was installed by setupECK.
func teardownECK() {
	if !shouldCleanupECK {
		_, _ = fmt.Fprintf(GinkgoWriter, "Skipping ECK cleanup (not installed by this suite)\n")
		return
	}

	By("uninstalling ECK")
	utils.UninstallECK()
}

// setupKeycloakCRDs installs the custom resource definitions of the Keycloak
// Operator, so that the manager finds the Keycloak kind when it starts. Skips
// installation if KEYCLOAK_OPERATOR_INSTALL_SKIP=true or if already present.
// The operator itself is installed by the management-keycloak flow, in its
// own namespace, for the time of that flow.
func setupKeycloakCRDs() {
	if os.Getenv("KEYCLOAK_OPERATOR_INSTALL_SKIP") == "true" {
		_, _ = fmt.Fprintf(
			GinkgoWriter, "Skipping Keycloak CRD installation (KEYCLOAK_OPERATOR_INSTALL_SKIP=true)\n",
		)
		return
	}

	By("checking if the Keycloak CRDs are already installed")
	if utils.AreKeycloakCRDsInstalled() {
		_, _ = fmt.Fprintf(GinkgoWriter, "The Keycloak CRDs are already installed. Skipping installation.\n")
		return
	}

	shouldCleanupKeycloakCRDs = true

	version, err := utils.KeycloakOperatorVersion()
	Expect(err).NotTo(HaveOccurred(), "Failed to read the Keycloak Operator version")

	By(fmt.Sprintf("installing the Keycloak CRDs of the Keycloak Operator %s", version))
	Expect(utils.InstallKeycloakCRDs()).To(Succeed(), "Failed to install the Keycloak CRDs")
}

// teardownKeycloakCRDs removes the Keycloak CRDs if setupKeycloakCRDs
// installed them.
func teardownKeycloakCRDs() {
	if !shouldCleanupKeycloakCRDs {
		_, _ = fmt.Fprintf(
			GinkgoWriter, "Skipping Keycloak CRD cleanup (not installed by this suite)\n",
		)
		return
	}

	By("uninstalling the Keycloak CRDs")
	utils.UninstallKeycloakCRDs()
}
