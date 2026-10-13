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
// and InvalidReference come from api/v1. While a claim waits, ocf reports
// PrerequisiteNotMet or OperationPending, so this group has no reason of its
// own for waiting.
const (
	// ReasonNoEnvironment means that the namespace of a claim has no
	// environment label while at least one Environment exists.
	ReasonNoEnvironment = "NoEnvironment"
	// ReasonUnknownEnvironment means that the environment label of the
	// namespace of a claim names an Environment that does not exist.
	ReasonUnknownEnvironment = "UnknownEnvironment"
	// ReasonNotAllowed means that the policy of the environment does not
	// allow a value of the claim, such as a preset, a release, a
	// registration, or a parameter. The message names each value.
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
)
