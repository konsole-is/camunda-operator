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

// Package logicalrestoreelasticsearch reconciles a
// LogicalRestoreElasticsearch. One resource restores one completed
// LogicalBackupElasticsearch into one suspended CamundaCluster, and runs
// once.
//
// The phase is the resume marker, and it is persisted before the side effect
// it names. Pending holds the restore until its references resolve, the
// target is suspended, and no other operation holds the cluster.
// ValidatingCompatibility refuses a target that cannot hold the backup.
// RestoringSecondaryStorage deletes the Camunda indices of the target and
// restores the snapshots of the backup into its Elasticsearch.
// RestoringPrimaryStorage gives the brokers empty data volumes and runs the
// Camunda restore application on them, once per broker.
//
// The controller only reads spec.suspend of the target. Whoever owns the
// cluster suspends it before the restore and unsuspends it after.
//
// The files follow the phases. admit.go resolves the references, holds the
// restore in Pending, and takes the claim on the cluster. compatibility.go
// compares the backup against the target. secondary.go rebuilds the
// Elasticsearch of the target. primary.go runs the shared primary-storage
// phase of pkg/restore. This file holds what every phase shares: Reconcile,
// the terminal transitions, and the wiring.
//
// The machinery that every restore kind shares lives in pkg/restore. This
// package holds the phase vocabulary of this kind, its own rules, and the
// mapping of a driver outcome onto its phase.
package logicalrestoreelasticsearch

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/clusterclaim"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/podstate"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/restore"
)

const (
	// clusterRefField indexes restores by the namespace and name of their
	// targetClusterRef, so an event on a cluster wakes the restores that wait
	// for it, for example on the flip of spec.suspend.
	clusterRefField = "logicalrestoreelasticsearch.spec.targetClusterRef"
	// backupRefField indexes restores by the namespace and name of their
	// backupRef, so a backup that reaches Completed wakes them. Index names
	// are global to the manager, so both carry the kind as a prefix.
	backupRefField = "logicalrestoreelasticsearch.spec.backupRef"

	// defaultPollInterval paces a running phase.
	defaultPollInterval = 5 * time.Second
	// defaultRetryInterval paces a hold that no watch resolves.
	defaultRetryInterval = 30 * time.Second
	// defaultMidRunGrace bounds how long a started restore waits on a
	// dependency that stopped resolving before the restore fails. A restore
	// that already deleted an index or a broker volume must reach a terminal
	// phase, so that whoever owns the cluster learns that it has to act.
	defaultMidRunGrace = 10 * time.Minute
)

// Options tunes a Reconciler. The zero value is the production configuration.
// Tests shrink the intervals.
type Options struct {
	// PollInterval paces a running phase. Zero means five seconds.
	PollInterval time.Duration
	// RetryInterval paces a hold that no watch resolves. Zero means thirty
	// seconds.
	RetryInterval time.Duration
	// MidRunGrace bounds how long a started restore waits on a dependency
	// that stopped resolving. Zero means ten minutes.
	MidRunGrace time.Duration
}

// withDefaults fills the zero fields of o with the production configuration.
func (o Options) withDefaults() Options {
	if o.PollInterval <= 0 {
		o.PollInterval = defaultPollInterval
	}
	if o.RetryInterval <= 0 {
		o.RetryInterval = defaultRetryInterval
	}
	if o.MidRunGrace <= 0 {
		o.MidRunGrace = defaultMidRunGrace
	}

	return o
}

// Reconciler drives a LogicalRestoreElasticsearch to a terminal phase.
type Reconciler struct {
	client.Client
	// APIReader reads without the cache. Every decision that deletes an index
	// or a broker volume reads live state. A stale suspend flag, a stale
	// claim, or a stale Job each let a restore act on a cluster that moved on.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the lifecycle events of the restore.
	// SetupWithManager sets it from the manager when it is nil.
	EventRecorder events.EventRecorder

	opts Options
}

// New returns a Reconciler with the given options, with every zero field
// filled from the production configuration.
func New(c client.Client, reader client.Reader, scheme *runtime.Scheme, options Options) *Reconciler {
	return &Reconciler{Client: c, APIReader: reader, Scheme: scheme, opts: options.withDefaults()}
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestoreelasticsearches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestoreelasticsearches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestoreelasticsearches/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters;secondarystorageconfigs;objectstorageconfigs;logicalbackupelasticsearches,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps;secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list;watch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// The cluster claim reads the resource of whichever kind holds the Lease,
// to decide whether that holder still needs the cluster. The list names every
// kind in clusterclaim.holderKinds, including the kind of this controller, so
// that it stays readable next to that map.
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackupelasticsearches;logicalbackuprdbmses;logicalrestoreelasticsearches;logicalrestorerdbmses;pointintimerestores,verbs=get
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile advances the restore by at most one phase. The phase is the
// resume marker: it is persisted before the side effect it names, so a
// restore that re-enters after a crash continues where it stopped.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	// The restore is its own state machine. A stale cached read of its status
	// re-enters a phase whose side effect already ran, and the side effects of
	// this controller delete data.
	var lres v1.LogicalRestoreElasticsearch
	if err := r.APIReader.Get(ctx, req.NamespacedName, &lres); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// The Jobs of a restore carry a controller reference to it, so the garbage
	// collector removes them with the restore. It writes nothing outside the
	// cluster, so it needs no finalizer.
	if !lres.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	rec := component.ReconcileContext{
		Client:        r.Client,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
		APIReader:     r.APIReader,
		Owner:         &lres,
	}
	defer func() {
		if flushErr := component.FlushStatus(ctx, rec, nil); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	if lres.Terminal() {
		// A conflict on the terminal flush can restore a stale Ready from the
		// server. Staging it again on every look heals that. This branch also
		// gives the claim back the first time, on the look that follows the
		// terminal transition. A release that failed heals here too.
		restore.StageTerminal(&lres, &lres.Status.RestoreProgress)

		// The Jobs go before the claim. A completed Job keeps its pod, the pod
		// keeps the broker volume it mounts, and the claim is what tells the
		// next operation that the cluster is free. Foreground propagation
		// finishes after this look, so the claim waits for the look that finds
		// the Jobs gone.
		collected, err := restore.CollectJobs(
			ctx, r.Client, r.APIReader, &lres, &lres.Status.RestoreProgress,
		)
		if err != nil || !collected.Done {
			return ctrl.Result{RequeueAfter: collected.Wait}, err
		}

		return ctrl.Result{}, r.releaseClaim(ctx, &lres)
	}

	var outcome restore.Outcome
	switch lres.Status.Phase {
	case "", v1.LogicalRestorePending:
		outcome, err = r.admit(ctx, &lres)
	case v1.LogicalRestoreValidatingCompatibility:
		outcome, err = r.validate(ctx, &lres)
	case v1.LogicalRestoreRestoringSecondaryStorage:
		outcome, err = r.restoreSecondaryStorage(ctx, &lres)
	case v1.LogicalRestoreRestoringPrimaryStorage:
		outcome, err = r.restorePrimaryStorage(ctx, &lres)
	default:
		return ctrl.Result{}, fmt.Errorf("unknown phase %q", lres.Status.Phase)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	// A restore that reached its terminal phase in this look keeps the claim
	// until the flush of this reconcile makes that phase durable. The look
	// that the flush wakes releases it through the branch above. A Lease that
	// outlives the phase by one look costs nothing, because the claim reports
	// a terminal holder inactive. A release before that point lets a second
	// operation start against a cluster whose restore the API still reports
	// as running.
	return ctrl.Result{RequeueAfter: outcome.Wait}, nil
}

// complete ends the restore. The secondary storage of the target holds the
// backup, the brokers hold the partitions of the backup, and whoever owns the
// cluster can unsuspend it.
func (r *Reconciler) complete(lres *v1.LogicalRestoreElasticsearch) {
	lres.Status.Phase = v1.LogicalRestoreCompleted
	restore.Complete(&lres.Status.RestoreProgress, metav1.Now())
	restore.StageTerminal(lres, &lres.Status.RestoreProgress)
	r.EventRecorder.Eventf(
		lres,
		nil,
		corev1.EventTypeNormal,
		restore.EventReasonCompleted,
		restore.EventActionRestore,
		"The restore of backup %d into CamundaCluster %s/%s finished",
		lres.Status.BackupID,
		lres.Namespace,
		lres.Spec.TargetClusterRef.Name,
	)
}

// fail ends the restore with reason and message. Status keeps the reason, so
// a terminal look stages the same one again after a write conflict.
func (r *Reconciler) fail(lres *v1.LogicalRestoreElasticsearch, reason, message string) {
	lres.Status.Phase = v1.LogicalRestoreFailed
	restore.Fail(&lres.Status.RestoreProgress, reason, message, metav1.Now())
	restore.StageTerminal(lres, &lres.Status.RestoreProgress)
	r.EventRecorder.Eventf(
		lres,
		nil,
		corev1.EventTypeWarning,
		restore.EventReasonFailed,
		restore.EventActionRestore,
		"The restore failed: %s",
		lres.Status.FailureMessage,
	)
}

// progressing stages that a phase of the restore runs.
func (r *Reconciler) progressing(lres *v1.LogicalRestoreElasticsearch, message string) {
	conditions.Stage(lres, conditions.Ready(
		metav1.ConditionFalse, v1.ReasonProgressing, message, lres.Generation,
	))
}

// waiting stages a pre-check failure that holds the restore in Pending. The
// restore has touched nothing, so it goes back to waiting and recovers on its
// own once the cause is gone. Nothing bounds this hold.
func (r *Reconciler) waiting(
	lres *v1.LogicalRestoreElasticsearch,
	failure *conditions.PreCheckFailure,
) restore.Outcome {
	lres.Status.Phase = v1.LogicalRestorePending
	restore.Recovered(&lres.Status.RestoreProgress)
	conditions.Stage(lres, conditions.Failed(lres, failure))

	return restore.Outcome{Wait: r.opts.RetryInterval}
}

// holdStarted holds a started restore on a dependency that stopped resolving,
// and ends it once the mid-run grace is over. The phase of this kind is the
// controller's to write, so the terminal transition happens here.
func (r *Reconciler) holdStarted(
	lres *v1.LogicalRestoreElasticsearch,
	failure *conditions.PreCheckFailure,
) restore.Outcome {
	outcome := restore.HoldRunning(
		lres,
		&lres.Status.RestoreProgress,
		failure,
		metav1.Now(),
		r.opts.MidRunGrace,
		r.opts.PollInterval,
	)
	if outcome.Failure != nil {
		r.fail(lres, outcome.Failure.Reason, outcome.Failure.Message)

		return restore.Outcome{}
	}

	return outcome
}

// claimant is the identity under which the restore holds the claim on its
// cluster.
func claimant(lres *v1.LogicalRestoreElasticsearch) clusterclaim.Claimant {
	return clusterclaim.Claimant{Kind: lres.GetKind(), Name: lres.Name, UID: lres.UID}
}

// releaseClaim gives the claim on the cluster back. It is a no-op when the
// restore does not hold it.
func (r *Reconciler) releaseClaim(ctx context.Context, lres *v1.LogicalRestoreElasticsearch) error {
	return restore.Give(
		ctx, r.Client, r.APIReader, lres.Namespace, lres.Spec.TargetClusterRef.Name, claimant(lres),
	)
}

// SetupWithManager registers the controller, the two field indexes, and the
// watches: the restores, the Jobs they own, the pods of those Jobs, the
// clusters they name, and the backups they read. A suspend flip and a backup
// that reaches Completed both wake a waiting restore without a timer.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder("logicalrestoreelasticsearch")
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1.LogicalRestoreElasticsearch{},
		clusterRefField,
		func(obj client.Object) []string {
			lres := obj.(*v1.LogicalRestoreElasticsearch)

			return []string{refindex.NamespacedKey(lres.Namespace, lres.Spec.TargetClusterRef.Name)}
		},
	); err != nil {
		return fmt.Errorf("indexing LogicalRestoreElasticsearch by targetClusterRef: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1.LogicalRestoreElasticsearch{},
		backupRefField,
		func(obj client.Object) []string {
			lres := obj.(*v1.LogicalRestoreElasticsearch)

			return []string{refindex.NamespacedKey(lres.Namespace, lres.Spec.BackupRef.Name)}
		},
	); err != nil {
		return fmt.Errorf("indexing LogicalRestoreElasticsearch by backupRef: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.LogicalRestoreElasticsearch{}).
		Owns(&batchv1.Job{}).
		// A Job reports nothing about a pod of its own that cannot start,
		// so the pods are watched next to the Jobs.
		Watches(
			&corev1.Pod{},
			podstate.EnqueueJobOwner(
				mgr.GetClient(), v1.GroupVersion.WithKind("LogicalRestoreElasticsearch").GroupKind(),
			),
		).
		Watches(
			&v1.CamundaCluster{},
			refindex.Enqueue(
				mgr.GetClient(),
				&v1.LogicalRestoreElasticsearchList{},
				clusterRefField,
				refindex.ObjectNamespacedName,
			),
		).
		Watches(
			&v1.LogicalBackupElasticsearch{},
			refindex.Enqueue(
				mgr.GetClient(),
				&v1.LogicalRestoreElasticsearchList{},
				backupRefField,
				refindex.ObjectNamespacedName,
			),
		).
		Named("logicalrestoreelasticsearch").
		Complete(r)
}
