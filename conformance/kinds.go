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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// kindInfo is one kind that the plugin can write.
type kindInfo struct {
	gvk        schema.GroupVersionKind
	namespaced bool
}

// pluginKinds returns the kinds that the roles bound to the ServiceAccount sa
// can create. A group that discovery cannot read is left out. The list is
// empty for a plugin that writes no Kubernetes object.
func pluginKinds(ctx context.Context, s *suite, sa pluginsv1alpha1.ServiceAccountRef) ([]kindInfo, error) {
	rules, err := pluginRules(ctx, s, sa)
	if err != nil {
		return nil, err
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

// pluginRules returns the rules of each ClusterRoleBinding and RoleBinding
// that binds sa.
func pluginRules(ctx context.Context, s *suite, sa pluginsv1alpha1.ServiceAccountRef) ([]rbacv1.PolicyRule, error) {
	var clusterBindings rbacv1.ClusterRoleBindingList
	if err := s.client.List(ctx, &clusterBindings); err != nil {
		return nil, fmt.Errorf("listing the ClusterRoleBindings: %w", err)
	}

	var bindings rbacv1.RoleBindingList
	if err := s.client.List(ctx, &bindings); err != nil {
		return nil, fmt.Errorf("listing the RoleBindings: %w", err)
	}

	type bound struct {
		namespace string
		ref       rbacv1.RoleRef
	}
	var refs []bound
	for _, b := range clusterBindings.Items {
		if bindsServiceAccount(b.Subjects, sa.Namespace, sa.Name) {
			refs = append(refs, bound{ref: b.RoleRef})
		}
	}
	for _, b := range bindings.Items {
		// A ServiceAccount subject of a RoleBinding without a namespace is in
		// the namespace of the RoleBinding.
		subjects := slices.Clone(b.Subjects)
		for i := range subjects {
			if subjects[i].Kind == rbacv1.ServiceAccountKind && subjects[i].Namespace == "" {
				subjects[i].Namespace = b.Namespace
			}
		}

		if bindsServiceAccount(subjects, sa.Namespace, sa.Name) {
			refs = append(refs, bound{namespace: b.Namespace, ref: b.RoleRef})
		}
	}

	var rules []rbacv1.PolicyRule
	for _, r := range refs {
		switch r.ref.Kind {
		case "ClusterRole":
			var role rbacv1.ClusterRole
			if err := s.client.Get(ctx, client.ObjectKey{Name: r.ref.Name}, &role); err != nil {
				return nil, fmt.Errorf("getting the ClusterRole %s: %w", r.ref.Name, err)
			}
			rules = append(rules, role.Rules...)
		case "Role":
			var role rbacv1.Role
			if err := s.client.Get(ctx, client.ObjectKey{Namespace: r.namespace, Name: r.ref.Name}, &role); err != nil {
				return nil, fmt.Errorf("getting the Role %s/%s: %w", r.namespace, r.ref.Name, err)
			}
			rules = append(rules, role.Rules...)
		}
	}

	return rules, nil
}

// bindsServiceAccount says whether subjects name the ServiceAccount, its
// user name, or one of its groups.
func bindsServiceAccount(subjects []rbacv1.Subject, namespace, name string) bool {
	groups := []string{"system:serviceaccounts", "system:serviceaccounts:" + namespace, "system:authenticated"}

	return slices.ContainsFunc(subjects, func(s rbacv1.Subject) bool {
		switch s.Kind {
		case rbacv1.ServiceAccountKind:
			return s.Namespace == namespace && s.Name == name
		case rbacv1.UserKind:
			return s.Name == "system:serviceaccount:"+namespace+":"+name
		case rbacv1.GroupKind:
			return slices.Contains(groups, s.Name)
		}
		return false
	})
}

// kindsOf returns each kind of resources that rules allow to create and that
// the API can create and list, one time, sorted by group and then kind. A subresource is
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
			if strings.Contains(r.Name, "/") || !slices.Contains(r.Verbs, "list") ||
				!slices.Contains(r.Verbs, "create") || seen[gk] ||
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
// the label of the request. It leaves out the objects of preexisting, which
// existed before the run created its requests, for example the orphaned
// objects of an earlier run.
func writtenObjects(
	ctx context.Context,
	s *suite,
	kinds []kindInfo,
	request string,
	preexisting map[types.UID]bool,
) ([]*unstructured.Unstructured, error) {
	var objs []*unstructured.Unstructured
	for _, k := range kinds {
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(k.gvk.GroupVersion().WithKind(k.gvk.Kind + "List"))

		if err := s.client.List(ctx, list, client.MatchingLabels{pluginsv1alpha1.LabelRequest: request}); err != nil {
			return nil, fmt.Errorf("listing the %s objects of the request: %w", k.gvk.Kind, err)
		}

		for i := range list.Items {
			if !preexisting[list.Items[i].GetUID()] {
				objs = append(objs, &list.Items[i])
			}
		}
	}

	return objs, nil
}
