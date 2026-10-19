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
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// setSummary sets the summary of status. It moves status.summaryTime to now
// only when the summary changed, so a pass that changes nothing writes
// nothing. An empty summary has no time.
func setSummary(status *pluginsv1alpha1.PluginRequestStatus, summary []pluginsv1alpha1.SummaryItem, now metav1.Time) {
	if len(summary) == 0 {
		status.Summary, status.SummaryTime = nil, nil
		return
	}

	if !equality.Semantic.DeepEqual(status.Summary, summary) || status.SummaryTime == nil {
		status.SummaryTime = &now
	}
	status.Summary = summary
}
