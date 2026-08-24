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

package camundamanagementcluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

const (
	// eventActionReplacePing is the action of the events about the Console
	// ping settings that the management plane writes on an attached cluster.
	eventActionReplacePing = "ReplaceConsolePing"
	// eventReasonPingEntryRemoved is recorded when the management plane
	// removes an entry of spec.extraEnv that holds a ping setting with
	// valueFrom.
	eventReasonPingEntryRemoved = "ConsolePingEntryRemoved"
)

// jsonPatchOp is one operation of an RFC 6902 JSON patch.
type jsonPatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value,omitempty"`
}

// syncPing points every attached cluster at the Console of mc and withdraws
// the ping settings from every other cluster: one that left the selector, one
// the management plane cannot serve, and every cluster of a management plane
// that deploys no Console.
//
// The API server refuses a ping when the cluster changed or went. That is a
// row in status.clusters of the cluster, not a failed reconcile. What one
// cluster answers must not hold back the rest of the management plane. Every
// other failure is returned, so the reconcile retries it with backoff.
func (r *Reconciler) syncPing(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	clusters []v1.CamundaCluster,
	attached []components.AttachedCluster,
) error {
	consoleURL := components.ConsoleServiceURL(mc)
	served := map[client.ObjectKey]string{}
	if mc.Spec.Console != nil {
		for _, cluster := range attached {
			served[client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name}] = cluster.Version
		}
	}

	var errs []error
	for i := range clusters {
		cluster := &clusters[i]
		key := client.ObjectKeyFromObject(cluster)

		version, serve := served[key]
		if !serve {
			// The claim is already withdrawn from a deselected cluster, so the
			// entries themselves are what says the ping is still there.
			if !components.PingsConsole(cluster.Spec.ExtraEnv, consoleURL) {
				continue
			}
			if err := r.withdrawPingFrom(ctx, cluster); err != nil {
				return err
			}
			continue
		}

		env := components.PingEnv(consoleURL, cluster.Name, version)
		err := r.replacePing(ctx, mc, cluster, version, env)
		if apierrors.IsConflict(err) || apierrors.IsNotFound(err) {
			reportPingFailure(mc, key, err)
			continue
		}
		if err != nil {
			// The row must not say attached while the ping is not written:
			// Console does not list a cluster whose ping never reached it.
			reportPingFailure(mc, key, err)
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// withdrawPing removes the ping settings of mc from every orchestration
// cluster. The finalizer calls it, so a deleted management cluster leaves no
// cluster reporting to a Console that is gone.
func (r *Reconciler) withdrawPing(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	clusters []v1.CamundaCluster,
) error {
	return r.syncPing(ctx, mc, clusters, nil)
}

// withdrawPingFrom removes the ping settings from one cluster. A cluster that
// went between the list and the apply needs no withdrawal.
func (r *Reconciler) withdrawPingFrom(ctx context.Context, cluster *v1.CamundaCluster) error {
	err := r.applyPing(ctx, cluster, nil)
	if apierrors.IsConflict(err) || apierrors.IsNotFound(err) {
		return nil
	}

	return err
}

// replacePing writes the ping settings of mc on one attached cluster. An
// entry that another field manager set with valueFrom under one of those
// names goes first: the apply of the operator cannot replace it.
func (r *Reconciler) replacePing(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	cluster *v1.CamundaCluster,
	version string,
	env []corev1.EnvVar,
) error {
	if err := r.removePingCollisions(ctx, mc, cluster, version); err != nil {
		return err
	}

	return r.applyPing(ctx, cluster, env)
}

// removePingCollisions deletes every entry of spec.extraEnv of cluster that
// holds a ping setting of the key set that version reads, with valueFrom, in
// one JSON patch, and records each removal as an event on mc. A cluster that carries no such entry is left
// alone, so the reconcile after a replacement patches nothing.
//
// The patch tests the UID of the cluster and the name at every index it
// deletes. A cluster that went, and a list that moved between the read and
// the patch, therefore refuse the patch instead of deleting the entry of
// somebody else.
func (r *Reconciler) removePingCollisions(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	cluster *v1.CamundaCluster,
	version string,
) error {
	collisions := components.PingCollisions(cluster, version)
	if len(collisions) == 0 {
		return nil
	}

	key := client.ObjectKeyFromObject(cluster)
	ops := []jsonPatchOp{{Op: "test", Path: "/metadata/uid", Value: cluster.UID}}
	for _, collision := range collisions {
		path := "/spec/extraEnv/" + strconv.Itoa(collision.Index)
		ops = append(
			ops,
			jsonPatchOp{Op: "test", Path: path + "/name", Value: collision.Name},
			jsonPatchOp{Op: "remove", Path: path},
		)
	}

	body, err := json.Marshal(ops)
	if err != nil {
		return fmt.Errorf("building the removal of the colliding ping entries of CamundaCluster %q: %w", key, err)
	}

	patch := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: cluster.Name, Namespace: cluster.Namespace},
	}
	if err := r.Patch(ctx, patch, client.RawPatch(types.JSONPatchType, body)); err != nil {
		return fmt.Errorf("removing the colliding ping entries of CamundaCluster %q: %w", key, err)
	}

	for _, collision := range collisions {
		r.EventRecorder.Eventf(
			mc,
			nil,
			corev1.EventTypeWarning,
			eventReasonPingEntryRemoved,
			eventActionReplacePing,
			"Removed the spec.extraEnv entry %q of CamundaCluster %q. %s set it with valueFrom. "+
				"The management plane owns the ping settings and writes its own value under that name",
			collision.Name, key, managerName(collision.Manager),
		)
	}

	return nil
}

// managerName names the field manager of a collision for an event note.
func managerName(manager string) string {
	if manager == "" {
		return "Another field manager"
	}

	return fmt.Sprintf("The field manager %q", manager)
}

// applyPing applies the minimal CamundaCluster object that carries env in the
// top-level spec.extraEnv under the ping field manager. Nil entries remove
// what that manager owns and leave every other entry alone. ForceOwnership
// takes an entry that another manager applied first: the value the operator
// computes wins, and only for the entries it names.
//
// The apply carries the UID of the cluster as a precondition. Server-side
// apply creates the object it does not find, so without it an apply against a
// cluster that went in the meantime would put an empty CamundaCluster in its
// place.
func (r *Reconciler) applyPing(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	env []corev1.EnvVar,
) error {
	patch := &v1.CamundaCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaCluster"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name,
			Namespace: cluster.Namespace,
			UID:       cluster.UID,
		},
		Spec: v1.CamundaClusterSpec{ExtraEnv: env},
	}

	//nolint:staticcheck // the repository applies through the deprecated client.Apply patch
	if err := r.Patch(
		ctx, patch, client.Apply,
		client.FieldOwner(components.PingFieldManager), client.ForceOwnership,
	); err != nil {
		return fmt.Errorf(
			"applying the Console ping settings on CamundaCluster %q: %w",
			client.ObjectKeyFromObject(cluster), err,
		)
	}

	return nil
}

// reportPingFailure records a refused ping in the status row of one cluster.
// Console lists a cluster only once the cluster reports to it, so a cluster
// that carries no ping is not attached.
func reportPingFailure(mc *v1.CamundaManagementCluster, cluster client.ObjectKey, err error) {
	for i := range mc.Status.Clusters {
		row := &mc.Status.Clusters[i]
		if row.Name != cluster.Name || row.Namespace != cluster.Namespace {
			continue
		}
		row.Attached = false
		row.Reason = v1.ReasonWriteFailed
		row.Message = conditions.BoundMessage(err.Error())

		return
	}
}
