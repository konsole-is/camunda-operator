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

package conditions

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
)

// Suspended builds a Suspended condition observed at the given generation:
// status True while the workload is intentionally scaled to zero, False
// otherwise. The caller supplies LastTransitionTime handling via
// PatchConditions.
func Suspended(status metav1.ConditionStatus, message string, observedGeneration int64) metav1.Condition {
	return metav1.Condition{
		Type: TypeSuspended, Status: status, Reason: ReasonSuspended,
		Message: message, ObservedGeneration: observedGeneration,
	}
}

// PatchConditions server-side-applies conds and status.observedGeneration in
// a single apply to obj's status subresource under FieldOwner. It preserves
// each condition's LastTransitionTime when that condition's status is
// unchanged and skips the API call entirely when the persisted status already
// matches. obj must be the freshly fetched resource with its status
// unmodified, for the same reasons as PatchReady.
//
// PatchConditions is for the CR-level conditions only, and conds must be the
// controller's complete CR-level condition set on every call: the apply lists
// exactly conds under FieldOwner with ForceOwnership, so a condition omitted
// from a later call is removed by the merge. Per-component conditions staged
// by ocf components must be persisted through the component's FlushStatus,
// never SSA-applied under FieldOwner.
func PatchConditions(
	ctx context.Context,
	c client.Client,
	obj Object,
	observedGeneration int64,
	conds ...metav1.Condition,
) error {
	if conditionsMatch(obj, observedGeneration, conds) {
		return nil
	}

	condMaps := make([]any, 0, len(conds))
	for _, cond := range conds {
		current := meta.FindStatusCondition(obj.GetConditions(), cond.Type)
		if current != nil && current.Status == cond.Status {
			cond.LastTransitionTime = current.LastTransitionTime
		}
		if cond.LastTransitionTime.IsZero() {
			cond.LastTransitionTime = metav1.Now()
		}

		condMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&cond)
		if err != nil {
			return fmt.Errorf("converting condition %q: %w", cond.Type, err)
		}
		condMaps = append(condMaps, condMap)
	}

	gvk, err := apiutil.GVKForObject(obj, c.Scheme())
	if err != nil {
		return fmt.Errorf("resolving GVK: %w", err)
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName(obj.GetName())
	u.SetNamespace(obj.GetNamespace())
	if err := unstructured.SetNestedField(u.Object, map[string]any{
		"observedGeneration": observedGeneration,
		"conditions":         condMaps,
	}, "status"); err != nil {
		return fmt.Errorf("building status patch: %w", err)
	}

	return c.Status().Apply(ctx, client.ApplyConfigurationFromUnstructured(u), FieldOwner, client.ForceOwnership)
}

// conditionsMatch reports whether obj's persisted status already carries
// every condition in conds unchanged at observedGeneration.
func conditionsMatch(obj Object, observedGeneration int64, conds []metav1.Condition) bool {
	if obj.GetObservedGeneration() != observedGeneration {
		return false
	}

	for _, cond := range conds {
		current := meta.FindStatusCondition(obj.GetConditions(), cond.Type)
		if current == nil || current.Status != cond.Status || current.Reason != cond.Reason ||
			current.Message != cond.Message || current.ObservedGeneration != cond.ObservedGeneration {
			return false
		}
	}

	return true
}
