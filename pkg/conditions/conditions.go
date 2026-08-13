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

// Package conditions provides the Ready-condition vocabulary, SSA status
// patching, and Ready derivation shared by the contract validation and
// storage backend controllers.
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
	// TypeSuspended is the condition type a suspendable CR reports while its
	// workload is intentionally scaled to zero.
	TypeSuspended = "Suspended"
	// ReasonHealthy indicates all validation checks passed.
	ReasonHealthy = "Healthy"
	// ReasonProgressing indicates the managed resources have not yet reached
	// their desired state.
	ReasonProgressing = "Progressing"
	// ReasonMissingSecret indicates a referenced Secret is missing or lacks a
	// configured key.
	ReasonMissingSecret = "MissingSecret"
	// ReasonInvalidReference indicates a referenced custom resource does not
	// exist.
	ReasonInvalidReference = "InvalidReference"
	// ReasonConnectionFailed indicates a backing server is unreachable or
	// rejects the configured credentials.
	ReasonConnectionFailed = "ConnectionFailed"
	// ReasonSuspended indicates the resource is suspended and intentionally
	// not serving.
	ReasonSuspended = "Suspended"
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

// PreCheckFailure is a failed reconciliation pre-check — an unresolved
// reference, a missing Secret, an unreachable server — mapped to its
// documented Ready reason and a condition-ready message.
type PreCheckFailure struct {
	// Reason is the documented Ready condition reason for the failure.
	Reason string
	// Message is the condition-ready failure message.
	Message string
}

// DeriveReady derives the CR-level Ready reason and message from the
// controller's pre-check result, the ocf component conditions, and the
// suspension flag. A pre-check failure wins outright; otherwise suspension
// reports Suspended; otherwise the first component condition whose status is
// not True reports Progressing with a message naming that component; with
// every component True the result is Healthy.
func DeriveReady(pre *PreCheckFailure, componentConds []metav1.Condition, suspended bool) (reason, message string) {
	if pre != nil {
		return pre.Reason, pre.Message
	}

	if suspended {
		return ReasonSuspended, "Suspended by spec.suspend"
	}

	for _, cond := range componentConds {
		if cond.Status == metav1.ConditionTrue {
			continue
		}
		detail := cond.Message
		if detail == "" {
			detail = cond.Reason
		}
		return ReasonProgressing, fmt.Sprintf("Waiting for %s: %s", cond.Type, detail)
	}

	return ReasonHealthy, "All components ready"
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
