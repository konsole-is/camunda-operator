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

package conformance

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

func TestKindsOfExpandsWildcardsAndDropsSubresources(t *testing.T) {
	rules := []rbacv1.PolicyRule{
		{APIGroups: []string{""}, Resources: []string{"configmaps", "pods/log"}, Verbs: []string{"create"}},
		{APIGroups: []string{"storage.k8s.io"}, Resources: []string{"*"}, Verbs: []string{"*"}},
		{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"get"}},
	}
	listed := []string{"get", "list", "watch", "create"}
	resources := []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: "configmaps", Kind: "ConfigMap", Namespaced: true, Verbs: listed},
				{Name: "secrets", Kind: "Secret", Namespaced: true, Verbs: listed},
				{Name: "pods/log", Kind: "Pod", Namespaced: true, Verbs: []string{"get"}},
			},
		},
		{
			GroupVersion: "storage.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{Name: "storageclasses", Kind: "StorageClass", Verbs: listed},
				{Name: "csidrivers", Kind: "CSIDriver", Verbs: listed},
				{Name: "volumeattachments/status", Kind: "VolumeAttachment", Verbs: []string{"get", "patch"}},
			},
		},
	}

	got := kindsOf(rules, resources)

	assert.Equal(
		t, []kindInfo{
			{gvk: schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, namespaced: true},
			{gvk: schema.GroupVersionKind{Group: "storage.k8s.io", Version: "v1", Kind: "CSIDriver"}},
			{gvk: schema.GroupVersionKind{Group: "storage.k8s.io", Version: "v1", Kind: "StorageClass"}},
		}, got,
	)
}

func TestKindsOfDropsAResourceThatTheAPICannotCreate(t *testing.T) {
	rules := []rbacv1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}}}
	resources := []*metav1.APIResourceList{{
		GroupVersion: "metrics.k8s.io/v1beta1",
		APIResources: []metav1.APIResource{
			{Name: "pods", Kind: "PodMetrics", Namespaced: true, Verbs: []string{"get", "list"}},
		},
	}}

	assert.Empty(t, kindsOf(rules, resources))
}

func TestKindsOfDropsAResourceThatCannotBeListed(t *testing.T) {
	rules := []rbacv1.PolicyRule{{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"create"}}}
	resources := []*metav1.APIResourceList{{
		GroupVersion: "authorization.k8s.io/v1",
		APIResources: []metav1.APIResource{
			{Name: "subjectaccessreviews", Kind: "SubjectAccessReview", Verbs: []string{"create"}},
		},
	}}

	assert.Empty(t, kindsOf(rules, resources))
}

func TestBindsServiceAccount(t *testing.T) {
	tests := []struct {
		name    string
		subject rbacv1.Subject
		binds   bool
	}{
		{
			"the ServiceAccount",
			rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "echo", Namespace: "plugins"},
			true,
		},
		{"its user name", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "system:serviceaccount:plugins:echo"}, true},
		{"all ServiceAccounts", rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:serviceaccounts"}, true},
		{
			"the ServiceAccounts of its namespace",
			rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:serviceaccounts:plugins"},
			true,
		},
		{"every authenticated user", rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:authenticated"}, true},
		{"another namespace", rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "echo", Namespace: "other"}, false},
		{"a user of the same name", rbacv1.Subject{Kind: rbacv1.UserKind, Name: "echo"}, false},
		{
			"the ServiceAccounts of another namespace",
			rbacv1.Subject{Kind: rbacv1.GroupKind, Name: "system:serviceaccounts:other"},
			false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.binds, bindsServiceAccount([]rbacv1.Subject{tt.subject}, "plugins", "echo"))
		})
	}
}

func TestPluginRulesTakesClusterRoleAndRoleBindings(t *testing.T) {
	sa := rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "echo", Namespace: "plugins"}
	cluster := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "echo"},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{"storage.k8s.io"}, Resources: []string{"storageclasses"}, Verbs: []string{"create"}},
		},
	}
	role := &rbacv1.Role{
		ObjectMeta: metav1.ObjectMeta{Namespace: "run", Name: "echo-configmaps"},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"create"}},
		},
	}
	objs := []client.Object{
		cluster,
		role,
		&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{Name: "echo"},
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "echo"},
			Subjects:   []rbacv1.Subject{sa},
		},
		&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{Namespace: "run", Name: "echo"},
			RoleRef:    rbacv1.RoleRef{Kind: "Role", Name: "echo-configmaps"},
			Subjects:   []rbacv1.Subject{sa},
		},
	}
	s := newTestSuite(t, validConfig(), interceptor.Funcs{}, objs...)

	rules, err := pluginRules(t.Context(), s, pluginsv1alpha1.ServiceAccountRef{Namespace: "plugins", Name: "echo"})

	require.NoError(t, err)
	assert.ElementsMatch(t, append(cluster.Rules, role.Rules...), rules)
}

func TestPluginRulesTakesASubjectWithoutNamespaceInTheNamespaceOfTheRoleBinding(t *testing.T) {
	role := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: "edit-configmaps"},
		Rules: []rbacv1.PolicyRule{
			{APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"create"}},
		},
	}
	binding := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: "plugins", Name: "echo"},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "edit-configmaps"},
		Subjects:   []rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: "echo"}},
	}
	s := newTestSuite(t, validConfig(), interceptor.Funcs{}, role, binding)

	rules, err := pluginRules(t.Context(), s, pluginsv1alpha1.ServiceAccountRef{Namespace: "plugins", Name: "echo"})

	require.NoError(t, err)
	assert.Equal(t, role.Rules, rules)
}
