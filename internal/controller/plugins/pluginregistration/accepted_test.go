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

package pluginregistration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/version"
)

func registrationOf(
	mode pluginsv1alpha1.Mode,
	rangeExpr string,
	sections ...string,
) *pluginsv1alpha1.PluginRegistration {
	return &pluginsv1alpha1.PluginRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: "my-option", Generation: 4},
		Spec: pluginsv1alpha1.PluginRegistrationSpec{
			Sections:         sections,
			Mode:             mode,
			OperatorVersions: rangeExpr,
		},
	}
}

func TestAccepted(t *testing.T) {
	tests := []struct {
		name            string
		reg             *pluginsv1alpha1.PluginRegistration
		operatorVersion string
		facts           facts
		wantStatus      metav1.ConditionStatus
		wantReason      string
		wantMessage     string
	}{
		{
			name:            "all checks pass",
			reg:             registrationOf(pluginsv1alpha1.ModeDefault, ">=0.3.0 <0.4.0", "backup-storage"),
			operatorVersion: "0.3.1",
			wantStatus:      metav1.ConditionTrue,
			wantReason:      v1.ReasonHealthy,
			wantMessage:     "All checks passed",
		},
		{
			name:            "missing sections are named in spec order",
			reg:             registrationOf(pluginsv1alpha1.ModeOptional, "", "encryption-key", "backup-storage"),
			operatorVersion: "0.3.0",
			facts:           facts{missingSections: []string{"encryption-key", "backup-storage"}},
			wantStatus:      metav1.ConditionFalse,
			wantReason:      pluginsv1alpha1.ReasonSectionMissing,
			wantMessage:     "These sections do not exist: encryption-key, backup-storage",
		},
		{
			name:            "a missing section wins over the version and the default checks",
			reg:             registrationOf(pluginsv1alpha1.ModeDefault, ">=9.0.0", "backup-storage"),
			operatorVersion: "0.3.0",
			facts: facts{
				missingSections: []string{"backup-storage"},
				defaultPeers:    map[string][]string{"backup-storage": {"other"}},
			},
			wantStatus:  metav1.ConditionFalse,
			wantReason:  pluginsv1alpha1.ReasonSectionMissing,
			wantMessage: "These sections do not exist: backup-storage",
		},
		{
			name:            "a version outside the range",
			reg:             registrationOf(pluginsv1alpha1.ModeOptional, ">=0.4.0", "backup-storage"),
			operatorVersion: "0.3.0",
			wantStatus:      metav1.ConditionFalse,
			wantReason:      pluginsv1alpha1.ReasonOperatorVersionUnsupported,
			wantMessage:     `Operator version 0.3.0 is outside operatorVersions ">=0.4.0"`,
		},
		{
			name:            "a prerelease is outside a plain range that holds its release",
			reg:             registrationOf(pluginsv1alpha1.ModeOptional, ">=0.3.0 <0.4.0", "backup-storage"),
			operatorVersion: "0.3.0-rc.1",
			wantStatus:      metav1.ConditionFalse,
			wantReason:      pluginsv1alpha1.ReasonOperatorVersionUnsupported,
			wantMessage:     `Operator version 0.3.0-rc.1 is outside operatorVersions ">=0.3.0 <0.4.0"`,
		},
		{
			name:            "a dev build is inside every valid range",
			reg:             registrationOf(pluginsv1alpha1.ModeOptional, ">=99.0.0", "backup-storage"),
			operatorVersion: version.Dev,
			wantStatus:      metav1.ConditionTrue,
			wantReason:      v1.ReasonHealthy,
			wantMessage:     "All checks passed",
		},
		{
			name:            "no range passes",
			reg:             registrationOf(pluginsv1alpha1.ModeOptional, "", "backup-storage"),
			operatorVersion: "0.3.0",
			wantStatus:      metav1.ConditionTrue,
			wantReason:      v1.ReasonHealthy,
			wantMessage:     "All checks passed",
		},
		{
			name:            "a version failure wins over the default check",
			reg:             registrationOf(pluginsv1alpha1.ModeDefault, ">=0.4.0", "backup-storage"),
			operatorVersion: "0.3.0",
			facts:           facts{defaultPeers: map[string][]string{"backup-storage": {"other"}}},
			wantStatus:      metav1.ConditionFalse,
			wantReason:      pluginsv1alpha1.ReasonOperatorVersionUnsupported,
			wantMessage:     `Operator version 0.3.0 is outside operatorVersions ">=0.4.0"`,
		},
		{
			name:            "a Default registration with Default peers names them per section",
			reg:             registrationOf(pluginsv1alpha1.ModeDefault, "", "secondary-storage", "backup-storage"),
			operatorVersion: "0.3.0",
			facts: facts{defaultPeers: map[string][]string{
				"secondary-storage": {"elasticsearch", "opensearch"},
				"backup-storage":    {"s3"},
			}},
			wantStatus: metav1.ConditionFalse,
			wantReason: pluginsv1alpha1.ReasonDefaultConflict,
			wantMessage: "In section secondary-storage, these registrations also have mode Default: " +
				"elasticsearch, opensearch. In section backup-storage, these registrations also have mode Default: s3",
		},
		{
			name:            "an Optional registration ignores Default peers",
			reg:             registrationOf(pluginsv1alpha1.ModeOptional, "", "secondary-storage"),
			operatorVersion: "0.3.0",
			facts:           facts{defaultPeers: map[string][]string{"secondary-storage": {"a", "b"}}},
			wantStatus:      metav1.ConditionTrue,
			wantReason:      v1.ReasonHealthy,
			wantMessage:     "All checks passed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := accepted(tt.reg, tt.operatorVersion, tt.facts)

			assert.Equal(t, pluginsv1alpha1.ConditionAccepted, got.Type)
			assert.Equal(t, tt.wantStatus, got.Status)
			assert.Equal(t, tt.wantReason, got.Reason)
			assert.Equal(t, tt.wantMessage, got.Message)
			assert.Equal(t, int64(4), got.ObservedGeneration)
		})
	}
}

func TestAcceptedReportsAMalformedRange(t *testing.T) {
	got := accepted(registrationOf(pluginsv1alpha1.ModeOptional, "bogus", "backup-storage"), "0.3.0", facts{})

	assert.Equal(t, metav1.ConditionFalse, got.Status)
	assert.Equal(t, pluginsv1alpha1.ReasonOperatorVersionUnsupported, got.Reason)
	assert.Contains(t, got.Message, "operatorVersions is not valid")
	assert.Contains(t, got.Message, `"bogus"`)
}
