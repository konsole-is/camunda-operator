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

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
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
			"CloudNativePG reports %q with %d of %d instances ready",
			cluster.Status.Phase, cluster.Status.ReadyInstances, cluster.Spec.Instances,
		),
	}, nil
}

// DefaultGraceStatusHandler grades a Cluster that is still not converged when
// the grace period of the component expires: every instance ready is Healthy,
// at least one ready instance is Degraded, because PostgreSQL still serves
// through the read-write service, and no ready instance is Down.
func DefaultGraceStatusHandler(cluster *cnpgv1.Cluster) (concepts.GraceStatusWithReason, error) {
	ready := cluster.Status.ReadyInstances

	switch {
	case ready >= cluster.Spec.Instances:
		return concepts.GraceStatusWithReason{
			Status: concepts.GraceStatusHealthy,
			Reason: fmt.Sprintf("%d of %d instances are ready", ready, cluster.Spec.Instances),
		}, nil
	case ready > 0:
		return concepts.GraceStatusWithReason{
			Status: concepts.GraceStatusDegraded,
			Reason: fmt.Sprintf("%d of %d instances are ready", ready, cluster.Spec.Instances),
		}, nil
	default:
		return concepts.GraceStatusWithReason{
			Status: concepts.GraceStatusDown,
			Reason: fmt.Sprintf("no instance is ready; CloudNativePG reports %q", cluster.Status.Phase),
		}, nil
	}
}

// DefaultSuspendMutationHandler scales the Cluster to zero instances.
// CloudNativePG removes the pods and keeps the PersistentVolumeClaims, so the
// data of the server survives the suspension and the instances come back on
// the claims they had.
func DefaultSuspendMutationHandler(m *Mutator) error {
	m.SetInstances(0)
	return nil
}

// DefaultSuspensionStatusHandler reports Suspended once CloudNativePG has
// scaled the Cluster to zero instances, and Suspending while any instance is
// still counted.
func DefaultSuspensionStatusHandler(cluster *cnpgv1.Cluster) (concepts.SuspensionStatusWithReason, error) {
	if cluster.Status.Instances > 0 || cluster.Status.ReadyInstances > 0 {
		return concepts.SuspensionStatusWithReason{
			Status: concepts.SuspensionStatusSuspending,
			Reason: fmt.Sprintf("%d instances are still running", cluster.Status.Instances),
		}, nil
	}

	return concepts.SuspensionStatusWithReason{
		Status: concepts.SuspensionStatusSuspended,
		Reason: "the Cluster is scaled to zero instances",
	}, nil
}

// DefaultDeleteOnSuspendHandler keeps the Cluster on suspension. Deleting it
// would hand the volumes of the server to the reclaim policy of their storage
// class, and suspension exists to keep the data.
func DefaultDeleteOnSuspendHandler(_ *cnpgv1.Cluster) bool {
	return false
}
