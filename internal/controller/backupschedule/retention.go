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
	"context"
	"fmt"
	"sort"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// The documented defaults of spec.retained. The schema fills them on
// admission; these constants cover an object that never passed it.
const (
	defaultRetainedCompleted int32 = 7
	defaultRetainedFailed    int32 = 3
)

// retainedBounds returns spec.retained with the documented defaults applied.
func retainedBounds(spec v1.BackupScheduleSpec) (completed, failed int32) {
	completed, failed = defaultRetainedCompleted, defaultRetainedFailed
	if spec.Retained == nil {
		return completed, failed
	}
	if spec.Retained.Completed != nil {
		completed = *spec.Retained.Completed
	}
	if spec.Retained.Failed != nil {
		failed = *spec.Retained.Failed
	}

	return completed, failed
}

// scheduledBackup is one backup that a schedule created, reduced to what the
// overlap check and the pruning need: the object to delete, its kind for the
// messages, its phase, and when it completed.
type scheduledBackup struct {
	object    client.Object
	kind      string
	phase     v1.LogicalBackupPhase
	completed metav1.Time
}

// terminal reports whether the backup can never transition again.
func (b scheduledBackup) terminal() bool {
	return b.phase == v1.LogicalBackupCompleted || b.phase == v1.LogicalBackupFailed
}

// scheduledBackups lists the backups of both kinds that carry the schedule's
// label, live: the pruning deletes data and the overlap check starts backups,
// so neither decides on a stale list. A terminal backup without a completion
// time sorts by its creation time, so it still ages out.
func (r *BackupScheduleReconciler) scheduledBackups(
	ctx context.Context,
	schedule *v1.BackupSchedule,
) ([]scheduledBackup, error) {
	scope := []client.ListOption{
		client.InNamespace(schedule.Namespace),
		client.MatchingLabels{labels.BackupScheduleKey: schedule.Name},
	}

	var es v1.LogicalBackupElasticsearchList
	if err := r.APIReader.List(ctx, &es, scope...); err != nil {
		return nil, fmt.Errorf("listing the LogicalBackupElasticsearch backups of the schedule: %w", err)
	}

	var rdbms v1.LogicalBackupRDBMSList
	if err := r.APIReader.List(ctx, &rdbms, scope...); err != nil {
		return nil, fmt.Errorf("listing the LogicalBackupRDBMS backups of the schedule: %w", err)
	}

	items := make([]scheduledBackup, 0, len(es.Items)+len(rdbms.Items))
	for i := range es.Items {
		backup := &es.Items[i]
		items = append(items, scheduledBackup{
			object:    backup,
			kind:      backup.GetKind(),
			phase:     backup.Status.Phase,
			completed: completedAt(backup.Status.CompletionTime, backup.CreationTimestamp),
		})
	}
	for i := range rdbms.Items {
		backup := &rdbms.Items[i]
		items = append(items, scheduledBackup{
			object:    backup,
			kind:      backup.GetKind(),
			phase:     backup.Status.Phase,
			completed: completedAt(backup.Status.CompletionTime, backup.CreationTimestamp),
		})
	}

	return items, nil
}

func completedAt(completion *metav1.Time, created metav1.Time) metav1.Time {
	if completion != nil {
		return *completion
	}

	return created
}

// nonTerminal returns the name of a backup that has not reached a terminal
// phase, or the empty string when every one has. One such backup skips the
// next trigger. The backup of exceptKind named exceptName does not count: it
// is the backup of the trigger under decision, which a crashed earlier
// attempt already created, and a trigger must not skip itself. The kind is
// part of the key because both kinds share the name scheme, and a same-named
// backup of the other kind is another trigger's backup, not this one's.
func nonTerminal(items []scheduledBackup, exceptKind, exceptName string) string {
	for _, item := range items {
		if item.kind == exceptKind && item.object.GetName() == exceptName {
			continue
		}
		if !item.terminal() {
			return item.object.GetName()
		}
	}

	return ""
}

// prunable returns the backups to delete: the oldest completed ones beyond
// the completed bound and the oldest failed ones beyond the failed bound,
// each phase sorted by completion time. A backup that is not terminal is
// never a candidate.
func prunable(items []scheduledBackup, completed, failed int32) []scheduledBackup {
	byPhase := map[v1.LogicalBackupPhase][]scheduledBackup{}
	for _, item := range items {
		if item.terminal() {
			byPhase[item.phase] = append(byPhase[item.phase], item)
		}
	}

	var overflow []scheduledBackup
	for phase, bound := range map[v1.LogicalBackupPhase]int32{
		v1.LogicalBackupCompleted: completed,
		v1.LogicalBackupFailed:    failed,
	} {
		// The schema bounds spec.retained below, but this function takes
		// plain integers. A negative bound reads as zero instead of a slice
		// panic.
		bound = max(bound, 0)
		candidates := byPhase[phase]
		if len(candidates) <= int(bound) {
			continue
		}
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].completed.Time.Before(candidates[j].completed.Time)
		})
		overflow = append(overflow, candidates[:len(candidates)-int(bound)]...)
	}

	return overflow
}

// prune deletes the backups of the schedule beyond spec.retained. Each delete
// carries a UID precondition, so a backup that was deleted and re-created
// under the same name in between is never touched. The deletion goes through
// the backup finalizer, which removes the stored artifacts first.
func (r *BackupScheduleReconciler) prune(ctx context.Context, schedule *v1.BackupSchedule) error {
	items, err := r.scheduledBackups(ctx, schedule)
	if err != nil {
		return err
	}

	completed, failed := retainedBounds(schedule.Spec)
	for _, item := range prunable(items, completed, failed) {
		uid := item.object.GetUID()
		err := r.Delete(ctx, item.object, client.Preconditions{UID: &uid})
		if apierrors.IsNotFound(err) || apierrors.IsConflict(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("pruning %s %q: %w", item.kind, item.object.GetName(), err)
		}
		r.EventRecorder.Eventf(
			schedule,
			nil,
			corev1.EventTypeNormal,
			eventReasonBackupPruned,
			eventActionPrune,
			"Deleted %s %q beyond spec.retained; its finalizer removes the stored artifacts",
			item.kind,
			item.object.GetName(),
		)
	}

	return nil
}
