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

// Package backupschedule reconciles a BackupSchedule. At each cron trigger it
// creates the logical backup kind that matches the storage type of the
// referenced cluster, and it prunes the terminal backups it created beyond
// spec.retained. It owns nothing it creates: the backups carry no owner
// reference, so they outlive the schedule on purpose, and their own finalizer
// removes the stored artifacts when the pruning deletes them.
//
// A trigger fires at most one backup, and a skipped trigger is consumed, not
// retried: the next backup runs at the next trigger, which is what a human
// reads from the cron expression. The skips are events; the Ready condition
// only says whether the references resolve.
package backupschedule

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

const (
	eventReasonTriggerSkipped  = "TriggerSkipped"
	eventReasonBackupCreated   = "BackupCreated"
	eventReasonBackupPruned    = "BackupPruned"
	eventReasonRetentionWindow = "RetentionWindowExceeded"
	eventReasonNameTaken       = "BackupNameTaken"
	eventActionSchedule        = "Schedule"
	eventActionPrune           = "Prune"
)

// Options configures the reconciler at construction. Every field has a
// default that fits production, and the tests set what they need to observe.
type Options struct {
	// Now returns the current time. Nil means time.Now. The tests inject a
	// clock they control, so a trigger fires without waiting for one.
	Now func() time.Time
}

// withDefaults fills the zero fields of o with the production defaults.
func (o Options) withDefaults() Options {
	if o.Now == nil {
		o.Now = time.Now
	}

	return o
}

// BackupScheduleReconciler reconciles a BackupSchedule.
type BackupScheduleReconciler struct {
	client.Client
	// APIReader reads without the cache. A trigger decides on live state: a
	// stale suspend flag or a stale backup list must not start or skip a
	// backup, and the pruning deletes data, so it counts live objects.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the trigger and pruning events.
	// SetupWithManager sets it from the manager.
	EventRecorder events.EventRecorder

	// opts is the construction-time configuration, defaults applied.
	opts Options
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=backupschedules,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=backupschedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackupelasticsearches;logicalbackuprdbmses,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters;secondarystorageconfigs;camundaclusterpresets,verbs=get
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile consumes the due trigger of the schedule, if one is due, and
// prunes the terminal backups of the schedule beyond spec.retained. It then
// requeues at the next trigger, so a schedule needs no watch to fire; the
// watches on the backup kinds only make the pruning react to a phase change
// without waiting for that requeue.
func (r *BackupScheduleReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (_ ctrl.Result, err error) {
	var schedule v1.BackupSchedule
	if err := r.APIReader.Get(ctx, req.NamespacedName, &schedule); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// The schedule holds no finalizer: its backups deliberately outlive it,
	// and nothing else needs cleanup.
	if !schedule.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	rec := component.ReconcileContext{
		Client:        r.Client,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
		APIReader:     r.APIReader,
		Owner:         &schedule,
	}
	defer func() {
		if flushErr := component.FlushStatus(ctx, rec, nil); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	if failure := nameProblem(&schedule); failure != nil {
		// Every backup carries the schedule and the cluster in a label, and
		// the pruning and the watches select backups by those labels. A name
		// that cannot be a label value fails the creation at admission and
		// the selector of the pruning alike, so nothing below can run and
		// the condition is all the schedule can offer. Only a new object
		// under a shorter name heals it.
		conditions.Stage(&schedule, conditions.Failed(&schedule, failure))

		return ctrl.Result{}, nil
	}

	now := r.opts.Now()

	sched, parseErr := parseCron(schedule.Spec.Schedule, now)
	if parseErr != nil {
		// The schema pattern admits a five-field shape with out-of-range
		// values, for example minute 99, and dates that never come, for
		// example the 31st of February. Only a spec change heals either, and
		// the For watch delivers it. The pruning does not read the cron, so
		// spec.retained stays enforced while the schedule cannot fire.
		conditions.Stage(&schedule, conditions.Ready(
			metav1.ConditionFalse,
			v1.ReasonInvalidReference,
			fmt.Sprintf("spec.schedule %q is not a valid cron expression: %v", schedule.Spec.Schedule, parseErr),
			schedule.Generation,
		))

		return ctrl.Result{}, r.prune(ctx, &schedule)
	}

	res, failure, err := r.resolve(ctx, &schedule)
	if err != nil {
		return ctrl.Result{}, err
	}
	if failure != nil {
		conditions.Stage(&schedule, conditions.Failed(&schedule, failure))
	} else {
		conditions.Stage(&schedule, conditions.Ready(
			metav1.ConditionTrue,
			v1.ReasonHealthy,
			"the schedule can run its backups",
			schedule.Generation,
		))
	}

	due, next := dueTrigger(sched, schedule.Status.LastScheduleTime, schedule.CreationTimestamp.Time, now)
	if due != nil {
		if err := r.trigger(ctx, &schedule, res, failure, sched, *due); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := r.prune(ctx, &schedule); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: next.Sub(now)}, nil
}

// nameProblem reports why the schedule cannot label the backups it creates,
// or nil when it can. A Kubernetes object name may be a 253-character DNS
// subdomain, while a label value is bounded at 63 characters, so a legal
// schedule name or cluster name can still be unusable as the label that ties
// a backup to its schedule.
func nameProblem(schedule *v1.BackupSchedule) *conditions.PreCheckFailure {
	for _, named := range []struct{ field, value string }{
		{"metadata.name", schedule.Name},
		{"spec.clusterRef.name", schedule.Spec.ClusterRef.Name},
	} {
		problems := validation.IsValidLabelValue(named.value)
		if len(problems) == 0 {
			continue
		}

		return logicalbackup.InvalidReference(
			"%s %q cannot be a label value, and every backup this schedule creates carries it in one: %s",
			named.field, named.value, strings.Join(problems, "; "),
		)
	}

	return nil
}

// resolved is what one look at the references answers: the cluster to back up
// and the storage type that selects the backup kind. Deeper checks stay with
// the backup kinds, whose pre-checks re-resolve everything they need; a
// schedule that resolves this much reports Healthy.
type resolved struct {
	cluster     *v1.CamundaCluster
	storageType v1.SecondaryStorageType
}

// resolve reads the cluster and its storage contract, live. A reference that
// does not resolve is a pre-check failure with the Ready reason
// InvalidReference; any other error is a transient API failure.
func (r *BackupScheduleReconciler) resolve(
	ctx context.Context,
	schedule *v1.BackupSchedule,
) (*resolved, *conditions.PreCheckFailure, error) {
	namespace := schedule.Namespace
	ref := schedule.Spec.ClusterRef.Name

	var cluster v1.CamundaCluster
	if err := r.APIReader.Get(ctx, types.NamespacedName{Name: ref, Namespace: namespace}, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference(
				"CamundaCluster %s/%s does not exist", namespace, ref,
			), nil
		}

		return nil, nil, fmt.Errorf("reading CamundaCluster %s/%s: %w", namespace, ref, err)
	}

	if cluster.Spec.BackupStorageRef == "" {
		return nil, logicalbackup.InvalidReference(
			"CamundaCluster %s/%s has no spec.backupStorageRef, so a backup has nowhere to write",
			namespace, ref,
		), nil
	}

	if cluster.Spec.StorageRef == "" {
		return nil, logicalbackup.InvalidReference(
			"CamundaCluster %s/%s has no spec.storageRef", namespace, ref,
		), nil
	}

	storageName := types.NamespacedName{Name: cluster.Spec.StorageRef, Namespace: namespace}
	var storage v1.SecondaryStorageConfig
	if err := r.APIReader.Get(ctx, storageName, &storage); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, logicalbackup.InvalidReference(
				"SecondaryStorageConfig %s does not exist", storageName,
			), nil
		}

		return nil, nil, fmt.Errorf("reading SecondaryStorageConfig %s: %w", storageName, err)
	}

	return &resolved{cluster: &cluster, storageType: storage.Spec.Type}, nil, nil
}

// trigger consumes the due trigger: it creates the backup, or it skips with
// an event when the cluster is suspended or a backup of the schedule has not
// finished. Every path below records the trigger as consumed, except a
// transient error, which leaves it due so the retry takes it again. The
// backup name repeats the trigger time, so a retry after a crash lands on
// AlreadyExists instead of a second backup.
func (r *BackupScheduleReconciler) trigger(
	ctx context.Context,
	schedule *v1.BackupSchedule,
	res *resolved,
	failure *conditions.PreCheckFailure,
	sched cron.Schedule,
	due time.Time,
) error {
	consumed := metav1.NewTime(due)

	// A reference that does not resolve is already on the Ready condition;
	// the trigger is skipped without a second signal.
	if failure != nil {
		schedule.Status.LastScheduleTime = &consumed

		return nil
	}

	if res.cluster.Spec.Suspend {
		schedule.Status.LastScheduleTime = &consumed
		r.EventRecorder.Eventf(
			schedule,
			nil,
			corev1.EventTypeNormal,
			eventReasonTriggerSkipped,
			eventActionSchedule,
			"Skipped the trigger at %s: CamundaCluster %q is suspended",
			due.Format(time.RFC3339),
			res.cluster.Name,
		)

		return nil
	}

	backup, kind := newBackup(schedule, res.storageType, due)

	items, err := r.scheduledBackups(ctx, schedule)
	if err != nil {
		return err
	}
	if running := nonTerminal(items, kind, backup.GetName()); running != "" {
		schedule.Status.LastScheduleTime = &consumed
		r.EventRecorder.Eventf(
			schedule,
			nil,
			corev1.EventTypeNormal,
			eventReasonTriggerSkipped,
			eventActionSchedule,
			"Skipped the trigger at %s: backup %q has not finished",
			due.Format(time.RFC3339),
			running,
		)

		return nil
	}

	if res.storageType == v1.SecondaryStorageTypeRDBMS {
		r.warnRetentionWindow(ctx, schedule, res.cluster, sched, due)
	}

	switch err := r.Create(ctx, backup); {
	case apierrors.IsAlreadyExists(err):
		// A reconcile that crashed between the create and the status flush
		// already created the backup of this trigger and recorded its event.
		// The retry only records what it found, once it proves the object is
		// that backup. A stranger under the same name backs up another
		// cluster, and reporting it as this schedule's backup would hide
		// that this trigger produced none.
		ours, err := r.backupBelongsTo(ctx, schedule, backup)
		if err != nil {
			return err
		}
		if !ours {
			schedule.Status.LastScheduleTime = &consumed
			r.EventRecorder.Eventf(
				schedule,
				nil,
				corev1.EventTypeWarning,
				eventReasonNameTaken,
				eventActionSchedule,
				"Skipped the trigger at %s: %s %q already exists and belongs to something else",
				due.Format(time.RFC3339),
				kind,
				backup.GetName(),
			)

			return nil
		}
	case err != nil:
		return fmt.Errorf("creating %s %q: %w", kind, backup.GetName(), err)
	default:
		r.EventRecorder.Eventf(
			schedule,
			nil,
			corev1.EventTypeNormal,
			eventReasonBackupCreated,
			eventActionSchedule,
			"Created %s %q",
			kind,
			backup.GetName(),
		)
	}

	schedule.Status.LastScheduleTime = &consumed
	schedule.Status.LastBackupName = backup.GetName()

	return nil
}

// backupBelongsTo reads the object that holds the name of want and reports
// whether it is the backup this schedule creates for this trigger: the same
// kind, carrying this schedule's label, and backing up the same cluster. It
// reads live, because the decision admits or refuses a trigger.
//
// An object that vanished between the failed create and this read is not
// ours to adopt either. The next reconcile creates the backup if the trigger
// is still due.
func (r *BackupScheduleReconciler) backupBelongsTo(
	ctx context.Context,
	schedule *v1.BackupSchedule,
	want client.Object,
) (bool, error) {
	found, ok := want.DeepCopyObject().(client.Object)
	if !ok {
		return false, fmt.Errorf("copying %T for the identity read", want)
	}
	if err := r.APIReader.Get(ctx, client.ObjectKeyFromObject(want), found); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("reading the existing %q: %w", want.GetName(), err)
	}

	if found.GetLabels()[labels.BackupScheduleKey] != schedule.Name {
		return false, nil
	}

	switch backup := found.(type) {
	case *v1.LogicalBackupElasticsearch:
		return backup.Spec.ClusterRef == schedule.Spec.ClusterRef, nil
	case *v1.LogicalBackupRDBMS:
		return backup.Spec.ClusterRef == schedule.Spec.ClusterRef, nil
	}

	return false, nil
}

// newBackup builds the backup of one trigger: the kind that matches the
// storage type, named <schedule>-<unix-timestamp> of the trigger, labeled
// with the cluster and the schedule, and without an owner reference, so
// deleting the schedule never deletes it.
func newBackup(
	schedule *v1.BackupSchedule,
	storageType v1.SecondaryStorageType,
	due time.Time,
) (client.Object, string) {
	objectMeta := metav1.ObjectMeta{
		Name:      schedule.Name + "-" + strconv.FormatInt(due.Unix(), 10),
		Namespace: schedule.Namespace,
		Labels: map[string]string{
			labels.ClusterKey:        schedule.Spec.ClusterRef.Name,
			labels.BackupScheduleKey: schedule.Name,
		},
	}

	if storageType == v1.SecondaryStorageTypeElasticsearch {
		backup := &v1.LogicalBackupElasticsearch{
			ObjectMeta: objectMeta,
			Spec:       v1.LogicalBackupElasticsearchSpec{ClusterRef: schedule.Spec.ClusterRef},
		}

		return backup, backup.GetKind()
	}

	backup := &v1.LogicalBackupRDBMS{
		ObjectMeta: objectMeta,
		Spec:       v1.LogicalBackupRDBMSSpec{ClusterRef: schedule.Spec.ClusterRef},
	}

	return backup, backup.GetKind()
}

// warnRetentionWindow warns, best effort, when the retained completed dumps
// of the schedule outlive the primary-storage retention window of the
// cluster: those dumps can no longer be restored, because Zeebe deleted the
// primary-storage backups they pair with. The policy comes from the merged
// preset the way the cluster renders it. A preset that does not resolve
// warns about nothing; the cluster controller already reports it.
func (r *BackupScheduleReconciler) warnRetentionWindow(
	ctx context.Context,
	schedule *v1.BackupSchedule,
	cluster *v1.CamundaCluster,
	sched cron.Schedule,
	due time.Time,
) {
	var preset *v1.CamundaClusterPresetSpec
	if cluster.Spec.PresetRef != "" {
		var obj v1.CamundaClusterPreset
		if err := r.APIReader.Get(ctx, types.NamespacedName{Name: cluster.Spec.PresetRef}, &obj); err != nil {
			return
		}
		preset = &obj.Spec
	}

	policy := components.NewEffective(components.MergePreset(cluster.Spec, preset)).PrimaryStorageBackup()
	completed, _ := retainedBounds(schedule.Spec)
	message := retentionWindowWarning(completed, triggerInterval(sched, due), policy)
	if message == "" {
		return
	}

	r.EventRecorder.Eventf(
		schedule,
		nil,
		corev1.EventTypeWarning,
		eventReasonRetentionWindow,
		eventActionSchedule,
		"%s",
		message,
	)
}

// SetupWithManager applies opts and registers the controller: the schedules,
// and both backup kinds so a phase change prunes without waiting for the
// requeue at the next trigger.
func (r *BackupScheduleReconciler) SetupWithManager(mgr ctrl.Manager, opts Options) error {
	r.opts = opts.withDefaults()
	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder("backupschedule")
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.BackupSchedule{}).
		Watches(
			&v1.LogicalBackupElasticsearch{},
			enqueueSchedule(),
			builder.WithPredicates(phaseChanged()),
		).
		Watches(
			&v1.LogicalBackupRDBMS{},
			enqueueSchedule(),
			builder.WithPredicates(phaseChanged()),
		).
		Named("backupschedule").
		Complete(r)
}

// enqueueSchedule maps a backup event to the schedule its label names. A
// backup without the label is manual and wakes nothing.
func enqueueSchedule() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(_ context.Context, obj client.Object) []ctrl.Request {
		name := obj.GetLabels()[labels.BackupScheduleKey]
		if name == "" {
			return nil
		}

		return []ctrl.Request{{
			NamespacedName: types.NamespacedName{Namespace: obj.GetNamespace(), Name: name},
		}}
	})
}

// phaseChanged passes the backup events the schedule reacts to: a phase
// change, which moves the overlap check and the pruning. The schedule
// created the backup itself and the requeue covers everything else, so
// creations and deletions wake nothing.
func phaseChanged() predicate.Predicate {
	return predicate.Funcs{
		CreateFunc:  func(event.CreateEvent) bool { return false },
		DeleteFunc:  func(event.DeleteEvent) bool { return false },
		GenericFunc: func(event.GenericEvent) bool { return false },
		UpdateFunc: func(e event.UpdateEvent) bool {
			return backupPhase(e.ObjectOld) != backupPhase(e.ObjectNew)
		},
	}
}

// backupPhase reads the phase of either backup kind.
func backupPhase(obj client.Object) v1.LogicalBackupPhase {
	switch backup := obj.(type) {
	case *v1.LogicalBackupElasticsearch:
		return backup.Status.Phase
	case *v1.LogicalBackupRDBMS:
		return backup.Status.Phase
	}

	return ""
}
