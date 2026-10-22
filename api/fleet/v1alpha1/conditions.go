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

package v1alpha1

// The condition vocabulary of the fleet group. Ready and its reasons Healthy
// and InvalidReference come from api/v1. While a live claim waits, ocf
// reports PrerequisiteNotMet or OperationPending, so this group has no reason
// of its own for that wait.
const (
	// ReasonNoEnvironment means that the namespace of a claim has no
	// environment label while at least one Environment exists.
	ReasonNoEnvironment = "NoEnvironment"
	// ReasonUnknownEnvironment means that the environment label of the
	// namespace of a claim names an Environment that does not exist.
	ReasonUnknownEnvironment = "UnknownEnvironment"
	// ReasonNotAllowed means that the claim asks for a value that is not
	// allowed, such as a preset, a release, a registration, a parameter, or
	// the deletion policy Orphan. The message names what is not allowed.
	ReasonNotAllowed = "NotAllowed"
	// ReasonMissingInput means that a registration of the claim needs an
	// input that no selected section gives.
	ReasonMissingInput = "MissingInput"
	// ReasonSlotConflict means that the claim sets a field that a selected
	// section fills.
	ReasonSlotConflict = "SlotConflict"
	// ReasonWiringCycle means that the inputs of the selected registrations
	// form a cycle. The message names the sections on the cycle.
	ReasonWiringCycle = "WiringCycle"
	// ReasonDeleting means that the claim is deleted and waits for the
	// objects that the message names.
	ReasonDeleting = "Deleting"
)
