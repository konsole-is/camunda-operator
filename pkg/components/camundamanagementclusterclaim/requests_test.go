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

package camundamanagementclusterclaim

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
)

func TestDatabasesFollowTheIdentityModeAndWebModeler(t *testing.T) {
	tests := []struct {
		name       string
		management fleetv1alpha1.ManagementOverrides
		want       []string
	}{
		{
			name: "keycloak with web modeler",
			management: fleetv1alpha1.ManagementOverrides{
				IdentityProvider: fleetv1alpha1.ManagementIdentityProvider{
					Keycloak: &fleetv1alpha1.KeycloakOverrides{},
				},
				WebModeler: &fleetv1alpha1.WebModelerOverrides{},
			},
			want: []string{"keycloak", "identity", "webModeler"},
		},
		{
			name: "oidc without web modeler",
			management: fleetv1alpha1.ManagementOverrides{
				IdentityProvider: fleetv1alpha1.ManagementIdentityProvider{OIDC: &v1.ManagementOIDCSpec{}},
			},
			want: []string{"identity"},
		},
		{
			name: "external keycloak with web modeler",
			management: fleetv1alpha1.ManagementOverrides{
				IdentityProvider: fleetv1alpha1.ManagementIdentityProvider{
					ExternalKeycloak: &v1.ExternalKeycloakSpec{URL: "https://kc.example.com"},
				},
				WebModeler: &fleetv1alpha1.WebModelerOverrides{},
			},
			want: []string{"identity", "webModeler"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Databases(tt.management))
		})
	}
}

func TestWithDatabasesSetsTheParameterOnTheDatabaseSectionOnly(t *testing.T) {
	params := []claimpolicy.RequestParameters{
		{
			Section:      databaseSection,
			Registration: "management-database-server",
			Parameters:   &apiextensionsv1.JSON{Raw: []byte(`{"preset":"small","databases":["x"]}`)},
		},
		{
			Section:      accessSection,
			Registration: "test-access",
			Parameters:   &apiextensionsv1.JSON{Raw: []byte(`{"hostnamePattern":"a"}`)},
		},
	}

	got, err := WithDatabases(params, sections, []string{"identity"})

	require.NoError(t, err)
	assert.JSONEq(t, `{"preset":"small","databases":["identity"]}`, string(got[0].Parameters.Raw))
	assert.Equal(t, `{"hostnamePattern":"a"}`, string(got[1].Parameters.Raw))
	assert.JSONEq(
		t,
		`{"preset":"small","databases":["x"]}`,
		string(params[0].Parameters.Raw),
		"the input is not changed",
	)
}

func TestWithDatabasesCreatesTheParametersObject(t *testing.T) {
	params := []claimpolicy.RequestParameters{{Section: databaseSection, Registration: "management-database-server"}}

	got, err := WithDatabases(params, sections, []string{"identity"})

	require.NoError(t, err)
	assert.JSONEq(t, `{"databases":["identity"]}`, string(got[0].Parameters.Raw))
}

func TestWithDatabasesRefusesParametersThatAreNotAnObject(t *testing.T) {
	params := []claimpolicy.RequestParameters{{
		Section:      databaseSection,
		Registration: "management-database-server",
		Parameters:   &apiextensionsv1.JSON{Raw: []byte(`[]`)},
	}}

	_, err := WithDatabases(params, sections, []string{"identity"})

	assert.Error(t, err)
}

func TestWriteOrderPutsTheDatabasesBeforeTheAccess(t *testing.T) {
	selections := []fleetv1alpha1.PluginSelection{
		{Section: accessSection, Registration: "test-access"},
		{Section: "extra", Registration: "test-extra"},
		{Section: databaseSection, Registration: "management-database-server"},
	}

	got := WriteOrder(selections, sections)

	assert.Equal(t, []string{databaseSection, accessSection, "extra"}, sectionsOf(got))
	assert.Equal(t, accessSection, selections[0].Section, "the input is not changed")
}

func sectionsOf(selections []fleetv1alpha1.PluginSelection) []string {
	names := make([]string, 0, len(selections))
	for _, s := range selections {
		names = append(names, s.Section)
	}
	return names
}

func TestSelectsFindsASelectionInASectionWithTheSlot(t *testing.T) {
	selections := []fleetv1alpha1.PluginSelection{
		{Section: "unknown", Registration: "x"},
		{Section: databaseSection, Registration: "management-database-server"},
	}

	assert.True(t, Selects(selections, sections, pluginsv1alpha1.SlotManagementDatabases))
	assert.False(t, Selects(selections, sections, pluginsv1alpha1.SlotExternalAccess))
}

func TestRequestsInputNamesThePlaneAsTheTarget(t *testing.T) {
	in := fullInput(t)
	in.Claim.Status.ResolvedPolicy = &fleetv1alpha1.ResolvedPolicy{DeletionPolicy: pluginsv1alpha1.DeletionPolicyOrphan}

	requests := RequestsInput(in)

	assert.Equal(t, pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim, requests.ClaimKind)
	assert.Equal(t, claimpolicy.KindCamundaManagementCluster, requests.TargetKind)
	assert.Equal(t, fleetv1alpha1.ConditionPlaneRequestsReady, requests.ConditionType)
	assert.Equal(t, pluginsv1alpha1.DeletionPolicyOrphan, requests.DeletionPolicy, "the snapshot default")
	assert.Empty(t, requests.ReleaseRef, "a plane claim has no release")
	assert.Nil(t, requests.AdditionalServiceAccounts)
}
