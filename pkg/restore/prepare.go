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
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// A restore writes two fields on its cluster, each under a field manager of
// its own. A single manager that applied both would remove whichever field an
// apply left out.
//
// The names are API surface. A GitOps tool and the layer above this operator
// read them to tell a write of a restore from a write of their own.
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
var versionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

// PrepareInput is what the preparation step of a restore reads. Read every
// value live, in the same look, because the step writes to the cluster.
type PrepareInput struct {
	// Owner is the restore resource. The step stages its Ready condition on
	// it while it works.
	Owner conditions.Owner
	// Cluster is the target cluster of the restore.
	Cluster *v1.CamundaCluster
	// Target is the live broker StatefulSet and the facts read off it.
	Target *Target
	// Version is the Camunda version that the backup recorded. The step
	// writes it on the cluster. A value that WritesVersion rejects writes
	// nothing.
	Version string
	// Poll paces a step that waits for the cluster to converge.
	Poll time.Duration
}

// Prepare carries the cluster of a restore to the state that the restore needs,
// and reports Done once the cluster is there: it carries the suspension hold of
// the restore, it is suspended with its brokers gone, and its broker
// StatefulSet carries the Camunda version of the backup. A re-entry repeats no
// write.
//
// Add HoldFinalizer to the restore first. Run Prepare during admission, before
// the restore destroys anything.
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

	if err := holdTarget(ctx, c, in.Owner, in.Cluster); err != nil {
		return Outcome{}, err
	}

	if !in.Cluster.Spec.Suspend {
		return suspendTarget(ctx, c, p, in, key)
	}

	// A failed or deleted restore leaves the cluster suspended. The remedy is
	// a new restore, which must give that suspension back when it finishes.
	// The field manager tells a suspension of a restore from one of the owner,
	// and no restore adopts the suspension of the owner.
	if !p.ClusterSuspended && suspendedByARestore(in.Cluster) {
		p.ClusterSuspended = true
		progressing(in.Owner, fmt.Sprintf(
			"the restore took over the suspension that an earlier restore left on CamundaCluster "+
				"%s, and it gives that suspension back when it completes", key,
		))

		return Outcome{Wait: Shortly}, nil
	}

	// spec.suspend does not show that the brokers stopped. A version that
	// reaches running brokers downgrades a running cluster.
	if running := in.Target.StatefulSet.Status.Replicas; running != 0 {
		progressing(in.Owner, fmt.Sprintf(
			"CamundaCluster %s is suspended, and %d of its brokers still run. The restore waits "+
				"for them to stop", key, running,
		))

		return Outcome{Wait: in.Poll}, nil
	}

	return versionTarget(ctx, c, in, key)
}

// suspendTarget makes its record durable before it writes.
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

// suspendedByARestore also answers true when another manager co-owns
// spec.suspend. That is safe: server-side apply keeps a field that another
// manager still declares, so a withdrawal leaves its value in place.
func suspendedByARestore(cluster *v1.CamundaCluster) bool {
	for _, entry := range cluster.ManagedFields {
		if entry.Manager != string(FieldManagerTargetSuspend) || entry.FieldsV1 == nil {
			continue
		}
		if declaresSuspend(entry.FieldsV1.GetRawBytes()) {
			return true
		}
	}

	return false
}

// declaresSuspend reads the fieldsV1 encoding, where each field name carries
// the "f:" prefix. An entry that does not parse names nothing.
func declaresSuspend(raw []byte) bool {
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		return false
	}

	spec, ok := fields["f:spec"]
	if !ok {
		return false
	}

	var inSpec map[string]json.RawMessage
	if err := json.Unmarshal(spec, &inSpec); err != nil {
		return false
	}
	_, ok = inSpec["f:suspend"]

	return ok
}

// versionTarget waits on the broker StatefulSet, not on spec.version: a cluster
// can take its version from a preset and have no spec.version, and a cluster
// part way through an upgrade declares a version that its StatefulSet does not
// carry yet.
//
// The apply carries the UID of the cluster and no resource version. The status
// of the cluster moves on nearly every reconcile, so an apply with a resource
// version would never land. The gap is safe: a broker of Camunda 8.9 refuses a
// snapshot that a newer version wrote, before it processes anything.
func versionTarget(
	ctx context.Context,
	c client.Client,
	in PrepareInput,
	key types.NamespacedName,
) (Outcome, error) {
	if !WritesVersion(in.Version) {
		return Outcome{Done: true}, nil
	}

	// A cluster mid-upgrade can carry the backup version on its StatefulSet and a
	// newer spec.version, which its controller would roll in under the restore. A
	// pruned annotation leaves the declared version without its sanction, and the
	// cluster controller then refuses the move that the restore waits on.
	sanctioned := in.Cluster.Annotations[components.AllowVersionDowngradeAnnotation]
	if in.Cluster.Spec.Version != in.Version ||
		(in.Target.Version != in.Version && sanctioned != in.Version) {
		// A manifest that leaves spec.version out takes nothing back: server-side
		// apply removes a field only from the manager that declared it.
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

// MovedVersion returns a failure when the broker StatefulSet no longer carries
// the Camunda version of the backup. It returns nil when it does, and when
// WritesVersion rejects the backup version. Ask it on every look after
// admission: another manager can move spec.version while the restore runs.
func MovedVersion(backupVersion, targetVersion string) *conditions.PreCheckFailure {
	if !WritesVersion(backupVersion) || targetVersion == backupVersion {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonIncompatibleTarget,
		Message: fmt.Sprintf(
			"the brokers of the target carry Camunda %s and the backup was taken with %s. The "+
				"restore set the version of the backup on the cluster before it started, so "+
				"another manager moved it while the restore ran",
			targetVersion, backupVersion,
		),
	}
}

// WritesVersion reports whether a restore writes version on the cluster it
// prepares: only a value of the form x.y.z. Ask it before you hold a cluster to
// the version of its backup, because nothing brings about any other value.
func WritesVersion(version string) bool {
	return versionPattern.MatchString(version)
}

// Resume gives the cluster back once the restore completed: it removes the
// suspension hold of the restore, and withdraws spec.suspend when the restore
// recorded that it suspended the cluster. A failed restore keeps both its hold
// and the suspension. Call it only after CollectJobs reports Done, because the
// pods of the Jobs hold the broker volumes.
func Resume(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	owner client.Object,
	p *v1.RestoreProgress,
	cluster types.NamespacedName,
) error {
	// The broker volumes of a failed restore can be half written, and brokers
	// that start over them are worse than a cluster that is down.
	if p.TerminalReason != v1.ReasonCompleted {
		return nil
	}
	if err := releaseHold(ctx, c, reader, owner, cluster); err != nil {
		return err
	}
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
	if existing.UID != p.TargetClusterUID {
		return nil
	}
	// The terminal branch runs on every event of the restore and of its
	// cluster, so an apply that changes nothing here repeats many times.
	if !existing.Spec.Suspend {
		return nil
	}

	if err := applySuspend(ctx, c, cluster, existing.UID, false); err != nil {
		// The UID precondition refuses only a cluster that went between the
		// read and the write, and such a cluster needs no withdrawal.
		if apierrors.IsConflict(err) {
			return nil
		}

		return err
	}

	return nil
}

// applySuspend withdraws the field for false: Suspend is omitempty, so the
// apply carries no spec.suspend.
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

// applyVersion writes the downgrade sanction in the same apply, so no crash
// leaves the version without it. The cluster controller refuses a downgrade
// without the sanction, and removes it once the brokers carry the version.
func applyVersion(
	ctx context.Context,
	c client.Client,
	cluster types.NamespacedName,
	uid types.UID,
	version string,
) error {
	patch := targetPatch(cluster, uid, v1.CamundaClusterSpec{Version: version})
	patch.Annotations = map[string]string{components.AllowVersionDowngradeAnnotation: version}
	if err := Apply(ctx, c, patch, FieldManagerTargetVersion); err != nil {
		return fmt.Errorf("setting spec.version of CamundaCluster %s to %s: %w", cluster, version, err)
	}

	return nil
}

// targetPatch carries uid as a precondition. Server-side apply creates an
// object that it does not find, so without it an apply against a deleted
// cluster puts an empty CamundaCluster in its place.
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
