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

// Finalizer guards the stored artifacts of a logical backup. While it is set,
// deleting the custom resource first deletes the snapshots or the dump that
// the backup produced.
//
// The label that names the BackupSchedule which created a backup is
// labels.BackupScheduleKey: pkg/labels is the one place that owns the label
// keys of this operator.
const Finalizer = "core.camunda.io/backup-artifacts"

// AllocateBackupID returns the identifier of a backup that starts at the
// given time: the Unix timestamp in milliseconds, the resolution at which the
// cluster generates ids of its own.
//
// Camunda requires the id to be greater than every id the cluster already
// holds, and an id can never be reused, not even after its backup is deleted.
// Neither this function nor the pre-checks guarantee that. The pre-checks only
// stop a second backup while one is running, so two backups of one cluster
// within the same tick still collide, and a clock that steps backwards
// defeats any timestamp. Milliseconds make a collision unlikely, not
// impossible. The cluster is the arbiter: it answers a repeated or lower id
// with camundaadmin.ErrConflict, and a caller must never resolve that by
// adopting the backup that already holds the id.
func AllocateBackupID(at metav1.Time) int64 {
	return at.UnixMilli()
}

// ObjectKeyPrefix returns the key prefix of every object that the backup id
// of the cluster namespace/name writes to the bucket:
// <basePath>/<namespace>/<name>/<id>. An empty basePath means the bucket
// root, and surrounding slashes of basePath are dropped so a key never
// carries an empty segment.
func ObjectKeyPrefix(basePath, namespace, cluster string, id int64) string {
	return ClusterPrefix(basePath, namespace, cluster) + "/" + strconv.FormatInt(id, 10)
}

// ClusterPrefix returns the layout prefix of one cluster inside the backup
// bucket: <basePath>/<namespace>/<cluster>, without a leading slash. It is
// the single definition of that layout: the Elasticsearch snapshot repository
// registers with it as base_path, and every object key of the RDBMS path is
// built under it, so the two can never disagree on where a cluster's backups
// live.
func ClusterPrefix(basePath, namespace, cluster string) string {
	segments := make([]string, 0, 3)
	if trimmed := strings.Trim(basePath, "/"); trimmed != "" {
		segments = append(segments, trimmed)
	}
	segments = append(segments, namespace, cluster)

	return strings.Join(segments, "/")
}
