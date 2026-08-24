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

// Package databaseserver reconciles the DatabaseServer CR. The controller
// merges the preset into the spec, drives the external CloudNativePG operator
// through a rendered cluster, archives the server to an object storage bucket
// through the Barman Cloud plugin, and publishes the DatabaseServerConfig
// contract that a Database and a PointInTimeRestore consume.
package databaseserver

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
)

// defaultRetryInterval is how long the controller waits before it looks again
// at the superuser Secret. CloudNativePG owns that Secret, so no watch of this
// controller reports its creation, and the published contract must not name it
// before it exists.
const defaultRetryInterval = 30 * time.Second

// resolvedSpec is everything the pre-checks resolved for one reconcile: the
// preset-merged spec, the platform config whose image settings rename the
// PostgreSQL image, and the archive bucket. Resolving each of them once is
// what keeps the components from re-reading the same objects per method.
type resolvedSpec struct {
	merged   v1.DatabaseServerSpec
	platform *v1.CamundaPlatformConfigSpec
	archive  *components.ArchiveStorage
	// holdForSuspension says that the archive of a suspended server that
	// already reports Ready did not resolve. The reconcile then stops and
	// leaves the condition alone: see preCheck.
	holdForSuspension bool
}

// serverComponents are the components of one reconcile, in the order they
// reconcile in. They are named rather than indexed, so a reorder cannot
// silently hand one of them to the wrong caller.
type serverComponents struct {
	cluster    *component.Component
	archive    *component.Component
	contract   *component.Component
	monitoring *component.Component
}

// all returns the components in reconcile order. Ready aggregates every one of
// them, and FlushStatus owns every one of their condition types.
func (c serverComponents) all() []*component.Component {
	return []*component.Component{c.cluster, c.archive, c.contract, c.monitoring}
}

// DatabaseServerReconciler runs a PostgreSQL server through the external
// CloudNativePG operator. It renders a CloudNativePG cluster, archives it to
// the bucket that spec.archive names, and publishes a DatabaseServerConfig in
// the namespace of the CR.
type DatabaseServerReconciler struct {
	client.Client
	// APIReader reads without the cache. The bucket credentials Secret must be
	// read live.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the component lifecycle events. SetupWithManager
	// sets it from the manager.
	EventRecorder events.EventRecorder

	// RetryInterval overrides how long the controller waits on the superuser
	// Secret. Zero means defaultRetryInterval; tests shorten it.
	RetryInterval time.Duration

	// componentClient is the uncached client that the ocf components reconcile
	// through. SetupWithManager builds it. The cached client of the manager
	// must not be used here: the typed Gets of ocf start a cluster-wide Secret
	// informer, which breaks the metadata-only Secret posture of the operator.
	componentClient client.Client
	// restMapper resolves whether the cluster serves the CloudNativePG, Barman
	// Cloud, and PodMonitor kinds. SetupWithManager sets it from the manager.
	restMapper meta.RESTMapper
	// cnpgInstalled and barmanInstalled record whether the cluster served the
	// CloudNativePG cluster kind and the Barman Cloud ObjectStore kind when
	// SetupWithManager ran. The watches on those kinds are registered then or
	// never, so the answers are fixed for the life of the process.
	cnpgInstalled   bool
	barmanInstalled bool
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseservers,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseservers/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseservers/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverpresets,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=objectstorageconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=postgresql.cnpg.io,resources=clusters;scheduledbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=postgresql.cnpg.io,resources=backups,verbs=get;list;watch
// +kubebuilder:rbac:groups=barmancloud.cnpg.io,resources=objectstores,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=podmonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile converges a DatabaseServer. It resolves the preset, runs the
// pre-checks, reconciles the cluster, archive, contract, and monitoring
// components, and derives the CR-level Ready condition.
//
// Status is written once per reconcile. The components and conditions.Stage
// stage conditions on the in-memory server, and the deferred FlushStatus
// persists them together with the observed cluster, identifier, archive
// history, and volumes.
func (r *DatabaseServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	var server v1.DatabaseServer
	if err := r.Get(ctx, req.NamespacedName, &server); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	recCtx := component.ReconcileContext{
		Client:        r.componentClient,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
		APIReader:     r.APIReader,
		Owner:         &server,
	}
	// Declared before the deferred flush, so the closure sees every component
	// that the reconcile builds below and FlushStatus owns their conditions.
	var comps []*component.Component
	defer func() {
		if flushErr := component.FlushStatus(ctx, recCtx, comps); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	// The contract, the archive, and the monitoring all name the cluster, so
	// it is recorded before anything renders and never derived twice.
	if server.Status.Cluster == "" {
		server.Status.Cluster = server.Name
	}

	resolved, err := r.preCheck(ctx, &server)
	var failure *conditions.PreCheckFailure
	if errors.As(err, &failure) {
		conditions.Stage(&server, conditions.Failed(&server, failure))
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	if resolved.holdForSuspension {
		return ctrl.Result{}, nil
	}

	archiveStart, err := r.archiveStart(ctx, &server, resolved.merged)
	if err != nil {
		return ctrl.Result{}, err
	}
	clearReenabledArchiveCondition(&server, resolved.merged)

	built, systemIdentifier, err := r.buildComponents(&server, resolved, archiveStart)
	if err != nil {
		return ctrl.Result{}, err
	}
	comps = built.all()

	reconcileErr := reconcileComponents(ctx, recCtx, comps)

	if identifier, ok := systemIdentifier.Get(); ok && identifier != "" {
		server.Status.SystemIdentifier = identifier
	}
	reconcileArchiveHistory(&server, resolved.merged, built.archive, archiveStart, metav1.Now())

	volumes, err := r.dataVolumes(ctx, &server)
	if err != nil {
		return ctrl.Result{}, errors.Join(reconcileErr, err)
	}
	server.Status.Volumes = volumes

	conditions.Stage(&server, conditions.Aggregate(&server, comps...))

	if reconcileErr != nil {
		return ctrl.Result{}, reconcileErr
	}

	// Nothing reports that the superuser Secret appeared: CloudNativePG owns
	// it, and this controller watches only what it owns itself. Every other
	// condition of this CR is backed by a watch.
	if !meta.IsStatusConditionTrue(server.Status.Conditions, components.ConditionContract) {
		return ctrl.Result{RequeueAfter: r.retryInterval()}, nil
	}

	return ctrl.Result{}, nil
}

// preCheck resolves the preset, validates the merged spec, and resolves the
// archive bucket. A failed check returns a *conditions.PreCheckFailure that
// carries its Ready reason. An operator that started without the CloudNativePG
// CRDs reports CNPGNotInstalled, and a server that asks for an archive on a
// cluster without the plugin CRD reports BarmanPluginNotInstalled. A dangling
// presetRef, an incomplete merge, a version below the floor, and a bucket the
// plugin cannot address all report InvalidReference. Any other error is a
// transient API failure.
//
// A bucket that stops resolving under a suspended server that already reports
// Ready is neither: the result carries holdForSuspension, and the caller stops
// the reconcile so the server keeps the Suspended it honestly reached.
func (r *DatabaseServerReconciler) preCheck(
	ctx context.Context,
	server *v1.DatabaseServer,
) (resolvedSpec, error) {
	resolved := resolvedSpec{merged: server.Spec}

	if !r.cnpgInstalled {
		return resolved, &conditions.PreCheckFailure{
			Reason:  v1.ReasonCNPGNotInstalled,
			Message: "CloudNativePG is not installed on this cluster. Install it, then restart the operator",
		}
	}

	if server.Spec.PresetRef != "" {
		var preset v1.DatabaseServerPreset
		if err := r.APIReader.Get(ctx, types.NamespacedName{Name: server.Spec.PresetRef}, &preset); err != nil {
			if apierrors.IsNotFound(err) {
				return resolved, &conditions.PreCheckFailure{
					Reason:  v1.ReasonInvalidReference,
					Message: fmt.Sprintf("DatabaseServerPreset %q not found", server.Spec.PresetRef),
				}
			}
			return resolved, fmt.Errorf("resolving preset %q: %w", server.Spec.PresetRef, err)
		}
		resolved.merged = components.MergePreset(server.Spec, &preset.Spec)
	}

	if err := components.ValidateMerged(resolved.merged); err != nil {
		return resolved, &conditions.PreCheckFailure{
			Reason:  v1.ReasonInvalidReference,
			Message: err.Error(),
		}
	}

	platform, err := r.resolvePlatform(ctx, resolved.merged)
	if err != nil {
		return resolved, err
	}
	resolved.platform = platform

	// The plugin check stands outside the suspension tolerance below. Nothing
	// reports that the plugin arrived, so a server held on it would sit with
	// no reason on its Ready condition until something else wrote the status.
	if resolved.merged.Archive != nil && !r.barmanInstalled {
		return resolved, &conditions.PreCheckFailure{
			Reason: v1.ReasonBarmanPluginNotInstalled,
			Message: "The Barman Cloud plugin is not installed on this cluster. " +
				"Install it, then restart the operator",
		}
	}

	archive, err := r.resolveArchiveStorage(ctx, resolved.merged)
	var failure *conditions.PreCheckFailure
	switch {
	case err == nil:
		resolved.archive = archive

	// A suspended server that already reached its desired state keeps
	// reporting Suspended when its bucket stops resolving. Its pods are gone
	// and it takes no backups, so the reference matters again only when it is
	// unsuspended, and flapping Ready in the meantime tells the reader
	// nothing they can act on. A server that never reached Ready is the other
	// case: it has no honest condition to keep, so it reports the failure.
	// Both references are watched, so either way the reconcile comes back.
	case resolved.merged.Suspend && errors.As(err, &failure) &&
		meta.IsStatusConditionTrue(server.Status.Conditions, v1.ConditionReady):
		resolved.holdForSuspension = true

	default:
		return resolved, err
	}

	return resolved, nil
}

// resolvePlatform reads the platform config that the merged spec names. Only
// its image settings are read, so a server that names none renders the default
// repository.
func (r *DatabaseServerReconciler) resolvePlatform(
	ctx context.Context,
	merged v1.DatabaseServerSpec,
) (*v1.CamundaPlatformConfigSpec, error) {
	if merged.PlatformConfigRef == "" {
		return nil, nil
	}

	var config v1.CamundaPlatformConfig
	if err := r.Get(ctx, types.NamespacedName{Name: merged.PlatformConfigRef}, &config); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &conditions.PreCheckFailure{
				Reason:  v1.ReasonInvalidReference,
				Message: fmt.Sprintf("CamundaPlatformConfig %q not found", merged.PlatformConfigRef),
			}
		}

		return nil, fmt.Errorf("resolving platform config %q: %w", merged.PlatformConfigRef, err)
	}

	return &config.Spec, nil
}

// resolveArchiveStorage resolves the archive bucket of the merged spec into
// the contract and, for a contract with static credentials, the keys of its
// Secret. It returns nil when the spec names no bucket, which means the server
// has no archive.
//
// A reference that does not resolve is a pre-check failure, not an error: the
// contract, or the Secret it names, can appear later, and both are watched.
func (r *DatabaseServerReconciler) resolveArchiveStorage(
	ctx context.Context,
	merged v1.DatabaseServerSpec,
) (*components.ArchiveStorage, error) {
	if merged.Archive == nil {
		return nil, nil
	}

	// The cached client: the type is watched, so the cache is current, and
	// every bucket event lands here again anyway.
	var config v1.ObjectStorageConfig
	if err := r.Get(ctx, types.NamespacedName{Name: merged.Archive.ObjectStorageRef}, &config); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &conditions.PreCheckFailure{
				Reason:  v1.ReasonInvalidReference,
				Message: fmt.Sprintf("ObjectStorageConfig %q not found", merged.Archive.ObjectStorageRef),
			}
		}

		return nil, fmt.Errorf("resolving archive storage %q: %w", merged.Archive.ObjectStorageRef, err)
	}

	if err := components.ValidateArchiveStorage(&config); err != nil {
		return nil, &conditions.PreCheckFailure{
			Reason:  v1.ReasonInvalidReference,
			Message: err.Error(),
		}
	}

	archive := &components.ArchiveStorage{Config: &config}

	credentialsSecret := config.CredentialsSecret()
	if credentialsSecret == nil {
		return archive, nil
	}

	// The Secret is read live: the watch on it is metadata-only.
	key := types.NamespacedName{Namespace: credentialsSecret.Namespace, Name: credentialsSecret.Name}
	bucketSecret, msg, err := secretref.Get(ctx, r.APIReader, key, credentialsSecret.Keys...)
	if err != nil {
		return nil, fmt.Errorf("reading credentials of archive storage %q: %w", config.Name, err)
	}
	if msg != "" {
		return nil, &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: msg}
	}

	credentials, err := objectstore.CredentialsFrom(&config, bucketSecret.Data)
	if err != nil {
		return nil, &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: err.Error()}
	}
	archive.Credentials = credentials

	return archive, nil
}

// archiveStart returns when the earliest base backup of the archive the server
// writes now completed, or nil when none has. It is what tells the archive
// component that the archive can be recovered from, and what opens the
// interval of that archive in status.
//
// A server that re-enabled its archive counts only the backups taken after the
// interval it closed. The backups of the archive it wrote before reach no
// point in the new one, so treating them as the start would declare a window
// that no restore can reach. A server that asks for no archive takes no base
// backups, so it reads none.
func (r *DatabaseServerReconciler) archiveStart(
	ctx context.Context,
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
) (*metav1.Time, error) {
	if merged.Archive == nil {
		return nil, nil
	}

	after := closedArchiveEnd(server)

	var backups cnpgv1.BackupList
	if err := r.List(
		ctx, &backups,
		client.InNamespace(server.Namespace),
		client.MatchingLabels{components.CNPGClusterNameLabel: components.ClusterName(server)},
	); err != nil {
		return nil, fmt.Errorf("listing base backups of %q: %w", components.ClusterName(server), err)
	}

	var earliest *metav1.Time
	for i := range backups.Items {
		backup := &backups.Items[i]
		// The label scopes the read. The cluster the backup names is what
		// decides, because this operator writes neither.
		if backup.Spec.Cluster.Name != components.ClusterName(server) ||
			backup.Status.Phase != cnpgv1.BackupPhaseCompleted ||
			backup.Status.StoppedAt == nil {
			continue
		}
		if after != nil && !backup.Status.StoppedAt.After(after.Time) {
			continue
		}
		if earliest == nil || backup.Status.StoppedAt.Before(earliest) {
			earliest = backup.Status.StoppedAt
		}
	}

	return earliest, nil
}

// clearReenabledArchiveCondition drops ArchiveReady when a server that had no
// archive asks for one again.
//
// ocf carries the condition status of the previous reconcile into a component
// that is converging again, and a disabled component carries True. Without
// this, an archive that comes back would keep reporting ready while its first
// base backup is still missing, and it would never correct itself: every
// following reconcile copies that True forward again. A condition that is
// absent starts from Unknown instead, which ocf derives from the resources.
func clearReenabledArchiveCondition(server *v1.DatabaseServer, merged v1.DatabaseServerSpec) {
	if merged.Archive == nil {
		return
	}

	condition := meta.FindStatusCondition(server.Status.Conditions, components.ConditionArchive)
	if condition == nil || condition.Reason != string(component.Disabled) {
		return
	}

	meta.RemoveStatusCondition(&server.Status.Conditions, components.ConditionArchive)
}

// closedArchiveEnd returns when the archive of the current cluster last
// closed, or nil when the server has never closed one for it. A closed
// interval means the server stopped archiving and started again, so everything
// the new archive holds comes after that point.
func closedArchiveEnd(server *v1.DatabaseServer) *metav1.Time {
	if server.Status.Archive == nil {
		return nil
	}

	var end *metav1.Time
	for i := range server.Status.Archive.History {
		record := &server.Status.Archive.History[i]
		if record.ServerName != components.ClusterName(server) || record.To == nil {
			continue
		}
		if end == nil || record.To.After(end.Time) {
			end = record.To
		}
	}

	return end
}

// buildComponents builds the four components in dependency order: cluster,
// archive, contract, monitoring. The returned data cell holds the PostgreSQL
// system identifier once the cluster component has reconciled.
func (r *DatabaseServerReconciler) buildComponents(
	server *v1.DatabaseServer,
	resolved resolvedSpec,
	archiveStart *metav1.Time,
) (serverComponents, *concepts.Data[string], error) {
	merged := resolved.merged
	var built serverComponents

	cluster, systemIdentifier, err := components.ClusterComponent(
		server, merged, resolved.archive, resolved.platform,
	)
	if err != nil {
		return built, nil, fmt.Errorf("building cluster component: %w", err)
	}
	built.cluster = cluster

	built.archive, err = components.ArchiveComponent(server, merged, resolved.archive, archiveStart)
	if err != nil {
		return built, nil, fmt.Errorf("building archive component: %w", err)
	}

	built.contract, err = components.ContractComponent(server, merged)
	if err != nil {
		return built, nil, fmt.Errorf("building contract component: %w", err)
	}

	built.monitoring, err = components.MonitoringComponent(server, merged, r.podMonitorSupported())
	if err != nil {
		return built, nil, fmt.Errorf("building monitoring component: %w", err)
	}

	return built, systemIdentifier, nil
}

// reconcileComponents reconciles comps in order. It continues past a failing
// component, so one failure does not stall the rest, and returns the first
// error.
func reconcileComponents(ctx context.Context, recCtx component.ReconcileContext, comps []*component.Component) error {
	var firstErr error
	for _, comp := range comps {
		if err := comp.Reconcile(ctx, recCtx); err != nil && firstErr == nil {
			firstErr = err
		}
	}

	return firstErr
}

// reconcileArchiveHistory keeps status.archive.history in step with the spec.
//
// While the server archives, the interval of the current archive opens once
// the archive component reports ready. A recovery reaches only a point inside
// a recorded interval, so the record must exist before the first restore, and
// its start is when the first base backup of that archive completed: the
// archive cannot be recovered to any point before that.
//
// When the spec drops the archive, the open record closes at now and no record
// is written again. The closed records stay: the bucket still holds those
// archives, and a server that archives again can recover from them. A server
// that archives again opens a record of its own, so the window with no archive
// stays outside every interval and no restore can ask for a point in it.
func reconcileArchiveHistory(
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	archiveComp *component.Component,
	archiveStart *metav1.Time,
	now metav1.Time,
) {
	if merged.Archive == nil {
		closeArchiveRecords(server, now)
		return
	}

	if archiveStart == nil || archiveComp.GetCondition(server).Status != metav1.ConditionTrue {
		return
	}

	if open := openArchiveRecord(server); open != nil {
		if open.ServerName == components.ClusterName(server) {
			return
		}
		// The server writes one archive at a time, so an archive of another
		// cluster that is still open ended where this one starts.
		open.To = archiveStart
	}

	if server.Status.Archive == nil {
		server.Status.Archive = &v1.DatabaseServerArchiveStatus{}
	}
	server.Status.Archive.History = append(server.Status.Archive.History, v1.ArchiveRecord{
		ServerName: components.ClusterName(server),
		From:       *archiveStart,
	})
}

// openArchiveRecord returns the archive the server writes now, or nil when it
// writes none. At most one record is open: closeArchiveRecords closes every
// one of them, and reconcileArchiveHistory closes the open record before it
// appends.
func openArchiveRecord(server *v1.DatabaseServer) *v1.ArchiveRecord {
	if server.Status.Archive == nil {
		return nil
	}

	for i := range server.Status.Archive.History {
		if server.Status.Archive.History[i].To == nil {
			return &server.Status.Archive.History[i]
		}
	}

	return nil
}

// closeArchiveRecords ends the interval of every archive the server still has
// open. One is open at most, and closing all of them is what keeps that true
// whatever an earlier version of this operator left behind.
func closeArchiveRecords(server *v1.DatabaseServer, at metav1.Time) {
	if server.Status.Archive == nil {
		return
	}

	for i := range server.Status.Archive.History {
		if server.Status.Archive.History[i].To == nil {
			server.Status.Archive.History[i].To = &at
		}
	}
}

// dataVolumes lists the data volumes of the current cluster, sorted by name.
// CloudNativePG labels every claim of a cluster with the cluster name.
func (r *DatabaseServerReconciler) dataVolumes(
	ctx context.Context,
	server *v1.DatabaseServer,
) ([]v1.VolumeStatus, error) {
	var claims corev1.PersistentVolumeClaimList
	if err := r.List(
		ctx, &claims,
		client.InNamespace(server.Namespace),
		client.MatchingLabels{components.CNPGClusterNameLabel: components.ClusterName(server)},
	); err != nil {
		return nil, fmt.Errorf("listing data volume claims of %q: %w", components.ClusterName(server), err)
	}

	var volumes []v1.VolumeStatus
	for i := range claims.Items {
		claim := &claims.Items[i]
		if capacity, ok := claim.Status.Capacity[corev1.ResourceStorage]; ok {
			volumes = append(volumes, v1.VolumeStatus{Name: claim.Name, Capacity: capacity})
		}
	}
	slices.SortFunc(volumes, func(a, b v1.VolumeStatus) int { return strings.Compare(a.Name, b.Name) })

	return volumes, nil
}

// retryInterval returns the wait before the superuser Secret is looked at
// again.
func (r *DatabaseServerReconciler) retryInterval() time.Duration {
	if r.RetryInterval > 0 {
		return r.RetryInterval
	}

	return defaultRetryInterval
}

// podMonitorSupported reports whether the cluster serves the PodMonitor kind.
// On a cluster without the prometheus-operator CRDs the monitoring component
// then omits the resource instead of failing every reconcile.
func (r *DatabaseServerReconciler) podMonitorSupported() bool {
	return r.served("monitoring.coreos.com", "PodMonitor", "v1")
}

// served reports whether the cluster serves the given kind at the given
// version. The version is named on purpose: a cluster that serves only another
// version would pass an any-version check and then fail at apply.
func (r *DatabaseServerReconciler) served(group, kind, version string) bool {
	if r.restMapper == nil {
		return false
	}

	_, err := r.restMapper.RESTMapping(schema.GroupKind{Group: group, Kind: kind}, version)

	return err == nil
}

// SetupWithManager registers the controller, ownership watches on the
// CloudNativePG cluster, the ObjectStore, the ScheduledBackup, the PodMonitor,
// and the published contract, a preset watch through a field index on
// spec.presetRef, watches on the archive bucket and its credentials Secret, and
// watches on the base backups and data volume claims that CloudNativePG
// creates.
//
// The CloudNativePG and Barman Cloud watches are registered only when the
// cluster serves those kinds. An informer on a kind that the API server does
// not serve fails the cache sync and stops the manager, and the operator must
// start on a cluster that runs no PostgreSQL of its own. Without them the
// controller still runs and reports CNPGNotInstalled or
// BarmanPluginNotInstalled. The decision is made once: install the missing
// operator, then restart this one.
func (r *DatabaseServerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder("databaseserver")
	}
	r.restMapper = mgr.GetRESTMapper()
	r.cnpgInstalled = r.served(cnpgv1.SchemeGroupVersion.Group, "Cluster", cnpgv1.SchemeGroupVersion.Version)
	r.barmanInstalled = r.served(
		barmanobjectstore.GroupVersion.Group, "ObjectStore", barmanobjectstore.GroupVersion.Version,
	)

	if r.componentClient == nil {
		// Uncached: see the componentClient field doc.
		componentClient, err := client.New(mgr.GetConfig(), client.Options{
			Scheme: mgr.GetScheme(),
			Mapper: mgr.GetRESTMapper(),
		})
		if err != nil {
			return fmt.Errorf("building the component client: %w", err)
		}
		r.componentClient = componentClient
	}

	return r.watches(mgr)
}
