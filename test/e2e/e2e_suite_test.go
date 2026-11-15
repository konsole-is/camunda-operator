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
	"context"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	"github.com/onsi/ginkgo/v2/types"
	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/require"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
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
	setupRustFS()
})

// teardownTimeout bounds the AfterSuite: finalizeTimeout for the custom
// resources of the operator, and less than a minute for the rest. Without a
// bound, a step that hangs waits until the timeout of go test.
const teardownTimeout = 10 * time.Minute

// finalizeTimeout bounds the wait of removeOperatorResources.
const finalizeTimeout = 5 * time.Minute

// operatorGroups are the API groups of the CRDs that the manager serves.
var operatorGroups = []string{
	v1.GroupVersion.Group,
	fleetv1alpha1.GroupVersion.Group,
	pluginsv1alpha1.GroupVersion.Group,
}

var _ = AfterSuite(func(ctx SpecContext) {
	stuck, err := removeOperatorResources(ctx)
	teardownRustFS()
	undeployManager(ctx)
	teardownKeycloakCRDs()
	teardownECK()
	teardownCNPG()
	teardownCertManager()

	Expect(err).NotTo(HaveOccurred())
	Expect(stuck).To(
		BeEmpty(),
		"custom resources of the operator did not go while the manager ran:\n%s",
		utils.FormatCustomResources(stuck),
	)
}, NodeTimeout(teardownTimeout))

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

// removeOperatorResources deletes every custom resource of operatorGroups and
// waits until they are gone, while the manager runs and releases their
// finalizers. A spec that deletes its resources without a wait, or that fails
// before its cleanup, leaves them to this step. When finalizeTimeout passes or
// ctx ends first, it prints the resources that remain, removes their
// finalizers so that the deletion of the CRDs can end, and returns them. It
// returns an error when the last list of the resources failed.
func removeOperatorResources(ctx context.Context) ([]utils.CustomResource, error) {
	By("removing the custom resources of the operator while the manager runs")
	start := time.Now()
	waitCtx, cancel := context.WithTimeout(ctx, finalizeTimeout)
	defer cancel()

	var remaining []utils.CustomResource
	var err error
	for waitCtx.Err() == nil {
		var crds []utils.CRD
		var resources []utils.CustomResource
		crds, resources, err = listOperatorResources()
		if err == nil && len(resources) == 0 {
			_, _ = fmt.Fprintf(
				GinkgoWriter,
				"The custom resources of the operator are gone after %s\n",
				time.Since(start).Round(time.Second),
			)
			return nil, nil
		}
		if err == nil {
			remaining = resources
		}

		// A controller can still create a resource, such as the backup of a
		// BackupSchedule, after an earlier round deleted everything.
		if slices.ContainsFunc(remaining, func(r utils.CustomResource) bool { return r.DeletionTimestamp == nil }) {
			_, _ = utils.RunContext(
				waitCtx,
				"kubectl",
				"delete",
				utils.CRDNames(crds),
				"--all",
				"--all-namespaces",
				"--ignore-not-found",
				"--wait=false",
			)
		}

		select {
		case <-waitCtx.Done():
		case <-time.After(5 * time.Second):
		}
	}

	_, _ = fmt.Fprintf(
		GinkgoWriter,
		"The custom resources of the operator did not go within %s:\n%s",
		time.Since(start).Round(time.Second),
		utils.FormatCustomResources(remaining),
	)
	for _, r := range remaining {
		if err := utils.RemoveFinalizers(r); err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "warning: removing the finalizers of %s %s: %v\n", r.Kind, r.Name, err)
		}
	}

	if err != nil {
		return remaining, fmt.Errorf("listing the custom resources of the operator: %w", err)
	}

	return remaining, nil
}

// listOperatorResources returns the CRDs of operatorGroups that the cluster
// serves now, and every resource of them.
func listOperatorResources() ([]utils.CRD, []utils.CustomResource, error) {
	crds, err := utils.CRDsOfGroups(operatorGroups...)
	if err != nil {
		return nil, nil, err
	}

	resources, err := utils.ListCustomResources(crds)

	return crds, resources, err
}

// undeployManager removes the controller-manager, the CRDs, and the manager
// namespace. A step that still runs when ctx ends is killed, and the custom
// resources of the operator that remain then are printed.
func undeployManager(ctx context.Context) {
	By("undeploying the controller-manager")
	_, _ = utils.RunContext(ctx, "make", "undeploy")

	By("uninstalling CRDs")
	_, _ = utils.RunContext(ctx, "make", "uninstall")

	By("removing manager namespace")
	_, _ = utils.RunContext(ctx, "kubectl", "delete", "ns", namespace)

	if ctx.Err() == nil {
		return
	}

	_, resources, err := listOperatorResources()
	if err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "warning: listing the custom resources of the operator: %v\n", err)
		return
	}
	_, _ = fmt.Fprintf(
		GinkgoWriter,
		"The teardown timed out with these custom resources of the operator:\n%s",
		utils.FormatCustomResources(resources),
	)
}

// setupRustFS deploys the object store of the backup flows. Each flow creates
// the bucket contract in its own namespace with createBackupStorage.
func setupRustFS() {
	By("creating the RustFS namespace")
	_, err := utils.Kubectl("create", "ns", rustfsNamespace)
	Expect(err).NotTo(HaveOccurred(), "Failed to create the RustFS namespace")

	By("deploying RustFS and creating the bucket")
	Expect(utils.InstallRustFS(rustfsNamespace)).To(Succeed(), "Failed to install RustFS")
}

// teardownRustFS removes the RustFS namespace.
func teardownRustFS() {
	By("removing the RustFS namespace")
	_, _ = utils.Kubectl("delete", "ns", rustfsNamespace, "--wait=false")
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
