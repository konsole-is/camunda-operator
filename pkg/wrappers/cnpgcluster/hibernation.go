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
)

// SetHibernation records the hibernation annotation of the Cluster.
// CloudNativePG removes every instance pod while it is on and keeps the
// volume claims, so the data of the server outlives the suspension.
//
// A resume needs no call: the resumed object no longer declares the
// annotation, and server-side apply removes a field this operator applied
// before. Pass false only to overwrite an annotation that somebody set by
// hand.
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
