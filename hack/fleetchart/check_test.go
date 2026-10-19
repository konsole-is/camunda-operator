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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	testName      = "verify-camunda-operator-fleet-manager"
	testNamespace = "default"
)

// testKinds stands for the CRD bases.
var testKinds = []groupResource{{"core.camunda.io", "camundaclusters"}}

var appLabels = map[string]string{
	"app.kubernetes.io/name":      "camunda-operator",
	"app.kubernetes.io/instance":  "verify",
	"app.kubernetes.io/component": "fleet-manager",
}

func rulesFor(grants []grant) []rbacv1.PolicyRule {
	rules := make([]rbacv1.PolicyRule, 0, len(grants))
	for _, g := range grants {
		rules = append(
			rules,
			rbacv1.PolicyRule{APIGroups: []string{g.group}, Resources: []string{g.resource}, Verbs: g.verbs},
		)
	}

	return rules
}

// validChart is a render that passes every check.
func validChart() chart {
	meta := metav1.ObjectMeta{Name: testName, Namespace: testNamespace, Labels: appLabels}
	cluster := metav1.ObjectMeta{Name: testName, Labels: appLabels}
	account := rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: testName, Namespace: testNamespace}

	return chart{
		deployment: appsv1.Deployment{
			TypeMeta:   metav1.TypeMeta{APIVersion: "apps/v1", Kind: "Deployment"},
			ObjectMeta: meta,
			Spec: appsv1.DeploymentSpec{
				Selector: &metav1.LabelSelector{MatchLabels: appLabels},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{Labels: appLabels},
					Spec: corev1.PodSpec{
						ServiceAccountName: testName,
						SecurityContext:    &corev1.PodSecurityContext{RunAsNonRoot: new(true)},
						Containers: []corev1.Container{{
							Name:            "fleet-manager",
							Env:             []corev1.EnvVar{{Name: "HOSTNAME", Value: "0.0.0.0"}},
							SecurityContext: &corev1.SecurityContext{ReadOnlyRootFilesystem: new(true)},
							VolumeMounts: []corev1.VolumeMount{
								{Name: "next-cache", MountPath: "/app/.next/cache"},
								{Name: "tmp", MountPath: "/tmp"},
							},
						}},
					},
				},
			},
		},
		service: corev1.Service{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Service"},
			ObjectMeta: meta,
			Spec:       corev1.ServiceSpec{Type: corev1.ServiceTypeClusterIP},
		},
		serviceAccount: corev1.ServiceAccount{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ServiceAccount"},
			ObjectMeta: meta,
		},
		clusterRole: rbacv1.ClusterRole{
			TypeMeta:   metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRole"},
			ObjectMeta: cluster,
			Rules: append(
				rulesFor(appRules),
				rulesFor([]grant{{"core.camunda.io", "camundaclusters", readVerbs}})...),
		},
		clusterRoleBinding: rbacv1.ClusterRoleBinding{
			TypeMeta:   metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "ClusterRoleBinding"},
			ObjectMeta: cluster,
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "ClusterRole", Name: testName},
			Subjects:   []rbacv1.Subject{account},
		},
		role: rbacv1.Role{
			TypeMeta:   metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "Role"},
			ObjectMeta: meta,
			Rules: []rbacv1.PolicyRule{
				{APIGroups: []string{""}, Resources: []string{"configmaps"}, Verbs: []string{"create"}},
				{
					APIGroups:     []string{""},
					Resources:     []string{"configmaps"},
					ResourceNames: []string{settingsConfigMap},
					Verbs:         []string{"get", "update", "patch"},
				},
			},
		},
		roleBinding: rbacv1.RoleBinding{
			TypeMeta:   metav1.TypeMeta{APIVersion: "rbac.authorization.k8s.io/v1", Kind: "RoleBinding"},
			ObjectMeta: meta,
			RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: "Role", Name: testName},
			Subjects:   []rbacv1.Subject{account},
		},
		networkPolicy: networkingv1.NetworkPolicy{
			TypeMeta:   metav1.TypeMeta{APIVersion: "networking.k8s.io/v1", Kind: "NetworkPolicy"},
			ObjectMeta: meta,
			Spec: networkingv1.NetworkPolicySpec{
				PodSelector: metav1.LabelSelector{MatchLabels: appLabels},
				PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeIngress},
			},
		},
	}
}

// render turns c, and the extra objects, into the objects of a render.
func render(t *testing.T, c chart, extra ...runtime.Object) []*unstructured.Unstructured {
	t.Helper()

	all := append([]runtime.Object{
		&c.deployment, &c.service, &c.serviceAccount, &c.clusterRole,
		&c.clusterRoleBinding, &c.role, &c.roleBinding, &c.networkPolicy,
	}, extra...)
	objects := make([]*unstructured.Unstructured, 0, len(all))
	for _, o := range all {
		content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(o)
		require.NoError(t, err)
		objects = append(objects, &unstructured.Unstructured{Object: content})
	}

	return objects
}

func TestCheckEnabledAcceptsTheAppWithEveryRule(t *testing.T) {
	t.Parallel()

	assert.Empty(t, checkEnabled(render(t, validChart()), testKinds))
}

func TestCheckEnabledRequiresEachObjectOfTheApp(t *testing.T) {
	t.Parallel()

	objects := render(t, validChart())
	withoutPolicy := objects[:len(objects)-1]

	assert.Contains(t, checkEnabled(withoutPolicy, testKinds), "want one NetworkPolicy of the fleet manager, found 0")
}

func TestCheckEnabledRequiresAReadRuleForEachOperatorKind(t *testing.T) {
	t.Parallel()

	kinds := append(testKinds, groupResource{"core.camunda.io", "newkinds"})

	problems := checkEnabled(render(t, validChart()), kinds)

	assert.Contains(t, problems, "the ClusterRole does not allow get on newkinds.core.camunda.io")
	assert.Contains(t, problems, "the ClusterRole does not allow watch on newkinds.core.camunda.io")
}

func TestCheckEnabledRequiresEachVerbOfTheRBACList(t *testing.T) {
	t.Parallel()

	c := validChart()
	for i, r := range c.clusterRole.Rules {
		if r.Resources[0] == "environments" {
			c.clusterRole.Rules[i].Verbs = []string{"get", "list", "watch", "create", "update", "patch"}
		}
	}

	assert.Equal(
		t,
		[]string{"the ClusterRole does not allow delete on environments.fleet.camunda.io"},
		checkEnabled(render(t, c), testKinds),
	)
}

func TestCheckEnabledRefusesSettingsRulesInTheClusterRole(t *testing.T) {
	t.Parallel()

	c := validChart()
	c.clusterRole.Rules = append(c.clusterRole.Rules, rbacv1.PolicyRule{
		APIGroups: []string{""},
		Resources: []string{"configmaps"},
		Verbs:     []string{"get", "update"},
	})

	assert.Equal(
		t,
		[]string{"the ClusterRole grants ConfigMaps, but the settings rules belong in the Role of the app namespace"},
		checkEnabled(render(t, c), testKinds),
	)
}

func TestCheckEnabledRefusesWildcardsAndExternalSecretWrites(t *testing.T) {
	t.Parallel()

	c := validChart()
	c.clusterRole.Rules = append(c.clusterRole.Rules,
		rbacv1.PolicyRule{APIGroups: []string{"apps"}, Resources: []string{"*"}, Verbs: []string{"get"}},
		rbacv1.PolicyRule{
			APIGroups: []string{"external-secrets.io"},
			Resources: []string{"externalsecrets"},
			Verbs:     []string{"create"},
		},
	)

	problems := checkEnabled(render(t, c), testKinds)

	assert.Equal(t, []string{
		"the ClusterRole has a wildcard rule on apiGroups [apps], resources [*], verbs [get]",
		"the ClusterRole allows create on externalsecrets.external-secrets.io, which the app only reads",
	}, problems)
}

func TestCheckEnabledRequiresTheSettingsRoleToNameTheConfigMap(t *testing.T) {
	t.Parallel()

	c := validChart()
	c.role.Rules[1].ResourceNames = nil

	problems := checkEnabled(render(t, c), testKinds)

	assert.Contains(t, problems, "the Role allows get on every ConfigMap, want only fleet-manager-settings")
	assert.Contains(t, problems, "the Role allows update on every ConfigMap, want only fleet-manager-settings")
	assert.Contains(t, problems, "the Role allows patch on every ConfigMap, want only fleet-manager-settings")
}

func TestCheckEnabledRequiresPatchOnTheSettingsConfigMap(t *testing.T) {
	t.Parallel()

	c := validChart()
	c.role.Rules[1].Verbs = []string{"get", "update"}

	assert.Equal(
		t,
		[]string{"the Role does not allow patch on the ConfigMap fleet-manager-settings"},
		checkEnabled(render(t, c), testKinds),
	)
}

func TestCheckEnabledRequiresTheSettingsRoleInTheAppNamespace(t *testing.T) {
	t.Parallel()

	c := validChart()
	c.role.Namespace = "other"

	assert.Contains(
		t,
		checkEnabled(render(t, c), testKinds),
		`the Role is in namespace "other", want the app namespace "default"`,
	)
}

func TestCheckEnabledRequiresBindingsToTheAppServiceAccount(t *testing.T) {
	t.Parallel()

	c := validChart()
	c.clusterRoleBinding.Subjects[0].Name = "someone-else"

	assert.Equal(
		t,
		[]string{"the ClusterRoleBinding does not bind the ServiceAccount of the app"},
		checkEnabled(render(t, c), testKinds),
	)
}

func TestCheckEnabledRefusesARenderedSettingsConfigMap(t *testing.T) {
	t.Parallel()

	settings := &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: settingsConfigMap, Namespace: testNamespace},
	}

	assert.Contains(
		t,
		checkEnabled(render(t, validChart(), settings), testKinds),
		"the chart renders the ConfigMap fleet-manager-settings, so an upgrade would overwrite the settings of the admin",
	)
}

func TestCheckEnabledRequiresAPolicyThatRefusesAllIngress(t *testing.T) {
	t.Parallel()

	c := validChart()
	c.networkPolicy.Spec.Ingress = []networkingv1.NetworkPolicyIngressRule{{}}

	assert.Equal(t, []string{"the NetworkPolicy allows ingress, want none"}, checkEnabled(render(t, c), testKinds))

	c = validChart()
	c.networkPolicy.Spec.PodSelector = metav1.LabelSelector{
		MatchLabels: map[string]string{"app.kubernetes.io/component": "other"},
	}

	assert.Equal(
		t,
		[]string{"the NetworkPolicy does not select the fleet manager pod"},
		checkEnabled(render(t, c), testKinds),
	)
}

func TestCheckEnabledRequiresAClusterIPService(t *testing.T) {
	t.Parallel()

	c := validChart()
	c.service.Spec.Type = corev1.ServiceTypeNodePort

	assert.Equal(t, []string{"the Service has type NodePort, want ClusterIP"}, checkEnabled(render(t, c), testKinds))
}

func TestCheckEnabledRequiresANonRootPod(t *testing.T) {
	t.Parallel()

	c := validChart()
	c.deployment.Spec.Template.Spec.SecurityContext.RunAsNonRoot = nil

	assert.Equal(
		t,
		[]string{"the fleet manager pod does not set runAsNonRoot: true"},
		checkEnabled(render(t, c), testKinds),
	)
}

func TestCheckEnabledRequiresTheServerToListenOnEveryAddress(t *testing.T) {
	t.Parallel()

	c := validChart()
	c.deployment.Spec.Template.Spec.Containers[0].Env = nil

	assert.Equal(
		t,
		[]string{"the container does not set HOSTNAME=0.0.0.0, so kubectl port-forward cannot reach the server"},
		checkEnabled(render(t, c), testKinds),
	)
}

func TestCheckEnabledRequiresAReadOnlyRootFileSystem(t *testing.T) {
	t.Parallel()

	c := validChart()
	c.deployment.Spec.Template.Spec.Containers[0].SecurityContext = nil

	assert.Equal(
		t,
		[]string{"the container does not set readOnlyRootFilesystem: true"},
		checkEnabled(render(t, c), testKinds),
	)
}

func TestCheckEnabledRequiresWritableCacheAndTmp(t *testing.T) {
	t.Parallel()

	c := validChart()
	c.deployment.Spec.Template.Spec.Containers[0].VolumeMounts = nil

	problems := checkEnabled(render(t, c), testKinds)

	assert.Contains(t, problems, "the container has a read-only root file system and no volume at /app/.next/cache")
	assert.Contains(t, problems, "the container has a read-only root file system and no volume at /tmp")
}

func TestCheckDisabledReportsEachAppObject(t *testing.T) {
	t.Parallel()

	operator := &corev1.ServiceAccount{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ServiceAccount"},
		ObjectMeta: metav1.ObjectMeta{Name: "verify-camunda-operator-controller-manager"},
	}
	objects := render(t, validChart(), operator)

	problems := checkDisabled(objects)

	assert.Len(t, problems, 8, "each object of the app, and not the operator ServiceAccount")
	assert.Empty(t, checkDisabled(objects[8:]))
}
