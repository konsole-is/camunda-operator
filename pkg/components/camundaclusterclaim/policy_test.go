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

package camundaclusterclaim

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

func TestClaimPolicyDoesNotRequireASectionOfTheManagementPlane(t *testing.T) {
	policy := fleetv1alpha1.ResolvedPolicy{Sections: map[string]fleetv1alpha1.SectionPolicy{
		"management-database": {Required: true, Allowed: []string{"management-database-server"}},
		"backup-storage":      {Required: true, Allowed: []string{"s3-backups"}},
		"brought-by-a-plugin": {Required: true, Allowed: []string{"plugin-option"}},
	}}
	sections := []pluginsv1alpha1.PluginSection{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "management-database"},
			Spec: pluginsv1alpha1.PluginSectionSpec{
				AppliesTo: []pluginsv1alpha1.ClaimKind{pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim},
			},
		},
		{
			ObjectMeta: metav1.ObjectMeta{Name: "backup-storage"},
			Spec: pluginsv1alpha1.PluginSectionSpec{
				AppliesTo: []pluginsv1alpha1.ClaimKind{pluginsv1alpha1.ClaimKindCamundaClusterClaim},
			},
		},
	}

	claimPolicy := ClaimPolicy(policy, sections)

	assert.False(t, claimPolicy.Sections["management-database"].Required)
	assert.True(t, claimPolicy.Sections["backup-storage"].Required)
	assert.True(
		t,
		claimPolicy.Sections["brought-by-a-plugin"].Required,
		"a section that does not exist keeps its policy",
	)
	assert.True(t, policy.Sections["management-database"].Required, "the recorded policy is not changed")
}
