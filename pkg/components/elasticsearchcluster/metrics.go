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

package elasticsearchcluster

import (
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
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/servicemonitor"
)

const (
	// ConditionMetrics is the condition type of the metrics component.
	ConditionMetrics = "MetricsReady"

	// DefaultExporterImage is the elasticsearch_exporter release that the
	// operator deploys unless spec.monitoring.exporter.image overrides it.
	DefaultExporterImage = "quay.io/prometheuscommunity/elasticsearch-exporter:v1.9.0"

	// exporterComponentLabel is the labels.ComponentKey value on the exporter
	// Deployment, its Service, and the ServiceMonitor that selects them.
	exporterComponentLabel = "elasticsearch-exporter"
	// exporterSuffix appended to the CR name yields the exporter Deployment
	// name.
	exporterSuffix = "-es-exporter"
	// metricsServiceSuffix appended to the CR name yields the name of the
	// Service that exposes the exporter and that the ServiceMonitor scrapes.
	metricsServiceSuffix = "-es-metrics"
	// exporterContainerName is the exporter container.
	exporterContainerName = "exporter"
	// metricsPortName and metricsPort are the port that elasticsearch_exporter
	// serves /metrics on.
	metricsPortName = "metrics"
	metricsPort     = 9114
	// caMountPath is where the exporter reads the CA that ECK publishes.
	caMountPath = "/etc/elasticsearch/ca"
	// exporterUsernameEnv and exporterPasswordEnv are the environment
	// variables that elasticsearch_exporter reads its basic auth from.
	exporterUsernameEnv = "ES_USERNAME"
	exporterPasswordEnv = "ES_PASSWORD"
)

// MonitoringEnabled reports whether the spec asks for Prometheus scraping.
func MonitoringEnabled(merged v1.ElasticsearchClusterSpec) bool {
	return merged.Monitoring != nil &&
		merged.Monitoring.ServiceMonitor != nil &&
		merged.Monitoring.ServiceMonitor.Enabled
}

// MetricsComponent builds the metrics component: the elasticsearch_exporter
// Deployment, the Service that exposes its metrics port, and the
// ServiceMonitor that scrapes that Service. Elasticsearch serves no Prometheus
// endpoint itself, so the exporter is what Prometheus scrapes. The component
// is feature-gated on spec.monitoring.serviceMonitor.enabled: disabled, it
// deletes its resources and reports Disabled. serviceMonitorSupported reports
// whether the cluster serves the ServiceMonitor kind. When it is false, the
// ServiceMonitor is omitted and the exporter still runs.
func MetricsComponent(
	cluster *v1.ElasticsearchCluster,
	merged v1.ElasticsearchClusterSpec,
	serviceMonitorSupported bool,
) (*component.Component, error) {
	exporter, err := deployment.NewBuilder(exporterDeployment(cluster, merged)).Build()
	if err != nil {
		return nil, err
	}

	metrics, err := service.NewBuilder(metricsService(cluster)).Build()
	if err != nil {
		return nil, err
	}

	monitor, err := servicemonitor.NewBuilder(serviceMonitor(cluster, merged)).Build()
	if err != nil {
		return nil, err
	}

	// The ServiceMonitor is included only where the cluster serves the kind
	// and never deleted by inclusion alone: when the CRD goes away, the API
	// server removes every instance, and a delete against a missing kind
	// fails.
	return component.NewComponentBuilder().
		WithName("metrics").
		WithConditionType(ConditionMetrics).
		WithFeatureGate(feature.NewBooleanGate(MonitoringEnabled(merged))).
		WithResource(exporter).
		WithResource(metrics).
		IncludeWhen(serviceMonitorSupported, func() component.Resource { return monitor }).
		Suspend(merged.Suspend).
		Build()
}

// exporterLabels returns the labels of the exporter Deployment and its
// Service. They differ from the Elasticsearch labels in the component, so the
// ServiceMonitor selects the exporter and not the Elasticsearch pods.
func exporterLabels(cluster *v1.ElasticsearchCluster) map[string]string {
	return labels.Managed(labels.ElasticsearchCluster(cluster.Name), exporterComponentLabel)
}

// exporterSelector returns the labels that the Deployment and the Service
// select the exporter pods by, and that the ServiceMonitor selects the Service
// by. A selector is immutable on a Deployment, so it carries only the owner
// and component.
func exporterSelector(cluster *v1.ElasticsearchCluster) map[string]string {
	return labels.Discovery(labels.ElasticsearchCluster(cluster.Name), exporterComponentLabel)
}

// exporterDeployment renders the elasticsearch_exporter Deployment. It talks
// to the ECK HTTPS service with the Camunda user from the file-realm Secret,
// checks TLS against the CA that ECK publishes, and serves metrics in plain
// HTTP on the metrics port.
func exporterDeployment(cluster *v1.ElasticsearchCluster, merged v1.ElasticsearchClusterSpec) *appsv1.Deployment {
	image := DefaultExporterImage
	var resources corev1.ResourceRequirements
	if merged.Monitoring != nil && merged.Monitoring.Exporter != nil {
		if merged.Monitoring.Exporter.Image != "" {
			image = merged.Monitoring.Exporter.Image
		}
		if merged.Monitoring.Exporter.Resources != nil {
			resources = *merged.Monitoring.Exporter.Resources
		}
	}

	secretEnv := func(name, key string) corev1.EnvVar {
		return corev1.EnvVar{
			Name: name,
			ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: UserSecretName(cluster)},
				Key:                  key,
			}},
		}
	}

	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + exporterSuffix,
			Namespace: cluster.Namespace,
			Labels:    exporterLabels(cluster),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(1)),
			Selector: &metav1.LabelSelector{MatchLabels: exporterSelector(cluster)},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: exporterLabels(cluster)},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  exporterContainerName,
						Image: image,
						Args: []string{
							"--es.uri=https://" + cluster.Name + httpServiceSuffix + "." + cluster.Namespace + ".svc:9200",
							"--es.ca=" + caMountPath + "/" + caCertKey,
							"--es.timeout=30s",
						},
						Env: []corev1.EnvVar{
							secretEnv(exporterUsernameEnv, usernameKey),
							secretEnv(exporterPasswordEnv, PasswordKey),
						},
						Ports: []corev1.ContainerPort{{
							Name:          metricsPortName,
							ContainerPort: metricsPort,
							Protocol:      corev1.ProtocolTCP,
						}},
						Resources: resources,
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "ca",
							MountPath: caMountPath,
							ReadOnly:  true,
						}},
						SecurityContext: &corev1.SecurityContext{
							AllowPrivilegeEscalation: new(false),
							ReadOnlyRootFilesystem:   new(true),
							RunAsNonRoot:             new(true),
							Capabilities:             &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}},
						},
					}},
					Volumes: []corev1.Volume{{
						Name: "ca",
						VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
							SecretName: cluster.Name + certsSecretSuffix,
							Items:      []corev1.KeyToPath{{Key: caCertKey, Path: caCertKey}},
						}},
					}},
				},
			},
		},
	}
}

// metricsService renders the Service that exposes the exporter metrics port.
func metricsService(cluster *v1.ElasticsearchCluster) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + metricsServiceSuffix,
			Namespace: cluster.Namespace,
			Labels:    exporterLabels(cluster),
		},
		Spec: corev1.ServiceSpec{
			Selector: exporterSelector(cluster),
			Ports: []corev1.ServicePort{{
				Name:       metricsPortName,
				Port:       metricsPort,
				TargetPort: intstr.FromString(metricsPortName),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}
}

// serviceMonitor renders the ServiceMonitor that scrapes the metrics Service.
// The exporter serves plain HTTP inside the cluster, so the endpoint needs no
// auth and no TLS.
func serviceMonitor(cluster *v1.ElasticsearchCluster, merged v1.ElasticsearchClusterSpec) *monitoringv1.ServiceMonitor {
	var userLabels, annotations map[string]string
	if merged.Monitoring != nil && merged.Monitoring.ServiceMonitor != nil {
		userLabels = merged.Monitoring.ServiceMonitor.Labels
		annotations = merged.Monitoring.ServiceMonitor.Annotations
	}

	return &monitoringv1.ServiceMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:        cluster.Name + metricsServiceSuffix,
			Namespace:   cluster.Namespace,
			Labels:      labels.Merge(userLabels, exporterLabels(cluster)),
			Annotations: annotations,
		},
		Spec: monitoringv1.ServiceMonitorSpec{
			Selector:  metav1.LabelSelector{MatchLabels: exporterSelector(cluster)},
			Endpoints: []monitoringv1.Endpoint{{Port: metricsPortName}},
		},
	}
}
