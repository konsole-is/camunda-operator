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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// errRefused is what a Kubernetes call answers in these tests.
var errRefused = errors.New("the API server refused the call")

func TestWrapReturnsNilForACallThatSucceeded(t *testing.T) {
	// errors.Join keeps every non-nil error, and a typed nil pointer in an
	// error is not nil. A step that succeeded must give back a nil interface.
	err := stepPing.wrap(nil)

	assert.NoError(t, err)
	assert.Nil(t, err)
}

func TestFirstStepPicksTheStepThatFailedFirst(t *testing.T) {
	tests := []struct {
		name string
		errs []error
		want step
	}{
		{
			name: "no step failed",
			errs: []error{nil, nil, nil},
		},
		{
			name: "only errors that name no step",
			errs: []error{nil, errRefused},
		},
		{
			name: "the one step that failed",
			errs: []error{nil, stepPing.wrap(errRefused), nil},
			want: stepPing,
		},
		{
			name: "the first of several, in the order given",
			errs: []error{
				stepWebModelerUsers.wrap(errRefused),
				stepPing.wrap(errRefused),
				stepWriteContract.wrapAs(v1.ReasonWriteFailed, errRefused),
			},
			want: stepWebModelerUsers,
		},
		{
			name: "an error that names no step never wins",
			errs: []error{errRefused, stepPing.wrap(errRefused)},
			want: stepPing,
		},
		{
			name: "a step failure that another error wraps",
			errs: []error{fmt.Errorf("reconciling: %w", stepReleaseClaims.wrap(errRefused))},
			want: stepReleaseClaims,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			failed := firstStep(tt.errs...)

			if tt.want == "" {
				assert.Nil(t, failed)
				return
			}
			require.NotNil(t, failed)
			assert.Equal(t, tt.want, failed.step)
		})
	}
}

func TestStepFailureNamesTheStepOnReady(t *testing.T) {
	mc := &v1.CamundaManagementCluster{}
	mc.Generation = 7

	failed := firstStep(stepWebModelerUsers.wrap(errRefused))
	require.NotNil(t, failed)

	condition := failed.condition(mc)

	assert.Equal(t, v1.ConditionReady, condition.Type)
	assert.Equal(t, metav1.ConditionFalse, condition.Status)
	assert.Equal(t, v1.ReasonStepFailed, condition.Reason)
	assert.Equal(t, "Could not sync the Web Modeler users: "+errRefused.Error(), condition.Message)
	assert.Equal(t, int64(7), condition.ObservedGeneration)
}

// The contract write is the one step with a reason of its own, because
// WriteFailed is what CamundaManagementCluster documents for a refused
// ManagementAuthConfig.
func TestTheContractWriteKeepsWriteFailed(t *testing.T) {
	mc := &v1.CamundaManagementCluster{}

	failed := firstStep(stepWriteContract.wrapAs(v1.ReasonWriteFailed, errRefused))
	require.NotNil(t, failed)

	condition := failed.condition(mc)

	assert.Equal(t, v1.ReasonWriteFailed, condition.Reason)
	assert.Equal(t, "Could not write the ManagementAuthConfig: "+errRefused.Error(), condition.Message)
}

func TestAStepFailureKeepsTheErrorOfTheCall(t *testing.T) {
	err := stepFindClusters.wrap(fmt.Errorf("listing the CamundaClusters: %w", errRefused))

	assert.ErrorIs(t, err, errRefused)
	assert.EqualError(
		t, err,
		"could not find the orchestration clusters: listing the CamundaClusters: "+errRefused.Error(),
	)
}
