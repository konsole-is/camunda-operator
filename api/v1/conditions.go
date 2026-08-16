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

package v1

// The condition vocabulary that more than one CRD reports. A reason that only
// one CRD reports is declared next to that CRD, in its types file. The CRD doc
// under docs/crds is the contract for both.
//
// A CRD that runs ocf components derives Ready from its component conditions
// once its pre-checks pass. Ready is True only when every one of those
// conditions is True. Its reason is then the ocf status (Healthy, Creating,
// Updating, Failing, Degraded, Down, Suspended, Error, and more) of the
// governing component, not a constant from this file.
const (
	// ConditionReady is the aggregate condition that every CRD reports.
	ConditionReady = "Ready"

	// ReasonHealthy means that all checks passed. It is also the ocf status
	// of a healthy component, so a derived Ready reports the same reason.
	ReasonHealthy = "Healthy"
	// ReasonInvalidReference means that a referenced custom resource does not
	// exist, or that a reference is otherwise not usable.
	ReasonInvalidReference = "InvalidReference"
	// ReasonMissingSecret means that a referenced Secret is missing or lacks a
	// configured key.
	ReasonMissingSecret = "MissingSecret"
	// ReasonConnectionFailed means that a backing server is unreachable or
	// rejects the configured credentials.
	ReasonConnectionFailed = "ConnectionFailed"
)
