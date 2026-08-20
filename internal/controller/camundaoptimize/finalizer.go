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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// finalize withdraws the exporter patch and releases the finalizer. The
// workloads, their Services and ServiceMonitors, and the copies of referenced
// Secrets carry an owner reference, so Kubernetes collects them; only the
// patch on the cluster is outside that chain.
//
// Only the CamundaOptimize that holds the attachment withdraws. Every
// CamundaOptimize applies the patch under the same field manager, so a
// deletion of one that never applied it would empty-apply that manager and
// strip the settings the holder still needs.
//
// The withdrawal goes first. Once the finalizer is gone the object is gone for
// good, and no retry can turn the exporter off. The cluster would then keep
// writing records that nothing reads.
func (r *Reconciler) finalize(ctx context.Context, optimize *v1.CamundaOptimize) error {
	if !controllerutil.ContainsFinalizer(optimize, Finalizer) {
		return nil
	}

	holds, err := r.holdsAttachment(ctx, optimize)
	if err != nil {
		return err
	}

	cluster := client.ObjectKey{Namespace: optimize.Namespace, Name: optimize.Spec.ClusterRef.Name}
	if holds {
		if err := r.withdrawExporter(ctx, cluster); err != nil {
			return err
		}
		r.EventRecorder.Eventf(
			optimize,
			nil,
			corev1.EventTypeNormal,
			eventReasonExporterRemoved,
			eventActionFinalize,
			"Removed the Elasticsearch exporter settings from CamundaCluster %q",
			cluster.Name,
		)
	}

	controllerutil.RemoveFinalizer(optimize, Finalizer)
	if err := r.Update(ctx, optimize); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("removing the finalizer: %w", err)
	}

	return nil
}
