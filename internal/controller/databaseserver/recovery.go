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
// RecoveryCompleted reports the Completed result. RecoveryFailed reports the
// Failed and the Unavailable results: neither gives the requester the point
// it asked for.
const (
	eventReasonRecoveryStarted         = "RecoveryStarted"
	eventReasonRecoveryCompleted       = "RecoveryCompleted"
	eventReasonRecoveryFailed          = "RecoveryFailed"
	eventReasonRecoveryClusterNotOwned = "RecoveryClusterNotOwned"
	eventActionRecover                 = "Recover"
)

// day is how long the retention period of an archive counts one day.
const day = 24 * time.Hour

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
	// itself. The contract is where the answer is published, and it is written
	// first. A look that finds one and not the other completes the pair. A
	// look that reads the answer from neither builds the cluster of an
	// answered request a second time.
	published := publishedOutcome(contract)
	switch {
	case recoveryAnswered(server.Status.Recovery, request):
		if err := r.republishRecoveryOutcome(ctx, contract, server.Status.Recovery); err != nil {
			return false, err
		}

		return false, r.cleanUpAnsweredRecovery(ctx, server)

	case request.AnsweredBy(published):
		if err := r.recordPublishedOutcome(ctx, server, contract, published); err != nil {
			return false, err
		}

		return false, r.cleanUpAnsweredRecovery(ctx, server)
	}

	// Past the cutover the contract already points at the cluster the recovery
	// built, and the cluster it replaces is on its way out. Nothing puts the
	// server back on it, so the recovery runs to Completed whatever the spec
	// says by then.
	if cutOver(server) {
		return r.completeRecovery(ctx, server, contract, request)
	}

	source, refusal := recoverySource(server, resolved, request)
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
			// recoverySource refuses a request the merged spec has no archive
			// for, so the block is here on every path that records one.
			Archive: recordedArchive(source, *resolved.merged.Archive, resolved.archive),
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

// recordedArchive returns the archive record of a recovery that is about to
// start: what it reads, the archive settings of the server at that moment, and
// the identity of the bucket. Every edit of spec.archive is held against those
// settings while the rollback is unanswered, so the archive keeps being
// rendered as it was.
func recordedArchive(
	source v1.ArchiveRecord,
	spec v1.DatabaseServerArchiveSpec,
	archive *components.ArchiveStorage,
) *v1.RecoveryArchiveRef {
	return &v1.RecoveryArchiveRef{
		ServerName:          source.ServerName,
		ObjectStorageRef:    source.ObjectStorageRef,
		Location:            source.Location,
		RetentionPeriodDays: spec.RetentionPeriodDays,
		BaseBackupSchedule:  spec.BaseBackupSchedule,
		Identity:            archive.Identity(),
	}
}

// cutOver reports whether the contract of the server already points at the
// cluster that the recorded recovery built.
func cutOver(server *v1.DatabaseServer) bool {
	recovery := server.Status.Recovery
	if recovery == nil || recovery.CompletedAt != nil || recovery.Cluster == "" {
		return false
	}

	return recovery.Cluster == server.Status.Cluster
}

// recoveryContract reads the contract that the recovery is answered on: the
// one the record names while a recovery runs, and the one the spec names
// otherwise. It is nil when the server names none, has not published it yet,
// or the object of that name belongs to another server.
//
// The record wins while a recovery is unanswered. A spec that is repointed at
// another contract mid-recovery abandons the cluster that is building and
// leaves whoever asked with no answer. preCheck keeps the merged spec on this
// contract in that case, and Ready reports why. The record always names a
// contract this server published, so the ownership test never hides one.
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

	// Two servers can name one contract. The request on it was written against
	// the archive and the endpoint of the server that owns it, so the other
	// one must not read it: it recovers its own archive and publishes the
	// answer on somebody else's contract.
	if !ownedByServer(server, &contract) {
		return nil, nil
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

	// A new request is taken only from a contract that declares the operator
	// answers it. The declaration is what the consumer reads before it asks,
	// so a request on a contract that declares external was written against a
	// server that never offered to roll itself back.
	if contract.Spec.Recovery == nil || !contract.OperatorRecovers() {
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
func (r *DatabaseServerReconciler) recordPublishedOutcome(
	ctx context.Context,
	server *v1.DatabaseServer,
	contract *v1.DatabaseServerConfig,
	published *v1.RecoveryOutcome,
) error {
	request := v1.RecoveryRequest{
		RequestID:   published.RequestID,
		RequestedBy: published.RequestedBy,
		TargetTime:  published.TargetTime,
	}

	var carry *v1.DatabaseServerRecoveryStatus
	if recoveryMatches(server.Status.Recovery, request) {
		carry = server.Status.Recovery
	}

	answered := answeredRecovery(
		carry,
		request,
		contract.Name,
		published.Result,
		published.Message,
		published.CompletedAt,
	)

	if published.Result == v1.RecoveryResultCompleted {
		current, err := r.recoveredClusterOf(ctx, server, contract)
		if err != nil {
			return err
		}
		if current != "" {
			server.Status.Cluster = current
			answered.Cluster = current
		}
	}

	server.Status.Recovery = answered

	return nil
}

// answeredRecovery builds the record of an answered recovery. The request and
// the outcome carry every part but three. Those three live only in the record
// of the running recovery. They are the cluster it built, the cluster it came
// from, and the archive it read. carry is that record when it belongs to this
// request, and nil when it belongs to another one.
//
// Both writers of status.recovery build the record here, so neither of them
// can drop a part that the other keeps.
func answeredRecovery(
	carry *v1.DatabaseServerRecoveryStatus,
	request v1.RecoveryRequest,
	contract string,
	result v1.RecoveryResult,
	message string,
	completedAt metav1.Time,
) *v1.DatabaseServerRecoveryStatus {
	answered := &v1.DatabaseServerRecoveryStatus{
		RequestID:   request.RequestID,
		Contract:    contract,
		RequestedBy: request.RequestedBy,
		TargetTime:  request.TargetTime,
		Result:      result,
		Message:     message,
		CompletedAt: &completedAt,
	}

	if carry != nil {
		answered.Cluster = carry.Cluster
		answered.PreviousCluster = carry.PreviousCluster
		answered.Archive = carry.Archive
	}

	return answered
}

// recoveredClusterOf returns the cluster of this server that the endpoint of
// the contract names, or the empty string when the endpoint names none of
// them.
//
// The contract is the record of which cluster the server runs from, and a
// recovery that finished and lost its status write left status.cluster naming
// a cluster that is gone. Reading the endpoint back is what puts that right.
//
// A name parsed out of an endpoint is not proof on its own. Two servers can
// name one contract, and the one that loses the race reads the endpoint of the
// other. Adopting that name makes this server delete its own live cluster as
// the superseded one, so the name counts only when this server owns a cluster
// of it.
func (r *DatabaseServerReconciler) recoveredClusterOf(
	ctx context.Context,
	server *v1.DatabaseServer,
	contract *v1.DatabaseServerConfig,
) (string, error) {
	name := components.ClusterFromReadWriteHost(server, contract.Spec.Host)
	if name == "" || name == components.ClusterName(server) {
		return "", nil
	}

	key := types.NamespacedName{Namespace: server.Namespace, Name: name}

	var candidate cnpgv1.Cluster
	if err := r.APIReader.Get(ctx, key, &candidate); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}

		return "", fmt.Errorf(
			"reading the cluster %s that DatabaseServerConfig %s names: %w",
			key, client.ObjectKeyFromObject(contract), err,
		)
	}
	if !ownedByServer(server, &candidate) {
		r.EventRecorder.Eventf(
			server,
			nil,
			corev1.EventTypeWarning,
			eventReasonRecoveryClusterNotOwned,
			eventActionRecover,
			"DatabaseServerConfig %s names the cluster %q, which this server does not own. The "+
				"server keeps running from %q",
			client.ObjectKeyFromObject(contract), name, components.ClusterName(server),
		)

		return "", nil
	}

	return name, nil
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

// cleanUpAnsweredRecovery removes what the answered recovery left behind: the
// cluster it built and did not become, and the cluster and the base backup
// schedule it replaced.
//
// It runs on every look that reads the answer back, not once beside the
// answer. The answer is written before anything is removed, so a delete that
// fails has to be tried again, and the look that reads the answer is the one
// that tries it.
func (r *DatabaseServerReconciler) cleanUpAnsweredRecovery(
	ctx context.Context,
	server *v1.DatabaseServer,
) error {
	if err := r.removeAbandonedRecoveryCluster(ctx, server); err != nil {
		return err
	}

	return r.removeSupersededCluster(ctx, server)
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

	// Live: a cached read can name a cluster that is already gone, and the
	// name is one that another recovery of this server takes again.
	var abandoned cnpgv1.Cluster
	if err := r.APIReader.Get(ctx, key, &abandoned); err != nil {
		return client.IgnoreNotFound(err)
	}
	if !ownedByServer(server, &abandoned) {
		return nil
	}

	if err := r.deleteOwned(ctx, &abandoned); err != nil {
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
// starts from, or why the server refuses the request. The archive it returns
// names the directory, the location it lives in, and the bucket contract of
// that location.
//
// It is answered again on every look, so a server that is suspended while a
// recovery runs refuses the request it was working on. That is the honest
// answer: hibernation takes the instances away, and a recovery needs one.
func recoverySource(
	server *v1.DatabaseServer,
	resolved resolvedSpec,
	request v1.RecoveryRequest,
) (v1.ArchiveRecord, *recoveryRefusal) {
	merged := resolved.merged

	// The archive of a recovery that is already running is the one it
	// recorded, whatever the history says now. The rules below still answer:
	// a server that is suspended in the middle refuses the request it was
	// working on.
	recorded := server.Status.Recovery
	pinned := recoveryMatches(recorded, request) && recorded.Archive != nil

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

	// The retention of a recovery that is already running is the one it
	// recorded, for the same reason the archive is.
	retention := merged.Archive.RetentionPeriodDays
	if pinned && recorded.Archive.RetentionPeriodDays > 0 {
		retention = recorded.Archive.RetentionPeriodDays
	}

	var reachableFrom *metav1.Time
	if server.Status.Archive != nil {
		reachableFrom = server.Status.Archive.ReachableFrom
	}

	if refusal := outOfReach(time.Now(), target, retention, reachableFrom); refusal != nil {
		return v1.ArchiveRecord{}, refusal
	}

	if pinned {
		return v1.ArchiveRecord{
			ServerName:       recorded.Archive.ServerName,
			ObjectStorageRef: recorded.Archive.ObjectStorageRef,
			Location:         recorded.Archive.Location,
		}, nil
	}

	var history []v1.ArchiveRecord
	if server.Status.Archive != nil {
		history = server.Status.Archive.History
	}

	source, err := components.SelectArchive(
		history, target, resolved.archiveLocation, merged.Archive.ObjectStorageRef,
	)
	if err != nil {
		return v1.ArchiveRecord{}, &recoveryRefusal{
			result:  v1.RecoveryResultUnavailable,
			message: err.Error(),
		}
	}

	return source, nil
}

// outOfReach reports why the archive holds no write-ahead log of the requested
// point, or nil when it can reach it. An interval of the history says which
// archive wrote a point. It does not say that the objects of that point are
// still there: the record that is open has no end, and the bucket drops
// everything older than the retention period.
//
// reachableFrom is status.archive.reachableFrom, the floor that the prunes of
// a shorter retention period left. It is nil for a server that archived before
// that field existed, and the retention period alone bounds that one.
func outOfReach(
	now, target time.Time,
	retentionDays int32,
	reachableFrom *metav1.Time,
) *recoveryRefusal {
	// The clock is the reason these rules are not schema rules on the request.
	if target.After(now) {
		return &recoveryRefusal{
			result: v1.RecoveryResultUnavailable,
			message: fmt.Sprintf(
				"targetTime %s lies in the future. The server holds no state of a point that "+
					"did not happen yet", target.UTC().Format(time.RFC3339),
			),
		}
	}

	if oldest := now.Add(-time.Duration(retentionDays) * day); target.Before(oldest) {
		return &recoveryRefusal{
			result: v1.RecoveryResultUnavailable,
			message: fmt.Sprintf(
				"targetTime %s is older than the retention period of the archive, which is %d "+
					"days. The bucket keeps nothing of that point any more",
				target.UTC().Format(time.RFC3339), retentionDays,
			),
		}
	}

	if reachableFrom != nil && target.Before(reachableFrom.Time) {
		return &recoveryRefusal{
			result: v1.RecoveryResultUnavailable,
			message: fmt.Sprintf(
				"targetTime %s is inside the retention period of %d days, but a shorter one "+
					"pruned the archive to %s before that. The archive reaches further back only "+
					"as it writes past that point",
				target.UTC().Format(time.RFC3339), retentionDays,
				reachableFrom.UTC().Format(time.RFC3339),
			),
		}
	}

	return nil
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

	// Live: a cached read can miss a cluster the operator created moments ago,
	// and a second create under one name is a second recovery of one request.
	//
	// A cluster that is going counts as gone. The name comes back once the
	// number of archives comes back, so a request that follows a refused one
	// is recorded under the name the refusal is still deleting. Grading that
	// cluster answers the new request from the state of the dead one, and it
	// answers it at once.
	var recovered cnpgv1.Cluster
	if err := r.APIReader.Get(ctx, key, &recovered); err != nil {
		if apierrors.IsNotFound(err) {
			return true, r.applyRecoveryCluster(ctx, server, resolved, source, request.TargetTime)
		}

		return false, fmt.Errorf("reading the recovery cluster %s: %w", key, err)
	}
	if !recovered.DeletionTimestamp.IsZero() {
		return true, nil
	}

	// The name is derived, so it can already be taken. A cluster that this
	// server does not own holds data of somebody else, and recovering into it
	// would destroy that data.
	if !ownedByServer(server, &recovered) {
		// The refusal owns no cluster, so the record must not name one. The
		// cleanup reads that name, and this cluster is somebody else's.
		server.Status.Recovery.Cluster = ""

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

	server.Status.Recovery.PreviousCluster = server.Status.Cluster
	server.Status.Cluster = recovered.Name

	return true, nil
}

// completeRecovery finishes a recovery whose cluster the contract already
// points at: it waits for the contract to reach that server, publishes the
// outcome, and removes what the recovery replaced.
func (r *DatabaseServerReconciler) completeRecovery(
	ctx context.Context,
	server *v1.DatabaseServer,
	contract *v1.DatabaseServerConfig,
	request v1.RecoveryRequest,
) (bool, error) {
	key := types.NamespacedName{Namespace: server.Namespace, Name: server.Status.Recovery.Cluster}

	var recovered cnpgv1.Cluster
	if err := r.APIReader.Get(ctx, key, &recovered); err != nil {
		if !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("reading the recovery cluster %s: %w", key, err)
		}

		// Somebody removed it. The server has no cluster to run from under
		// this name, and nothing here can build one: a recovery reads an
		// archive, and the point it read is gone.
		return false, r.abandonRecovery(
			ctx, server, contract, request, fmt.Sprintf("CloudNativePG cluster %s was removed", key),
		)
	}

	// A cluster that carries a finalizer keeps reporting the phase it last
	// reached while it goes, and that phase can be healthy. Completing on it
	// publishes a cluster that is on its way out and deletes the one the
	// server came from, which leaves the server with no cluster at all.
	if !recovered.DeletionTimestamp.IsZero() {
		return false, r.abandonRecovery(
			ctx, server, contract, request, fmt.Sprintf("CloudNativePG cluster %s is being removed", key),
		)
	}

	// The ownership was tested when the cluster was built, and the name is
	// derived, so it is tested again on what is there now. A cluster that
	// somebody removed and created again under this name holds a database of
	// theirs. Completing on it moves the contract onto that database and
	// deletes the cluster that holds the data of this server.
	if !ownedByServer(server, &recovered) {
		return false, r.abandonRecovery(
			ctx, server, contract, request,
			fmt.Sprintf("A CloudNativePG cluster %s exists and this DatabaseServer does not own it", key),
		)
	}

	// A cluster that turns unrecoverable once the contract points at it takes
	// the server with it.
	if phase, failing := cnpgcluster.Failing(&recovered); failing {
		return false, r.abandonRecovery(
			ctx, server, contract, request,
			fmt.Sprintf("CloudNativePG reports %q for %s", phase, key),
		)
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

	// The archive of the cluster that goes ends where the contract left it.
	// Closing it at the moment the pointer moved strands every point after
	// that if the move never becomes visible, and the archive of the
	// recovered cluster reaches no point before its first base backup, so the
	// window between the two lies in no interval.
	//
	// That one record only. The cluster this recovery built takes its first
	// base backup whenever CloudNativePG gets to it, so its own record can
	// already be open here, and closing it leaves the server with an archive
	// that no restore can reach.
	closeArchiveRecord(server, server.Status.Recovery.PreviousCluster, metav1.Now())

	if err := r.answerRecovery(
		ctx, server, contract, request, v1.RecoveryResultCompleted, "",
	); err != nil {
		return false, err
	}

	return false, r.removeSupersededCluster(ctx, server)
}

// abandonRecovery refuses a request whose cluster the contract already moved
// to and which cannot serve the server any more.
//
// The contract goes back to the cluster it came from. That cluster still holds
// the data, and nothing has removed it: the removal runs after the answer, and
// the answer is this refusal.
//
// A record that names no cluster to go back to is refused instead. Leaving
// status.cluster on the cluster it abandons points the sweep of the next look
// at every other cluster of this server, and the one holding the data is among
// them. The record is written with the cutover in one status write, so no
// reconcile reaches this without it.
func (r *DatabaseServerReconciler) abandonRecovery(
	ctx context.Context,
	server *v1.DatabaseServer,
	contract *v1.DatabaseServerConfig,
	request v1.RecoveryRequest,
	cause string,
) error {
	previous := server.Status.Recovery.PreviousCluster
	if previous == "" {
		return fmt.Errorf(
			"%s, and DatabaseServer %s records no cluster to go back to. Its status.recovery names "+
				"%q with no previousCluster beside it",
			cause, client.ObjectKeyFromObject(server), server.Status.Recovery.Cluster,
		)
	}

	server.Status.Cluster = previous
	server.Status.Recovery.Cluster = ""

	return r.answerRecovery(
		ctx, server, contract, request, v1.RecoveryResultFailed,
		fmt.Sprintf(
			"%s. The server had already moved to it, and it runs from %q again", cause, previous,
		),
	)
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
//
// Both loops delete what this server owns, not what carries its label. A label
// is a value anybody can write, and the objects here hold whole databases.
//
// The lists are cached. Both kinds are watched, and every delete names the
// object it read by uid, so a cached entry of an object that has gone deletes
// nothing.
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
	if err := r.List(ctx, &clusters, scope...); err != nil {
		return fmt.Errorf("listing the clusters of %q: %w", server.Name, err)
	}
	for i := range clusters.Items {
		superseded := &clusters.Items[i]
		if superseded.Name == current || !ownedByServer(server, superseded) {
			continue
		}
		if err := r.deleteOwned(ctx, superseded); err != nil {
			return fmt.Errorf("deleting the superseded cluster %q: %w", superseded.Name, err)
		}
	}

	var schedules cnpgv1.ScheduledBackupList
	if err := r.List(ctx, &schedules, scope...); err != nil {
		return fmt.Errorf("listing the base backup schedules of %q: %w", server.Name, err)
	}
	for i := range schedules.Items {
		superseded := &schedules.Items[i]
		if superseded.Name == components.BaseBackupName(server) || !ownedByServer(server, superseded) {
			continue
		}
		if err := r.deleteOwned(ctx, superseded); err != nil {
			return fmt.Errorf(
				"deleting the superseded base backup schedule %q: %w", superseded.Name, err,
			)
		}
	}

	return nil
}

// deleteOwned deletes the object that the caller read, and no other object of
// its name. A name that this server derives is a name it takes again, so the
// delete carries the identity of what was read.
func (r *DatabaseServerReconciler) deleteOwned(ctx context.Context, obj client.Object) error {
	uid := obj.GetUID()
	if err := r.Delete(ctx, obj, client.Preconditions{UID: &uid}); err != nil &&
		!apierrors.IsNotFound(err) && !apierrors.IsConflict(err) {
		return err
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

	var carry *v1.DatabaseServerRecoveryStatus
	if recoveryMatches(server.Status.Recovery, request) {
		carry = server.Status.Recovery
	}
	server.Status.Recovery = answeredRecovery(
		carry, request, contract.Name, result, message, now,
	)

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
			eventReasonRecoveryCompleted,
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
		eventReasonRecoveryFailed,
		eventActionRecover,
		"The rollback to %s that %s asked for ended as %s: %s",
		request.TargetTime,
		request.RequestedBy,
		result,
		message,
	)
}
