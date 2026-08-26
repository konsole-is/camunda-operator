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

package databaseserver

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The tolerance that holds a reconcile when the bucket of a suspended server
// stops resolving keys on this, so a state it accepts is a state in which the
// server runs nothing. A server that reports Ready for any other reason still
// has instances to take down.
//
// The distinction lives in one reconcile, between the spec turning to suspend
// and CloudNativePG confirming the hibernation, so an envtest cannot hold the
// two apart. It is pinned here instead.
func TestInstancesAreDown(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		condition *metav1.Condition
		want      bool
	}{
		{name: "never reconciled"},
		{
			name: "running and healthy",
			condition: &metav1.Condition{
				Type: v1.ConditionClusterReady, Status: metav1.ConditionTrue,
				Reason: string(component.Healthy),
			},
		},
		{
			name: "asked for the suspension, instances still up",
			condition: &metav1.Condition{
				Type: v1.ConditionClusterReady, Status: metav1.ConditionFalse,
				Reason: string(component.Suspending),
			},
		},
		{
			name: "instances down",
			condition: &metav1.Condition{
				Type: v1.ConditionClusterReady, Status: metav1.ConditionTrue,
				Reason: string(component.Suspended),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var server v1.DatabaseServer
			if tt.condition != nil {
				server.Status.Conditions = []metav1.Condition{*tt.condition}
			}

			assert.Equal(t, tt.want, instancesAreDown(&server))
		})
	}
}
