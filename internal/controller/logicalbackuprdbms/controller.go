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

// Package logicalbackuprdbms reconciles a LogicalBackupRDBMS: it runs the Job
// that dumps the logical database of a relational cluster to the backup
// bucket, then requests one Zeebe backup — Camunda's own backup of its
// primary storage — so the pair is a complete restore point.
//
// Admission and runtime are split. The full pre-checks — references, storage
// type, serialization against other backups — run only until the backup
// starts. A running backup re-resolves only what its current step needs, so
// a reference that breaks mid-run cannot park it: it either finishes on what
// it already holds, or terminalizes after a bounded grace.
//
// The files follow the steps: admit.go admits and starts a backup, dump.go
// runs the dump Job, zeebebackup.go requests and polls the Zeebe backup,
// serialization.go gates the backups of one cluster, finalizer.go cleans up.
// This file holds what every step shares: Reconcile, the terminal phases,
// and the wiring.
package logicalbackuprdbms

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
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
)

const (
	// fieldManager owns the Job of a backup in Server-Side Apply.
	fieldManager = "camunda-operator/backup"
	// clusterKeyIndex indexes backups by the namespace/name of the cluster
	// they reference, for the serialization pre-check and the cluster watch.
	clusterKeyIndex = "logicalbackuprdbms.spec.clusterRef"
	// defaultRetryInterval paces the requeues of waiting states: an
	// unreachable management API, a running sibling backup, or a reference
	// that nothing watches.
	defaultRetryInterval = 30 * time.Second
	// defaultMidRunGrace bounds how long a running backup waits on a
	// dependency that stopped resolving before it fails. A broken reference
	// parks one backup for this long at most, never forever — a parked
	// non-terminal backup blocks every later backup of the cluster.
	defaultMidRunGrace = 10 * time.Minute
	// defaultRegistrationGrace bounds how long the Zeebe backup poll
	// tolerates a backup the cluster does not report yet: the partitions
	// register their parts asynchronously after the 202.
	defaultRegistrationGrace = 2 * time.Minute

	eventReasonStarted   = "BackupStarted"
	eventReasonCompleted = "BackupCompleted"
	eventReasonFailed    = "BackupFailed"
	eventReasonCleanup   = "ArtifactCleanupFailed"
	eventActionBackup    = "Backup"
	eventActionFinalize  = "Finalize"
)

// ArtifactBucket is the slice of the backup bucket that the finalizer needs.
type ArtifactBucket interface {
	Delete(ctx context.Context, key string) error
	Close()
}

// hold is the domain result of one reconcile step: how long to wait before
// the next look, or nothing when watches carry the wake-up. Only Reconcile
// turns it into a ctrl.Result.
type hold struct {
	after time.Duration
}

var (
	// settle waits on watches alone.
	settle = hold{}
	// shortly re-enters to persist staged status before acting on it.
	shortly = hold{after: time.Second}
)

// Options configures the reconciler at construction. Only CLIImage is
// required; every other field has a default that fits production, and the
// tests set what they need to observe.
type Options struct {
	// CLIImage is the camunda-operator-cli image that the upload container
	// of the dump Job runs. The manager passes --camunda-operator-cli-image.
	CLIImage string
	// OpenBucket opens the backup bucket for the finalizer. Nil means
	// pkg/objectstore; tests point it at a local fake.
	OpenBucket func(
		ctx context.Context, cfg *v1.ObjectStorageConfig, creds *objectstore.Credentials,
	) (ArtifactBucket, error)
	// SiblingInProgress reports a started, non-terminal backup of the other
	// kind for the same cluster; pending siblings do not block. The manager
	// wires the LogicalBackupElasticsearch controller in here; nil means no
	// other kind is checked.
	SiblingInProgress logicalbackup.SiblingInProgress
	// RetryInterval overrides defaultRetryInterval. Zero means the default.
	RetryInterval time.Duration
	// MidRunGrace overrides defaultMidRunGrace. Zero means the default.
	MidRunGrace time.Duration
	// RegistrationGrace overrides defaultRegistrationGrace. Zero means the
	// default.
	RegistrationGrace time.Duration
}

// withDefaults fills the zero fields of o with the production defaults and
// rejects an empty CLIImage: the Job builder could only guess an image.
func (o Options) withDefaults() (Options, error) {
	if o.CLIImage == "" {
		return o, errors.New("the camunda-operator-cli image is required")
	}
	if o.OpenBucket == nil {
		o.OpenBucket = func(
			ctx context.Context, cfg *v1.ObjectStorageConfig, creds *objectstore.Credentials,
		) (ArtifactBucket, error) {
			return objectstore.Open(ctx, cfg, creds)
		}
	}
	if o.RetryInterval <= 0 {
		o.RetryInterval = defaultRetryInterval
	}
	if o.MidRunGrace <= 0 {
		o.MidRunGrace = defaultMidRunGrace
	}
	if o.RegistrationGrace <= 0 {
		o.RegistrationGrace = defaultRegistrationGrace
	}

	return o, nil
}

// LogicalBackupRDBMSReconciler reconciles a LogicalBackupRDBMS.
type LogicalBackupRDBMSReconciler struct {
	client.Client
	// APIReader reads without the cache. Admission decides on live state:
	// a stale suspend flag or a stale sibling list must not start a backup.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the backup lifecycle events. SetupWithManager
	// sets it from the manager.
	EventRecorder events.EventRecorder

	// opts is the construction-time configuration, defaults applied.
	opts Options
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackuprdbmses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackuprdbmses/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackuprdbmses/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters;secondarystorageconfigs;databaseconfigs;databaseserverconfigs;objectstorageconfigs;camundaclusterpresets,verbs=get;list;watch
// +kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=pods,verbs=list
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile drives one backup to a terminal phase: admission, the dump Job,
// then the Zeebe backup of the cluster. It is the only function that builds
// a ctrl.Result.
//
// The backup is read live, not from the cache: the step switch below is the
// state machine's authority, and a requeue that arrives before the informer
// caught up with the last flush must not re-enter a step that already ran.
// Each branch is re-entrant on its own — the identity is persisted before
// the Job exists, the Zeebe backup id before it is polled — so this is the
// second layer, not the only one.
func (r *LogicalBackupRDBMSReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (_ ctrl.Result, err error) {
	var backup v1.LogicalBackupRDBMS
	if err := r.APIReader.Get(ctx, req.NamespacedName, &backup); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !backup.DeletionTimestamp.IsZero() {
		wait, err := r.finalize(ctx, &backup)

		return ctrl.Result{RequeueAfter: wait.after}, err
	}

	if !controllerutil.ContainsFinalizer(&backup, logicalbackup.Finalizer) {
		controllerutil.AddFinalizer(&backup, logicalbackup.Finalizer)
		if err := r.Update(ctx, &backup); err != nil {
			return ctrl.Result{}, fmt.Errorf("adding the finalizer: %w", err)
		}
	}

	rec := component.ReconcileContext{
		Client:        r.Client,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
		APIReader:     r.APIReader,
		Owner:         &backup,
	}
	defer func() {
		if flushErr := component.FlushStatus(ctx, rec, nil); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	if backup.Terminal() {
		// A conflict on the terminal flush can restore a stale Ready from
		// the server; re-staging the terminal condition is idempotent and
		// heals it on the next look.
		r.stageTerminal(&backup)

		return ctrl.Result{}, nil
	}

	var wait hold
	switch {
	case backup.Status.BackupID == 0:
		wait, err = r.admit(ctx, &backup)
	case backup.Status.Step == v1.StepDumping:
		wait, err = r.dump(ctx, &backup)
	case backup.Status.Step == v1.StepZeebeBackup:
		wait, err = r.requestZeebeBackup(ctx, &backup)
	default:
		return ctrl.Result{}, fmt.Errorf("unknown step %q", backup.Status.Step)
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: wait.after}, nil
}

func (r *LogicalBackupRDBMSReconciler) complete(backup *v1.LogicalBackupRDBMS) {
	now := metav1.Now()
	backup.Status.Phase = v1.LogicalBackupCompleted
	backup.Status.CompletionTime = &now
	r.stageTerminal(backup)
	r.EventRecorder.Eventf(
		backup,
		nil,
		corev1.EventTypeNormal,
		eventReasonCompleted,
		eventActionBackup,
		"Backup %d completed",
		backup.Status.BackupID,
	)
}

// fail terminalizes the backup with message. The message often carries an
// external error — a management API body, a pod's waiting message, a Job
// reason — whose size the controller cannot know, so it is bounded before it
// reaches the free-form status field; the Ready condition it also feeds is
// bounded by conditions itself.
func (r *LogicalBackupRDBMSReconciler) fail(backup *v1.LogicalBackupRDBMS, message string) {
	now := metav1.Now()
	message = conditions.BoundMessage(message)
	backup.Status.Phase = v1.LogicalBackupFailed
	backup.Status.CompletionTime = &now
	backup.Status.FailureMessage = message
	r.stageTerminal(backup)
	r.EventRecorder.Eventf(
		backup,
		nil,
		corev1.EventTypeWarning,
		eventReasonFailed,
		eventActionBackup,
		"Backup %d failed: %s",
		backup.Status.BackupID,
		message,
	)
}

// stageTerminal stages the Ready condition of a terminal phase. It is
// idempotent, so a terminal backup re-stages it on every look and heals a
// conflict that restored a stale condition.
func (r *LogicalBackupRDBMSReconciler) stageTerminal(backup *v1.LogicalBackupRDBMS) {
	switch backup.Status.Phase {
	case v1.LogicalBackupCompleted:
		conditions.Stage(backup, conditions.Ready(
			metav1.ConditionTrue,
			v1.ReasonCompleted,
			"the backup finished and is restorable",
			backup.Generation,
		))
	case v1.LogicalBackupFailed:
		conditions.Stage(backup, conditions.Ready(
			metav1.ConditionFalse, v1.ReasonFailed, backup.Status.FailureMessage, backup.Generation,
		))
	}
}

func progressing(backup *v1.LogicalBackupRDBMS, message string) metav1.Condition {
	return conditions.Ready(metav1.ConditionFalse, v1.ReasonProgressing, message, backup.Generation)
}

// SetupWithManager applies opts, registers the controller, the cluster-key
// index, and the watches: the backups, the Jobs they own, and the referenced
// clusters.
func (r *LogicalBackupRDBMSReconciler) SetupWithManager(mgr ctrl.Manager, opts Options) error {
	resolved, err := opts.withDefaults()
	if err != nil {
		return err
	}
	r.opts = resolved
	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder("logicalbackuprdbms")
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1.LogicalBackupRDBMS{},
		clusterKeyIndex,
		func(obj client.Object) []string {
			return []string{clusterKey(obj.(*v1.LogicalBackupRDBMS))}
		},
	); err != nil {
		return fmt.Errorf("indexing backups by cluster: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.LogicalBackupRDBMS{}).
		Owns(&batchv1.Job{}).
		Watches(
			&v1.CamundaCluster{},
			r.enqueueForCluster(),
			builder.WithPredicates(clusterChanged()),
		).
		Named("logicalbackuprdbms").
		Complete(r)
}

// clusterChanged passes the cluster events a waiting backup cares about: a
// spec change (suspend, references) or a change of the published management
// binding. Bare status noise wakes nothing.
func clusterChanged() predicate.Predicate {
	changed := predicate.Funcs{
		UpdateFunc: func(e event.UpdateEvent) bool {
			older, okOld := e.ObjectOld.(*v1.CamundaCluster)
			newer, okNew := e.ObjectNew.(*v1.CamundaCluster)
			if !okOld || !okNew {
				return false
			}
			if older.Generation != newer.Generation {
				return true
			}

			return !managementEqual(older.Status.Management, newer.Status.Management)
		},
	}

	return changed
}

func managementEqual(a, b *v1.ManagementBinding) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}

	return *a == *b
}

// enqueueForCluster maps a cluster event to every non-terminal backup that
// references it, so a suspend flip or a published binding wakes the waiting
// ones.
func (r *LogicalBackupRDBMSReconciler) enqueueForCluster() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, obj client.Object) []ctrl.Request {
		var list v1.LogicalBackupRDBMSList
		if err := r.List(ctx, &list, client.MatchingFields{
			clusterKeyIndex: refindex.NamespacedKey(obj.GetNamespace(), obj.GetName()),
		}); err != nil {
			return nil
		}

		requests := make([]ctrl.Request, 0, len(list.Items))
		for i := range list.Items {
			if list.Items[i].Terminal() {
				continue
			}
			requests = append(requests, ctrl.Request{
				NamespacedName: client.ObjectKeyFromObject(&list.Items[i]),
			})
		}

		return requests
	})
}
