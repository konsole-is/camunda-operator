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
// when its pre-checks pass. Ready is True only when every one of those
// conditions is True. Its reason is then the ocf status of the governing
// component, not a constant from this file. Examples are Healthy, Creating,
// Updating, Failing, Degraded, Down, Suspended, and Error.
const (
	// ConditionReady is the aggregate condition that every CRD reports.
	ConditionReady = "Ready"
	// ConditionMirroredSecretsReady reports whether the operator copied every
	// referenced Secret from another namespace into the namespace of the
	// workloads. A pod reads only the Secrets of its own namespace. The
	// condition affects Ready only when such a Secret is referenced. It reads
	// Disabled when no such Secret is referenced.
	ConditionMirroredSecretsReady = "MirroredSecretsReady"

	// ReasonHealthy means that all checks passed. It is also the ocf status
	// of a healthy component, so a derived Ready has the same reason.
	ReasonHealthy = "Healthy"
	// ReasonInvalidReference means that a referenced custom resource does not
	// exist, or that a reference is otherwise not usable.
	ReasonInvalidReference = "InvalidReference"
	// ReasonMissingSecret means that a referenced Secret is missing, or that
	// it does not have a configured key.
	ReasonMissingSecret = "MissingSecret"
	// ReasonConnectionFailed means that the operator cannot reach a server
	// that the resource uses, or that the server refuses the configured
	// credentials.
	ReasonConnectionFailed = "ConnectionFailed"
	// ReasonStorageTypeMismatch means that the secondary storage type of the
	// referenced cluster does not fit this resource.
	ReasonStorageTypeMismatch = "StorageTypeMismatch"
	// ReasonWaitingForHandover means that workloads of another holder of the
	// claim of this resource still exist. This resource can already hold the
	// claim, or it can wait to take it. Until these workloads are gone, it
	// keeps its own workloads stopped, or its own work on hold. The message
	// names what it waits for. The CRD doc of each resource that reports it
	// names the workloads that it waits for.
	ReasonWaitingForHandover = "WaitingForHandover"
	// ReasonStorageAlreadyAttached means that another CamundaCluster than the
	// one that this resource names holds the storage claim of the backend.
	// One backend serves one CamundaCluster. The index names and the tables
	// are fixed, so two clusters on one backend write to the data of each
	// other. A CamundaCluster stays suspended, with its volumes, until the
	// holder moves to another backend or is deleted. Then it resumes
	// automatically. A restore waits and writes nothing to the backend. The
	// message names the holder and the backend.
	ReasonStorageAlreadyAttached = "StorageAlreadyAttached"
	// ReasonSuspensionHeld means that a CamundaCluster has at least one
	// suspension hold annotation, so it stays suspended whatever spec.suspend
	// says. The message names each hold and its reason.
	ReasonSuspensionHeld = "SuspensionHeld"
)
