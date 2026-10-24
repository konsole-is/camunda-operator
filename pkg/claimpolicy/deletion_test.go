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

package claimpolicy

import (
	"testing"

	"github.com/stretchr/testify/assert"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

func TestCheckDeletionPolicyFixtures(t *testing.T) {
	t.Parallel()

	type input struct {
		Policy        pluginsv1alpha1.DeletionPolicy       `json:"policy"`
		Selections    []fleetv1alpha1.PluginSelection      `json:"selections,omitempty"`
		Registrations []pluginsv1alpha1.PluginRegistration `json:"registrations,omitempty"`
	}

	runFixtures(t, "check-deletion-policy", func(in input) (any, error) {
		return struct{}{}, CheckDeletionPolicy(in.Policy, in.Selections, in.Registrations)
	})
}

func TestDeletionPolicy(t *testing.T) {
	t.Parallel()

	orphan := &fleetv1alpha1.ResolvedPolicy{DeletionPolicy: pluginsv1alpha1.DeletionPolicyOrphan}

	tests := []struct {
		name     string
		own      pluginsv1alpha1.DeletionPolicy
		resolved *fleetv1alpha1.ResolvedPolicy
		want     pluginsv1alpha1.DeletionPolicy
	}{
		{
			"the policy of the claim wins",
			pluginsv1alpha1.DeletionPolicyDelete,
			orphan,
			pluginsv1alpha1.DeletionPolicyDelete,
		},
		{"no policy of the claim takes the default", "", orphan, pluginsv1alpha1.DeletionPolicyOrphan},
		{"no snapshot means Delete", "", nil, pluginsv1alpha1.DeletionPolicyDelete},
		{
			"a snapshot without a default means Delete",
			"",
			&fleetv1alpha1.ResolvedPolicy{},
			pluginsv1alpha1.DeletionPolicyDelete,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tt.want, DeletionPolicy(tt.own, tt.resolved))
		})
	}
}
