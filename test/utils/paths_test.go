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
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestEnvtestBinaryDirPrefersKubebuilderAssets proves that the version
// 'make test' selected wins over whatever bin/k8s happens to hold. Without
// this a stale directory quietly downgrades the control plane of every suite.
func TestEnvtestBinaryDirPrefersKubebuilderAssets(t *testing.T) {
	t.Setenv(envKubebuilderAssets, "/selected/by/make/test")

	assert.Equal(t, "/selected/by/make/test", EnvtestBinaryDir())
}

// TestEnvtestBinaryDirFallsBackToTheModuleDirectory proves that a suite run
// from an IDE, which sets no environment, still finds the binaries.
func TestEnvtestBinaryDirFallsBackToTheModuleDirectory(t *testing.T) {
	t.Setenv(envKubebuilderAssets, "")

	dir := EnvtestBinaryDir()
	if dir == "" {
		t.Skip("bin/k8s is not populated; run make setup-envtest")
	}

	assert.Contains(t, dir, envtestBinaryDir)
}
