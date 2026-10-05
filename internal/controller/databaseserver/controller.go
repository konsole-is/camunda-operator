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
// merges the preset and the release into the spec, drives the external
// CloudNativePG operator through a rendered cluster, archives the server to an
// object storage bucket through the Barman Cloud plugin, and publishes the
// DatabaseServerConfig contract that a Database and a PointInTimeRestore
// consume.
package databaseserver

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/observability"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/grace"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
)

// controllerName is the name the controller registers with controller-runtime.
// It labels its events and every metrics series it records.
const controllerName = "databaseserver"

// defaultRetryInterval is how long the controller waits before it looks again
// at the superuser Secret. CloudNativePG owns that Secret, so no watch of this
// controller reports its creation, and the published contract must not name it
// before it exists.
const defaultRetryInterval = 30 * time.Second

// resolvedSpec is everything the pre-checks resolved for one reconcile: the
// merged spec, the platform config whose image settings rename the
// PostgreSQL image, and the archive bucket. Resolving each of them once is
// what keeps the components from re-reading the same objects per method.
type resolvedSpec struct {
	merged   v1.DatabaseServerSpec
	platform *v1.CamundaPlatformConfigSpec
	archive  *components.ArchiveStorage
	// archiveLocation is where in object storage the archive of the server is
	// written, rendered from the resolved bucket. status.archive compares
	// intervals by it rather than by the name of the ObjectStorageConfig,
	// which can be edited in place. It is empty when the bucket does not
	// resolve.
	archiveLocation string
	// holdForSuspension says that the archive of a server whose instances are
	// already down did not resolve. The reconcile then stops and leaves the
	// conditions alone: see preCheck.
	holdForSuspension bool
	// holdForRecovery says that the spec moved the contract, or moved or
	// removed the archive, that the running recovery depends on. The merged
	// spec then keeps what the recovery recorded, and Ready reports why: see
	// preCheck.
	holdForRecovery *conditions.PreCheckFailure
	// holdArchive says that the archive moved under a recovery that still
	// reads it. The archive component then does not reconcile, so the
	// ObjectStore keeps describing the archive the recovery asked for: see
	// recoveryHoldsLocation.
	holdArchive bool
	// contractTaken says why the DatabaseServerConfig the merged spec names is
	// not this server's to publish, and it is empty when the name is free or
	// the contract is the server's own. The contract component blocks the
	// apply on it and ContractReady reports ContractTaken: see contractTaken.
	contractTaken string
	// archiveTaken says why the Barman Cloud ObjectStore of the name the
	// server derives is not this server's to write, and it is empty when the
	// name is free or the ObjectStore is the server's own. The cluster then
	// carries no archive plugin, the base backup schedule goes, the contract
	// publishes no point-in-time-recovery capability, a rollback is refused,
	// and ArchiveReady reports ArchiveTaken: see archiveTaken.
	archiveTaken string
	// serviceAccountTaken says why the ServiceAccount of the instance pods is
	// not this server's, and it is empty when the name is free or the account
	// is the server's own. A rollback is refused while it is set: see
	// serviceAccountTaken.
	serviceAccountTaken string
	// archivePluginRoles are the clusters of the server that the Role of the
	// Barman Cloud plugin is bound for: see archivePluginRoles.
	archivePluginRoles []components.ArchivePluginRole
	// clusterTaken says why a CloudNativePG cluster of the name the server
	// derives is not this server's to write, and it is empty when the name is
	// free or the cluster is the server's own. Every component reads it and
	// withdraws what names that cluster, and ClusterReady reports
	// ClusterTaken. The recovery decides the name, so this is filled in after
	// the recovery and not by preCheck: see clusterTaken.
	clusterTaken string
	// clusterBlocked is why the cluster component must not apply the cluster of
	// the name the server derives. It is clusterTaken while the name is held,
	// and it also covers the cluster that a running rollback cut over to and
	// that is gone: see clusterGuardReason.
	clusterBlocked string
	// archiveOutage is the stop in the write-ahead log uploads that the server
	// reports on, or nil when it reports on none. It blocks the archive
	// component, it reports ArchiveFailing, and it marks the open archive
	// record: see reportedArchiveOutage.
	archiveOutage *components.ArchiveOutage
	// requested is what the merged spec asked for before keepAppliedStorageSize
	// raised it to the volumes that are there.
	requested components.RequestedStorage
}

// serverComponents are the components of one reconcile, in the order they
// reconcile in. They are named rather than indexed, so a reorder cannot
// silently hand one of them to the wrong caller.
type serverComponents struct {
	cluster    *component.Component
	archive    *component.Component
	contract   *component.Component
	monitoring *component.Component
	// ready are the components that take part in Ready: the cluster, the
	// contract, and the archive of a server that asks for one. Monitoring
	// keeps its own MonitoringReady condition, and an archive the spec does
	// not ask for keeps ArchiveReady, so Ready never reports Disabled.
	// Monitoring stays out whether or not it is enabled, because a PodMonitor
	// observes the server rather than runs it. ElasticsearchCluster keeps its
	// metrics exporter out of Ready for the same reason.
	ready []*component.Component
	// systemIdentifier holds the PostgreSQL system identifier once the cluster
	// component has reconciled, and the reconcile mirrors it to status.
	systemIdentifier *concepts.Data[string]
	// archiveDestination is the destination path the archive component applied
	// on its ObjectStore: the bucket URL that holds the archives of this
	// server, prefix included. It is unset when that apply did not happen:
	// see components.ArchiveComponent.
	archiveDestination *concepts.Data[string]
}

// DatabaseServerReconciler runs a PostgreSQL server through the external
// CloudNativePG operator. It renders a CloudNativePG cluster, archives it to
// the bucket that spec.archive names, and publishes a DatabaseServerConfig in
// the namespace of the CR.
type DatabaseServerReconciler struct {
	client.Client
	// APIReader reads without the cache. The server itself and the bucket
	// credentials Secret are both read live: the recovery keys its steps on
	// status.recovery, and the Secret is watched with metadata only.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the component lifecycle events. SetupWithManager
	// sets it from the manager.
	EventRecorder events.EventRecorder
	// Metrics records the condition gauge and the apply counters of the
	// framework. SetupWithManager sets it when it is nil.
	Metrics component.MetricsRecorder

	// RetryInterval overrides how long the controller waits on the superuser
	// Secret. Zero means defaultRetryInterval; tests shorten it.
	RetryInterval time.Duration
	// GracePeriods are the grace periods of the cluster component, which uses
	// the datastore period. The zero value keeps ClusterReady on its progress
	// reason.
	GracePeriods grace.Periods

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
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundareleases,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=objectstorageconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=postgresql.cnpg.io,resources=clusters;scheduledbackups,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=postgresql.cnpg.io,resources=backups,verbs=get;list;watch
// +kubebuilder:rbac:groups=barmancloud.cnpg.io,resources=objectstores,verbs=get;list;watch;create;update;patch;delete
// Writes no ObjectStore status. The Role of the Barman Cloud plugin grants it, and the
// API server lets the operator bind that Role only while it holds every rule of it.
// +kubebuilder:rbac:groups=barmancloud.cnpg.io,resources=objectstores/status,verbs=update
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=roles,verbs=get;list;watch
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=monitoring.coreos.com,resources=podmonitors,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile converges a DatabaseServer. It resolves the preset and the
// release, runs the pre-checks, reconciles the cluster, archive, contract, and
// monitoring components, and derives the CR-level Ready condition.
//
// Status is written once per reconcile. The components and conditions.Stage
// stage conditions on the in-memory server, and the deferred FlushStatus
// persists them together with the observed cluster, identifier, archive
// history, and volumes.
func (r *DatabaseServerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	// Live, not cached. A recovery is a state machine whose marker is
	// status.recovery: each step reads it to tell what the last one did, and
	// the steps create and delete CloudNativePG clusters. A stale read of that
	// marker re-enters a step whose side effect already ran.
	var server v1.DatabaseServer
	if err := r.APIReader.Get(ctx, req.NamespacedName, &server); err != nil {
		if apierrors.IsNotFound(err) {
			observability.Forget(r.Metrics, new(v1.DatabaseServer).GetKind(), req.NamespacedName)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
	}

	recCtx := component.ReconcileContext{
		Client:        r.componentClient,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
		Metrics:       r.Metrics,
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

	// Ready as the last reconcile left it. This one stages over it further
	// down, and the version refusal needs to know whether it already stood.
	standingReady := meta.FindStatusCondition(server.Status.Conditions, v1.ConditionReady).DeepCopy()

	resolved, err := r.preCheck(ctx, &server)
	var failure *conditions.PreCheckFailure
	if errors.As(err, &failure) {
		conditions.Stage(&server, conditions.Failed(&server, failure))
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}

	// Before anything renders, and before the recovery below builds from the
	// merged spec. A refused major change is not a stop: the merged spec keeps
	// the major the data directory runs, everything below reconciles on it,
	// and Ready reports the refusal further down.
	refusedVersion, err := r.keepRunningVersion(ctx, &server, &resolved.merged)
	if err != nil {
		return ctrl.Result{}, err
	}

	// After the guard, so the published version is the major the instances
	// run rather than the one a refused change asks for.
	server.Status.Version = resolved.merged.Version

	// Read once and used twice: the clamp below, and status.Volumes further
	// down. Nothing this reconcile applies creates a claim, so the second
	// reader loses nothing by taking the same answer.
	volumes, err := r.volumeClaims(ctx, &server)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Before the recovery below: the cluster a rollback builds carries the
	// volume sizes of the merged spec, and it must not come back smaller than
	// the server it replaces.
	r.keepAppliedStorageSize(&server, &resolved, volumes)

	// Before the hold below: a suspended server refuses a recovery request,
	// and a request nobody answers holds whoever asked for good.
	recovering, err := r.reconcileRecovery(ctx, &server, resolved)
	if err != nil {
		return ctrl.Result{}, err
	}

	if resolved.holdForSuspension {
		return ctrl.Result{}, nil
	}

	// The move is decided before the components build, and a reconcile that
	// finds one reads no backups at all: the ObjectStore it is about to apply
	// is what puts the archive in the new place, and every backup that exists
	// now began before that. The archive component blocks on a nil start and
	// still applies the ObjectStore, which is registered ahead of the guard.
	//
	// A held archive has moved nowhere. Nothing applies the location the spec
	// resolves to now, so it is decided again on the reconcile after the hold
	// lifts, against the location that is applied by then.
	moved := !resolved.holdArchive &&
		archiveMoved(&server, archiveRef(resolved.merged), resolved.archiveLocation)

	var archiveStart *metav1.Time
	if !moved {
		archiveStart, err = r.archiveStart(
			ctx, &server, resolved.merged,
			archiveBoundary(&server, resolved.merged),
		)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	// After the recovery, which records the cluster that a rollback builds.
	resolved.archivePluginRoles, err = r.archivePluginRoles(ctx, &server, resolved.serviceAccountTaken)
	if err != nil {
		return ctrl.Result{}, err
	}

	// After the recovery, because the recovery is what moves status.cluster:
	// it moves onto the cluster it built at the cutover, and back onto the
	// previous one when it abandons that cluster. The name every component
	// below renders is the name that has to be free, so the read is here and
	// not in preCheck.
	//
	// It is the last read before the apply. Between the two, an object of that
	// name that goes is built again by the apply, which is what the guard on a
	// rollback that cut over exists to stop, so nothing that reads the API
	// server belongs in between.
	derived, err := r.readDerivedCluster(ctx, &server)
	if err != nil {
		return ctrl.Result{}, err
	}
	resolved.clusterTaken = derived.taken
	resolved.clusterBlocked = clusterGuardReason(&server, derived)
	resolved.archiveOutage = reportedArchiveOutage(derived.outage, resolved.merged)

	built, err := r.buildComponents(&server, resolved, archiveStart)
	if err != nil {
		return ctrl.Result{}, err
	}
	comps = built.all()

	// A cluster whose name was held gets the whole grace period once its name
	// is free. Otherwise the transition time of the held period ends it at once.
	if resolved.clusterTaken == "" {
		grace.Restart(&server, v1.ConditionClusterReady, v1.ReasonClusterTaken)
	}

	reconcileErr := reconcileComponents(ctx, recCtx, built.applying(resolved.holdArchive))

	if err := r.removeSupersededContracts(ctx, &server, resolved.merged); err != nil {
		return ctrl.Result{}, errors.Join(reconcileErr, err)
	}

	if identifier, ok := built.systemIdentifier.Get(); ok && identifier != "" {
		server.Status.SystemIdentifier = identifier
	}
	// The clock is read here, after the ObjectStore of the new location is
	// applied. A backup that began while the old one still stood therefore
	// began before the boundary, whatever its start says.
	//
	// A held archive writes no history at all. Every record names where its
	// objects are, and the location the spec resolves to now is nowhere the
	// server has written.
	//
	// A taken cluster writes none either. The archive of that name belongs to
	// whoever holds the cluster, and its base backups carry the label this
	// read counts by, so a record here calls somebody else's archive an
	// archive of this server.
	//
	// A taken ObjectStore writes none for the same reason. The server takes
	// the archive off the cluster while that name is held, so nothing of this
	// server reaches the bucket that a record here names.
	if !resolved.holdArchive && resolved.clusterTaken == "" && resolved.archiveTaken == "" {
		now := metav1.Now()
		advanceArchiveFloor(&server, resolved.merged, now)
		reconcileArchiveHistory(
			&server, resolved.merged, built.archive, archiveStart,
			resolved.archiveLocation, moved, built.archiveDestination.IsSet(), now,
		)
		markArchiveOutage(&server, resolved.archiveOutage)
	}

	server.Status.Volumes = volumes.all()

	// Before the aggregate below, which reads the component conditions off
	// the server. A taken name goes last of the two: an ObjectStore that
	// belongs to somebody else takes the archive off the cluster, so there are
	// no uploads of this server to report on.
	stageArchiveOutage(&server, resolved.archiveOutage)
	stageTakenNames(&server, resolved)

	conditions.Stage(&server, conditions.Aggregate(&server, built.ready...))

	// A refusal and a hold are the reasons the reader acts on, so each wins
	// over whatever the components report. The hold goes last: a rollback that
	// nobody answers holds whoever asked for good, while a refused version
	// leaves a server that runs.
	if refusedVersion != nil {
		r.recordRefusedVersionChange(&server, standingReady, refusedVersion)
		conditions.Stage(&server, conditions.Failed(&server, refusedVersion))
	}
	if resolved.holdForRecovery != nil {
		conditions.Stage(&server, conditions.Failed(&server, resolved.holdForRecovery))
	}

	if reconcileErr != nil {
		return ctrl.Result{}, reconcileErr
	}

	return ctrl.Result{RequeueAfter: r.requeueAfter(&server, resolved, derived, recovering, built.cluster)}, nil
}

// all returns the components in reconcile order. FlushStatus owns every one of
// their condition types.
func (c serverComponents) all() []*component.Component {
	return []*component.Component{c.cluster, c.archive, c.contract, c.monitoring}
}

// applying returns the components to reconcile, in the same order. It leaves
// the archive out while the server holds it: the ObjectStore is one object,
// and rewriting it while a recovery reads the archive it describes points the
// cluster that is recovering somewhere that archive is not.
//
// A cluster that another owner holds is not a reason to leave a component out.
// The contract, the base backup schedule, and the PodMonitor all name the
// cluster of that name, and the ones the server applied before it lost the
// cluster are still there. Each of those components withdraws its own objects
// while the name is held, and withdrawing takes a reconcile.
func (c serverComponents) applying(holdArchive bool) []*component.Component {
	if !holdArchive {
		return c.all()
	}

	return []*component.Component{c.cluster, c.contract, c.monitoring}
}

// buildComponents builds the four components in dependency order: cluster,
// archive, contract, monitoring, and records which of them take part in Ready.
// components.Archiving decides both the gate of the archive component and its
// part in Ready, so the two can never disagree.
func (r *DatabaseServerReconciler) buildComponents(
	server *v1.DatabaseServer,
	resolved resolvedSpec,
	archiveStart *metav1.Time,
) (serverComponents, error) {
	merged := resolved.merged
	var built serverComponents

	cluster, systemIdentifier, err := components.ClusterComponent(
		server, merged, resolved.requested, resolved.archive, resolved.archiveTaken,
		resolved.platform, resolved.clusterBlocked, resolved.archivePluginRoles, r.GracePeriods.Datastore,
	)
	if err != nil {
		return built, fmt.Errorf("building cluster component: %w", err)
	}
	built.cluster = cluster
	built.systemIdentifier = systemIdentifier

	archive, destination, err := components.ArchiveComponent(
		server, merged, resolved.archive, archiveStart, resolved.archiveOutage,
		resolved.clusterTaken, resolved.archiveTaken,
	)
	if err != nil {
		return built, fmt.Errorf("building archive component: %w", err)
	}
	built.archive = archive
	built.archiveDestination = destination

	built.contract, err = components.ContractComponent(
		server, merged, resolved.clusterTaken, resolved.contractTaken, resolved.archiveTaken,
	)
	if err != nil {
		return built, fmt.Errorf("building contract component: %w", err)
	}

	built.monitoring, err = components.MonitoringComponent(
		server, merged, r.podMonitorSupported(), resolved.clusterTaken,
	)
	if err != nil {
		return built, fmt.Errorf("building monitoring component: %w", err)
	}

	built.ready = []*component.Component{built.cluster, built.contract}
	if components.Archiving(merged) {
		built.ready = append(built.ready, built.archive)
	}

	return built, nil
}

// podMonitorSupported reports whether the cluster serves the PodMonitor kind.
// On a cluster without the prometheus-operator CRDs the monitoring component
// then omits the resource instead of failing every reconcile.
func (r *DatabaseServerReconciler) podMonitorSupported() bool {
	return r.served("monitoring.coreos.com", "PodMonitor", "v1")
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

// requeueAfter returns the shortest wait this reconcile asks for, or zero when
// it asks for none.
func (r *DatabaseServerReconciler) requeueAfter(
	server *v1.DatabaseServer,
	resolved resolvedSpec,
	derived derivedCluster,
	recovering bool,
	cluster *component.Component,
) time.Duration {
	var waits []time.Duration

	// Nothing reports that the superuser Secret appeared: CloudNativePG owns
	// it, and this controller watches only what it owns itself. Every other
	// condition of this CR is backed by a watch. A running recovery waits on
	// that same Secret, one cluster further on.
	//
	// Nothing reports that a taken cluster name became free either, for the
	// same reason: the object of that name belongs to somebody else, so no
	// watch of this controller carries its deletion. ContractReady is True
	// while the name is held, because the component withdrew the contract on
	// purpose, so that test cannot stand in for this one. A taken ObjectStore
	// name asks for a look of its own for the same reason: the object belongs
	// to somebody else, and its deletion reaches no watch of this controller
	// either.
	//
	// Each of these repeats: nothing carries the moment the thing it waits for
	// arrives, so the look runs again until it does.
	if recovering || resolved.clusterTaken != "" || resolved.archiveTaken != "" ||
		!meta.IsStatusConditionTrue(server.Status.Conditions, v1.ConditionContractReady) {
		waits = append(waits, r.retryInterval())
	}

	// Failing write-ahead log uploads ask for one look, when the grace period
	// of the outage ends. CloudNativePG writes that condition once and leaves
	// it, so nothing carries that moment either. Uploads that run again before
	// it comes rewrite the condition on the cluster, which this controller
	// owns, so that arrives on a watch and needs no look of its own.
	if wait := pendingArchiveOutageWait(derived.outage, resolved.merged, time.Now()); wait > 0 {
		waits = append(waits, wait)
	}

	if wait, ok := grace.Remaining(server, cluster); ok {
		waits = append(waits, wait)
	}

	if len(waits) == 0 {
		return 0
	}

	return slices.Min(waits)
}

// retryInterval returns the wait before the superuser Secret is looked at
// again.
func (r *DatabaseServerReconciler) retryInterval() time.Duration {
	if r.RetryInterval > 0 {
		return r.RetryInterval
	}

	return defaultRetryInterval
}

// SetupWithManager registers the controller and its watches with mgr.
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
		r.EventRecorder = mgr.GetEventRecorder(controllerName)
	}
	if r.Metrics == nil {
		r.Metrics = observability.Recorder(controllerName)
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
