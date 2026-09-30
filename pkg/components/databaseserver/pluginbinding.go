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

// This file exists until a Barman Cloud plugin release honors
// spec.serviceAccountName (cloudnative-pg/plugin-barman-cloud#1117 and #869).
// Up to v0.14.0 the plugin binds the Role it creates for a cluster to a
// ServiceAccount named after the cluster, and the instance pods of a server
// run under ServiceAccountName instead. Without the binding here they cannot
// read the ObjectStore, and the archive stops. Remove the file and its
// callers once the plugin floor has the fix.

import (
	"strings"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// archivePluginRoleSuffix appended to a cluster name yields the Role that the
// Barman Cloud plugin creates for that cluster.
const archivePluginRoleSuffix = "-barman-cloud"

// ArchivePluginRole names a CloudNativePG cluster of the server for which the
// Barman Cloud plugin has created its Role.
type ArchivePluginRole struct {
	// Cluster is the name of the cluster.
	Cluster string
	// ClusterUID is the UID of the cluster. The RoleBinding carries it as an
	// owner reference, so it goes when the cluster goes.
	ClusterUID types.UID
}

// ArchivePluginRoleName returns the name of the Role that the Barman Cloud
// plugin creates for cluster.
func ArchivePluginRoleName(cluster string) string {
	return cluster + archivePluginRoleSuffix
}

// ArchivePluginRoleCluster returns the cluster that a Role of the Barman Cloud
// plugin is for, or false when role is not named like one.
func ArchivePluginRoleCluster(role string) (string, bool) {
	cluster, found := strings.CutSuffix(role, archivePluginRoleSuffix)

	return cluster, found && cluster != ""
}

// archivePluginBinding renders the RoleBinding that gives the Role of the
// plugin for one cluster to the ServiceAccount of the server.
func archivePluginBinding(server *v1.DatabaseServer, role ArchivePluginRole) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ArchivePluginRoleName(role.Cluster) + serviceAccountSuffix,
			Namespace: server.Namespace,
			Labels:    managedLabels(server),
			OwnerReferences: []metav1.OwnerReference{{
				APIVersion: cnpgv1.SchemeGroupVersion.String(),
				Kind:       "Cluster",
				Name:       role.Cluster,
				UID:        role.ClusterUID,
			}},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "Role",
			Name:     ArchivePluginRoleName(role.Cluster),
		},
		Subjects: []rbacv1.Subject{{
			Kind:      rbacv1.ServiceAccountKind,
			Name:      ServiceAccountName(server),
			Namespace: server.Namespace,
		}},
	}
}
