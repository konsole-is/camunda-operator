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

package conditions

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// k8sClient talks to a bare envtest API server with the operator CRDs
// installed and no manager or controllers running, so tests own their objects
// exclusively — no reconciler races their writes.
var k8sClient client.Client

func TestMain(m *testing.M) {
	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	// Allows running tests from IDEs without KUBEBUILDER_ASSETS set.
	if dir := firstEnvTestBinaryDir(); dir != "" {
		testEnv.BinaryAssetsDirectory = dir
	}

	cfg, err := testEnv.Start()
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting envtest: %v\n", err)
		os.Exit(1)
	}

	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		fmt.Fprintf(os.Stderr, "registering client-go scheme: %v\n", err)
		os.Exit(1)
	}
	if err := v1.AddToScheme(scheme); err != nil {
		fmt.Fprintf(os.Stderr, "registering api/v1 scheme: %v\n", err)
		os.Exit(1)
	}

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		fmt.Fprintf(os.Stderr, "creating client: %v\n", err)
		os.Exit(1)
	}

	code := m.Run()

	if err := testEnv.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "stopping envtest: %v\n", err)
	}

	os.Exit(code)
}

// firstEnvTestBinaryDir locates envtest binaries under bin/k8s for runs that
// bypass the Makefile (which exports KUBEBUILDER_ASSETS itself).
func firstEnvTestBinaryDir() string {
	basePath := filepath.Join("..", "..", "bin", "k8s")

	entries, err := os.ReadDir(basePath)
	if err != nil {
		return ""
	}

	for _, entry := range entries {
		if entry.IsDir() {
			return filepath.Join(basePath, entry.Name())
		}
	}

	return ""
}

// newDatabaseServerConfig creates a uniquely named minimal DatabaseServerConfig
// and registers its deletion with t.Cleanup.
func newDatabaseServerConfig(ctx context.Context, t *testing.T) *v1.DatabaseServerConfig {
	t.Helper()

	resource := &v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsc-" + utilrand.String(8)},
		Spec: v1.DatabaseServerConfigSpec{
			Engine: v1.DatabaseEnginePostgres,
			Host:   "postgres.camunda-system.svc.cluster.local",
			Port:   5432,
			AdminCredentialsSecretRef: v1.CredentialsSecretRef{
				Name: "admin-creds", Namespace: "camunda-system",
				UsernameKey: "username", PasswordKey: "password",
			},
		},
	}
	require.NoError(t, k8sClient.Create(ctx, resource))

	t.Cleanup(func() {
		// t.Context is canceled before cleanups run; use a fresh context.
		assert.NoError(t, k8sClient.Delete(context.Background(), resource))
	})

	return resource
}

func fetchDatabaseServerConfig(ctx context.Context, t *testing.T, name string) *v1.DatabaseServerConfig {
	t.Helper()

	var fetched v1.DatabaseServerConfig
	require.NoError(t, k8sClient.Get(ctx, types.NamespacedName{Name: name}, &fetched))

	return &fetched
}

func TestPatchReadyAppliesConditionAndObservedGeneration(t *testing.T) {
	ctx := t.Context()
	resource := newDatabaseServerConfig(ctx, t)

	cond := Ready(metav1.ConditionTrue, ReasonHealthy, "All checks passed", resource.Generation)
	require.NoError(t, PatchReady(ctx, k8sClient, resource, cond))

	fetched := fetchDatabaseServerConfig(ctx, t, resource.Name)
	assert.Equal(t, resource.Generation, fetched.Status.ObservedGeneration)

	ready := meta.FindStatusCondition(fetched.Status.Conditions, TypeReady)
	require.NotNil(t, ready)
	assert.Equal(t, metav1.ConditionTrue, ready.Status)
	assert.Equal(t, ReasonHealthy, ready.Reason)
	assert.Equal(t, "All checks passed", ready.Message)
	assert.Equal(t, resource.Generation, ready.ObservedGeneration)
	assert.False(t, ready.LastTransitionTime.IsZero())
}

func TestPatchReadyPreservesLastTransitionTimeWhenStatusUnchanged(t *testing.T) {
	ctx := t.Context()
	resource := newDatabaseServerConfig(ctx, t)

	require.NoError(t, PatchReady(ctx, k8sClient, resource, Ready(
		metav1.ConditionFalse, ReasonMissingSecret, `Secret "ns/s" not found`, resource.Generation,
	)))

	// Backdate the persisted transition time: metav1.Time has second precision
	// and both patches land within one second, so preservation would otherwise
	// be indistinguishable from a re-stamp.
	backdated := metav1.NewTime(time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC))
	persisted := fetchDatabaseServerConfig(ctx, t, resource.Name)
	meta.FindStatusCondition(persisted.Status.Conditions, TypeReady).LastTransitionTime = backdated
	require.NoError(t, k8sClient.Status().Update(ctx, persisted))

	fresh := fetchDatabaseServerConfig(ctx, t, resource.Name)
	require.NoError(t, PatchReady(ctx, k8sClient, fresh, Ready(
		metav1.ConditionFalse, ReasonMissingSecret, `Secret "ns/s" is missing key "k"`, fresh.Generation,
	)))

	ready := meta.FindStatusCondition(
		fetchDatabaseServerConfig(ctx, t, resource.Name).Status.Conditions, TypeReady,
	)
	require.NotNil(t, ready)
	assert.Equal(t, `Secret "ns/s" is missing key "k"`, ready.Message)
	// time.Time.Equal compares instants; struct equality would fail on the time
	// zone the apiserver serialized the timestamp into.
	assert.True(
		t,
		ready.LastTransitionTime.Time.Equal(backdated.Time),
		"LastTransitionTime %s should equal backdated %s", ready.LastTransitionTime, backdated,
	)
}

func TestPatchReadySkipsAPICallWhenPersistedStatusMatches(t *testing.T) {
	ctx := t.Context()
	resource := newDatabaseServerConfig(ctx, t)

	cond := Ready(metav1.ConditionTrue, ReasonHealthy, "All checks passed", resource.Generation)
	require.NoError(t, PatchReady(ctx, k8sClient, resource, cond))

	before := fetchDatabaseServerConfig(ctx, t, resource.Name)
	require.NoError(t, PatchReady(ctx, k8sClient, before, cond))

	after := fetchDatabaseServerConfig(ctx, t, resource.Name)
	assert.Equal(
		t,
		before.ResourceVersion, after.ResourceVersion,
		"a no-op PatchReady must not write to the API server",
	)
}
