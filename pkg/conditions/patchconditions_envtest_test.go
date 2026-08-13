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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestPatchConditionsAppliesAllConditionsAndObservedGeneration(t *testing.T) {
	ctx := t.Context()
	resource := newDatabaseServerConfig(ctx, t)

	require.NoError(t, PatchConditions(ctx, k8sClient, resource, resource.Generation,
		Ready(metav1.ConditionFalse, ReasonSuspended, "Suspended by spec.suspend", resource.Generation),
		Suspended(metav1.ConditionTrue, "Node set scaled to zero", resource.Generation),
	))

	fetched := fetchDatabaseServerConfig(ctx, t, resource.Name)
	assert.Equal(t, resource.Generation, fetched.Status.ObservedGeneration)

	ready := meta.FindStatusCondition(fetched.Status.Conditions, TypeReady)
	require.NotNil(t, ready)
	assert.Equal(t, metav1.ConditionFalse, ready.Status)
	assert.Equal(t, ReasonSuspended, ready.Reason)

	suspended := meta.FindStatusCondition(fetched.Status.Conditions, TypeSuspended)
	require.NotNil(t, suspended)
	assert.Equal(t, metav1.ConditionTrue, suspended.Status)
	assert.Equal(t, ReasonSuspended, suspended.Reason)
	assert.Equal(t, "Node set scaled to zero", suspended.Message)
	assert.False(t, suspended.LastTransitionTime.IsZero())
}

func TestPatchConditionsKeepsEveryListedConditionAcrossReapplies(t *testing.T) {
	ctx := t.Context()
	resource := newDatabaseServerConfig(ctx, t)

	require.NoError(t, PatchConditions(ctx, k8sClient, resource, resource.Generation,
		Ready(metav1.ConditionFalse, ReasonProgressing, "Waiting for ElasticsearchReady: creating", resource.Generation),
		Suspended(metav1.ConditionFalse, "Suspension not requested", resource.Generation),
	))

	fresh := fetchDatabaseServerConfig(ctx, t, resource.Name)
	require.NoError(t, PatchConditions(ctx, k8sClient, fresh, fresh.Generation,
		Ready(metav1.ConditionTrue, ReasonHealthy, "All components ready", fresh.Generation),
		Suspended(metav1.ConditionFalse, "Suspension not requested", fresh.Generation),
	))

	fetched := fetchDatabaseServerConfig(ctx, t, resource.Name)
	ready := meta.FindStatusCondition(fetched.Status.Conditions, TypeReady)
	require.NotNil(t, ready)
	assert.Equal(t, ReasonHealthy, ready.Reason)
	assert.NotNil(t, meta.FindStatusCondition(fetched.Status.Conditions, TypeSuspended),
		"reapplying the full condition set must not remove any of its members")
}

// The apply owns exactly the listed conditions: one omitted from a later call
// is removed by the SSA merge. Controllers must therefore pass their full
// CR-level condition set on every call — this pins the sharp edge the GoDoc
// warns about.
func TestPatchConditionsRemovesConditionsOmittedFromALaterApply(t *testing.T) {
	ctx := t.Context()
	resource := newDatabaseServerConfig(ctx, t)

	require.NoError(t, PatchConditions(ctx, k8sClient, resource, resource.Generation,
		Ready(metav1.ConditionFalse, ReasonProgressing, "Waiting", resource.Generation),
		Suspended(metav1.ConditionFalse, "Suspension not requested", resource.Generation),
	))

	fresh := fetchDatabaseServerConfig(ctx, t, resource.Name)
	require.NoError(t, PatchConditions(ctx, k8sClient, fresh, fresh.Generation,
		Ready(metav1.ConditionTrue, ReasonHealthy, "All components ready", fresh.Generation),
	))

	fetched := fetchDatabaseServerConfig(ctx, t, resource.Name)
	assert.NotNil(t, meta.FindStatusCondition(fetched.Status.Conditions, TypeReady))
	assert.Nil(t, meta.FindStatusCondition(fetched.Status.Conditions, TypeSuspended))
}

func TestPatchConditionsPreservesLastTransitionTimePerUnchangedCondition(t *testing.T) {
	ctx := t.Context()
	resource := newDatabaseServerConfig(ctx, t)

	require.NoError(t, PatchConditions(ctx, k8sClient, resource, resource.Generation,
		Ready(metav1.ConditionFalse, ReasonProgressing, "Waiting", resource.Generation),
		Suspended(metav1.ConditionFalse, "Suspension not requested", resource.Generation),
	))

	// Backdate the persisted transition times: metav1.Time has second
	// precision and both patches land within one second, so preservation
	// would otherwise be indistinguishable from a re-stamp.
	backdated := metav1.NewTime(time.Date(2020, 1, 2, 3, 4, 5, 0, time.UTC))
	persisted := fetchDatabaseServerConfig(ctx, t, resource.Name)
	for i := range persisted.Status.Conditions {
		persisted.Status.Conditions[i].LastTransitionTime = backdated
	}
	require.NoError(t, k8sClient.Status().Update(ctx, persisted))

	fresh := fetchDatabaseServerConfig(ctx, t, resource.Name)
	require.NoError(t, PatchConditions(ctx, k8sClient, fresh, fresh.Generation,
		Ready(metav1.ConditionFalse, ReasonProgressing, "Still waiting", fresh.Generation),
		Suspended(metav1.ConditionTrue, "Node set scaled to zero", fresh.Generation),
	))

	fetched := fetchDatabaseServerConfig(ctx, t, resource.Name)

	ready := meta.FindStatusCondition(fetched.Status.Conditions, TypeReady)
	require.NotNil(t, ready)
	assert.True(t, ready.LastTransitionTime.Time.Equal(backdated.Time),
		"an unchanged condition status must keep its transition time")

	suspended := meta.FindStatusCondition(fetched.Status.Conditions, TypeSuspended)
	require.NotNil(t, suspended)
	assert.False(t, suspended.LastTransitionTime.Time.Equal(backdated.Time),
		"a flipped condition status must be re-stamped")
}

func TestPatchConditionsSkipsAPICallWhenPersistedStatusMatches(t *testing.T) {
	ctx := t.Context()
	resource := newDatabaseServerConfig(ctx, t)

	conds := []metav1.Condition{
		Ready(metav1.ConditionTrue, ReasonHealthy, "All components ready", resource.Generation),
		Suspended(metav1.ConditionFalse, "Suspension not requested", resource.Generation),
	}
	require.NoError(t, PatchConditions(ctx, k8sClient, resource, resource.Generation, conds...))

	before := fetchDatabaseServerConfig(ctx, t, resource.Name)
	require.NoError(t, PatchConditions(ctx, k8sClient, before, before.Generation, conds...))

	after := fetchDatabaseServerConfig(ctx, t, resource.Name)
	assert.Equal(t, before.ResourceVersion, after.ResourceVersion,
		"a no-op PatchConditions must not write to the API server")
}
