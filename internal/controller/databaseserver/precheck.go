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

package databaseserver

import (
	"context"
	"errors"
	"fmt"
	"time"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

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

// preCheck resolves the preset and the release, validates the merged spec, and
// resolves the archive bucket. A failed check returns a
// *conditions.PreCheckFailure that carries its Ready reason. An operator that
// started without the CloudNativePG CRDs reports CNPGNotInstalled, and a server
// that asks for an archive on a cluster without the plugin CRD reports
// BarmanPluginNotInstalled. A dangling presetRef or releaseRef, an incomplete
// merge, a version below the floor, and a bucket the plugin cannot address all
// report InvalidReference. Any other error is a transient API failure.
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

	var presetSpec *v1.DatabaseServerPresetSpec
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
		presetSpec = &preset.Spec
	}

	var releaseSpec *v1.CamundaReleaseSpec
	if server.Spec.ReleaseRef != "" {
		var release v1.CamundaRelease
		if err := r.APIReader.Get(ctx, types.NamespacedName{Name: server.Spec.ReleaseRef}, &release); err != nil {
			if apierrors.IsNotFound(err) {
				return resolved, &conditions.PreCheckFailure{
					Reason:  v1.ReasonInvalidReference,
					Message: fmt.Sprintf("CamundaRelease %q not found", server.Spec.ReleaseRef),
				}
			}
			return resolved, fmt.Errorf("resolving release %q: %w", server.Spec.ReleaseRef, err)
		}
		releaseSpec = &release.Spec
	}

	resolved.merged = components.MergeSpec(server.Spec, presetSpec, releaseSpec)

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

	archive, err := r.resolveArchiveStorage(ctx, server.Namespace, resolved.merged)
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

	// The cluster component blocks on this account, but a rollback builds its
	// cluster outside the component, so the recovery reads it here.
	serviceAccountTaken, err := r.serviceAccountTaken(ctx, server)
	if err != nil {
		return resolved, err
	}
	resolved.serviceAccountTaken = serviceAccountTaken

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
// Secret. The bucket is read from namespace, the namespace of the server. It
// returns nil when the spec names no bucket, which means the server has no
// archive.
//
// A reference that does not resolve is a pre-check failure, not an error: the
// contract, or the Secret it names, can appear later, and both are watched.
func (r *DatabaseServerReconciler) resolveArchiveStorage(
	ctx context.Context,
	namespace string,
	merged v1.DatabaseServerSpec,
) (*components.ArchiveStorage, error) {
	if merged.Archive == nil {
		return nil, nil
	}

	bucketKey := types.NamespacedName{Namespace: namespace, Name: merged.Archive.ObjectStorageRef}

	// The cached client: the type is watched, so the cache is current, and
	// every bucket event lands here again anyway.
	var config v1.ObjectStorageConfig
	if err := r.Get(ctx, bucketKey, &config); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &conditions.PreCheckFailure{
				Reason:  v1.ReasonInvalidReference,
				Message: fmt.Sprintf("ObjectStorageConfig %s not found", bucketKey),
			}
		}

		return nil, fmt.Errorf("resolving archive storage %s: %w", bucketKey, err)
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

// readDerivedCluster reads the CloudNativePG cluster of the name the server
// derives. Its taken message drives two things: the cluster component blocks
// the apply on it, and the other three withdraw every object of theirs that
// names that cluster, except as contractWithdrawalReason says. That
// withdrawal is a decision above one resource, and it is made before anything
// renders, so this read stays even though ocf reads the cluster again before
// each apply.
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

// contractWithdrawalReason returns why the contract component withdraws the
// contract, or the empty string when the contract stays. A rollback that cut
// over keeps it until the rollback is answered.
func contractWithdrawalReason(server *v1.DatabaseServer, derived derivedCluster) string {
	// The answer goes on the contract. An owner can take the cluster after the
	// recovery read it, and a withdrawn contract then leaves the rollback with
	// nothing to answer on, so it is never abandoned.
	if cutOver(server) {
		return ""
	}

	return derived.taken
}
