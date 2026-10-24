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

// The condition vocabulary of the plugins group. Ready and its True reason
// Healthy come from api/v1, so this group does not declare them again.
const (
	// ConditionAccepted reports whether the registry controller accepts a
	// PluginRegistration. Its False reasons are SectionMissing,
	// OperatorVersionUnsupported, and DefaultConflict.
	ConditionAccepted = "Accepted"
	// ConditionOutputsReady reports whether the plugin wrote the outputs of a
	// PluginRequest for its current generation. The plugin sets its reasons.
	ConditionOutputsReady = "OutputsReady"
	// ConditionApplied reports whether the identity extension applied the
	// fragment of a Grant. Its False reasons are OverCeiling, OverSizeLimit,
	// and Rejected.
	ConditionApplied = "Applied"
	// ConditionMatchesTemplate reports whether the fragment of a Grant still
	// matches its template in the registration. Its False reason is
	// TemplateChanged.
	ConditionMatchesTemplate = "MatchesTemplate"

	// ReasonSectionMissing means that a section of the registration does not
	// exist.
	ReasonSectionMissing = "SectionMissing"
	// ReasonOperatorVersionUnsupported means that the version of the
	// operator is outside the operatorVersions range of the registration.
	ReasonOperatorVersionUnsupported = "OperatorVersionUnsupported"
	// ReasonDefaultConflict means that another registration of the same
	// section also has mode Default.
	ReasonDefaultConflict = "DefaultConflict"
	// ReasonGrantRejected means that a Grant of the request has Ready=False.
	// The outputs of the request stay usable.
	ReasonGrantRejected = "GrantRejected"
	// ReasonWaitingForGrants means that a Grant of the request does not have
	// Ready=True yet, and no Grant of the request has Ready=False. The
	// outputs of the request stay usable.
	ReasonWaitingForGrants = "WaitingForGrants"
	// ReasonWaitingForEndpoints means that an access request waits for the
	// endpoints in the status of its target.
	ReasonWaitingForEndpoints = "WaitingForEndpoints"
	// ReasonInvalidParameters means that the parameters of the request do
	// not pass the schema or the rules of its registration.
	ReasonInvalidParameters = "InvalidParameters"
	// ReasonOverCeiling means that the fragment of a Grant asks for more than
	// the ceiling of the identity extension allows.
	ReasonOverCeiling = "OverCeiling"
	// ReasonOverSizeLimit means that the fragment of a Grant takes the
	// policy of the identity over the size limit of the cloud.
	ReasonOverSizeLimit = "OverSizeLimit"
	// ReasonRejected means that the identity extension or the cloud refused
	// the fragment of a Grant for another reason. The message says why.
	ReasonRejected = "Rejected"
	// ReasonTemplateChanged means that the fragment of a Grant no longer
	// matches its template in the registration.
	ReasonTemplateChanged = "TemplateChanged"
)
