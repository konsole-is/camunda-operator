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

package logicalrestoreelasticsearch

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/restore"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

const (
	// eventReasonRecoveryHeld records that a restore keeps its backend while
	// Elasticsearch recovers the snapshots that the restore asked for.
	eventReasonRecoveryHeld = "RecoveryHeld"
	// eventReasonRecoveryEnded records that the recovery ended and the
	// restore gave its backend back.
	eventReasonRecoveryEnded = "RecoveryEnded"
	// eventReasonRecoveryUnknown records that the restore gave its backend
	// back without reading the end of the recovery.
	eventReasonRecoveryUnknown = "RecoveryUnknown"
)

// finalize ends a deleted restore. It returns how long to wait before the
// next look, or zero once the finalizer is gone.
func (r *Reconciler) finalize(ctx context.Context, lres *v1.LogicalRestoreElasticsearch) (time.Duration, error) {
	if !lres.Terminal() {
		r.holdForRecovery(lres)
	}
	wait, err := r.holdRecovery(ctx, lres)
	if err != nil || wait > 0 {
		return wait, err
	}

	if lres.Status.Backend != "" {
		err := restore.ReleaseWriter(
			ctx, r.Client, r.opts.ClaimNamespace, lres.Status.Backend, lres, lres.Status.TargetClusterUID,
		)
		if err != nil {
			return 0, err
		}
	}

	finalized, err := restore.FinalizeHold(
		ctx,
		r.Client,
		r.APIReader,
		lres,
		labels.LogicalRestoreElasticsearch(lres.Name),
		lres.Spec.TargetClusterRef.Name,
	)

	return finalized.Wait, err
}

// holdForRecovery marks a restore that ends while Elasticsearch can still
// recover its snapshots as held.
func (r *Reconciler) holdForRecovery(lres *v1.LogicalRestoreElasticsearch) {
	if lres.Status.Repository == "" || lres.Status.RecoveryHeld {
		return
	}

	lres.Status.RecoveryHeld = true
	r.EventRecorder.Eventf(
		lres,
		nil,
		corev1.EventTypeNormal,
		eventReasonRecoveryHeld,
		restore.EventActionRestore,
		"The restore keeps its backend until Elasticsearch finishes the recovery of the restored indices",
	)
}

// holdRecovery keeps a held restore on its backend while Elasticsearch
// recovers the restored indices. It returns how long to wait before the next
// look, or zero once the hold is over and the caller can release the backend.
func (r *Reconciler) holdRecovery(ctx context.Context, lres *v1.LogicalRestoreElasticsearch) (time.Duration, error) {
	if !lres.Status.RecoveryHeld {
		return 0, nil
	}

	recovering, failure, err := r.readRecovery(ctx, lres)
	if err != nil {
		return 0, err
	}

	now := metav1.Now()
	switch {
	case failure != nil:
		if lres.Status.RecoveryUnknownSince == nil {
			lres.Status.RecoveryUnknownSince = &now
		}
		if now.Sub(lres.Status.RecoveryUnknownSince.Time) < r.opts.MidRunGrace {
			return r.keepWriter(ctx, lres)
		}
		r.EventRecorder.Eventf(
			lres,
			nil,
			corev1.EventTypeWarning,
			eventReasonRecoveryUnknown,
			restore.EventActionRestore,
			"The restore gives its backend back, but Elasticsearch can still recover the restored indices. "+
				"It did not read the recovery for %s: %s",
			r.opts.MidRunGrace,
			failure.Message,
		)
	case recovering:
		lres.Status.RecoveryUnknownSince = nil

		return r.keepWriter(ctx, lres)
	default:
		r.EventRecorder.Eventf(
			lres,
			nil,
			corev1.EventTypeNormal,
			eventReasonRecoveryEnded,
			restore.EventActionRestore,
			"Elasticsearch finished the recovery of the restored indices, and the restore gives its backend back",
		)
	}

	lres.Status.RecoveryHeld = false
	lres.Status.RecoveryUnknownSince = nil

	return 0, nil
}

func (r *Reconciler) keepWriter(ctx context.Context, lres *v1.LogicalRestoreElasticsearch) (time.Duration, error) {
	if lres.Status.Backend == "" {
		return r.opts.PollInterval, nil
	}
	// A look that released the registration can crash before the cleared hold is
	// in status, so the next look registers it again rather than only renewing it.
	err := restore.RegisterWriter(
		ctx, r.Client, r.APIReader, r.opts.ClaimNamespace, lres.Status.Backend, lres, lres.Status.TargetClusterUID,
	)

	return r.opts.PollInterval, err
}

// readRecovery reports whether Elasticsearch still recovers an index that the
// restore replaces. A failure says that the recovery cannot be read.
func (r *Reconciler) readRecovery(
	ctx context.Context,
	lres *v1.LogicalRestoreElasticsearch,
) (bool, *conditions.PreCheckFailure, error) {
	cluster, failure, err := restore.ResolveCluster(
		ctx,
		r.APIReader,
		types.NamespacedName{Namespace: lres.Namespace, Name: lres.Spec.TargetClusterRef.Name},
		lres.Status.TargetClusterUID,
	)
	if err != nil || failure != nil {
		return false, failure, err
	}

	storage, failure, err := restore.ResolveStorage(ctx, r.APIReader, cluster)
	if err != nil || failure != nil {
		return false, failure, err
	}

	// A target that moved to another Elasticsearch cannot answer for the recovery on the pinned one.
	backend, failure, err := restore.BackendOf(ctx, r.APIReader, storage)
	if err != nil || failure != nil {
		return false, failure, err
	}
	if failure := restore.MovedBackend(cluster, backend, lres.Status.Backend); failure != nil {
		return false, failure, nil
	}

	admin, failure, err := secondarystorageconfig.ElasticsearchAdmin(ctx, r.APIReader, storage)
	if err != nil || failure != nil {
		return false, failure, err
	}

	// A restore that ended before it recorded its snapshots can have asked
	// for an Optimize snapshot, so it reads the Optimize indices too.
	snapshots := lres.Status.RestoredSnapshots
	optimize := len(snapshots) == 0 || logicalbackup.HasOptimizeSnapshot(snapshots)

	state, err := admin.RestoreProgress(ctx, logicalbackup.CamundaIndexPatterns(optimize))
	if err != nil {
		return false, elasticsearchFailure("reading the recovery of the restored indices", err), nil
	}

	return state == esadmin.RestoreInProgress, nil, nil
}
