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
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

func TestCheckDeletionPolicyFixtures(t *testing.T) {
	t.Parallel()

	type input struct {
		Policy        pluginsv1alpha1.DeletionPolicy       `json:"policy"`
		Selections    []fleetv1alpha1.PluginSelection      `json:"selections,omitempty"`
		Registrations []pluginsv1alpha1.PluginRegistration `json:"registrations,omitempty"`
		Missing       string                               `json:"missing,omitempty"`
	}

	runFixtures(t, "check-deletion-policy", func(in input) (any, error) {
		missing := MissingPasses
		switch in.Missing {
		case "":
		case "Refuses":
			missing = MissingRefuses
		default:
			return nil, fmt.Errorf("unknown missing %q", in.Missing)
		}

		return struct{}{}, CheckDeletionPolicy(in.Policy, in.Selections, in.Registrations, missing)
	})
}

func TestCheckDeletionPolicyWithMissingRefuses(t *testing.T) {
	t.Parallel()

	selections := []fleetv1alpha1.PluginSelection{
		{Section: "access", Registration: "ingress"},
		{Section: "encryption-key", Registration: "kms"},
	}
	kms := pluginsv1alpha1.PluginRegistration{
		ObjectMeta: metav1.ObjectMeta{Name: "kms"},
		Spec: pluginsv1alpha1.PluginRegistrationSpec{
			DeletionPolicies: []pluginsv1alpha1.DeletionPolicy{pluginsv1alpha1.DeletionPolicyDelete},
		},
	}

	t.Run("refuses Orphan for a registration that does not exist, and names it", func(t *testing.T) {
		t.Parallel()

		err := CheckDeletionPolicy(pluginsv1alpha1.DeletionPolicyOrphan, selections[:1], nil, MissingRefuses)

		var failure *conditions.PreCheckFailure
		require.ErrorAs(t, err, &failure)
		assert.Equal(t, fleetv1alpha1.ReasonNotAllowed, failure.Reason)
		assert.Equal(
			t,
			`the deletion policy is Orphan, and the claim cannot tell that these registrations support it, `+
				`because they do not exist: PluginRegistration "ingress" of section "access"`,
			failure.Message,
		)
	})

	t.Run("names a missing and an unsupported registration in one refusal", func(t *testing.T) {
		t.Parallel()

		err := CheckDeletionPolicy(
			pluginsv1alpha1.DeletionPolicyOrphan,
			selections,
			[]pluginsv1alpha1.PluginRegistration{kms},
			MissingRefuses,
		)

		var failure *conditions.PreCheckFailure
		require.ErrorAs(t, err, &failure)
		assert.Contains(t, failure.Message, `PluginRegistration "kms" of section "encryption-key"`)
		assert.Contains(t, failure.Message, `do not exist: PluginRegistration "ingress" of section "access"`)
	})

	t.Run("passes Delete for a registration that does not exist", func(t *testing.T) {
		t.Parallel()

		assert.NoError(t, CheckDeletionPolicy(pluginsv1alpha1.DeletionPolicyDelete, selections, nil, MissingRefuses))
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
