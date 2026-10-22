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
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

var _ = Describe("CamundaManagementClusterClaim controller", func() {
	It("writes no PluginRequest before status.resolvedPolicy is recorded", func() {
		claim := newPlaneClaim()
		create(claim)

		Eventually(func() error {
			_, err := requestOf(claim, pluginstartup.SectionManagementDatabase)
			return err
		}, timeout, interval).Should(Succeed())
		recorded := witness.recorded(client.ObjectKeyFromObject(claim))
		Expect(recorded).NotTo(BeEmpty())
		Expect(recorded).To(HaveEach(BeTrue()))
		Expect(latest(Default, claim).Finalizers).To(ContainElement(fleetv1alpha1.FinalizerCleanup))
	})

	It("writes the plane only after both requests have outputs", func() {
		plugin.hold(registrationAccess)
		DeferCleanup(plugin.release, registrationAccess)
		claim := newPlaneClaim()
		create(claim)

		Eventually(func(g Gomega) {
			request, err := requestOf(claim, pluginstartup.SectionManagementDatabase)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(meta.IsStatusConditionTrue(request.Status.Conditions, pluginsv1alpha1.ConditionOutputsReady)).
				To(BeTrue())
		}, timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			_, err := planeOf(claim)
			g.Expect(err).To(HaveOccurred())
			g.Expect(meta.IsStatusConditionTrue(
				latest(g, claim).Status.Conditions,
				fleetv1alpha1.ConditionPlaneRequestsReady,
			)).To(BeFalse())
		}, consistently, interval).Should(Succeed())

		plugin.release(registrationAccess)

		eventuallyPlane(claim, nil)
	})

	It("selects the namespaces of the served environments, and follows a change of the list", func() {
		first, second := planeEnvironment(), planeEnvironment()
		createEnvironment(second)
		claim := newClaim(newEnvironmentNamespace(first), first.Name)
		create(claim)

		eventuallyPlane(claim, func(g Gomega, plane *v1.CamundaManagementCluster) {
			g.Expect(plane.Spec.NamespaceSelector).To(Equal(&metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{{
					Key:      fleetv1alpha1.LabelEnvironment,
					Operator: metav1.LabelSelectorOpIn,
					Values:   []string{first.Name},
				}},
			}))
		})

		update(claim, func(c *fleetv1alpha1.CamundaManagementClusterClaim) {
			c.Spec.Environments = []string{first.Name, second.Name}
		})

		eventuallyPlane(claim, func(g Gomega, plane *v1.CamundaManagementCluster) {
			g.Expect(plane.Spec.NamespaceSelector.MatchExpressions).To(HaveLen(1))
			g.Expect(plane.Spec.NamespaceSelector.MatchExpressions[0].Values).
				To(Equal([]string{first.Name, second.Name}))
		})
	})

	It("passes the databases of the identity mode, and drops Web Modeler when it is turned off", func() {
		claim := newPlaneClaim()
		create(claim)
		expectDatabases(claim, "keycloak", "identity", "webModeler")

		update(claim, func(c *fleetv1alpha1.CamundaManagementClusterClaim) {
			c.Spec.Management.WebModeler = nil
			c.Spec.Versions.WebModeler = ""
		})

		expectDatabases(claim, "keycloak", "identity")
	})

	It("moves the plane to a new identity mode", func() {
		claim := newPlaneClaim()
		create(claim)
		eventuallyPlane(claim, func(g Gomega, plane *v1.CamundaManagementCluster) {
			g.Expect(plane.Spec.IdentityProvider.Keycloak).NotTo(BeNil())
		})

		update(claim, func(c *fleetv1alpha1.CamundaManagementClusterClaim) {
			c.Spec.Management.IdentityProvider = fleetv1alpha1.ManagementIdentityProvider{
				OIDC: &v1.ManagementOIDCSpec{},
			}
			c.Spec.Management.Identity.Admin = v1.IdentityAdminSpec{ClaimName: "groups", ClaimValue: "admins"}
			c.Spec.Versions.Keycloak = ""
		})

		eventuallyPlane(claim, func(g Gomega, plane *v1.CamundaManagementCluster) {
			g.Expect(plane.Spec.IdentityProvider.OIDC).NotTo(BeNil())
			g.Expect(plane.Spec.IdentityProvider.Keycloak).To(BeNil())
			g.Expect(plane.Spec.Identity.Admin).
				To(Equal(v1.IdentityAdminSpec{ClaimName: "groups", ClaimValue: "admins"}))
		})
		expectDatabases(claim, "identity", "webModeler")
	})

	It("builds the five URLs from the access host", func() {
		claim := newPlaneClaim()
		create(claim)

		eventuallyPlane(claim, func(g Gomega, plane *v1.CamundaManagementCluster) {
			host := "https://" + managementHost
			g.Expect(plane.Spec.IdentityProvider.Keycloak.ExternalURL).To(Equal(host + "/auth"))
			g.Expect(plane.Spec.Identity.ExternalURL).To(Equal(host + "/identity"))
			g.Expect(plane.Spec.Console.ExternalURL).To(Equal(host + "/console"))
			g.Expect(plane.Spec.WebModeler.ExternalURL).To(Equal(host + "/modeler"))
			g.Expect(plane.Spec.WebModeler.WebsocketsExternalURL).To(Equal(host + "/modeler-ws"))
		})
	})

	It("passes the URLs of the claim without an access selection", func() {
		claim := newPlaneClaim()
		setURLs(claim)
		create(claim)

		eventuallyPlane(claim, func(g Gomega, plane *v1.CamundaManagementCluster) {
			g.Expect(plane.Spec.IdentityProvider.Keycloak.ExternalURL).To(Equal("https://login.example.com/auth"))
			g.Expect(plane.Spec.Identity.ExternalURL).To(Equal("https://own.example.com/identity"))
			g.Expect(plane.Spec.Console.ExternalURL).To(Equal("https://console.example.com"))
			g.Expect(plane.Spec.WebModeler.ExternalURL).To(Equal("https://modeler.example.com"))
			g.Expect(plane.Spec.WebModeler.WebsocketsExternalURL).To(Equal("https://modeler-ws.example.com"))
		})
	})

	It("passes the versions through, and the database references from the outputs", func() {
		claim := newPlaneClaim()
		create(claim)

		eventuallyPlane(claim, func(g Gomega, plane *v1.CamundaManagementCluster) {
			g.Expect(plane.Spec.Identity.Version).To(Equal("8.9.10"))
			g.Expect(plane.Spec.IdentityProvider.Keycloak.Version).To(Equal("26.3.1"))
			g.Expect(plane.Spec.Console.Version).To(Equal("8.9.122"))
			g.Expect(plane.Spec.WebModeler.Version).To(Equal("8.9.11"))
			g.Expect(plane.Spec.IdentityProvider.Keycloak.DatabaseConfigRef).To(Equal("plane-keycloak"))
			g.Expect(plane.Spec.Identity.DatabaseConfigRef).To(Equal("plane-identity"))
			g.Expect(plane.Spec.WebModeler.DatabaseConfigRef).To(Equal("plane-webmodeler"))
			g.Expect(plane.Spec.PlatformConfigRef).To(Equal(platformConfigName))
			g.Expect(metav1.IsControlledBy(plane, latest(g, claim))).To(BeTrue())
		})
	})

	It("shows the Ready reason and message of the plane on ManagementClusterReady", func() {
		claim := newPlaneClaim()
		create(claim)
		eventuallyPlane(claim, nil)

		setPlaneReady(claim, metav1.ConditionFalse, "Creating", "Management Identity is starting")

		expectCondition(
			claim,
			fleetv1alpha1.ConditionManagementClusterReady,
			metav1.ConditionFalse,
			"Creating",
			ContainSubstring("Creating: Management Identity is starting"),
		)

		setPlaneReady(claim, metav1.ConditionTrue, v1.ReasonHealthy, "The plane is ready")

		expectCondition(claim, v1.ConditionReady, metav1.ConditionTrue, v1.ReasonHealthy, Not(BeEmpty()))
	})

	It("names the plane and the requests in status", func() {
		claim := newPlaneClaim()
		create(claim)

		Eventually(func(g Gomega) {
			current := latest(g, claim)
			g.Expect(current.Status.ManagementCluster).To(Equal(&fleetv1alpha1.ManagementClusterRef{Name: claim.Name}))
			g.Expect(current.Status.Requests).To(ConsistOf(
				fleetv1alpha1.RequestStatus{
					Section:      pluginstartup.SectionAccess,
					Registration: registrationAccess,
					Name:         claim.Name + "-" + pluginstartup.SectionAccess,
					OutputsReady: metav1.ConditionTrue,
					Ready:        metav1.ConditionTrue,
				},
				fleetv1alpha1.RequestStatus{
					Section:      pluginstartup.SectionManagementDatabase,
					Registration: pluginstartup.RegistrationManagementDatabaseServer,
					Name:         claim.Name + "-" + pluginstartup.SectionManagementDatabase,
					OutputsReady: metav1.ConditionTrue,
					Ready:        metav1.ConditionTrue,
				},
			))
		}, timeout, interval).Should(Succeed())
	})

	It("refuses an external URL with an access selection, and keeps the plane as it is", func() {
		claim := newPlaneClaim()
		create(claim)
		eventuallyPlane(claim, nil)

		update(claim, func(c *fleetv1alpha1.CamundaManagementClusterClaim) {
			c.Spec.Management.Identity.ExternalURL = "https://own.example.com/identity"
		})

		expectFalse(
			claim, v1.ConditionReady, fleetv1alpha1.ReasonSlotConflict,
			ContainSubstring("spec.management.identity.externalUrl"),
		)
		Consistently(func(g Gomega) {
			plane, err := planeOf(claim)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(plane.Spec.Identity.ExternalURL).To(Equal("https://" + managementHost + "/identity"))
		}, consistently, interval).Should(Succeed())
	})

	It("applies only the fields that the claim sets", func() {
		claim := newPlaneClaim()
		claim.Spec.Management = fleetv1alpha1.ManagementOverrides{
			IdentityProvider: fleetv1alpha1.ManagementIdentityProvider{OIDC: &v1.ManagementOIDCSpec{}},
			Identity: fleetv1alpha1.IdentityOverrides{
				Admin: v1.IdentityAdminSpec{ClaimName: "groups", ClaimValue: "admins"},
			},
		}
		claim.Spec.Versions = fleetv1alpha1.ManagementVersions{Identity: "8.9.10"}
		create(claim)

		plane := eventuallyPlane(claim, nil)

		var fields map[string]any
		for _, entry := range plane.ManagedFields {
			if strings.HasPrefix(entry.Manager, fleetv1alpha1.FieldManagerPrefixPlaneClaim) {
				Expect(json.Unmarshal(entry.FieldsV1.Raw, &fields)).To(Succeed())
			}
		}
		Expect(fields).To(HaveKey("f:spec"))
		spec, ok := fields["f:spec"].(map[string]any)
		Expect(ok).To(BeTrue())
		for _, unset := range []string{"f:console", "f:webModeler", "f:clusterSelector", "f:suspend"} {
			Expect(spec).NotTo(HaveKey(unset))
		}
	})
})

// expectDatabases polls until the databases parameter of the database
// request of claim is want.
func expectDatabases(claim *fleetv1alpha1.CamundaManagementClusterClaim, want ...string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		request, err := requestOf(claim, pluginstartup.SectionManagementDatabase)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(request.Spec.Parameters).NotTo(BeNil())

		var parameters struct {
			Databases []string `json:"databases"`
		}
		g.Expect(json.Unmarshal(request.Spec.Parameters.Raw, &parameters)).To(Succeed())
		g.Expect(parameters.Databases).To(Equal(want))
	}, timeout, interval).Should(Succeed())
}
