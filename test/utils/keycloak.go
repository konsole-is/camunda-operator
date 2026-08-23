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
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2" // nolint:revive,staticcheck
)

const (
	// keycloakCRDVersionFile holds the Keycloak Operator release that
	// internal/testenv vendors the Keycloak CRD from. The e2e suite installs
	// that release when the matrix names no Keycloak, so the envtest suites
	// and the kind suite agree on the schema.
	keycloakCRDVersionFile = "internal/testenv/crds/keycloak/VERSION"
	// envKeycloakVersion is the Keycloak release that the management plane
	// flow runs, from the matrix entry of the run. The suite installs the
	// Keycloak Operator of the same release: the operator supports the
	// Keycloak it was released with, and no other
	// (https://www.keycloak.org/operator/customizing-keycloak).
	envKeycloakVersion = "KEYCLOAK_VERSION"
	// envKeycloakOperatorVersion names a Keycloak Operator release of its
	// own. It wins over the two sources above.
	envKeycloakOperatorVersion = "KEYCLOAK_OPERATOR_VERSION"
	// keycloakResourceURLTmpl addresses one file of the kubernetes directory
	// of keycloak/keycloak-k8s-resources at a release tag.
	keycloakResourceURLTmpl = "https://raw.githubusercontent.com/keycloak/keycloak-k8s-resources/%s/kubernetes/%s"
	// keycloakOperatorManifest holds the ServiceAccount, the roles, the
	// Service, and the Deployment of the Keycloak Operator.
	keycloakOperatorManifest = "kubernetes.yml"
	// keycloakOperatorDeployment is the Deployment of that manifest.
	keycloakOperatorDeployment = "keycloak-operator"
)

// keycloakOperatorCRDs are the custom resource definitions of the Keycloak
// Operator. The operator registers a controller for each kind it ships (the
// QUARKUS_OPERATOR_SDK_CONTROLLERS_* variables of its Deployment name them),
// so every one the release publishes is installed even though this operator
// writes only Keycloak resources. The two client kinds arrived in 26.7.0, so
// the installer skips a file that the release does not publish.
var keycloakOperatorCRDs = []string{
	"keycloaks.k8s.keycloak.org-v1.yml",
	"keycloakrealmimports.k8s.keycloak.org-v1.yml",
	"keycloakoidcclients.k8s.keycloak.org-v1.yml",
	"keycloaksamlclients.k8s.keycloak.org-v1.yml",
}

// KeycloakOperatorVersion returns the Keycloak Operator release that the
// suite installs: the value of KEYCLOAK_OPERATOR_VERSION, else the Keycloak
// release of the matrix entry (KEYCLOAK_VERSION), else the version of the CRD
// that internal/testenv vendors.
func KeycloakOperatorVersion() (string, error) {
	for _, name := range []string{envKeycloakOperatorVersion, envKeycloakVersion} {
		if v, ok := os.LookupEnv(name); ok && v != "" {
			return v, nil
		}
	}

	dir, err := GetProjectDir()
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, keycloakCRDVersionFile)
	content, err := os.ReadFile(path) // nolint:gosec // a path of this repository
	if err != nil {
		return "", fmt.Errorf("reading the vendored Keycloak CRD version %q: %w", path, err)
	}

	version := strings.TrimSpace(string(content))
	if version == "" {
		return "", fmt.Errorf("the vendored Keycloak CRD version %q is empty", path)
	}

	return version, nil
}

// IsKeycloakOperatorInstalled reports whether the cluster serves the Keycloak
// CRD and runs the Keycloak Operator Deployment in namespace. A cluster with
// the CRDs but without the operator, for example after a partial uninstall,
// is not installed: the suite then installs the Keycloak Operator again.
func IsKeycloakOperatorInstalled(namespace string) bool {
	crds, err := Run(exec.Command("kubectl", "get", "crds"))
	if err != nil || !strings.Contains(crds, "keycloaks.k8s.keycloak.org") {
		return false
	}

	operator, err := Run(exec.Command(
		"kubectl", "get", "deployment", keycloakOperatorDeployment,
		"-n", namespace, "--ignore-not-found", "-o", "name",
	))
	if err != nil {
		return false
	}

	return strings.TrimSpace(operator) != ""
}

// InstallKeycloakOperator installs the Keycloak CRDs and runs the Keycloak
// Operator in namespace, then waits until its Deployment is rolled out.
//
// The operator watches the namespace it runs in and no other
// (JOSDK_WATCH_CURRENT in its Deployment), so it goes next to the Keycloak
// resources it manages rather than into an operator namespace of its own.
func InstallKeycloakOperator(namespace string) error {
	version, err := KeycloakOperatorVersion()
	if err != nil {
		return err
	}

	for _, crd := range keycloakOperatorCRDs {
		url := fmt.Sprintf(keycloakResourceURLTmpl, version, crd)
		published, err := urlExists(url)
		if err != nil {
			return err
		}
		if !published {
			_, _ = fmt.Fprintf(GinkgoWriter, "The Keycloak Operator %s publishes no %s, skipping it\n", version, crd)
			continue
		}

		// Server-side apply: the CRD manifests exceed the annotation size
		// that client-side apply records, and apply also completes a partial
		// install where create would stop with AlreadyExists.
		cmd := exec.Command("kubectl", "apply", "--server-side", "-f", url)
		if _, err := Run(cmd); err != nil {
			return err
		}
	}

	cmd := exec.Command(
		"kubectl", "apply", "--server-side", "-n", namespace,
		"-f", fmt.Sprintf(keycloakResourceURLTmpl, version, keycloakOperatorManifest),
	)
	if _, err := Run(cmd); err != nil {
		return err
	}

	cmd = exec.Command(
		"kubectl", "rollout", "status", "deployment/"+keycloakOperatorDeployment,
		"--namespace", namespace, "--timeout", "5m",
	)
	_, err = Run(cmd)

	return err
}

// UninstallKeycloakOperator removes the Keycloak Operator from namespace and
// its CRDs.
func UninstallKeycloakOperator(namespace string) {
	version, err := KeycloakOperatorVersion()
	if err != nil {
		warnError(err)
		return
	}

	cmd := exec.Command(
		"kubectl", "delete", "-n", namespace, "--ignore-not-found",
		"-f", fmt.Sprintf(keycloakResourceURLTmpl, version, keycloakOperatorManifest),
	)
	if _, err := Run(cmd); err != nil {
		warnError(err)
	}

	for _, crd := range keycloakOperatorCRDs {
		url := fmt.Sprintf(keycloakResourceURLTmpl, version, crd)
		published, err := urlExists(url)
		if err != nil {
			warnError(err)
			continue
		}
		if !published {
			continue
		}

		cmd := exec.Command("kubectl", "delete", "--ignore-not-found", "-f", url)
		if _, err := Run(cmd); err != nil {
			warnError(err)
		}
	}
}

// urlProbeTimeout bounds one probe of a release file, so a stalled connection
// fails the install instead of holding the suite.
const urlProbeTimeout = 30 * time.Second

// urlExists reports whether a HEAD of url answers 200. A 404 is false; any
// other answer or a transport error is an error, so a flaky network never
// reads as a release that publishes no such file.
func urlExists(url string) (bool, error) {
	client := &http.Client{Timeout: urlProbeTimeout}
	resp, err := client.Head(url) // nolint:gosec // a URL of keycloak-k8s-resources at a release tag
	if err != nil {
		return false, fmt.Errorf("probing %q: %w", url, err)
	}
	_ = resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	default:
		return false, fmt.Errorf("probing %q: HTTP %d", url, resp.StatusCode)
	}
}
