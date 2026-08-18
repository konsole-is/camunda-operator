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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin/camundaadmintest"
)

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

func runtimeBackup(id int64) *v1.LogicalBackupElasticsearch {
	backup := &v1.LogicalBackupElasticsearch{}
	backup.Status.BackupID = id
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
	backup := runtimeBackup(1_000)
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

// The request went out and the cluster accepted it, but the response was
// lost before the status flush. The next request conflicts. With the intent
// recorded, the conflict is the acceptance, and the step goes on to poll.
func TestRuntimeRequestConflictWithIntentIsAnAcceptance(t *testing.T) {
	r, mgmt, server := runtimeRig(t)
	backup := runtimeBackup(1_000)
	now := metav1.Now()
	backup.Status.RuntimeRequestedTime = &now
	server.SetRuntimeState(1_000, "IN_PROGRESS", "")

	_, err := r.requestRuntimeBackup(t.Context(), mgmt, backup, &backup.Status.Runtime)
	require.NoError(t, err)

	assert.Zero(t, server.RuntimeStarts(1_000))
	assert.NotNil(t, backup.Status.RuntimeAcceptedTime)
	assert.Equal(t, v1.StepBackupRuntime, backup.Status.Step)
	assert.Empty(t, backup.Status.FailureMessage)
}

// Without the intent recorded, the reconcile records it and requests
// nothing. The intent must be durable before the request goes out.
func TestRuntimeRequestRecordsTheIntentFirst(t *testing.T) {
	r, mgmt, server := runtimeRig(t)
	backup := runtimeBackup(1_000)

	_, err := r.requestRuntimeBackup(t.Context(), mgmt, backup, &backup.Status.Runtime)
	require.NoError(t, err)

	assert.Zero(t, server.RuntimeStarts(1_000))
	assert.NotNil(t, backup.Status.RuntimeRequestedTime)
	assert.Nil(t, backup.Status.RuntimeAcceptedTime)
}
