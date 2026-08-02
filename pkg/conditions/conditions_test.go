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

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestReadyBuildsConditionVerbatim(t *testing.T) {
	cond := Ready(metav1.ConditionFalse, ReasonMissingSecret, `Secret "ns/name" not found`, 3)

	assert.Equal(t, TypeReady, cond.Type)
	assert.Equal(t, metav1.ConditionFalse, cond.Status)
	assert.Equal(t, ReasonMissingSecret, cond.Reason)
	assert.Equal(t, `Secret "ns/name" not found`, cond.Message)
	assert.Equal(t, int64(3), cond.ObservedGeneration)
}

// objectWith returns a contract CR whose persisted status carries the given
// Ready condition and observed generation.
func objectWith(observedGeneration int64, cond *metav1.Condition) Object {
	obj := &v1.DatabaseServerConfig{}
	obj.Status.ObservedGeneration = observedGeneration
	if cond != nil {
		obj.Status.Conditions = []metav1.Condition{*cond}
	}
	return obj
}

func TestNeedsPatch(t *testing.T) {
	healthy := Ready(metav1.ConditionTrue, ReasonHealthy, "All checks passed", 2)

	tests := []struct {
		name string
		obj  Object
		cond metav1.Condition
		want bool
	}{
		{
			name: "no Ready condition",
			obj:  objectWith(2, nil),
			cond: healthy,
			want: true,
		},
		{
			name: "status differs",
			obj:  objectWith(2, &healthy),
			cond: Ready(metav1.ConditionFalse, ReasonHealthy, "All checks passed", 2),
			want: true,
		},
		{
			name: "reason differs",
			obj:  objectWith(2, &healthy),
			cond: Ready(metav1.ConditionTrue, ReasonMissingSecret, "All checks passed", 2),
			want: true,
		},
		{
			name: "message differs",
			obj:  objectWith(2, &healthy),
			cond: Ready(metav1.ConditionTrue, ReasonHealthy, `Secret "ns/s" not found`, 2),
			want: true,
		},
		{
			name: "status observedGeneration lags",
			obj:  objectWith(1, &healthy),
			cond: healthy,
			want: true,
		},
		{
			name: "condition observedGeneration lags",
			obj:  objectWith(3, &healthy),
			cond: Ready(metav1.ConditionTrue, ReasonHealthy, "All checks passed", 3),
			want: true,
		},
		{
			name: "everything matches",
			obj:  objectWith(2, &healthy),
			cond: healthy,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, needsPatch(tt.obj, tt.cond))
		})
	}
}
