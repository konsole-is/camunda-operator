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

// Package conditions provides the Ready-condition vocabulary, the SSA status
// patch, and the Ready derivation that the contract validation controllers
// and the storage backend controllers share.
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
	// TypeReady is the condition type that every contract CRD reports.
	TypeReady = "Ready"
	// TypeSuspended is the condition type that a suspendable CR reports while
	// its workload is intentionally scaled to zero.
	TypeSuspended = "Suspended"
	// ReasonHealthy means that all validation checks passed.
	ReasonHealthy = "Healthy"
	// ReasonProgressing means that the managed resources have not reached
	// their desired state.
	ReasonProgressing = "Progressing"
	// ReasonMissingSecret means that a referenced Secret is missing or lacks a
	// configured key.
	ReasonMissingSecret = "MissingSecret"
	// ReasonInvalidReference means that a referenced custom resource does not
	// exist.
	ReasonInvalidReference = "InvalidReference"
	// ReasonConnectionFailed means that a backing server is unreachable or
	// rejects the configured credentials.
	ReasonConnectionFailed = "ConnectionFailed"
	// ReasonSuspended means that the resource is suspended and intentionally
	// not serving.
	ReasonSuspended = "Suspended"
)

// FieldOwner is the server-side-apply field manager for all camunda-operator
// writes.
const FieldOwner = client.FieldOwner("camunda-operator")

// Object is the contract CR whose Ready condition a validation controller
// maintains.
type Object interface {
	client.Object
	// GetConditions returns the status conditions of the resource.
	GetConditions() []metav1.Condition
	// GetObservedGeneration returns the last reconciled generation that status
	// records.
	GetObservedGeneration() int64
}

// PreCheckFailure is a failed reconciliation pre-check, for example an
// unresolved reference, a missing Secret, or an unreachable server. It maps
// the failure to its documented Ready reason and a condition-ready message. It
// is an error, so pre-checks return it through the ordinary error path. The
// reconciler picks it out with errors.As and reports it as a Ready condition,
// not as a reconcile error.
type PreCheckFailure struct {
	// Reason is the documented Ready condition reason for the failure.
	Reason string
	// Message is the condition-ready failure message.
	Message string
}

// Error returns the condition-ready message.
func (f *PreCheckFailure) Error() string { return f.Message }

// DeriveReady derives the CR-level Ready reason and message from the pre-check
// result of the controller, the ocf component conditions, and the suspension
// flag. A pre-check failure wins outright. Otherwise suspension reports
// Suspended. Otherwise the first component condition whose status is not True
// reports Progressing, with a message that names that component. With every
// component True, the result is Healthy. An empty component list is
// Progressing too. A controller always has at least one component, so an
// empty list means that none has reported yet.
func DeriveReady(pre *PreCheckFailure, componentConds []metav1.Condition, suspended bool) (reason, message string) {
	if pre != nil {
		return pre.Reason, pre.Message
	}

	if suspended {
		return ReasonSuspended, "Suspended by spec.suspend"
	}

	if len(componentConds) == 0 {
		return ReasonProgressing, "Waiting for components to report"
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

// Ready builds a Ready condition observed at the given generation. It sets no
// LastTransitionTime, because PatchReady and meta.SetStatusCondition both
// supply it.
func Ready(status metav1.ConditionStatus, reason, message string, observedGeneration int64) metav1.Condition {
	return metav1.Condition{
		Type: TypeReady, Status: status, Reason: reason,
		Message: message, ObservedGeneration: observedGeneration,
	}
}

// needsPatch reports whether the persisted status of obj differs from cond.
func needsPatch(obj Object, cond metav1.Condition) bool {
	current := meta.FindStatusCondition(obj.GetConditions(), TypeReady)
	if current == nil || obj.GetObservedGeneration() != cond.ObservedGeneration {
		return true
	}

	return current.Status != cond.Status || current.Reason != cond.Reason ||
		current.Message != cond.Message || current.ObservedGeneration != cond.ObservedGeneration
}

// PatchReady server-side-applies cond and status.observedGeneration
// (cond.ObservedGeneration) to the status subresource of obj under FieldOwner.
// It preserves LastTransitionTime when the condition status is unchanged. It
// skips the API call when the persisted status already matches, so obj must be
// the freshly fetched resource with its status unmodified.
//
// PatchReady is for controllers whose status is the single Ready condition,
// that is, the contract validators. A controller that runs ocf components
// stages Ready on the in-memory owner instead, and the FlushStatus of the
// component persists every condition in one write. An SSA-applied Ready next
// to that flush puts two field managers on the same conditions list.
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
