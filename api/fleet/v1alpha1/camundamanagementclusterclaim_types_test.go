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

package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
)

// Users match these condition types with kubectl wait, and RequestsReady has
// one string on both claim kinds. A changed string strands the old condition.
func TestPlaneClaimConditionTypesKeepTheirStoredStrings(t *testing.T) {
	assert.Equal(t, "RequestsReady", v1alpha1.ConditionPlaneRequestsReady)
	assert.Equal(t, "ManagementClusterReady", v1alpha1.ConditionManagementClusterReady)
}

func TestCamundaManagementClusterClaimStagesItsStatus(t *testing.T) {
	var claim v1alpha1.CamundaManagementClusterClaim

	*claim.GetStatusConditions() = append(*claim.GetStatusConditions(), metav1.Condition{Type: "Ready"})
	claim.SetObservedGeneration(4)

	assert.Equal(t, []metav1.Condition{{Type: "Ready"}}, claim.Status.Conditions)
	assert.Equal(t, int64(4), claim.Status.ObservedGeneration)
	assert.Equal(t, "CamundaManagementClusterClaim", claim.GetKind())
}
