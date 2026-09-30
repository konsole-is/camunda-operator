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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
)

// The cluster that a rollback left runs until it is removed, also after the
// answer. Its binding must stay in reach, so that it goes with the others
// when another owner takes the ServiceAccount.
func TestArchivePluginRolesKeepTheClusterTheRollbackLeft(t *testing.T) {
	t.Parallel()

	for name, completedAt := range map[string]*metav1.Time{
		"running":  nil,
		"answered": new(metav1.Now()),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertBothClustersBound(t, completedAt)
		})
	}
}

// assertBothClustersBound runs archivePluginRoles after a cutover whose
// recovery completed at completedAt, and expects both clusters granted.
func assertBothClustersBound(t *testing.T, completedAt *metav1.Time) {
	t.Helper()

	server := &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{Name: "camunda", Namespace: "camunda-ns", UID: "server-uid"},
		Status: v1.DatabaseServerStatus{
			Cluster: "camunda-r1",
			Recovery: &v1.DatabaseServerRecoveryStatus{
				Cluster:         "camunda-r1",
				PreviousCluster: "camunda",
				CompletedAt:     completedAt,
			},
		},
	}

	objects := make([]client.Object, 0, 4)
	for _, name := range []string{"camunda", "camunda-r1"} {
		meta := ownedClusterMeta(server)
		meta.Name = name
		meta.UID = types.UID("uid-" + name)
		cluster := &cnpgv1.Cluster{ObjectMeta: meta}
		objects = append(objects, cluster, &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{
			Name:      components.ArchivePluginRoleName(name),
			Namespace: server.Namespace,
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: cnpgv1.SchemeGroupVersion.String(),
				Kind:       "Cluster",
				Name:       name,
				UID:        meta.UID,
				Controller: new(true),
			}},
		}})
	}

	s := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(s))
	require.NoError(t, cnpgv1.AddToScheme(s))
	require.NoError(t, rbacv1.AddToScheme(s))
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(objects...).Build()
	reconciler := &DatabaseServerReconciler{Client: c, APIReader: c, Scheme: s}

	roles, err := reconciler.archivePluginRoles(t.Context(), server, "")
	require.NoError(t, err)

	names := make([]string, 0, len(roles))
	for _, role := range roles {
		assert.True(t, role.Granted, role.Cluster)
		names = append(names, role.Cluster)
	}
	assert.ElementsMatch(t, []string{"camunda-r1", "camunda"}, names)
}
