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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The Barman Cloud plugin binds its Role to a ServiceAccount named after the
// cluster. The pods run under the ServiceAccount of the server, so each
// cluster with a plugin Role gets a binding to that account, and the binding
// goes with the cluster.
func TestArchivePluginRolesAreBoundToTheServiceAccount(t *testing.T) {
	t.Parallel()

	server, preset, release := goldenMinimalDatabaseServer()
	server.Spec.Archive = archiveSpec()
	merged := MergeSpec(server.Spec, preset, release)
	roles := []ArchivePluginRole{
		{Cluster: "my-cluster-db", ClusterUID: "uid-running", Granted: true},
		{Cluster: "my-cluster-db-r1", ClusterUID: "uid-recovering", Granted: true},
	}

	comp, _, err := ClusterComponent(server, merged, nil, "", nil, "", roles)
	require.NoError(t, err)
	objects, err := comp.Preview()
	require.NoError(t, err)

	var bindings []*rbacv1.RoleBinding
	for _, obj := range objects {
		if binding, ok := obj.(*rbacv1.RoleBinding); ok {
			bindings = append(bindings, binding)
		}
	}
	require.Len(t, bindings, 2)

	for i, binding := range bindings {
		role := roles[i]
		assert.Equal(t, role.Cluster+"-barman-cloud-postgres", binding.Name)
		assert.Equal(
			t, rbacv1.RoleRef{
				APIGroup: rbacv1.GroupName, Kind: "Role", Name: role.Cluster + "-barman-cloud",
			}, binding.RoleRef,
		)
		assert.Equal(
			t, []rbacv1.Subject{{
				Kind: rbacv1.ServiceAccountKind, Name: "my-cluster-db-postgres", Namespace: server.Namespace,
			}}, binding.Subjects,
		)
		require.Len(t, binding.OwnerReferences, 1)
		assert.Equal(t, "Cluster", binding.OwnerReferences[0].Kind)
		assert.Equal(t, role.ClusterUID, binding.OwnerReferences[0].UID)
	}
}

func TestArchivePluginRoleCluster(t *testing.T) {
	t.Parallel()

	cluster, ok := ArchivePluginRoleCluster("my-cluster-db-r1-barman-cloud")
	assert.True(t, ok)
	assert.Equal(t, "my-cluster-db-r1", cluster)

	_, ok = ArchivePluginRoleCluster("my-cluster-db")
	assert.False(t, ok)

	_, ok = ArchivePluginRoleCluster("-barman-cloud")
	assert.False(t, ok)

	cluster, ok = ArchivePluginBindingCluster("my-cluster-db-r1-barman-cloud-postgres")
	assert.True(t, ok)
	assert.Equal(t, "my-cluster-db-r1", cluster)

	_, ok = ArchivePluginBindingCluster("my-cluster-db-barman-cloud")
	assert.False(t, ok)
}

// A rollback builds its cluster outside the cluster component, and its pods
// take the identity that the ServiceAccount carries when they start.
func TestServiceAccountCarries(t *testing.T) {
	t.Parallel()

	server, preset, release := goldenMinimalDatabaseServer()
	server.Spec.Archive = archiveSpec()
	merged := MergeSpec(server.Spec, preset, release)
	archive := &ArchiveStorage{
		Config: archiveBucket(v1.S3StorageAuth{
			Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
			WorkloadIdentity: &v1.S3WorkloadIdentity{RoleARN: "arn:aws:iam::123456789012:role/new"},
		}),
	}

	stale := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Annotations: map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/old"},
	}}
	assert.False(t, ServiceAccountCarries(stale, server, merged, archive))

	current := &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
		Annotations: map[string]string{
			v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/new",
			"added-by-someone":       "else",
		},
	}}
	assert.True(t, ServiceAccountCarries(current, server, merged, archive))
}
