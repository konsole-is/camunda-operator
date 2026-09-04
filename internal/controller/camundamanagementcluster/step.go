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

package camundamanagementcluster

import (
	"errors"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// step names one part of a reconcile that runs outside the ocf components,
// almost always a call to the Kubernetes API. The components report their own
// conditions. A step reports nowhere else, so Ready is where a failed one
// lands.
type step string

// The steps of one reconcile, in the order Reconcile runs them. A step reads
// as the imperative of what it does, because the message of a failed step
// reads "Could not <step>: <answer>".
const (
	stepResolveReferences = step("resolve the references of the spec")
	stepClaimRealm        = step("claim the Keycloak realm")
	stepFindClusters      = step("find the orchestration clusters")
	stepSelectNamespaces  = step("select the namespaces")
	stepClaimClusters     = step("claim the orchestration clusters")
	stepDiscoverOptimize  = step("discover the Optimize instances")
	stepWithdrawCallbacks = step("withdraw the login callbacks from the realm the spec left")
	// stepRecordCallbackRealm is the status write of the realm that the plane
	// points Management Identity at. It is the one status write of a
	// converging pass that the deferred flush does not make, because the
	// record has to be durable before Identity can register the callbacks.
	stepRecordCallbackRealm = step("record the realm of the login callbacks")
	stepBuildComponents     = step("build the components")
	stepRecordClaim         = step("record the initial administrator claim")
	stepWebModelerUsers     = step("sync the Web Modeler users")
	stepPing                = step("point the orchestration clusters at Console")
	stepReleaseClaims       = step("release the clusters that left the selector")
	stepWriteContract       = step("write the ManagementAuthConfig")
	stepOptimizeCallbacks   = step("register the login callbacks of Optimize")
)

// stepError is the failure of one step. It carries the Ready reason of the
// step, so that the one rule in readyCondition builds the condition of every
// step and no step needs a branch of its own.
type stepError struct {
	step   step
	reason string
	err    error
}

// wrap returns err as a failure of s under the StepFailed reason, or nil when
// err is nil.
func (s step) wrap(err error) error {
	return s.wrapAs(v1.ReasonStepFailed, err)
}

// wrapAs is wrap under a Ready reason of its own. Only the contract write
// takes it: WriteFailed is the reason that CamundaManagementCluster documents
// for a refused ManagementAuthConfig, and ManagementAuthReady reports the same
// reason beside Ready.
func (s step) wrapAs(reason string, err error) error {
	if err == nil {
		return nil
	}

	return &stepError{step: s, reason: reason, err: err}
}

// stop stages Ready for a step that ends the reconcile and returns the failure
// for Reconcile to return. err must not be nil.
func (s step) stop(mc *v1.CamundaManagementCluster, err error) error {
	failed := &stepError{step: s, reason: v1.ReasonStepFailed, err: err}
	conditions.Stage(mc, failed.condition(mc))

	return failed
}

// condition is the Ready condition of the failed step.
func (e *stepError) condition(mc *v1.CamundaManagementCluster) metav1.Condition {
	return conditions.Ready(
		metav1.ConditionFalse,
		e.reason,
		fmt.Sprintf("Could not %s: %s", e.step, e.err),
		mc.GetGeneration(),
	)
}

// firstStep returns the failure of the first step of errs that failed, or nil
// when none of them did. The caller passes the errors in reconcile order, so
// the step that failed first is the one Ready names. An error that is not a
// step failure is skipped.
func firstStep(errs ...error) *stepError {
	for _, err := range errs {
		var failed *stepError
		if errors.As(err, &failed) {
			return failed
		}
	}

	return nil
}

// Error names the step and what the call answered.
func (e *stepError) Error() string {
	return fmt.Sprintf("could not %s: %s", e.step, e.err)
}

// Unwrap gives errors.Is and errors.As the error of the call.
func (e *stepError) Unwrap() error {
	return e.err
}
