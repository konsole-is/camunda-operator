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

// Package logicalrestore reconciles a LogicalRestore. One resource restores
// one completed logical backup into one suspended CamundaCluster, and runs
// once.
//
// The phase is the resume marker, and it is persisted before the side effect
// it names. Pending holds the restore until its references resolve and the
// target is suspended. ValidatingCompatibility refuses a target that cannot
// hold the backup. RestoringSecondaryStorage writes the backup back into
// Elasticsearch or into the logical database. RestoringPrimaryStorage gives
// the brokers empty data volumes and runs the Camunda restore application on
// them, once per broker.
//
// The controller only reads spec.suspend of the target. Whoever owns the
// cluster suspends it before the restore and unsuspends it after.
//
// The files follow the phases. admit.go resolves the references and holds the
// restore in Pending. compatibility.go compares the backup against the
// target. secondary_elasticsearch.go and secondary_rdbms.go rebuild secondary
// storage, one file per storage type. primary.go recreates the broker volumes
// and runs the restore Jobs. This file holds what every phase shares:
// Reconcile, the terminal transitions, and the wiring.
package logicalrestore

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
	"github.com/konsole-is/camunda-operator/pkg/refindex"
)

const (
	// clusterKeyIndex indexes restores by the namespace and name of their
	// targetClusterRef, so a suspend flip wakes the restores that wait for it.
	clusterKeyIndex = "logicalrestore.spec.targetClusterRef"
	// backupKeyIndex indexes restores by the kind, namespace, and name of
	// their backupRef, so a backup that reaches Completed wakes them.
	backupKeyIndex = "logicalrestore.spec.backupRef"

	// defaultPollInterval paces a phase that runs. It is the fallback of the
	// watches on the Jobs and on the referenced resources.
	defaultPollInterval = 5 * time.Second
	// defaultRetryInterval paces a hold that no watch resolves.
	defaultRetryInterval = 30 * time.Second
	// defaultMidRunGrace bounds how long a started restore waits on a
	// dependency that stopped resolving before the restore fails. A restore
	// that already rewrote storage must reach a terminal phase, so a broken
	// dependency can hold it for this long at most.
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

var (
	// settle waits on the watches alone.
	settle = hold{}
	// shortly re-enters to persist staged status before acting on it.
	shortly = hold{after: time.Second}
)

// Options configures the reconciler at construction. Only CLIImage is
// required. Every other field has a default that fits production, and a test
// sets what it needs to observe.
type Options struct {
	// CLIImage is the camunda-operator-cli image that downloads the dump of a
	// relational backup. The manager passes --camunda-operator-cli-image.
	CLIImage string
	// PollInterval overrides defaultPollInterval. Zero means the default.
	PollInterval time.Duration
	// RetryInterval overrides defaultRetryInterval. Zero means the default.
	RetryInterval time.Duration
	// MidRunGrace overrides defaultMidRunGrace. Zero means the default.
	MidRunGrace time.Duration
}

// withDefaults fills the zero fields of o with the production defaults. It
// rejects an empty CLIImage, because the relational path cannot guess an
// image and the restore would fail only once it reached that phase.
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

// Reconciler drives a LogicalRestore to a terminal phase.
type Reconciler struct {
	client.Client
	// APIReader reads without the cache. It also reads the restore itself: a
	// stale phase re-enters a phase whose side effect already ran, and a
	// stale suspend flag admits a restore that must wait.
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

// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestores/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestores/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters;camundaclusterpresets;secondarystorageconfigs;databaseconfigs;databaseserverconfigs;objectstorageconfigs;logicalbackupelasticsearches;logicalbackuprdbmses,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile advances the restore by at most one phase. It is the only
// function that builds a ctrl.Result.
//
// The restore is read live, not from the cache. The phase switch is the
// authority of the state machine, and a requeue that arrives before the
// informer caught up with the last flush must not re-enter a phase whose side
// effect already ran.
//
// The restore needs no finalizer. It writes no artifact to an external store,
// and its Jobs carry a controller reference, so deleting the resource removes
// them.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	var restore v1.LogicalRestore
	if err := r.APIReader.Get(ctx, req.NamespacedName, &restore); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !restore.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	rec := component.ReconcileContext{
		Client:        r.Client,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
		APIReader:     r.APIReader,
		Owner:         &restore,
	}
	defer func() {
		if flushErr := component.FlushStatus(ctx, rec, nil); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	if restore.Terminal() {
		// A conflict on the terminal flush can restore a stale Ready from the
		// server. stageTerminal is idempotent and heals it on the next look.
		r.stageTerminal(&restore)

		return ctrl.Result{}, nil
	}

	var wait hold
	switch restore.Status.Phase {
	case "", v1.LogicalRestorePending:
		wait, err = r.admit(ctx, &restore)
	case v1.LogicalRestoreValidatingCompatibility:
		wait, err = r.validate(ctx, &restore)
	case v1.LogicalRestoreRestoringSecondaryStorage:
		wait, err = r.restoreSecondaryStorage(ctx, &restore)
	case v1.LogicalRestoreRestoringPrimaryStorage:
		wait, err = r.restorePrimaryStorage(ctx, &restore)
	default:
		return ctrl.Result{}, fmt.Errorf("unknown phase %q", restore.Status.Phase)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: wait.after}, nil
}

// restoreSecondaryStorage routes the phase to the procedure of the pinned
// storage type. The type is pinned at admission, so a storage contract that
// changes mid-run cannot switch the procedure.
func (r *Reconciler) restoreSecondaryStorage(
	ctx context.Context,
	restore *v1.LogicalRestore,
) (hold, error) {
	switch restore.Status.StorageType {
	case v1.SecondaryStorageTypeElasticsearch:
		return r.restoreElasticsearch(ctx, restore)
	case v1.SecondaryStorageTypeRDBMS:
		return r.restoreDatabase(ctx, restore)
	default:
		return settle, fmt.Errorf("unknown secondary storage type %q", restore.Status.StorageType)
	}
}

// complete ends the restore. The target can be unsuspended.
func (r *Reconciler) complete(restore *v1.LogicalRestore) {
	now := metav1.Now()
	restore.Status.Phase = v1.LogicalRestoreCompleted
	restore.Status.CompletionTime = &now
	restore.Status.TerminalReason = v1.ReasonCompleted
	r.stageTerminal(restore)
	r.EventRecorder.Eventf(
		restore,
		nil,
		corev1.EventTypeNormal,
		eventReasonCompleted,
		eventActionRestore,
		"Restore of backup %d completed",
		restore.Status.BackupID,
	)
}

// fail ends the restore with reason and message. The reason reaches the Ready
// condition, and status keeps it, so a later look stages the same reason
// again. The message often carries an external error whose size the controller
// cannot know, for example an Elasticsearch body, the waiting message of a
// pod, or a Job reason. So fail bounds it before it reaches the free-form
// status field.
func (r *Reconciler) fail(restore *v1.LogicalRestore, reason, message string) {
	now := metav1.Now()
	message = conditions.BoundMessage(message)
	restore.Status.Phase = v1.LogicalRestoreFailed
	restore.Status.CompletionTime = &now
	restore.Status.TerminalReason = reason
	restore.Status.FailureMessage = message
	r.stageTerminal(restore)
	r.EventRecorder.Eventf(
		restore,
		nil,
		corev1.EventTypeWarning,
		eventReasonFailed,
		eventActionRestore,
		"Restore failed: %s",
		message,
	)
}

// stageTerminal stages the Ready condition of a terminal phase. It is
// idempotent, so a terminal restore stages it again on every look and heals a
// conflict that restored a stale condition.
func (r *Reconciler) stageTerminal(restore *v1.LogicalRestore) {
	switch restore.Status.Phase {
	case v1.LogicalRestoreCompleted:
		conditions.Stage(restore, conditions.Ready(
			metav1.ConditionTrue,
			v1.ReasonCompleted,
			"the restore finished; the target cluster can be unsuspended",
			restore.Generation,
		))
	case v1.LogicalRestoreFailed:
		reason := restore.Status.TerminalReason
		if reason == "" {
			reason = v1.ReasonFailed
		}
		conditions.Stage(restore, conditions.Ready(
			metav1.ConditionFalse, reason, restore.Status.FailureMessage, restore.Generation,
		))
	}
}

// progressing is the Ready condition of a phase that runs.
func progressing(restore *v1.LogicalRestore, message string) metav1.Condition {
	return conditions.Ready(metav1.ConditionFalse, v1.ReasonProgressing, message, restore.Generation)
}

// holdRunning stages a mid-run failure and decides its fate. Within the
// grace it holds the restore on a timer. Past the grace the restore fails.
// The grace counts from when the dependency first stopped resolving, which
// status records, so an old restore gets the same full grace as a fresh one.
func (r *Reconciler) holdRunning(
	restore *v1.LogicalRestore,
	failure *conditions.PreCheckFailure,
) hold {
	now := metav1.Now()
	if restore.Status.FirstFailedAt == nil {
		restore.Status.FirstFailedAt = &now
	}
	if now.Sub(restore.Status.FirstFailedAt.Time) > r.opts.MidRunGrace {
		r.fail(restore, v1.ReasonFailed, fmt.Sprintf(
			"a dependency stopped resolving and did not recover: %s", failure.Message,
		))

		return settle
	}

	conditions.Stage(restore, conditions.Failed(restore, failure))

	// A started restore re-checks at the cadence of a running phase, not at
	// the slower cadence of an admission hold. The grace is measured in this
	// loop, so the loop has to run inside it.
	return hold{after: r.opts.PollInterval}
}

// recovered clears the mid-run failure clock. The phase just succeeded at
// what it needed, so the next failure gets the full grace again.
func recovered(restore *v1.LogicalRestore) {
	restore.Status.FirstFailedAt = nil
}

// backupIndexKey is the index key of one backup: the kind and the key of the
// resource. A LogicalBackupElasticsearch and a LogicalBackupRDBMS of the same
// name can live in one namespace, so the kind belongs in the key.
func backupIndexKey(kind v1.LogicalBackupKind, namespace, name string) string {
	return string(kind) + "/" + refindex.NamespacedKey(namespace, name)
}

// SetupWithManager applies the options, registers the controller, the two
// indexes, and the watches: the restores, the Jobs they own, the target
// clusters, and both backup kinds. A suspend flip and a backup that reaches
// Completed both wake a waiting restore without a timer.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	resolved, err := r.opts.withDefaults()
	if err != nil {
		return err
	}
	r.opts = resolved

	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder("logicalrestore")
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1.LogicalRestore{},
		clusterKeyIndex,
		func(obj client.Object) []string {
			restore := obj.(*v1.LogicalRestore)

			return []string{refindex.NamespacedKey(restore.Namespace, restore.Spec.TargetClusterRef.Name)}
		},
	); err != nil {
		return fmt.Errorf("indexing LogicalRestore by targetClusterRef: %w", err)
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1.LogicalRestore{},
		backupKeyIndex,
		func(obj client.Object) []string {
			restore := obj.(*v1.LogicalRestore)

			return []string{backupIndexKey(
				restore.Spec.BackupRef.Kind, restore.Namespace, restore.Spec.BackupRef.Name,
			)}
		},
	); err != nil {
		return fmt.Errorf("indexing LogicalRestore by backupRef: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.LogicalRestore{}).
		Owns(&batchv1.Job{}).
		Watches(
			&v1.CamundaCluster{},
			refindex.Enqueue(
				mgr.GetClient(), &v1.LogicalRestoreList{}, clusterKeyIndex, refindex.ObjectNamespacedName,
			),
		).
		Watches(
			&v1.LogicalBackupElasticsearch{},
			refindex.Enqueue(
				mgr.GetClient(),
				&v1.LogicalRestoreList{},
				backupKeyIndex,
				func(o client.Object) string {
					return backupIndexKey(v1.LogicalBackupKindElasticsearch, o.GetNamespace(), o.GetName())
				},
			),
		).
		Watches(
			&v1.LogicalBackupRDBMS{},
			refindex.Enqueue(
				mgr.GetClient(),
				&v1.LogicalRestoreList{},
				backupKeyIndex,
				func(o client.Object) string {
					return backupIndexKey(v1.LogicalBackupKindRDBMS, o.GetNamespace(), o.GetName())
				},
			),
		).
		Named("logicalrestore").
		Complete(r)
}
