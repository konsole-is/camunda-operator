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

package databaseserver

import (
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/podmonitor"
)

// podMonitorSuffix appended to the server name yields the name of the
// PodMonitor that scrapes the instance pods.
const podMonitorSuffix = "-metrics"

// MonitoringComponent builds the monitoring component: the PodMonitor that
// scrapes the metrics endpoint of every instance pod. CloudNativePG serves
// those metrics itself, so nothing else is deployed. The component is
// feature-gated on spec.monitoring.podMonitor.enabled: disabled, it deletes the
// PodMonitor and reports Disabled. podMonitorSupported reports whether the
// cluster serves the PodMonitor kind. When it is false the resource is left
// out, and the component reports ready with nothing to do.
//
// The PodMonitor is derived by name and blocks on a foreign controller, so one
// that another owner already controls under that name is neither rewritten nor
// removed, and MonitoringReady names that owner.
func MonitoringComponent(
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	podMonitorSupported bool,
	clusterTaken string,
) (*component.Component, error) {
	monitor, err := podmonitor.NewBuilder(podMonitor(server, merged)).Build()
	if err != nil {
		return nil, err
	}

	// The PodMonitor is included only where the cluster serves the kind and
	// never deleted by inclusion alone: when the CRD goes away, the API server
	// removes every instance, and a delete against a missing kind fails.
	return component.NewComponentBuilder().
		WithName("monitoring").
		WithConditionType(v1.ConditionMonitoringReady).
		WithFeatureGate(feature.NewBooleanGate(MonitoringEnabled(merged) && clusterTaken == "")).
		IncludeWhen(
			podMonitorSupported,
			func() component.Resource { return monitor },
			component.BlockOnForeignController(),
		).
		Suspend(merged.Suspend).
		Build()
}

// podMonitor renders the PodMonitor over the instance pods. CloudNativePG
// labels every pod of a cluster with the cluster name and serves the metrics on
// a named port, so the selector needs nothing this operator applies itself.
func podMonitor(server *v1.DatabaseServer, merged v1.DatabaseServerSpec) *monitoringv1.PodMonitor {
	var userLabels, annotations map[string]string
	var interval monitoringv1.Duration
	if merged.Monitoring != nil && merged.Monitoring.PodMonitor != nil {
		userLabels = merged.Monitoring.PodMonitor.Labels
		annotations = merged.Monitoring.PodMonitor.Annotations
		interval = monitoringv1.Duration(merged.Monitoring.PodMonitor.Interval)
	}

	return &monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{
			Name:        PodMonitorName(server),
			Namespace:   server.Namespace,
			Labels:      labels.Merge(userLabels, managedLabels(server)),
			Annotations: annotations,
		},
		Spec: monitoringv1.PodMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{CNPGClusterNameLabel: ClusterName(server)},
			},
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{
				Port:     new(MetricsPortName),
				Interval: interval,
			}},
		},
	}
}

// PodMonitorName returns the name of the PodMonitor that scrapes the instance
// pods of the server.
func PodMonitorName(server *v1.DatabaseServer) string {
	return server.Name + podMonitorSuffix
}

// MonitoringEnabled reports whether the spec asks for Prometheus scraping.
func MonitoringEnabled(merged v1.DatabaseServerSpec) bool {
	return merged.Monitoring != nil &&
		merged.Monitoring.PodMonitor != nil &&
		merged.Monitoring.PodMonitor.Enabled
}
