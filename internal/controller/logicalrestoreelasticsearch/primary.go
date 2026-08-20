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
	"strconv"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/restore"
)

// restorePrimaryStorage gives the brokers empty data volumes and runs the
// Camunda restore application on them, once per broker. It is the last phase,
// and every check that protects it already passed: the rules of the target,
// the compatibility of the backup, and the claim on the cluster. The
// secondary storage of the target already holds the backup, so the brokers
// and the web applications agree once this phase finishes.
//
// The phase itself is the shared driver of pkg/restore. This kind adds the
// backup it reads and the mapping of the driver's outcome onto its own phase.
func (r *Reconciler) restorePrimaryStorage(
	ctx context.Context,
	lres *v1.LogicalRestoreElasticsearch,
) (restore.Outcome, error) {
	resolved, failure, err := r.resolve(ctx, lres)
	if err != nil {
		return restore.Outcome{}, err
	}
	if failure != nil {
		return r.holdStarted(lres, failure), nil
	}

	outcome, err := restore.Primary(
		ctx,
		r.Client,
		r.APIReader,
		r.Scheme,
		&lres.Status.RestoreProgress,
		restore.PrimaryInput{
			Owner:      lres,
			OwnerLabel: labels.LogicalRestoreElasticsearch(lres.Name),
			Target:     resolved.target,
			// The backup recorded the size its brokers needed. A backup that
			// recorded none leaves the claim template of the StatefulSet.
			Size:         resolved.target.ClaimSize(resolved.backup.ZeebeSize),
			FieldManager: restore.FieldManagerLogicalRestoreElasticsearch,
			Recorder:     r.EventRecorder,
			// The restore application reads the primary-storage backup that
			// the backup id names, out of the bucket of the target.
			Args:  []string{"--backupId=" + strconv.FormatInt(lres.Status.BackupID, 10)},
			Poll:  r.opts.PollInterval,
			Grace: r.opts.MidRunGrace,
		},
	)
	if err != nil {
		return restore.Outcome{}, err
	}

	switch {
	case outcome.Failure != nil:
		r.fail(lres, outcome.Failure.Reason, outcome.Failure.Message)

		return restore.Outcome{}, nil
	case outcome.Done:
		r.complete(lres)

		return restore.Outcome{}, nil
	}

	return outcome, nil
}
