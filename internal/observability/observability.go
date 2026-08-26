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

// Package observability owns the Prometheus collectors of the framework
// metrics and builds the recorder that each controller feeds them through.
// The collectors are registered once per process with the registry that the
// manager serves on /metrics.
package observability

import (
	ocm "github.com/sourcehawk/go-crd-condition-metrics/pkg/crd-condition-metrics"
	"github.com/sourcehawk/operator-component-framework/pkg/metrics"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// MetricNamespace prefixes the condition gauge, so it is served as
// camunda_operator_controller_condition. The dashboards and alert rules under
// config/prometheus are rendered for this value; `make observability`
// re-renders them.
const MetricNamespace = "camunda_operator"

var (
	conditions = ocm.NewOperatorConditionsGauge(MetricNamespace)
	collectors = metrics.NewCollectors()
)

func init() {
	ctrlmetrics.Registry.MustRegister(conditions, collectors)
}

// Recorder returns the metrics recorder of one controller for its
// ReconcileContext. controller is the `controller` label of every series the
// recorder emits and must equal the name the controller registers with
// controller-runtime through Named, or the shipped dashboards cannot join the
// framework's series with the reconcile series of the same controller.
func Recorder(controller string) *metrics.Recorder {
	return metrics.NewRecorder(controller, conditions, collectors)
}
