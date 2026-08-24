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

package utils

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const (
	// cnpgCRDDir holds the CloudNativePG CRDs the envtest suites need. This
	// operator writes two of them, Cluster and ScheduledBackup. The third,
	// Backup, is vendored because a ScheduledBackup produces Backup objects
	// and a later controller reads them.
	//
	// The published Go module github.com/cloudnative-pg/api carries the types
	// alone, so the schemas are vendored instead of resolved from the module
	// cache the way the ECK ones are.
	cnpgCRDDir = "internal/testenv/crds/cnpg"
	// barmanCRDDir holds the ObjectStore CRD of the Barman Cloud plugin.
	barmanCRDDir = "internal/testenv/crds/barmancloud"
)

const (
	// EnvCNPGVersion names the CloudNativePG release that the e2e suite
	// installs. The matrix entry of the run sets it, and the suite fails at
	// start when it is unset.
	EnvCNPGVersion = "CNPG_VERSION"
	// EnvBarmanPluginVersion names the Barman Cloud plugin release that the
	// e2e suite installs. The matrix entry of the run sets it, and the suite
	// fails at start when it is unset.
	EnvBarmanPluginVersion = "BARMAN_PLUGIN_VERSION"
	// cnpgManifestURLTmpl addresses the release manifest of CloudNativePG:
	// the release branch of the minor, then the file of the patch.
	cnpgManifestURLTmpl = "https://raw.githubusercontent.com/cloudnative-pg/cloudnative-pg/%s/releases/cnpg-%s.yaml"
	// barmanPluginManifestURLTmpl addresses the release manifest of the
	// Barman Cloud plugin.
	barmanPluginManifestURLTmpl = "https://github.com/cloudnative-pg/plugin-barman-cloud" +
		"/releases/download/v%s/manifest.yaml"
	// cnpgNamespace holds both. The plugin runs beside the CloudNativePG
	// operator, and each manifest names the namespace itself.
	cnpgNamespace = "cnpg-system"
	// cnpgDeployment and barmanPluginDeployment are the two workloads.
	cnpgDeployment         = "cnpg-controller-manager"
	barmanPluginDeployment = "barman-cloud"
	// cnpgClusterCRD and barmanObjectStoreCRD are the kinds this operator
	// writes. It decides at startup whether it watches them, so both have to
	// be served before the manager starts.
	cnpgClusterCRD       = "clusters.postgresql.cnpg.io"
	barmanObjectStoreCRD = "objectstores.barmancloud.cnpg.io"
)

// CNPGCRDPath returns the directory of the vendored CloudNativePG CRDs, for
// envtest to install. The VERSION file beside them names the CloudNativePG
// release they come from.
func CNPGCRDPath() (string, error) {
	return vendoredCRDPath(cnpgCRDDir)
}

// BarmanCRDPath returns the directory of the vendored ObjectStore CRD of the
// Barman Cloud plugin, for envtest to install. The VERSION file beside it
// names the plugin release it comes from.
func BarmanCRDPath() (string, error) {
	return vendoredCRDPath(barmanCRDDir)
}

// CNPGVersion returns the CloudNativePG release that the suite installs, from
// the matrix entry of the run.
func CNPGVersion() string { return os.Getenv(EnvCNPGVersion) }

// BarmanPluginVersion returns the Barman Cloud plugin release that the suite
// installs, from the matrix entry of the run.
func BarmanPluginVersion() string { return os.Getenv(EnvBarmanPluginVersion) }

// IsCNPGInstalled reports whether the cluster serves both CloudNativePG and
// the Barman Cloud plugin: the two CRDs this operator writes, and the two
// Deployments that act on them. The suite installs the pair together, so a
// cluster that carries only one of them is not installed and the suite
// installs both again.
func IsCNPGInstalled() bool {
	crds, err := Run(exec.Command("kubectl", "get", "crds"))
	if err != nil {
		return false
	}

	for _, crd := range []string{cnpgClusterCRD, barmanObjectStoreCRD} {
		if !strings.Contains(crds, crd) {
			return false
		}
	}

	for _, deployment := range []string{cnpgDeployment, barmanPluginDeployment} {
		if !deploymentExists(cnpgNamespace, deployment) {
			return false
		}
	}

	return true
}

// InstallCNPG installs the CloudNativePG operator of CNPGVersion and waits
// until its Deployment is rolled out. Its manifest creates the cnpg-system
// namespace that InstallBarmanPlugin installs into.
//
// It lowers the CPU request of the operator on the way. One kind node carries
// the whole run, and the sum of the requests on it is what decides whether the
// broker pod of an orchestration cluster is scheduled at all. The release asks
// for 100m of a node that has none to spare, and the operator spends most of a
// run idle. The limit of the release stays, so the change moves where the pod
// is scheduled and not what it may use.
func InstallCNPG() error {
	branch, err := cnpgReleaseBranch(CNPGVersion())
	if err != nil {
		return err
	}

	if err := applyManifest(fmt.Sprintf(cnpgManifestURLTmpl, branch, CNPGVersion())); err != nil {
		return err
	}

	if _, err := Run(exec.Command(
		"kubectl", "patch", "deployment", cnpgDeployment, "-n", cnpgNamespace, "--type=json",
		"-p", `[{"op":"add","path":"/spec/template/spec/containers/0/resources/requests/cpu","value":"10m"}]`,
	)); err != nil {
		return err
	}

	return waitForRollout(cnpgDeployment)
}

// InstallBarmanPlugin installs the Barman Cloud plugin of
// BarmanPluginVersion and waits until its Deployment is rolled out. It runs
// after InstallCNPG, whose manifest creates the namespace the two share, and
// after cert-manager, which issues the certificate the plugin serves its
// CNPG-I endpoint with.
func InstallBarmanPlugin() error {
	version := BarmanPluginVersion()
	if version == "" {
		return fmt.Errorf("%s is not set", EnvBarmanPluginVersion)
	}

	if err := applyManifest(fmt.Sprintf(barmanPluginManifestURLTmpl, version)); err != nil {
		return err
	}

	return waitForRollout(barmanPluginDeployment)
}

// UninstallCNPG removes the Barman Cloud plugin and the CloudNativePG
// operator, plugin first: a CloudNativePG cluster reads its ObjectStore
// through it, and nothing of the operator manifest depends on the plugin.
func UninstallCNPG() {
	version := BarmanPluginVersion()
	if version == "" {
		warnError(fmt.Errorf("%s is not set", EnvBarmanPluginVersion))
	} else {
		deleteManifest(fmt.Sprintf(barmanPluginManifestURLTmpl, version))
	}

	branch, err := cnpgReleaseBranch(CNPGVersion())
	if err != nil {
		warnError(err)
		return
	}

	deleteManifest(fmt.Sprintf(cnpgManifestURLTmpl, branch, CNPGVersion()))
}

// cnpgReleaseBranch returns the branch that carries the release manifests of
// the minor that version belongs to. CloudNativePG publishes the manifest of
// every patch on the branch of its minor, and no release asset holds it.
func cnpgReleaseBranch(version string) (string, error) {
	major, rest, found := strings.Cut(version, ".")
	if !found {
		return "", fmt.Errorf("%s=%q names no CloudNativePG release", EnvCNPGVersion, version)
	}

	minor, _, found := strings.Cut(rest, ".")
	if !found {
		return "", fmt.Errorf("%s=%q names no CloudNativePG release", EnvCNPGVersion, version)
	}

	return "release-" + major + "." + minor, nil
}

// applyManifest applies url with server-side apply: both manifests carry a
// CRD larger than the annotation that client-side apply records, and apply
// also completes a partial install where create would stop with
// AlreadyExists.
func applyManifest(url string) error {
	_, err := Run(exec.Command("kubectl", "apply", "--server-side", "-f", url))

	return err
}

// waitForRollout waits until the named Deployment of cnpgNamespace is rolled
// out.
func waitForRollout(deployment string) error {
	_, err := Run(exec.Command(
		"kubectl", "rollout", "status", "deployment/"+deployment,
		"--namespace", cnpgNamespace,
		"--timeout", "5m",
	))

	return err
}

// deleteManifest removes what applyManifest applied and warns instead of
// failing, so one teardown step never stops the next.
func deleteManifest(url string) {
	if _, err := Run(exec.Command("kubectl", "delete", "-f", url, "--ignore-not-found")); err != nil {
		warnError(err)
	}
}

// deploymentExists reports whether namespace holds the named Deployment.
func deploymentExists(namespace, name string) bool {
	out, err := Run(exec.Command(
		"kubectl", "get", "deployment", name,
		"-n", namespace, "--ignore-not-found", "-o", "name",
	))
	if err != nil {
		return false
	}

	return strings.TrimSpace(out) != ""
}
