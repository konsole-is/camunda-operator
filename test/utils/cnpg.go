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

	. "github.com/onsi/ginkgo/v2" // nolint:revive,staticcheck
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
	// cnpgDeploymentSelector and barmanPluginSelector select the pods of
	// cnpgDeployment and barmanPluginDeployment. dumpInstallDiagnostics
	// describes the pods these match when a wait on the two fails.
	cnpgDeploymentSelector = "app.kubernetes.io/name=cloudnative-pg"
	barmanPluginSelector   = "app=barman-cloud"
	// barmanClientCertificate and barmanServerCertificate are the two
	// cert-manager Certificates the plugin manifest issues from a
	// self-signed Issuer in the same apply, and that its Deployment mounts
	// as Secrets.
	barmanClientCertificate = "barman-cloud-client"
	barmanServerCertificate = "barman-cloud-server"
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

// InstallCNPG installs the CloudNativePG operator of CNPGVersion, lowers its
// CPU request, and waits until its Deployment is rolled out. Its manifest
// creates the cnpg-system namespace that InstallBarmanPlugin installs into.
func InstallCNPG() error {
	branch, err := cnpgReleaseBranch(CNPGVersion())
	if err != nil {
		return err
	}

	if err := applyManifest(fmt.Sprintf(cnpgManifestURLTmpl, branch, CNPGVersion())); err != nil {
		return err
	}

	// One kind node carries the whole run, and the sum of the requests on it
	// decides whether the broker pod of an orchestration cluster is scheduled
	// at all. The release asks for 100m and the operator idles for most of a
	// run. The limit of the release stays, so the pod keeps what it can use.
	if _, err := Run(exec.Command(
		"kubectl", "patch", "deployment", cnpgDeployment, "-n", cnpgNamespace, "--type=json",
		"-p", `[{"op":"add","path":"/spec/template/spec/containers/0/resources/requests/cpu","value":"10m"}]`,
	)); err != nil {
		return err
	}

	return waitForRollout(cnpgDeployment, cnpgDeploymentSelector)
}

// InstallBarmanPlugin installs the Barman Cloud plugin of
// BarmanPluginVersion and waits until its Deployment is rolled out. It runs
// after InstallCNPG, whose manifest creates the namespace the two share, and
// after cert-manager, which issues the two certificates the plugin serves
// its CNPG-I endpoint with.
func InstallBarmanPlugin() error {
	version := BarmanPluginVersion()
	if version == "" {
		return fmt.Errorf("%s is not set", EnvBarmanPluginVersion)
	}

	if err := applyManifest(fmt.Sprintf(barmanPluginManifestURLTmpl, version)); err != nil {
		return err
	}

	// The plugin Deployment mounts these as Secrets that only exist once
	// cert-manager issues them. Waiting on the Certificates first turns a
	// stuck issuance into its own clear failure instead of the "0 of 1
	// replicas available" a Deployment rollout timeout reports for it.
	certs := []string{barmanClientCertificate, barmanServerCertificate}
	if err := waitForCertificates(barmanPluginSelector, certs...); err != nil {
		return err
	}

	return waitForRollout(barmanPluginDeployment, barmanPluginSelector)
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

// applyManifest applies url with server-side apply. Both manifests carry a
// CRD larger than the annotation that client-side apply records. Apply also
// completes a partial install, where create stops with AlreadyExists.
func applyManifest(url string) error {
	_, err := Run(exec.Command("kubectl", "apply", "--server-side", "-f", url))

	return err
}

// waitForRollout waits until the named Deployment of cnpgNamespace is rolled
// out. On a failure it dumps the diagnostics of the pods that selector
// matches before it returns the error.
func waitForRollout(deployment, selector string) error {
	_, err := Run(exec.Command(
		"kubectl", "rollout", "status", "deployment/"+deployment,
		"--namespace", cnpgNamespace,
		"--timeout", "5m",
	))
	if err != nil {
		dumpInstallDiagnostics(selector)
	}

	return err
}

// waitForCertificates waits until every named cert-manager Certificate of
// cnpgNamespace carries condition=Ready. On a failure it dumps the
// diagnostics of the pods that selector matches before it returns the error.
func waitForCertificates(selector string, names ...string) error {
	args := make([]string, 0, 7+len(names))
	args = append(args, "wait", "--for", "condition=Ready", "--namespace", cnpgNamespace, "--timeout", "2m")
	for _, name := range names {
		args = append(args, "certificate/"+name)
	}

	if _, err := Run(exec.Command("kubectl", args...)); err != nil {
		dumpInstallDiagnostics(selector)
		return err
	}

	return nil
}

// dumpInstallDiagnostics writes the pod descriptions of selector and the
// events of cnpgNamespace to the Ginkgo writer, so a failed install explains
// itself instead of leaving only the error message.
func dumpInstallDiagnostics(selector string) {
	pods, err := Run(exec.Command("kubectl", "describe", "pod", "-n", cnpgNamespace, "-l", selector))
	if err != nil {
		pods = err.Error()
	}
	_, _ = fmt.Fprintf(GinkgoWriter, "pods matching %q in %s:\n%s\n", selector, cnpgNamespace, pods)

	events, err := Run(exec.Command("kubectl", "get", "events", "-n", cnpgNamespace, "--sort-by=.lastTimestamp"))
	if err != nil {
		events = err.Error()
	}
	_, _ = fmt.Fprintf(GinkgoWriter, "events in %s:\n%s\n", cnpgNamespace, events)
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
