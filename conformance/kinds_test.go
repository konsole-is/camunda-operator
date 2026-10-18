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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

func TestBindsServiceAccountMatchesKindNameAndNamespace(t *testing.T) {
	sa := rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "echo", Namespace: "plugins"}

	assert.True(t, bindsServiceAccount([]rbacv1.Subject{{Kind: "User", Name: "x"}, sa}, "plugins", "echo"))
	assert.False(
		t,
		bindsServiceAccount(
			[]rbacv1.Subject{{Kind: rbacv1.ServiceAccountKind, Name: "echo", Namespace: "other"}},
			"plugins",
			"echo",
		),
	)
	assert.False(
		t,
		bindsServiceAccount([]rbacv1.Subject{{Kind: "User", Name: "echo", Namespace: "plugins"}}, "plugins", "echo"),
	)
}
