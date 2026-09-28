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

package v1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestCamundaClusterSuspended(t *testing.T) {
	cases := map[string]struct {
		cluster v1.CamundaCluster
		want    bool
	}{
		"spec.suspend is set": {
			cluster: v1.CamundaCluster{Spec: v1.CamundaClusterSpec{Suspend: true}},
			want:    true,
		},
		"another cluster holds the backend": {
			cluster: v1.CamundaCluster{Status: v1.CamundaClusterStatus{Conditions: []metav1.Condition{{
				Type:   v1.ConditionReady,
				Status: metav1.ConditionFalse,
				Reason: v1.ReasonStorageAlreadyAttached,
			}}}},
			want: true,
		},
		"the cluster waits for the pods of the previous holder": {
			cluster: v1.CamundaCluster{Status: v1.CamundaClusterStatus{Conditions: []metav1.Condition{{
				Type:   v1.ConditionReady,
				Status: metav1.ConditionFalse,
				Reason: v1.ReasonWaitingForHandover,
			}}}},
			want: true,
		},
		"a reference of the cluster does not resolve": {
			cluster: v1.CamundaCluster{Status: v1.CamundaClusterStatus{Conditions: []metav1.Condition{{
				Type:   v1.ConditionReady,
				Status: metav1.ConditionFalse,
				Reason: v1.ReasonInvalidReference,
			}}}},
			want: false,
		},
		"a referenced Secret of the cluster is missing": {
			cluster: v1.CamundaCluster{Status: v1.CamundaClusterStatus{Conditions: []metav1.Condition{{
				Type:   v1.ConditionReady,
				Status: metav1.ConditionFalse,
				Reason: v1.ReasonMissingSecret,
			}}}},
			want: false,
		},
		"a refused downgrade keeps the cluster running": {
			cluster: v1.CamundaCluster{Status: v1.CamundaClusterStatus{Conditions: []metav1.Condition{{
				Type:   v1.ConditionReady,
				Status: metav1.ConditionFalse,
				Reason: v1.ReasonVersionDowngradeRefused,
			}}}},
			want: false,
		},
		"Ready carries another reason": {
			cluster: v1.CamundaCluster{Status: v1.CamundaClusterStatus{Conditions: []metav1.Condition{{
				Type:   v1.ConditionReady,
				Status: metav1.ConditionTrue,
				Reason: v1.ReasonHealthy,
			}}}},
			want: false,
		},
		"the cluster reports no condition yet": {
			cluster: v1.CamundaCluster{},
			want:    false,
		},
		"a suspension hold suspends a cluster that does not set spec.suspend": {
			cluster: v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
				v1.SuspensionHoldPrefix + "holder": "restores into this cluster",
			}}},
			want: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.cluster.Suspended())
		})
	}
}

// Only annotations under the prefix are holds, and they come back sorted by key.
func TestSuspensionHoldsReadsThePrefixOnly(t *testing.T) {
	cluster := v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{
		v1.SuspensionHoldPrefix + "b": "second",
		"camunda.io/config-hash":      "not a hold",
		v1.SuspensionHoldPrefix + "a": "first",
	}}}

	assert.Equal(
		t, []v1.SuspensionHold{
			{Key: v1.SuspensionHoldPrefix + "a", Reason: "first"},
			{Key: v1.SuspensionHoldPrefix + "b", Reason: "second"},
		}, cluster.SuspensionHolds(),
	)
	assert.True(t, cluster.SuspendRequested())
	assert.False(t, (&v1.CamundaCluster{}).SuspendRequested())
}
