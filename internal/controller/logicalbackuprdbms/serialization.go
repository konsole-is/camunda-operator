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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
)

// inProgress is the fairness pre-filter of admission. It reports another
// backup of this kind and the same cluster that goes first: a started
// sibling, or the older pending one by the tie-break. It orders who tries
// the claim first. The claim itself is the Lease that Claim takes. Only the
// Lease decides who holds the cluster, across both backup kinds. A backup
// that holds the claim already, on a re-entry after a failed flush, goes
// first whatever the tie-break says. Otherwise the tie-break and the claim
// block each other.
func (r *LogicalBackupRDBMSReconciler) inProgress(backup *v1.LogicalBackupRDBMS) logicalbackup.InProgress {
	return func(ctx context.Context) (string, error) {
		holds, err := logicalbackup.Holds(
			ctx, r.APIReader, backup.Namespace, backup.Spec.ClusterRef.Name, claimant(backup),
		)
		if err != nil {
			return "", err
		}
		if holds {
			return "", nil
		}

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
			if other.Spec.ClusterRef.Name != backup.Spec.ClusterRef.Name {
				continue
			}
			if blocks(other, backup) {
				return other.Name, nil
			}
		}

		return "", nil
	}
}

// blocks reports whether other must run before backup. A sibling that holds
// an id started work that is worth the wait. Between two backups that did
// not start, the older one goes first, and the smaller name breaks a tie in
// the creation time. Both live in the namespace of the cluster, so the name
// is unique among them. The order is total, so two waiting backups can never
// deadlock on each other.
func blocks(other, backup *v1.LogicalBackupRDBMS) bool {
	if other.Status.BackupID != 0 {
		return true
	}
	if !other.CreationTimestamp.Equal(&backup.CreationTimestamp) {
		return other.CreationTimestamp.Before(&backup.CreationTimestamp)
	}

	return other.Name < backup.Name
}

// claimant is the identity under which the backup holds the claim on its
// cluster.
func claimant(backup *v1.LogicalBackupRDBMS) logicalbackup.Claimant {
	return logicalbackup.Claimant{Kind: backup.GetKind(), Name: backup.Name, UID: backup.UID}
}

// claimCluster takes the claim on the cluster for the backup. It returns
// the display name of the holder that blocks, or "" when the backup holds
// the claim. It runs before the controller allocates and flushes the backup
// id. A re-entry after a failed flush finds itself as the holder and proceeds.
func (r *LogicalBackupRDBMSReconciler) claimCluster(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) (string, error) {
	holder, err := logicalbackup.Claim(
		ctx, r.Client, r.APIReader, backup.Namespace, backup.Spec.ClusterRef.Name, claimant(backup),
	)
	if err != nil {
		return "", fmt.Errorf(
			"claiming CamundaCluster %s/%s: %w", backup.Namespace, backup.Spec.ClusterRef.Name, err,
		)
	}
	if holder == "" {
		return "", nil
	}
	if parsed, err := logicalbackup.ParseClaimant(holder); err == nil {
		return parsed.Display(), nil
	}

	return holder, nil
}

// releaseClaim gives the claim on the cluster back. It is a no-op when the
// backup does not hold it.
func (r *LogicalBackupRDBMSReconciler) releaseClaim(
	ctx context.Context,
	backup *v1.LogicalBackupRDBMS,
) error {
	err := logicalbackup.Release(
		ctx, r.Client, r.APIReader, backup.Namespace, backup.Spec.ClusterRef.Name, claimant(backup),
	)
	if err != nil {
		return fmt.Errorf(
			"releasing CamundaCluster %s/%s: %w", backup.Namespace, backup.Spec.ClusterRef.Name, err,
		)
	}

	return nil
}

// clusterKey is the index value of one backup: the namespace and name of the
// cluster it references, which is the backup's own namespace.
func clusterKey(backup *v1.LogicalBackupRDBMS) string {
	return refindex.NamespacedKey(backup.Namespace, backup.Spec.ClusterRef.Name)
}
