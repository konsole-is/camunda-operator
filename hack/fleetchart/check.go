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

package main

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/utils/ptr"
)

const (
	// componentLabel and componentValue mark each object of the app.
	componentLabel = "app.kubernetes.io/component"
	componentValue = "fleet-manager"
	// settingsConfigMap holds the settings of the app (contract 6.1, CL9).
	settingsConfigMap = "fleet-manager-settings"
)

// chart holds the objects of the app in one render, one of each kind.
type chart struct {
	deployment         appsv1.Deployment
	service            corev1.Service
	serviceAccount     corev1.ServiceAccount
	clusterRole        rbacv1.ClusterRole
	clusterRoleBinding rbacv1.ClusterRoleBinding
	role               rbacv1.Role
	roleBinding        rbacv1.RoleBinding
	networkPolicy      networkingv1.NetworkPolicy
}

// checkEnabled returns a problem for each rule of the app that the render
// breaks. kinds are the operator kinds that the app must read.
func checkEnabled(objects []*unstructured.Unstructured, kinds []groupResource) []string {
	for _, o := range objects {
		if o.GetKind() == "ConfigMap" && o.GetName() == settingsConfigMap {
			return []string{
				"the chart renders the ConfigMap fleet-manager-settings, so an upgrade would overwrite the settings of the admin",
			}
		}
	}

	c, problems := collect(objects)
	if len(problems) > 0 {
		return problems
	}
	problems = append(problems, checkWorkload(c)...)
	problems = append(problems, checkBindings(c)...)
	problems = append(problems, checkClusterRole(c.clusterRole.Rules, kinds)...)
	problems = append(problems, checkSettingsRole(c.role.Rules)...)

	return append(problems, checkNetwork(c)...)
}

// collect converts the objects of the app into a chart. It returns a problem
// for each kind that the render does not hold exactly once.
func collect(objects []*unstructured.Unstructured) (chart, []string) {
	var c chart
	targets := map[string]any{
		"Deployment":         &c.deployment,
		"Service":            &c.service,
		"ServiceAccount":     &c.serviceAccount,
		"ClusterRole":        &c.clusterRole,
		"ClusterRoleBinding": &c.clusterRoleBinding,
		"Role":               &c.role,
		"RoleBinding":        &c.roleBinding,
		"NetworkPolicy":      &c.networkPolicy,
	}
	found := map[string]int{}
	var problems []string
	for _, o := range objects {
		target, ok := targets[o.GetKind()]
		if !ok || !isApp(o) {
			continue
		}
		found[o.GetKind()]++
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(o.Object, target); err != nil {
			problems = append(problems, fmt.Sprintf("%s %s: %v", o.GetKind(), o.GetName(), err))
		}
	}
	for _, kind := range slices.Sorted(maps.Keys(targets)) {
		if found[kind] != 1 {
			problems = append(problems, fmt.Sprintf("want one %s of the fleet manager, found %d", kind, found[kind]))
		}
	}

	return c, problems
}

func isApp(o *unstructured.Unstructured) bool {
	return o.GetLabels()[componentLabel] == componentValue
}

func checkWorkload(c chart) []string {
	var problems []string
	pod := c.deployment.Spec.Template.Spec
	if pod.SecurityContext == nil || !ptr.Deref(pod.SecurityContext.RunAsNonRoot, false) {
		problems = append(problems, "the fleet manager pod does not set runAsNonRoot: true")
	}
	if pod.ServiceAccountName != c.serviceAccount.Name {
		problems = append(
			problems,
			fmt.Sprintf("the pod runs as ServiceAccount %q, want %q", pod.ServiceAccountName, c.serviceAccount.Name),
		)
	}
	if c.service.Spec.Type != "" && c.service.Spec.Type != corev1.ServiceTypeClusterIP {
		problems = append(problems, fmt.Sprintf("the Service has type %s, want ClusterIP", c.service.Spec.Type))
	}
	if len(pod.Containers) != 1 {
		return append(problems, fmt.Sprintf("the pod has %d containers, want 1", len(pod.Containers)))
	}

	container := pod.Containers[0]
	// The kubelet sets HOSTNAME to the pod name, and the standalone server
	// binds to HOSTNAME. kubectl port-forward connects to localhost.
	if !slices.Contains(container.Env, corev1.EnvVar{Name: "HOSTNAME", Value: "0.0.0.0"}) {
		problems = append(
			problems,
			"the container does not set HOSTNAME=0.0.0.0, so kubectl port-forward cannot reach the server",
		)
	}
	if container.SecurityContext == nil || !ptr.Deref(container.SecurityContext.ReadOnlyRootFilesystem, false) {
		return append(problems, "the container does not set readOnlyRootFilesystem: true")
	}
	for _, path := range []string{"/app/.next/cache", "/tmp"} {
		mounted := slices.ContainsFunc(container.VolumeMounts, func(m corev1.VolumeMount) bool {
			return m.MountPath == path
		})
		if !mounted {
			problems = append(problems, "the container has a read-only root file system and no volume at "+path)
		}
	}

	return problems
}

func checkBindings(c chart) []string {
	var problems []string
	account := rbacv1.Subject{
		Kind:      rbacv1.ServiceAccountKind,
		Name:      c.serviceAccount.Name,
		Namespace: c.serviceAccount.Namespace,
	}
	if ref := c.clusterRoleBinding.RoleRef; ref.Kind != "ClusterRole" || ref.Name != c.clusterRole.Name {
		problems = append(
			problems,
			fmt.Sprintf(
				"the ClusterRoleBinding refers to %s %q, want ClusterRole %q",
				ref.Kind,
				ref.Name,
				c.clusterRole.Name,
			),
		)
	}
	if !slices.Contains(c.clusterRoleBinding.Subjects, account) {
		problems = append(problems, "the ClusterRoleBinding does not bind the ServiceAccount of the app")
	}
	if ref := c.roleBinding.RoleRef; ref.Kind != "Role" || ref.Name != c.role.Name {
		problems = append(
			problems,
			fmt.Sprintf("the RoleBinding refers to %s %q, want Role %q", ref.Kind, ref.Name, c.role.Name),
		)
	}
	if !slices.Contains(c.roleBinding.Subjects, account) {
		problems = append(problems, "the RoleBinding does not bind the ServiceAccount of the app")
	}
	for kind, namespace := range map[string]string{
		"Role":           c.role.Namespace,
		"RoleBinding":    c.roleBinding.Namespace,
		"ServiceAccount": c.serviceAccount.Namespace,
		"NetworkPolicy":  c.networkPolicy.Namespace,
	} {
		if namespace != c.deployment.Namespace {
			problems = append(
				problems,
				fmt.Sprintf(
					"the %s is in namespace %q, want the app namespace %q",
					kind,
					namespace,
					c.deployment.Namespace,
				),
			)
		}
	}
	slices.Sort(problems)

	return problems
}

func checkClusterRole(rules []rbacv1.PolicyRule, kinds []groupResource) []string {
	required := slices.Clone(appRules)
	for _, k := range kinds {
		required = append(required, grant{k.group, k.resource, readVerbs})
	}

	var problems []string
	for _, g := range required {
		for _, verb := range g.verbs {
			if !allows(rules, g.group, g.resource, verb, "") {
				problems = append(
					problems,
					fmt.Sprintf("the ClusterRole does not allow %s on %s", verb, qualified(g.group, g.resource)),
				)
			}
		}
	}
	for _, r := range rules {
		if slices.Contains(r.APIGroups, "*") || slices.Contains(r.Resources, "*") || slices.Contains(r.Verbs, "*") {
			problems = append(problems, "the ClusterRole has the wildcard rule "+r.String())
		}
		if slices.Contains(r.APIGroups, "") && slices.Contains(r.Resources, "configmaps") {
			problems = append(
				problems,
				"the ClusterRole grants ConfigMaps, but the settings rules belong in the Role of the app namespace",
			)
		}
	}
	for _, verb := range []string{"create", "update", "patch", "delete"} {
		if allows(rules, "external-secrets.io", "externalsecrets", verb, "") {
			problems = append(
				problems,
				fmt.Sprintf(
					"the ClusterRole allows %s on externalsecrets.external-secrets.io, which the app only reads",
					verb,
				),
			)
		}
	}

	return problems
}

// allows reports whether a rule grants verb on resource of group. A rule
// with resourceNames counts only when name is one of them. Pass an empty name
// to ask for a rule that covers every name.
func allows(rules []rbacv1.PolicyRule, group, resource, verb, name string) bool {
	return slices.ContainsFunc(rules, func(r rbacv1.PolicyRule) bool {
		if !slices.Contains(r.APIGroups, group) || !slices.Contains(r.Resources, resource) ||
			!slices.Contains(r.Verbs, verb) {
			return false
		}

		return len(r.ResourceNames) == 0 || (name != "" && slices.Contains(r.ResourceNames, name))
	})
}

// qualified writes a resource as kubectl does, such as
// environments.fleet.camunda.io, or pods for the core group.
func qualified(group, resource string) string {
	if group == "" {
		return resource
	}

	return resource + "." + group
}

func checkSettingsRole(rules []rbacv1.PolicyRule) []string {
	var problems []string
	if !allows(rules, "", "configmaps", "create", "") {
		problems = append(problems, "the Role does not allow create on configmaps")
	}
	for _, verb := range []string{"get", "update"} {
		if !allows(rules, "", "configmaps", verb, settingsConfigMap) {
			problems = append(
				problems,
				fmt.Sprintf("the Role does not allow %s on the ConfigMap %s", verb, settingsConfigMap),
			)
		}
		if allows(rules, "", "configmaps", verb, "") {
			problems = append(
				problems,
				fmt.Sprintf("the Role allows %s on every ConfigMap, want only %s", verb, settingsConfigMap),
			)
		}
	}

	return problems
}

func checkNetwork(c chart) []string {
	policy := c.networkPolicy.Spec
	var problems []string
	selector, err := metav1.LabelSelectorAsSelector(&policy.PodSelector)
	if err != nil || selector.Empty() || !selector.Matches(labels.Set(c.deployment.Spec.Template.Labels)) {
		problems = append(problems, "the NetworkPolicy does not select the fleet manager pod")
	}
	if !slices.Contains(policy.PolicyTypes, networkingv1.PolicyTypeIngress) {
		problems = append(problems, "the NetworkPolicy does not have the policy type Ingress")
	}
	if len(policy.Ingress) > 0 {
		problems = append(problems, "the NetworkPolicy allows ingress, want none")
	}

	return problems
}

// checkDisabled returns a problem for each object of the app, found by its
// component label or by fleet-manager in its name.
func checkDisabled(objects []*unstructured.Unstructured) []string {
	var problems []string
	for _, o := range objects {
		if isApp(o) || strings.Contains(o.GetName(), componentValue) {
			problems = append(
				problems,
				fmt.Sprintf("%s %s renders with fleetManager.enabled=false", o.GetKind(), o.GetName()),
			)
		}
	}

	return problems
}
