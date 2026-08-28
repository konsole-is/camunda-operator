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
	"net/url"
	"strings"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// Built is what Build returns.
type Built struct {
	// Components is every component, in reconcile order. The controller
	// passes it to FlushStatus, which then owns the condition of each one.
	Components []*component.Component
	// Ready are the components that the Ready condition aggregates over.
	Ready []*component.Component
}

// The probe timings of the management plane. The startup probe allows five
// minutes, which covers the first start, where Management Identity and Web
// Modeler migrate their database schema, and readiness polls only after it
// passes.
const (
	startupFailureThreshold int32 = 60
	startupPeriodSeconds    int32 = 5
	readinessPeriodSeconds  int32 = 10
)

// builders render the components of the management plane, in reconcile order:
// the copied and the generated Secrets first, because the workloads mount
// them, then Keycloak, whose operator writes the Secret that Management
// Identity signs in with, then one entry per remaining workload. A builder
// reports which of its components take part in Ready, so that a component the
// spec turned off still reconciles and deletes what it left behind without
// its Disabled reason becoming the reason of the whole management plane. This
// list is the extension point of the package.
var builders = []func(Input) (Built, error){
	mirroredSecretComponent,
	secretsComponents,
	keycloakComponents,
	identityComponents,
	consoleComponents,
	webModelerComponents,
}

// Build renders every component of one management plane, in the order the
// builders are registered.
func Build(in Input) (Built, error) {
	var built Built
	for _, build := range builders {
		rendered, err := build(in)
		if err != nil {
			return Built{}, err
		}
		built.Components = append(built.Components, rendered.Components...)
		built.Ready = append(built.Ready, rendered.Ready...)
	}

	return built, nil
}

// managedLabels returns the labels of an object that the operator applies for
// a component of the management plane.
func managedLabels(in Input, comp string) map[string]string {
	return labels.Managed(labels.ManagementCluster(in.Cluster.Name), comp)
}

// discoveryLabels returns the labels of the pods and the selectors of a
// component.
func discoveryLabels(in Input, comp string) map[string]string {
	return labels.Discovery(labels.ManagementCluster(in.Cluster.Name), comp)
}

// probe builds an HTTP probe on a health endpoint of a named port. A zero
// failureThreshold keeps the Kubernetes default.
func probe(port, path string, periodSeconds, failureThreshold int32) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler: corev1.ProbeHandler{HTTPGet: &corev1.HTTPGetAction{
			Path: path,
			Port: intstr.FromString(port),
		}},
		PeriodSeconds:    periodSeconds,
		FailureThreshold: failureThreshold,
	}
}

// secretSource builds the source of an environment variable that reads one key
// of a Secret in the pod's namespace. Every reference in Input already points
// at a Secret of that namespace, because the controller copies the ones that
// live elsewhere.
func secretSource(name, key string) *corev1.EnvVarSource {
	return &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: name},
		Key:                  key,
	}}
}

// configMapSource builds the source of an environment variable that reads one
// key of a ConfigMap in the pod's namespace.
func configMapSource(name, key string) *corev1.EnvVarSource {
	return &corev1.EnvVarSource{ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: name},
		Key:                  key,
	}}
}

// parseURL reads an external URL of the spec. The CRD validates every one of
// them as an http or https URL with a host, so a URL that does not parse
// cannot reach the renderer. It yields the empty URL rather than an error the
// caller could not act on.
func parseURL(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		return &url.URL{}
	}

	return parsed
}

// externalPath returns the path of a parsed external URL, without a trailing
// slash. It is empty for a URL that serves the root of its host. A component
// that runs under a path of its own has to be told that path.
func externalPath(external *url.URL) string {
	return strings.TrimSuffix(external.Path, "/")
}
