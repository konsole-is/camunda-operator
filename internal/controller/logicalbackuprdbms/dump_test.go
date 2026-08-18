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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
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
	backup.UID = "uid-of-this-backup"
	backup.Status.BackupID = 1748937221000
	backup.Status.Phase = v1.LogicalBackupRunning
	backup.Status.Step = v1.StepDumping
	backup.Status.JobName = components.JobName(backup)

	return backup
}

// foreignUID marks a Job or pod of another backup in the fixtures.
const foreignUID = "uid-of-someone-else"

// ownJob is the Job of backup as BuildJob stamps it: named after the backup
// and carrying its UID.
func ownJob(backup *v1.LogicalBackupRDBMS) *batchv1.Job {
	return &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name: backup.Status.JobName, Namespace: backup.Namespace,
		Labels: map[string]string{components.BackupUIDLabel: string(backup.UID)},
	}}
}

// TestDumpTrustsTheLiveViewOverAStaleCache pins the read-your-writes gap:
// right after the apply, the informer cache may not hold the Job yet. A
// cached NotFound alone must never terminally fail the backup while the live
// view still sees the Job.
func TestDumpTrustsTheLiveViewOverAStaleCache(t *testing.T) {
	t.Parallel()

	scheme := dumpScheme(t)
	backup := trackedBackup()
	job := ownJob(backup)

	r := &LogicalBackupRDBMSReconciler{
		// The cache lost the race: no Job. The live view has it.
		Client:    fake.NewClientBuilder().WithScheme(scheme).Build(),
		APIReader: fake.NewClientBuilder().WithScheme(scheme).WithObjects(job).Build(),
		opts:      Options{RetryInterval: time.Second},
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
		opts:          Options{RetryInterval: time.Second},
	}

	wait, err := r.dump(context.Background(), backup)
	require.NoError(t, err)
	assert.Equal(t, v1.LogicalBackupFailed, backup.Status.Phase)
	assert.Contains(t, backup.Status.FailureMessage, "disappeared")
	assert.Equal(t, settle, wait)
}

// TestDumpNeverAdoptsAnotherBackupsJob pins the identity check: a Job under
// this backup's name that carries another UID belongs to another backup —
// tracking it would let this backup advance without a dump of its own, so it
// is a hard failure naming the conflicting Job.
func TestDumpNeverAdoptsAnotherBackupsJob(t *testing.T) {
	t.Parallel()

	scheme := dumpScheme(t)
	backup := trackedBackup()
	stranger := ownJob(backup)
	stranger.Labels[components.BackupUIDLabel] = foreignUID

	r := &LogicalBackupRDBMSReconciler{
		Client:        fake.NewClientBuilder().WithScheme(scheme).WithObjects(stranger).Build(),
		APIReader:     fake.NewClientBuilder().WithScheme(scheme).WithObjects(stranger).Build(),
		EventRecorder: events.NewFakeRecorder(4),
		opts:          Options{RetryInterval: time.Second},
	}

	_, err := r.dump(context.Background(), backup)
	require.NoError(t, err)
	assert.Equal(t, v1.LogicalBackupFailed, backup.Status.Phase)
	assert.Contains(t, backup.Status.FailureMessage, "belongs to another backup")
	assert.Contains(t, backup.Status.FailureMessage, stranger.Name)
}

// TestStuckPodClassifiesWaitingStates pins which pod states count as stuck:
// the waiting reasons the kubelet retries forever, and an unschedulable pod;
// a plain Pending pod without either is still progressing.
func TestStuckPodClassifiesWaitingStates(t *testing.T) {
	t.Parallel()

	scheme := dumpScheme(t)
	backup := trackedBackup()
	podOf := func(name string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
			Name: name, Namespace: backup.Namespace,
			Labels: map[string]string{components.BackupUIDLabel: string(backup.UID)},
		}, Status: corev1.PodStatus{Phase: corev1.PodPending}}
	}

	progressing := podOf("fine")
	progressing.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name: "upload", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "PodInitializing"}},
	}}
	pulling := podOf("pull")
	pulling.Status.InitContainerStatuses = []corev1.ContainerStatus{{
		Name: "dump", State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
			Reason: "ImagePullBackOff", Message: "postgres:99 not found",
		}},
	}}
	unschedulable := podOf("unbound")
	unschedulable.Status.Conditions = []corev1.PodCondition{{
		Type: corev1.PodScheduled, Status: corev1.ConditionFalse,
		Reason: corev1.PodReasonUnschedulable, Message: "unbound immediate PersistentVolumeClaims",
	}}

	cases := map[string]struct {
		pod    *corev1.Pod
		reason string
		text   string
	}{
		"progressing":   {pod: progressing},
		"image pull":    {pod: pulling, reason: v1.ReasonInvalidReference, text: "ImagePullBackOff"},
		"unschedulable": {pod: unschedulable, reason: v1.ReasonProgressing, text: "unbound"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := &LogicalBackupRDBMSReconciler{
				APIReader: fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.pod).Build(),
			}
			failure, err := r.stuckPod(context.Background(), backup)
			require.NoError(t, err)
			if tc.reason == "" {
				assert.Nil(t, failure)

				return
			}
			require.NotNil(t, failure)
			assert.Equal(t, tc.reason, failure.Reason)
			assert.Contains(t, failure.Message, tc.text)
			assert.Contains(t, failure.Message, tc.pod.Name)
		})
	}
}
