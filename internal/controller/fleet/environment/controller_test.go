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

package environment

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

const healthyMessage = "Every object that the spec names exists"

var _ = Describe("Environment Ready", func() {
	It("reports Healthy when every object that the spec names exists", func() {
		platform := create(newPlatformConfig(uniqueName("cpc")))
		clusterPreset := create(newClusterPreset(uniqueName("ccp")))
		esPreset := create(newElasticsearchPreset(uniqueName("escp")))
		dbPreset := create(newDatabaseServerPreset(uniqueName("dbsp")))
		release := create(newRelease(uniqueName("cr")))
		section := create(newSection(uniqueName("section")))
		registration := create(newRegistration(uniqueName("registration")))

		env := createEnvironment(fleetv1alpha1.EnvironmentSpec{
			PlatformConfigRef: platform.GetName(),
			Presets: fleetv1alpha1.EnvironmentPresets{
				CamundaCluster:       []string{clusterPreset.GetName()},
				ElasticsearchCluster: []string{esPreset.GetName()},
				DatabaseServer:       []string{dbPreset.GetName()},
			},
			Releases: &fleetv1alpha1.EnvironmentReleases{
				Allowed: []string{release.GetName()},
				Default: release.GetName(),
			},
			Sections: map[string]fleetv1alpha1.SectionPolicy{
				section.GetName(): {Default: registration.GetName(), Allowed: []string{registration.GetName()}},
			},
			Parameters: map[string]fleetv1alpha1.ParameterPolicies{
				registration.GetName(): {"retentionDays": {ClaimEditable: true}},
			},
		})

		expectReady(env, metav1.ConditionTrue, v1.ReasonHealthy, Equal(healthyMessage))
	})

	It("reports Healthy for an empty spec", func() {
		env := createEnvironment(fleetv1alpha1.EnvironmentSpec{})

		expectReady(env, metav1.ConditionTrue, v1.ReasonHealthy, Equal(healthyMessage))
	})

	DescribeTable(
		"reports InvalidReference for a missing object, and Healthy once it exists, without a change to the environment",
		func(
			kind string,
			build func(name string) client.Object,
			refer func(spec *fleetv1alpha1.EnvironmentSpec, name string),
		) {
			name := uniqueName("missing")
			var spec fleetv1alpha1.EnvironmentSpec
			refer(&spec, name)
			env := createEnvironment(spec)

			expectReady(
				env, metav1.ConditionFalse, v1.ReasonInvalidReference,
				Equal("The spec names objects that do not exist: "+kind+" "+name),
			)
			generation := generationOf(env)

			create(build(name))

			expectReady(env, metav1.ConditionTrue, v1.ReasonHealthy, Equal(healthyMessage))
			Expect(generationOf(env)).To(Equal(generation))
		},
		Entry(
			"a release", "CamundaRelease", newRelease,
			func(s *fleetv1alpha1.EnvironmentSpec, n string) {
				s.Releases = &fleetv1alpha1.EnvironmentReleases{Allowed: []string{n}}
			},
		),
		Entry(
			"a cluster preset", "CamundaClusterPreset", newClusterPreset,
			func(s *fleetv1alpha1.EnvironmentSpec, n string) { s.Presets.CamundaCluster = []string{n} },
		),
		Entry(
			"an Elasticsearch preset", "ElasticsearchClusterPreset", newElasticsearchPreset,
			func(s *fleetv1alpha1.EnvironmentSpec, n string) { s.Presets.ElasticsearchCluster = []string{n} },
		),
		Entry(
			"a database server preset", "DatabaseServerPreset", newDatabaseServerPreset,
			func(s *fleetv1alpha1.EnvironmentSpec, n string) { s.Presets.DatabaseServer = []string{n} },
		),
		Entry(
			"a platform config", "CamundaPlatformConfig", newPlatformConfig,
			func(s *fleetv1alpha1.EnvironmentSpec, n string) { s.PlatformConfigRef = n },
		),
		Entry(
			"a section", "PluginSection", newSection,
			func(s *fleetv1alpha1.EnvironmentSpec, n string) {
				s.Sections = map[string]fleetv1alpha1.SectionPolicy{n: {}}
			},
		),
		Entry(
			"a registration of a section policy", "PluginRegistration", newRegistration,
			func(s *fleetv1alpha1.EnvironmentSpec, n string) {
				section := create(newSection(uniqueName("section")))
				s.Sections = map[string]fleetv1alpha1.SectionPolicy{section.GetName(): {Allowed: []string{n}}}
			},
		),
		Entry(
			"a registration of a parameter policy", "PluginRegistration", newRegistration,
			func(s *fleetv1alpha1.EnvironmentSpec, n string) {
				s.Parameters = map[string]fleetv1alpha1.ParameterPolicies{n: {"retentionDays": {ClaimEditable: true}}}
			},
		),
	)

	It("names every missing object in one message", func() {
		release, section := uniqueName("cr"), uniqueName("section")
		env := createEnvironment(fleetv1alpha1.EnvironmentSpec{
			Releases: &fleetv1alpha1.EnvironmentReleases{Allowed: []string{release}},
			Sections: map[string]fleetv1alpha1.SectionPolicy{section: {}},
		})

		expectReady(
			env, metav1.ConditionFalse, v1.ReasonInvalidReference,
			Equal("The spec names objects that do not exist: CamundaRelease "+release+", PluginSection "+section),
		)
	})

	It("reports InvalidReference again when a named object is deleted", func() {
		release := create(newRelease(uniqueName("cr")))
		env := createEnvironment(fleetv1alpha1.EnvironmentSpec{
			Releases: &fleetv1alpha1.EnvironmentReleases{Allowed: []string{release.GetName()}},
		})
		expectReady(env, metav1.ConditionTrue, v1.ReasonHealthy, Equal(healthyMessage))

		Expect(k8sClient.Delete(ctx, release)).To(Succeed())

		expectReady(
			env, metav1.ConditionFalse, v1.ReasonInvalidReference,
			ContainSubstring("CamundaRelease "+release.GetName()),
		)
	})

	It("reports on the new generation after a change of the spec", func() {
		env := createEnvironment(fleetv1alpha1.EnvironmentSpec{})
		expectReady(env, metav1.ConditionTrue, v1.ReasonHealthy, Equal(healthyMessage))

		release := uniqueName("cr")
		Eventually(func() error {
			var latest fleetv1alpha1.Environment
			if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(env), &latest); err != nil {
				return err
			}
			latest.Spec.Releases = &fleetv1alpha1.EnvironmentReleases{Allowed: []string{release}}
			return k8sClient.Update(ctx, &latest)
		}, timeout, interval).Should(Succeed())

		// expectReady also asserts that the observed generation is the new one.
		expectReady(env, metav1.ConditionFalse, v1.ReasonInvalidReference, ContainSubstring(release))
	})
})
