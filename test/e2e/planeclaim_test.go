//go:build e2e
// +build e2e

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

package e2e

import (
	"os"
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	dscomponents "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/planedatabases"
	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	// planeClaimNamespace holds the claim, the Keycloak of
	// testdata/keycloak.yaml, and everything that the claim creates.
	planeClaimNamespace = "plane-claim-e2e"
	planeClaimName      = "plane"
	// The cluster-scoped objects of the flow. The Environment gives the
	// platform config and the database server preset, so the claim selects
	// the option and sets no parameter.
	planeClaimEnvironment = "plane-claim-e2e"
	planeClaimPlatform    = "plane-claim-e2e"
	planeClaimDBPreset    = "plane-claim-e2e"

	planeClaimResource = "camundamanagementclusterclaims.fleet.camunda.io"

	// planeClaimReadyTimeout covers the PostgreSQL server, whose databases
	// the plane waits for, and then the start of the plane.
	planeClaimReadyTimeout = dsReadyTimeout + mcReadyTimeout
)

var _ = Describe("CamundaManagementClusterClaim", Ordered, Label(utils.LabelPlaneClaim), func() {
	BeforeAll(func() {
		By("creating the namespace of the claim in the Environment")
		Expect(apply(&corev1.Namespace{
			TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Namespace"},
			ObjectMeta: metav1.ObjectMeta{
				Name:   planeClaimNamespace,
				Labels: map[string]string{fleetv1alpha1.LabelEnvironment: planeClaimEnvironment},
			},
		})).To(Succeed())

		By("deploying Keycloak with the e2e realm")
		_, err := utils.Kubectl("apply", "-n", planeClaimNamespace, "-f", "test/e2e/testdata/keycloak.yaml")
		Expect(err).NotTo(HaveOccurred())
		_, err = utils.Kubectl(
			"rollout", "status", "deployment/keycloak", "-n", planeClaimNamespace, "--timeout", "6m",
		)
		Expect(err).NotTo(HaveOccurred())

		By("creating the client secret, the platform config, and the database server preset")
		Expect(apply(oidcManagementClientSecret())).To(Succeed())
		Expect(apply(oidcManagementPlatformConfig(planeClaimPlatform, planeClaimNamespace))).To(Succeed())
		Expect(apply(&v1.DatabaseServerPreset{
			TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "DatabaseServerPreset"},
			ObjectMeta: metav1.ObjectMeta{Name: planeClaimDBPreset},
			Spec: v1.DatabaseServerPresetSpec{
				Server: v1.DatabaseServerSpec{
					Instances:   new(int32(1)),
					Resources:   requests("250m", "512Mi"),
					StorageSize: new(resource.MustParse("2Gi")),
				},
			},
		})).To(Succeed())

		By("creating the Environment and waiting until it finds every object it names")
		Expect(apply(planeClaimEnvironmentObject())).To(Succeed())
		Eventually(func(g Gomega) {
			expectReady(g, environmentResource, planeClaimEnvironment, "", v1.ReasonHealthy)
		}, 2*time.Minute, 5*time.Second).Should(Succeed())
	})

	AfterAll(func() {
		By("removing the claim, the cluster-scoped objects, and the namespace")
		_, _ = utils.Kubectl(
			"delete", planeClaimResource, planeClaimName, "-n", planeClaimNamespace,
			"--ignore-not-found", "--wait=false",
		)
		_, _ = utils.Kubectl("delete", environmentResource, planeClaimEnvironment, "--ignore-not-found")
		_, _ = utils.Kubectl("delete", ccPlatformResource, planeClaimPlatform, "--ignore-not-found")
		_, _ = utils.Kubectl(
			"delete", "databaseserverpresets.core.camunda.io", planeClaimDBPreset, "--ignore-not-found",
		)
		_, _ = utils.Kubectl(
			"delete", "secret", mcOIDCClientSecretName, "-n", namespace, "--ignore-not-found",
		)
		_, _ = utils.Kubectl("delete", "ns", planeClaimNamespace, "--ignore-not-found", "--wait=false")
	})

	AfterEach(func() {
		dumpDiagnostics(planeClaimNamespace)
	})

	It("runs a management plane on the built-in database server with no access from outside", func() {
		By("creating the claim")
		Expect(apply(planeClaimObject())).To(Succeed())

		By("waiting for the database server, the plane, and the claim to report Healthy")
		server := planeClaimName + pluginstartup.BackingSuffixManagementDatabaseServer
		Eventually(func(g Gomega) {
			expectReady(g, dsResource, server, planeClaimNamespace, v1.ReasonHealthy)
			expectReady(g, mcResource, planeClaimName, planeClaimNamespace, v1.ReasonHealthy)
			expectReady(g, planeClaimResource, planeClaimName, planeClaimNamespace, v1.ReasonHealthy)
		}, planeClaimReadyTimeout, 10*time.Second).Should(Succeed())

		By("running the database server on CloudNativePG")
		var current v1.DatabaseServer
		Expect(utils.Get(dsResource, server, planeClaimNamespace, &current)).To(Succeed())
		exists, err := utils.Exists("clusters.postgresql.cnpg.io", dscomponents.ClusterName(&current), planeClaimNamespace)
		Expect(err).NotTo(HaveOccurred())
		Expect(exists).To(BeTrue(), "no CloudNativePG cluster runs the database server")

		By("selecting no access option and giving Identity the database of the request")
		var database string
		Eventually(func(g Gomega) {
			requests := claimRequests(g, planeClaimResource, planeClaimName, planeClaimNamespace)
			g.Expect(requests).To(HaveLen(1), "the claim must select only the database section")
			database = requestOutput(
				g,
				requests[pluginstartup.SectionManagementDatabase],
				planeClaimNamespace,
				planedatabases.Of(planedatabases.Identity).Output,
			)
			g.Expect(database).NotTo(BeEmpty())
		}, time.Minute, 5*time.Second).Should(Succeed())

		var plane v1.CamundaManagementCluster
		Expect(utils.Get(mcResource, planeClaimName, planeClaimNamespace, &plane)).To(Succeed())
		Expect(plane.Labels).To(HaveKeyWithValue(fleetv1alpha1.LabelClaim, planeClaimName))
		Expect(plane.Spec.PlatformConfigRef).To(Equal(planeClaimPlatform))
		Expect(plane.Spec.Identity.DatabaseConfigRef).To(Equal(database))
		Expect(plane.Spec.IdentityProvider.OIDC).NotTo(BeNil())
	})
})

func planeClaimEnvironmentObject() *fleetv1alpha1.Environment {
	return &fleetv1alpha1.Environment{
		TypeMeta:   metav1.TypeMeta{APIVersion: fleetv1alpha1.GroupVersion.String(), Kind: "Environment"},
		ObjectMeta: metav1.ObjectMeta{Name: planeClaimEnvironment},
		Spec: fleetv1alpha1.EnvironmentSpec{
			PlatformConfigRef: planeClaimPlatform,
			Presets:           fleetv1alpha1.EnvironmentPresets{DatabaseServer: []string{planeClaimDBPreset}},
			Sections: map[string]fleetv1alpha1.SectionPolicy{
				pluginstartup.SectionManagementDatabase: {
					Required: true,
					Default:  pluginstartup.RegistrationManagementDatabaseServer,
					Allowed:  []string{pluginstartup.RegistrationManagementDatabaseServer},
				},
			},
			// No version: the plane runs the default major of the
			// registration.
			Parameters: map[string]fleetv1alpha1.ParameterPolicies{
				pluginstartup.RegistrationManagementDatabaseServer: {
					"preset": {Value: &apiextensionsv1.JSON{Raw: []byte(strconv.Quote(planeClaimDBPreset))}},
				},
			},
		},
	}
}

func planeClaimObject() *fleetv1alpha1.CamundaManagementClusterClaim {
	// The claim gives the plane its own name.
	plane := &v1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{Name: planeClaimName, Namespace: planeClaimNamespace},
	}

	return &fleetv1alpha1.CamundaManagementClusterClaim{
		TypeMeta: metav1.TypeMeta{
			APIVersion: fleetv1alpha1.GroupVersion.String(),
			Kind:       "CamundaManagementClusterClaim",
		},
		ObjectMeta: metav1.ObjectMeta{Name: planeClaimName, Namespace: planeClaimNamespace},
		Spec: fleetv1alpha1.CamundaManagementClusterClaimSpec{
			Environments: []string{planeClaimEnvironment},
			Management: fleetv1alpha1.ManagementOverrides{
				IdentityProvider: fleetv1alpha1.ManagementIdentityProvider{OIDC: &v1.ManagementOIDCSpec{}},
				// Without an access option, the claim sets the URL itself.
				Identity: fleetv1alpha1.IdentityOverrides{
					ExternalURL: components.IdentityServiceURL(plane),
					Admin: v1.IdentityAdminSpec{
						ClaimName:  mcOIDCAdminClaimName,
						ClaimValue: mcOIDCAdminClaimValue,
					},
					WorkloadSpec: v1.WorkloadSpec{Resources: capped("150m", "512Mi", "1280Mi")},
				},
			},
			Versions: fleetv1alpha1.ManagementVersions{Identity: os.Getenv(envIdentityVersion)},
			Plugins: []fleetv1alpha1.PluginSelection{{
				Section:      pluginstartup.SectionManagementDatabase,
				Registration: pluginstartup.RegistrationManagementDatabaseServer,
			}},
		},
	}
}
