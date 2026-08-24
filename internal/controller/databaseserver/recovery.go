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

package databaseserver

import (
	"context"
	"fmt"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/cnpgcluster"
)

// The events of a recovery. A recovery replaces the server behind the
// contract, so every step of it is worth a line in kubectl describe.
const (
	eventReasonRecoveryStarted  = "RecoveryStarted"
	eventReasonRecoveryFinished = "RecoveryFinished"
	eventReasonRecoveryRefused  = "RecoveryRefused"
	eventActionRecover          = "Recover"
)

// recoveryRefusal is the answer to a recovery request that the server does not
// perform.
type recoveryRefusal struct {
	result  v1.RecoveryResult
	message string
}

// reconcileRecovery answers the recovery request that the published contract
// carries, and reports whether a recovery is still running. Each step is
// keyed on what exists, so a reconcile that re-enters in the middle continues
// where the last one stopped.
//
// The steps are: record the request, build the cluster that recovers to the
// requested point, wait for CloudNativePG to declare it healthy, point the
// contract at it, publish the outcome, and remove what it replaced. The
// contract points at the new cluster before the old one goes, so a failure at
// any point up to that leaves the old server whole and the restore reads a
// refusal rather than an empty server.
//
// status.recovery is the record of the answer, and the contract is where the
// answer is published. Every step reads the record, so an answer whose status
// write was lost is given again rather than acted on twice, and an answer that
// a replaced contract no longer carries is published again.
//
// It runs before the reconcile holds a suspended server, because a suspended
// server has to answer the request it refuses. Everything it needs for that
// answer is the request itself.
func (r *DatabaseServerReconciler) reconcileRecovery(
	ctx context.Context,
	server *v1.DatabaseServer,
	resolved resolvedSpec,
) (bool, error) {
	contract, err := r.recoveryContract(ctx, server, resolved.merged)
	if err != nil || contract == nil {
		return false, err
	}

	request, ok := pendingRequest(server, contract)
	if !ok {
		return false, nil
	}

	// The answer lives in two places, and either one alone is enough to know
	// that the request is done. The record is what the server keeps for
	// itself; the contract is where the answer is published, and it is written
	// first. A look that finds one and not the other completes the pair. A
	// look that reads the answer from neither builds the cluster of an
	// answered request a second time.
	published := publishedOutcome(contract)
	switch {
	case recoveryAnswered(server.Status.Recovery, request):
		if err := r.republishRecoveryOutcome(ctx, contract, server.Status.Recovery); err != nil {
			return false, err
		}

		// The cluster that a refused recovery abandoned goes here, on the look
		// that reads the answer back. The answer is durable before anything is
		// removed, so a lost status write can never leave a request that reads
		// unanswered with no cluster to explain it.
		return false, r.removeAbandonedRecoveryCluster(ctx, server)

	case request.AnsweredBy(published):
		recordPublishedOutcome(server, contract, published)

		return false, r.removeAbandonedRecoveryCluster(ctx, server)
	}

	source, refusal := recoverySource(server, resolved.merged, request)
	if refusal != nil {
		return false, r.answerRecovery(ctx, server, contract, request, refusal.result, refusal.message)
	}

	// The record goes in before anything is built, and it carries the name of
	// the cluster to build. A name derived again later moves with the archive
	// history, so a recovery that resumes after the history grew builds a
	// second cluster.
	if !recoveryMatches(server.Status.Recovery, request) {
		server.Status.Recovery = &v1.DatabaseServerRecoveryStatus{
			RequestID:   request.RequestID,
			Contract:    contract.Name,
			RequestedBy: request.RequestedBy,
			TargetTime:  request.TargetTime,
			Cluster:     components.RecoveryClusterName(server),
		}
		r.EventRecorder.Eventf(
			server,
			nil,
			corev1.EventTypeNormal,
			eventReasonRecoveryStarted,
			eventActionRecover,
			"%s asked for a rollback to %s. The server recovers into %q from the archive of %q",
			request.RequestedBy,
			request.TargetTime,
			server.Status.Recovery.Cluster,
			source.ServerName,
		)

		return true, nil
	}

	return r.advanceRecovery(ctx, server, contract, resolved, request, source)
}

// recoveryContract reads the contract that the recovery is answered on: the
// one the record names while a recovery runs, and the one the spec names
// otherwise. It is nil when the server names none or has not published it yet.
//
// The record wins while a recovery is unanswered. A spec that is repointed at
// another contract mid-recovery abandons the cluster that is building and
// leaves whoever asked with no answer. preCheck keeps the merged spec on this
// contract in that case, and Ready reports why.
//
// The read is cached: the contract is owned and watched, so every write to it
// comes back here.
func (r *DatabaseServerReconciler) recoveryContract(
	ctx context.Context,
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
) (*v1.DatabaseServerConfig, error) {
	name := merged.DatabaseServerConfig
	if running := server.Status.Recovery; running != nil && running.CompletedAt == nil {
		name = running.Contract
	}
	if name == "" {
		return nil, nil
	}

	key := types.NamespacedName{Namespace: server.Namespace, Name: name}

	var contract v1.DatabaseServerConfig
	if err := r.Get(ctx, key, &contract); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("reading DatabaseServerConfig %s: %w", key, err)
	}

	return &contract, nil
}

// pendingRequest returns the request that the server works on, and whether
// there is one.
//
// A recorded recovery without an answer is the one the server runs, whatever
// the contract says now. Its cluster is already building, and a request that
// arrived after it asks for another point: answering that one from this
// cluster answers a question nobody asked. The later request starts once
// the recorded one is answered, and it starts under a name of its own.
func pendingRequest(
	server *v1.DatabaseServer,
	contract *v1.DatabaseServerConfig,
) (v1.RecoveryRequest, bool) {
	if running := server.Status.Recovery; running != nil && running.CompletedAt == nil {
		return recordedRequest(running), true
	}

	if contract.Spec.Recovery == nil {
		return v1.RecoveryRequest{}, false
	}

	return *contract.Spec.Recovery, true
}

// publishedOutcome returns the answer that the contract carries, or nil when it
// carries none.
func publishedOutcome(contract *v1.DatabaseServerConfig) *v1.RecoveryOutcome {
	if contract.Spec.PITR == nil {
		return nil
	}

	return contract.Spec.PITR.LastRecovery
}

// recordPublishedOutcome writes the published answer into the record. The name
// of the cluster the recovery built is kept when the record already names one:
// it is the only place that holds it, and the cleanup reads it.
func recordPublishedOutcome(
	server *v1.DatabaseServer,
	contract *v1.DatabaseServerConfig,
	published *v1.RecoveryOutcome,
) {
	cluster := ""
	if recorded := server.Status.Recovery; recorded != nil && recorded.RequestID == published.RequestID {
		cluster = recorded.Cluster
	}

	completedAt := published.CompletedAt
	server.Status.Recovery = &v1.DatabaseServerRecoveryStatus{
		RequestID:   published.RequestID,
		Contract:    contract.Name,
		RequestedBy: published.RequestedBy,
		TargetTime:  published.TargetTime,
		Cluster:     cluster,
		Result:      published.Result,
		Message:     published.Message,
		CompletedAt: &completedAt,
	}
}

// recordedRequest returns the request that the record holds.
func recordedRequest(recorded *v1.DatabaseServerRecoveryStatus) v1.RecoveryRequest {
	return v1.RecoveryRequest{
		RequestID:   recorded.RequestID,
		RequestedBy: recorded.RequestedBy,
		TargetTime:  recorded.TargetTime,
	}
}

// republishRecoveryOutcome publishes the recorded answer on the contract when
// the contract does not already carry it. A contract that somebody deleted and
// created again under its name carries no answer at all, and whoever asked
// reads the contract, not the status of this server.
func (r *DatabaseServerReconciler) republishRecoveryOutcome(
	ctx context.Context,
	contract *v1.DatabaseServerConfig,
	answered *v1.DatabaseServerRecoveryStatus,
) error {
	if recordedRequest(answered).AnsweredBy(publishedOutcome(contract)) {
		return nil
	}

	return r.publishRecoveryOutcome(ctx, contract, v1.RecoveryOutcome{
		RequestID:   answered.RequestID,
		RequestedBy: answered.RequestedBy,
		TargetTime:  answered.TargetTime,
		CompletedAt: *answered.CompletedAt,
		Result:      answered.Result,
		Message:     answered.Message,
	})
}

// removeAbandonedRecoveryCluster removes the cluster that an answered recovery
// built and did not become. A recovery that the server refused leaves one, and
// a recovery that finished leaves the cluster it replaced, which
// removeSupersededCluster takes instead.
func (r *DatabaseServerReconciler) removeAbandonedRecoveryCluster(
	ctx context.Context,
	server *v1.DatabaseServer,
) error {
	answered := server.Status.Recovery
	if answered.Cluster == "" || answered.Cluster == components.ClusterName(server) {
		return nil
	}

	key := types.NamespacedName{Namespace: server.Namespace, Name: answered.Cluster}

	var abandoned cnpgv1.Cluster
	if err := r.Get(ctx, key, &abandoned); err != nil {
		return client.IgnoreNotFound(err)
	}
	if !ownedByServer(server, &abandoned) {
		return nil
	}
	if err := r.Delete(ctx, &abandoned); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting the abandoned recovery cluster %s: %w", key, err)
	}

	return nil
}

// ownedByServer reports whether server is the controller of obj and labelled
// it. Both are what the operator writes on everything it builds, so an object
// that carries neither is somebody else's, whatever its name is.
func ownedByServer(server *v1.DatabaseServer, obj client.Object) bool {
	return metav1.IsControlledBy(obj, server) &&
		obj.GetLabels()[labels.DatabaseServerKey] == labels.OwnerName(server.Name)
}

// recoveryAnswered reports whether the server already published the answer to
// request.
func recoveryAnswered(recorded *v1.DatabaseServerRecoveryStatus, request v1.RecoveryRequest) bool {
	return recoveryMatches(recorded, request) && recorded.CompletedAt != nil
}

// recoveryMatches reports whether the recorded recovery is the one that
// request asks for.
func recoveryMatches(recorded *v1.DatabaseServerRecoveryStatus, request v1.RecoveryRequest) bool {
	return recorded != nil &&
		recorded.RequestID == request.RequestID &&
		recorded.RequestedBy == request.RequestedBy &&
		recorded.TargetTime == request.TargetTime
}

// recoverySource returns the archive that a recovery to the requested point
// starts from, or why the server refuses the request.
//
// It is answered again on every look, so a server that is suspended while a
// recovery runs refuses the request it was working on. That is the honest
// answer: hibernation takes the instances away, and a recovery needs one.
func recoverySource(
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	request v1.RecoveryRequest,
) (v1.ArchiveRecord, *recoveryRefusal) {
	switch {
	case merged.Suspend:
		return v1.ArchiveRecord{}, &recoveryRefusal{
			result: v1.RecoveryResultFailed,
			message: "The server is suspended, so it cannot roll back to a point in time. " +
				"Unsuspend it, then create a new restore",
		}

	case merged.Archive == nil:
		return v1.ArchiveRecord{}, &recoveryRefusal{
			result:  v1.RecoveryResultUnavailable,
			message: "The server writes no archive, so it holds no point to roll back to",
		}
	}

	target, err := time.Parse(time.RFC3339Nano, request.TargetTime)
	if err != nil {
		return v1.ArchiveRecord{}, &recoveryRefusal{
			result:  v1.RecoveryResultFailed,
			message: fmt.Sprintf("targetTime %q is not a timestamp with a zone", request.TargetTime),
		}
	}

	var history []v1.ArchiveRecord
	if server.Status.Archive != nil {
		history = server.Status.Archive.History
	}

	source, err := components.SelectArchive(history, target)
	if err != nil {
		return v1.ArchiveRecord{}, &recoveryRefusal{
			result:  v1.RecoveryResultUnavailable,
			message: err.Error(),
		}
	}

	return source, nil
}

// advanceRecovery carries the recorded recovery one step further and reports
// whether it is still running.
func (r *DatabaseServerReconciler) advanceRecovery(
	ctx context.Context,
	server *v1.DatabaseServer,
	contract *v1.DatabaseServerConfig,
	resolved resolvedSpec,
	request v1.RecoveryRequest,
	source v1.ArchiveRecord,
) (bool, error) {
	key := types.NamespacedName{Namespace: server.Namespace, Name: server.Status.Recovery.Cluster}

	var recovered cnpgv1.Cluster
	if err := r.Get(ctx, key, &recovered); err != nil {
		if apierrors.IsNotFound(err) {
			return true, r.applyRecoveryCluster(ctx, server, resolved, source, request.TargetTime)
		}

		return false, fmt.Errorf("reading the recovery cluster %s: %w", key, err)
	}

	// The name is derived, so it can already be taken. A cluster that this
	// server does not own holds data of somebody else, and recovering into it
	// would destroy that data.
	if !ownedByServer(server, &recovered) {
		return false, r.answerRecovery(
			ctx, server, contract, request, v1.RecoveryResultFailed,
			fmt.Sprintf(
				"A CloudNativePG cluster %s already exists and this DatabaseServer does not own "+
					"it. Remove that cluster, or rename the server, then ask again", key,
			),
		)
	}

	if phase, failing := cnpgcluster.Failing(&recovered); failing {
		// The answer goes out before the cluster it abandons. The cluster is
		// removed on the look that reads the answer back, so a lost status
		// write cannot leave a request that reads unanswered with no cluster.
		return false, r.answerRecovery(
			ctx, server, contract, request, v1.RecoveryResultFailed,
			fmt.Sprintf(
				"CloudNativePG could not recover %s to %s. It reports %q",
				key, request.TargetTime, phase,
			),
		)
	}

	if !cnpgcluster.Converged(&recovered) {
		return true, nil
	}

	if server.Status.Cluster != recovered.Name {
		// The archive of the cluster that goes ends here. Its write-ahead log
		// stops when the cluster is removed, and the archive of the recovered
		// cluster reaches no point before its first base backup, so the window
		// between the two lies in no interval and no restore can ask for it.
		closeArchiveRecords(server, metav1.Now())
		server.Status.Cluster = recovered.Name

		return true, nil
	}

	// The components of the previous reconcile republish the contract from
	// status.cluster, and the contract controller then reaches the server it
	// names now. Both are read back here: the endpoint says the contract was
	// republished, and the observed generation with the identity says the
	// probe answered for that endpoint rather than for the one before it.
	if contract.Spec.Host != components.ReadWriteHost(server) ||
		contract.Status.ObservedGeneration != contract.Generation ||
		contract.Status.SystemIdentifier == "" {
		return true, nil
	}

	if err := r.answerRecovery(
		ctx, server, contract, request, v1.RecoveryResultCompleted, "",
	); err != nil {
		return false, err
	}

	return false, r.removeSupersededCluster(ctx, server)
}

// applyRecoveryCluster applies the cluster that recovers the server to the
// requested point. It carries the owner reference of the server, so the
// recovery of a server that is deleted goes with it.
func (r *DatabaseServerReconciler) applyRecoveryCluster(
	ctx context.Context,
	server *v1.DatabaseServer,
	resolved resolvedSpec,
	source v1.ArchiveRecord,
	target string,
) error {
	recovered, err := components.RecoveryCluster(
		server, resolved.merged, resolved.archive, resolved.platform, source, target,
	)
	if err != nil {
		return err
	}

	if err := ctrl.SetControllerReference(server, recovered, r.Scheme); err != nil {
		return fmt.Errorf("setting the owner of the recovery cluster %q: %w", recovered.Name, err)
	}

	//nolint:staticcheck // the operator applies through the deprecated client.Apply patch
	if err := r.Patch(
		ctx, recovered, client.Apply, components.RecoveryFieldManager, client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("applying the recovery cluster %q: %w", recovered.Name, err)
	}

	return nil
}

// removeSupersededCluster deletes every CloudNativePG cluster of the server,
// and every base backup schedule of one, that is not the cluster the contract
// now points at.
//
// The schedule has to go with the cluster it names. It follows the cluster
// name, so the archive component writes a schedule for the recovered cluster
// on its own, and a schedule left behind keeps firing base backups against a
// cluster that no longer exists.
func (r *DatabaseServerReconciler) removeSupersededCluster(
	ctx context.Context,
	server *v1.DatabaseServer,
) error {
	current := components.ClusterName(server)
	scope := []client.ListOption{
		client.InNamespace(server.Namespace),
		client.MatchingLabels{labels.DatabaseServerKey: labels.OwnerName(server.Name)},
	}

	var clusters cnpgv1.ClusterList
	if err := r.APIReader.List(ctx, &clusters, scope...); err != nil {
		return fmt.Errorf("listing the clusters of %q: %w", server.Name, err)
	}
	for i := range clusters.Items {
		if clusters.Items[i].Name == current {
			continue
		}
		if err := r.Delete(ctx, &clusters.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf("deleting the superseded cluster %q: %w", clusters.Items[i].Name, err)
		}
	}

	var schedules cnpgv1.ScheduledBackupList
	if err := r.APIReader.List(ctx, &schedules, scope...); err != nil {
		return fmt.Errorf("listing the base backup schedules of %q: %w", server.Name, err)
	}
	for i := range schedules.Items {
		if schedules.Items[i].Name == components.BaseBackupName(server) {
			continue
		}
		if err := r.Delete(ctx, &schedules.Items[i]); err != nil && !apierrors.IsNotFound(err) {
			return fmt.Errorf(
				"deleting the superseded base backup schedule %q: %w", schedules.Items[i].Name, err,
			)
		}
	}

	return nil
}

// answerRecovery publishes the outcome of request on the contract and records
// the whole of it in status.
//
// The outcome goes on the contract under a field manager of its own, not
// through the contract component. A suspended server publishes no contract at
// all, and it still has to answer the request it refuses.
func (r *DatabaseServerReconciler) answerRecovery(
	ctx context.Context,
	server *v1.DatabaseServer,
	contract *v1.DatabaseServerConfig,
	request v1.RecoveryRequest,
	result v1.RecoveryResult,
	message string,
) error {
	now := metav1.Now()

	if err := r.publishRecoveryOutcome(
		ctx, contract, components.RecoveryOutcomeFor(request, result, message, now),
	); err != nil {
		return err
	}

	cluster := ""
	if recoveryMatches(server.Status.Recovery, request) {
		cluster = server.Status.Recovery.Cluster
	}
	server.Status.Recovery = &v1.DatabaseServerRecoveryStatus{
		RequestID:   request.RequestID,
		Contract:    contract.Name,
		RequestedBy: request.RequestedBy,
		TargetTime:  request.TargetTime,
		Cluster:     cluster,
		Result:      result,
		Message:     message,
		CompletedAt: &now,
	}

	r.recordRecoveryOutcome(server, request, result, message)

	return nil
}

// publishRecoveryOutcome applies outcome on the contract. The apply states
// spec.pitr.lastRecovery and nothing else, so it declares no field that the
// contract component or the consumer owns.
func (r *DatabaseServerReconciler) publishRecoveryOutcome(
	ctx context.Context,
	contract *v1.DatabaseServerConfig,
	outcome v1.RecoveryOutcome,
) error {
	key := client.ObjectKeyFromObject(contract)

	patch, err := components.RecoveryOutcomePatch(key, outcome)
	if err != nil {
		return err
	}

	//nolint:staticcheck // the operator applies through the deprecated client.Apply patch
	if err := r.Patch(
		ctx, patch, client.Apply, components.RecoveryFieldManager, client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("publishing the recovery outcome on DatabaseServerConfig %s: %w", key, err)
	}

	return nil
}

// recordRecoveryOutcome publishes the outcome of request as an event on the
// server.
func (r *DatabaseServerReconciler) recordRecoveryOutcome(
	server *v1.DatabaseServer,
	request v1.RecoveryRequest,
	result v1.RecoveryResult,
	message string,
) {
	if result == v1.RecoveryResultCompleted {
		r.EventRecorder.Eventf(
			server,
			nil,
			corev1.EventTypeNormal,
			eventReasonRecoveryFinished,
			eventActionRecover,
			"The server holds the state of %s and runs from %q",
			request.TargetTime,
			components.ClusterName(server),
		)

		return
	}

	r.EventRecorder.Eventf(
		server,
		nil,
		corev1.EventTypeWarning,
		eventReasonRecoveryRefused,
		eventActionRecover,
		"The rollback to %s that %s asked for ended as %s: %s",
		request.TargetTime,
		request.RequestedBy,
		result,
		message,
	)
}
