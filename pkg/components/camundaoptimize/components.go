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

package camundaoptimize

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/deployment"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/service"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	clustercomponents "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/workloadmutations"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/servicemonitor"
)

// The probe timings of both workloads. They match the timings of the
// CamundaCluster workloads: the startup probe allows five minutes, which
// covers the first build of the Optimize index schema, and readiness and
// liveness poll only after it passes.
const (
	startupFailureThreshold int32 = 60
	startupPeriodSeconds    int32 = 5
	readinessPeriodSeconds  int32 = 10
	livenessPeriodSeconds   int32 = 30
)

// optimizeUID is the user and group of the camunda/optimize image
// (optimize.Dockerfile, USER 1001:1001).
const optimizeUID int64 = 1001

// components are the components of one CamundaOptimize, in reconcile order.
var components = []string{ComponentWebapp, ComponentImporter}

// conditionTypes maps a component to the condition it reports on the
// CamundaOptimize.
var conditionTypes = map[string]string{
	ComponentWebapp:   v1.ConditionWebappReady,
	ComponentImporter: v1.ConditionImporterReady,
}

// Build returns one component per Optimize workload, in reconcile order: the
// webapp, then the importer. Each carries a Deployment, its Service, and,
// where the Kubernetes cluster serves the kind and the spec asks for it, a
// ServiceMonitor. The copies of referenced Secrets are a separate component,
// see MirroredSecretComponent.
func Build(in Input) ([]*component.Component, error) {
	comps := make([]*component.Component, 0, len(components))
	for _, name := range components {
		comp, err := buildComponent(in, name)
		if err != nil {
			return nil, fmt.Errorf("building %s component: %w", name, err)
		}
		comps = append(comps, comp)
	}

	return comps, nil
}

// buildComponent builds the Deployment, the Service, and the optional
// ServiceMonitor of one component.
func buildComponent(in Input, name string) (*component.Component, error) {
	workload, err := deployment.NewBuilder(deploymentFor(in, name)).
		WithMutation(workloadmutations.Mutations(in.workload(name), optimizeContainer)...).
		Build()
	if err != nil {
		return nil, err
	}

	svc, err := service.NewBuilder(serviceFor(in, name)).Build()
	if err != nil {
		return nil, err
	}

	monitor, err := servicemonitor.NewBuilder(serviceMonitorFor(in, name)).Build()
	if err != nil {
		return nil, err
	}

	return component.NewComponentBuilder().
		WithName(name).
		WithConditionType(component.ConditionType(conditionTypes[name])).
		WithResource(workload).
		WithResource(svc).
		IncludeWhen(in.ServiceMonitorSupported, func() component.Resource { return monitor }, monitoringGate(in)).
		Suspend(in.Suspended).
		Build()
}

// deploymentFor renders the base Deployment of a component.
// workloadmutations.Mutations layers the overrides on top.
func deploymentFor(in Input, comp string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WorkloadName(in.Optimize, comp),
			Namespace: in.Optimize.Namespace,
			Labels:    managedLabels(in, comp),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(in.replicas(comp)),
			Selector: &metav1.LabelSelector{MatchLabels: discoveryLabels(in, comp)},
			Strategy: appsv1.DeploymentStrategy{Type: strategyFor(comp)},
			Template: podTemplate(in, comp),
		},
	}
}

// managedLabels returns the labels of an object that the operator applies for
// a component. The owner is the referenced CamundaCluster, so the Optimize
// workloads carry the same camunda.io/cluster value as the workloads of that
// cluster.
func managedLabels(in Input, comp string) map[string]string {
	return labels.Managed(labels.Cluster(in.ClusterName), comp)
}

// discoveryLabels returns the labels of the pods and the selectors of a
// component.
func discoveryLabels(in Input, comp string) map[string]string {
	return labels.Discovery(labels.Cluster(in.ClusterName), comp)
}

// strategyFor returns the rollout strategy of a component. The importer takes
// Recreate: a rolling update starts the new pod before it stops the old one,
// and two importers that write the same indices at the same time make the
// analytics data inconsistent. The webapp serves read traffic and rolls.
func strategyFor(comp string) appsv1.DeploymentStrategyType {
	if comp == ComponentImporter {
		return appsv1.RecreateDeploymentStrategyType
	}

	return appsv1.RollingUpdateDeploymentStrategyType
}

// podTemplate renders the base pod template of a component: the discovery
// labels, the config hash annotation, the container, the Elasticsearch CA
// volume, and the security context of the image user.
func podTemplate(in Input, comp string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      podLabels(in, comp),
			Annotations: map[string]string{ConfigHashAnnotation: ConfigHash(in, comp)},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{container(in, comp)},
			Volumes:    caVolumes(in),
			SecurityContext: &corev1.PodSecurityContext{
				RunAsUser:    new(optimizeUID),
				RunAsGroup:   new(optimizeUID),
				FSGroup:      new(optimizeUID),
				RunAsNonRoot: new(true),
			},
		},
	}
}

// podLabels returns the labels of the pods of a component: the discovery
// labels and the SecondaryStorageConfig they run on. The importer writes the
// analytics indices of that contract, so a cluster that takes the contract
// over finds these pods with the same selector as the pods of the previous
// holder. The label is on the pods, never on the selector, so a repoint of
// the cluster rolls them and the new ones carry the new value.
func podLabels(in Input, comp string) map[string]string {
	return labels.Merge(
		discoveryLabels(in, comp),
		clustercomponents.StoragePodLabels(in.ClusterName, in.StorageContract),
	)
}

// container renders the Optimize container of a component. Only the importer
// imports the exported records of the cluster.
func container(in Input, comp string) corev1.Container {
	return corev1.Container{
		Name:           optimizeContainer,
		Image:          Image(in),
		Env:            baseEnv(in, comp == ComponentImporter),
		Ports:          containerPorts(),
		VolumeMounts:   caMounts(in),
		StartupProbe:   probe(portNameHTTP, readinessPath, startupPeriodSeconds, startupFailureThreshold),
		ReadinessProbe: probe(portNameHTTP, readinessPath, readinessPeriodSeconds, 0),
		LivenessProbe:  probe(portNameManagement, livenessPath, livenessPeriodSeconds, 0),
	}
}

func containerPorts() []corev1.ContainerPort {
	return []corev1.ContainerPort{
		{Name: portNameHTTP, ContainerPort: PortHTTP, Protocol: corev1.ProtocolTCP},
		{Name: portNameManagement, ContainerPort: PortManagement, Protocol: corev1.ProtocolTCP},
	}
}

// probe builds an HTTP probe on a named port. A zero failureThreshold keeps
// the Kubernetes default.
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

// serviceFor renders the Service of a component. Both ports are exposed: the
// HTTP port serves the user interface and the API, the management port the
// actuator endpoints that the ServiceMonitor scrapes.
func serviceFor(in Input, comp string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WorkloadName(in.Optimize, comp),
			Namespace: in.Optimize.Namespace,
			Labels:    managedLabels(in, comp),
		},
		Spec: corev1.ServiceSpec{
			Selector: discoveryLabels(in, comp),
			Ports: []corev1.ServicePort{
				{
					Name:       portNameHTTP,
					Port:       PortHTTP,
					TargetPort: intstr.FromString(portNameHTTP),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       portNameManagement,
					Port:       PortManagement,
					TargetPort: intstr.FromString(portNameManagement),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// serviceMonitorFor renders the ServiceMonitor that scrapes
// /actuator/prometheus on the management port of a component, through its
// Service.
func serviceMonitorFor(in Input, comp string) *monitoringv1.ServiceMonitor {
	var userLabels, annotations map[string]string
	if monitor := in.serviceMonitorSpec(); monitor != nil {
		userLabels = monitor.Labels
		annotations = monitor.Annotations
	}

	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:        WorkloadName(in.Optimize, comp),
			Namespace:   in.Optimize.Namespace,
			Labels:      labels.Merge(userLabels, managedLabels(in, comp)),
			Annotations: annotations,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector:  metav1.LabelSelector{MatchLabels: discoveryLabels(in, comp)},
			Endpoints: []monitoringv1.Endpoint{{Port: portNameManagement, Path: metricsPath}},
		},
	}
}

// monitoringGate gates the ServiceMonitor on
// spec.monitoring.serviceMonitor.enabled.
func monitoringGate(in Input) component.ResourceOption {
	monitor := in.serviceMonitorSpec()

	return component.GatedBy(feature.NewBooleanGate(monitor != nil && monitor.Enabled))
}
