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

package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitRepositoryCreatesABareRepositoryThatTakesPushes(t *testing.T) {
	t.Parallel()
	root := t.TempDir()

	require.NoError(t, initRepository(t.Context(), root, "fleet.git", "main"))

	repository := filepath.Join(root, "fleet.git")
	assert.Equal(t, "true", gitOut(t, repository, "rev-parse", "--is-bare-repository"))
	assert.Equal(t, "refs/heads/main", gitOut(t, repository, "symbolic-ref", "HEAD"))
	assert.Equal(t, "true", gitOut(t, repository, "config", "http.receivepack"))
}

func TestInitRepositoryKeepsAnExistingRepository(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, initRepository(t.Context(), root, "fleet.git", "main"))
	url := serve(t, root)
	pushed := commitAndPush(t, url, "fleet/ns/configmaps/probe.yaml", "kind: ConfigMap\n")

	require.NoError(t, initRepository(t.Context(), root, "fleet.git", "main"))

	assert.Equal(t, pushed, gitOut(t, filepath.Join(root, "fleet.git"), "rev-parse", "main"))
}

func TestHandlerServesPushAndClone(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, initRepository(t.Context(), root, "fleet.git", "main"))
	url := serve(t, root)
	content := "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: probe\n"

	pushed := commitAndPush(t, url, "fleet/ns/configmaps/probe.yaml", content)

	clone := filepath.Join(t.TempDir(), "clone")
	gitOut(t, "", "clone", "--branch", "main", url, clone)
	cloned, err := os.ReadFile(filepath.Join(clone, "fleet", "ns", "configmaps", "probe.yaml"))
	require.NoError(t, err)
	assert.Equal(t, content, string(cloned))
	assert.Equal(t, pushed, gitOut(t, filepath.Join(root, "fleet.git"), "rev-parse", "main"))
}

func TestHandlerRefusesAnUnknownRepository(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	require.NoError(t, initRepository(t.Context(), root, "fleet.git", "main"))
	url := serve(t, root)

	resp, err := http.Get(strings.TrimSuffix(url, "fleet.git") + "other.git/info/refs?service=git-upload-pack")
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
}

// serve starts the handler of root on a test server and returns the URL of
// the repository fleet.git.
func serve(t *testing.T, root string) string {
	t.Helper()

	handler, err := newHandler(root)
	require.NoError(t, err)
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)

	return server.URL + "/fleet.git"
}

// commitAndPush commits one file in a new work tree, pushes it to the main
// branch of url, and returns the commit.
func commitAndPush(t *testing.T, url, path, content string) string {
	t.Helper()

	work := t.TempDir()
	gitOut(t, work, "init", "--initial-branch=main")
	require.NoError(t, os.MkdirAll(filepath.Join(work, filepath.Dir(path)), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(work, path), []byte(content), 0o600))
	gitOut(t, work, "add", ".")
	gitOut(t, work, "-c", "user.name=fleet-harness", "-c", "user.email=harness@fleet.invalid", "commit", "-m", "probe")
	gitOut(t, work, "push", url, "main")

	return gitOut(t, work, "rev-parse", "HEAD")
}

// gitOut runs git in dir, or in the current directory when dir is empty, and
// returns its trimmed output.
func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()

	_, err := exec.LookPath("git")
	require.NoError(t, err, "the tests need the git binary")

	if dir != "" {
		args = append([]string{"-C", dir}, args...)
	}
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "git %s: %s", strings.Join(args, " "), out)

	return strings.TrimSpace(string(out))
}
