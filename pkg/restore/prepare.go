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

package restore

import (
	"context"
	"fmt"
	"regexp"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// A restore writes two fields on the cluster it restores: the suspension it
// needs for its whole run, and the Camunda version that its backup was taken
// with. Each field carries a field manager of its own, so one apply is a
// statement about one field and about nothing else. A single manager that
// applied both would remove whichever of the two an apply left out.
//
// The names are the contract. A GitOps tool that owns the CamundaCluster, and
// the layer above this operator, both read them to tell a write of a restore
// from a write of their own.
const (
	// FieldManagerTargetSuspend owns spec.suspend of the cluster that a
	// restore prepares. The restore withdraws the field when it completes.
	FieldManagerTargetSuspend client.FieldOwner = "camunda-operator/restore-suspend"
	// FieldManagerTargetVersion owns spec.version of the cluster that a
	// restore prepares. The restore keeps the field: the cluster runs the
	// version of the backup from then on.
	FieldManagerTargetVersion client.FieldOwner = "camunda-operator/restore-version"
)

// versionPattern is the shape that spec.version of a CamundaCluster accepts.
// A backup that recorded anything else names no version that the restore can
// write, and the version rule of the restore kind reports what that means.
var versionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// PrepareInput is what the preparation step of a restore reads. Every value
// is live, read in this look: the step decides from the state of the cluster
// now, and it writes to that cluster.
type PrepareInput struct {
	// Owner is the restore resource. The step stages its Ready condition on
	// it while it works.
	Owner conditions.Owner
	// Cluster is the target cluster of the restore.
	Cluster *v1.CamundaCluster
	// Target is the live broker StatefulSet and the facts read off it. The
	// step reads the running broker count and the Camunda version from it,
	// because both answer what the cluster does rather than what it was
	// asked to do.
	Target *Target
	// Version is the Camunda version that the backup recorded. The step
	// writes it on the cluster, so the brokers start again on the version
	// whose state the volumes hold by then.
	//
	// An empty value, and a value that is not of the form x.y.z, write
	// nothing. Such a backup breaks the version rule of its kind, and that
	// rule reports it. A PointInTimeRestore reads no backup at all and
	// passes nothing.
	Version string
	// Poll paces a step that waits for the cluster to converge.
	Poll time.Duration
}

// Prepare carries the cluster of a restore to the state that the restore
// needs, and reports Done once the cluster is there. It suspends the cluster,
// waits until the brokers are gone, and sets the Camunda version of the
// backup. The caller runs it during admission, before it leaves the phase in
// which the restore has destroyed nothing.
//
// The order is the safety property, and it is what makes a downgrade safe.
// Camunda does not support a running cluster that moves backwards: a broker
// compares the version in its data directory against its own binary at
// startup, reports a downgrade, and applies no migration. No broker performs
// that comparison here. The cluster is suspended before the version changes,
// so nothing runs while it changes, and the volumes are erased before a
// broker of the older version starts. The first broker that starts again
// finds the state of the backup at the version of the backup.
//
// The suspension is withdrawn by Resume, and only when this restore is what
// applied it. The version is not withdrawn: the cluster keeps running the
// version of the backup, which is the point of writing it. The next apply or
// GitOps sync of the cluster takes the field back, which is correct.
//
// The step is idempotent, so a caller that re-enters it repeats no write.
func Prepare(
	ctx context.Context,
	c client.Client,
	p *v1.RestoreProgress,
	in PrepareInput,
) (Outcome, error) {
	if err := in.Target.complete(); err != nil {
		return failure(fmt.Sprintf("the restore cannot prepare its cluster: %s", err)), nil
	}

	key := client.ObjectKeyFromObject(in.Cluster)

	if !in.Cluster.Spec.Suspend {
		return suspendTarget(ctx, c, p, in, key)
	}

	// spec.suspend says what was asked for. The StatefulSet says what
	// happened. A version that reaches the brokers while they still run is
	// the downgrade of a running cluster that this order exists to avoid.
	if running := in.Target.StatefulSet.Status.Replicas; running != 0 {
		progressing(in.Owner, fmt.Sprintf(
			"CamundaCluster %s is suspended, and %d of its brokers still run. The restore waits "+
				"for them to stop", key, running,
		))

		return Outcome{Wait: in.Poll}, nil
	}

	return versionTarget(ctx, c, in, key)
}

// suspendTarget suspends the cluster of the restore. It records that this
// restore is the one that suspended it, and it lets the record become durable
// before it writes.
//
// The order is what keeps the cluster recoverable. A crash between the write
// and the record would leave a suspended cluster that no restore ever
// unsuspends again, because Resume reads the record to decide.
func suspendTarget(
	ctx context.Context,
	c client.Client,
	p *v1.RestoreProgress,
	in PrepareInput,
	key types.NamespacedName,
) (Outcome, error) {
	if !p.ClusterSuspended {
		p.ClusterSuspended = true
		progressing(in.Owner, fmt.Sprintf(
			"the restore suspends CamundaCluster %s, and unsuspends it again when it completes", key,
		))

		return Outcome{Wait: Shortly}, nil
	}

	if err := applySuspend(ctx, c, key, in.Cluster.UID, true); err != nil {
		return Outcome{}, err
	}
	progressing(in.Owner, fmt.Sprintf(
		"the restore suspended CamundaCluster %s. It waits for the brokers to stop", key,
	))

	return Outcome{Wait: in.Poll}, nil
}

// versionTarget sets the Camunda version of the backup on the cluster and
// waits until the brokers carry it.
//
// The two comparisons answer two questions. spec.version says which version
// the cluster was asked to run, and it decides whether the step still has a
// write to make. The tag of the broker image says which version a broker
// would really start on, and it alone decides that the cluster converged: a
// cluster that takes its version from a preset carries no spec.version at
// all, and a spec.version that the cluster controller has not rolled out yet
// says nothing about a broker. The tag is also what the restore Jobs run,
// because they copy the broker image.
//
// Both are needed. A cluster that is part way through an upgrade carries the
// newer version in spec.version and the older one on its image. Reading the
// image alone would call that cluster converged, and the cluster controller
// would then roll the newer image in under the restore.
//
// A version that has not converged yet is a wait, not a failure. The restore
// has destroyed nothing at this point, so the wait costs nothing.
func versionTarget(
	ctx context.Context,
	c client.Client,
	in PrepareInput,
	key types.NamespacedName,
) (Outcome, error) {
	if !versionPattern.MatchString(in.Version) {
		return Outcome{Done: true}, nil
	}

	if in.Cluster.Spec.Version != in.Version {
		if err := applyVersion(ctx, c, key, in.Cluster.UID, in.Version); err != nil {
			return Outcome{}, err
		}
		progressing(in.Owner, fmt.Sprintf(
			"the restore set CamundaCluster %s to Camunda %s, the version its backup was taken "+
				"with", key, in.Version,
		))

		return Outcome{Wait: in.Poll}, nil
	}

	if in.Target.Version != in.Version {
		progressing(in.Owner, fmt.Sprintf(
			"CamundaCluster %s moves to Camunda %s, the version its backup was taken with. Its "+
				"brokers carry %s", key, in.Version, in.Target.Version,
		))

		return Outcome{Wait: in.Poll}, nil
	}

	return Outcome{Done: true}, nil
}

// Resume withdraws the suspension that this restore applied to its cluster.
// It writes nothing unless the restore recorded that it suspended the
// cluster, so a cluster that its owner suspended for reasons of their own
// stays suspended.
//
// Only a restore that reached its Completed phase calls it. A failed restore
// leaves the cluster suspended: its broker volumes can be empty or half
// written, and brokers that start over them are worse than a cluster that is
// down.
//
// The withdrawal applies an object without spec.suspend, so server-side apply
// removes the field that this manager owns and leaves the value of any other
// manager in place. The cluster is read first, and the apply carries its UID:
// without it, an apply against a cluster that is already gone would put an
// empty CamundaCluster in its place.
func Resume(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	p *v1.RestoreProgress,
	cluster types.NamespacedName,
) error {
	if !p.ClusterSuspended {
		return nil
	}

	var existing v1.CamundaCluster
	if err := reader.Get(ctx, cluster, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}

		return fmt.Errorf("reading CamundaCluster %s: %w", cluster, err)
	}

	if err := applySuspend(ctx, c, cluster, existing.UID, false); err != nil {
		// The cluster went between the read and the write, which is the one
		// thing the UID precondition refuses. Such a cluster needs no
		// withdrawal.
		if apierrors.IsConflict(err) {
			return nil
		}

		return err
	}

	return nil
}

// applySuspend applies spec.suspend of the cluster under the suspend manager.
// Suspend is omitempty, so a false value applies an object that carries no
// spec.suspend, which is the withdrawal.
func applySuspend(
	ctx context.Context,
	c client.Client,
	cluster types.NamespacedName,
	uid types.UID,
	suspend bool,
) error {
	patch := targetPatch(cluster, uid, v1.CamundaClusterSpec{Suspend: suspend})
	if err := Apply(ctx, c, patch, FieldManagerTargetSuspend); err != nil {
		return fmt.Errorf("setting spec.suspend of CamundaCluster %s to %t: %w", cluster, suspend, err)
	}

	return nil
}

// applyVersion applies spec.version of the cluster under the version manager.
func applyVersion(
	ctx context.Context,
	c client.Client,
	cluster types.NamespacedName,
	uid types.UID,
	version string,
) error {
	patch := targetPatch(cluster, uid, v1.CamundaClusterSpec{Version: version})
	if err := Apply(ctx, c, patch, FieldManagerTargetVersion); err != nil {
		return fmt.Errorf("setting spec.version of CamundaCluster %s to %s: %w", cluster, version, err)
	}

	return nil
}

// targetPatch returns the apply object of one field of a cluster: a
// CamundaCluster that carries its own identity and the given spec, and
// nothing else. Every other field of the cluster stays with whoever owns it.
//
// The apply carries uid as a precondition. Server-side apply creates the
// object that it does not find, so an apply against a cluster that went
// between the read and the write would otherwise put an empty CamundaCluster
// in its place.
func targetPatch(
	cluster types.NamespacedName,
	uid types.UID,
	spec v1.CamundaClusterSpec,
) *v1.CamundaCluster {
	return &v1.CamundaCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaCluster"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
			UID:       uid,
		},
		Spec: spec,
	}
}
