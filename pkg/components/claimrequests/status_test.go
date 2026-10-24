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

package claimrequests

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

func TestStatusesReportEachRequestInClaimOrder(t *testing.T) {
	in := Input{
		Selections: []fleetv1alpha1.PluginSelection{
			{Section: "access", Registration: "test-access"},
			{Section: "encryption-key", Registration: "test-key"},
			{Section: "cloud-identity", Registration: "test-identity"},
		},
		Requests: []pluginsv1alpha1.PluginRequest{
			{
				ObjectMeta: metav1.ObjectMeta{Name: "orders-encryption-key"},
			},
			{
				ObjectMeta: metav1.ObjectMeta{Name: "orders-access"},
				Status: pluginsv1alpha1.PluginRequestStatus{Conditions: []metav1.Condition{
					{Type: pluginsv1alpha1.ConditionOutputsReady, Status: metav1.ConditionTrue},
					{Type: "Ready", Status: metav1.ConditionFalse},
				}},
			},
		},
	}

	assert.Equal(
		t, []fleetv1alpha1.RequestStatus{
			{
				Section:      "access",
				Registration: "test-access",
				Name:         "orders-access",
				OutputsReady: metav1.ConditionTrue,
				Ready:        metav1.ConditionFalse,
			},
			{Section: "encryption-key", Registration: "test-key", Name: "orders-encryption-key"},
			{Section: "cloud-identity", Registration: "test-identity", Name: "orders-cloud-identity"},
		}, Statuses(claim(), in),
	)
}

func TestStatusesReportEachRequestOfAManySection(t *testing.T) {
	in := Input{
		Selections: []fleetv1alpha1.PluginSelection{
			{Section: "alerts", Registration: "mail"},
			{Section: "alerts", Registration: "chat"},
		},
		Sections: []pluginsv1alpha1.PluginSection{{
			ObjectMeta: metav1.ObjectMeta{Name: "alerts"},
			Spec:       pluginsv1alpha1.PluginSectionSpec{Selection: pluginsv1alpha1.SelectionMany},
		}},
		Requests: []pluginsv1alpha1.PluginRequest{{
			ObjectMeta: metav1.ObjectMeta{Name: "orders-alerts-chat"},
			Status: pluginsv1alpha1.PluginRequestStatus{Conditions: []metav1.Condition{
				{Type: "Ready", Status: metav1.ConditionTrue},
			}},
		}},
	}

	assert.Equal(
		t, []fleetv1alpha1.RequestStatus{
			{Section: "alerts", Registration: "mail", Name: "orders-alerts-mail"},
			{Section: "alerts", Registration: "chat", Name: "orders-alerts-chat", Ready: metav1.ConditionTrue},
		}, Statuses(claim(), in),
	)
}
