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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
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
)

// TestE2E runs the e2e test suite to validate the solution in an isolated environment.
// The default setup requires Kind, CertManager, and the ECK operator. The suite
// installs CertManager and ECK when the cluster does not serve them.
//
// To skip CertManager installation, set: CERT_MANAGER_INSTALL_SKIP=true
// To skip ECK installation, set: ECK_INSTALL_SKIP=true
// To install a different ECK release, set: ECK_VERSION=<version>
// To run against another Camunda minor: make test-e2e E2E_CAMUNDA_MINOR=<minor>
// That picks the file of test/e2e/versions/ that the suite reads its image
// versions from.
func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	_, _ = fmt.Fprintf(GinkgoWriter, "Starting camunda-operator e2e test suite\n")
	RunSpecs(t, "e2e suite")
}

// The suite deploys the manager once, after ECK. The ECK CRDs must be present
// before the manager starts, because the ElasticsearchCluster controller
// watches the ECK Elasticsearch kind.
var _ = BeforeSuite(func() {
	By("checking the image versions of the run")
	for _, name := range utils.VersionEnv {
		Expect(os.Getenv(name)).NotTo(
			BeEmpty(),
			"%s is not set. Run the suite through make test-e2e, which exports it from test/e2e/versions/<minor>.env",
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
	setupECK()
	deployManager()
	setupMinIO()
})

var _ = AfterSuite(func() {
	teardownMinIO()
	undeployManager()
	teardownECK()
	teardownCertManager()
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

// setupMinIO deploys the object store of the backup flows and creates the
// bucket contract that they write through. It runs after the manager, so the
// CRD of the contract is installed.
func setupMinIO() {
	By("creating the MinIO namespace")
	_, err := utils.Kubectl("create", "ns", minioNamespace)
	Expect(err).NotTo(HaveOccurred(), "Failed to create the MinIO namespace")

	By("deploying MinIO and creating the bucket")
	Expect(utils.InstallMinIO(minioNamespace)).To(Succeed(), "Failed to install MinIO")

	By("creating the ObjectStorageConfig of the backup flows")
	Expect(apply(backupObjectStorage())).To(Succeed(), "Failed to create the ObjectStorageConfig")
	Eventually(func(g Gomega) {
		expectReady(g, oscResource, backupStorage, "", v1.ReasonHealthy)
	}, 2*time.Minute).Should(Succeed())
}

// teardownMinIO removes the bucket contract and the MinIO namespace. It runs
// before the manager is undeployed, so the contract is removed while its CRD
// is still served.
func teardownMinIO() {
	By("removing the ObjectStorageConfig and the MinIO namespace")
	_, _ = utils.Kubectl("delete", oscResource, backupStorage, "--ignore-not-found")
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
