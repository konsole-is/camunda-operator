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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
)

func dumpScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	return scheme
}

func trackedBackup() *v1.LogicalBackupRDBMS {
	backup := &v1.LogicalBackupRDBMS{
		ObjectMeta: metav1.ObjectMeta{Name: "backup-1", Namespace: "camunda"},
		Spec:       v1.LogicalBackupRDBMSSpec{ClusterRef: v1.ClusterRef{Name: "my-cluster"}},
	}
	backup.Status.BackupID = 1748937221000
	backup.Status.Phase = v1.LogicalBackupRunning
	backup.Status.Step = v1.StepDumping
	backup.Status.JobName = components.JobName(backup)

	return backup
}

// TestDumpTrustsTheLiveViewOverAStaleCache pins the read-your-writes gap:
// right after the apply, the informer cache may not hold the Job yet. A
// cached NotFound alone must never terminally fail the backup while the live
// view still sees the Job.
func TestDumpTrustsTheLiveViewOverAStaleCache(t *testing.T) {
	t.Parallel()

	scheme := dumpScheme(t)
	backup := trackedBackup()
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: backup.Status.JobName, Namespace: backup.Namespace,
	}}

	r := &LogicalBackupRDBMSReconciler{
		// The cache lost the race: no Job. The live view has it.
		Client:    fake.NewClientBuilder().WithScheme(scheme).Build(),
		APIReader: fake.NewClientBuilder().WithScheme(scheme).WithObjects(job).Build(),
	}

	wait, err := r.dump(context.Background(), backup)
	require.NoError(t, err)
	assert.NotEqual(t, v1.LogicalBackupFailed, backup.Status.Phase)
	assert.Equal(t, v1.StepDumping, backup.Status.Step)
	assert.NotZero(t, wait.after)
}

// TestDumpFailsWhenTheJobIsGoneFromTheLiveView is the true hand-deletion
// case: both views agree the Job is gone, so the dump cannot be trusted to
// have uploaded.
func TestDumpFailsWhenTheJobIsGoneFromTheLiveView(t *testing.T) {
	t.Parallel()

	scheme := dumpScheme(t)
	backup := trackedBackup()

	r := &LogicalBackupRDBMSReconciler{
		Client:        fake.NewClientBuilder().WithScheme(scheme).Build(),
		APIReader:     fake.NewClientBuilder().WithScheme(scheme).Build(),
		EventRecorder: events.NewFakeRecorder(4),
	}

	wait, err := r.dump(context.Background(), backup)
	require.NoError(t, err)
	assert.Equal(t, v1.LogicalBackupFailed, backup.Status.Phase)
	assert.Contains(t, backup.Status.FailureMessage, "disappeared")
	assert.Equal(t, settle, wait)
}
