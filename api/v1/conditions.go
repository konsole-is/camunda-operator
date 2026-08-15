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
const (
	// ConditionReady is the aggregate condition that every CRD reports.
	ConditionReady = "Ready"

	// ReasonHealthy means that all checks passed and every component is
	// ready.
	ReasonHealthy = "Healthy"
	// ReasonProgressing means that the managed resources have not reached
	// their desired state.
	ReasonProgressing = "Progressing"
	// ReasonInvalidReference means that a referenced custom resource does not
	// exist, or that a reference is otherwise not usable.
	ReasonInvalidReference = "InvalidReference"
	// ReasonMissingSecret means that a referenced Secret is missing or lacks a
	// configured key.
	ReasonMissingSecret = "MissingSecret"
	// ReasonConnectionFailed means that a backing server is unreachable or
	// rejects the configured credentials.
	ReasonConnectionFailed = "ConnectionFailed"
	// ReasonSuspended means that the resource is suspended by its spec and
	// intentionally not serving. Every suspendable CRD reports it on Ready.
	ReasonSuspended = "Suspended"
)
