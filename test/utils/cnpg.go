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
	// cnpgCRDDir holds the CloudNativePG CRDs that this operator writes:
	// Cluster, ScheduledBackup, and Backup. The published Go module
	// github.com/cloudnative-pg/api carries the types alone, so the schemas
	// are vendored instead of resolved from the module cache the way the ECK
	// ones are.
	cnpgCRDDir = "internal/testenv/crds/cnpg"
	// barmanCRDDir holds the ObjectStore CRD of the Barman Cloud plugin.
	barmanCRDDir = "internal/testenv/crds/barmancloud"
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

// vendoredCRDPath resolves dir against the root of this module. It walks up
// from the working directory rather than trimming a known suffix, so a
// package at any depth resolves the same directory.
func vendoredCRDPath(dir string) (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}

	path := filepath.Join(root, dir)
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("resolving the vendored CRD directory %q: %w", path, err)
	}

	return path, nil
}

// moduleRoot walks up from the working directory to the directory that holds
// go.mod.
func moduleRoot() (string, error) {
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
