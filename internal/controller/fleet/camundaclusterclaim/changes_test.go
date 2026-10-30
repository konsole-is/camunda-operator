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
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	clustercomponents "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
)

var _ = Describe("Changing a cluster claim", func() {
	It("refuses a release that the Environment does not allow, and applies one that it allows", func() {
		environment := allowingEnvironment(map[string][]string{
			pluginstartup.SectionSecondaryStorage: {pluginstartup.RegistrationExistingSecondaryStorage},
		})
		createEnvironment(environment)
		claim := newClaim(newNamespace(map[string]string{fleetv1alpha1.LabelEnvironment: environment.Name}))
		create(claim)
		eventuallyCluster(claim)

		updateClaim(claim, func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.ReleaseRef = nextReleaseName })

		expectFalse(
			claim,
			v1.ConditionReady,
			fleetv1alpha1.ReasonNotAllowed,
			ContainSubstring(`CamundaRelease "suite-release-next" is not in spec.releases.allowed`),
		)
		Consistently(func(g Gomega) {
			cluster, err := clusterOf(claim)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cluster.Spec.ReleaseRef).To(Equal(releaseName))
		}, consistently, interval).Should(Succeed())

		By("allowing the release")
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(environment), environment)).To(Succeed())
		environment.Spec.Releases.Allowed = []string{releaseName, nextReleaseName}
		Expect(k8sClient.Update(ctx, environment)).To(Succeed())

		Eventually(func(g Gomega) {
			cluster, err := clusterOf(claim)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cluster.Spec.ReleaseRef).To(Equal(nextReleaseName))
		}, timeout, interval).Should(Succeed())
	})

	It("keeps applying a claim whose release the Environment no longer allows", func() {
		environment := allowingEnvironment(map[string][]string{
			pluginstartup.SectionSecondaryStorage: {pluginstartup.RegistrationExistingSecondaryStorage},
		})
		createEnvironment(environment)
		claim := newClaim(newNamespace(map[string]string{fleetv1alpha1.LabelEnvironment: environment.Name}))
		create(claim)
		eventuallyCluster(claim)

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(environment), environment)).To(Succeed())
		environment.Spec.Releases = &fleetv1alpha1.EnvironmentReleases{
			Allowed: []string{nextReleaseName},
			Default: nextReleaseName,
		}
		Expect(k8sClient.Update(ctx, environment)).To(Succeed())
		updateClaim(claim, func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.Suspend = true })

		Eventually(func(g Gomega) {
			cluster, err := clusterOf(claim)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cluster.Spec.Suspend).To(BeTrue())
		}, timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			ready := meta.FindStatusCondition(latest(g, claim).Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Reason).NotTo(Equal(fleetv1alpha1.ReasonNotAllowed))
		}, consistently, interval).Should(Succeed())
	})

	It("passes a claim-editable parameter to the request and refuses another key", func() {
		environment := allowingEnvironment(map[string][]string{
			pluginstartup.SectionSecondaryStorage: {pluginstartup.RegistrationExistingSecondaryStorage},
			pluginstartup.SectionEncryptionKey:    {registrationKey},
		})
		environment.Spec.Parameters = map[string]fleetv1alpha1.ParameterPolicies{
			registrationKey: {
				"region": {ClaimEditable: true},
				"tier":   {Value: &apiextensionsv1.JSON{Raw: []byte(`"gold"`)}},
			},
		}
		createEnvironment(environment)
		key := selection(pluginstartup.SectionEncryptionKey, registrationKey)
		key.Parameters = &apiextensionsv1.JSON{Raw: []byte(`{"region":"eu"}`)}
		claim := newClaim(newNamespace(map[string]string{fleetv1alpha1.LabelEnvironment: environment.Name}), key)
		create(claim)
		expectParameters(claim, `{"region":"eu","tier":"gold"}`)

		setKeyParameters(claim, `{"region":"us"}`)
		expectParameters(claim, `{"region":"us","tier":"gold"}`)

		By("changing a key that the claim cannot edit")
		setKeyParameters(claim, `{"region":"us","tier":"silver"}`)
		expectFalse(
			claim,
			v1.ConditionReady,
			fleetv1alpha1.ReasonNotAllowed,
			ContainSubstring(`parameter "tier" of registration "test-key" is not claim-editable`),
		)
	})

	It("copies suspend to the cluster and leaves the requests as they are", func() {
		claim := readyClaim(newNamespace(nil))
		request, err := requestOf(claim, pluginstartup.SectionEncryptionKey)
		Expect(err).NotTo(HaveOccurred())

		updateClaim(claim, func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.Suspend = true })
		Eventually(func(g Gomega) {
			cluster, err := clusterOf(claim)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cluster.Spec.Suspend).To(BeTrue())
		}, timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			current, err := requestOf(claim, pluginstartup.SectionEncryptionKey)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(current.Generation).To(Equal(request.Generation))
		}, consistently, interval).Should(Succeed())

		By("resuming")
		updateClaim(claim, func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.Suspend = false })
		Eventually(func(g Gomega) {
			cluster, err := clusterOf(claim)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cluster.Spec.Suspend).To(BeFalse())
		}, timeout, interval).Should(Succeed())
	})

	Context("with a downgrade", func() {
		var claim *fleetv1alpha1.CamundaClusterClaim

		BeforeEach(func() {
			claim = readyClaim(newNamespace(nil))
			setManagement(claim, "8.9.9")
		})

		// downgrade moves the claim to the older release and sanctions its
		// version in the same edit.
		downgrade := func() {
			GinkgoHelper()
			updateClaim(claim, func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.ReleaseRef = olderReleaseName
				c.Spec.AllowVersionDowngrade = olderVersion
			})
			Eventually(func(g Gomega) {
				g.Expect(downgradeAnnotation(g, claim)).To(Equal(olderVersion))
			}, timeout, interval).Should(Succeed())
		}

		expectNoAnnotation := func() {
			GinkgoHelper()
			Consistently(func(g Gomega) {
				g.Expect(downgradeAnnotation(g, claim)).To(BeEmpty())
			}, consistently, interval).Should(Succeed())
		}

		It("sets the downgrade annotation while the cluster does not run the target", func() {
			downgrade()
		})

		It("keeps the annotation until the operator removes it", func() {
			downgrade()
			setManagement(claim, olderVersion)

			expectNewPass(claim)
			Consistently(func(g Gomega) {
				g.Expect(downgradeAnnotation(g, claim)).To(Equal(olderVersion))
			}, consistently, interval).Should(Succeed())
		})

		It("does not add the annotation again once the cluster runs the target", func() {
			downgrade()
			setManagement(claim, olderVersion)
			consumeSanction(claim)

			expectNewPass(claim)
			expectNewPass(claim)
			expectNoAnnotation()
		})

		It("keeps the annotation off when the cluster is suspended after the sanction was spent", func() {
			downgrade()
			setManagement(claim, olderVersion)
			consumeSanction(claim)

			updateClaim(claim, func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.Suspend = true })
			setManagement(claim, "")

			expectNewPass(claim)
			expectNewPass(claim)
			expectNoAnnotation()
		})

		It("adds the annotation when the resumed cluster refuses a downgrade set during the suspension", func() {
			updateClaim(claim, func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.Suspend = true })
			setManagement(claim, "")
			updateClaim(claim, func(c *fleetv1alpha1.CamundaClusterClaim) {
				c.Spec.ReleaseRef = olderReleaseName
				c.Spec.AllowVersionDowngrade = olderVersion
			})
			expectNewPass(claim)
			expectNoAnnotation()

			updateClaim(claim, func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.Suspend = false })
			Eventually(func(g Gomega) {
				cluster, err := clusterOf(claim)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(cluster.Spec.Suspend).To(BeFalse())
			}, timeout, interval).Should(Succeed())
			setClusterReady(claim, metav1.ConditionFalse, v1.ReasonVersionDowngradeRefused, "")

			Eventually(func(g Gomega) {
				g.Expect(downgradeAnnotation(g, claim)).To(Equal(olderVersion))
			}, timeout, interval).Should(Succeed())
		})

		It("removes the annotation when the field is cleared", func() {
			downgrade()
			updateClaim(claim, func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.AllowVersionDowngrade = "" })

			Eventually(func(g Gomega) {
				g.Expect(downgradeAnnotation(g, claim)).To(BeEmpty())
			}, timeout, interval).Should(Succeed())
		})

		It("waits for the release when the field is set first", func() {
			updateClaim(
				claim,
				func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.AllowVersionDowngrade = olderVersion },
			)

			expectNewPass(claim)
			expectNewPass(claim)
			expectNoAnnotation()

			By("moving to the release of the target")
			updateClaim(claim, func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.ReleaseRef = olderReleaseName })
			Eventually(func(g Gomega) {
				g.Expect(downgradeAnnotation(g, claim)).To(Equal(olderVersion))
			}, timeout, interval).Should(Succeed())
		})
	})
})

// downgradeAnnotation reads the downgrade annotation of the CamundaCluster
// of claim. It is empty when the cluster does not carry it.
func downgradeAnnotation(g Gomega, claim *fleetv1alpha1.CamundaClusterClaim) string {
	cluster, err := clusterOf(claim)
	g.Expect(err).NotTo(HaveOccurred())
	return cluster.Annotations[clustercomponents.AllowVersionDowngradeAnnotation]
}

// setKeyParameters sets the parameters of the encryption-key selection of
// claim to raw.
func setKeyParameters(claim *fleetv1alpha1.CamundaClusterClaim, raw string) {
	GinkgoHelper()
	updateClaim(claim, func(c *fleetv1alpha1.CamundaClusterClaim) {
		for i := range c.Spec.Plugins {
			if c.Spec.Plugins[i].Section == pluginstartup.SectionEncryptionKey {
				c.Spec.Plugins[i].Parameters = &apiextensionsv1.JSON{Raw: []byte(raw)}
			}
		}
	})
}

// expectParameters polls until the encryption-key request of claim has the
// parameters want.
func expectParameters(claim *fleetv1alpha1.CamundaClusterClaim, want string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		request, err := requestOf(claim, pluginstartup.SectionEncryptionKey)
		g.Expect(err).NotTo(HaveOccurred())
		g.Expect(request.Spec.Parameters).NotTo(BeNil())
		g.Expect(string(request.Spec.Parameters.Raw)).To(MatchJSON(want))
	}, timeout, interval).Should(Succeed())
}
