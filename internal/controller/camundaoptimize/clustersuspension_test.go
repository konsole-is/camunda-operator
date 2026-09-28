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
	"strings"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// TestSuspendedByNamesEachSourceOfASuspension covers each source of a
// suspension that the pre-check reads.
func TestSuspendedByNamesEachSourceOfASuspension(t *testing.T) {
	cases := map[string]struct {
		arrange func(t *testing.T, h *reconcileHarness)
		want    v1.OptimizeSuspension
	}{
		"nothing holds the workloads": {
			arrange: func(*testing.T, *reconcileHarness) {},
			want:    "",
		},
		"the cluster is suspended": {
			arrange: func(t *testing.T, h *reconcileHarness) { h.setClusterSuspend(t, true) },
			want:    v1.OptimizeSuspensionCluster,
		},
		"the cluster does not hold the storage claim": {
			arrange: func(t *testing.T, h *reconcileHarness) { h.releaseClaim(t) },
			want:    v1.OptimizeSuspensionStorageClaim,
		},
		"a pod of another cluster still writes the backend": {
			arrange: func(t *testing.T, h *reconcileHarness) { h.runForeignWriter() },
			want:    v1.OptimizeSuspensionStorageClaim,
		},
		"a restore into another cluster writes the backend": {
			arrange: func(t *testing.T, h *reconcileHarness) { h.runForeignRestore(t) },
			want:    v1.OptimizeSuspensionStorageClaim,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newReconcileHarness(t)
			tc.arrange(t, h)

			res, err := h.reconciler.preCheck(t.Context(), h.latest(t))

			require.NoError(t, err)
			assert.Equal(t, tc.want, suspendedBy(res))
		})
	}
}

// TestReadSuspensionReadsTheStatus covers the prior state that a pass carries
// in: the suspension comes from status.suspendedBy and nothing else.
func TestReadSuspensionReadsTheStatus(t *testing.T) {
	t.Parallel()

	notRendered := &v1.CamundaOptimize{}
	notRendered.Status.SuspendedBy = v1.OptimizeSuspensionStorageClaim
	assert.Equal(
		t,
		suspension{by: v1.OptimizeSuspensionStorageClaim},
		readSuspension(notRendered),
		"an instance created beside a parked cluster follows the wait with no workload",
	)

	rendered := &v1.CamundaOptimize{}
	meta.SetStatusCondition(&rendered.Status.Conditions, metav1.Condition{
		Type:   v1.ConditionImporterReady,
		Status: metav1.ConditionTrue,
		Reason: string(component.Suspended),
	})
	assert.Equal(t, suspension{rendered: true}, readSuspension(rendered))
}

// TestRecordSuspensionChangeRecordsTheStartAndTheEndOnly pins which changes
// reach the event stream: the start and the end of a suspension, and no other.
// Every pass stages the state it decided on.
func TestRecordSuspensionChangeRecordsTheStartAndTheEndOnly(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		prior suspension
		now   v1.OptimizeSuspension
		want  []string
	}{
		"a suspension starts": {
			prior: suspension{rendered: true},
			now:   v1.OptimizeSuspensionCluster,
			want:  []string{eventReasonClusterSuspended},
		},
		"a suspension ends": {
			prior: suspension{by: v1.OptimizeSuspensionStorageClaim, rendered: true},
			now:   "",
			want:  []string{eventReasonClusterResumed},
		},
		"a suspension holds": {
			prior: suspension{by: v1.OptimizeSuspensionCluster, rendered: true},
			now:   v1.OptimizeSuspensionCluster,
			want:  []string{},
		},
		"the wait changes": {
			prior: suspension{by: v1.OptimizeSuspensionCluster, rendered: true},
			now:   v1.OptimizeSuspensionStorageClaim,
			want:  []string{},
		},
		"no suspension holds": {
			prior: suspension{rendered: true},
			now:   "",
			want:  []string{},
		},
		"the instance rendered no workload yet": {
			prior: suspension{},
			now:   v1.OptimizeSuspensionStorageClaim,
			want:  []string{},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			recorder := events.NewFakeRecorder(10)
			r := &Reconciler{EventRecorder: recorder}
			optimize := suspendedOptimize()

			r.recordSuspensionChange(optimize, tc.prior, tc.now)

			notes := recordedNotes(recorder)
			reasons := make([]string, 0, len(notes))
			for _, note := range notes {
				reasons = append(reasons, strings.Fields(note)[1])
			}
			assert.Equal(t, tc.want, reasons)
			assert.Equal(t, tc.now, optimize.Status.SuspendedBy, "the pass stages what it decided on")
		})
	}
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
		prior suspension
		now   v1.OptimizeSuspension
		want  string
	}{
		"the cluster is suspended": {
			prior: suspension{rendered: true},
			now:   v1.OptimizeSuspensionCluster,
			want: `Normal ClusterSuspended CamundaCluster "my-cluster" is suspended, ` +
				"the Optimize workloads follow it to zero",
		},
		"the claim of the backend holds the workloads": {
			prior: suspension{rendered: true},
			now:   v1.OptimizeSuspensionStorageClaim,
			want: `Normal StorageClaimAwaited CamundaCluster "my-cluster" does not hold its backend, ` +
				"or pods of another cluster or of a previous instance, or a writer for another cluster, " +
				"such as a restore, still write it, the Optimize workloads follow it to zero",
		},
		// One note covers a resume from either wait, because the cluster was
		// not suspended in one of them.
		"the workloads follow their spec again": {
			prior: suspension{by: v1.OptimizeSuspensionCluster, rendered: true},
			want: `Normal ClusterResumed CamundaCluster "my-cluster" holds its backend and is not suspended, ` +
				"the Optimize workloads follow their spec",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			recorder := events.NewFakeRecorder(10)
			r := &Reconciler{EventRecorder: recorder}

			r.recordSuspensionChange(suspendedOptimize(), tc.prior, tc.now)

			assert.Equal(t, []string{tc.want}, recordedNotes(recorder))
		})
	}
}

// TestBackendClaimAwaitedSaysBothHalvesOfTheGate pins the claim wait. The gate
// covers a cluster that does not hold the claim of its backend, and one that
// holds it while other writers still write that backend. Everything this wait
// says reaches a user, so none of it may name the first half alone.
func TestBackendClaimAwaitedSaysBothHalvesOfTheGate(t *testing.T) {
	t.Parallel()

	const bothHalves = "does not hold its backend, or pods of another cluster or of a previous instance, " +
		"or a writer for another cluster, such as a restore, still write it"
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

// TestReconcileNamesTheWaitWhenACheckFails covers the pre-check failure path,
// which records the start of a wait and no other transition. It names the same
// two waits: a cluster that reports itself suspended, and the claim of the
// backend that a healthy cluster does not hold.
func TestReconcileNamesTheWaitWhenACheckFails(t *testing.T) {
	cases := map[string]struct {
		arrange func(t *testing.T, h *reconcileHarness)
		want    string
	}{
		"the cluster is suspended": {
			arrange: func(t *testing.T, h *reconcileHarness) { h.setClusterSuspend(t, true) },
			want: `Normal ClusterSuspended CamundaCluster "my-cluster" is suspended, ` +
				"the Optimize workloads follow it to zero",
		},
		"the cluster does not hold the storage claim": {
			arrange: func(t *testing.T, h *reconcileHarness) { h.releaseClaim(t) },
			want: `Normal StorageClaimAwaited CamundaCluster "my-cluster" does not hold its backend, ` +
				"or pods of another cluster or of a previous instance, or a writer for another cluster, " +
				"such as a restore, still write it, the Optimize workloads follow it to zero",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			h := newReconcileHarness(t)
			h.start(t)
			require.NoError(t, h.client.Delete(t.Context(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
				Name: "mac-client", Namespace: h.optimize.Namespace,
			}}))
			tc.arrange(t, h)

			require.NoError(t, h.reconcile(t))

			ready := meta.FindStatusCondition(h.latest(t).Status.Conditions, v1.ConditionReady)
			require.NotNil(t, ready)
			require.Equal(t, v1.ReasonMissingSecret, ready.Reason, "the pass took the failure path")
			var suspensionNotes []string
			for _, note := range recordedNotes(h.recorder) {
				switch strings.Fields(note)[1] {
				case eventReasonClusterSuspended, eventReasonClusterResumed, eventReasonStorageClaimAwaited:
					suspensionNotes = append(suspensionNotes, note)
				}
			}
			assert.Equal(t, []string{tc.want}, suspensionNotes)
		})
	}
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
