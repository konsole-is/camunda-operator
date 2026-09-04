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
		"another cluster holds the storage contract": {
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
			want: true,
		},
		"a referenced Secret of the cluster is missing": {
			cluster: v1.CamundaCluster{Status: v1.CamundaClusterStatus{Conditions: []metav1.Condition{{
				Type:   v1.ConditionReady,
				Status: metav1.ConditionFalse,
				Reason: v1.ReasonMissingSecret,
			}}}},
			want: true,
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
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tc.want, tc.cluster.Suspended())
		})
	}
}
