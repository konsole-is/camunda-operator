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
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// TestRemoveComponentConditionsDropsWhatAParkedHolderNoLongerRenders pins the
// status of a deposed holder. It renders no component, so FlushStatus owns none
// of the component condition types and writes back whatever the object carries.
// A WebappReady of True over a deleted Deployment is the state this prevents.
func TestRemoveComponentConditionsDropsWhatAParkedHolderNoLongerRenders(t *testing.T) {
	optimize := &v1.CamundaOptimize{}
	for _, conditionType := range []string{
		v1.ConditionWebappReady,
		v1.ConditionImporterReady,
		v1.ConditionMirroredSecretsReady,
	} {
		meta.SetStatusCondition(optimize.GetStatusConditions(), metav1.Condition{
			Type:   conditionType,
			Status: metav1.ConditionTrue,
			Reason: "Healthy",
		})
	}
	meta.SetStatusCondition(optimize.GetStatusConditions(), metav1.Condition{
		Type:   v1.ConditionReady,
		Status: metav1.ConditionFalse,
		Reason: v1.ReasonClusterAlreadyAttached,
	})

	removeComponentConditions(optimize)

	assert.Nil(t, meta.FindStatusCondition(*optimize.GetStatusConditions(), v1.ConditionWebappReady))
	assert.Nil(t, meta.FindStatusCondition(*optimize.GetStatusConditions(), v1.ConditionImporterReady))
	assert.Nil(
		t, meta.FindStatusCondition(*optimize.GetStatusConditions(), v1.ConditionMirroredSecretsReady),
	)

	ready := meta.FindStatusCondition(*optimize.GetStatusConditions(), v1.ConditionReady)
	if assert.NotNil(t, ready, "Ready carries the reason the CamundaOptimize is parked") {
		assert.Equal(t, v1.ReasonClusterAlreadyAttached, ready.Reason)
	}
}

// TestRemoveComponentConditionsOnAHolderThatNeverRendered pins that the call is
// safe on a CamundaOptimize that was parked from its first reconcile. It never
// wrote a component condition, so there is nothing to drop.
func TestRemoveComponentConditionsOnAHolderThatNeverRendered(t *testing.T) {
	optimize := &v1.CamundaOptimize{}

	removeComponentConditions(optimize)

	assert.Empty(t, *optimize.GetStatusConditions())
}
