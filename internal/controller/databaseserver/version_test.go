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

package databaseserver

import (
	"testing"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// The name of the cluster is derived, so a cluster under it can hold the data
// directory of somebody else. Reading the major off that one pins the merged
// spec to a version this server never ran, and Ready then reports a refused
// version where the name being held is what the reader has to act on.
func TestKeepRunningVersionReadsOnlyAClusterTheServerOwns(t *testing.T) {
	t.Parallel()

	server := &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{Name: "camunda", Namespace: "camunda-ns", UID: types.UID("server-uid")},
	}

	tests := []struct {
		name     string
		meta     metav1.ObjectMeta
		refuses  bool
		expected string
	}{
		{
			name:     "the cluster of the server",
			meta:     ownedClusterMeta(server),
			refuses:  true,
			expected: "16",
		},
		{
			name:     "a cluster another owner controls",
			meta:     foreignClusterMeta(server, "other-uid"),
			expected: "17",
		},
		{
			name:     "a cluster nothing controls",
			meta:     unownedClusterMeta(server),
			expected: "17",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			reconciler := &DatabaseServerReconciler{
				APIReader: versionReader(t, &cnpgv1.Cluster{
					ObjectMeta: tt.meta,
					Status: cnpgv1.ClusterStatus{
						PGDataImageInfo: &cnpgv1.ImageInfo{MajorVersion: 16},
					},
				}),
			}
			merged := v1.DatabaseServerSpec{Version: "17", DatabaseServerConfig: "camunda"}

			refused, err := reconciler.keepRunningVersion(t.Context(), server, &merged)

			require.NoError(t, err)
			assert.Equal(t, tt.expected, merged.Version)
			if !tt.refuses {
				assert.Nil(t, refused)
				return
			}

			require.NotNil(t, refused)
			assert.Equal(t, v1.ReasonVersionChangeRefused, refused.Reason)
		})
	}
}

// ownedClusterMeta is the metadata of a cluster that the operator built for
// server: the controller reference and the label, which is what every object
// it applies carries.
func ownedClusterMeta(server *v1.DatabaseServer) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Name:      server.Name,
		Namespace: server.Namespace,
		Labels:    map[string]string{labels.DatabaseServerKey: server.Name},
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: v1.GroupVersion.String(),
			Kind:       "DatabaseServer",
			Name:       server.Name,
			UID:        server.UID,
			Controller: new(true),
		}},
	}
}

// foreignClusterMeta is the metadata of a cluster of the derived name that
// another DatabaseServer controls. It carries the label of this server too,
// because a label is a value anybody can write.
func foreignClusterMeta(server *v1.DatabaseServer, holder types.UID) metav1.ObjectMeta {
	meta := ownedClusterMeta(server)
	meta.OwnerReferences[0].Name = "holder"
	meta.OwnerReferences[0].UID = holder

	return meta
}

// unownedClusterMeta is the metadata of a cluster of the derived name that
// nothing controls.
func unownedClusterMeta(server *v1.DatabaseServer) metav1.ObjectMeta {
	meta := ownedClusterMeta(server)
	meta.OwnerReferences = nil

	return meta
}

// versionReader is the live reader of the version guard, holding the objects
// the guard reads: the CloudNativePG clusters and the published contracts.
func versionReader(t *testing.T, objects ...client.Object) client.Reader {
	t.Helper()

	s := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(s))
	require.NoError(t, cnpgv1.AddToScheme(s))

	return fake.NewClientBuilder().WithScheme(s).WithObjects(objects...).Build()
}

// A refusal keeps a running server on the major it has. The two states that
// must not refuse are the ones a server passes through on its way up: no
// cluster yet, and a cluster that has not reported the major of its data
// directory.
func TestRefusedMajorChange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		running *cnpgv1.ImageInfo
		refuses bool
	}{
		{
			name:    "the same major",
			version: "17",
			running: &cnpgv1.ImageInfo{MajorVersion: 17},
		},
		{
			name:    "a higher major",
			version: "18",
			running: &cnpgv1.ImageInfo{MajorVersion: 17},
			refuses: true,
		},
		{
			name:    "a lower major",
			version: "16",
			running: &cnpgv1.ImageInfo{MajorVersion: 17},
			refuses: true,
		},
		{
			name:    "no major reported yet",
			version: "18",
		},
		{
			name:    "an empty major reported",
			version: "18",
			running: &cnpgv1.ImageInfo{Image: "ghcr.io/cloudnative-pg/postgresql:18"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			refused := refusedMajorChange(tt.version, tt.running)
			if !tt.refuses {
				assert.Nil(t, refused)
				return
			}

			require.NotNil(t, refused)
			assert.Equal(t, v1.ReasonVersionChangeRefused, refused.Reason)
			assert.Contains(t, refused.Message, tt.version)
			assert.Contains(t, refused.Message, "17")
		})
	}
}

// The guard reads every contract the server owns, because the one the spec
// names now can be unpublished: a rename lands before the reconcile repairs
// status.cluster, and the cluster is then named only by the contract of the
// name before it. The order decides which answer wins when several name a
// cluster, and the contract a rollback answers on names the one it moved to.
func TestContractsByPreference(t *testing.T) {
	t.Parallel()

	published := []v1.DatabaseServerConfig{
		{ObjectMeta: metav1.ObjectMeta{Name: "older"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "spec"}},
		{ObjectMeta: metav1.ObjectMeta{Name: "rollback"}},
	}

	names := func(ordered []v1.DatabaseServerConfig) []string {
		out := make([]string, 0, len(ordered))
		for _, contract := range ordered {
			out = append(out, contract.Name)
		}

		return out
	}

	answering := &v1.DatabaseServer{
		Status: v1.DatabaseServerStatus{
			Recovery: &v1.DatabaseServerRecoveryStatus{Contract: "rollback"},
		},
	}
	assert.Equal(
		t,
		[]string{"rollback", "spec", "older"},
		names(contractsByPreference(answering, "spec", published)),
	)

	// With no rollback recorded the spec leads, and the rest keep their order.
	assert.Equal(
		t,
		[]string{"spec", "older", "rollback"},
		names(contractsByPreference(&v1.DatabaseServer{}, "spec", published)),
	)

	// A contract of neither name is still read: it can be the only one left
	// that names the cluster the server runs.
	assert.Len(t, contractsByPreference(&v1.DatabaseServer{}, "gone", published), 3)
}
