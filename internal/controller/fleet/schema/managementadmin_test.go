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

package schema

import (
	. "github.com/onsi/ginkgo/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// identityMode is one of the three identity provider modes of a management
// plane.
type identityMode string

const (
	modeOIDC             identityMode = "oidc"
	modeKeycloak         identityMode = "keycloak"
	modeExternalKeycloak identityMode = "externalKeycloak"
)

// The admin rules exist twice: on CamundaManagementClusterSpec and on
// ManagementOverrides. Each case creates both kinds with the same admin, so a
// change to one copy alone fails here.
var _ = Describe("the admin rules of a management plane", func() {
	DescribeTable(
		"admission of a CamundaManagementCluster and of a plane claim",
		func(mode identityMode, admin v1.IdentityAdminSpec, webModeler bool, wantErr string) {
			By("creating the CamundaManagementCluster")
			expectAdmission(managementClusterFor(mode, admin, webModeler), wantErr)

			By("creating the plane claim")
			expectAdmission(planeClaimFor(mode, admin, webModeler), wantErr)
		},
		Entry("admits a claim admin in the oidc mode", modeOIDC, claimAdmin(), false, ""),
		Entry(
			"refuses a username admin in the oidc mode",
			modeOIDC, v1.IdentityAdminSpec{Username: "admin"}, false,
			"identity.admin: set claimName and claimValue in oidc mode, username in the keycloak modes",
		),
		Entry(
			"refuses a claim admin in the keycloak mode",
			modeKeycloak, claimAdmin(), false,
			"identity.admin: set claimName and claimValue in oidc mode, username in the keycloak modes",
		),
		Entry(
			"refuses a claim admin in the externalKeycloak mode",
			modeExternalKeycloak, claimAdmin(), false,
			"identity.admin: set claimName and claimValue in oidc mode, username in the keycloak modes",
		),
		Entry(
			"admits a username admin in the keycloak mode",
			modeKeycloak, v1.IdentityAdminSpec{Username: "admin"}, false, "",
		),
		Entry(
			"admits a username admin in the externalKeycloak mode",
			modeExternalKeycloak, v1.IdentityAdminSpec{Username: "admin"}, false, "",
		),
		Entry(
			"refuses a passwordSecretRef in the oidc mode",
			modeOIDC, withPassword(claimAdmin()), false,
			"identity.admin.passwordSecretRef applies to the keycloak modes only",
		),
		Entry(
			"admits a passwordSecretRef in the keycloak mode",
			modeKeycloak, withPassword(v1.IdentityAdminSpec{Username: "admin"}), false, "",
		),
		Entry(
			"refuses Web Modeler without an admin email in the keycloak mode",
			modeKeycloak, v1.IdentityAdminSpec{Username: "admin"}, true,
			"identity.admin.email is required when webModeler is set in a keycloak mode",
		),
		Entry(
			"refuses Web Modeler without an admin email in the externalKeycloak mode",
			modeExternalKeycloak, v1.IdentityAdminSpec{Username: "admin"}, true,
			"identity.admin.email is required when webModeler is set in a keycloak mode",
		),
		Entry(
			"admits Web Modeler with an admin email in the keycloak mode",
			modeKeycloak, v1.IdentityAdminSpec{Username: "admin", Email: "admin@example.com"}, true, "",
		),
		Entry("admits Web Modeler without an admin email in the oidc mode", modeOIDC, claimAdmin(), true, ""),
	)
})

func claimAdmin() v1.IdentityAdminSpec {
	return v1.IdentityAdminSpec{ClaimName: "oid", ClaimValue: oidValue}
}

func withPassword(admin v1.IdentityAdminSpec) v1.IdentityAdminSpec {
	admin.PasswordSecretRef = &v1.LocalSecretKeyRef{Name: "admin-credentials", Key: "password"}

	return admin
}

// managementClusterFor returns a CamundaManagementCluster with a unique name
// in mode, with admin, and with Web Modeler when webModeler is true.
func managementClusterFor(mode identityMode, admin v1.IdentityAdminSpec, webModeler bool) *v1.CamundaManagementCluster {
	mc := &v1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "mc-" + utilrand.String(8), Namespace: metav1.NamespaceDefault},
		Spec: v1.CamundaManagementClusterSpec{
			PlatformConfigRef: "my-platform",
			Identity: v1.IdentitySpec{
				Version:           "8.9.10",
				ExternalURL:       "https://identity.example.com",
				DatabaseConfigRef: "identity-db",
				Admin:             admin,
			},
		},
	}

	switch mode {
	case modeOIDC:
		mc.Spec.IdentityProvider.OIDC = &v1.ManagementOIDCSpec{}
	case modeKeycloak:
		mc.Spec.IdentityProvider.Keycloak = &v1.ManagedKeycloakSpec{
			Version:           "26.6.4",
			ExternalURL:       "https://keycloak.example.com/auth",
			DatabaseConfigRef: "keycloak-db",
		}
	case modeExternalKeycloak:
		mc.Spec.IdentityProvider.ExternalKeycloak = externalKeycloak()
	}

	if webModeler {
		mc.Spec.WebModeler = &v1.WebModelerSpec{
			Version:               "8.9.10",
			ExternalURL:           "https://modeler.example.com",
			WebsocketsExternalURL: "https://modeler-ws.example.com",
			DatabaseConfigRef:     "web-modeler-db",
			Mail:                  mail(),
		}
	}

	return mc
}

// planeClaimFor returns a plane claim with a unique name in mode, with admin,
// and with Web Modeler when webModeler is true.
func planeClaimFor(mode identityMode, admin v1.IdentityAdminSpec, webModeler bool) *planeClaim {
	claim := validPlaneClaim()
	claim.Spec.Management.Identity.Admin = admin

	switch mode {
	case modeOIDC:
		claim.Spec.Management.IdentityProvider = fleetv1alpha1.ManagementIdentityProvider{
			OIDC: &v1.ManagementOIDCSpec{},
		}
	case modeKeycloak:
		claim.Spec.Management.IdentityProvider = fleetv1alpha1.ManagementIdentityProvider{
			Keycloak: &fleetv1alpha1.KeycloakOverrides{},
		}
		claim.Spec.Versions.Keycloak = "26.6.4"
	case modeExternalKeycloak:
		claim.Spec.Management.IdentityProvider = fleetv1alpha1.ManagementIdentityProvider{
			ExternalKeycloak: externalKeycloak(),
		}
	}

	if webModeler {
		claim.Spec.Management.WebModeler = &fleetv1alpha1.WebModelerOverrides{Mail: mail()}
		claim.Spec.Versions.WebModeler = "8.9.10"
	}

	return claim
}

func externalKeycloak() *v1.ExternalKeycloakSpec {
	return &v1.ExternalKeycloakSpec{
		URL: "https://keycloak.example.com/auth",
		AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
			Name:        "keycloak-admin",
			UsernameKey: "username",
			PasswordKey: "password",
		},
	}
}

func mail() v1.WebModelerMailSpec {
	return v1.WebModelerMailSpec{SMTPHost: "smtp.example.com", FromAddress: "camunda@example.com"}
}
