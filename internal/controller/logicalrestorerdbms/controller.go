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

// Package logicalrestorerdbms reconciles a LogicalRestoreRDBMS. One resource
// restores one completed LogicalBackupRDBMS into one suspended
// CamundaCluster, and runs once.
//
// The phase is the resume marker, and it is persisted before the side effect
// it names. Pending holds the restore until its references resolve, the
// target is suspended, and no other operation holds the cluster.
// ValidatingCompatibility refuses a target that cannot hold the backup.
// RestoringSecondaryStorage writes the dump back into the logical database of
// the target with pg_restore. RestoringPrimaryStorage gives the brokers empty
// data volumes and runs the Camunda restore application on them, once per
// broker.
//
// Admission prepares the target: it claims the cluster, suspends it, waits
// for the brokers to stop, and sets spec.version to the Camunda version of the
// backup. The controller withdraws the suspension when the restore completes,
// and only when it applied that suspension itself. Every phase after
// admission reads spec.suspend again, because a target that is unsuspended
// mid-run starts its workloads over storage that the restore is rewriting.
//
// The files follow the phases. admit.go resolves the references, holds the
// restore in Pending, and takes the claim on the cluster. compatibility.go
// compares the backup against the target. secondary.go rebuilds the logical
// database. primary.go runs the shared primary-storage phase of pkg/restore.
// This file holds what every phase shares: Reconcile, the terminal
// transitions, and the wiring.
//
// The machinery that every restore kind shares lives in pkg/restore. This
// package holds the phase vocabulary of this kind, its own rules, and the
// mapping of a driver outcome onto its phase.
package logicalrestorerdbms

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
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/clusterclaim"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/restore"
)

const (
	// clusterRefField indexes restores by the namespace and name of their
	// targetClusterRef, so an event on a cluster wakes the restores that wait
	// for it, for example on the flip of spec.suspend.
	clusterRefField = "logicalrestorerdbms.spec.targetClusterRef"
	// backupRefField indexes restores by the namespace and name of their
	// backupRef, so a backup that reaches Completed wakes them.
	backupRefField = "logicalrestorerdbms.spec.backupRef"

	// defaultPollInterval paces a running phase.
	defaultPollInterval = 5 * time.Second
	// defaultRetryInterval paces a hold that no watch resolves.
	defaultRetryInterval = 30 * time.Second
	// defaultMidRunGrace bounds how long a started restore waits on a
	// dependency that stopped resolving before the restore fails. A restore
	// that already rewrote the logical database or deleted a broker volume
	// must reach a terminal phase, so that whoever owns the cluster learns
	// that it has to act.
	defaultMidRunGrace = 10 * time.Minute
)

// Options tunes a Reconciler. Only CLIImage is required. Every other field
// has a production default, and a test sets what it needs to observe.
type Options struct {
	// CLIImage is the camunda-operator-cli image that downloads the dump of
	// the backup. The manager passes --camunda-operator-cli-image.
	CLIImage string
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
// It rejects an empty CLIImage, because the restore cannot guess an image and
// would fail only once it reached the secondary-storage phase.
func (o Options) withDefaults() (Options, error) {
	if o.CLIImage == "" {
		return o, errors.New("the camunda-operator-cli image is required")
	}
	if o.PollInterval <= 0 {
		o.PollInterval = defaultPollInterval
	}
	if o.RetryInterval <= 0 {
		o.RetryInterval = defaultRetryInterval
	}
	if o.MidRunGrace <= 0 {
		o.MidRunGrace = defaultMidRunGrace
	}

	return o, nil
}

// Reconciler drives a LogicalRestoreRDBMS to a terminal phase.
type Reconciler struct {
	client.Client
	// APIReader reads without the cache. Every decision that rewrites the
	// logical database or deletes a broker volume reads live state. A stale
	// suspend flag, a stale claim, or a stale Job each let a restore act on a
	// cluster that moved on.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the lifecycle events of the restore.
	// SetupWithManager sets it from the manager when it is nil.
	EventRecorder events.EventRecorder

	opts Options
}

// New returns a Reconciler with the given options. SetupWithManager applies
// their defaults and rejects an incomplete set.
func New(c client.Client, reader client.Reader, scheme *runtime.Scheme, options Options) *Reconciler {
	return &Reconciler{Client: c, APIReader: reader, Scheme: scheme, opts: options}
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestorerdbmses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestorerdbmses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestorerdbmses/finalizers,verbs=update
// The restore prepares its own cluster: it suspends the cluster, sets the
// Camunda version of the backup, and withdraws the suspension when it
// completes. Each write is a server-side apply of one field under a field
// manager of its own, so patch is the only verb it needs on a CamundaCluster.
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusterpresets;secondarystorageconfigs;databaseconfigs;databaseserverconfigs;objectstorageconfigs;logicalbackuprdbmses,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
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
	// this controller erase data.
	var lrr v1.LogicalRestoreRDBMS
	if err := r.APIReader.Get(ctx, req.NamespacedName, &lrr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// The Jobs of a restore carry a controller reference to it, so the garbage
	// collector removes them with the restore. It writes nothing outside the
	// cluster, so it needs no finalizer.
	if !lrr.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	rec := component.ReconcileContext{
		Client:        r.Client,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
		APIReader:     r.APIReader,
		Owner:         &lrr,
	}
	defer func() {
		if flushErr := component.FlushStatus(ctx, rec, nil); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	if lrr.Terminal() {
		// A conflict on the terminal flush can restore a stale Ready from the
		// server. Staging it again on every look heals that. This branch also
		// gives the claim back the first time, on the look that follows the
		// terminal transition. A release that failed heals here too.
		restore.StageTerminal(&lrr, &lrr.Status.RestoreProgress)

		// The Jobs go before the claim. A completed Job keeps its pod, the pod
		// keeps the broker volume it mounts, and the claim is what tells the
		// next operation that the cluster is free.
		if err := restore.CollectJobs(
			ctx, r.Client, r.APIReader, &lrr, &lrr.Status.RestoreProgress,
		); err != nil {
			return ctrl.Result{}, err
		}

		// The Jobs also go before the unsuspend, and that order is the
		// correctness of it. The pods of the collected Jobs are what hold the
		// broker volumes, and unsuspending starts the brokers again. A broker
		// that the scheduler places on another node cannot attach a
		// ReadWriteOnce volume that a completed pod still counts as a user of,
		// so an unsuspend that ran first would stall the cluster on the very
		// volumes this branch is freeing.
		if err := r.resumeCluster(ctx, &lrr); err != nil {
			return ctrl.Result{}, err
		}

		return ctrl.Result{}, r.releaseClaim(ctx, &lrr)
	}

	var outcome restore.Outcome
	switch lrr.Status.Phase {
	case "", v1.LogicalRestorePending:
		outcome, err = r.admit(ctx, &lrr)
	case v1.LogicalRestoreValidatingCompatibility:
		outcome, err = r.validate(ctx, &lrr)
	case v1.LogicalRestoreRestoringSecondaryStorage:
		outcome, err = r.restoreDatabase(ctx, &lrr)
	case v1.LogicalRestoreRestoringPrimaryStorage:
		outcome, err = r.restorePrimaryStorage(ctx, &lrr)
	default:
		return ctrl.Result{}, fmt.Errorf("unknown phase %q", lrr.Status.Phase)
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

// complete ends the restore. The logical database and the broker volumes now
// hold the state of the backup, and whoever owns the cluster can unsuspend it.
func (r *Reconciler) complete(lrr *v1.LogicalRestoreRDBMS) {
	lrr.Status.Phase = v1.LogicalRestoreCompleted
	restore.Complete(&lrr.Status.RestoreProgress, metav1.Now())
	restore.StageTerminal(lrr, &lrr.Status.RestoreProgress)
	r.EventRecorder.Eventf(
		lrr,
		nil,
		corev1.EventTypeNormal,
		restore.EventReasonCompleted,
		restore.EventActionRestore,
		"The restore of backup %d into CamundaCluster %s/%s finished",
		lrr.Status.BackupID,
		lrr.Namespace,
		lrr.Spec.TargetClusterRef.Name,
	)
}

// fail ends the restore with reason and message. Status keeps the reason, so
// a terminal look stages the same one again after a write conflict.
func (r *Reconciler) fail(lrr *v1.LogicalRestoreRDBMS, reason, message string) {
	lrr.Status.Phase = v1.LogicalRestoreFailed
	restore.Fail(&lrr.Status.RestoreProgress, reason, message, metav1.Now())
	restore.StageTerminal(lrr, &lrr.Status.RestoreProgress)
	r.EventRecorder.Eventf(
		lrr,
		nil,
		corev1.EventTypeWarning,
		restore.EventReasonFailed,
		restore.EventActionRestore,
		"The restore failed: %s",
		lrr.Status.FailureMessage,
	)
}

// progressing stages that a phase of the restore runs.
func (r *Reconciler) progressing(lrr *v1.LogicalRestoreRDBMS, message string) {
	conditions.Stage(lrr, conditions.Ready(
		metav1.ConditionFalse, v1.ReasonProgressing, message, lrr.Generation,
	))
}

// waiting stages a pre-check failure that holds the restore in Pending. The
// restore has touched nothing, so it goes back to waiting and recovers on its
// own once the cause is gone. Nothing bounds this hold.
func (r *Reconciler) waiting(
	lrr *v1.LogicalRestoreRDBMS,
	failure *conditions.PreCheckFailure,
) restore.Outcome {
	lrr.Status.Phase = v1.LogicalRestorePending
	restore.Recovered(&lrr.Status.RestoreProgress)
	conditions.Stage(lrr, conditions.Failed(lrr, failure))

	// The watches on the cluster and on the backup resolve every hold that
	// admission reports. The timer is the safety net.
	return restore.Outcome{Wait: r.opts.RetryInterval}
}

// holdStarted holds a started restore on a dependency that stopped resolving,
// and ends it once the mid-run grace is over. The phase of this kind is the
// controller's to write, so the terminal transition happens here.
func (r *Reconciler) holdStarted(
	lrr *v1.LogicalRestoreRDBMS,
	failure *conditions.PreCheckFailure,
) restore.Outcome {
	outcome := restore.HoldRunning(
		lrr,
		&lrr.Status.RestoreProgress,
		failure,
		metav1.Now(),
		r.opts.MidRunGrace,
		r.opts.PollInterval,
	)
	if outcome.Failure != nil {
		r.fail(lrr, outcome.Failure.Reason, outcome.Failure.Message)

		return restore.Outcome{}
	}

	return outcome
}

// claimant is the identity under which the restore holds the claim on its
// target cluster.
func claimant(lrr *v1.LogicalRestoreRDBMS) clusterclaim.Claimant {
	return clusterclaim.Claimant{Kind: lrr.GetKind(), Name: lrr.Name, UID: lrr.UID}
}

// resumeCluster withdraws the suspension that this restore applied to its
// target. It is a no-op unless this restore completed and this restore is what
// suspended the target, which restore.Resume decides from the recorded progress.
//
// It runs on every look of a terminal restore, so an attempt that failed
// heals on the next one, and it writes nothing once the field is gone.
func (r *Reconciler) resumeCluster(ctx context.Context, lrr *v1.LogicalRestoreRDBMS) error {
	return restore.Resume(
		ctx,
		r.Client,
		r.APIReader,
		&lrr.Status.RestoreProgress,
		types.NamespacedName{Namespace: lrr.Namespace, Name: lrr.Spec.TargetClusterRef.Name},
	)
}

// releaseClaim gives the claim on the target cluster back. It is a no-op when
// the restore does not hold it.
func (r *Reconciler) releaseClaim(ctx context.Context, lrr *v1.LogicalRestoreRDBMS) error {
	return restore.Give(
		ctx, r.Client, r.APIReader, lrr.Namespace, lrr.Spec.TargetClusterRef.Name, claimant(lrr),
	)
}

// SetupWithManager applies the options, registers the controller, the two
// field indexes, and the watches: the restores, the Jobs they own, the target
// clusters, and the backups they read. A suspend flip and a backup that
// reaches Completed both wake a waiting restore without a timer.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	resolved, err := r.opts.withDefaults()
	if err != nil {
		return err
	}
	r.opts = resolved

	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder("logicalrestorerdbms")
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1.LogicalRestoreRDBMS{},
		clusterRefField,
		func(obj client.Object) []string {
			lrr := obj.(*v1.LogicalRestoreRDBMS)

			return []string{refindex.NamespacedKey(lrr.Namespace, lrr.Spec.TargetClusterRef.Name)}
		},
	); err != nil {
		return fmt.Errorf("indexing LogicalRestoreRDBMS by targetClusterRef: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1.LogicalRestoreRDBMS{},
		backupRefField,
		func(obj client.Object) []string {
			lrr := obj.(*v1.LogicalRestoreRDBMS)

			return []string{refindex.NamespacedKey(lrr.Namespace, lrr.Spec.BackupRef.Name)}
		},
	); err != nil {
		return fmt.Errorf("indexing LogicalRestoreRDBMS by backupRef: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.LogicalRestoreRDBMS{}).
		Owns(&batchv1.Job{}).
		Watches(
			&v1.CamundaCluster{},
			refindex.Enqueue(
				mgr.GetClient(),
				&v1.LogicalRestoreRDBMSList{},
				clusterRefField,
				refindex.ObjectNamespacedName,
			),
		).
		Watches(
			&v1.LogicalBackupRDBMS{},
			refindex.Enqueue(
				mgr.GetClient(),
				&v1.LogicalRestoreRDBMSList{},
				backupRefField,
				refindex.ObjectNamespacedName,
			),
		).
		Named("logicalrestorerdbms").
		Complete(r)
}
