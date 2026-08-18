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

package logicalbackuprdbms

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin/camundaadmintest"
)

// TestStartZeebeBackupConflictClearsTheFailureClock pins that an answered
// request clears the mid-run failure clock, the conflict answer included. An
// earlier outage sets the clock. A conflict is an answer from a reachable
// API. Without the clear, the next unreachable answer counts from the old
// outage and can fail the step on one bad answer.
func TestStartZeebeBackupConflictClearsTheFailureClock(t *testing.T) {
	t.Parallel()

	server := camundaadmintest.New()
	t.Cleanup(server.Close)
	server.ConflictNextRuntimeStart(1)
	admin, err := camundaadmin.New(camundaadmin.Binding{Endpoint: server.URL(), Version: "8.9.9"})
	require.NoError(t, err)

	backup := trackedBackup()
	backup.Status.Step = v1.StepZeebeBackup
	stale := metav1.NewTime(time.Now().Add(-time.Hour))
	backup.Status.FirstFailedAt = &stale
	r := &LogicalBackupRDBMSReconciler{opts: Options{MidRunGrace: time.Minute}}

	_, err = r.startZeebeBackup(context.Background(), backup, &v1.CamundaCluster{}, admin)
	require.ErrorIs(t, err, camundaadmin.ErrConflict)
	assert.Nil(t, backup.Status.ZeebeBackupID, "the conflicting id is never adopted")
	assert.Nil(t, backup.Status.FirstFailedAt, "the answer clears the clock of the earlier outage")
}
