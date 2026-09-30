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
	rbacv1 "k8s.io/api/rbac/v1"
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
		{Cluster: "my-cluster-db", ClusterUID: "uid-running"},
		{Cluster: "my-cluster-db-r1", ClusterUID: "uid-recovering"},
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
}
