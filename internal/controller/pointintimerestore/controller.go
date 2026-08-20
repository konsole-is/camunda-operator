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
// CamundaCluster with a database that somebody else already restored to a
// point in time. The operator never restores the database server.
//
// The files follow the phases. admit.go resolves the storage chain and checks
// the rules of the server. dbstate.go reads the exporter position of every
// partition and refuses a database that is ahead of the requested point.
// primary.go recreates the broker data volumes and runs the restore
// application on them. This file holds what every phase shares: Reconcile, the
// terminal transitions, and the wiring.
//
// Nothing is destroyed until every check passed. Every failure before
// primary.go holds the restore in Pending and touches no volume, so the owner
// can correct the cause and the same resource proceeds.
package pointintimerestore

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
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/pgbootstrap"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
)

const (
	// clusterRefField indexes restores by the namespace and name of their
	// clusterRef, so an event on a cluster wakes the restores that wait for
	// it, for example on the flip of spec.suspend.
	clusterRefField = "pointintimerestore.spec.clusterRef"
	// databaseServerRefField indexes Database resources by the name of their
	// serverRef, so the dedicated-server rule is one indexed list and not a
	// full scan. The Database controller indexes the same field under a name
	// of its own, and a manager rejects a second registration of one index
	// name, so this one carries the name of this controller.
	databaseServerRefField = "pointintimerestore.database.spec.serverRef"

	// defaultPollInterval paces a running phase.
	defaultPollInterval = 5 * time.Second
	// defaultRetryInterval paces a hold that no watch resolves.
	defaultRetryInterval = 30 * time.Second
	// defaultMidRunGrace bounds how long a started restore waits on a
	// dependency that stopped resolving before the restore fails. A restore
	// that already deleted a broker volume must reach a terminal phase, so
	// that whoever owns the cluster learns that it has to act.
	defaultMidRunGrace = 10 * time.Minute

	eventReasonStarted   = "RestoreStarted"
	eventReasonCompleted = "RestoreCompleted"
	eventReasonFailed    = "RestoreFailed"
	eventActionRestore   = "Restore"
)

// hold is the domain result of one reconcile phase. It says how long to wait
// before the next look, or nothing when the watches carry the wake-up. Only
// Reconcile turns it into a ctrl.Result.
type hold struct {
	after time.Duration
}

// settle waits on the watches alone.
var settle = hold{}

// shortly re-enters after a staged status is persisted, so the next side
// effect acts on a durable record.
var shortly = hold{after: time.Second}

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
	// volume reads live state: a stale suspend flag, a stale claim, or a
	// stale Job would each let a restore act on a cluster that moved on.
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

// +kubebuilder:rbac:groups=core.camunda.io,resources=pointintimerestores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=pointintimerestores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=pointintimerestores/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters;secondarystorageconfigs;databaseconfigs;databaseserverconfigs;databases,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
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
		return ctrl.Result{}, client.IgnoreNotFound(err)
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
		APIReader:     r.APIReader,
		Owner:         &pitr,
	}
	defer func() {
		if flushErr := component.FlushStatus(ctx, rec, nil); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	if pitr.Terminal() {
		// A conflict on the terminal flush can restore a stale Ready from the
		// server. Staging it again on every look heals that.
		r.stageTerminal(&pitr)

		return ctrl.Result{}, nil
	}

	var wait hold
	switch pitr.Status.Phase {
	case "", v1.PointInTimeRestorePending:
		wait, err = r.admit(ctx, &pitr)
	case v1.PointInTimeRestoreValidatingDatabaseState:
		wait, err = r.enterDatabaseState(ctx, &pitr)
	case v1.PointInTimeRestoreRestoringPrimaryStorage:
		wait, err = r.restorePrimaryStorage(ctx, &pitr)
	default:
		return ctrl.Result{}, fmt.Errorf("unknown phase %q", pitr.Status.Phase)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: wait.after}, nil
}

// complete ends the restore. The brokers now hold the state of the requested
// point, and whoever owns the cluster can unsuspend it.
func (r *Reconciler) complete(pitr *v1.PointInTimeRestore) {
	now := metav1.Now()
	pitr.Status.Phase = v1.PointInTimeRestoreCompleted
	pitr.Status.CompletionTime = &now
	pitr.Status.TerminalReason = v1.ReasonCompleted
	r.stageTerminal(pitr)
	r.EventRecorder.Eventf(
		pitr,
		nil,
		corev1.EventTypeNormal,
		eventReasonCompleted,
		eventActionRestore,
		"The restore of CamundaCluster %s/%s to %s finished",
		pitr.Namespace,
		pitr.Spec.ClusterRef.Name,
		pitr.Spec.Timestamp.UTC().Format(time.RFC3339),
	)
}

// fail ends the restore with reason and message. Status keeps the reason, so a
// later look stages the same one again. The message carries an external error
// whose size the controller cannot know, for example the reason of a Job or of
// a pod, so it is bounded before it reaches the free-form status field.
func (r *Reconciler) fail(pitr *v1.PointInTimeRestore, reason, message string) {
	now := metav1.Now()
	message = conditions.BoundMessage(message)
	pitr.Status.Phase = v1.PointInTimeRestoreFailed
	pitr.Status.CompletionTime = &now
	pitr.Status.TerminalReason = reason
	pitr.Status.FailureMessage = message
	r.stageTerminal(pitr)
	r.EventRecorder.Eventf(
		pitr,
		nil,
		corev1.EventTypeWarning,
		eventReasonFailed,
		eventActionRestore,
		"The restore failed: %s",
		message,
	)
}

// stageTerminal stages the Ready condition of a terminal phase. It is
// idempotent, so a terminal restore stages it again on every look and heals a
// conflict that restored a stale condition.
func (r *Reconciler) stageTerminal(pitr *v1.PointInTimeRestore) {
	switch pitr.Status.Phase {
	case v1.PointInTimeRestoreCompleted:
		conditions.Stage(pitr, conditions.Ready(
			metav1.ConditionTrue,
			v1.ReasonCompleted,
			"The restore finished. The cluster can be unsuspended",
			pitr.Generation,
		))
	case v1.PointInTimeRestoreFailed:
		reason := pitr.Status.TerminalReason
		if reason == "" {
			reason = v1.ReasonFailed
		}
		conditions.Stage(pitr, conditions.Ready(
			metav1.ConditionFalse, reason, pitr.Status.FailureMessage, pitr.Generation,
		))
	}
}

// progressing stages that a phase of the restore runs.
func (r *Reconciler) progressing(pitr *v1.PointInTimeRestore, message string) {
	conditions.Stage(pitr, conditions.Ready(
		metav1.ConditionFalse, v1.ReasonProgressing, message, pitr.Generation,
	))
}

// waiting stages a pre-check failure that holds the restore in Pending. The
// restore has touched nothing, so it goes back to waiting and recovers on its
// own once the cause is gone.
func (r *Reconciler) waiting(pitr *v1.PointInTimeRestore, failure *conditions.PreCheckFailure) hold {
	pitr.Status.Phase = v1.PointInTimeRestorePending
	pitr.Status.FirstFailedAt = nil
	conditions.Stage(pitr, conditions.Failed(pitr, failure))

	return hold{after: r.opts.RetryInterval}
}

// holdStarted stages a dependency failure of a started restore and decides its
// fate. Within the grace it holds the restore on a timer. Past the grace the
// restore fails, because a restore that already recreated a broker volume has
// nothing to go back to. The grace counts from when the dependency first
// stopped resolving, which recovered clears.
func (r *Reconciler) holdStarted(
	pitr *v1.PointInTimeRestore,
	failure *conditions.PreCheckFailure,
) hold {
	now := metav1.Now()
	if pitr.Status.FirstFailedAt == nil {
		pitr.Status.FirstFailedAt = &now
	}
	if now.Sub(pitr.Status.FirstFailedAt.Time) > r.opts.MidRunGrace {
		r.fail(pitr, v1.ReasonFailed, fmt.Sprintf(
			"a dependency stopped resolving and did not recover: %s", failure.Message,
		))

		return settle
	}
	conditions.Stage(pitr, conditions.Failed(pitr, failure))

	// A started restore looks again at the cadence of a running phase, not at
	// the slower cadence of an admission hold. The grace is measured in this
	// loop, so the loop has to run inside it.
	return hold{after: r.opts.PollInterval}
}

// recovered clears the clock of the mid-run grace. The phase just resolved
// what it needs, so the next failure gets the full grace again.
func recovered(pitr *v1.PointInTimeRestore) {
	pitr.Status.FirstFailedAt = nil
}

// SetupWithManager registers the controller, the two field indexes, and the
// watches: the restores, the Jobs they own, and the clusters they name.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder("pointintimerestore")
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

	// A DatabaseServerConfig is cluster-scoped, so the dedicated-server rule
	// counts the Database resources of every namespace. The index keys them by
	// the name of the server alone.
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1.Database{},
		databaseServerRefField,
		func(obj client.Object) []string {
			return []string{obj.(*v1.Database).Spec.ServerRef}
		},
	); err != nil {
		return fmt.Errorf("indexing Database by serverRef: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.PointInTimeRestore{}).
		Owns(&batchv1.Job{}).
		Watches(
			&v1.CamundaCluster{},
			refindex.Enqueue(
				mgr.GetClient(),
				&v1.PointInTimeRestoreList{},
				clusterRefField,
				refindex.ObjectNamespacedName,
			),
		).
		Named("pointintimerestore").
		Complete(r)
}
