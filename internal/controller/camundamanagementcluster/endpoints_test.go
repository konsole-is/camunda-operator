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

package camundamanagementcluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sourcehawk/operator-component-framework/pkg/component"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// expectEndpoints polls until the published management cluster satisfies
// match.
func expectEndpoints(mc *v1.CamundaManagementCluster, match func(Gomega, *v1.CamundaManagementCluster)) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		match(g, readManagementCluster(g, mc))
	}, timeout, interval).Should(Succeed())
}

var _ = Describe("CamundaManagementCluster endpoints", func() {
	It("publishes the routes of a new plane in the keycloak mode", func() {
		s := newScenario(withManagedKeycloak, func(f *fixture) {
			f.mc.Spec.Identity.ExternalURL = "https://camunda.example.com/identity"
		})
		http := v1.EndpointProtocolHTTP

		expectEndpoints(s.mc, func(g Gomega, latest *v1.CamundaManagementCluster) {
			g.Expect(latest.Status.Endpoints).To(Equal([]v1.Endpoint{
				{Name: "identity", Service: s.mc.Name + "-identity", Port: 80, PathPrefix: "/identity", Protocol: http},
				{
					Name: "keycloak", Service: s.mc.Name + "-keycloak-service", Port: 8080, PathPrefix: "/auth",
					Protocol: http,
				},
			}))
		})
	})

	// The oidc mode runs no Keycloak, so an access plugin must not route
	// /auth to a Service that does not exist.
	It("publishes no keycloak route for a plane in the oidc mode", func() {
		s := newScenario(withConsole, withWebModeler)
		http := v1.EndpointProtocolHTTP

		expectEndpoints(s.mc, func(g Gomega, latest *v1.CamundaManagementCluster) {
			g.Expect(latest.Status.Endpoints).To(Equal([]v1.Endpoint{
				{Name: "console", Service: s.mc.Name + "-console", Port: 80, PathPrefix: "/", Protocol: http},
				{Name: "identity", Service: s.mc.Name + "-identity", Port: 80, PathPrefix: "/", Protocol: http},
				{
					Name: "web-modeler", Service: s.mc.Name + "-web-modeler-restapi", Port: 80, PathPrefix: "/",
					Protocol: http,
				},
				{
					Name: "web-modeler-websockets", Service: s.mc.Name + "-web-modeler-websockets", Port: 80,
					PathPrefix: "/modeler-ws", Protocol: http,
				},
			}))
		})
	})

	// The Services stay while the pods are at zero, so a route answers again
	// when the plane resumes, without a new hostname or certificate.
	It("keeps its routes while suspended", func() {
		s := newScenario(withConsole)

		var before []v1.Endpoint
		expectEndpoints(s.mc, func(g Gomega, latest *v1.CamundaManagementCluster) {
			g.Expect(latest.Status.Endpoints).NotTo(BeEmpty())
			before = latest.Status.Endpoints
		})

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.Suspend = true
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// Only a pass that read spec.suspend reports the suspension, and the
		// same flush writes the routes of that pass.
		expectEndpoints(s.mc, func(g Gomega, latest *v1.CamundaManagementCluster) {
			identity := conditionOf(g, latest, v1.ConditionIdentityReady)
			g.Expect(identity.Reason).To(BeElementOf(string(component.Suspending), string(component.Suspended)))
			g.Expect(latest.Status.Endpoints).To(Equal(before))
		})
	})
})
