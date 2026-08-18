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

package logicalbackupelasticsearch

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin/camundaadmintest"
)

// testBackupID is the backup ID of every unit-tested runtime request.
const testBackupID int64 = 1_000

// runtimeRig is a reconciler and a management fake for the runtime request
// alone, without a manager. The request path is a pure function of the
// status and the answers of the cluster.
func runtimeRig(t *testing.T) (*Reconciler, *camundaadmin.Client, *camundaadmintest.Server) {
	t.Helper()
	server := camundaadmintest.New()
	t.Cleanup(server.Close)
	mgmt, err := camundaadmin.New(camundaadmin.Binding{Endpoint: server.URL(), Version: "8.9.9"})
	require.NoError(t, err)
	r := &Reconciler{
		EventRecorder: events.NewFakeRecorder(16),
		options:       Options{RuntimeRegistrationGrace: time.Minute},
	}
	return r, mgmt, server
}

func runtimeBackup() *v1.LogicalBackupElasticsearch {
	backup := &v1.LogicalBackupElasticsearch{}
	backup.Status.BackupID = testBackupID
	backup.Status.Phase = v1.LogicalBackupRunning
	backup.Status.Step = v1.StepBackupRuntime
	backup.Status.Runtime = v1.BackupPart{State: v1.BackupPartPending}
	return backup
}

// The intent is written long before the request goes out, for example across
// an operator outage. The registration grace must start at the acceptance,
// not at the intent, or a normal registration lag looks expired.
func TestRuntimeRequestGraceStartsAtTheAcceptance(t *testing.T) {
	r, mgmt, server := runtimeRig(t)
	backup := runtimeBackup()
	longAgo := metav1.NewTime(time.Now().Add(-time.Hour))
	backup.Status.RuntimeRequestedTime = &longAgo

	_, err := r.requestRuntimeBackup(t.Context(), mgmt, backup, &backup.Status.Runtime)
	require.NoError(t, err)
	assert.Equal(t, 1, server.RuntimeStarts(1_000))
	require.NotNil(t, backup.Status.RuntimeAcceptedTime)
	assert.Equal(t, v1.BackupPartInProgress, backup.Status.Runtime.State)

	// The cluster reports the backup absent for a moment: registration lag.
	// The step polls and does not request again.
	_, err = r.requestRuntimeBackup(t.Context(), mgmt, backup, &backup.Status.Runtime)
	require.NoError(t, err)
	assert.Equal(t, 1, server.RuntimeStarts(1_000))
	assert.Equal(t, v1.StepBackupRuntime, backup.Status.Step)
	assert.Empty(t, backup.Status.FailureMessage)

	// Past the grace, still absent: the step fails through resume.
	expired := metav1.NewTime(time.Now().Add(-2 * time.Minute))
	backup.Status.RuntimeAcceptedTime = &expired
	_, err = r.requestRuntimeBackup(t.Context(), mgmt, backup, &backup.Status.Runtime)
	require.NoError(t, err)
	assert.Equal(t, 1, server.RuntimeStarts(1_000))
	assert.Equal(t, v1.StepResumeExporting, backup.Status.Step)
	assert.Contains(t, backup.Status.FailureMessage, "holds no runtime backup 1000")
	assert.Equal(t, v1.BackupPartFailed, backup.Status.Runtime.State)
}

// The request conflicts: the cluster holds the ID. With the intent recorded
// that can be this backup's own request with a lost response, or another
// actor's backup. Nothing tells them apart, so a conflict is never an
// acceptance: the step fails without adopting, and says so.
func TestRuntimeRequestConflictWithIntentFailsWithoutAdoption(t *testing.T) {
	r, mgmt, server := runtimeRig(t)
	backup := runtimeBackup()
	now := metav1.Now()
	backup.Status.RuntimeRequestedTime = &now
	server.SetRuntimeState(1_000, "IN_PROGRESS", "")

	_, err := r.requestRuntimeBackup(t.Context(), mgmt, backup, &backup.Status.Runtime)
	require.NoError(t, err)

	assert.Zero(t, server.RuntimeStarts(1_000))
	assert.Nil(t, backup.Status.RuntimeAcceptedTime)
	assert.Equal(t, v1.StepResumeExporting, backup.Status.Step)
	assert.Equal(t, v1.BackupPartFailed, backup.Status.Runtime.State)
	assert.Contains(t, backup.Status.FailureMessage, "lost response, or one of another actor")
	assert.Contains(t, backup.Status.FailureMessage, "not adopted")
}

// A runtime backup that exists under the ID before any intent was recorded
// cannot be this backup's, because the intent is flushed one reconcile
// before every request. It is not adopted, whatever its state.
func TestRuntimeBackupFoundWithoutIntentIsNotAdopted(t *testing.T) {
	for _, state := range []string{"IN_PROGRESS", "COMPLETED"} {
		t.Run(state, func(t *testing.T) {
			r, mgmt, server := runtimeRig(t)
			server.SetRuntimeState(1_000, state, "")
			backup := runtimeBackup()
			cluster := &v1.CamundaCluster{}
			cluster.Status.Management = &v1.ManagementBinding{
				Endpoint: server.URL(), Version: "8.9.9",
				Auth: v1.ManagementAuth{Method: v1.ManagementAuthMethodNone},
			}
			r.APIReader = fake.NewClientBuilder().Build()
			_ = mgmt

			_, err := r.backupRuntime(t.Context(), backup, cluster)
			require.NoError(t, err)

			assert.Equal(t, v1.StepResumeExporting, backup.Status.Step)
			assert.Equal(t, v1.BackupPartFailed, backup.Status.Runtime.State)
			assert.Contains(t, backup.Status.FailureMessage, "belongs to another actor")
			assert.Contains(t, backup.Status.FailureMessage, "not adopted")
			assert.Nil(t, backup.Status.RuntimeAcceptedTime)
		})
	}
}

// The resume deadline bounds attempting, and the time inside an attempt
// counts. A slow endpoint that eats the client timeout on every attempt
// exhausts a deadline in about the deadline, not in many times the deadline.
func TestResumeDeadlineCountsTheTimeInsideAnAttempt(t *testing.T) {
	const attempt = 300 * time.Millisecond
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(attempt)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("still not ready"))
	}))
	t.Cleanup(slow.Close)

	r := &Reconciler{
		APIReader:     fake.NewClientBuilder().Build(),
		EventRecorder: events.NewFakeRecorder(16),
		options:       Options{ResumeDeadline: time.Second, PollInterval: 50 * time.Millisecond},
	}
	cluster := &v1.CamundaCluster{}
	cluster.Status.Management = &v1.ManagementBinding{
		Endpoint: slow.URL, Version: "8.9.9",
		Auth: v1.ManagementAuth{Method: v1.ManagementAuthMethodNone},
	}
	backup := runtimeBackup()
	backup.Status.Step = v1.StepResumeExporting

	started := time.Now()
	attempts := 0
	for backup.Status.Phase != v1.LogicalBackupFailed {
		_, err := r.resumeExporting(t.Context(), backup, cluster)
		require.NoError(t, err)
		attempts++
		require.Less(t, attempts, 20, "the deadline never fired")
		time.Sleep(r.poll())
	}
	elapsed := time.Since(started)

	assert.Equal(t, v1.ReasonResumeFailed, backup.Status.TerminalReason)
	// Four attempts of 300 ms plus three gaps of 50 ms pass one second.
	assert.LessOrEqual(t, attempts, 5)
	assert.Less(t, elapsed, 2*time.Second, "the deadline is exhausted in about the deadline")
}

// Without the intent recorded, the reconcile records it and requests
// nothing. The intent must be durable before the request goes out.
func TestRuntimeRequestRecordsTheIntentFirst(t *testing.T) {
	r, mgmt, server := runtimeRig(t)
	backup := runtimeBackup()

	_, err := r.requestRuntimeBackup(t.Context(), mgmt, backup, &backup.Status.Runtime)
	require.NoError(t, err)

	assert.Zero(t, server.RuntimeStarts(1_000))
	assert.NotNil(t, backup.Status.RuntimeRequestedTime)
	assert.Nil(t, backup.Status.RuntimeAcceptedTime)
}
