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
// contract at it, remove what it replaced, and publish the outcome. The
// contract points at the new cluster before the old one goes, so a failure at
// any point up to that leaves the old server whole and the restore reads a
// refusal rather than an empty server.
//
// It runs before the reconcile holds a suspended server, because a suspended
// server has to answer the request it refuses. Everything it needs for that
// answer is the request itself.
func (r *DatabaseServerReconciler) reconcileRecovery(
	ctx context.Context,
	server *v1.DatabaseServer,
	resolved resolvedSpec,
) (bool, error) {
	contract, err := r.publishedContract(ctx, server, resolved.merged)
	if err != nil || contract == nil || contract.Spec.Recovery == nil {
		return false, err
	}

	request := *contract.Spec.Recovery
	if recoveryAnswered(server.Status.Recovery, request) {
		return false, nil
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

// publishedContract reads the contract that the server publishes, or nil when
// the server names none or has not published it yet. The read is cached: the
// contract is owned and watched, so every write to it comes back here.
func (r *DatabaseServerReconciler) publishedContract(
	ctx context.Context,
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
) (*v1.DatabaseServerConfig, error) {
	if merged.DatabaseServerConfig == "" {
		return nil, nil
	}

	key := types.NamespacedName{Namespace: server.Namespace, Name: merged.DatabaseServerConfig}

	var contract v1.DatabaseServerConfig
	if err := r.Get(ctx, key, &contract); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		}

		return nil, fmt.Errorf("reading DatabaseServerConfig %s: %w", key, err)
	}

	return &contract, nil
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

	target, err := time.Parse(time.RFC3339, request.TargetTime)
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

	if phase, failing := cnpgcluster.Failing(&recovered); failing {
		// The half-built cluster goes with the refusal. It holds no state
		// anybody can use, its volumes cost money, and a retry of the same
		// request builds the same name again.
		if err := r.Delete(ctx, &recovered); err != nil && !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("deleting the failed recovery cluster %s: %w", key, err)
		}

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
		server.Status.Cluster = recovered.Name

		return true, nil
	}

	// The components of the previous reconcile republish the contract from
	// status.cluster. Reading the endpoint back is what proves that they did:
	// the contract blocks while the superuser Secret of the new cluster is
	// missing, and nothing else here notices.
	if contract.Spec.Host != components.ReadWriteHost(server) {
		return true, nil
	}

	if err := r.removeSupersededCluster(ctx, server); err != nil {
		return false, err
	}

	return false, r.answerRecovery(ctx, server, contract, request, v1.RecoveryResultCompleted, "")
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
	if err := r.List(ctx, &clusters, scope...); err != nil {
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
	if err := r.List(ctx, &schedules, scope...); err != nil {
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
// it in status.
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
	key := client.ObjectKeyFromObject(contract)
	now := metav1.Now()

	patch, err := components.RecoveryOutcomePatch(
		key, components.RecoveryOutcomeFor(request, result, message, now),
	)
	if err != nil {
		return err
	}

	//nolint:staticcheck // the operator applies through the deprecated client.Apply patch
	if err := r.Patch(
		ctx, patch, client.Apply, components.RecoveryFieldManager, client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("publishing the recovery outcome on DatabaseServerConfig %s: %w", key, err)
	}

	cluster := ""
	if recoveryMatches(server.Status.Recovery, request) {
		cluster = server.Status.Recovery.Cluster
	}
	server.Status.Recovery = &v1.DatabaseServerRecoveryStatus{
		RequestedBy: request.RequestedBy,
		TargetTime:  request.TargetTime,
		Cluster:     cluster,
		CompletedAt: &now,
	}

	r.recordRecoveryOutcome(server, request, result, message)

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
