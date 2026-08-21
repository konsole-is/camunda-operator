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

package camundaoptimize

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
)

// patchExporter turns the Elasticsearch exporter of the referenced cluster on.
// The apply carries only spec.zeebe.extraEnv, under this controller's own
// field manager, so the entries sit next to the entries of the user and of any
// other manager. ForceOwnership takes an entry that another manager applied
// first: the value the operator computes wins, and only for the entries it
// names.
func (r *Reconciler) patchExporter(ctx context.Context, res resolved) error {
	env := components.ExporterEnv(res.ExporterStorage)

	return r.applyExporterPatch(ctx, res.ClusterKey, res.ClusterUID, env)
}

// withdrawExporter removes the exporter entries of this controller from the
// cluster. It applies the same object with no entries, so server-side apply
// drops what this field manager owns and leaves every other entry alone.
//
// The cluster is read first, and the apply carries its UID. Server-side apply
// creates the object it does not find, so an apply against a cluster that is
// already gone would put an empty CamundaCluster back in its place. The read
// alone does not stop that, because the cluster can go between the read and
// the apply. The UID makes the apply fail instead, and a cluster that is gone
// needs no withdrawal.
func (r *Reconciler) withdrawExporter(ctx context.Context, cluster client.ObjectKey) error {
	var existing v1.CamundaCluster
	if err := r.APIReader.Get(ctx, cluster, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("reading CamundaCluster %q: %w", cluster, err)
	}

	err := r.applyExporterPatch(ctx, cluster, existing.UID, nil)
	if apierrors.IsConflict(err) {
		return nil
	}

	return err
}

// applyExporterPatch applies the minimal CamundaCluster object that carries
// env under this controller's field manager.
//
// The apply carries uid as a precondition. Without it the apply creates a
// CamundaCluster that holds nothing but these entries, whenever the real one
// goes between the read that found it and this write.
func (r *Reconciler) applyExporterPatch(
	ctx context.Context,
	cluster client.ObjectKey,
	uid types.UID,
	env []corev1.EnvVar,
) error {
	patch := components.ExporterPatch(cluster, uid, env)

	//nolint:staticcheck // the repository applies through the deprecated client.Apply patch
	if err := r.Patch(
		ctx, patch, client.Apply,
		client.FieldOwner(components.ExporterFieldManager), client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("patching the exporter settings of CamundaCluster %q: %w", cluster, err)
	}

	return nil
}
