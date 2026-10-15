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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

type planeClaim = fleetv1alpha1.CamundaManagementClusterClaim

// oidValue is the claim value of the oidc admin of the examples.
const oidValue = "1f8c0e2a-2b1a-4a5f-9f0d-2b0a1c3d4e5f"

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
		Keycloak:   "26.6.4",
		Console:    "8.9.122",
		WebModeler: "8.9.10",
	}

	return claim
}
