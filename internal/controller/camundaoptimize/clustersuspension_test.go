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
	"k8s.io/client-go/tools/events"

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

// TestSuspensionNotesSpeakForTheClusterOnly pins what the two events say. The
// Ready condition cannot name the cluster, so the note must. Each note reports
// the state of that cluster and nothing about a replica count: an importer that
// spec.importer.replicas holds at zero does not start when the cluster resumes.
func TestSuspensionNotesSpeakForTheClusterOnly(t *testing.T) {
	t.Parallel()

	optimize := &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{Name: "co-a", Namespace: "team-a"},
		Spec:       v1.CamundaOptimizeSpec{ClusterRef: v1.ClusterRef{Name: "my-cluster"}},
	}
	recorder := events.NewFakeRecorder(10)
	r := &Reconciler{EventRecorder: recorder}

	r.recordClusterSuspended(optimize)
	r.recordSuspensionChange(optimize, true, false)

	assert.Equal(
		t,
		[]string{
			`Normal ClusterSuspended CamundaCluster "my-cluster" is suspended, ` +
				"the Optimize workloads follow it to zero",
			`Normal ClusterResumed CamundaCluster "my-cluster" is no longer suspended, ` +
				"the Optimize workloads follow their spec",
		},
		recordedNotes(recorder),
	)
}

// recordedNotes drains recorder and returns what it holds.
func recordedNotes(recorder *events.FakeRecorder) []string {
	var notes []string
	for {
		select {
		case recorded := <-recorder.Events:
			notes = append(notes, recorded)
		default:
			return notes
		}
	}
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
