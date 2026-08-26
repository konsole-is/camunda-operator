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

// Package logicalbackupelasticsearch reconciles LogicalBackupElasticsearch.
// One resource is one hot backup of an Elasticsearch-backed CamundaCluster.
// The backup is a coordinated set under one backup ID, tracked to a terminal
// phase. The controller reaches the cluster through the management binding
// that the cluster publishes. It reaches Elasticsearch through the
// SecondaryStorageConfig. It never reads the internals of another controller.
//
// Admission and runtime are separate. The full pre-checks run only before the
// backup starts. After the start, a missing dependency routes through the
// state machine to resume-exporting and then to a terminal phase. A cluster
// that is not addressable for a moment parks the procedure in place. A running
// backup never returns to Pending and never starts again. Its identity, the
// backup ID, the pinned repository, and the pinned storage destination, is
// written before the first side effect and only read afterwards.
package logicalbackupelasticsearch

import (
	"context"
	"errors"
	"fmt"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"github.com/sourcehawk/operator-component-framework/pkg/component"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/observability"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
)

// controllerName is the name the controller registers with controller-runtime.
// It labels its events and every metrics series it records.
const controllerName = "logicalbackupelasticsearch"

const (
	// clusterRefField indexes backups by the effective namespace/name of
	// their clusterRef, so a cluster event enqueues its backups.
	clusterRefField = "logicalbackupelasticsearch.spec.clusterRef"

	// defaultResumeDeadline bounds the accumulated time of active resume
	// attempts. After it, the controller gives the cluster to a human.
	defaultResumeDeadline = 30 * time.Minute
	// defaultPollInterval paces the polling of the running procedure. It
	// also paces the waits on pre-checks that resolve on their own.
	defaultPollInterval = 5 * time.Second
	// retryInterval paces admission failures that no watch resolves.
	retryInterval = 30 * time.Second
	// defaultRuntimeRegistrationGrace bounds how long an absent runtime
	// backup is polled after its request was recorded. The cluster registers
	// the backup asynchronously and can report it absent for a moment.
	// Within the grace the step polls. After it, the backup is missing.
	defaultRuntimeRegistrationGrace = 2 * time.Minute
	// unreachableBound bounds the retry of an unreachable endpoint at the
	// working steps, which run with exporting paused. The endpoint is the
	// management API or Elasticsearch, whichever the step calls. After the
	// bound, the step fails and the procedure resumes exporting. A
	// black-holed backup route beside a healthy resume route must never
	// leave the cluster paused for good.
	unreachableBound = 10 * time.Minute
	// concurrentReconciles bounds the parallel reconciles. Every step is a
	// synchronous HTTP call. One black-holed endpoint must not block the
	// polling and the finalizers of every other backup.
	concurrentReconciles = 4
)

const (
	eventReasonStarted      = "BackupStarted"
	eventReasonCompleted    = "BackupCompleted"
	eventReasonStepFailed   = "BackupStepFailed"
	eventReasonResumeFailed = "ResumeFailed"
	eventReasonReleased     = "ArtifactsUnreachable"
	eventReasonDeleteHeld   = "DeletionWaiting"
	eventActionBackup       = "Backup"
	eventActionFinalize     = "Finalize"
)

// Options tunes a Reconciler. The zero value is the production configuration.
// Tests set the fields to fit the procedure inside a test timeout.
type Options struct {
	// ResumeDeadline bounds the accumulated time of active resume attempts.
	// After it, the phase goes Failed with reason ResumeFailed. Zero means
	// 30 minutes.
	ResumeDeadline time.Duration
	// PollInterval paces the polling of a running backup. Zero means five
	// seconds.
	PollInterval time.Duration
	// UnreachableBound bounds the retry of an unreachable management API or
	// Elasticsearch endpoint at the working steps, while exporting is
	// paused. Zero means ten minutes.
	UnreachableBound time.Duration
	// RuntimeRegistrationGrace bounds how long an absent runtime or history
	// backup is polled after its request was accepted. Zero means two
	// minutes.
	RuntimeRegistrationGrace time.Duration
}

// Reconciler drives a LogicalBackupElasticsearch to a terminal phase.
type Reconciler struct {
	client.Client
	// APIReader reads referenced resources without the cache. It also reads
	// the backup itself. A stale status re-runs a side effect. A stale
	// suspend flag or storage reference admits a backup that must wait.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the lifecycle events of the backup.
	// SetupWithManager sets it from the manager when it is nil.
	EventRecorder events.EventRecorder
	// Metrics records the condition gauge and the apply counters of the
	// framework. SetupWithManager sets it when it is nil.
	Metrics component.MetricsRecorder

	options Options
}

// New returns a Reconciler with the given options.
func New(c client.Client, reader client.Reader, scheme *runtime.Scheme, options Options) *Reconciler {
	return &Reconciler{Client: c, APIReader: reader, Scheme: scheme, options: options}
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackupelasticsearches,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackupelasticsearches/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackupelasticsearches/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=secondarystorageconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=objectstorageconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
// The cluster claim reads the resource of whichever kind holds the Lease,
// to decide whether that holder still needs the cluster.
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalrestoreelasticsearches;logicalrestorerdbmses;pointintimerestores,verbs=get

// Reconcile advances the backup by at most one step. The recorded step is
// persisted before the next side effect, so a crash re-enters where it left
// off.
func (r *Reconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	// The backup is its own state machine. A stale cached read of its status
	// re-enters a step whose side effect already ran, or allocates the backup
	// ID again and orphans artifacts. Its own object is always read live.
	var backup v1.LogicalBackupElasticsearch
	if err := r.APIReader.Get(ctx, req.NamespacedName, &backup); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !backup.DeletionTimestamp.IsZero() {
		return r.finalize(ctx, &backup)
	}

	// The finalizer must exist before the first external side effect. A
	// deletion between the side effect and the next write leaks the
	// artifacts otherwise.
	if controllerutil.AddFinalizer(&backup, logicalbackup.Finalizer) {
		if err := r.Update(ctx, &backup); err != nil {
			// A deletion that races this write is fine. The deletion path
			// owns the object from here.
			if apierrors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}
			return ctrl.Result{}, fmt.Errorf("adding the finalizer: %w", err)
		}
	}

	defer func() {
		if flushErr := r.persistStatus(ctx, &backup); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	// A write conflict at the terminal transition can restore an older Ready
	// from the server. The terminal condition is staged again from the
	// recorded outcome. SetStatusCondition makes the repeat a no-op.
	//
	// The claim on the cluster goes back here, after the terminal status
	// was flushed and never before it. The claim follows the pause, not the
	// phase. A backup that ended as ResumeFailed left exporting paused. Its
	// claim is what keeps a sibling from backing up the paused cluster, so
	// it keeps the Lease and holds. Deleting it resumes exporting in the
	// finalizer, and only that release lets a sibling start.
	if backup.Terminal() {
		conditions.Stage(&backup, terminalReady(&backup))
		if backup.Status.TerminalReason == v1.ReasonResumeFailed {
			return ctrl.Result{}, nil
		}
		if err := r.releaseClaim(ctx, &backup); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	if backup.Status.BackupID == 0 {
		return r.admit(ctx, &backup)
	}

	return r.run(ctx, &backup)
}

// persistStatus writes the status of backup through the ocf flush: the one
// status write of a reconcile. Reconcile defers it. The finalizer runs
// before that defer is installed. It calls persistStatus when it must make
// a status fact durable before it acts on it.
func (r *Reconciler) persistStatus(ctx context.Context, backup *v1.LogicalBackupElasticsearch) error {
	return component.FlushStatus(
		ctx, component.ReconcileContext{
			Client:        r.Client,
			Scheme:        r.Scheme,
			EventRecorder: r.EventRecorder,
			Metrics:       r.Metrics,
			APIReader:     r.APIReader,
			Owner:         backup,
		}, nil,
	)
}

// terminalReady rebuilds the Ready condition of a terminal backup from the
// recorded outcome.
func terminalReady(backup *v1.LogicalBackupElasticsearch) metav1.Condition {
	if backup.Status.Phase == v1.LogicalBackupCompleted {
		return conditions.Ready(
			metav1.ConditionTrue,
			v1.ReasonCompleted,
			"The backup finished and is restorable",
			backup.Generation,
		)
	}

	reason := backup.Status.TerminalReason
	if reason == "" {
		reason = v1.ReasonFailed
	}

	message := backup.Status.FailureMessage
	if resume := backup.Status.ResumeFailureMessage; resume != "" && resume != message {
		message = message + ". " + resume
	}

	return conditions.Ready(metav1.ConditionFalse, reason, message, backup.Generation)
}

func (r *Reconciler) poll() time.Duration {
	if r.options.PollInterval > 0 {
		return r.options.PollInterval
	}
	return defaultPollInterval
}

func (r *Reconciler) resumeDeadline() time.Duration {
	if r.options.ResumeDeadline > 0 {
		return r.options.ResumeDeadline
	}
	return defaultResumeDeadline
}

func (r *Reconciler) unreachableBound() time.Duration {
	if r.options.UnreachableBound > 0 {
		return r.options.UnreachableBound
	}
	return unreachableBound
}

func (r *Reconciler) runtimeRegistrationGrace() time.Duration {
	if r.options.RuntimeRegistrationGrace > 0 {
		return r.options.RuntimeRegistrationGrace
	}
	return defaultRuntimeRegistrationGrace
}

// SetupWithManager registers the controller, the clusterRef index, and the
// cluster watch. The watch wakes waiting backups when the binding appears.
// SetupWithManager sets EventRecorder and Metrics when they are nil.
func (r *Reconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder(controllerName)
	}
	if r.Metrics == nil {
		r.Metrics = observability.Recorder(controllerName)
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(),
		&v1.LogicalBackupElasticsearch{},
		clusterRefField,
		func(o client.Object) []string {
			backup := o.(*v1.LogicalBackupElasticsearch)
			return []string{refindex.NamespacedKey(backup.Namespace, backup.Spec.ClusterRef.Name)}
		},
	); err != nil {
		return fmt.Errorf("indexing LogicalBackupElasticsearch by clusterRef: %w", err)
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.LogicalBackupElasticsearch{}).
		Named(controllerName).
		WithOptions(controller.Options{MaxConcurrentReconciles: concurrentReconciles}).
		Watches(
			&v1.CamundaCluster{},
			refindex.Enqueue(
				mgr.GetClient(),
				&v1.LogicalBackupElasticsearchList{},
				clusterRefField,
				refindex.ObjectNamespacedName,
			),
		).
		Complete(r)
}
