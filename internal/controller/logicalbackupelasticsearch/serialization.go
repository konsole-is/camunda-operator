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

package logicalbackupelasticsearch

import (
	"context"
	"fmt"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

// inProgress reports another non-terminal backup of the same cluster. It
// checks this kind itself. It checks the sibling kind when the manager wires
// SiblingInProgress.
func (r *Reconciler) inProgress(backup *v1.LogicalBackupElasticsearch) logicalbackup.InProgress {
	return func(ctx context.Context) (string, error) {
		cluster := clusterKey(backup)

		var list v1.LogicalBackupElasticsearchList
		if err := r.APIReader.List(ctx, &list); err != nil {
			return "", fmt.Errorf("listing LogicalBackupElasticsearch: %w", err)
		}

		for i := range list.Items {
			other := &list.Items[i]
			if other.UID == backup.UID || other.Terminal() {
				continue
			}
			if clusterKey(other) != cluster {
				continue
			}
			if blocks(other, backup) {
				return other.Name, nil
			}
		}

		if r.options.SiblingInProgress == nil {
			return "", nil
		}

		return r.options.SiblingInProgress(ctx, cluster)
	}
}

// blocks reports whether other must run before backup. A sibling that holds
// an ID has begun work worth waiting for. Between two backups that have not
// started, the older one goes first, and the smaller name breaks a tie in the
// creation time. The order is deterministic, so two waiting backups can never
// deadlock on each other.
func blocks(other, backup *v1.LogicalBackupElasticsearch) bool {
	if other.Status.BackupID != 0 {
		return true
	}
	if !other.CreationTimestamp.Equal(&backup.CreationTimestamp) {
		return other.CreationTimestamp.Before(&backup.CreationTimestamp)
	}
	return other.Name < backup.Name
}
