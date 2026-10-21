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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

var _ = Describe("CamundaClusterClaim precheck", func() {
	DescribeTable(
		"refuses the claim and writes nothing",
		func(claimOf func() *fleetv1alpha1.CamundaClusterClaim, reason, message string) {
			claim := claimOf()
			create(claim)

			expectCondition(claim, v1.ConditionReady, metav1.ConditionFalse, reason, ContainSubstring(message))
			Consistently(func(g Gomega) {
				g.Expect(latest(g, claim).Status.ResolvedPolicy).To(BeNil())

				var requests pluginsv1alpha1.PluginRequestList
				g.Expect(k8sClient.List(
					ctx,
					&requests,
					client.InNamespace(claim.Namespace),
					client.MatchingLabels{fleetv1alpha1.LabelClaim: claim.Name},
				)).To(Succeed())
				g.Expect(requests.Items).To(BeEmpty())

				var clusters v1.CamundaClusterList
				g.Expect(k8sClient.List(ctx, &clusters, client.InNamespace(claim.Namespace))).To(Succeed())
				g.Expect(clusters.Items).To(BeEmpty())
			}, consistently, interval).Should(Succeed())
		},
		Entry(
			"a namespace without the label while an Environment exists", func() *fleetv1alpha1.CamundaClusterClaim {
				createEnvironment(allowingEnvironment(nil))
				return newClaim(newNamespace(nil))
			}, fleetv1alpha1.ReasonNoEnvironment, fleetv1alpha1.LabelEnvironment,
		),
		Entry(
			"a label that names no Environment", func() *fleetv1alpha1.CamundaClusterClaim {
				return newClaim(newNamespace(map[string]string{fleetv1alpha1.LabelEnvironment: "missing"}))
			}, fleetv1alpha1.ReasonUnknownEnvironment, `"missing"`,
		),
		Entry(
			"a preset that does not exist", func() *fleetv1alpha1.CamundaClusterClaim {
				claim := newClaim(newNamespace(nil))
				claim.Spec.PresetRef = "missing-preset"
				return claim
			}, v1.ReasonInvalidReference, `CamundaClusterPreset "missing-preset"`,
		),
		Entry(
			"a preset that the Environment does not allow", func() *fleetv1alpha1.CamundaClusterClaim {
				environment := allowingEnvironment(map[string][]string{
					pluginstartup.SectionSecondaryStorage: {pluginstartup.RegistrationExistingSecondaryStorage},
				})
				environment.Spec.Presets.CamundaCluster = nil
				createEnvironment(environment)
				return newClaim(newNamespace(map[string]string{fleetv1alpha1.LabelEnvironment: environment.Name}))
			}, fleetv1alpha1.ReasonNotAllowed, presetName,
		),
		Entry(
			"an override of a field that a selected slot fills", func() *fleetv1alpha1.CamundaClusterClaim {
				claim := newClaim(newNamespace(nil), selection(pluginstartup.SectionAccess, registrationAccess))
				claim.Spec.Cluster = &v1.CamundaClusterSpec{ExternalURL: "https://own.example.com"}
				return claim
			}, fleetv1alpha1.ReasonSlotConflict, "externalUrl",
		),
		Entry(
			"a required input without a producer", func() *fleetv1alpha1.CamundaClusterClaim {
				return newClaim(
					newNamespace(nil),
					selection(pluginstartup.SectionVolumeEncryption, registrationVolumes),
				)
			}, fleetv1alpha1.ReasonMissingInput, pluginstartup.SectionEncryptionKey,
		),
		Entry(
			"inputs that form a cycle", func() *fleetv1alpha1.CamundaClusterClaim {
				first := testRegistration(uniqueName("cycle-key"), pluginstartup.SectionEncryptionKey)
				first.Spec.Inputs = []pluginsv1alpha1.InputDecl{{Name: "url", From: "access/externalUrl"}}
				second := testRegistration(uniqueName("cycle-access"), pluginstartup.SectionAccess)
				second.Spec.Inputs = []pluginsv1alpha1.InputDecl{{Name: "key", From: "encryption-key/keyRef"}}
				create(first)
				create(second)
				return newClaim(
					newNamespace(nil),
					selection(pluginstartup.SectionEncryptionKey, first.Name),
					selection(pluginstartup.SectionAccess, second.Name),
				)
			}, fleetv1alpha1.ReasonWiringCycle, pluginstartup.SectionAccess,
		),
	)
})
