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
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	// The namespaces that the manifests of config/example name. Every spec of
	// this flow takes them for the time it runs and removes them afterwards,
	// because the inventories share the names.
	exNamespace           = "my-cluster-ns"
	exManagementNamespace = "my-management-ns"

	// exPlatformConfig is the cluster-scoped CamundaPlatformConfig that each
	// inventory writes under this one name with contents of its own. It
	// outlives a namespace, so the teardown deletes it as well.
	exPlatformConfig = "my-platform-config"

	// The resources of the inventories that this flow waits on.
	exCluster        = "my-cluster"
	exDatabaseServer = "my-db"
	exDatabase       = "my-camunda-db"

	// The directories of config/example, under the module root.
	exPresets                = "config/example/presets"
	exElasticsearchInventory = "config/example/camunda-cluster/elasticsearch"
	exRDBMSInventory         = "config/example/camunda-cluster/rdbms"
	exKeycloakInventory      = "config/example/camunda-management-cluster/keycloak"
	exOIDCInventory          = "config/example/camunda-management-cluster/oidc"

	// exBrokenInventory holds the elasticsearch inventory with one reference
	// broken.
	exBrokenInventory = "test/e2e/testdata/broken-example"

	// exParkTimeout bounds the wait for a resource that cannot converge, so
	// it reports why instead. A pre-check failure needs one reconcile, not an
	// image pull.
	exParkTimeout = 3 * time.Minute
	// exDatabaseTimeout bounds the wait for a logical database on a server
	// that already runs.
	exDatabaseTimeout = 5 * time.Minute
	// exTeardownTimeout bounds the removal of one inventory, the volumes of
	// its storage backend included.
	exTeardownTimeout = 10 * time.Minute
)

// The manifests of a management inventory up to its CamundaManagementCluster,
// in the order its README applies them. This flow stops there. The plane needs
// a license key, an SMTP host, and identity provider URLs that CI cannot
// invent. The two management flows of this suite prove that a plane converges
// against real ones.
var (
	exKeycloakStorageFiles = []string{
		"01-namespaces.yaml",
		"02-secrets.yaml",
		"03-database-server.yaml",
		"04-databases.yaml",
		"05-platform-config.yaml",
	}
	exOIDCStorageFiles = []string{
		"01-namespace.yaml",
		"02-secrets.yaml",
		"03-database-server.yaml",
		"04-databases.yaml",
		"05-platform-config.yaml",
	}
)

// The envtest guard in internal/controller/example_schema_test.go proves that
// every inventory is schema-valid against the real CRDs. It proves nothing
// about convergence. A storageRef that names a contract nobody publishes, a
// preset no node can schedule, and an image tag that does not pull all pass a
// dry run and fail on a cluster. This flow applies the inventories to a kind
// cluster and waits for the top resource of each chain.
var _ = Describe("Example inventories", Ordered, Label(utils.LabelExample), func() {
	var (
		// root is the module root, which the paths above resolve against. The
		// suite runs from test/e2e.
		root string
		// applied holds the namespaces of the inventory of the running spec,
		// for the dump and for the teardown.
		applied []string
	)

	BeforeAll(func() {
		var err error
		root, err = utils.ModuleRoot()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		for _, ns := range applied {
			dumpDiagnostics(ns)
		}

		removeInventory(applied)
		applied = nil
	})

	It("refuses an inventory whose storageRef names no contract", func() {
		applied = []string{exNamespace}

		By("applying the elasticsearch inventory with a broken storageRef")
		_, err := utils.Kubectl("apply", "-k", filepath.Join(root, exBrokenInventory))
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the CamundaCluster to report the dangling reference")
		Eventually(func(g Gomega) {
			expectConditionFalse(
				g, ccResource, exCluster, exNamespace, v1.ConditionReady, v1.ReasonInvalidReference,
			)
		}, exParkTimeout, 5*time.Second).Should(Succeed())
	})

	// The bar is the CamundaCluster alone. The ElasticsearchCluster of this
	// inventory holds Ready=False for as long as the cluster exports to it,
	// because Camunda creates every index with one replica and one
	// Elasticsearch node cannot assign it. Issue #362 carries that. Wait for
	// the ElasticsearchCluster again once it is answered.
	It("stands the camunda-cluster/elasticsearch inventory up", func() {
		applied = []string{exNamespace}

		By("applying the inventory")
		_, err := utils.Kubectl("apply", "-k", filepath.Join(root, exElasticsearchInventory))
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the CamundaCluster")
		Eventually(func(g Gomega) {
			expectReady(g, ccResource, exCluster, exNamespace, v1.ReasonHealthy)
		}, ccReadyTimeout, 5*time.Second).Should(Succeed())
	})

	It("stands the camunda-cluster/rdbms inventory up", func() {
		applied = []string{exNamespace}

		By("applying the inventory")
		_, err := utils.Kubectl("apply", "-k", filepath.Join(root, exRDBMSInventory))
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the DatabaseServer")
		Eventually(func(g Gomega) {
			expectReady(g, dsResource, exDatabaseServer, exNamespace, v1.ReasonHealthy)
		}, dsReadyTimeout, 5*time.Second).Should(Succeed())

		By("waiting for the Database")
		Eventually(func(g Gomega) {
			expectReady(g, dbResource, exDatabase, exNamespace, v1.ReasonHealthy)
		}, exDatabaseTimeout, 5*time.Second).Should(Succeed())

		By("waiting for the CamundaCluster")
		Eventually(func(g Gomega) {
			expectReady(g, ccResource, exCluster, exNamespace, v1.ReasonHealthy)
		}, ccReadyTimeout, 5*time.Second).Should(Succeed())
	})

	It("stands the storage of the camunda-management-cluster/keycloak inventory up", func() {
		applied = []string{exManagementNamespace, exNamespace}

		applyInventoryStorage(root, exKeycloakInventory, exKeycloakStorageFiles)
		expectManagementStorage("my-keycloak-db", "my-identity-db", "my-web-modeler-db")
	})

	It("stands the storage of the camunda-management-cluster/oidc inventory up", func() {
		applied = []string{exManagementNamespace}

		applyInventoryStorage(root, exOIDCInventory, exOIDCStorageFiles)
		expectManagementStorage("my-identity-db", "my-web-modeler-db")
	})
})

// removeInventory deletes the namespaces of an inventory and the
// CamundaPlatformConfig that every inventory writes under one name, then waits
// until all of them are gone. The next spec creates its resources under the
// same names, and an object that still terminates accepts no create.
//
// The presets and the release stay. They are cluster scoped, every inventory
// agrees on them, and a user applies them once.
func removeInventory(namespaces []string) {
	GinkgoHelper()

	for _, ns := range namespaces {
		_, _ = utils.Kubectl("delete", "ns", ns, "--ignore-not-found", "--wait=false")
	}

	_, _ = utils.Kubectl(
		"delete", ccPlatformResource, exPlatformConfig, "--ignore-not-found", "--wait=false",
	)

	// A namespace that never goes takes every later spec of the container with
	// it, and the report names the namespace and nothing else. The failure is
	// intercepted so that the dump reads the objects that hold it while they
	// are still there.
	failure := InterceptGomegaFailure(func() {
		Eventually(func(g Gomega) {
			expectGone(g, ccPlatformResource, exPlatformConfig, "")
			for _, ns := range namespaces {
				expectGone(g, "namespaces", ns, "")
			}
		}, exTeardownTimeout, 10*time.Second).Should(Succeed())
	})
	if failure != nil {
		dumpTeardown(namespaces)
		Fail(failure.Error())
	}
}

// dumpTeardown writes what holds a teardown that did not finish: each
// namespace with its conditions, which name the API groups that still have
// content, the custom resources left in it, and the manager logs.
//
// dumpDiagnostics answers this question for a spec that failed. It reads the
// report of the spec and writes nothing while the failure is intercepted, so
// the teardown carries a dump of its own.
func dumpTeardown(namespaces []string) {
	for _, ns := range namespaces {
		out, err := utils.Kubectl("get", "ns", ns, "-o", "yaml")
		if err != nil {
			_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get namespace %s: %s\n", ns, err)
		} else {
			_, _ = fmt.Fprintf(GinkgoWriter, "namespace %s:\n%s\n", ns, out)
		}

		dumpCustomResources(ns)
	}

	out, err := utils.Kubectl(
		"logs", "-l", "control-plane=controller-manager", "-n", namespace, "--tail=-1",
	)
	if err != nil {
		_, _ = fmt.Fprintf(GinkgoWriter, "Failed to get the controller-manager logs: %s\n", err)
		return
	}
	_, _ = fmt.Fprintf(GinkgoWriter, "controller-manager logs:\n%s\n", out)
}

// applyInventoryStorage applies the shared presets and then the named files of
// the inventory. The kustomization of a management inventory covers its plane
// as well, so this flow applies the files instead.
func applyInventoryStorage(root, inventory string, files []string) {
	GinkgoHelper()

	By("applying the shared presets")
	_, err := utils.Kubectl("apply", "-k", filepath.Join(root, exPresets))
	Expect(err).NotTo(HaveOccurred())

	By("applying the inventory up to its management cluster")
	args := []string{"apply"}
	for _, file := range files {
		args = append(args, "-f", filepath.Join(root, inventory, file))
	}
	_, err = utils.Kubectl(args...)
	Expect(err).NotTo(HaveOccurred())
}

// expectManagementStorage waits for the PostgreSQL server of a management
// inventory and for each of its logical databases.
func expectManagementStorage(databases ...string) {
	GinkgoHelper()

	By("waiting for the DatabaseServer")
	Eventually(func(g Gomega) {
		expectReady(g, dsResource, exDatabaseServer, exManagementNamespace, v1.ReasonHealthy)
	}, dsReadyTimeout, 5*time.Second).Should(Succeed())

	for _, database := range databases {
		By("waiting for the Database " + database)
		Eventually(func(g Gomega) {
			expectReady(g, dbResource, database, exManagementNamespace, v1.ReasonHealthy)
		}, exDatabaseTimeout, 5*time.Second).Should(Succeed())
	}
}
