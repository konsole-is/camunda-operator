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

package backupschedule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestRetainedBounds(t *testing.T) {
	t.Run("applies the documented defaults to an undefaulted spec", func(t *testing.T) {
		completed, failed := retainedBounds(v1.BackupScheduleSpec{})
		assert.Equal(t, int32(7), completed)
		assert.Equal(t, int32(3), failed)
	})

	t.Run("keeps an explicit zero for failed", func(t *testing.T) {
		completed, failed := retainedBounds(v1.BackupScheduleSpec{
			Retained: &v1.RetainedBackups{Failed: ptr.To(int32(0))},
		})
		assert.Equal(t, int32(7), completed)
		assert.Equal(t, int32(0), failed)
	})
}

func TestPrunable(t *testing.T) {
	at := func(minute int) metav1.Time {
		return metav1.NewTime(time.Date(2026, 8, 20, 2, minute, 0, 0, time.UTC))
	}
	item := func(name string, phase v1.LogicalBackupPhase, completed metav1.Time) scheduledBackup {
		return scheduledBackup{
			object:    &v1.LogicalBackupRDBMS{ObjectMeta: metav1.ObjectMeta{Name: name}},
			phase:     phase,
			completed: completed,
		}
	}
	names := func(items []scheduledBackup) []string {
		out := make([]string, 0, len(items))
		for _, it := range items {
			out = append(out, it.object.GetName())
		}
		return out
	}

	t.Run("deletes the oldest of each terminal phase beyond its bound", func(t *testing.T) {
		items := []scheduledBackup{
			item("c1", v1.LogicalBackupCompleted, at(1)),
			item("c2", v1.LogicalBackupCompleted, at(2)),
			item("c3", v1.LogicalBackupCompleted, at(3)),
			item("f1", v1.LogicalBackupFailed, at(1)),
			item("f2", v1.LogicalBackupFailed, at(2)),
		}
		assert.ElementsMatch(t, []string{"c1", "c2", "f1"}, names(prunable(items, 1, 1)))
	})

	t.Run("never touches a non-terminal backup, whatever the bounds", func(t *testing.T) {
		items := []scheduledBackup{
			item("pending", v1.LogicalBackupPending, at(1)),
			item("running", v1.LogicalBackupRunning, at(2)),
			item("zero", "", at(3)),
		}
		assert.Empty(t, prunable(items, 0, 0))
	})

	t.Run("a zero failed bound deletes every failed backup", func(t *testing.T) {
		items := []scheduledBackup{item("f1", v1.LogicalBackupFailed, at(1))}
		assert.ElementsMatch(t, []string{"f1"}, names(prunable(items, 7, 0)))
	})

	t.Run("a negative bound deletes everything of the phase and never panics", func(t *testing.T) {
		items := []scheduledBackup{
			item("c1", v1.LogicalBackupCompleted, at(1)),
			item("f1", v1.LogicalBackupFailed, at(1)),
		}
		assert.ElementsMatch(t, []string{"c1", "f1"}, names(prunable(items, -1, -5)))
	})

	t.Run("bounds at or above the count delete nothing", func(t *testing.T) {
		items := []scheduledBackup{
			item("c1", v1.LogicalBackupCompleted, at(1)),
			item("c2", v1.LogicalBackupCompleted, at(2)),
		}
		assert.Empty(t, prunable(items, 2, 3))
	})
}

func TestNonTerminal(t *testing.T) {
	item := func(kind, name string, phase v1.LogicalBackupPhase) scheduledBackup {
		return scheduledBackup{
			object: &v1.LogicalBackupRDBMS{ObjectMeta: metav1.ObjectMeta{Name: name}},
			kind:   kind,
			phase:  phase,
		}
	}
	running := item("LogicalBackupRDBMS", "nightly-100", v1.LogicalBackupRunning)

	t.Run("names a non-terminal backup and ignores terminal ones", func(t *testing.T) {
		items := []scheduledBackup{
			item("LogicalBackupRDBMS", "done", v1.LogicalBackupCompleted),
			running,
		}
		assert.Equal(t, "nightly-100", nonTerminal(items, "LogicalBackupRDBMS", "other"))
	})

	t.Run("exempts the backup of the trigger under decision", func(t *testing.T) {
		assert.Empty(
			t,
			nonTerminal([]scheduledBackup{running}, "LogicalBackupRDBMS", "nightly-100"),
		)
	})

	t.Run("a same-named backup of the other kind is not exempted", func(t *testing.T) {
		// A crash mid-trigger and a storage repoint before the retry leave a
		// non-terminal backup of the other kind under the trigger's name. It
		// must still block the trigger.
		assert.Equal(
			t,
			"nightly-100",
			nonTerminal([]scheduledBackup{running}, "LogicalBackupElasticsearch", "nightly-100"),
		)
	})
}

func TestCompletedAt(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 8, 20, 2, 0, 0, 0, time.UTC))
	completion := metav1.NewTime(created.Add(time.Hour))

	assert.Equal(t, completion, completedAt(&completion, created))
	// A terminal backup that lost its completion time still ages out by its
	// creation time instead of being unprunable.
	assert.Equal(t, created, completedAt(nil, created))
}
