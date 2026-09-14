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
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
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

// TestSuspensionNotesSpeakForTheClusterOnly pins what each event says. The
// Ready condition cannot name the cluster, so the note must. Each note reports
// the state of that cluster and nothing about a replica count: an importer that
// spec.importer.replicas holds at zero does not start when the cluster resumes.
//
// The cluster can report itself healthy while the claim of its backend holds
// these workloads at zero, so that wait gets an event that says so.
func TestSuspensionNotesSpeakForTheClusterOnly(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		res  resolved
		want string
	}{
		"the cluster is suspended": {
			res: resolved{Input: components.Input{Suspended: true}},
			want: `Normal ClusterSuspended CamundaCluster "my-cluster" is suspended, ` +
				"the Optimize workloads follow it to zero",
		},
		"the claim of the backend holds the workloads": {
			res: resolved{Input: components.Input{Suspended: true}, AwaitsBackendClaim: true},
			want: `Normal StorageClaimAwaited CamundaCluster "my-cluster" does not hold its backend, ` +
				"or pods of another cluster still write it, the Optimize workloads follow it to zero",
		},
		// One note covers a resume from either wait, because the cluster was
		// not suspended in one of them.
		"the workloads follow their spec again": {
			res: resolved{},
			want: `Normal ClusterResumed CamundaCluster "my-cluster" holds its backend and is not suspended, ` +
				"the Optimize workloads follow their spec",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			recorder := events.NewFakeRecorder(10)
			r := &Reconciler{EventRecorder: recorder}

			r.recordSuspensionChange(suspendedOptimize(), !tc.res.Input.Suspended, tc.res)

			assert.Equal(t, []string{tc.want}, recordedNotes(recorder))
		})
	}
}

// TestBackendClaimAwaitedSaysBothHalvesOfTheGate pins the claim wait. The gate
// covers a cluster that does not hold the claim of its backend, and one that
// holds it while pods of another cluster still write that backend. Everything
// this wait says reaches a user, so none of it may name the first half alone.
func TestBackendClaimAwaitedSaysBothHalvesOfTheGate(t *testing.T) {
	t.Parallel()

	const bothHalves = "does not hold its backend, or pods of another cluster still write it"
	said := map[string]string{
		"the event note":                      backendClaimAwaited.eventNote,
		"the note on Ready":                   backendClaimAwaited.failureNote,
		"the condition of a stopped workload": backendClaimAwaited.condition,
		"the event of a workload it lowered":  backendClaimAwaited.workloadNote,
	}
	for name, says := range said {
		assert.Contains(t, says, bothHalves, name)
	}
}

// TestRecordClusterSuspendedNamesTheWait covers the pre-check failure path,
// which records the start of a wait and no other transition. It names the same
// two waits: a cluster that reports itself suspended, and the claim of the
// backend that a healthy cluster does not hold.
func TestRecordClusterSuspendedNamesTheWait(t *testing.T) {
	t.Parallel()

	recorder := events.NewFakeRecorder(10)
	r := &Reconciler{EventRecorder: recorder}

	r.recordClusterSuspended(suspendedOptimize(), resolved{Input: components.Input{Suspended: true}})
	r.recordClusterSuspended(
		suspendedOptimize(),
		resolved{Input: components.Input{Suspended: true}, AwaitsBackendClaim: true},
	)

	assert.Equal(
		t,
		[]string{
			`Normal ClusterSuspended CamundaCluster "my-cluster" is suspended, ` +
				"the Optimize workloads follow it to zero",
			`Normal StorageClaimAwaited CamundaCluster "my-cluster" does not hold its backend, ` +
				"or pods of another cluster still write it, the Optimize workloads follow it to zero",
		},
		recordedNotes(recorder),
	)
}

// suspendedOptimize returns a CamundaOptimize attached to my-cluster, which is
// the name that every note above carries.
func suspendedOptimize() *v1.CamundaOptimize {
	return &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{Name: "co-a", Namespace: "team-a"},
		Spec:       v1.CamundaOptimizeSpec{ClusterRef: v1.ClusterRef{Name: "my-cluster"}},
	}
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

// TestHasWorkloadsReadsEveryWorkloadCondition covers the partial render: the
// importer patch was rejected and the webapp stopped, so only the webapp
// reports. The instance rendered a workload either way, and a resume of it is a
// transition a user acts on.
func TestHasWorkloadsReadsEveryWorkloadCondition(t *testing.T) {
	t.Parallel()

	for _, conditionType := range []string{v1.ConditionImporterReady, v1.ConditionWebappReady} {
		optimize := &v1.CamundaOptimize{}
		meta.SetStatusCondition(&optimize.Status.Conditions, metav1.Condition{
			Type:   conditionType,
			Status: metav1.ConditionTrue,
			Reason: v1.ReasonHealthy,
		})
		assert.True(t, hasWorkloads(optimize), conditionType)
	}

	assert.False(
		t,
		hasWorkloads(withReadyReason(v1.ReasonHealthy)),
		"Ready alone says nothing about a workload",
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
