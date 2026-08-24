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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The handlers in this file are the kind-specific status logic NewBuilder
// registers by default; they replace the scaffolded defaults the ocf
// generator writes into builder.go. After regenerating the wrapper with
// --force, delete the scaffolded Default* handlers from builder.go again so
// these implementations take their place.

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
	if _, failing := failingPhases[cluster.Status.Phase]; failing {
		return concepts.AliveStatusWithReason{
			Status: concepts.AliveConvergingStatusFailing,
			Reason: fmt.Sprintf("CloudNativePG reports %q", cluster.Status.Phase),
		}, nil
	}

	if cluster.Status.Phase == cnpgv1.PhaseHealthy && cluster.Status.ReadyInstances == cluster.Spec.Instances {
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
// the grace period of the component expires: a healthy phase with every
// instance ready is Healthy, at least one ready instance is Degraded, because
// PostgreSQL still serves through the read-write service, and no ready
// instance is Down.
//
// The phase is part of the Healthy test because the framework calls this
// handler only for a state DefaultConvergingStatusHandler did not call
// Healthy. Reading the ready count alone would call a Cluster in a failing
// phase healthy, which contradicts convergence and makes the component log an
// inconsistency on every reconcile.
func DefaultGraceStatusHandler(cluster *cnpgv1.Cluster) (concepts.GraceStatusWithReason, error) {
	ready := cluster.Status.ReadyInstances

	switch {
	case cluster.Status.Phase == cnpgv1.PhaseHealthy && ready >= cluster.Spec.Instances:
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

// DefaultSuspendMutationHandler hibernates the Cluster. CloudNativePG
// removes the instance pods and keeps the volume claims, so the data of the
// server survives the suspension and the instances come back on the claims
// they had.
func DefaultSuspendMutationHandler(m *Mutator) error {
	m.SetHibernation(true)
	return nil
}

// DefaultSuspensionStatusHandler reports Suspended once CloudNativePG has
// hibernated the Cluster: the hibernation condition is True, or, before
// CloudNativePG writes that condition, no instance is ready. Anything else is
// Suspending.
//
// The count of status.instances is not part of the answer. It counts the
// volume claim groups of the server, which hibernation keeps on purpose, so
// it never drains.
func DefaultSuspensionStatusHandler(cluster *cnpgv1.Cluster) (concepts.SuspensionStatusWithReason, error) {
	condition := meta.FindStatusCondition(cluster.Status.Conditions, HibernationCondition)

	switch {
	case condition != nil && condition.Status == metav1.ConditionTrue:
		return concepts.SuspensionStatusWithReason{
			Status: concepts.SuspensionStatusSuspended,
			Reason: "CloudNativePG reports the Cluster as hibernated",
		}, nil
	case condition == nil && cluster.Status.ReadyInstances == 0:
		return concepts.SuspensionStatusWithReason{
			Status: concepts.SuspensionStatusSuspended,
			Reason: "no instance of the Cluster is ready",
		}, nil
	default:
		return concepts.SuspensionStatusWithReason{
			Status: concepts.SuspensionStatusSuspending,
			Reason: fmt.Sprintf(
				"CloudNativePG is deleting the instance pods; %d of %d are still ready",
				cluster.Status.ReadyInstances, cluster.Spec.Instances,
			),
		}, nil
	}
}

// DefaultDeleteOnSuspendHandler keeps the Cluster on suspension. Deleting it
// would hand the volumes of the server to the reclaim policy of their storage
// class, and suspension exists to keep the data.
func DefaultDeleteOnSuspendHandler(_ *cnpgv1.Cluster) bool {
	return false
}
