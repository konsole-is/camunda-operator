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

import "github.com/sourcehawk/operator-component-framework/pkg/mutation/editors"

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
	// HibernationConditionReason is the reason of that condition.
	HibernationConditionReason = "Hibernated"
	// hibernationMutationName is the mutation NewBuilder registers to keep
	// the annotation owned by this operator. It is the first mutation of
	// every Cluster, so a suspension applied afterwards wins.
	hibernationMutationName = "hibernation-off"
)

// hibernationOffMutation writes the off value on every Cluster this operator
// applies, so the annotation is a field it owns. Without it a hibernation
// somebody set by hand would keep the server down forever: the operator never
// declared the field, so server-side apply leaves it alone and no suspension
// state of the component contradicts it.
//
// The ocf suspender runs after every feature mutation, so a suspended
// component still ends with the on value.
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
