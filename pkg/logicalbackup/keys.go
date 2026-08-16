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

// Package logicalbackup is the shared skeleton of the two logical backup
// kinds, LogicalBackupElasticsearch and LogicalBackupRDBMS: the reconcile
// pre-checks they run in the same order, the identifiers and key layout their
// artifacts carry, and the effective restore sizes they record. It is pure
// logic and reads the API server only through the reader the caller passes
// in; it renders nothing and writes nothing.
package logicalbackup

import (
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Finalizer guards the stored artifacts of a logical backup. While it is
	// set, deleting the custom resource first deletes the snapshots or the
	// dump that the backup produced.
	Finalizer = "core.camunda.io/backup-artifacts"
	// ScheduleLabel names the BackupSchedule that created a backup. Retention
	// counts and prunes only the backups that carry it, so a backup created
	// by hand is never pruned by a policy.
	ScheduleLabel = "camunda.io/backup-schedule"
)

// AllocateBackupID returns the identifier of a backup that starts at the
// given time: the Unix timestamp in seconds. It is monotonic per cluster
// because the pre-checks let only one backup of a cluster run at a time.
func AllocateBackupID(at metav1.Time) int64 {
	return at.Unix()
}

// ObjectKeyPrefix returns the key prefix of every object that the backup id
// of the cluster namespace/name writes to the bucket:
// <basePath>/<namespace>/<name>/<id>. An empty basePath means the bucket
// root, and surrounding slashes of basePath are dropped so a key never
// carries an empty segment.
func ObjectKeyPrefix(basePath, namespace, cluster string, id int64) string {
	segments := make([]string, 0, 4)
	if trimmed := strings.Trim(basePath, "/"); trimmed != "" {
		segments = append(segments, trimmed)
	}
	segments = append(segments, namespace, cluster, strconv.FormatInt(id, 10))

	return strings.Join(segments, "/")
}
