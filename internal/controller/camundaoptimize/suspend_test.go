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

package camundaoptimize

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// TestWasSuspendingCoversEveryStageOfASuspension pins what stops the
// suspension event from being recorded twice.
//
// A suspension passes through three ocf statuses before it settles. A
// reconcile that catches the workloads mid-drain reads one of the first two,
// and it must not read them as "not suspended yet", or it records the
// transition again on every look until the drain finishes.
func TestWasSuspendingCoversEveryStageOfASuspension(t *testing.T) {
	t.Parallel()

	suspending := []string{
		string(component.PendingSuspension),
		string(component.Suspending),
		string(component.Suspended),
	}
	for _, reason := range suspending {
		assert.True(t, wasSuspending(withReadyReason(reason)), reason)
	}

	running := []string{
		v1.ReasonHealthy,
		string(component.AliveUpdating),
		string(component.Down),
		v1.ReasonClusterAlreadyAttached,
	}
	for _, reason := range running {
		assert.False(t, wasSuspending(withReadyReason(reason)), reason)
	}

	assert.False(
		t,
		wasSuspending(&v1.CamundaOptimize{}),
		"a resource with no Ready yet has not been suspended",
	)
}

// withReadyReason returns a CamundaOptimize whose Ready condition carries the
// given reason.
func withReadyReason(reason string) *v1.CamundaOptimize {
	optimize := &v1.CamundaOptimize{}
	optimize.Status.Conditions = []metav1.Condition{{
		Type:   v1.ConditionReady,
		Status: metav1.ConditionTrue,
		Reason: reason,
	}}

	return optimize
}
