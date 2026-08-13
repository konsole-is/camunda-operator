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

// Package conditions provides the Ready-condition vocabulary and SSA status
// patching shared by the contract validation controllers.
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

const (
	// TypeReady is the condition type every contract CRD reports.
	TypeReady = "Ready"
	// ReasonHealthy indicates all validation checks passed.
	ReasonHealthy = "Healthy"
	// ReasonMissingSecret indicates a referenced Secret is missing or lacks a
	// configured key.
	ReasonMissingSecret = "MissingSecret"
	// ReasonInvalidReference indicates a referenced custom resource does not
	// exist.
	ReasonInvalidReference = "InvalidReference"
)

// FieldOwner is the server-side-apply field manager for all camunda-operator
// writes.
const FieldOwner = client.FieldOwner("camunda-operator")

// Object is implemented by contract CRs whose Ready condition the validation
// controllers maintain.
type Object interface {
	client.Object
	// GetConditions returns the resource's status conditions.
	GetConditions() []metav1.Condition
	// GetObservedGeneration returns the last reconciled generation recorded in
	// status.
	GetObservedGeneration() int64
}

// Ready builds a Ready condition observed at the given generation. The caller
// supplies LastTransitionTime handling via PatchReady.
func Ready(status metav1.ConditionStatus, reason, message string, observedGeneration int64) metav1.Condition {
	return metav1.Condition{
		Type: TypeReady, Status: status, Reason: reason,
		Message: message, ObservedGeneration: observedGeneration,
	}
}

// needsPatch reports whether obj's persisted status differs from cond.
func needsPatch(obj Object, cond metav1.Condition) bool {
	current := meta.FindStatusCondition(obj.GetConditions(), TypeReady)
	if current == nil || obj.GetObservedGeneration() != cond.ObservedGeneration {
		return true
	}

	return current.Status != cond.Status || current.Reason != cond.Reason ||
		current.Message != cond.Message || current.ObservedGeneration != cond.ObservedGeneration
}

// PatchReady server-side-applies cond and status.observedGeneration
// (cond.ObservedGeneration) to obj's status subresource under FieldOwner. It
// preserves LastTransitionTime when the condition status is unchanged and
// skips the API call entirely when the persisted status already matches.
// obj must be the freshly fetched resource with its status unmodified: the
// skip and preservation decisions compare against obj's in-memory status, so a
// locally mutated status produces wrong skips or transition times.
func PatchReady(ctx context.Context, c client.Client, obj Object, cond metav1.Condition) error {
	if !needsPatch(obj, cond) {
		return nil
	}

	current := meta.FindStatusCondition(obj.GetConditions(), TypeReady)
	if current != nil && current.Status == cond.Status {
		cond.LastTransitionTime = current.LastTransitionTime
	}
	if cond.LastTransitionTime.IsZero() {
		cond.LastTransitionTime = metav1.Now()
	}

	gvk, err := apiutil.GVKForObject(obj, c.Scheme())
	if err != nil {
		return fmt.Errorf("resolving GVK: %w", err)
	}

	condMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&cond)
	if err != nil {
		return fmt.Errorf("converting condition: %w", err)
	}

	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName(obj.GetName())
	u.SetNamespace(obj.GetNamespace())
	if err := unstructured.SetNestedField(u.Object, map[string]any{
		"observedGeneration": cond.ObservedGeneration,
		"conditions":         []any{condMap},
	}, "status"); err != nil {
		return fmt.Errorf("building status patch: %w", err)
	}

	return c.Status().Apply(ctx, client.ApplyConfigurationFromUnstructured(u), FieldOwner, client.ForceOwnership)
}
