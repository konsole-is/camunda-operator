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

package pluginrequest

import (
	"strconv"

	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// The handlers in this file replace the scaffolded defaults that the ocf
// generator writes into builder.go. After a run of the generator with
// --force, delete the scaffolded Default* handlers from builder.go again.

// DefaultOperationalStatusHandler reports OperationPending until the
// plugin reports the outputs of the current generation: OutputsReady=True
// with the observedGeneration of the condition equal to
// metadata.generation. It reports Operational then. The reason names the
// section of the request, and it carries the reason and the message of
// OutputsReady=False.
func DefaultOperationalStatusHandler(
	_ concepts.ConvergingOperation, request *pluginsv1alpha1.PluginRequest,
) (concepts.OperationalStatusWithReason, error) {
	ready, reason := outputsReady(request)
	if ready {
		return concepts.OperationalStatusWithReason{Status: concepts.OperationalStatusOperational, Reason: reason}, nil
	}

	return concepts.OperationalStatusWithReason{Status: concepts.OperationalStatusPending, Reason: reason}, nil
}

// outputsReady reports whether the outputs of request hold for its current
// generation, with a reason that names the section.
func outputsReady(request *pluginsv1alpha1.PluginRequest) (bool, string) {
	section := request.Spec.Section
	outputs := meta.FindStatusCondition(request.Status.Conditions, pluginsv1alpha1.ConditionOutputsReady)

	switch {
	case outputs == nil:
		return false, "The plugin has not reported the outputs of section " + section + " yet"
	case outputs.ObservedGeneration != request.Generation:
		return false, "The plugin has not reported the outputs of section " + section +
			" for generation " + strconv.FormatInt(request.Generation, 10) + " yet"
	case outputs.Status != metav1.ConditionTrue:
		reason := "The outputs of section " + section + " are not ready (" + outputs.Reason + ")"
		if outputs.Message != "" {
			reason += ": " + outputs.Message
		}

		return false, reason
	}

	return true, "The outputs of section " + section + " are ready"
}

// DefaultGraceStatusHandler grades a PluginRequest at the end of the grace
// period: Healthy when its outputs hold for the current generation, and
// Down otherwise.
func DefaultGraceStatusHandler(request *pluginsv1alpha1.PluginRequest) (concepts.GraceStatusWithReason, error) {
	ready, reason := outputsReady(request)
	if ready {
		return concepts.GraceStatusWithReason{Status: concepts.GraceStatusHealthy, Reason: reason}, nil
	}

	return concepts.GraceStatusWithReason{Status: concepts.GraceStatusDown, Reason: reason}, nil
}

// DefaultSuspendMutationHandler records no change. A claim does not
// suspend its requests.
func DefaultSuspendMutationHandler(_ *Mutator) error {
	return nil
}

// DefaultSuspensionStatusHandler reports Suspended at once, because the
// suspension of the component changes nothing on the PluginRequest.
func DefaultSuspensionStatusHandler(_ *pluginsv1alpha1.PluginRequest) (concepts.SuspensionStatusWithReason, error) {
	return concepts.SuspensionStatusWithReason{
		Status: concepts.SuspensionStatusSuspended,
		Reason: "The suspension of the component leaves the PluginRequest as it is",
	}, nil
}

// DefaultDeleteOnSuspendHandler keeps the PluginRequest on suspension.
func DefaultDeleteOnSuspendHandler(_ *pluginsv1alpha1.PluginRequest) bool {
	return false
}
