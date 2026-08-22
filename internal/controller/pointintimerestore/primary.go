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

package pointintimerestore

import (
	"context"
	"time"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/restore"
)

// restorePrimaryStorage gives the brokers empty data volumes and runs the
// Camunda restore application on them, once per broker. It is the destructive
// phase, and every check that protects it already passed: the rules of the
// cluster and of the server, the state of the database, and the claim on the
// cluster.
//
// The phase itself is the shared driver of pkg/restore. This kind adds the
// storage chain it resolves from, the point it restores to, and the mapping
// of the driver's outcome onto its own phase.
func (r *Reconciler) restorePrimaryStorage(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
) (restore.Outcome, error) {
	resolved, failure, err := r.resolve(ctx, pitr)
	if err != nil {
		return r.resolveFailed(pitr, err)
	}
	if failure != nil {
		return r.holdStarted(pitr, failure), nil
	}
	// Suspension is a standing condition of this phase, and admission proved
	// it once. A cluster that is unsuspended mid-run starts its brokers over
	// the volumes that the restore is erasing.
	if failure := notSuspended(resolved.cluster); failure != nil {
		return r.holdStarted(pitr, failure), nil
	}

	outcome, err := restore.Primary(
		ctx,
		r.Client,
		r.APIReader,
		r.Scheme,
		&pitr.Status.RestoreProgress,
		restore.PrimaryInput{
			Owner:      pitr,
			OwnerLabel: labels.PointInTimeRestore(pitr.Name),
			Target:     resolved.target,
			// A point-in-time restore reads no backup resource, so no recorded
			// size exists. The claim template of the StatefulSet is the size
			// the cluster runs with.
			Size:         resolved.target.ClaimSize(nil),
			FieldManager: restore.FieldManagerPointInTimeRestore,
			Recorder:     r.EventRecorder,
			// The restore application takes the point as an argument. It finds
			// the newest checkpoint at or before it, and it refuses a point
			// that lies before the state of the restored database.
			Args:  []string{"--to=" + pitr.Spec.Timestamp.UTC().Format(time.RFC3339)},
			Poll:  r.opts.PollInterval,
			Grace: r.opts.MidRunGrace,
			// The operator compares timestamps and the restore application
			// compares log positions, so a restore that passes every rule of
			// this kind can still be refused. Only the application knows, and
			// only its log says so.
			JobFailure: r.jobRefusal,
		},
	)
	if err != nil {
		return restore.Outcome{}, err
	}

	switch {
	case outcome.Failure != nil:
		r.fail(pitr, outcome.Failure.Reason, outcome.Failure.Message)

		return restore.Outcome{}, nil
	case outcome.Done:
		r.complete(pitr)

		return restore.Outcome{}, nil
	}

	return outcome, nil
}
