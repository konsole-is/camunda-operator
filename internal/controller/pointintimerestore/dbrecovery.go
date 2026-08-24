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
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/restore"
)

// recoveryFieldManager owns spec.recovery of the DatabaseServerConfig that the
// restore asks. The producer of the contract never carries that field, so the
// request and the contract never meet on one field, and the managed fields of
// the contract name the restore that asked.
const recoveryFieldManager client.FieldOwner = "camunda-operator/pointintimerestore-recovery"

// enterDatabaseRecovery asks the database server to roll itself back to
// spec.timestamp and waits for the answer.
//
// The restore reaches this phase only when the contract declares
// pitr.recovery: operator. It writes spec.recovery on the contract, holds
// until pitr.lastRecovery answers that request, and reads the result. Nothing
// bounds the hold: the restore has erased nothing, the recovery of a large
// database takes as long as it takes, and a producer that never answers is
// something the owner of the server fixes.
//
// A Completed answer is not the end of the wait. Pointing the contract at the
// recovered server is a change of its spec, which clears the identity it
// published until it reaches the server again. The restore refreshes its
// pinned chain from that new identity, so the phases that erase volumes are
// measured against the server the database now lives on.
func (r *Reconciler) enterDatabaseRecovery(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
) (restore.Outcome, error) {
	resolved, failure, err := r.resolve(ctx, pitr, pinAcrossRecovery)
	if err != nil {
		return r.resolveFailed(pitr, err)
	}
	if failure != nil {
		return r.holdRecovering(pitr, failure), nil
	}
	// The brokers must stay down for the whole rollback. This one is bounded:
	// a cluster that runs again writes into the database the restore is
	// rolling back, and nobody but its owner can stop that.
	if failure := notSuspended(resolved.cluster); failure != nil {
		return r.holdStarted(pitr, failure), nil
	}

	contract := resolved.server
	if !contract.OperatorRecovers() {
		return r.holdRecovering(pitr, &conditions.PreCheckFailure{
			Reason: v1.ReasonPitrUnavailable,
			Message: fmt.Sprintf(
				"DatabaseServerConfig %s no longer declares pitr.recovery: operator, so nobody "+
					"answers the request of this restore. Set it back, or roll the server back by "+
					"hand and create a new restore",
				client.ObjectKeyFromObject(contract),
			),
		}), nil
	}

	request := recoveryRequest(pitr)
	if outcome := contract.Spec.PITR.LastRecovery; request.AnsweredBy(outcome) {
		return r.recoveryAnswered(pitr, resolved, outcome)
	}

	if err := r.askForRecovery(ctx, contract, request); err != nil {
		return restore.Outcome{}, err
	}
	r.progressing(pitr, fmt.Sprintf(
		"Waiting for DatabaseServerConfig %s to answer the recovery request from %s to %s",
		client.ObjectKeyFromObject(contract), client.ObjectKeyFromObject(pitr), request.TargetTime,
	))

	return restore.Outcome{Wait: r.opts.PollInterval}, nil
}

// recoveryRequest renders the request that this restore makes: the identity of
// the restore, its namespace and name, and spec.timestamp in RFC 3339 UTC.
//
// The zone is explicit, because PostgreSQL reads a timestamp without one as
// the local time of the server.
//
// The uid is what makes the request this restore's own. A restore that is
// deleted and created again under one name asks for the same point on behalf
// of the same name, and the answer to the first says nothing about the state
// the second asks for.
func recoveryRequest(pitr *v1.PointInTimeRestore) v1.RecoveryRequest {
	return v1.RecoveryRequest{
		RequestID:   string(pitr.UID),
		RequestedBy: pitr.Namespace + "/" + pitr.Name,
		TargetTime:  pitr.Spec.Timestamp.UTC().Format(time.RFC3339Nano),
	}
}

// recoveryAnswered maps the answer of the producer onto the outcome of this
// phase. Unavailable means the server never held the requested point, which
// is the same refusal that a retention period reports, so it fails the restore
// with PitrUnavailable. Failed means the rollback started and did not finish.
// Completed continues, once the contract reaches the server it now names.
func (r *Reconciler) recoveryAnswered(
	pitr *v1.PointInTimeRestore,
	resolved *chain,
	outcome *v1.RecoveryOutcome,
) (restore.Outcome, error) {
	contract := resolved.server

	switch outcome.Result {
	case v1.RecoveryResultUnavailable:
		r.fail(pitr, v1.ReasonPitrUnavailable, outcome.Message)

		return restore.Outcome{}, nil

	case v1.RecoveryResultFailed:
		r.fail(pitr, v1.ReasonFailed, outcome.Message)

		return restore.Outcome{}, nil
	}

	// Ready alone is the answer to the spec that was probed, which can still be
	// the spec from before the endpoint moved. The observed generation is what
	// says that the record describes the endpoint the contract names now.
	if contract.Status.ObservedGeneration != contract.Generation ||
		!meta.IsStatusConditionTrue(contract.Status.Conditions, v1.ConditionReady) ||
		contract.Status.SystemIdentifier == "" {
		r.progressing(pitr, fmt.Sprintf(
			"DatabaseServerConfig %s rolled its server back to %s. The restore waits for it to "+
				"reach the server it now names",
			client.ObjectKeyFromObject(contract), outcome.TargetTime,
		))

		return restore.Outcome{Wait: r.opts.PollInterval}, nil
	}

	// The pin is replaced, not compared. The rollback was asked for by this
	// restore, so the server behind the contract is meant to be another one,
	// and every later look is measured against this record instead.
	pitr.Status.Storage = pinnedChain(resolved.storage, resolved.dbConfig, contract)
	restore.Recovered(&pitr.Status.RestoreProgress)

	pitr.Status.Phase = v1.PointInTimeRestoreValidatingDatabaseState
	r.progressing(pitr, fmt.Sprintf(
		"The database server holds the state of %s. The restore reads the database next",
		outcome.TargetTime,
	))

	return restore.Outcome{Wait: restore.Shortly}, nil
}

// askForRecovery writes the request on the contract, unless the contract
// already carries exactly it. The apply states spec.recovery and nothing
// else, so it declares no field that the producer of the contract owns.
func (r *Reconciler) askForRecovery(
	ctx context.Context,
	contract *v1.DatabaseServerConfig,
	request v1.RecoveryRequest,
) error {
	if contract.Spec.Recovery != nil && *contract.Spec.Recovery == request {
		return nil
	}

	key := client.ObjectKeyFromObject(contract)

	fields, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&request)
	if err != nil {
		return fmt.Errorf("rendering the recovery request for DatabaseServerConfig %s: %w", key, err)
	}

	patch := &unstructured.Unstructured{}
	patch.SetGroupVersionKind(v1.GroupVersion.WithKind("DatabaseServerConfig"))
	patch.SetNamespace(key.Namespace)
	patch.SetName(key.Name)
	if err := unstructured.SetNestedMap(patch.Object, fields, "spec", "recovery"); err != nil {
		return fmt.Errorf("rendering the recovery request for DatabaseServerConfig %s: %w", key, err)
	}

	//nolint:staticcheck // the operator applies through the deprecated client.Apply patch
	if err := r.Patch(ctx, patch, client.Apply, recoveryFieldManager, client.ForceOwnership); err != nil {
		return fmt.Errorf(
			"asking DatabaseServerConfig %s to roll back to %s: %w", key, request.TargetTime, err,
		)
	}

	return nil
}

// holdRecovering holds the restore in RestoringDatabase and reports why. The
// restore has erased nothing, so the hold is unbounded and it recovers on its
// own once the cause is gone. The phase stays, because the request on the
// contract stands and the answer is still the thing the restore waits for.
func (r *Reconciler) holdRecovering(
	pitr *v1.PointInTimeRestore,
	failure *conditions.PreCheckFailure,
) restore.Outcome {
	conditions.Stage(pitr, conditions.Failed(pitr, failure))

	return restore.Outcome{Wait: r.opts.RetryInterval}
}
