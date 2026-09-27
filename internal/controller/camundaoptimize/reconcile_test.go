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
	"errors"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
)

// TestReconcileHarnessRenders covers the harness itself: a Reconcile passes the
// whole pre-check and renders both workloads. Every other test of this file
// starts from there.
func TestReconcileHarnessRenders(t *testing.T) {
	h := newReconcileHarness(t)

	require.NoError(t, h.reconcile(t))

	for _, key := range h.workloadKeys() {
		assert.Equal(t, int32(1), h.replicas(t, key), key.Name)
	}
	ready := meta.FindStatusCondition(h.latest(t).Status.Conditions, v1.ConditionReady)
	require.NotNil(t, ready)
	assert.Equal(t, metav1.ConditionFalse, ready.Status, "no workload reports a ready pod yet")
}

// TestReconcileReportsTheDrainOfASuspension covers what envtest cannot show,
// because it has no kubelet: a suspension reports Suspending while the pods of
// the workloads are still up, and Suspended once they are gone. The event
// marks the transition once, however many passes the drain takes.
func TestReconcileReportsTheDrainOfASuspension(t *testing.T) {
	h := newReconcileHarness(t)
	h.start(t)

	h.setClusterSuspend(t, true)
	require.NoError(t, h.reconcile(t))

	for _, key := range h.workloadKeys() {
		assert.Equal(t, int32(0), h.replicas(t, key), key.Name)
	}
	h.expectWorkloads(t, string(component.Suspending))
	ready := meta.FindStatusCondition(h.latest(t).Status.Conditions, v1.ConditionReady)
	require.NotNil(t, ready)
	assert.Equal(t, metav1.ConditionFalse, ready.Status, "the pods are still up")
	assert.Equal(t, string(component.Suspending), ready.Reason)

	require.NoError(t, h.reconcile(t), "a second pass over the same drain")
	h.observe(t, 0)
	require.NoError(t, h.reconcile(t))

	h.expectWorkloads(t, string(component.Suspended))
	ready = meta.FindStatusCondition(h.latest(t).Status.Conditions, v1.ConditionReady)
	require.NotNil(t, ready)
	assert.Equal(t, metav1.ConditionTrue, ready.Status, "a suspension is the desired state")
	assert.Equal(t, string(component.Suspended), ready.Reason)
	assert.Equal(t, map[string]int{eventReasonClusterSuspended: 1}, h.suspensionEvents())
}

// TestReconcileRecordsOneResumeWhenAnApplyFails covers a resume pass on which
// one component apply fails. The pass decided that the suspension ends, so it
// records the resume, and the pass that retries the apply must not record it
// again.
func TestReconcileRecordsOneResumeWhenAnApplyFails(t *testing.T) {
	h := newReconcileHarness(t)
	h.start(t)
	h.setClusterSuspend(t, true)
	require.NoError(t, h.reconcile(t))
	h.observe(t, 0)
	require.NoError(t, h.reconcile(t))
	h.events()

	h.setClusterSuspend(t, false)
	webapp := h.workloadKey(components.ComponentWebapp).Name
	h.failApply = func(obj client.Object) error {
		if obj.GetName() == webapp {
			return errors.New("admission webhook denied the request")
		}

		return nil
	}
	require.Error(t, h.reconcile(t), "the rejected apply is returned, so the reconcile retries")

	h.failApply = nil
	require.NoError(t, h.reconcile(t))
	h.observe(t, 1)
	require.NoError(t, h.reconcile(t))

	for _, key := range h.workloadKeys() {
		assert.Equal(t, int32(1), h.replicas(t, key), key.Name)
	}
	assert.Equal(t, map[string]int{eventReasonClusterResumed: 1}, h.suspensionEvents())
}

// TestReconcileRecordsOneSuspensionWhenTheFlushConflicts covers a status
// conflict on the failure path. A failed check builds no component, so the
// flush owns no condition type and takes every condition from the server on a
// conflict, the workload conditions that carry the suspension included. The
// next pass must still know that the suspension was recorded.
func TestReconcileRecordsOneSuspensionWhenTheFlushConflicts(t *testing.T) {
	h := newReconcileHarness(t)
	h.start(t)

	require.NoError(t, h.client.Delete(t.Context(), &corev1.Secret{ObjectMeta: metav1.ObjectMeta{
		Name: "mac-client", Namespace: h.optimize.Namespace,
	}}))
	h.setClusterSuspend(t, true)
	conflicts := 1
	h.failStatusUpdate = func(obj client.Object) error {
		if conflicts == 0 {
			return nil
		}
		conflicts--

		return apierrors.NewConflict(
			v1.GroupVersion.WithResource("camundaoptimizes").GroupResource(),
			obj.GetName(),
			errors.New("the object has been modified"),
		)
	}
	require.NoError(t, h.reconcile(t))
	require.NoError(t, h.reconcile(t))

	for _, key := range h.workloadKeys() {
		assert.Equal(t, int32(0), h.replicas(t, key), key.Name)
	}
	ready := meta.FindStatusCondition(h.latest(t).Status.Conditions, v1.ConditionReady)
	require.NotNil(t, ready)
	assert.Equal(t, v1.ReasonMissingSecret, ready.Reason)
	assert.Equal(t, map[string]int{eventReasonClusterSuspended: 1}, h.suspensionEvents())
}
