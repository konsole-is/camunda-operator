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

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// attachmentHolder returns the CamundaOptimize that holds the attachment to
// the cluster that optimize names: the oldest one, and among equally old ones
// the one whose name sorts first. Every reconcile of every attached
// CamundaOptimize picks the same holder, so the two cannot take the
// attachment from each other.
//
// One cluster carries one Optimize instance. The Optimize index prefix is
// fixed, so two instances write the same analytics indices of the same
// Elasticsearch, and their pods would carry identical discovery labels, which
// makes each Service route to the pods of both. The holder does the work; the
// others report ClusterAlreadyAttached and touch nothing.
//
// A CamundaOptimize under deletion still counts. It keeps the attachment until
// its finalizer withdraws the exporter settings, so the next holder never
// applies them while the previous one is still removing them.
//
// The list is read live rather than from the cache. A cached list can miss a
// CamundaOptimize that was created moments ago, and both would then believe
// they hold the attachment and fight over the exporter settings.
func (r *Reconciler) attachmentHolder(
	ctx context.Context,
	optimize *v1.CamundaOptimize,
) (*v1.CamundaOptimize, error) {
	var list v1.CamundaOptimizeList
	if err := r.APIReader.List(ctx, &list, client.InNamespace(optimize.Namespace)); err != nil {
		return nil, fmt.Errorf("listing the CamundaOptimizes of namespace %q: %w", optimize.Namespace, err)
	}

	var holder *v1.CamundaOptimize
	for i := range list.Items {
		candidate := &list.Items[i]
		if candidate.Spec.ClusterRef.Name != optimize.Spec.ClusterRef.Name {
			continue
		}
		if holder == nil || olderThan(candidate, holder) {
			holder = candidate
		}
	}

	return holder, nil
}

// olderThan reports whether a was created before b, with the name breaking a
// tie. Two objects created in the same second are common, so the tie-break
// must be deterministic.
func olderThan(a, b *v1.CamundaOptimize) bool {
	if !a.CreationTimestamp.Equal(&b.CreationTimestamp) {
		return a.CreationTimestamp.Before(&b.CreationTimestamp)
	}

	return a.Name < b.Name
}

// holdsAttachment reports whether optimize is the CamundaOptimize that holds
// the attachment to the cluster it names.
func (r *Reconciler) holdsAttachment(ctx context.Context, optimize *v1.CamundaOptimize) (bool, error) {
	holder, err := r.attachmentHolder(ctx, optimize)
	if err != nil {
		return false, err
	}

	return holder != nil && holder.Name == optimize.Name, nil
}
