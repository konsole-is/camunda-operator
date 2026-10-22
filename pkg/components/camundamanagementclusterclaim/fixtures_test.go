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

	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
)

const (
	databaseSection = "management-database"
	accessSection   = "access"
)

// sections are the two plane sections with their slots.
var sections = []pluginsv1alpha1.PluginSection{
	{
		ObjectMeta: metav1.ObjectMeta{Name: databaseSection},
		Spec:       pluginsv1alpha1.PluginSectionSpec{Slot: pluginsv1alpha1.SlotManagementDatabases},
	},
	{
		ObjectMeta: metav1.ObjectMeta{Name: accessSection},
		Spec:       pluginsv1alpha1.PluginSectionSpec{Slot: pluginsv1alpha1.SlotExternalAccess},
	},
}

// registrations serve the two plane sections.
var registrations = []pluginsv1alpha1.PluginRegistration{
	{
		ObjectMeta: metav1.ObjectMeta{Name: "management-database-server"},
		Spec:       pluginsv1alpha1.PluginRegistrationSpec{Sections: []string{databaseSection}},
	},
	{
		ObjectMeta: metav1.ObjectMeta{Name: "test-access"},
		Spec:       pluginsv1alpha1.PluginRegistrationSpec{Sections: []string{accessSection}},
	},
}

// databaseOutputs are the outputs of the database request of my-plane.
var databaseOutputs = map[string]string{
	"keycloakDatabaseConfig":   "my-plane-keycloak",
	"identityDatabaseConfig":   "my-plane-identity",
	"webModelerDatabaseConfig": "my-plane-webmodeler",
}

// fullInput is the input of a claim my-plane in Keycloak mode with Console
// and Web Modeler, which selects both sections. Both requests report their
// outputs.
func fullInput(t *testing.T) Input {
	t.Helper()

	claim := &fleetv1alpha1.CamundaManagementClusterClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "my-plane", Namespace: "my-plane-ns"},
		Spec: fleetv1alpha1.CamundaManagementClusterClaimSpec{
			Environments: []string{"dev", "test"},
			Management: fleetv1alpha1.ManagementOverrides{
				IdentityProvider: fleetv1alpha1.ManagementIdentityProvider{
					Keycloak: &fleetv1alpha1.KeycloakOverrides{Replicas: ptr.To[int32](2)},
				},
				Identity: fleetv1alpha1.IdentityOverrides{
					Admin: v1.IdentityAdminSpec{Username: "admin", Email: "admin@example.com"},
				},
				Console: &fleetv1alpha1.ConsoleOverrides{},
				WebModeler: &fleetv1alpha1.WebModelerOverrides{
					Mail: v1.WebModelerMailSpec{SMTPHost: "smtp.example.com", FromAddress: "modeler@example.com"},
				},
			},
			Versions: fleetv1alpha1.ManagementVersions{
				Identity:   "8.9.10",
				Keycloak:   "26.3.1",
				Console:    "8.9.122",
				WebModeler: "8.9.11",
			},
			Plugins: []fleetv1alpha1.PluginSelection{
				{Section: accessSection, Registration: "test-access"},
				{Section: databaseSection, Registration: "management-database-server"},
			},
		},
	}

	in := Input{
		Claim:             claim,
		PlatformConfigRef: "default",
		Sections:          sections,
		Registrations:     registrations,
		Requests: []pluginsv1alpha1.PluginRequest{
			readyRequest(accessSection, "test-access", map[string]string{"managementHost": "plane.example.com"}),
			readyRequest(databaseSection, "management-database-server", databaseOutputs),
		},
	}

	order, err := claimpolicy.Order(WriteOrder(claim.Spec.Plugins, sections), registrations)
	require.NoError(t, err)
	in.Order = order

	return in
}

// withoutAccess changes in to a claim without an access selection that sets
// every URL in the claim.
func withoutAccess(in Input) Input {
	m := &in.Claim.Spec.Management
	m.IdentityProvider.Keycloak.ExternalURL = "https://login.example.com/auth"
	m.Identity.ExternalURL = "https://plane.example.com/identity"
	m.Console.ExternalURL = "https://console.example.com"
	m.WebModeler.ExternalURL = "https://modeler.example.com"
	m.WebModeler.WebsocketsExternalURL = "https://modeler-ws.example.com"
	in.Claim.Spec.Plugins = in.Claim.Spec.Plugins[1:]
	in.Order = in.Claim.Spec.Plugins
	in.Requests = in.Requests[1:]

	return in
}

// readyRequest returns the request of my-plane in section, with
// OutputsReady=True for its generation and outputs.
func readyRequest(section, registration string, outputs map[string]string) pluginsv1alpha1.PluginRequest {
	return pluginsv1alpha1.PluginRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "my-plane-" + section, Namespace: "my-plane-ns", Generation: 1},
		Spec:       pluginsv1alpha1.PluginRequestSpec{Section: section, Registration: registration},
		Status: pluginsv1alpha1.PluginRequestStatus{
			Conditions: []metav1.Condition{{
				Type:               pluginsv1alpha1.ConditionOutputsReady,
				Status:             metav1.ConditionTrue,
				ObservedGeneration: 1,
			}},
			Outputs: outputs,
		},
	}
}

// notReady removes the OutputsReady condition from the request of section.
func notReady(in Input, section string) {
	for i := range in.Requests {
		if in.Requests[i].Spec.Section == section {
			in.Requests[i].Status.Conditions = nil
		}
	}
}

// previewManagementCluster returns the CamundaManagementCluster that the
// management cluster component of in renders.
func previewManagementCluster(t *testing.T, in Input) *v1.CamundaManagementCluster {
	t.Helper()

	comps, err := Build(in)
	require.NoError(t, err)

	objects, err := comps.ManagementCluster.Preview()
	require.NoError(t, err)
	require.Len(t, objects, 1)

	plane, ok := objects[0].(*v1.CamundaManagementCluster)
	require.True(t, ok, "unexpected %T", objects[0])

	return plane
}
