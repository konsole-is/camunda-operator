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

// Package pointintimerestore reconciles a PointInTimeRestore. One resource
// aligns the primary storage of one suspended, relational-backed
// CamundaCluster with a database at a point in time.
//
// The database reaches that point in one of two ways. A DatabaseServerConfig
// that declares pitr.recovery: operator is rolled back by whoever publishes
// it, and the restore asks for that through the contract. A contract that
// declares external is rolled back before the restore is created.
//
// The files follow the phases. admit.go resolves the storage chain, checks
// the rules of the server, takes the claim on the cluster, and suspends the
// cluster. dbrecovery.go asks the contract for the rollback and waits for the
// answer. dbstate.go reads the exporter position of every partition and
// refuses a database that is ahead of the requested point. primary.go runs the
// shared primary-storage phase of pkg/restore, which recreates the broker data
// volumes and runs the restore application on them. refusal.go reads the log
// of a restore Job that failed and names the one refusal that the operator
// reports for the user. This file holds what every phase shares: Reconcile,
// the terminal transitions, and the wiring.
//
// The machinery that every restore kind shares lives in pkg/restore. This
// package holds the phase vocabulary of this kind, its own rules, and the
// mapping of a driver outcome onto its phase.
//
// Nothing is destroyed before the RestoringPrimaryStorage phase. A failure of
// an earlier phase holds the restore and touches no volume, in one of two
// ways. A rule of the cluster or of the server that does not hold, and a
// database that is ahead of the requested point, hold the restore in Pending
// without a bound: the owner corrects the cause, and the same resource
// continues. A database that the operator cannot reach holds it in
// ValidatingDatabaseState, where the mid-run grace bounds the wait and the
// restore fails after it.
//
// One refusal cannot be caught before the destruction. The operator compares
// timestamps, and the restore application compares log positions, so only the
// application knows whether a primary-storage checkpoint covers the exporter
// position of the database. That answer arrives in the log of a failed Job,
// after the volumes are erased. refusal.go reads it and reports the remedy.
package pointintimerestore

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/observability"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/pgbootstrap"
	"github.com/konsole-is/camunda-operator/pkg/podstate"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/restore"
)

// controllerName is the name the controller registers with controller-runtime.
// It labels its events and every metrics series it records.
const controllerName = "pointintimerestore"

const (
	// clusterRefField indexes restores by the namespace and name of their
	// clusterRef, so an event on a cluster wakes the restores that wait for
	// it, for example on the flip of spec.suspend.
	clusterRefField = "pointintimerestore.spec.clusterRef"
	// defaultPollInterval paces a running phase.
	defaultPollInterval = 5 * time.Second
	// defaultRetryInterval paces a hold that no watch resolves.
	defaultRetryInterval = 30 * time.Second
	// defaultMidRunGrace bounds how long a started restore waits on a
	// dependency that stopped resolving before the restore fails. A restore
	// that already deleted a broker volume must reach a terminal phase, so
	// that whoever owns the cluster learns that it has to act.
	defaultMidRunGrace = 10 * time.Minute
)

// ReadPositions reads the exporter position of every partition from the
// logical database of a cluster.
type ReadPositions func(
	ctx context.Context, conn pgbootstrap.Connection, database string,
) ([]v1.PartitionPosition, error)

// Options tunes a Reconciler. The zero value is the production configuration.
// Tests shrink the intervals and point the reader at a fake.
type Options struct {
	// PollInterval paces a running phase. Zero means five seconds.
	PollInterval time.Duration
	// RetryInterval paces a hold that no watch resolves. Zero means thirty
	// seconds.
	RetryInterval time.Duration
	// MidRunGrace bounds how long a started restore waits on a dependency
	// that stopped resolving. Zero means ten minutes.
	MidRunGrace time.Duration
	// ReadPositions reads the exporter position of every partition from the
	// logical database. Nil means the production reader, which connects with
	// pgbootstrap. The tests point it at a fake.
	ReadPositions ReadPositions
	// ReadJobLog reads the log of a restore Job that already failed, so that
	// the controller can name the cause. Nil means the production reader,
	// which SetupWithManager builds from the REST configuration of the
	// manager. The tests point it at a fake, because envtest serves no pod
	// log.
	ReadJobLog ReadJobLog
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
	if o.ReadPositions == nil {
		o.ReadPositions = readPositions
	}

	return o
}

// Reconciler drives a PointInTimeRestore to a terminal phase.
type Reconciler struct {
	client.Client
	// APIReader reads without the cache. Every decision that deletes a broker
	// volume reads live state. A stale suspend flag, a stale claim, or a stale
	// Job each let a restore act on a cluster that moved on.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the lifecycle events of the restore.
	// SetupWithManager sets it from the manager when it is nil.
	EventRecorder events.EventRecorder
	// Metrics records the condition gauge and the apply counters of the
	// framework. SetupWithManager sets it when it is nil.
	Metrics component.MetricsRecorder
	// ClaimNamespace holds the claim Lease of every logical database. The
	// dedicated-server rule reads them to learn which PostgreSQL instance
	// each Database occupies. It is the namespace of the operator, and
	// SetupWithManager refuses an empty one.
	ClaimNamespace string

	opts Options
}

// New returns a Reconciler with the given options, with every zero field
// filled from the production configuration.
func New(
	c client.Client,
	reader client.Reader,
	scheme *runtime.Scheme,
	claimNamespace string,
	options Options,
) *Reconciler {
	return &Reconciler{
		Client:         c,
		APIReader:      reader,
		Scheme:         scheme,
		ClaimNamespace: claimNamespace,
		opts:           options.withDefaults(),
	}
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=pointintimerestores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=pointintimerestores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=pointintimerestores/finalizers,verbs=update
// The restore prepares its own cluster: it suspends the cluster and withdraws
// that suspension when it completes. This kind writes no version, because it
// restores the primary storage of a cluster from the continuous backups of
// that same cluster. The write is a server-side apply of one field under a
// field manager of its own, so patch is the only verb it needs on a
// CamundaCluster.
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=secondarystorageconfigs;databaseconfigs;databases,verbs=get;list;watch
// A contract that declares pitr.recovery: operator is asked to roll its own
// server back. The restore applies spec.recovery on it under a field manager
// of its own, and it has no permission on whoever publishes the contract.
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverconfigs,verbs=get;list;watch;patch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps;secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list;watch
// The log of a failed restore Job carries the one refusal that the operator
// names for the user: no primary-storage checkpoint covers the exporter
// position of the restored database.
// +kubebuilder:rbac:groups="",resources=pods/log,verbs=get
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// The cluster claim reads the resource of whichever kind holds the Lease,
// to decide whether that holder still needs the cluster.
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackupelasticsearches;logicalbackuprdbmses;logicalrestoreelasticsearches;logicalrestorerdbmses,verbs=get
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile advances the restore by at most one phase. The phase is the resume
// marker: it is persisted before the side effect it names, so a restore that
// re-enters after a crash continues where it stopped.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	// The restore is its own state machine. A stale cached read of its status
	// re-enters a phase whose side effect already ran, and the side effect of
	// this controller deletes data.
	var pitr v1.PointInTimeRestore
	if err := r.APIReader.Get(ctx, req.NamespacedName, &pitr); err != nil {
		if apierrors.IsNotFound(err) {
			observability.Forget(r.Metrics, new(v1.PointInTimeRestore).GetKind(), req.NamespacedName)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	// The Jobs of a restore carry a controller reference to it, so the garbage
	// collector removes them with the restore. It writes nothing outside the
	// cluster, so it needs no finalizer.
	if !pitr.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	rec := component.ReconcileContext{
		Client:        r.Client,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
		Metrics:       r.Metrics,
		APIReader:     r.APIReader,
		Owner:         &pitr,
	}
	defer func() {
		if flushErr := component.FlushStatus(ctx, rec, nil); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	if pitr.Terminal() {
		// The terminal branch that every restore kind shares. It stages the
		// recorded outcome again, and it gives the Jobs, the suspension, and
		// the claim back in the one order that frees the broker volumes. The
		// Jobs of a completed restore take more than one look to go, so an
		// answer that is not Done holds the two steps behind them.
		finished, err := restore.Finish(
			ctx, r.Client, r.APIReader, &pitr, &pitr.Status.RestoreProgress, pitr.Spec.ClusterRef.Name,
		)

		return ctrl.Result{RequeueAfter: finished.Wait}, err
	}

	var outcome restore.Outcome
	switch pitr.Status.Phase {
	case "", v1.PointInTimeRestorePending:
		outcome, err = r.admit(ctx, &pitr)
	case v1.PointInTimeRestoreRestoringDatabase:
		outcome, err = r.enterDatabaseRecovery(ctx, &pitr)
	case v1.PointInTimeRestoreValidatingDatabaseState:
		outcome, err = r.enterDatabaseState(ctx, &pitr)
	case v1.PointInTimeRestoreRestoringPrimaryStorage:
		outcome, err = r.restorePrimaryStorage(ctx, &pitr)
	default:
		return ctrl.Result{}, fmt.Errorf("unknown phase %q", pitr.Status.Phase)
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

// complete ends the restore. The brokers now hold the state of the requested
// point, and whoever owns the cluster can unsuspend it.
func (r *Reconciler) complete(pitr *v1.PointInTimeRestore) {
	pitr.Status.Phase = v1.PointInTimeRestoreCompleted
	restore.Complete(&pitr.Status.RestoreProgress, metav1.Now())
	restore.StageTerminal(pitr, &pitr.Status.RestoreProgress)
	r.EventRecorder.Eventf(
		pitr,
		nil,
		corev1.EventTypeNormal,
		restore.EventReasonCompleted,
		restore.EventActionRestore,
		"The restore of CamundaCluster %s/%s to %s finished",
		pitr.Namespace,
		pitr.Spec.ClusterRef.Name,
		pitr.Spec.Timestamp.UTC().Format(time.RFC3339),
	)
}

// fail ends the restore with reason and message. Every failure that this kind
// raises itself reports the reason Failed: the causes that carry a reason of
// their own, for example a server without point-in-time recovery, hold the
// restore instead of ending it. Status keeps the reason, so a terminal look
// stages the same one again after a write conflict.
func (r *Reconciler) fail(pitr *v1.PointInTimeRestore, reason, message string) {
	pitr.Status.Phase = v1.PointInTimeRestoreFailed
	restore.Fail(&pitr.Status.RestoreProgress, reason, message, metav1.Now())
	restore.StageTerminal(pitr, &pitr.Status.RestoreProgress)
	r.EventRecorder.Eventf(
		pitr,
		nil,
		corev1.EventTypeWarning,
		restore.EventReasonFailed,
		restore.EventActionRestore,
		"The restore failed: %s",
		pitr.Status.FailureMessage,
	)
}

// progressing stages that a phase of the restore runs.
func (r *Reconciler) progressing(pitr *v1.PointInTimeRestore, message string) {
	conditions.Stage(pitr, conditions.Ready(
		metav1.ConditionFalse, v1.ReasonProgressing, message, pitr.Generation,
	))
}

// waiting stages a pre-check failure that holds the restore in Pending. The
// restore has touched nothing, so it goes back to waiting and recovers on its
// own once the cause is gone. Nothing bounds this hold.
func (r *Reconciler) waiting(
	pitr *v1.PointInTimeRestore,
	failure *conditions.PreCheckFailure,
) restore.Outcome {
	pitr.Status.Phase = v1.PointInTimeRestorePending
	restore.Recovered(&pitr.Status.RestoreProgress)
	conditions.Stage(pitr, conditions.Failed(pitr, failure))

	return restore.Outcome{Wait: r.opts.RetryInterval}
}

// holdStarted holds a started restore on a dependency that stopped resolving,
// and ends it once the mid-run grace is over. The phase of this kind is the
// controller's to write, so the terminal transition happens here.
func (r *Reconciler) holdStarted(
	pitr *v1.PointInTimeRestore,
	failure *conditions.PreCheckFailure,
) restore.Outcome {
	outcome := restore.HoldRunning(
		pitr,
		&pitr.Status.RestoreProgress,
		failure,
		metav1.Now(),
		r.opts.MidRunGrace,
		r.opts.PollInterval,
	)
	if outcome.Failure != nil {
		r.fail(pitr, outcome.Failure.Reason, outcome.Failure.Message)

		return restore.Outcome{}
	}

	return outcome
}

// SetupWithManager registers the controller, the field index, and the
// watches: the restores, the Jobs they own, the pods of those Jobs, and the
// clusters they name. It refuses a reconciler without a namespace for the
// claim Leases.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.ClaimNamespace == "" {
		return errors.New("the namespace of the claim Leases is required")
	}

	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder(controllerName)
	}
	if r.Metrics == nil {
		r.Metrics = observability.Recorder(controllerName)
	}

	// A pod log is a stream, so it needs a typed REST client that the
	// client.Client of the manager cannot give. The reader is built here
	// rather than in withDefaults, because only the manager holds the REST
	// configuration.
	if r.opts.ReadJobLog == nil {
		clientset, err := kubernetes.NewForConfig(mgr.GetConfig())
		if err != nil {
			return fmt.Errorf("building the client that reads restore Job logs: %w", err)
		}
		r.opts.ReadJobLog = podLogReader(clientset.CoreV1())
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1.PointInTimeRestore{},
		clusterRefField,
		func(obj client.Object) []string {
			pitr := obj.(*v1.PointInTimeRestore)

			return []string{refindex.NamespacedKey(pitr.Namespace, pitr.Spec.ClusterRef.Name)}
		},
	); err != nil {
		return fmt.Errorf("indexing PointInTimeRestore by clusterRef: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.PointInTimeRestore{}).
		Owns(&batchv1.Job{}).
		// A Job reports nothing about a pod of its own that cannot start,
		// so the pods are watched next to the Jobs.
		Watches(
			&corev1.Pod{},
			podstate.EnqueueJobOwner(
				mgr.GetClient(), v1.GroupVersion.WithKind("PointInTimeRestore").GroupKind(),
			),
		).
		Watches(
			&v1.CamundaCluster{},
			refindex.Enqueue(
				mgr.GetClient(),
				&v1.PointInTimeRestoreList{},
				clusterRefField,
				refindex.ObjectNamespacedName,
			),
		).
		Named(controllerName).
		Complete(r)
}
