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

	if converged(cluster) {
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

// converged reports whether CloudNativePG holds the Cluster in the state the
// spec asks for. The count has to match exactly: during a scale-down
// CloudNativePG reports more ready pods than the spec wants, which is a
// Cluster still converging, not one that has arrived.
//
// DefaultConvergingStatusHandler and DefaultGraceStatusHandler share this
// test. A grace handler that answered Healthy for a state convergence did not
// would make the component log an inconsistency on every reconcile.
func converged(cluster *cnpgv1.Cluster) bool {
	return cluster.Status.Phase == cnpgv1.PhaseHealthy &&
		cluster.Status.ReadyInstances == cluster.Spec.Instances
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
	case converged(cluster):
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

// DefaultSuspensionStatusHandler reports Suspended only once CloudNativePG
// reports the hibernation condition True. Anything else is Suspending.
//
// Neither instance count answers the question. status.instances counts the
// volume claim groups of the server, which hibernation keeps on purpose, so
// it never drains. status.readyInstances reaching zero says the pods stopped
// passing their probes, not that they are gone, and CloudNativePG also defers
// hibernation while the Cluster is unhealthy. Only the condition states that
// the pods are actually down.
func DefaultSuspensionStatusHandler(cluster *cnpgv1.Cluster) (concepts.SuspensionStatusWithReason, error) {
	condition := meta.FindStatusCondition(cluster.Status.Conditions, HibernationCondition)

	if condition != nil && condition.Status == metav1.ConditionTrue {
		return concepts.SuspensionStatusWithReason{
			Status: concepts.SuspensionStatusSuspended,
			Reason: "CloudNativePG reports the Cluster as hibernated",
		}, nil
	}

	return concepts.SuspensionStatusWithReason{
		Status: concepts.SuspensionStatusSuspending,
		Reason: fmt.Sprintf(
			"%s; %d of %d instances are still ready",
			hibernationText(condition), cluster.Status.ReadyInstances, cluster.Spec.Instances,
		),
	}, nil
}

// hibernationText says where the hibernation stands, for a status reason. It
// carries the reason and the message of the condition, because a False
// condition covers two different states: CloudNativePG is still deleting the
// pods, and CloudNativePG has deferred the hibernation because the Cluster is
// not healthy. Only its own words separate them.
func hibernationText(condition *metav1.Condition) string {
	if condition == nil {
		return "CloudNativePG has not reported the hibernation condition yet"
	}

	const prefix = "CloudNativePG has not hibernated the Cluster yet"

	switch {
	case condition.Reason != "" && condition.Message != "":
		return fmt.Sprintf("%s (%s: %s)", prefix, condition.Reason, condition.Message)
	case condition.Reason != "":
		return fmt.Sprintf("%s (%s)", prefix, condition.Reason)
	case condition.Message != "":
		return fmt.Sprintf("%s (%s)", prefix, condition.Message)
	default:
		return prefix
	}
}

// DefaultDeleteOnSuspendHandler keeps the Cluster on suspension. Deleting it
// would hand the volumes of the server to the reclaim policy of their storage
// class, and suspension exists to keep the data.
func DefaultDeleteOnSuspendHandler(_ *cnpgv1.Cluster) bool {
	return false
}
