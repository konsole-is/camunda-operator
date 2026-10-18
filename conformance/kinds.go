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
	"context"
	"fmt"
	"slices"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// kindInfo is one kind that the plugin can write.
type kindInfo struct {
	gvk        schema.GroupVersionKind
	namespaced bool
}

// pluginKinds returns the kinds that the ClusterRoles bound to the
// ServiceAccount sa can create. A group that discovery cannot read is left
// out. It is an error when no ClusterRoleBinding binds sa.
func pluginKinds(ctx context.Context, s *suite, sa pluginsv1alpha1.ServiceAccountRef) ([]kindInfo, error) {
	var bindings rbacv1.ClusterRoleBindingList
	if err := s.client.List(ctx, &bindings); err != nil {
		return nil, fmt.Errorf("listing the ClusterRoleBindings: %w", err)
	}

	var rules []rbacv1.PolicyRule
	bound := false
	for _, b := range bindings.Items {
		if b.RoleRef.Kind != "ClusterRole" || !bindsServiceAccount(b.Subjects, sa.Namespace, sa.Name) {
			continue
		}
		bound = true

		var role rbacv1.ClusterRole
		if err := s.client.Get(ctx, client.ObjectKey{Name: b.RoleRef.Name}, &role); err != nil {
			return nil, fmt.Errorf("getting the ClusterRole %s of the ClusterRoleBinding %s: %w", b.RoleRef.Name, b.Name, err)
		}
		rules = append(rules, role.Rules...)
	}

	if !bound {
		return nil, fmt.Errorf(
			"no ClusterRoleBinding binds the ServiceAccount %s/%s of the registration",
			sa.Namespace,
			sa.Name,
		)
	}

	dc, err := discovery.NewDiscoveryClientForConfig(s.restConfig)
	if err != nil {
		return nil, fmt.Errorf("connecting to the discovery API: %w", err)
	}

	resources, err := dc.ServerPreferredResources()
	if err != nil && !discovery.IsGroupDiscoveryFailedError(err) {
		return nil, fmt.Errorf("reading the resources of the cluster: %w", err)
	}

	return kindsOf(rules, resources), nil
}

func bindsServiceAccount(subjects []rbacv1.Subject, namespace, name string) bool {
	return slices.ContainsFunc(subjects, func(s rbacv1.Subject) bool {
		return s.Kind == rbacv1.ServiceAccountKind && s.Namespace == namespace && s.Name == name
	})
}

// kindsOf returns each kind of resources that rules allow to create and that
// the API can list, one time, sorted by group and then kind. A subresource is
// not a kind.
func kindsOf(rules []rbacv1.PolicyRule, resources []*metav1.APIResourceList) []kindInfo {
	seen := map[schema.GroupKind]bool{}
	var kinds []kindInfo

	for _, list := range resources {
		gv, err := schema.ParseGroupVersion(list.GroupVersion)
		if err != nil {
			continue
		}

		for _, r := range list.APIResources {
			gk := schema.GroupKind{Group: gv.Group, Kind: r.Kind}
			if strings.Contains(r.Name, "/") || !slices.Contains(r.Verbs, "list") || seen[gk] ||
				!allowsCreate(rules, gv.Group, r.Name) {
				continue
			}

			seen[gk] = true
			kinds = append(kinds, kindInfo{gvk: gv.WithKind(r.Kind), namespaced: r.Namespaced})
		}
	}

	slices.SortFunc(kinds, func(a, b kindInfo) int {
		return strings.Compare(a.gvk.Group+"/"+a.gvk.Kind, b.gvk.Group+"/"+b.gvk.Kind)
	})

	return kinds
}

func allowsCreate(rules []rbacv1.PolicyRule, group, resource string) bool {
	matches := func(values []string, value string) bool {
		return slices.Contains(values, value) || slices.Contains(values, "*")
	}

	return slices.ContainsFunc(rules, func(r rbacv1.PolicyRule) bool {
		return matches(r.APIGroups, group) && matches(r.Resources, resource) && matches(r.Verbs, "create")
	})
}

// writtenObjects lists, over every namespace, the objects of kinds that carry
// the label of the request.
func writtenObjects(
	ctx context.Context,
	s *suite,
	kinds []kindInfo,
	request string,
) ([]*unstructured.Unstructured, error) {
	var objs []*unstructured.Unstructured
	for _, k := range kinds {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(k.gvk.GroupVersion().WithKind(k.gvk.Kind + "List"))

		if err := s.client.List(ctx, list, client.MatchingLabels{pluginsv1alpha1.LabelRequest: request}); err != nil {
			return nil, fmt.Errorf("listing the %s objects of the request: %w", k.gvk.Kind, err)
		}

		for i := range list.Items {
			objs = append(objs, &list.Items[i])
		}
	}

	return objs, nil
}
