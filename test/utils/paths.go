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
	"path/filepath"
)

const (
	// envtestBinaryDir holds the control plane binaries that
	// 'make setup-envtest' downloads, one directory per Kubernetes version.
	envtestBinaryDir = "bin/k8s"
	// envKubebuilderAssets names the control plane binaries directly.
	// 'make setup-envtest' writes it and envtest itself reads it.
	envKubebuilderAssets = "KUBEBUILDER_ASSETS"
)

// ModuleRoot walks up from the working directory to the directory that holds
// go.mod, so a package at any depth resolves the same paths.
//
// GetProjectDir answers the same question by cutting "/test/e2e" off the
// working directory, which only holds for a caller in that one package.
func ModuleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolving working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod found in or above %q", dir)
		}
		dir = parent
	}
}

// EnvtestBinaryDir returns the control plane binaries a suite boots against.
//
// KUBEBUILDER_ASSETS wins when it is set, because that is the version
// 'make test' selected, and bin/k8s can hold an older one beside it. Without
// it the first versioned directory under bin/k8s answers, so a suite runs
// from an IDE that sets no environment.
//
// An empty string means neither is available. envtest then reports the
// missing binaries itself.
func EnvtestBinaryDir() string {
	if assets := os.Getenv(envKubebuilderAssets); assets != "" {
		return assets
	}

	root, err := ModuleRoot()
	if err != nil {
		return ""
	}

	base := filepath.Join(root, envtestBinaryDir)

	entries, err := os.ReadDir(base)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(base, entry.Name())
		}
	}

	return ""
}

// vendoredCRDPath resolves a CRD directory of this repository against the
// module root and fails when it is missing, so a suite that would boot a
// control plane without the schema it needs stops with the path instead.
func vendoredCRDPath(dir string) (string, error) {
	root, err := ModuleRoot()
	if err != nil {
		return "", err
	}

	path := filepath.Join(root, dir)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("resolving the vendored CRD directory %q: %w", path, err)
	}

	return path, nil
}
