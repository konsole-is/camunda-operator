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

package logicalrestorerdbms

import (
	"context"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/restore"
)

// restorePrimaryStorage gives the brokers empty data volumes and runs the
// Camunda restore application on them, once per broker. The logical database
// already holds the dump, and every check that protects this phase passed:
// the rules of the cluster, the compatibility of the target, and the claim on
// the cluster.
//
// The phase itself is the shared driver of pkg/restore. This kind adds the
// backup it resolves from and the mapping of the driver's outcome onto its
// own phase.
func (r *Reconciler) restorePrimaryStorage(
	ctx context.Context,
	lrr *v1.LogicalRestoreRDBMS,
) (restore.Outcome, error) {
	resolved, failure, err := r.resolveTarget(ctx, lrr)
	if err != nil {
		return restore.Outcome{}, err
	}
	if failure != nil {
		return r.holdStarted(lrr, failure), nil
	}

	outcome, err := restore.Primary(
		ctx,
		r.Client,
		r.APIReader,
		r.Scheme,
		&lrr.Status.RestoreProgress,
		restore.PrimaryInput{
			Owner:      lrr,
			OwnerLabel: labels.LogicalRestoreRDBMS(lrr.Name),
			Target:     resolved.target,
			// The backup recorded the effective restore size of the broker
			// volumes. A restored volume holds the data of the backup, which
			// the claim template of the target can be too small for.
			Size:         resolved.target.ClaimSize(resolved.backup.ZeebeSize),
			FieldManager: restore.FieldManagerLogicalRestoreRDBMS,
			Recorder:     r.EventRecorder,
			// The restore application takes no argument on this path. It reads
			// the exporter position from the restored database and picks the
			// primary-storage backups itself.
			Args:  nil,
			Poll:  r.opts.PollInterval,
			Grace: r.opts.MidRunGrace,
		},
	)
	if err != nil {
		return restore.Outcome{}, err
	}

	switch {
	case outcome.Failure != nil:
		r.fail(lrr, outcome.Failure.Reason, outcome.Failure.Message)

		return restore.Outcome{}, nil
	case outcome.Done:
		r.complete(lrr)

		return restore.Outcome{}, nil
	}

	return outcome, nil
}
