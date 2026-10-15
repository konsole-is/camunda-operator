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
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

type planeClaim = fleetv1alpha1.CamundaManagementClusterClaim

// oidValue is the claim value of the oidc admin of the examples.
const oidValue = "1f8c0e2a-2b1a-4a5f-9f0d-2b0a1c3d4e5f"

// keycloakVersion is the Keycloak version of the examples, from the 8.9 row
// of the images table.
const keycloakVersion = "26.6.4"

// validPlaneClaim returns the smallest plane claim with a unique name: the
// oidc mode, a claim admin, and the Identity version.
func validPlaneClaim() *planeClaim {
	return &planeClaim{
		ObjectMeta: metav1.ObjectMeta{Name: "plane-" + utilrand.String(8), Namespace: metav1.NamespaceDefault},
		Spec: fleetv1alpha1.CamundaManagementClusterClaimSpec{
			Environments: []string{"production"},
			Management: fleetv1alpha1.ManagementOverrides{
				IdentityProvider: fleetv1alpha1.ManagementIdentityProvider{OIDC: &v1.ManagementOIDCSpec{}},
				Identity: fleetv1alpha1.IdentityOverrides{
					Admin: v1.IdentityAdminSpec{ClaimName: "oid", ClaimValue: oidValue},
				},
			},
			Versions: fleetv1alpha1.ManagementVersions{Identity: "8.9.10"},
		},
	}
}

// realisticPlaneClaim returns a plane claim with a unique name in the
// keycloak mode, with Console and Web Modeler. It sets every URL field.
func realisticPlaneClaim() *planeClaim {
	claim := validPlaneClaim()
	claim.Spec.Management = fleetv1alpha1.ManagementOverrides{
		IdentityProvider: fleetv1alpha1.ManagementIdentityProvider{
			Keycloak: &fleetv1alpha1.KeycloakOverrides{ExternalURL: "https://camunda.example.com/auth"},
		},
		Identity: fleetv1alpha1.IdentityOverrides{
			ExternalURL: "https://camunda.example.com/identity",
			Admin:       v1.IdentityAdminSpec{Username: "admin", Email: "admin@example.com"},
		},
		Console: &fleetv1alpha1.ConsoleOverrides{ExternalURL: "https://camunda.example.com/console"},
		WebModeler: &fleetv1alpha1.WebModelerOverrides{
			ExternalURL:           "https://camunda.example.com/modeler",
			WebsocketsExternalURL: "https://camunda.example.com/modeler-ws",
			Mail: v1.WebModelerMailSpec{
				SMTPHost:    "smtp.example.com",
				FromAddress: "camunda@example.com",
			},
		},
	}
	claim.Spec.Versions = fleetv1alpha1.ManagementVersions{
		Identity:   "8.9.10",
		Keycloak:   keycloakVersion,
		Console:    "8.9.122",
		WebModeler: "8.9.10",
	}

	return claim
}

// nameBoundMessage is the message of the name rule of both claim kinds.
const nameBoundMessage = "metadata.name must be a DNS-1035 label of at most 40 characters"

// fixedSectionsMessage is the message of the rule that keeps the selections of
// a claim.
const fixedSectionsMessage = "sections and their registrations are fixed after creation"

var _ = Describe("CamundaManagementClusterClaim schema", func() {
	DescribeTable(
		"admission",
		func(build func() *planeClaim, mutate func(*planeClaim), wantErr string) {
			obj := build()
			mutate(obj)

			expectAdmission(obj, wantErr)
		},
		Entry("admits the smallest oidc claim", validPlaneClaim, func(*planeClaim) {}, ""),
		Entry("admits a claim that sets every URL field", realisticPlaneClaim, func(*planeClaim) {}, ""),

		Entry(
			"refuses two identity providers",
			validPlaneClaim, func(c *planeClaim) {
				c.Spec.Management.IdentityProvider.Keycloak = &fleetv1alpha1.KeycloakOverrides{}
				c.Spec.Versions.Keycloak = keycloakVersion
			}, "exactly one identity provider: keycloak, externalKeycloak, or oidc",
		),
		Entry(
			"refuses no identity provider",
			validPlaneClaim, func(c *planeClaim) {
				c.Spec.Management.IdentityProvider.OIDC = nil
			}, "exactly one identity provider: keycloak, externalKeycloak, or oidc",
		),

		Entry(
			"refuses the keycloak mode without versions.keycloak",
			realisticPlaneClaim, func(c *planeClaim) { c.Spec.Versions.Keycloak = "" },
			"versions.keycloak is required with the keycloak identity provider",
		),
		Entry(
			"refuses Console without versions.console",
			realisticPlaneClaim, func(c *planeClaim) { c.Spec.Versions.Console = "" },
			"versions.console is required when management.console is set",
		),
		Entry(
			"refuses Web Modeler without versions.webModeler",
			realisticPlaneClaim, func(c *planeClaim) { c.Spec.Versions.WebModeler = "" },
			"versions.webModeler is required when management.webModeler is set",
		),
		Entry(
			"admits versions.keycloak in the oidc mode",
			validPlaneClaim, func(c *planeClaim) { c.Spec.Versions.Keycloak = keycloakVersion }, "",
		),
		Entry(
			"refuses a versions.keycloak of two parts",
			realisticPlaneClaim, func(c *planeClaim) { c.Spec.Versions.Keycloak = "26.6" },
			"spec.versions.keycloak in body should match",
		),
		Entry(
			"refuses an empty versions.identity",
			validPlaneClaim, func(c *planeClaim) { c.Spec.Versions.Identity = "" },
			"spec.versions.identity in body should match",
		),

		Entry(
			"refuses a Keycloak URL without the /auth path",
			realisticPlaneClaim, func(c *planeClaim) {
				c.Spec.Management.IdentityProvider.Keycloak.ExternalURL = "https://kc.example.com"
			}, "externalUrl must carry the /auth path",
		),
		Entry(
			"refuses a Keycloak URL with a path in front of /auth",
			realisticPlaneClaim, func(c *planeClaim) {
				c.Spec.Management.IdentityProvider.Keycloak.ExternalURL = "https://kc.example.com/sso/auth"
			}, "externalUrl must carry the /auth path",
		),
		Entry(
			"refuses a Keycloak URL without a scheme",
			realisticPlaneClaim, func(c *planeClaim) {
				c.Spec.Management.IdentityProvider.Keycloak.ExternalURL = "kc.example.com/auth"
			}, "externalUrl must be a valid http or https URL",
		),
		Entry(
			"refuses a Keycloak URL with a query",
			realisticPlaneClaim, func(c *planeClaim) {
				c.Spec.Management.IdentityProvider.Keycloak.ExternalURL = "https://kc.example.com/auth?tenant=a"
			}, "externalUrl must carry no query and no fragment",
		),
		Entry(
			"admits a keycloak mode without a Keycloak URL",
			realisticPlaneClaim, func(c *planeClaim) {
				c.Spec.Management.IdentityProvider.Keycloak.ExternalURL = ""
			}, "",
		),

		Entry(
			"refuses an http Identity URL with a path",
			realisticPlaneClaim, func(c *planeClaim) {
				c.Spec.Management.Identity.ExternalURL = "http://camunda.example.com/identity"
			}, "externalUrl with a path needs https",
		),
		Entry(
			"admits an http Identity URL without a path",
			realisticPlaneClaim, func(c *planeClaim) {
				c.Spec.Management.Identity.ExternalURL = "http://identity.example.com"
			}, "",
		),
		pathRuleEntries("Identity", "externalUrl", func(c *planeClaim, url string) {
			c.Spec.Management.Identity.ExternalURL = url
		}),
		pathRuleEntries("Console", "externalUrl", func(c *planeClaim, url string) {
			c.Spec.Management.Console.ExternalURL = url
		}),
		pathRuleEntries("Web Modeler", "externalUrl", func(c *planeClaim, url string) {
			c.Spec.Management.WebModeler.ExternalURL = url
		}),
		pathRuleEntries("Web Modeler websockets", "websocketsExternalUrl", func(c *planeClaim, url string) {
			c.Spec.Management.WebModeler.WebsocketsExternalURL = url
		}),

		Entry(
			"refuses an Identity extraEnv that sets the Keycloak URL",
			validPlaneClaim, func(c *planeClaim) {
				c.Spec.Management.Identity.ExtraEnv = []corev1.EnvVar{
					{Name: "KEYCLOAK_URL", Value: "https://elsewhere.example.com/auth"},
				}
			}, "must not set KEYCLOAK_URL or KEYCLOAK_REALM",
		),
		Entry(
			"admits an Identity extraEnv of another variable",
			validPlaneClaim, func(c *planeClaim) {
				c.Spec.Management.Identity.ExtraEnv = []corev1.EnvVar{{Name: "IDENTITY_LOG_LEVEL", Value: "DEBUG"}}
			}, "",
		),

		Entry(
			"refuses an empty environments",
			validPlaneClaim, func(c *planeClaim) { c.Spec.Environments = []string{} },
			"should have at least 1 items",
		),
		Entry(
			"refuses an environment name of 64 characters",
			validPlaneClaim, func(c *planeClaim) { c.Spec.Environments = []string{strings.Repeat("e", 64)} },
			"Too long",
		),
		Entry(
			"refuses an environment named twice",
			validPlaneClaim, func(c *planeClaim) { c.Spec.Environments = []string{"production", "production"} },
			"Duplicate value",
		),
		Entry(
			"admits two environments",
			validPlaneClaim, func(c *planeClaim) { c.Spec.Environments = []string{"production", "staging"} }, "",
		),

		Entry(
			"refuses a claim name of 41 characters",
			validPlaneClaim, func(c *planeClaim) { c.Name = "p" + strings.Repeat("a", 40) },
			nameBoundMessage,
		),
		Entry(
			"admits a claim name of 40 characters",
			validPlaneClaim, func(c *planeClaim) { c.Name = "p" + utilrand.String(39) }, "",
		),
		Entry(
			"refuses a claim name that starts with a digit",
			validPlaneClaim, func(c *planeClaim) { c.Name = "1" + utilrand.String(8) },
			nameBoundMessage,
		),

		Entry(
			"refuses 33 selections",
			validPlaneClaim, func(c *planeClaim) { c.Spec.Plugins = selections(33) }, "Too many",
		),
		Entry(
			"admits 32 selections",
			validPlaneClaim, func(c *planeClaim) { c.Spec.Plugins = selections(32) }, "",
		),
		Entry(
			"refuses a managementAuthConfigName that is not a DNS name",
			validPlaneClaim, func(c *planeClaim) { c.Spec.ManagementAuthConfigName = "Plane_Auth" },
			"spec.managementAuthConfigName in body should match",
		),
		Entry(
			"refuses a platformConfigRef that is not a DNS name",
			validPlaneClaim, func(c *planeClaim) { c.Spec.PlatformConfigRef = "Platform_Config" },
			"spec.platformConfigRef in body should match",
		),
		Entry(
			"refuses an unknown deletionPolicy",
			validPlaneClaim, func(c *planeClaim) { c.Spec.DeletionPolicy = pluginsv1alpha1.DeletionPolicy("Keep") },
			`Unsupported value: "Keep"`,
		),
	)

	// The typed client drops an empty string under omitempty, so only an
	// unstructured object carries one to the API server.
	It("refuses an explicitly empty Identity URL", func() {
		u := toUnstructured(validPlaneClaim(), "CamundaManagementClusterClaim")
		Expect(unstructured.SetNestedField(u.Object, "", "spec", "management", "identity", "externalUrl")).To(Succeed())

		expectAdmission(u, "externalUrl must be a valid http or https URL")
	})

	DescribeTable(
		"update",
		func(build func() *planeClaim, change func(*planeClaim), wantErr string) {
			obj := build()
			expectAdmission(obj, "")

			change(obj)
			expectUpdate(obj, wantErr)
		},
		Entry(
			"refuses a change of a registration",
			withSelections, func(c *planeClaim) { c.Spec.Plugins[0].Registration = "other-access" },
			fixedSectionsMessage,
		),
		Entry(
			"refuses a new selection",
			withSelections, func(c *planeClaim) {
				c.Spec.Plugins = append(c.Spec.Plugins, fleetv1alpha1.PluginSelection{
					Section:      "cloud-identity",
					Registration: "aws-identity",
				})
			}, fixedSectionsMessage,
		),
		Entry(
			"refuses the removal of a selection",
			withSelections, func(c *planeClaim) { c.Spec.Plugins = c.Spec.Plugins[:1] },
			fixedSectionsMessage,
		),
		Entry(
			"refuses the removal of every selection",
			withSelections, func(c *planeClaim) { c.Spec.Plugins = nil },
			fixedSectionsMessage,
		),
		Entry(
			"admits a change of the parameters only",
			withSelections, func(c *planeClaim) {
				c.Spec.Plugins[0].Parameters = &apiextensionsv1.JSON{Raw: []byte(`{"host":"plane.example.com"}`)}
			}, "",
		),
		Entry(
			"refuses a first selection on a claim created without one",
			validPlaneClaim, func(c *planeClaim) { c.Spec.Plugins = selections(1) }, fixedSectionsMessage,
		),
		Entry(
			"admits a move from the keycloak mode to the oidc mode with a claim admin",
			realisticPlaneClaim, func(c *planeClaim) {
				c.Spec.Management.IdentityProvider = fleetv1alpha1.ManagementIdentityProvider{
					OIDC: &v1.ManagementOIDCSpec{},
				}
				c.Spec.Management.Identity.Admin = v1.IdentityAdminSpec{ClaimName: "oid", ClaimValue: oidValue}
			}, "",
		),
		Entry(
			"admits an update of an existing claim without a name check",
			validPlaneClaim, func(c *planeClaim) { c.Spec.Suspend = true }, "",
		),
	)
})

// pathRuleEntries returns the entries of the rules that every URL field of
// Identity, Console, and Web Modeler has. label names the field in the
// entry descriptions, and field starts each message.
func pathRuleEntries(label, field string, set func(*planeClaim, string)) []TableEntry {
	entry := func(description, url, wantErr string) TableEntry {
		return Entry(description, realisticPlaneClaim, func(c *planeClaim) { set(c, url) }, wantErr)
	}

	return []TableEntry{
		entry(
			"refuses the "+label+" URL without a scheme",
			"camunda.example.com", field+" must be a valid http or https URL",
		),
		entry(
			"refuses the "+label+" URL with a percent-encoded character",
			"https://camunda.example.com/app%2F", field+" must carry no percent-encoded character",
		),
		entry(
			"refuses the "+label+" URL with a backslash",
			`https://camunda.example.com/..\app`, field+" must carry no backslash",
		),
		entry(
			"refuses the "+label+" URL with a dot segment",
			"https://camunda.example.com/a/../app", field+" must carry no . or .. path segment",
		),
		entry("admits the "+label+" URL with dots inside a segment", "https://camunda.example.com/v1.2/.app", ""),
	}
}

// selections returns n selections, each in a section of its own.
func selections(n int) []fleetv1alpha1.PluginSelection {
	out := make([]fleetv1alpha1.PluginSelection, n)
	for i := range out {
		out[i] = fleetv1alpha1.PluginSelection{
			Section:      fmt.Sprintf("section-%d", i),
			Registration: fmt.Sprintf("registration-%d", i),
		}
	}

	return out
}

// withSelections returns validPlaneClaim with an access and a
// management-database selection.
func withSelections() *planeClaim {
	claim := validPlaneClaim()
	claim.Spec.Plugins = []fleetv1alpha1.PluginSelection{
		{Section: "access", Registration: "gateway-access"},
		{Section: "management-database", Registration: "cloudnative-pg"},
	}

	return claim
}
