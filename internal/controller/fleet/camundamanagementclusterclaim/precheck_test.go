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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

var _ = Describe("CamundaManagementClusterClaim precheck", func() {
	DescribeTable(
		"refuses the claim and writes nothing",
		func(claimOf func() *fleetv1alpha1.CamundaManagementClusterClaim, reason, message string) {
			claim := claimOf()
			create(claim)

			expectFalse(claim, v1.ConditionReady, reason, ContainSubstring(message))
			Consistently(func(g Gomega) {
				current := latest(g, claim)
				g.Expect(current.Status.ResolvedPolicy).To(BeNil())
				g.Expect(current.Finalizers).To(BeEmpty())

				var requests pluginsv1alpha1.PluginRequestList
				g.Expect(k8sClient.List(ctx, &requests, client.InNamespace(claim.Namespace))).To(Succeed())
				g.Expect(requests.Items).To(BeEmpty())

				var planes v1.CamundaManagementClusterList
				g.Expect(k8sClient.List(ctx, &planes, client.InNamespace(claim.Namespace))).To(Succeed())
				g.Expect(planes.Items).To(BeEmpty())
			}, consistently, interval).Should(Succeed())
		},
		Entry(
			"a namespace without the label while an Environment exists",
			func() *fleetv1alpha1.CamundaManagementClusterClaim {
				environment := planeEnvironment()
				createEnvironment(environment)
				return newClaim(newNamespace(nil), environment.Name)
			}, fleetv1alpha1.ReasonNoEnvironment, fleetv1alpha1.LabelEnvironment,
		),
		Entry(
			"a label that names no Environment", func() *fleetv1alpha1.CamundaManagementClusterClaim {
				return newClaim(newNamespace(map[string]string{fleetv1alpha1.LabelEnvironment: "missing"}), "missing")
			}, fleetv1alpha1.ReasonUnknownEnvironment, `"missing"`,
		),
		Entry(
			"a served environment that does not exist", func() *fleetv1alpha1.CamundaManagementClusterClaim {
				environment := planeEnvironment()
				return newClaim(newEnvironmentNamespace(environment), environment.Name, "nope", "gone")
			}, v1.ReasonInvalidReference, `Environment "nope", Environment "gone"`,
		),
		Entry(
			"a platform config that does not exist", func() *fleetv1alpha1.CamundaManagementClusterClaim {
				claim := newPlaneClaim()
				claim.Spec.PlatformConfigRef = "missing-platform"
				return claim
			}, v1.ReasonInvalidReference, `CamundaPlatformConfig "missing-platform"`,
		),
		Entry(
			"a section that does not apply to a plane", func() *fleetv1alpha1.CamundaManagementClusterClaim {
				claim := newPlaneClaim()
				claim.Spec.Plugins = append(
					claim.Spec.Plugins,
					selection(pluginstartup.SectionCloudIdentity, registrationIdentity),
				)
				return claim
			}, fleetv1alpha1.ReasonNotAllowed, `section "cloud-identity" does not apply`,
		),
		Entry(
			"a registration that the Environment does not allow", func() *fleetv1alpha1.CamundaManagementClusterClaim {
				environment := planeEnvironment()
				environment.Spec.Sections[pluginstartup.SectionAccess] = fleetv1alpha1.SectionPolicy{}
				return newClaim(newEnvironmentNamespace(environment), environment.Name)
			}, fleetv1alpha1.ReasonNotAllowed, `registration "test-access" is not allowed`,
		),
		Entry(
			"Orphan with a registration that does not list Orphan",
			func() *fleetv1alpha1.CamundaManagementClusterClaim {
				claim := newPlaneClaim()
				claim.Spec.Plugins[1] = selection(pluginstartup.SectionManagementDatabase, registrationDeleteOnly)
				claim.Spec.DeletionPolicy = pluginsv1alpha1.DeletionPolicyOrphan
				return claim
			}, fleetv1alpha1.ReasonNotAllowed, `section "`+pluginstartup.SectionManagementDatabase+`"`,
		),
		Entry(
			"an external URL with an access selection", func() *fleetv1alpha1.CamundaManagementClusterClaim {
				claim := newPlaneClaim()
				claim.Spec.Management.Identity.ExternalURL = "https://own.example.com/identity"
				return claim
			}, fleetv1alpha1.ReasonSlotConflict, "spec.management.identity.externalUrl",
		),
		Entry(
			"a missing URL without an access selection", func() *fleetv1alpha1.CamundaManagementClusterClaim {
				claim := newPlaneClaim()
				setURLs(claim)
				claim.Spec.Management.WebModeler.WebsocketsExternalURL = ""
				return claim
			}, fleetv1alpha1.ReasonMissingInput, "spec.management.webModeler.websocketsExternalUrl",
		),
		Entry(
			"no selection in the database section", func() *fleetv1alpha1.CamundaManagementClusterClaim {
				claim := newPlaneClaim()
				claim.Spec.Plugins = claim.Spec.Plugins[:1]
				return claim
			}, fleetv1alpha1.ReasonMissingInput, "managementDatabases",
		),
		Entry(
			"no platform config from the claim or its Environment",
			func() *fleetv1alpha1.CamundaManagementClusterClaim {
				environment := planeEnvironment()
				environment.Spec.PlatformConfigRef = ""
				return newClaim(newEnvironmentNamespace(environment), environment.Name)
			},
			fleetv1alpha1.ReasonMissingInput,
			"CamundaPlatformConfig",
		),
	)
})
