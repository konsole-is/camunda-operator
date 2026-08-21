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

package podstate

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// jobKind is the kind of the workload that owns every pod this package
// reports on.
var jobKind = schema.GroupKind{Group: batchv1.GroupName, Kind: "Job"}

// EnqueueJobOwner returns an event handler that maps a pod event onto the
// resource that owns the Job that owns the pod. It enqueues one request for
// a pod of a Job that owner controls, and nothing for any other pod.
//
// The chain runs over the controller references, not over the labels of the
// pod. An owner label value is bounded to the length of a DNS label, and the
// name of a custom resource can be longer, so a name read back off a label
// is not always the name of a resource.
//
// reader reads the Job. Pass the cached client of the manager. The controller
// that created the Job watches its Jobs, so the cache holds it. A Job that
// the cache does not hold enqueues nothing, and the poll of the running phase
// reads the pods again on its next look.
func EnqueueJobOwner(reader client.Reader, owner schema.GroupKind) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(
		func(ctx context.Context, pod client.Object) []reconcile.Request {
			jobRef := controllerOf(pod, jobKind)
			if jobRef == nil {
				return nil
			}

			key := types.NamespacedName{Namespace: pod.GetNamespace(), Name: jobRef.Name}

			var job batchv1.Job
			if err := reader.Get(ctx, key, &job); err != nil {
				if !apierrors.IsNotFound(err) {
					logf.FromContext(ctx).Error(err, "Could not read the Job of a pod event", "job", key)
				}

				return nil
			}

			ownerRef := controllerOf(&job, owner)
			if ownerRef == nil {
				return nil
			}

			return []reconcile.Request{{NamespacedName: types.NamespacedName{
				Namespace: job.Namespace, Name: ownerRef.Name,
			}}}
		},
	)
}

// controllerOf returns the controller reference of obj when it names a
// resource of kind gk, and nil otherwise. A reference whose apiVersion does
// not parse names no kind that this operator serves.
func controllerOf(obj metav1.Object, gk schema.GroupKind) *metav1.OwnerReference {
	ref := metav1.GetControllerOf(obj)
	if ref == nil || ref.Kind != gk.Kind {
		return nil
	}

	group, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil || group.Group != gk.Group {
		return nil
	}

	return ref
}
