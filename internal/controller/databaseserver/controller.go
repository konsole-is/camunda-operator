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
	"k8s.io/apimachinery/pkg/api/resource"
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
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
)

// defaultRetryInterval is how long the controller waits before it looks again
// at the superuser Secret. CloudNativePG owns that Secret, so no watch of this
// controller reports its creation, and the published contract must not name it
// before it exists.
const defaultRetryInterval = 30 * time.Second

// eventReasonStorageShrinkIgnored is the Warning event that the controller
// records when a merged volume size is below the size that is already there.
// It keeps the size that is there, because PostgreSQL volumes cannot be
// reduced in place.
const eventReasonStorageShrinkIgnored = "StorageShrinkIgnored"

// eventReasonWALStorageKept is the Warning event that the controller records
// when a merged spec asks for no write-ahead log volume under a server that
// has one. It keeps the volume, because CloudNativePG refuses a cluster that
// gives one up.
const eventReasonWALStorageKept = "WALStorageKept"

// eventActionResize is the action of the events that the controller records
// about the size of the volumes.
const eventActionResize = "Resize"

// resolvedSpec is everything the pre-checks resolved for one reconcile: the
// preset-merged spec, the platform config whose image settings rename the
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
	// archiveDestination holds the bucket URL of the ObjectStore that the
	// archive component applied, and is unset when that apply did not happen:
	// see components.ArchiveComponent.
	archiveDestination *concepts.Data[string]
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
	// Live, not cached. A recovery is a state machine whose marker is
	// status.recovery: each step reads it to tell what the last one did, and
	// the steps create and delete CloudNativePG clusters. A stale read of that
	// marker re-enters a step whose side effect already ran.
	var server v1.DatabaseServer
	if err := r.APIReader.Get(ctx, req.NamespacedName, &server); err != nil {
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
	r.keepAppliedStorageSize(&server, &resolved.merged, volumes)

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

	// Nothing reports that the superuser Secret appeared: CloudNativePG owns
	// it, and this controller watches only what it owns itself. Every other
	// condition of this CR is backed by a watch. A running recovery waits on
	// that same Secret, one cluster further on.
	//
	// Nothing reports that a taken cluster name became free either, for the
	// same reason: the object of that name belongs to somebody else, so no
	// watch of this controller carries its deletion. ContractReady is True
	// while the name is held, because the component withdrew the contract on
	// purpose, so the test above cannot stand in for this one. A taken
	// ObjectStore name asks for a look of its own for the same reason: the
	// object belongs to somebody else, and its deletion reaches no watch of
	// this controller either.
	//
	// Failing write-ahead log uploads ask for one too. CloudNativePG writes
	// that condition once and leaves it, so nothing reports that the grace
	// period of the outage passed.
	if recovering || resolved.clusterTaken != "" || resolved.archiveTaken != "" ||
		pendingArchiveOutage(derived.outage, resolved.merged) ||
		!meta.IsStatusConditionTrue(server.Status.Conditions, v1.ConditionContractReady) {
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
// A bucket that stops resolving under a server whose instances are already
// down is neither: the result carries holdForSuspension, and the caller stops
// the reconcile so the server keeps the Suspended it reached.
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

	// Not a failure: the server keeps the contract and the archive that the
	// running recovery depends on, and it reports why. The recovery needs the
	// contract republished to finish, so holding the components instead holds
	// the recovery that the hold exists to protect.
	if hold := recoveryHoldsSpec(server, resolved.merged); hold != nil {
		running := server.Status.Recovery
		resolved.holdForRecovery = hold
		resolved.merged.DatabaseServerConfig = running.Contract
		if running.Archive != nil {
			resolved.merged.Archive = heldArchive(running.Archive, resolved.merged.Archive)
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
		resolved.archiveLocation = archive.ArchiveLocation(server)

	// A server whose instances are already down keeps reporting Suspended
	// when its bucket stops resolving. It runs nothing and takes no backups,
	// so the reference matters again only when it is unsuspended, and
	// flapping Ready in the meantime tells the reader nothing they can act
	// on. Both references are watched, so the reconcile comes back.
	//
	// The test is the suspension the server reached, never the suspension it
	// asked for. A running server whose spec has just turned to suspend still
	// has instances to take down, and holding it there would leave them
	// running under a Ready that says otherwise.
	case resolved.merged.Suspend && errors.As(err, &failure) && instancesAreDown(server):
		resolved.holdForSuspension = true

	default:
		return resolved, err
	}

	// After the bucket resolved, because an ObjectStorageConfig edited in
	// place keeps its name and only the location it resolves to shows the
	// move. The archive component is held with it, so the ObjectStore that
	// the recovering cluster reads keeps describing the archive it asked for.
	// The archive history is held with it too: nothing applies the location
	// the spec resolves to now, so no record of the server belongs to it.
	if hold := recoveryHoldsLocation(server, resolved.archiveLocation); hold != nil {
		resolved.holdForRecovery = hold
		resolved.holdArchive = true
		// The ObjectStore of a bucket with workload identity names no
		// identity of its own, so the hold on that object holds nothing of
		// the identity. The record is what keeps the pods on the identity
		// that reads the archive the rollback asked for.
		resolved.archive.HeldIdentity = server.Status.Recovery.Archive.Identity
	}

	// After every hold above, because a held recovery keeps the server on the
	// contract that the record names and the name is read for that contract.
	taken, err := r.contractTaken(ctx, server, resolved.merged)
	if err != nil {
		return resolved, err
	}
	resolved.contractTaken = taken

	// Here rather than beside the read of the cluster, which waits for the
	// recovery to settle status.cluster. The ObjectStore is named after the
	// server, and no recovery moves that name, so the answer is the same on
	// either side of the recovery. The recovery is what needs it first: a
	// rollback reads the archive that this ObjectStore describes, so it is
	// refused while the object belongs to somebody else.
	archiveTaken, err := r.archiveTaken(ctx, server, resolved.merged)
	if err != nil {
		return resolved, err
	}
	resolved.archiveTaken = archiveTaken

	return resolved, nil
}

// recoveryHoldsLocation reports that the archive of the server moved while a
// recovery still reads it, or nil when it did not.
//
// recoveryHoldsSpec pins the bucket contract by name, and an
// ObjectStorageConfig edited in place keeps its name, so only the location it
// resolves to shows this move.
func recoveryHoldsLocation(
	server *v1.DatabaseServer,
	location string,
) *conditions.PreCheckFailure {
	running := server.Status.Recovery
	if running == nil || running.CompletedAt != nil || running.Archive == nil {
		return nil
	}

	recorded := running.Archive.Location
	if recorded == "" || location == "" || recorded == location {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonInvalidReference,
		Message: fmt.Sprintf(
			"A rollback that %s asked for reads the archive at %q, and ObjectStorageConfig %q "+
				"names %q now. Point that ObjectStorageConfig back at the archive the rollback "+
				"reads, or wait for the rollback to finish, before the archive of the server "+
				"moves",
			running.RequestedBy, recorded, running.Archive.ObjectStorageRef, location,
		),
	}
}

// recoveryHoldsSpec reports that the spec moved something the running recovery
// depends on, or nil when it did not.
//
// A recovery is a question that one contract asked, answered out of one
// archive. Publishing another contract while it runs leaves the cluster that
// is building with nobody to answer. Pointing spec.archive at another bucket
// takes away the copy it reads, removing spec.archive takes the archive with
// it, and a shorter retentionPeriodDays prunes the base backup it starts from.
// The server keeps the contract and the whole archive block until the request
// is answered.
func recoveryHoldsSpec(
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
) *conditions.PreCheckFailure {
	running := server.Status.Recovery
	if running == nil || running.CompletedAt != nil {
		return nil
	}

	if running.Contract != "" && running.Contract != merged.DatabaseServerConfig {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"A rollback that %s asked for on DatabaseServerConfig %q is still running. Set "+
					"spec.databaseServerConfig back to that name, or wait for the rollback to "+
					"finish, before the server publishes another contract",
				running.RequestedBy, running.Contract,
			),
		}
	}

	if running.Archive == nil {
		return nil
	}

	// A record that carries no retention was written before the settings were
	// recorded, and nothing else holds them. Holding the removal against it
	// would render "0d" on the ObjectStore and publish a retention the
	// contract refuses, so the removal applies and the rollback is refused
	// instead. That takes nothing out of the bucket.
	if merged.Archive == nil && running.Archive.RetentionPeriodDays > 0 {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"A rollback that %s asked for is still running, and it reads the archive in "+
					"ObjectStorageConfig %q. The archive cannot be removed until that rollback "+
					"is answered. Put spec.archive back, or wait for the rollback to finish",
				running.RequestedBy, running.Archive.ObjectStorageRef,
			),
		}
	}

	if merged.Archive == nil {
		return nil
	}

	if running.Archive.ObjectStorageRef != merged.Archive.ObjectStorageRef {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"A rollback that %s asked for reads the archive in ObjectStorageConfig %q, which "+
					"is still running. Set spec.archive.objectStorageRef back to that name, or "+
					"wait for the rollback to finish",
				running.RequestedBy, running.Archive.ObjectStorageRef,
			),
		}
	}

	// heldArchive is the rule: whatever it puts back is what the spec moved.
	// A preset carries these settings as readily as an inline block, and a
	// shrunk retention reaches the merged spec either way.
	if held := heldArchive(running.Archive, merged.Archive); *held != *merged.Archive {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"A rollback that %s asked for is still running, and it reads the archive of this "+
					"server. A change to the retention or the schedule of the archive changes "+
					"what it holds, so the archive keeps its settings until the rollback is "+
					"answered. Set spec.archive back to retentionPeriodDays %d and "+
					"baseBackupSchedule %q, or wait for the rollback to finish",
				running.RequestedBy, held.RetentionPeriodDays, held.BaseBackupSchedule,
			),
		}
	}

	return nil
}

// heldArchive returns the archive block that a held spec renders: the bucket,
// the retention, and the schedule that status.recovery recorded. Every one of
// them comes from the record, so no edit of spec.archive reaches the archive a
// rollback reads. It is also what decides that the spec moved one of them, so
// the block the server renders and the edit it reports never disagree.
//
// A record written before the settings were recorded carries neither of them,
// and the spec fills what it still has. A removal is not held against such a
// record at all: see recoveryHoldsSpec.
func heldArchive(
	recorded *v1.RecoveryArchiveRef,
	spec *v1.DatabaseServerArchiveSpec,
) *v1.DatabaseServerArchiveSpec {
	block := v1.DatabaseServerArchiveSpec{
		ObjectStorageRef:    recorded.ObjectStorageRef,
		RetentionPeriodDays: recorded.RetentionPeriodDays,
		BaseBackupSchedule:  recorded.BaseBackupSchedule,
	}

	if spec != nil {
		if block.RetentionPeriodDays < 1 {
			block.RetentionPeriodDays = spec.RetentionPeriodDays
		}
		if block.BaseBackupSchedule == "" {
			block.BaseBackupSchedule = spec.BaseBackupSchedule
		}
	}

	return &block
}

// instancesAreDown reports whether CloudNativePG has taken the instances of the
// server down. The cluster component reports Suspended only once CloudNativePG
// has confirmed the hibernation, so a server that asked for a suspension and
// has not reached it yet is still running.
func instancesAreDown(server *v1.DatabaseServer) bool {
	condition := meta.FindStatusCondition(server.Status.Conditions, v1.ConditionClusterReady)

	return condition != nil && condition.Reason == string(component.Suspended)
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

// contractTaken says why the DatabaseServerConfig the merged spec names is not
// this server's to publish, and returns the empty string when it is: the
// object does not exist, or this server controls it.
//
// A contract that exists and carries no controller is taken, the way a
// CloudNativePG cluster of the same shape is: it is the bring-your-own-server
// API, so a person wrote it for a PostgreSQL server the operator does not run.
// The guard of the contract component reads this message, because
// component.BlockOnForeignController blocks on a controller of somebody else
// and that contract carries none.
//
// The read also reaches the holder on a pass where the block never fires: the
// contract sits behind the superuser Secret, and a blocked resource stops
// every resource after it, so a server still waiting for that Secret would
// report the wait and never the holder.
func (r *DatabaseServerReconciler) contractTaken(
	ctx context.Context,
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
) (string, error) {
	if merged.DatabaseServerConfig == "" {
		return "", nil
	}

	key := types.NamespacedName{Namespace: server.Namespace, Name: merged.DatabaseServerConfig}

	var contract v1.DatabaseServerConfig
	if err := r.APIReader.Get(ctx, key, &contract); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}

		return "", fmt.Errorf("reading DatabaseServerConfig %s: %w", key, err)
	}

	// The controller reference alone, not ownedByServer. A contract that
	// carries our reference and lost its label is ours to repair, and holding
	// it against ourselves would name this server as its own holder.
	holder := metav1.GetControllerOf(&contract)
	if holder != nil && holder.UID == server.UID {
		return "", nil
	}

	return components.ContractTakenMessage(merged.DatabaseServerConfig, holder), nil
}

// archiveTaken says why the Barman Cloud ObjectStore of the name the server
// derives is not this server's to write, and returns the empty string when it
// is: the spec asks for no archive, no object of that name exists, nothing
// controls it, or this server controls it.
//
// An ObjectStore that nothing controls is adopted, where a contract and a
// CloudNativePG cluster of the same shape are refused. It carries the location
// of an archive and the way the plugin reaches it, both of which this server
// resolves from its own ObjectStorageConfig, so the apply takes no data of
// anybody. It is also what component.BlockOnForeignController does with one,
// so a message here names a holder that the apply then writes over on the
// same pass.
//
// The read is what keeps the cluster off the bucket of another owner. The
// archive plugin entry of the cluster names this ObjectStore, so the apply of
// the cluster runs before the block on the ObjectStore is ever reached, and
// CloudNativePG writes the write-ahead log of this server through whatever
// object holds the name.
func (r *DatabaseServerReconciler) archiveTaken(
	ctx context.Context,
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
) (string, error) {
	if !components.Archiving(merged) {
		return "", nil
	}

	name := components.ObjectStoreName(server)
	key := types.NamespacedName{Namespace: server.Namespace, Name: name}

	var store barmanobjectstore.ObjectStore
	if err := r.APIReader.Get(ctx, key, &store); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}

		return "", fmt.Errorf("reading the Barman Cloud ObjectStore %s: %w", key, err)
	}

	holder := metav1.GetControllerOf(&store)
	if holder == nil || holder.UID == server.UID {
		return "", nil
	}

	return components.ArchiveTakenMessage(name, *holder), nil
}

// derivedCluster is what a live read of the CloudNativePG cluster of the name
// the server derives found.
type derivedCluster struct {
	// taken says why that cluster is not this server's to write, and it is
	// empty when the name is free or the cluster is the server's own.
	taken string
	// absent says that no object of that name exists.
	absent bool
	// outage is what CloudNativePG reports about the write-ahead log uploads
	// of that cluster, or nil when it reports nothing wrong. A cluster that
	// belongs to somebody else carries none: its uploads are not this
	// server's archive.
	outage *components.ArchiveOutage
}

// readDerivedCluster reads the CloudNativePG cluster of the name the server
// derives. Its taken message drives two things: the cluster component blocks
// the apply on it, and the other three withdraw every object of theirs that
// names that cluster. That withdrawal is a decision above one resource, and it
// is made before anything renders, so this read stays even though ocf reads
// the cluster again before each apply.
//
// The caller reads it once the recovery has settled status.cluster. A name
// read before that is the name of the cluster the server is leaving, and the
// answer then guards the wrong object for a whole pass.
//
// A cluster with no controller counts as taken, the way a contract of the
// same shape does: see components.ClusterTakenMessage.
//
// The read is live. A cached read that has not seen the cluster of the other
// owner yet lets this server apply over it, which is the write the guard
// exists to stop.
func (r *DatabaseServerReconciler) readDerivedCluster(
	ctx context.Context,
	server *v1.DatabaseServer,
) (derivedCluster, error) {
	name := components.ClusterName(server)
	key := types.NamespacedName{Namespace: server.Namespace, Name: name}

	var cluster cnpgv1.Cluster
	if err := r.APIReader.Get(ctx, key, &cluster); err != nil {
		if apierrors.IsNotFound(err) {
			return derivedCluster{absent: true}, nil
		}

		return derivedCluster{}, fmt.Errorf("reading the CloudNativePG cluster %s: %w", key, err)
	}

	controller := metav1.GetControllerOf(&cluster)
	if controller != nil && controller.UID == server.UID {
		return derivedCluster{outage: components.ArchiveOutageOf(&cluster, time.Now())}, nil
	}

	return derivedCluster{taken: components.ClusterTakenMessage(name, controller)}, nil
}

// clusterGuardReason returns why the cluster component must not apply the
// cluster of the name the server derives, or the empty string when it may.
//
// A rollback that cut over owns that name until it is answered, so a cluster
// that is gone under it stops the component instead of being built again. The
// component renders no bootstrap, so the apply would put an empty database
// under the recovered name, and completeRecovery would read the object the
// component had just built and wait for a probe that cannot come. Blocking
// leaves the name absent for the next look, which abandons the rollback and
// puts the server back on the cluster it came from.
func clusterGuardReason(server *v1.DatabaseServer, derived derivedCluster) string {
	if derived.taken != "" || !derived.absent || !cutOver(server) {
		return derived.taken
	}

	return components.RecoveryHoldsClusterMessage(components.ClusterName(server))
}

// reportedArchiveOutage returns the stop in the write-ahead log uploads that
// the server reports on ArchiveReady and on its open archive record, or nil
// when it reports on none.
//
// An unconfirmed outage is none of them. CloudNativePG raises its condition on
// one failed upload, and the plugin uploads the segment again, so a reported
// outage waits for the grace period of components.ArchiveOutageGracePeriod.
//
// A suspended server reports on none either. Its instances are gone, so it
// writes no write-ahead log to lose, and what CloudNativePG left on the
// condition describes the server that ran before the suspension. A server that
// asks for no archive is the same case: the cluster carries no archive plugin,
// and what stands on the condition is what the server archived before.
func reportedArchiveOutage(
	outage *components.ArchiveOutage,
	merged v1.DatabaseServerSpec,
) *components.ArchiveOutage {
	if outage == nil || !outage.Confirmed || merged.Suspend || !components.Archiving(merged) {
		return nil
	}

	return outage
}

// pendingArchiveOutage reports whether the write-ahead log uploads of the
// server are failing and the grace period has not passed yet. Nothing wakes
// the reconcile when it does: CloudNativePG writes that condition once and
// leaves it standing.
func pendingArchiveOutage(outage *components.ArchiveOutage, merged v1.DatabaseServerSpec) bool {
	return outage != nil && !outage.Confirmed && !merged.Suspend && components.Archiving(merged)
}

// serverVolumes are the volumes of the current cluster: one entry per
// PersistentVolumeClaim that reports a capacity, split by what the claim
// holds, and the sizes the applied CloudNativePG cluster asks for. The applied
// sizes matter on their own while the server is suspended, when the claims are
// there and report their capacity before any instance comes back.
type serverVolumes struct {
	data        []v1.VolumeStatus
	wal         []v1.VolumeStatus
	appliedData *resource.Quantity
	appliedWAL  *resource.Quantity
}

// all returns every claim of the cluster, sorted by name.
func (v serverVolumes) all() []v1.VolumeStatus {
	volumes := append(append([]v1.VolumeStatus{}, v.data...), v.wal...)
	slices.SortFunc(volumes, func(a, b v1.VolumeStatus) int { return strings.Compare(a.Name, b.Name) })

	return volumes
}

// volumeClaims reads the volumes of the current cluster. CloudNativePG labels
// every claim of a cluster with the cluster name, and it names the write-ahead
// log claim of an instance after the data claim of that instance, with the
// suffix -wal.
//
// It returns nothing at all for a cluster of that name this server does not
// own. status.Volumes and the clamp of keepAppliedStorageSize are the two
// readers, and neither has anything to say about the volumes of somebody
// else's database.
func (r *DatabaseServerReconciler) volumeClaims(
	ctx context.Context,
	server *v1.DatabaseServer,
) (serverVolumes, error) {
	key := types.NamespacedName{Namespace: server.Namespace, Name: components.ClusterName(server)}

	var cluster cnpgv1.Cluster
	applied := true
	if err := r.Get(ctx, key, &cluster); err != nil {
		if !apierrors.IsNotFound(err) {
			return serverVolumes{}, fmt.Errorf("reading the applied cluster %s: %w", key, err)
		}

		applied = false
	}

	// The name is derived, so a cluster under it can hold the database of
	// somebody else. Its claims carry the label this list selects by, so both
	// the claims and the sizes it applies belong to that database, and the
	// clamp would render this server on them.
	//
	// A name with no cluster is read as before. The claims of the cluster that
	// was there outlive it under a retain policy, and a server that must not
	// come back smaller than them is what the clamp exists for.
	if applied && !ownedByServer(server, &cluster) {
		return serverVolumes{}, nil
	}

	var claims corev1.PersistentVolumeClaimList
	if err := r.List(
		ctx, &claims,
		client.InNamespace(server.Namespace),
		client.MatchingLabels{components.CNPGClusterNameLabel: key.Name},
	); err != nil {
		return serverVolumes{}, fmt.Errorf("listing the volume claims of %q: %w", key.Name, err)
	}

	var volumes serverVolumes
	for i := range claims.Items {
		claim := &claims.Items[i]
		capacity, ok := claim.Status.Capacity[corev1.ResourceStorage]
		if !ok {
			continue
		}
		status := v1.VolumeStatus{Name: claim.Name, Capacity: capacity}
		if strings.HasSuffix(claim.Name, cnpgv1.WalArchiveVolumeSuffix) {
			volumes.wal = append(volumes.wal, status)
			continue
		}
		volumes.data = append(volumes.data, status)
	}

	if !applied {
		return volumes, nil
	}

	volumes.appliedData = parsedSize(cluster.Spec.StorageConfiguration.Size)
	if cluster.Spec.WalStorage != nil {
		volumes.appliedWAL = parsedSize(cluster.Spec.WalStorage.Size)
	}

	return volumes, nil
}

// parsedSize reads one storage size of the applied cluster, or nil when the
// field is empty or holds something that is not a quantity. Nothing of this
// operator writes such a value, and a cluster that carries one is answered by
// leaving the size out of the comparison rather than by failing the reconcile.
func parsedSize(size string) *resource.Quantity {
	quantity, err := resource.ParseQuantity(size)
	if err != nil {
		return nil
	}

	return &quantity
}

// keepAppliedStorageSize raises each merged volume size back to the size that
// is already there, and keeps a write-ahead log volume that the merged spec no
// longer asks for.
//
// It guards against the edits that admission cannot see: a preset baseline
// lowered or cleared under a server, or an inline size set below a size that a
// preset provided before. The CEL transition rules of storageSize and
// walStorageSize bind the spec of the DatabaseServer, and none of those edits
// touches it.
// CloudNativePG refuses a cluster whose storage is smaller than the one it
// applied, so a size that reaches it stops the server from converging.
func (r *DatabaseServerReconciler) keepAppliedStorageSize(
	server *v1.DatabaseServer,
	merged *v1.DatabaseServerSpec,
	volumes serverVolumes,
) {
	merged.StorageSize = r.keepAppliedSize(
		server, "storageSize", merged.StorageSize, largestVolume(volumes.data, volumes.appliedData),
	)
	merged.WALStorageSize = r.keepAppliedWALSize(
		server, merged.WALStorageSize, largestVolume(volumes.wal, volumes.appliedWAL),
	)
}

// keepAppliedWALSize returns the size to render for the write-ahead log volume
// of the server: the clamp of keepAppliedSize, and the size that is there when
// the merged spec asks for no such volume at all.
//
// CloudNativePG refuses a cluster that gives up the write-ahead log volume it
// applied, with "walStorage cannot be disabled once configured". It accepts
// one that adds it, so the field itself stays free to set.
func (r *DatabaseServerReconciler) keepAppliedWALSize(
	server *v1.DatabaseServer,
	requested, existing *resource.Quantity,
) *resource.Quantity {
	if requested != nil || existing == nil {
		return r.keepAppliedSize(server, "walStorageSize", requested, existing)
	}

	r.EventRecorder.Eventf(
		server,
		nil,
		corev1.EventTypeWarning,
		eventReasonWALStorageKept,
		eventActionResize,
		"walStorageSize is no longer set, and the write-ahead log volume of %s is already there. "+
			"Keeping it, because CloudNativePG does not take a write-ahead log volume away from "+
			"a server that has one",
		existing,
	)

	return existing
}

// keepAppliedSize returns the size to render for one volume of the server:
// requested, or existing when requested is below it. It records the Warning
// event whenever it keeps existing.
func (r *DatabaseServerReconciler) keepAppliedSize(
	server *v1.DatabaseServer,
	field string,
	requested, existing *resource.Quantity,
) *resource.Quantity {
	if requested == nil || existing == nil || requested.Cmp(*existing) >= 0 {
		return requested
	}

	r.EventRecorder.Eventf(
		server,
		nil,
		corev1.EventTypeWarning,
		eventReasonStorageShrinkIgnored,
		eventActionResize,
		"%s %s is below the existing volume size %s. Keeping %s, because PostgreSQL volumes "+
			"cannot be reduced in place",
		field,
		requested,
		existing,
		existing,
	)

	return existing
}

// largestVolume returns the largest of the claim capacities and applied, or
// nil when neither exists. It is the size that a rendered volume must not go
// below.
func largestVolume(volumes []v1.VolumeStatus, applied *resource.Quantity) *resource.Quantity {
	largest := applied
	for i := range volumes {
		if largest == nil || volumes[i].Capacity.Cmp(*largest) > 0 {
			largest = &volumes[i].Capacity
		}
	}

	return largest
}

// archiveBoundary returns the point after which a base backup belongs to the
// archive the server writes now, or nil when nothing bounds it. It is read on
// a reconcile that finds no move; one that finds a move reads no backups.
//
// Two things bound the current archive, and the later of them wins: a record
// the server closed before, and a move it recorded on an earlier reconcile.
// The second is what covers a server with no interval open, which is where an
// archive that was disabled and re-enabled elsewhere leaves it.
func archiveBoundary(server *v1.DatabaseServer, merged v1.DatabaseServerSpec) *metav1.Time {
	closed := closedArchiveEnd(server)
	if merged.Archive == nil {
		return closed
	}

	if recorded := archiveBoundaryOf(server); recorded != nil {
		return laterTime(closed, &recorded.At)
	}

	return closed
}

// archiveMoved reports whether the archive the server writes now is not the
// one it wrote before.
//
// The location decides it. A record written before the location was recorded
// is placed by the bucket contract that named it, which is all such a record
// carries: the same contract reads as the same archive, and another one as a
// move. A server that has written no archive has not moved.
func archiveMoved(server *v1.DatabaseServer, ref, location string) bool {
	previousRef, previousLocation := previousArchive(server)

	switch {
	case previousRef == "" && previousLocation == "":
		return false
	case previousLocation == "":
		return ref != "" && previousRef != ref
	default:
		return location != "" && previousLocation != location
	}
}

// previousArchive returns the bucket contract and the location of the archive
// the server wrote before this reconcile: the ones an unspent boundary marks,
// the ones the open record names, or the ones of the last record it wrote.
func previousArchive(server *v1.DatabaseServer) (ref, location string) {
	status := server.Status.Archive
	if status == nil {
		return "", ""
	}
	if status.Boundary != nil {
		return status.Boundary.ObjectStorageRef, status.Boundary.Location
	}
	if open := openArchiveRecord(server); open != nil {
		return open.ObjectStorageRef, open.Location
	}
	if last := len(status.History) - 1; last >= 0 {
		return status.History[last].ObjectStorageRef, status.History[last].Location
	}

	return "", ""
}

// archiveBoundaryOf returns the move of the archive that no record holds yet,
// or nil when the server has none.
func archiveBoundaryOf(server *v1.DatabaseServer) *v1.ArchiveBoundary {
	if server.Status.Archive == nil {
		return nil
	}

	return server.Status.Archive.Boundary
}

// laterTime returns the later of a and b, or the one that is set when the
// other is nil.
func laterTime(a, b *metav1.Time) *metav1.Time {
	if a == nil {
		return b
	}
	if b == nil || a.After(b.Time) {
		return a
	}

	return b
}

// archiveStart returns when the earliest base backup of the archive the server
// writes now completed, or nil when none has. Only a backup that the archive
// plugin took counts. It is what tells the archive component that the archive
// can be recovered from, and what opens the interval of that archive in
// status.
//
// after is the boundary of the current archive, from archiveBoundary. Only the
// backups that began after it count: the backups of an archive the server
// wrote before reach no point in the one it writes now, so treating one of
// them as the start declares a window that no restore can reach. A backup that
// recorded no start is skipped there. The first archive of a server has no
// boundary, and every completed backup counts by its end. A server that asks
// for no archive takes no base backups, so it reads none.
func (r *DatabaseServerReconciler) archiveStart(
	ctx context.Context,
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	after *metav1.Time,
) (*metav1.Time, error) {
	if merged.Archive == nil {
		return nil, nil
	}

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
		// A backup that another method or another plugin took writes nothing
		// to this archive, so it reaches no point a restore of it can use.
		if backup.Spec.Method != cnpgv1.BackupMethodPlugin ||
			backup.Spec.PluginConfiguration == nil ||
			backup.Spec.PluginConfiguration.Name != components.BarmanPluginName {
			continue
		}
		// The start decides, not the end. A backup that was already running
		// when the interval closed writes its base backup to the bucket the
		// server left: the plugin gave it that destination when it started,
		// and the one ObjectStore is rewritten in place. Its end falls after
		// the close, so an end-only test opens the new interval on an object
		// that the new bucket does not hold. A backup with no recorded start
		// sits on neither side of the boundary and goes the same way.
		//
		// That skip is a guard rather than a path a supported stack takes.
		// The Barman Cloud plugin reports the start of every backup it
		// completes, and CloudNativePG copies it into status.startedAt.
		if after != nil && (backup.Status.StartedAt == nil || !backup.Status.StartedAt.After(after.Time)) {
			continue
		}
		if earliest == nil || backup.Status.StoppedAt.Before(earliest) {
			earliest = backup.Status.StoppedAt
		}
	}

	return earliest, nil
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
		server, merged, resolved.archive, resolved.archiveTaken,
		resolved.platform, resolved.clusterBlocked,
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

// removeSupersededContracts deletes every DatabaseServerConfig that the server
// owns and no longer publishes.
//
// spec.databaseServerConfig can be renamed. The contract of the name before it
// keeps its owner reference and its pitr.recovery: operator declaration, so a
// PointInTimeRestore that resolves through it writes a request on an object
// that this controller never reads again. That restore waits for an answer
// that never comes.
//
// It runs only when the contract the spec names now is published, so a
// reconcile that could not apply it never takes the one that is there.
//
// The contract that status.recovery names is never swept. It carries the
// request while the recovery runs, and it carries the answer afterwards:
// spec.pitr.lastRecovery on that object is the only place a PointInTimeRestore
// reads the result from. Sweeping it the moment the answer lands fails a
// rollback that succeeded. It goes when the next recovery answers on another
// contract, and status.recovery names that one instead.
//
// The list is cached: the contract is owned and watched.
func (r *DatabaseServerReconciler) removeSupersededContracts(
	ctx context.Context,
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
) error {
	answering := ""
	if running := server.Status.Recovery; running != nil {
		if running.CompletedAt == nil {
			return nil
		}
		answering = running.Contract
	}

	var contracts v1.DatabaseServerConfigList
	if err := r.List(
		ctx, &contracts,
		client.InNamespace(server.Namespace),
		client.MatchingLabels{labels.DatabaseServerKey: labels.OwnerName(server.Name)},
	); err != nil {
		return fmt.Errorf("listing the contracts of %q: %w", server.Name, err)
	}

	// The replacement goes in first. The contract component blocks while the
	// superuser Secret is missing and it publishes nothing then, so a sweep
	// on that look leaves the server with no contract at all.
	if !slices.ContainsFunc(contracts.Items, func(published v1.DatabaseServerConfig) bool {
		return published.Name == merged.DatabaseServerConfig && ownedByServer(server, &published)
	}) {
		return nil
	}

	for i := range contracts.Items {
		superseded := &contracts.Items[i]
		if superseded.Name == merged.DatabaseServerConfig || superseded.Name == answering ||
			!ownedByServer(server, superseded) {
			continue
		}
		if err := r.deleteOwned(ctx, superseded); err != nil {
			return fmt.Errorf("deleting the superseded contract %q: %w", superseded.Name, err)
		}
	}

	return nil
}

// stageArchiveOutage reports on ArchiveReady that the write-ahead log of the
// server stopped reaching the bucket. The guard on the archive blocks on the
// same outage, and ocf reports that as Blocked, which reads the same as the
// wait for the first base backup. This names what CloudNativePG reports and
// what the archive still holds.
func stageArchiveOutage(server *v1.DatabaseServer, outage *components.ArchiveOutage) {
	if outage == nil {
		return
	}

	stageFailure(
		server, v1.ConditionArchiveReady, v1.ReasonArchiveFailing,
		components.ArchiveFailingMessage(outage),
	)
}

// stageTakenNames reports every derived name of the server that somebody else
// holds, on the condition of the component that writes that object.
func stageTakenNames(server *v1.DatabaseServer, resolved resolvedSpec) {
	taken := []struct {
		conditionType string
		reason        string
		message       string
	}{
		{v1.ConditionContractReady, v1.ReasonContractTaken, resolved.contractTaken},
		{v1.ConditionClusterReady, v1.ReasonClusterTaken, resolved.clusterTaken},
		{v1.ConditionArchiveReady, v1.ReasonArchiveTaken, resolved.archiveTaken},
	}

	for _, held := range taken {
		if held.message != "" {
			stageFailure(server, held.conditionType, held.reason, held.message)
		}
	}
}

// stageFailure puts a reason and a remedy on the condition of one component,
// over whatever ocf reported there. ocf answers a blocked apply with Blocked
// and the object it stopped at, which carries no remedy and reads the same as
// every other wait. The callers here name what happened and what to do about
// it, which is what the user acts on.
func stageFailure(server *v1.DatabaseServer, conditionType, reason, message string) {
	meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            conditions.BoundMessage(message),
		ObservedGeneration: server.Generation,
	})
}

// advanceArchiveFloor raises status.archive.reachableFrom to the point the
// retention period prunes the bucket to at now. A server that writes no
// archive prunes nothing, so its floor stands still.
func advanceArchiveFloor(server *v1.DatabaseServer, merged v1.DatabaseServerSpec, now metav1.Time) {
	if merged.Archive == nil {
		return
	}

	floor := metav1.NewTime(now.Add(-time.Duration(merged.Archive.RetentionPeriodDays) * day))
	if server.Status.Archive == nil {
		server.Status.Archive = &v1.DatabaseServerArchiveStatus{}
	}

	// The plugin prunes by the retention period that is in force when it runs,
	// and what it removes is gone. A raised period therefore lowers nothing.
	// The floor stays at the highest point any past period pruned to, and the
	// window widens to the new period only as the archive writes past it.
	if reached := server.Status.Archive.ReachableFrom; reached != nil && !floor.After(reached.Time) {
		return
	}

	server.Status.Archive.ReachableFrom = &floor
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
//
// A spec that moves the archive to another location closes the open record the
// same way, at the moment the archive arrives there. Each record therefore
// names one location, and a restore of that interval knows where to read it.
// With no record open, the move is recorded as status.archive.boundary
// instead, and the next record opens after it.
//
// applied says that the ObjectStore of the new location reached the API
// server. Until it does, the plugin still writes to the location the server
// came from, so the record stays open and no boundary is written: a base
// backup that completes in that window belongs to the archive that is still
// being written. The move is found again on every reconcile until one of them
// applies it.
//
// A recovery closes the record of the cluster it replaces itself, at the
// moment the contract moves. What is left here is the record of an archive
// that another cluster of this server wrote and never closed, which only an
// operator of an older version can leave behind.
func reconcileArchiveHistory(
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	archiveComp *component.Component,
	archiveStart *metav1.Time,
	location string,
	moved bool,
	applied bool,
	now metav1.Time,
) {
	if merged.Archive == nil {
		closeArchiveRecords(server, now)
		return
	}

	open := openArchiveRecord(server)
	if open != nil {
		// A record from before these fields existed names neither the bucket
		// nor the location. Nothing else can place it, so it takes what the
		// server writes to now.
		if open.ObjectStorageRef == "" {
			open.ObjectStorageRef = merged.Archive.ObjectStorageRef
		}
		// The location is adopted only into a record of the contract the
		// server writes through now. A record of another contract moved since,
		// and labelling it with the current location would call the archive it
		// holds the one the server writes.
		if open.Location == "" && open.ObjectStorageRef == merged.Archive.ObjectStorageRef {
			open.Location = location
		}
	}

	if moved {
		if !applied {
			return
		}

		// The interval of the location the server leaves ends here, at the
		// same now that archiveBoundary already gave the guard. The record of
		// the new location opens on a later look, once a base backup of it has
		// completed after this point. The boundary carries that point until
		// then, because a closed record alone cannot: an archive that was
		// disabled and re-enabled elsewhere closes no record at the move.
		if open != nil {
			open.To = &now
		}
		markArchiveBoundary(server, now, location, merged.Archive.ObjectStorageRef)

		return
	}

	if archiveStart == nil || archiveComp.GetCondition(server).Status != metav1.ConditionTrue {
		return
	}

	if open != nil {
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
	// The record holds the move from here on, so the boundary is spent.
	server.Status.Archive.Boundary = nil
	server.Status.Archive.History = append(server.Status.Archive.History, v1.ArchiveRecord{
		ServerName:       components.ClusterName(server),
		ObjectStorageRef: merged.Archive.ObjectStorageRef,
		Location:         location,
		From:             *archiveStart,
	})
}

// markArchiveBoundary records that the archive of the server moved to location
// at now, with no interval open to hold the move.
func markArchiveBoundary(server *v1.DatabaseServer, now metav1.Time, location, bucket string) {
	if server.Status.Archive == nil {
		server.Status.Archive = &v1.DatabaseServerArchiveStatus{}
	}

	server.Status.Archive.Boundary = &v1.ArchiveBoundary{
		At:               now,
		Location:         location,
		ObjectStorageRef: bucket,
	}
}

// markArchiveOutage records on the open archive record the point from which
// the archive can be missing write-ahead log, and clears the point once the
// uploads run again. The plugin uploads the segments it held back then, so
// every point of the interval can be reached again.
func markArchiveOutage(server *v1.DatabaseServer, outage *components.ArchiveOutage) {
	open := openArchiveRecord(server)
	if open == nil {
		return
	}

	if outage == nil {
		open.UnverifiedFrom = nil

		return
	}

	open.UnverifiedFrom = outage.Since.DeepCopy()
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
// open. Only the drop of spec.archive calls it. The server then writes no
// archive at all, so every open interval is over, including one that another
// cluster of this server left open.
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

// closeArchiveRecord ends the interval of the archive that serverName has
// open, and leaves every other record alone. A recovery closes the archive of
// the cluster it replaces, and the cluster it built can already have opened one
// of its own by then: its first base backup completes whenever CloudNativePG
// takes it, before or after the contract moves.
func closeArchiveRecord(server *v1.DatabaseServer, serverName string, at metav1.Time) {
	if server.Status.Archive == nil {
		return
	}

	for i := range server.Status.Archive.History {
		record := &server.Status.Archive.History[i]
		if record.ServerName == serverName && record.To == nil {
			record.To = &at
		}
	}
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
