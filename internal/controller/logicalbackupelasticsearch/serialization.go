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

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/clusterclaim"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

// inProgress is the fairness pre-filter of admission. It reports another
// backup of this kind and the same cluster that goes first: a started
// sibling, or the older pending one by the tie-break. It orders who tries
// the claim first. The claim itself is the Lease that Claim takes, and only
// the Lease decides who holds the cluster. A backup that holds the claim
// already, on a re-entry after a failed flush, goes first whatever the
// tie-break says. Otherwise the tie-break and the claim block each other.
func (r *Reconciler) inProgress(backup *v1.LogicalBackupElasticsearch) logicalbackup.InProgress {
	return func(ctx context.Context) (string, error) {
		holds, err := clusterclaim.Holds(
			ctx,
			r.APIReader,
			backup.Namespace,
			backup.Spec.ClusterRef.Name,
			claimant(backup),
		)
		if err != nil {
			return "", err
		}
		if holds {
			return "", nil
		}

		cluster := clusterKey(backup)

		var list v1.LogicalBackupElasticsearchList
		if err := r.APIReader.List(ctx, &list, client.InNamespace(backup.Namespace)); err != nil {
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

		return "", nil
	}
}

// claimant is the identity under which the backup holds the claim on its
// cluster.
func claimant(backup *v1.LogicalBackupElasticsearch) clusterclaim.Claimant {
	return clusterclaim.Claimant{Kind: backup.GetKind(), Name: backup.Name, UID: backup.UID}
}

// claimCluster takes the claim on the cluster for the backup. It returns
// the waiting message of the holder that blocks, or "" when the backup
// holds the claim. It runs before the backup ID is allocated and flushed. A
// re-entry after a failed flush finds itself as the holder and proceeds. A
// holder that ended as ResumeFailed keeps the cluster paused, and the
// message says so. The user must delete or repair that backup. Waiting does
// not resolve it.
func (r *Reconciler) claimCluster(ctx context.Context, backup *v1.LogicalBackupElasticsearch) (string, error) {
	cluster := backup.Spec.ClusterRef.Name
	// Writes go through the client, reads through the API reader: the claim
	// is never decided from the cache.
	holder, err := clusterclaim.Claim(
		ctx, r.Client, r.APIReader, backup.Namespace, cluster, claimant(backup),
	)
	if err != nil {
		return "", fmt.Errorf("claiming CamundaCluster %s/%s: %w", backup.Namespace, cluster, err)
	}
	if holder == "" {
		return "", nil
	}

	parsed, err := clusterclaim.ParseClaimant(holder)
	if err != nil {
		return fmt.Sprintf(
			"%q holds CamundaCluster %s/%s; backups of one cluster run one at a time",
			holder, backup.Namespace, cluster,
		), nil
	}
	paused, err := clusterclaim.HolderKeepsClusterPaused(ctx, r.APIReader, backup.Namespace, parsed)
	if err != nil {
		return "", err
	}
	if paused {
		return fmt.Sprintf(
			"backup %s ended without resuming the exporting of CamundaCluster %s/%s; "+
				"the cluster is still paused and needs that backup's deletion or repair",
			parsed.Display(), backup.Namespace, cluster,
		), nil
	}
	return fmt.Sprintf(
		"backup %s of CamundaCluster %s/%s holds the cluster; backups of one cluster run one at a time",
		parsed.Display(), backup.Namespace, cluster,
	), nil
}

// releaseClaim gives the claim on the cluster back. It is a no-op when the
// backup does not hold it.
func (r *Reconciler) releaseClaim(ctx context.Context, backup *v1.LogicalBackupElasticsearch) error {
	err := clusterclaim.Release(
		ctx, r.Client, r.APIReader, backup.Namespace, backup.Spec.ClusterRef.Name, claimant(backup),
	)
	if err != nil {
		return fmt.Errorf("releasing CamundaCluster %s/%s: %w", backup.Namespace, backup.Spec.ClusterRef.Name, err)
	}
	return nil
}

// blocks reports whether other must run before backup. A sibling that holds
// an ID has begun work worth waiting for. Between two backups that have not
// started, the older one goes first, and the smaller name breaks a tie in
// the creation time. Both backups live in the namespace of the cluster, so
// the name is unique among them. The order is total, so two waiting backups
// can never deadlock on each other.
func blocks(other, backup *v1.LogicalBackupElasticsearch) bool {
	if other.Status.BackupID != 0 {
		return true
	}
	if !other.CreationTimestamp.Equal(&backup.CreationTimestamp) {
		return other.CreationTimestamp.Before(&backup.CreationTimestamp)
	}
	return other.Name < backup.Name
}
