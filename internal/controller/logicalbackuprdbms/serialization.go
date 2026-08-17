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

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
)

// inProgress serializes the backups of one cluster with a deterministic
// entry gate: an already-running backup blocks everything else, and among
// the pending ones only the oldest (creation time, then name) may start.
// Both halves read live state, so two backups admitted from a stale cache
// cannot both start. The sibling seam follows the same rule from the other
// side: it reports only started backups, so a pending sibling never blocks.
func (r *LogicalBackupRDBMSReconciler) inProgress(backup *v1.LogicalBackupRDBMS) logicalbackup.InProgress {
	return func(ctx context.Context) (string, error) {
		cluster := types.NamespacedName{Namespace: backup.Namespace, Name: backup.Spec.ClusterRef.Name}

		// A ClusterRef never crosses namespaces, so the backups of one
		// cluster all live in its namespace.
		var list v1.LogicalBackupRDBMSList
		if err := r.APIReader.List(ctx, &list, client.InNamespace(backup.Namespace)); err != nil {
			return "", err
		}
		for i := range list.Items {
			other := &list.Items[i]
			if other.UID == backup.UID || other.Terminal() {
				continue
			}
			if other.Spec.ClusterRef.Name != cluster.Name {
				continue
			}

			if other.Status.BackupID != 0 || olderBackup(other, backup) {
				return other.Name, nil
			}
		}

		if r.opts.SiblingInProgress == nil {
			return "", nil
		}

		return r.opts.SiblingInProgress(ctx, cluster)
	}
}

// olderBackup reports whether a was created before b, with the name as the
// tie-break, so exactly one of two pending backups ever starts first.
func olderBackup(a, b *v1.LogicalBackupRDBMS) bool {
	if !a.CreationTimestamp.Equal(&b.CreationTimestamp) {
		return a.CreationTimestamp.Before(&b.CreationTimestamp)
	}

	return a.Name < b.Name
}

// clusterKey is the index value of one backup: the namespace and name of the
// cluster it references, which is the backup's own namespace.
func clusterKey(backup *v1.LogicalBackupRDBMS) string {
	return refindex.NamespacedKey(backup.Namespace, backup.Spec.ClusterRef.Name)
}
