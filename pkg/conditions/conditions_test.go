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
	cond := Ready(metav1.ConditionFalse, v1.ReasonMissingSecret, `Secret "ns/name" not found`, 3)

	assert.Equal(t, v1.ConditionReady, cond.Type)
	assert.Equal(t, metav1.ConditionFalse, cond.Status)
	assert.Equal(t, v1.ReasonMissingSecret, cond.Reason)
	assert.Equal(t, `Secret "ns/name" not found`, cond.Message)
	assert.Equal(t, int64(3), cond.ObservedGeneration)
}
