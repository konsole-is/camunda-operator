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


package builtin

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

func TestSetSummaryKeepsTheTimeOfAnUnchangedSummary(t *testing.T) {
	first := metav1.NewTime(time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC))
	later := metav1.NewTime(first.Add(time.Minute))
	items := []pluginsv1alpha1.SummaryItem{{Label: "ObjectStorageConfig", Value: "my-bucket, Ready"}}
	status := pluginsv1alpha1.PluginRequestStatus{}

	setSummary(&status, items, first)
	setSummary(&status, slices.Clone(items), later)

	assert.Equal(t, items, status.Summary)
	assert.Equal(t, first, *status.SummaryTime)
}

func TestSetSummaryStampsAChangedSummary(t *testing.T) {
	first := metav1.NewTime(time.Date(2026, 10, 18, 9, 0, 0, 0, time.UTC))
	later := metav1.NewTime(first.Add(time.Minute))
	changed := []pluginsv1alpha1.SummaryItem{{Label: "ObjectStorageConfig", Value: "my-bucket, Ready"}}
	status := pluginsv1alpha1.PluginRequestStatus{}

	setSummary(&status, []pluginsv1alpha1.SummaryItem{{Label: "ObjectStorageConfig", Value: "my-bucket, waiting"}}, first)
	setSummary(&status, changed, later)

	assert.Equal(t, changed, status.Summary)
	assert.Equal(t, later, *status.SummaryTime)
}

func TestSetSummaryWithoutItemsHasNoTime(t *testing.T) {
	status := pluginsv1alpha1.PluginRequestStatus{
		Summary:     []pluginsv1alpha1.SummaryItem{{Label: "ObjectStorageConfig", Value: "my-bucket, Ready"}},
		SummaryTime: new(metav1.Now()),
	}

	setSummary(&status, nil, metav1.Now())

	assert.Nil(t, status.Summary)
	assert.Nil(t, status.SummaryTime)
}
