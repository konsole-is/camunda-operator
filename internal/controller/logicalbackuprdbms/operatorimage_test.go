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

package logicalbackuprdbms

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// ownPodName is the hostname the fakes report: the name of the operator's
// own Pod.
const ownPodName = "operator-7f9-x2"

func ownPod(namespace, name string, containers ...corev1.Container) *corev1.Pod {
	pod := &corev1.Pod{}
	pod.Namespace, pod.Name = namespace, name
	pod.Spec.Containers = containers

	return pod
}

func podReader(t *testing.T, objects ...client.Object) client.Reader {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).WithObjects(objects...).Build()
}

func TestResolveOperatorImageReadsTheManagerContainer(t *testing.T) {
	t.Parallel()

	reader := podReader(t, ownPod(
		"operator-system", ownPodName,
		corev1.Container{Name: "sidecar", Image: "proxy:1"},
		corev1.Container{Name: "manager", Image: "ghcr.io/konsole-is/camunda-operator:0.3.0"},
	))
	hostname := func() (string, error) { return ownPodName, nil }
	readFile := func(string) ([]byte, error) { return []byte("operator-system\n"), nil }

	image, err := ResolveOperatorImage(context.Background(), reader, hostname, readFile)
	require.NoError(t, err)
	assert.Equal(t, "ghcr.io/konsole-is/camunda-operator:0.3.0", image)
}

func TestResolveOperatorImageFailsWithoutAManagerContainer(t *testing.T) {
	t.Parallel()

	reader := podReader(t, ownPod(
		"operator-system", ownPodName,
		corev1.Container{Name: "app", Image: "other:1"},
	))
	hostname := func() (string, error) { return ownPodName, nil }
	readFile := func(string) ([]byte, error) { return []byte("operator-system"), nil }

	_, err := ResolveOperatorImage(context.Background(), reader, hostname, readFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "manager")
}

func TestResolveOperatorImageSurfacesAMissingNamespaceFile(t *testing.T) {
	t.Parallel()

	hostname := func() (string, error) { return ownPodName, nil }
	readFile := func(string) ([]byte, error) { return nil, errors.New("no such file") }

	_, err := ResolveOperatorImage(context.Background(), podReader(t), hostname, readFile)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "namespace")
}

func TestOperatorImagePrefersTheExplicitOverride(t *testing.T) {
	t.Parallel()

	// No APIReader: touching it would panic, proving the override short-circuits.
	r := &LogicalBackupRDBMSReconciler{OperatorImage: "registry.example/operator:pinned"}

	image, err := r.operatorImage(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "registry.example/operator:pinned", image)
}
