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
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/editors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The declarative hibernation surface of CloudNativePG. The published api
// module declares no constant for any of them, so they are taken from
// https://cloudnative-pg.io/docs/devel/declarative_hibernation.
const (
	// HibernationAnnotation stops and restarts the instances of a Cluster.
	HibernationAnnotation = "cnpg.io/hibernation"
	// HibernationOn removes every instance pod and keeps the volume claims.
	HibernationOn = "on"
	// HibernationOff recreates the instance pods on the claims that were
	// kept. Removing the annotation does the same.
	HibernationOff = "off"
	// HibernationCondition is the condition CloudNativePG reports on a
	// Cluster it has hibernated. It turns True once every pod is gone.
	HibernationCondition = "cnpg.io/hibernation"
	// hibernationMutationName is the mutation NewBuilder registers to keep
	// the annotation owned by this operator.
	hibernationMutationName = "hibernation-off"
)

// hibernationOffMutation writes the off value on every Cluster this operator
// applies, so the annotation is a field it owns. Without it a hibernation
// somebody set by hand would keep the server down forever: the operator never
// declared the field, so server-side apply leaves it alone and no suspension
// state of the component contradicts it.
//
// NewBuilder registers it before anything a consumer adds, so a consumer that
// wants to drive the annotation itself can register a mutation that overrides
// it. Suspension does not need that: the ocf suspender runs after every
// feature mutation whatever their order.
func hibernationOffMutation() Mutation {
	return Mutation{
		Name: hibernationMutationName,
		Mutate: func(m *Mutator) error {
			m.SetHibernation(false)
			return nil
		},
	}
}

// SetHibernation records the hibernation annotation of the Cluster.
// CloudNativePG removes every instance pod while it is on and keeps the
// volume claims, so the data of the server outlives the suspension.
//
// Scaling to zero is not the alternative it looks like: the CloudNativePG
// schema puts a minimum of 1 on spec.instances, so the API server rejects
// the apply.
func (m *Mutator) SetHibernation(on bool) {
	value := HibernationOff
	if on {
		value = HibernationOn
	}

	m.EditObjectMetadata(func(e *editors.ObjectMetaEditor) error {
		e.EnsureAnnotation(HibernationAnnotation, value)
		return nil
	})
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
// Neither instance count answers the question: hibernation keeps the volume
// claim groups on purpose, and a pod that fails its probes is still running.
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
// condition covers several states: CloudNativePG has deferred the hibernation
// until the Cluster is healthy (WaitingForHealthy), it is waiting for the pods
// to go (WaitingPodsDeletion), or it is deleting them (DeletingPods). Only its
// own words separate them.
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
