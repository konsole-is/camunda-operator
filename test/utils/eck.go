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
	// defaultECKVersion is the ECK operator release that the e2e suite
	// installs when ECK_VERSION is unset. The Makefile pins the same value
	// and passes it through the environment.
	defaultECKVersion  = "3.5.0"
	eckCRDsURLTmpl     = "https://download.elastic.co/downloads/eck/%s/crds.yaml"
	eckOperatorURLTmpl = "https://download.elastic.co/downloads/eck/%s/operator.yaml"
	eckNamespace       = "elastic-system"
)

// ECKVersion returns the ECK operator release that the suite installs: the
// value of ECK_VERSION, or the pinned default.
func ECKVersion() string {
	if v, ok := os.LookupEnv("ECK_VERSION"); ok && v != "" {
		return v
	}
	return defaultECKVersion
}

// IsECKInstalled reports whether the cluster serves the ECK Elasticsearch
// CRD and runs the ECK operator StatefulSet. A cluster with the CRDs but
// without the operator, for example after a partial uninstall, is not
// installed: the suite then installs ECK again.
func IsECKInstalled() bool {
	crds, err := Run(exec.Command("kubectl", "get", "crds"))
	if err != nil || !strings.Contains(crds, "elasticsearches.elasticsearch.k8s.elastic.co") {
		return false
	}

	operator, err := Run(exec.Command(
		"kubectl", "get", "statefulset", "elastic-operator",
		"-n", eckNamespace, "--ignore-not-found", "-o", "name",
	))
	if err != nil {
		return false
	}

	return strings.TrimSpace(operator) != ""
}

// InstallECK installs the ECK CRDs and operator of ECKVersion and waits until
// the operator StatefulSet is rolled out.
func InstallECK() error {
	for _, url := range []string{
		fmt.Sprintf(eckCRDsURLTmpl, ECKVersion()),
		fmt.Sprintf(eckOperatorURLTmpl, ECKVersion()),
	} {
		// create, not apply: the CRD manifest exceeds the annotation size that
		// client-side apply records.
		cmd := exec.Command("kubectl", "create", "-f", url)
		if _, err := Run(cmd); err != nil {
			return err
		}
	}

	cmd := exec.Command(
		"kubectl", "rollout", "status", "statefulset/elastic-operator",
		"--namespace", eckNamespace,
		"--timeout", "5m",
	)
	_, err := Run(cmd)
	return err
}

// UninstallECK removes the ECK operator and its CRDs.
func UninstallECK() {
	for _, url := range []string{
		fmt.Sprintf(eckOperatorURLTmpl, ECKVersion()),
		fmt.Sprintf(eckCRDsURLTmpl, ECKVersion()),
	} {
		cmd := exec.Command("kubectl", "delete", "-f", url, "--ignore-not-found")
		if _, err := Run(cmd); err != nil {
			warnError(err)
		}
	}
}
