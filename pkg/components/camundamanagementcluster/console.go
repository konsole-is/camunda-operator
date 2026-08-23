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

package camundamanagementcluster

import (
	"fmt"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/deployment"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/service"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
	"github.com/konsole-is/camunda-operator/pkg/images"
	"github.com/konsole-is/camunda-operator/pkg/workloadmutations"
)

// The environment variables of the Console container that no other component
// of the management plane sets. Console reads the Identity SDK settings
// through the CAMUNDA_IDENTITY_* variables that identity.go declares.
//
// Console configuration:
// https://docs.camunda.io/docs/self-managed/components/console/configuration/
const (
	// The Keycloak that Console sends a browser to, the one it reaches from
	// inside the Kubernetes cluster, and the realm both of them hold.
	consoleEnvKeycloakBaseURL         = "KEYCLOAK_BASE_URL"
	consoleEnvKeycloakInternalBaseURL = "KEYCLOAK_INTERNAL_BASE_URL"
	consoleEnvKeycloakRealm           = "KEYCLOAK_REALM"
	// consoleEnvContextPath is the path Console serves under.
	consoleEnvContextPath = "CAMUNDA_CONSOLE_CONTEXT_PATH"
	// consoleEnvDiscoveryMode lets an orchestration cluster register itself
	// through the Console API instead of a written cluster list. The
	// documentation calls the mode experimental.
	consoleEnvDiscoveryMode = "CAMUNDA_CONSOLE_EXPERIMENTAL_DISCOVERY_MODE"
	// consoleEnvNodeEnv is the environment the image runs under. The Console
	// configuration reference does not list it, so the operator sets what the
	// Helm chart sets.
	consoleEnvNodeEnv = "NODE_ENV"
)

// The literal values the renderer sets. The health endpoint, the environment
// of the image, and the ports come from the Console templates of the 8.9 Helm
// chart
// (https://github.com/camunda/camunda-platform-helm/blob/main/charts/camunda-platform-8.9/templates/console).
const (
	// consoleHealthPath is the health endpoint on the management port.
	consoleHealthPath = "/health/readiness"
	// consoleNodeEnv is the value the chart runs the image with.
	consoleNodeEnv = "prod"
	// consoleDiscoveryMode turns the registration API on. The default is
	// false, so the operator sets it on every Console it deploys: a cluster
	// that reports to a Console without it stays invisible.
	consoleDiscoveryMode = "true"
	// consoleContainer is the container of the Console Deployment.
	consoleContainer = "console"
)

// consoleComponents renders Console: its Deployment and its Service, in one
// component under the ConsoleReady condition.
//
// The component is built while spec.console is unset too, gated off. A
// management cluster that drops Console then has its workload deleted instead
// of left running, and the gate keeps the Disabled condition out of Ready.
func consoleComponents(in Input) (Built, error) {
	deployed := in.Cluster.Spec.Console != nil
	gate := feature.NewBooleanGate(deployed)

	workload, err := deployment.NewBuilder(consoleDeployment(in)).
		WithMutation(workloadmutations.Mutations(in.workload(ComponentConsole), consoleContainer)...).
		Build()
	if err != nil {
		return Built{}, fmt.Errorf("building the %s component: %w", ComponentConsole, err)
	}

	svc, err := service.NewBuilder(consoleService(in)).Build()
	if err != nil {
		return Built{}, fmt.Errorf("building the %s component: %w", ComponentConsole, err)
	}

	comp, err := component.NewComponentBuilder().
		WithName(ComponentConsole).
		WithConditionType(component.ConditionType(v1.ConditionConsoleReady)).
		WithFeatureGate(gate).
		WithResource(workload, component.GatedBy(gate)).
		WithResource(svc, component.GatedBy(gate)).
		Suspend(in.Suspended).
		Build()
	if err != nil {
		return Built{}, fmt.Errorf("building the %s component: %w", ComponentConsole, err)
	}

	built := Built{Components: []*component.Component{comp}}
	if deployed {
		built.Ready = built.Components
	}

	return built, nil
}

// consoleDeployment renders the base Deployment. workloadmutations.Mutations
// layers the override surfaces of spec.console on top.
func consoleDeployment(in Input) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConsoleName(in.Cluster),
			Namespace: in.Cluster.Namespace,
			Labels:    managedLabels(in, ComponentConsole),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(in.replicas(ComponentConsole)),
			Selector: &metav1.LabelSelector{MatchLabels: discoveryLabels(in, ComponentConsole)},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      discoveryLabels(in, ComponentConsole),
					Annotations: map[string]string{ConfigHashAnnotation: ConfigHash(in, ComponentConsole)},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{consoleContainerSpec(in)}},
			},
		},
	}
}

// consoleContainerSpec renders the Console container. It carries the readiness
// probe of the 8.9 Helm chart, and no liveness probe, which is what the chart
// enables too. It carries no startup probe either: Console migrates no schema,
// so its readiness probe polls from the start.
func consoleContainerSpec(in Input) corev1.Container {
	return corev1.Container{
		Name:  consoleContainer,
		Image: images.Resolve(in.Platform, images.Console, in.console().Version),
		Env:   consoleEnv(in),
		Ports: []corev1.ContainerPort{
			{Name: portNameHTTP, ContainerPort: ConsolePortHTTP, Protocol: corev1.ProtocolTCP},
			{Name: portNameManagement, ContainerPort: ConsolePortManagement, Protocol: corev1.ProtocolTCP},
		},
		ReadinessProbe: probe(
			portNameManagement, consoleHealthPath, readinessPeriodSeconds, 0,
		),
	}
}

// consoleEnv renders the environment of the Console container: the identity
// provider, the public client of Console, the path it serves under, the
// license, and the discovery mode.
func consoleEnv(in Input) []corev1.EnvVar {
	provider := in.Provider
	client := provider.Clients.Console

	var env []corev1.EnvVar
	if provider.SpringProfile != "" {
		env = append(env, corev1.EnvVar{
			Name:  camundaconfig.EnvSpringProfilesActive,
			Value: provider.SpringProfile,
		})
	}

	env = append(
		env,
		corev1.EnvVar{Name: identityEnvType, Value: provider.Type},
		corev1.EnvVar{Name: identityEnvBaseURL, Value: IdentityServiceURL(in.Cluster)},
		corev1.EnvVar{Name: identityEnvIssuer, Value: provider.IssuerURL},
		corev1.EnvVar{Name: identityEnvIssuerBackendURL, Value: provider.IssuerBackendURL},
		corev1.EnvVar{Name: identityEnvClientID, Value: client.ID},
		corev1.EnvVar{Name: identityEnvAudience, Value: client.Audience},
	)
	if provider.KeycloakURL != "" {
		env = append(
			env,
			corev1.EnvVar{Name: consoleEnvKeycloakBaseURL, Value: provider.KeycloakPublicURL},
			corev1.EnvVar{Name: consoleEnvKeycloakInternalBaseURL, Value: provider.KeycloakURL},
			corev1.EnvVar{Name: consoleEnvKeycloakRealm, Value: provider.Realm},
		)
	}
	if path := externalPath(parseURL(in.console().ExternalURL)); path != "" {
		env = append(env, corev1.EnvVar{Name: consoleEnvContextPath, Value: path})
	}

	env = append(
		env,
		corev1.EnvVar{Name: consoleEnvDiscoveryMode, Value: consoleDiscoveryMode},
		corev1.EnvVar{Name: consoleEnvNodeEnv, Value: consoleNodeEnv},
	)
	if ref := in.Platform.LicenseSecretRef; ref != nil {
		env = append(env, corev1.EnvVar{
			Name:      camundaconfig.EnvLicenseKey,
			ValueFrom: secretSource(ref.Name, ref.Key),
		})
	}

	return env
}

// consoleService renders the Service of Console. Both ports are exposed: the
// HTTP port serves the user interface and the API that an orchestration
// cluster registers itself through, the management port the health endpoints.
func consoleService(in Input) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConsoleName(in.Cluster),
			Namespace: in.Cluster.Namespace,
			Labels:    managedLabels(in, ComponentConsole),
		},
		Spec: corev1.ServiceSpec{
			Selector: discoveryLabels(in, ComponentConsole),
			Ports: []corev1.ServicePort{
				{
					Name:       portNameHTTP,
					Port:       ConsoleServicePortHTTP,
					TargetPort: intstr.FromString(portNameHTTP),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       portNameManagement,
					Port:       ConsoleServicePortManagement,
					TargetPort: intstr.FromString(portNameManagement),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}
