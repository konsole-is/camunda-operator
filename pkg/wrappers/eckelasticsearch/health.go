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

package eckelasticsearch

import (
	"fmt"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
)

// The handlers in this file are the kind-specific status logic NewBuilder
// registers by default; they replace the scaffolded defaults the ocf
// generator writes into builder.go. After regenerating the wrapper with
// --force, delete the scaffolded Default* handlers from builder.go again so
// these implementations take their place.

// DefaultConvergingStatusHandler maps ECK's reported cluster health to the
// component convergence state: green is Healthy, yellow is still converging
// (Creating on first apply, Updating otherwise), and red is Failing. A
// missing or unknown health is Creating only on the first apply, when ECK has
// simply not formed the cluster yet; on any later reconcile ECK also reports
// unknown when it loses contact with a formed cluster, so it maps to Failing,
// mirroring the grace handler's unknown-to-Down.
func DefaultConvergingStatusHandler(
	op concepts.ConvergingOperation, es *esv1.Elasticsearch,
) (concepts.AliveStatusWithReason, error) {
	switch es.Status.Health {
	case esv1.ElasticsearchGreenHealth:
		return concepts.AliveStatusWithReason{
			Status: concepts.AliveConvergingStatusHealthy,
			Reason: "Elasticsearch reports green health",
		}, nil
	case esv1.ElasticsearchYellowHealth:
		status := concepts.AliveConvergingStatusUpdating
		if op == concepts.ConvergingOperationCreated {
			status = concepts.AliveConvergingStatusCreating
		}
		return concepts.AliveStatusWithReason{
			Status: status,
			Reason: "Elasticsearch reports yellow health while converging",
		}, nil
	case esv1.ElasticsearchRedHealth:
		return concepts.AliveStatusWithReason{
			Status: concepts.AliveConvergingStatusFailing,
			Reason: "Elasticsearch reports red health",
		}, nil
	default:
		if op == concepts.ConvergingOperationCreated {
			return concepts.AliveStatusWithReason{
				Status: concepts.AliveConvergingStatusCreating,
				Reason: "Elasticsearch has not reported health yet",
			}, nil
		}
		return concepts.AliveStatusWithReason{
			Status: concepts.AliveConvergingStatusFailing,
			Reason: fmt.Sprintf("Elasticsearch reports %q health past its first apply", es.Status.Health),
		}, nil
	}
}

// DefaultGraceStatusHandler grades a cluster that is still not converged
// after the grace period: green is Healthy, yellow is Degraded (some replicas
// unassigned but serving), and red or unreported health is Down.
func DefaultGraceStatusHandler(es *esv1.Elasticsearch) (concepts.GraceStatusWithReason, error) {
	switch es.Status.Health {
	case esv1.ElasticsearchGreenHealth:
		return concepts.GraceStatusWithReason{
			Status: concepts.GraceStatusHealthy,
			Reason: "Elasticsearch reports green health",
		}, nil
	case esv1.ElasticsearchYellowHealth:
		return concepts.GraceStatusWithReason{
			Status: concepts.GraceStatusDegraded,
			Reason: "Elasticsearch reports yellow health",
		}, nil
	default:
		return concepts.GraceStatusWithReason{
			Status: concepts.GraceStatusDown,
			Reason: fmt.Sprintf("Elasticsearch reports %q health", es.Status.Health),
		}, nil
	}
}

// DefaultSuspendMutationHandler makes sure that the applied Elasticsearch CR
// retains its data volumes when it is deleted: it sets the volume claim delete
// policy to DeleteOnScaledownOnly. Suspension deletes the CR, because ECK
// removes the PersistentVolumeClaim of every node that it scales away, under
// either policy. Scaling the node sets to zero would therefore erase the data
// that suspension must keep. The rendered CR carries the policy that the
// retention policy of the owner asks for, so this mutation is the step that
// switches a cluster with a Delete policy to retention before the deletion.
func DefaultSuspendMutationHandler(m *Mutator) error {
	m.RetainVolumesOnDeletion()
	return nil
}

// DefaultSuspensionStatusHandler reports Suspended when the applied
// Elasticsearch CR can be deleted without data loss: it carries
// DeleteOnScaledownOnly, ECK has observed the current generation, so the
// policy is in effect, and no data migration is in progress. Otherwise it
// reports PendingSuspension with the reason. The component then deletes the
// CR. On the next reconcile the CR is absent, and the component reports
// Suspended on its own.
func DefaultSuspensionStatusHandler(es *esv1.Elasticsearch) (concepts.SuspensionStatusWithReason, error) {
	if es.Spec.VolumeClaimDeletePolicy != esv1.DeleteOnScaledownOnlyPolicy {
		return concepts.SuspensionStatusWithReason{
			Status: concepts.SuspensionStatusPending,
			Reason: "Elasticsearch volume claim delete policy is not yet DeleteOnScaledownOnly",
		}, nil
	}

	if es.Status.ObservedGeneration < es.Generation {
		return concepts.SuspensionStatusWithReason{
			Status: concepts.SuspensionStatusPending,
			Reason: "Elasticsearch status does not yet reflect the current generation",
		}, nil
	}

	if es.Status.Phase == esv1.ElasticsearchMigratingDataPhase {
		return concepts.SuspensionStatusWithReason{
			Status: concepts.SuspensionStatusPending,
			Reason: "Elasticsearch is migrating data",
		}, nil
	}

	return concepts.SuspensionStatusWithReason{
		Status: concepts.SuspensionStatusSuspended,
		Reason: "Elasticsearch can be deleted; DeleteOnScaledownOnly retains the data volumes",
	}, nil
}

// DefaultDeleteOnSuspendHandler deletes the Elasticsearch CR on suspension.
// ECK retains the data volumes under DeleteOnScaledownOnly, and re-creation
// of the CR reattaches them. This is the way that Elastic documents to stop a
// cluster and keep its data.
func DefaultDeleteOnSuspendHandler(_ *esv1.Elasticsearch) bool {
	return true
}
