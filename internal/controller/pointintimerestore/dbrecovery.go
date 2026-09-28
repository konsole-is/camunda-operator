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
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
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
// published until it reaches the server again. The restore then refreshes its
// pinned chain, so the phases that erase volumes are measured against the
// endpoint the database now lives behind. The identity itself comes back
// unchanged: a physical recovery restores the pg_control of the base backup.
func (r *Reconciler) enterDatabaseRecovery(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
) (restore.Outcome, error) {
	if outcome, done, err := r.targetGone(ctx, pitr); done || err != nil {
		return outcome, err
	}

	resolved, failure, err := r.resolve(ctx, pitr, pinAcrossRecovery)
	switch {
	case errors.Is(err, errClusterReplaced):
		// The cluster was replaced after targetGone read it.
		return r.goneDuringRollback(ctx, pitr, fmt.Sprintf(
			"CamundaCluster %s/%s was replaced", pitr.Namespace, pitr.Spec.ClusterRef.Name,
		))
	case errors.Is(err, errChainChanged):
		outcome, held, holdErr := r.holdForRollback(ctx, pitr, v1.ReasonInvalidReference, fmt.Sprintf(
			"The storage chain of CamundaCluster %s/%s changed", pitr.Namespace, pitr.Spec.ClusterRef.Name,
		))
		if holdErr != nil || held {
			return outcome, holdErr
		}
	}
	if err != nil {
		return r.resolveFailed(pitr, err)
	}
	if failure != nil {
		contract, err := r.pinnedContract(ctx, pitr)
		if err != nil {
			return restore.Outcome{}, err
		}
		if contract == nil {
			return r.holdRecovering(pitr, failure), nil
		}
		if _, err := r.followBackend(ctx, pitr, contract); err != nil {
			return restore.Outcome{}, err
		}
		if contract.Spec.PITR != nil && recoveryRequest(pitr).AnsweredBy(contract.Spec.PITR.LastRecovery) {
			return r.holdStarted(pitr, failure), nil
		}

		return r.holdRecovering(pitr, failure), nil
	}
	// A cluster claims the endpoint that the contract names, Ready or not.
	if _, err := r.followBackend(ctx, pitr, resolved.server); err != nil {
		return restore.Outcome{}, err
	}
	// The brokers must stay down for the whole rollback.
	if failure := notSuspended(resolved.cluster); failure != nil {
		outcome, held, err := r.holdForRollback(ctx, pitr, failure.Reason, fmt.Sprintf(
			"CamundaCluster %s/%s started running again", pitr.Namespace, pitr.Spec.ClusterRef.Name,
		))
		if err != nil || held {
			return outcome, err
		}

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
		return r.recoveryAnswered(ctx, pitr, resolved, outcome)
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

// targetGone reports done, with the outcome of the look, when the pinned
// cluster is deleted, being deleted, or replaced.
func (r *Reconciler) targetGone(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
) (outcome restore.Outcome, done bool, err error) {
	key := types.NamespacedName{Namespace: pitr.Namespace, Name: pitr.Spec.ClusterRef.Name}
	var cluster v1.CamundaCluster
	err = r.APIReader.Get(ctx, key, &cluster)
	switch {
	case err == nil && cluster.UID == pitr.Status.TargetClusterUID && cluster.DeletionTimestamp.IsZero():
		return restore.Outcome{}, false, nil
	case err != nil && !apierrors.IsNotFound(err):
		return restore.Outcome{}, false, fmt.Errorf("reading CamundaCluster %s: %w", key, err)
	}
	gone := fmt.Sprintf("CamundaCluster %s was deleted", key)
	if err == nil && cluster.UID != pitr.Status.TargetClusterUID {
		gone = fmt.Sprintf("CamundaCluster %s was replaced", key)
	}
	outcome, err = r.goneDuringRollback(ctx, pitr, gone)

	return outcome, true, err
}

// goneDuringRollback holds the restore while the rollback that it asked for
// still runs, and fails it once the rollback no longer runs. gone says what
// happened to the cluster.
func (r *Reconciler) goneDuringRollback(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
	gone string,
) (restore.Outcome, error) {
	outcome, held, err := r.holdForRollback(ctx, pitr, v1.ReasonInvalidReference, gone)
	if err != nil || held {
		return outcome, err
	}

	r.fail(pitr, v1.ReasonFailed, fmt.Sprintf(
		"%s during the rollback of its database, so no cluster takes the restored state. Create a "+
			"new restore for the cluster that uses the database now",
		gone,
	))

	return restore.Outcome{}, nil
}

// holdForRollback holds the restore while the rollback that it asked for still
// runs. held is false when no rollback of this restore runs.
func (r *Reconciler) holdForRollback(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
	reason, what string,
) (outcome restore.Outcome, held bool, err error) {
	contract, err := r.runningRollback(ctx, pitr)
	if err != nil || contract == nil {
		return restore.Outcome{}, false, err
	}
	if _, err := r.followBackend(ctx, pitr, contract); err != nil {
		return restore.Outcome{}, false, err
	}

	return r.holdRecovering(pitr, &conditions.PreCheckFailure{
		Reason: reason,
		Message: fmt.Sprintf(
			"%s while its database server rolls back. The restore ends when DatabaseServerConfig "+
				"%s answers the recovery request. Until then, no other cluster starts on the database",
			what, client.ObjectKeyFromObject(contract),
		),
	}), true, nil
}

// runningRollback returns the pinned contract while it carries the request of
// this restore and has not answered it, or nil when no rollback of this
// restore runs.
func (r *Reconciler) runningRollback(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
) (*v1.DatabaseServerConfig, error) {
	contract, err := r.pinnedContract(ctx, pitr)
	if err != nil || contract == nil {
		return nil, err
	}

	request := recoveryRequest(pitr)
	asked := contract.Spec.Recovery != nil && *contract.Spec.Recovery == request
	if !contract.OperatorRecovers() || !asked || request.AnsweredBy(contract.Spec.PITR.LastRecovery) {
		return nil, nil
	}

	return contract, nil
}

// pinnedContract returns the contract that the restore pinned, or nil when it
// pinned none or the contract is gone or replaced.
func (r *Reconciler) pinnedContract(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
) (*v1.DatabaseServerConfig, error) {
	pinned := pitr.Status.Storage
	if pinned == nil {
		return nil, nil
	}

	var contract v1.DatabaseServerConfig
	key := types.NamespacedName{Namespace: pitr.Namespace, Name: pinned.DatabaseServerConfig}
	if err := r.APIReader.Get(ctx, key, &contract); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("reading DatabaseServerConfig %s: %w", key, err)
	}
	if contract.UID != pinned.DatabaseServerConfigUID {
		return nil, nil
	}

	return &contract, nil
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
		// RFC3339Nano keeps the fraction that the restore asked for on the
		// wire. A time truncated to the second renders the same as RFC 3339.
		TargetTime: pitr.Spec.Timestamp.UTC().Format(time.RFC3339Nano),
	}
}

// recoveryAnswered maps the answer of the producer onto the outcome of this
// phase. Unavailable means the server never held the requested point, which
// is the same refusal that a retention period reports, so it fails the restore
// with PitrUnavailable. Failed means the rollback started and did not finish.
// Completed continues, once the contract reaches the server it now names.
func (r *Reconciler) recoveryAnswered(
	ctx context.Context,
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

	failure, err := r.followBackend(ctx, pitr, contract)
	if err != nil {
		return restore.Outcome{}, err
	}
	if failure != nil {
		return r.holdRecovering(pitr, failure), nil
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

// followBackend moves the writer registration of the restore to the backend
// of the pinned database at the endpoint that contract names.
func (r *Reconciler) followBackend(
	ctx context.Context,
	pitr *v1.PointInTimeRestore,
	contract *v1.DatabaseServerConfig,
) (*conditions.PreCheckFailure, error) {
	pinned := pitr.Status.Storage
	if pinned == nil {
		return nil, nil
	}
	storage := &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Namespace: pitr.Namespace, Name: pinned.SecondaryStorageConfig},
		Spec:       v1.SecondaryStorageConfigSpec{Type: v1.SecondaryStorageTypeRDBMS},
	}
	dbConfig := &v1.DatabaseConfig{Spec: v1.DatabaseConfigSpec{DatabaseName: pinned.DatabaseName}}
	backend, failure := restore.DatabaseBackend(storage, dbConfig, contract)
	if failure != nil || backend == pitr.Status.Backend {
		return failure, nil
	}

	err := restore.RegisterWriter(
		ctx, r.Client, r.APIReader, r.ClaimNamespace, backend, pitr, pitr.Status.TargetClusterUID,
	)
	if err != nil {
		return nil, err
	}
	// The renewer renews status.backend only, so the new key goes there
	// before the old one is released.
	old := pitr.Status.Backend
	pitr.Status.Backend = backend
	if old == "" {
		return nil, nil
	}

	return nil, restore.ReleaseWriter(ctx, r.Client, r.ClaimNamespace, old, pitr, pitr.Status.TargetClusterUID)
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
