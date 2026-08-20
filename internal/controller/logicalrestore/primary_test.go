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

package logicalrestore

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// TestRestoreArgs pins the arguments of the restore application. The
// Elasticsearch path names the backup id, because the id keys every snapshot
// of the set, and the relational path names nothing, because the application
// reads the exporter position from the restored database itself (Camunda 8.9
// restore guides).
func TestRestoreArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		storage v1.SecondaryStorageType
		want    []string
	}{
		{
			name:    "the elasticsearch path names the backup id",
			storage: v1.SecondaryStorageTypeElasticsearch,
			want:    []string{"--backupId=1772001869309"},
		},
		{
			name:    "the relational path names nothing",
			storage: v1.SecondaryStorageTypeRDBMS,
			want:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			restore := &v1.LogicalRestore{}
			restore.Status.StorageType = tt.storage
			restore.Status.BackupID = 1772001869309

			assert.Equal(t, tt.want, restoreArgs(restore))
		})
	}
}

// TestJobProgressedAfter pins what counts as progress of the restore Jobs. A
// Job that finished after the clock of the mid-run grace started is progress:
// the phase moved while the failure held. A Job that finished before it is
// not, because it finished before the failure began.
func TestJobProgressedAfter(t *testing.T) {
	t.Parallel()

	clock := metav1.NewTime(time.Now())
	before := metav1.NewTime(clock.Add(-time.Minute))
	after := metav1.NewTime(clock.Add(time.Minute))

	completed := func(at *metav1.Time) batchv1.Job {
		return batchv1.Job{Status: batchv1.JobStatus{CompletionTime: at}}
	}

	tests := []struct {
		name  string
		since *metav1.Time
		jobs  []batchv1.Job
		want  bool
	}{
		{
			name: "no clock runs, so nothing has to be cleared",
			jobs: []batchv1.Job{completed(&after)},
		},
		{
			name:  "a Job that finished after the clock started",
			since: &clock,
			jobs:  []batchv1.Job{completed(&before), completed(&after)},
			want:  true,
		},
		{
			name:  "a Job that finished in the second the clock started",
			since: &clock,
			jobs:  []batchv1.Job{completed(&clock)},
			want:  true,
		},
		{
			name:  "every Job finished before the clock started",
			since: &clock,
			jobs:  []batchv1.Job{completed(&before)},
		},
		{
			name:  "no Job finished at all",
			since: &clock,
			jobs:  []batchv1.Job{completed(nil)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, jobProgressedAfter(tt.jobs, tt.since))
		})
	}
}
