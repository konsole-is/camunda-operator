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

package cnpgcluster

import (
	"fmt"
	"strconv"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
)

// The handlers in this file and in hibernation.go are the kind-specific
// status logic NewBuilder registers by default; they replace the scaffolded
// defaults the ocf generator writes into builder.go. After regenerating the
// wrapper with --force, delete the scaffolded Default* handlers from
// builder.go again so these implementations take their place.

// failingPhases are the phases CloudNativePG reports for a Cluster that no
// longer converges on its own. Every other phase is a step of a reconcile
// that still runs, including "Failing over", which is a failover in progress
// rather than a failure.
var failingPhases = map[string]struct{}{
	cnpgv1.PhaseUnrecoverable:              {},
	cnpgv1.PhaseUnknownPlugin:              {},
	cnpgv1.PhaseFailurePlugin:              {},
	cnpgv1.PhaseImageCatalogError:          {},
	cnpgv1.PhaseCannotCreateClusterObjects: {},
	cnpgv1.PhaseDefinitionInvalid:          {},
	cnpgv1.PhaseArchitectureBinaryMissing:  {},
	cnpgv1.PhaseWaitingForUser:             {},
}

// DefaultConvergingStatusHandler maps the phase CloudNativePG reports to the
// component convergence state. The Cluster is Healthy when the phase is
// PhaseHealthy and every instance the spec asks for is ready, Failing on a
// phase of failingPhases, Creating while the Cluster has no phase on its
// first apply, and Updating otherwise.
func DefaultConvergingStatusHandler(
	op concepts.ConvergingOperation, cluster *cnpgv1.Cluster,
) (concepts.AliveStatusWithReason, error) {
	if phase, failing := Failing(cluster); failing {
		return concepts.AliveStatusWithReason{
			Status: concepts.AliveConvergingStatusFailing,
			Reason: fmt.Sprintf("CloudNativePG reports %q", phase),
		}, nil
	}

	if Converged(cluster) {
		return concepts.AliveStatusWithReason{
			Status: concepts.AliveConvergingStatusHealthy,
			Reason: fmt.Sprintf("%d of %d instances are ready", cluster.Status.ReadyInstances, cluster.Spec.Instances),
		}, nil
	}

	if cluster.Status.Phase == "" && op == concepts.ConvergingOperationCreated {
		return concepts.AliveStatusWithReason{
			Status: concepts.AliveConvergingStatusCreating,
			Reason: "CloudNativePG has not reported a phase yet",
		}, nil
	}

	return concepts.AliveStatusWithReason{
		Status: concepts.AliveConvergingStatusUpdating,
		Reason: fmt.Sprintf(
			"CloudNativePG reports %s with %d of %d instances ready",
			phaseText(cluster.Status.Phase), cluster.Status.ReadyInstances, cluster.Spec.Instances,
		),
	}, nil
}

// Converged reports whether CloudNativePG holds the Cluster in the state the
// spec asks for. The converging handler and the grace handler share it, so the
// two can never disagree about which states are healthy. The count has to
// match exactly, because a scale-down reports more ready pods than the spec
// wants.
func Converged(cluster *cnpgv1.Cluster) bool {
	return cluster.Status.Phase == cnpgv1.PhaseHealthy &&
		cluster.Status.ReadyInstances == cluster.Spec.Instances
}

// Failing reports whether CloudNativePG holds the Cluster in a phase it no
// longer converges out of on its own, and names that phase. A caller that
// drives a Cluster outside a component, for example the one a recovery
// builds, grades it with this and with Converged, so its reading of a phase
// is the reading of the status handlers here.
func Failing(cluster *cnpgv1.Cluster) (string, bool) {
	_, failing := failingPhases[cluster.Status.Phase]

	return cluster.Status.Phase, failing
}

// phaseText renders a phase for a status reason. An empty phase means
// CloudNativePG has not written one, which reads better as words than as a
// pair of empty quotes.
func phaseText(phase string) string {
	if phase == "" {
		return "no phase yet"
	}

	return strconv.Quote(phase)
}

// DefaultGraceStatusHandler grades a Cluster that is still not converged when
// the grace period of the component expires: a converged Cluster is Healthy,
// at least one ready instance is Degraded, because PostgreSQL still serves
// through the read-write service, and no ready instance is Down.
func DefaultGraceStatusHandler(cluster *cnpgv1.Cluster) (concepts.GraceStatusWithReason, error) {
	ready := cluster.Status.ReadyInstances

	switch {
	case Converged(cluster):
		return concepts.GraceStatusWithReason{
			Status: concepts.GraceStatusHealthy,
			Reason: fmt.Sprintf("%d of %d instances are ready", ready, cluster.Spec.Instances),
		}, nil
	case ready > 0:
		return concepts.GraceStatusWithReason{
			Status: concepts.GraceStatusDegraded,
			Reason: fmt.Sprintf(
				"%d of %d instances are ready; CloudNativePG reports %s",
				ready, cluster.Spec.Instances, phaseText(cluster.Status.Phase),
			),
		}, nil
	default:
		return concepts.GraceStatusWithReason{
			Status: concepts.GraceStatusDown,
			Reason: fmt.Sprintf(
				"no instance is ready; CloudNativePG reports %s", phaseText(cluster.Status.Phase),
			),
		}, nil
	}
}
