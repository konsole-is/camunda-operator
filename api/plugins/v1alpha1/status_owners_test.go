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

	"github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

func TestPluginRequestStagesItsStatus(t *testing.T) {
	var request v1alpha1.PluginRequest

	*request.GetStatusConditions() = append(*request.GetStatusConditions(), metav1.Condition{Type: "OutputsReady"})
	request.SetObservedGeneration(3)

	assert.Equal(t, []metav1.Condition{{Type: "OutputsReady"}}, request.Status.Conditions)
	assert.Equal(t, int64(3), request.Status.ObservedGeneration)
	assert.Equal(t, "PluginRequest", request.GetKind())
}

func TestPluginRegistrationStagesItsStatus(t *testing.T) {
	var reg v1alpha1.PluginRegistration

	*reg.GetStatusConditions() = append(*reg.GetStatusConditions(), metav1.Condition{Type: "Ready"})
	reg.SetObservedGeneration(3)

	assert.Equal(t, []metav1.Condition{{Type: "Ready"}}, reg.Status.Conditions)
	assert.Equal(t, int64(3), reg.Status.ObservedGeneration)
	assert.Equal(t, "PluginRegistration", reg.GetKind())
}

func TestGrantStagesItsConditions(t *testing.T) {
	var grant v1alpha1.Grant

	*grant.GetStatusConditions() = append(*grant.GetStatusConditions(), metav1.Condition{Type: "Applied"})

	assert.Equal(t, []metav1.Condition{{Type: "Applied"}}, grant.Status.Conditions)
	assert.Equal(t, "Grant", grant.GetKind())
}
