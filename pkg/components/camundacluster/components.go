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

package camundacluster

import (
	"fmt"

	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/deployment"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/service"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/serviceaccount"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/servicemonitor"
)

// The container and port names of the rendered workloads.
const (
	// camundaContainer is the container of every unified process.
	camundaContainer = "camunda"
	// connectorsContainer is the container of the connectors runtime.
	connectorsContainer = "connectors"

	portNameGRPC       = "grpc"
	portNameHTTP       = "http"
	portNameManagement = "management"
	portNameCommand    = "command"
	portNameInternal   = "internal"
)

// The health endpoints (HealthConfigurationInitializer.java:58-61 for the
// unified binary on the management port; helm chart 14.8.3
// values.yaml:2383,2402 for connectors on its HTTP port).
const (
	healthStartupPath   = "/actuator/health/startup"
	healthReadinessPath = "/actuator/health/readiness"
	healthLivenessPath  = "/actuator/health/liveness"
)

// metricsPath is the Prometheus endpoint. The unified binary exposes every
// actuator endpoint on the management port and enables the prometheus one
// (dist/src/main/resources/application.properties:29,35-36); the connectors
// runtime exposes it under the same context path on its HTTP port (helm
// chart 14.8.3 templates/connectors/files/_application.yaml:12-17).
const metricsPath = "/actuator/prometheus"

// The probe timings of the unified processes.
const (
	startupFailureThreshold int32 = 60
	startupPeriodSeconds    int32 = 5
	readinessPeriodSeconds  int32 = 10
	livenessPeriodSeconds   int32 = 30
)

// camundaUID is the user and group of the camunda/camunda image
// (camunda.Dockerfile:154, USER 1001:1001) and of the connectors runtime
// (helm chart 14.8.3 values.yaml:2438 fsGroup, :2455 runAsUser).
const camundaUID int64 = 1001

// ProcessComponent is the ocf component of one process, together with the
// process it renders.
type ProcessComponent struct {
	Process   Process
	Component *component.Component
}

// Build returns one component per process, in Resolve order, for every
// process of the cluster. The component of a disabled process is gated off:
// its reconcile deletes the workload, the Service, and the ServiceMonitor
// that an earlier topology created and reports Disabled. Only the components
// of enabled processes take part in Ready. The admin Secret and the mirrored
// Secrets are separate components, see AdminSecretComponent and
// MirroredSecretComponent. The zeebe component also carries the
// ServiceAccount, so it exists before any pod references it: it is rendered
// when spec.serviceAccount is set or a referenced bucket derives a
// workload-identity annotation.
func Build(in Input) ([]ProcessComponent, error) {
	processes := Resolve(in.Effective)
	comps := make([]ProcessComponent, 0, len(processes))

	for _, p := range processes {
		var comp *component.Component
		var err error
		if p.Kind == ProcessStatefulSet {
			comp, err = zeebeComponent(in, p)
		} else {
			comp, err = deploymentComponent(in, p)
		}
		if err != nil {
			return nil, fmt.Errorf("building %s component: %w", p.Component, err)
		}
		comps = append(comps, ProcessComponent{Process: p, Component: comp})
	}

	return comps, nil
}

// BrokerClaimSelector is the label selector of the broker volume claims.
func BrokerClaimSelector(cluster *v1.CamundaCluster) map[string]string {
	return discoveryLabels(cluster, ComponentZeebe)
}

// managedLabels returns the labels of an object that the operator applies
// for a component.
func managedLabels(cluster *v1.CamundaCluster, comp string) map[string]string {
	return labels.Managed(labels.Cluster(cluster.Name), comp)
}

// discoveryLabels returns the labels of the pods, the volume claims, and the
// selectors of a component.
func discoveryLabels(cluster *v1.CamundaCluster, comp string) map[string]string {
	return labels.Discovery(labels.Cluster(cluster.Name), comp)
}

// zeebeComponent builds the brokers: the optional ServiceAccount, the
// StatefulSet, the headless Service, and the optional ServiceMonitor. It is
// gated on p.Enabled like every process component.
func zeebeComponent(in Input, p Process) (*component.Component, error) {
	account, err := serviceaccount.NewBuilder(serviceAccountFor(in)).Build()
	if err != nil {
		return nil, err
	}

	sts, err := statefulset.NewBuilder(zeebeStatefulSet(in, p)).
		WithMutation(statefulSetMutations(in, p)...).
		Build()
	if err != nil {
		return nil, err
	}

	svc, err := service.NewBuilder(serviceFor(in, p)).Build()
	if err != nil {
		return nil, err
	}

	monitor, err := servicemonitor.NewBuilder(serviceMonitorFor(in, p)).Build()
	if err != nil {
		return nil, err
	}

	return component.NewComponentBuilder().
		WithName(p.Component).
		WithConditionType(component.ConditionType(p.ConditionType)).
		WithFeatureGate(feature.NewBooleanGate(p.Enabled)).
		WithResource(account, component.GatedBy(feature.NewBooleanGate(serviceAccountRendered(in)))).
		WithResource(sts).
		WithResource(svc).
		IncludeWhen(in.ServiceMonitorSupported, func() component.Resource { return monitor }, monitoringGate(in)).
		Suspend(in.Effective.Suspend).
		Build()
}

// deploymentComponent builds a Deployment-backed process (the gateway, a web
// application, or connectors): the Deployment, its Service, and the optional
// ServiceMonitor. It is gated on p.Enabled, so an embedded or disabled
// process deletes its resources.
func deploymentComponent(in Input, p Process) (*component.Component, error) {
	workload, err := deployment.NewBuilder(deploymentFor(in, p)).
		WithMutation(deploymentMutations(in, p)...).
		Build()
	if err != nil {
		return nil, err
	}

	svc, err := service.NewBuilder(serviceFor(in, p)).Build()
	if err != nil {
		return nil, err
	}

	monitor, err := servicemonitor.NewBuilder(serviceMonitorFor(in, p)).Build()
	if err != nil {
		return nil, err
	}

	return component.NewComponentBuilder().
		WithName(p.Component).
		WithConditionType(component.ConditionType(p.ConditionType)).
		WithFeatureGate(feature.NewBooleanGate(p.Enabled)).
		WithResource(workload).
		WithResource(svc).
		IncludeWhen(in.ServiceMonitorSupported, func() component.Resource { return monitor }, monitoringGate(in)).
		Suspend(in.Effective.Suspend).
		Build()
}

// monitoringGate gates the ServiceMonitor on spec.monitoring.serviceMonitor.enabled.
func monitoringGate(in Input) component.ResourceOption {
	e := in.Effective
	enabled := e.Monitoring != nil && e.Monitoring.ServiceMonitor != nil && e.Monitoring.ServiceMonitor.Enabled
	return component.GatedBy(feature.NewBooleanGate(enabled))
}

// serviceAccountRendered reports whether the cluster needs a ServiceAccount
// of its own: the user asked for one, or a referenced bucket derives a
// workload-identity annotation that has to live somewhere.
func serviceAccountRendered(in Input) bool {
	return in.Effective.ServiceAccount != nil || len(in.ServiceAccountAnnotations) > 0
}

// serviceAccountFor renders the ServiceAccount of every workload pod: the
// workload-identity annotations that the referenced buckets derive, with the
// annotations of spec.serviceAccount merged over them, so an explicit user
// value on the same key wins.
func serviceAccountFor(in Input) *corev1.ServiceAccount {
	var user map[string]string
	if in.Effective.ServiceAccount != nil {
		user = in.Effective.ServiceAccount.Annotations
	}
	annotations := labels.Merge(in.ServiceAccountAnnotations, user)
	if len(annotations) == 0 {
		annotations = nil
	}

	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:        ServiceAccountName(in.Cluster),
			Namespace:   in.Cluster.Namespace,
			Labels:      managedLabels(in.Cluster, ComponentZeebe),
			Annotations: annotations,
		},
	}
}

// zeebeStatefulSet renders the base broker StatefulSet: parallel pod
// management, a rolling update, the data volume claim template with
// in.VolumeClaimSize (the effective size when unset), the requested storage
// size annotation, and the default retention policy (the volumes go with the
// cluster; a scale-down always retains). statefulSetMutations layer the
// overrides on top.
func zeebeStatefulSet(in Input, p Process) *appsv1.StatefulSet {
	e := in.Effective
	storageSize := e.StorageSize()
	claimSize := storageSize
	if in.VolumeClaimSize != nil {
		claimSize = *in.VolumeClaimSize
	}

	var storageClassName *string
	if e.Zeebe != nil {
		storageClassName = e.Zeebe.StorageClassName
	}

	template := podTemplate(in, p)
	template.Spec.Containers[0].VolumeMounts = append(
		[]corev1.VolumeMount{{Name: DataVolumeName, MountPath: DataMountPath}},
		template.Spec.Containers[0].VolumeMounts...,
	)

	return &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:        WorkloadName(in.Cluster, p.Component),
			Namespace:   in.Cluster.Namespace,
			Labels:      managedLabels(in.Cluster, p.Component),
			Annotations: map[string]string{RequestedStorageSizeAnnotation: storageSize.String()},
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas:            new(p.Replicas),
			Selector:            &metav1.LabelSelector{MatchLabels: discoveryLabels(in.Cluster, p.Component)},
			ServiceName:         WorkloadName(in.Cluster, p.Component),
			PodManagementPolicy: appsv1.ParallelPodManagement,
			UpdateStrategy: appsv1.StatefulSetUpdateStrategy{
				Type: appsv1.RollingUpdateStatefulSetStrategyType,
			},
			PersistentVolumeClaimRetentionPolicy: &appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
				WhenDeleted: appsv1.DeletePersistentVolumeClaimRetentionPolicyType,
				WhenScaled:  appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
			},
			Template: template,
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name:   DataVolumeName,
					Labels: discoveryLabels(in.Cluster, p.Component),
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes:      []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					StorageClassName: storageClassName,
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceStorage: claimSize},
					},
				},
			}},
		},
	}
}

// deploymentFor renders the base rolling-update Deployment of a process.
// deploymentMutations layer the overrides on top.
func deploymentFor(in Input, p Process) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WorkloadName(in.Cluster, p.Component),
			Namespace: in.Cluster.Namespace,
			Labels:    managedLabels(in.Cluster, p.Component),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(p.Replicas),
			Selector: &metav1.LabelSelector{MatchLabels: discoveryLabels(in.Cluster, p.Component)},
			Strategy: appsv1.DeploymentStrategy{Type: appsv1.RollingUpdateDeploymentStrategyType},
			Template: podTemplate(in, p),
		},
	}
}

// podTemplate renders the base pod template of a process: the discovery
// labels, the config hash annotation of the process, the container, the
// volumes of the rendered configuration, and the security context of the
// image user. The override surfaces (user pod metadata, resources,
// scheduling, the ServiceAccount) are mutations, see workloadMutations.
func podTemplate(in Input, p Process) corev1.PodTemplateSpec {
	r := render(in, p)

	return corev1.PodTemplateSpec{
		ObjectMeta: metav1.ObjectMeta{
			Labels:      labels.Merge(derivedPodLabels(in), discoveryLabels(in.Cluster, p.Component)),
			Annotations: map[string]string{ConfigHashAnnotation: configHash(in, p, r)},
		},
		Spec: corev1.PodSpec{
			Containers:      []corev1.Container{container(in, p, r)},
			Volumes:         r.volumes,
			SecurityContext: podSecurityContext(),
		},
	}
}

// podSecurityContext runs the pods as the image user 1001:1001, with the
// data volume owned by the same group.
func podSecurityContext() *corev1.PodSecurityContext {
	return &corev1.PodSecurityContext{
		RunAsUser:    new(camundaUID),
		RunAsGroup:   new(camundaUID),
		FSGroup:      new(camundaUID),
		RunAsNonRoot: new(true),
	}
}

// containerName returns the name of the container of a process: connectors
// for the connectors runtime, camunda for every unified process.
func containerName(p Process) string {
	if p.Component == ComponentConnectors {
		return connectorsContainer
	}
	return camundaContainer
}

// container renders the container of a process from the rendered
// configuration. The resources are a mutation, see workloadMutations.
func container(in Input, p Process, r rendered) corev1.Container {
	c := corev1.Container{
		Name:           containerName(p),
		Image:          Image(in, p),
		Command:        r.command,
		Env:            r.env,
		EnvFrom:        r.envFrom,
		Ports:          containerPorts(p),
		VolumeMounts:   r.mounts,
		StartupProbe:   probe(portNameManagement, healthStartupPath, startupPeriodSeconds, startupFailureThreshold),
		ReadinessProbe: probe(portNameManagement, healthReadinessPath, readinessPeriodSeconds, 0),
		LivenessProbe:  probe(portNameManagement, healthLivenessPath, livenessPeriodSeconds, 0),
	}

	if p.Component == ComponentConnectors {
		c.StartupProbe = nil
		c.ReadinessProbe = probe(portNameHTTP, healthReadinessPath, readinessPeriodSeconds, 0)
		c.LivenessProbe = probe(portNameHTTP, healthLivenessPath, livenessPeriodSeconds, 0)
	}

	return c
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

// namedPort is one port of a process, shared by the container and the
// Service.
type namedPort struct {
	name string
	port int32
}

// ports returns the ports of a process: connectors serve HTTP only; the
// brokers serve command, internal, and management, plus gRPC and HTTP with an
// embedded gateway; the gateway and web application processes serve HTTP and
// management, plus gRPC for the gateway.
func ports(p Process) []namedPort {
	if p.Component == ComponentConnectors {
		return []namedPort{{portNameHTTP, PortHTTP}}
	}

	var result []namedPort
	if p.ServesGRPC {
		result = append(result, namedPort{portNameGRPC, PortGRPC})
	}
	if p.ServesHTTP {
		result = append(result, namedPort{portNameHTTP, PortHTTP})
	}
	if p.Kind == ProcessStatefulSet {
		result = append(result, namedPort{portNameCommand, PortCommand}, namedPort{portNameInternal, PortInternal})
	}
	return append(result, namedPort{portNameManagement, PortManagement})
}

func containerPorts(p Process) []corev1.ContainerPort {
	named := ports(p)
	result := make([]corev1.ContainerPort, 0, len(named))
	for _, np := range named {
		result = append(
			result,
			corev1.ContainerPort{Name: np.name, ContainerPort: np.port, Protocol: corev1.ProtocolTCP},
		)
	}
	return result
}

func servicePorts(p Process) []corev1.ServicePort {
	named := ports(p)
	result := make([]corev1.ServicePort, 0, len(named))
	for _, np := range named {
		result = append(result, corev1.ServicePort{
			Name:       np.name,
			Port:       np.port,
			TargetPort: intstr.FromString(np.name),
			Protocol:   corev1.ProtocolTCP,
		})
	}
	return result
}

// serviceFor renders the Service of a process. The zeebe Service is headless
// and publishes not-ready addresses, so the brokers find each other through
// the contact points before they are ready.
func serviceFor(in Input, p Process) *corev1.Service {
	svc := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WorkloadName(in.Cluster, p.Component),
			Namespace: in.Cluster.Namespace,
			Labels:    managedLabels(in.Cluster, p.Component),
		},
		Spec: corev1.ServiceSpec{
			Selector: discoveryLabels(in.Cluster, p.Component),
			Ports:    servicePorts(p),
		},
	}

	if p.Kind == ProcessStatefulSet {
		svc.Spec.ClusterIP = corev1.ClusterIPNone
		svc.Spec.PublishNotReadyAddresses = true
	}

	return svc
}

// serviceMonitorFor renders the ServiceMonitor that scrapes
// /actuator/prometheus on the management port of a unified process, or on
// the HTTP port of connectors, through its Service.
func serviceMonitorFor(in Input, p Process) *monitoringv1.ServiceMonitor {
	var userLabels, annotations map[string]string
	if m := in.Effective.Monitoring; m != nil && m.ServiceMonitor != nil {
		userLabels = m.ServiceMonitor.Labels
		annotations = m.ServiceMonitor.Annotations
	}

	port := portNameManagement
	if p.Component == ComponentConnectors {
		port = portNameHTTP
	}

	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:        WorkloadName(in.Cluster, p.Component),
			Namespace:   in.Cluster.Namespace,
			Labels:      labels.Merge(userLabels, managedLabels(in.Cluster, p.Component)),
			Annotations: annotations,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector:  metav1.LabelSelector{MatchLabels: discoveryLabels(in.Cluster, p.Component)},
			Endpoints: []monitoringv1.Endpoint{{Port: port, Path: metricsPath}},
		},
	}
}
