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
	"slices"
	"strconv"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ZeebeRecordIndices is the index pattern of the exported Zeebe record
// indices. It is the default prefix of the legacy exporter, and the operator
// configures no other prefix (Camunda 8.9 "Configure Elasticsearch and
// OpenSearch index prefixes": global.elasticsearch.prefix defaults to
// zeebe-record).
const ZeebeRecordIndices = "zeebe-record*"

// camundaIndices are the index patterns of a Camunda 8.9 cluster that a
// restore replaces, without the Optimize indices. The Camunda Exporter writes
// the orchestration cluster indices under the legacy prefixes camunda,
// operate, and tasklist, and the operator sets no cluster index prefix
// (Camunda 8.9 "Schema and data migration": the index name is
// {cluster-index-prefix}-{legacy-prefix}-{datatype}-{schemaversion}). The
// legacy exporter writes the record indices.
//
// The list holds exactly what a backup restores. The web-application
// snapshots hold the camunda, operate, and tasklist indices, and the record
// snapshot holds the record indices (Camunda 8.9 "Web applications backup
// management API" and backup guide step 6).
var camundaIndices = []string{"camunda-*", "operate-*", "tasklist-*", ZeebeRecordIndices}

// optimizeIndices is the index pattern of the Optimize indices. Optimize
// prepends the prefix "optimize" to every index and alias name, and the
// operator sets no other prefix (Camunda 8.9 Optimize system configuration:
// es.settings.index.prefix defaults to optimize).
const optimizeIndices = "optimize-*"

// optimizeSnapshotMarker is the fragment that appears in the name of every
// Optimize snapshot and in no other Camunda snapshot name. An Optimize
// snapshot is camunda_optimize_<id>_<version>_part_N_of_M, and the
// web-application snapshots of the same backup are camunda_webapps_...
// (Camunda 8.9 restore guide, "find available backup IDs").
const optimizeSnapshotMarker = "_optimize_"

// RecordsSnapshotName returns the name of the Elasticsearch snapshot that
// holds the exported Zeebe record indices of a backup id. The backup writes
// it, and a restore locates it by the same rule.
func RecordsSnapshotName(id int64) string {
	return "camunda_zeebe_records_backup_" + strconv.FormatInt(id, 10)
}

// CamundaIndexPatterns returns the index patterns that a restore deletes from
// the target before it restores the snapshots. It includes the Optimize
// indices only when the backup holds Optimize snapshots. A backup without
// them cannot restore them, and deleting them erases Optimize data that the
// restore cannot put back. The answer is a new slice every time.
func CamundaIndexPatterns(withOptimize bool) []string {
	patterns := slices.Clone(camundaIndices)
	if withOptimize {
		patterns = append(patterns, optimizeIndices)
	}

	return patterns
}

// HasOptimizeSnapshot reports whether a backup's recorded snapshot names hold
// an Optimize snapshot.
func HasOptimizeSnapshot(names []string) bool {
	for _, name := range names {
		if strings.Contains(name, optimizeSnapshotMarker) {
			return true
		}
	}

	return false
}

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

// AllocateBackupIDAfter returns the identifier of a backup that starts at the
// given time and must come after highest: the larger of AllocateBackupID(at)
// and highest+1. A caller passes the highest id among the backups of the
// same cluster that it can see. A clock that stepped backwards then cannot
// hand out an id that one of them already holds. Ids that no visible backup
// records stay with the cluster as the arbiter. Those are the backups of the
// other kind and the backups whose resource is deleted. The cluster answers
// a repeated or lower id with camundaadmin.ErrConflict.
func AllocateBackupIDAfter(at metav1.Time, highest int64) int64 {
	return max(AllocateBackupID(at), highest+1)
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
