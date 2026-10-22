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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

// planeSections are the sections of the requests of newClaim.
var planeSections = []string{pluginstartup.SectionAccess, pluginstartup.SectionManagementDatabase}

var _ = Describe("Deleting a plane claim", func() {
	It("copies the claim policy to both requests before they get a deletion timestamp", func() {
		claim := readyClaim(newPlaneClaim())

		update(claim, func(c *fleetv1alpha1.CamundaManagementClusterClaim) {
			c.Spec.DeletionPolicy = pluginsv1alpha1.DeletionPolicyOrphan
		})
		Expect(k8sClient.Delete(ctx, claim)).To(Succeed())

		expectPoliciesAtDeletion(claim, pluginsv1alpha1.DeletionPolicyOrphan)
		expectGone(claim)
	})

	It("uses the default of the snapshot after the Environment changed", func() {
		environment := planeEnvironment()
		environment.Spec.DeletionPolicy = pluginsv1alpha1.DeletionPolicyOrphan
		claim := readyClaim(newClaim(newEnvironmentNamespace(environment), environment.Name))

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(environment), environment)).To(Succeed())
		environment.Spec.DeletionPolicy = pluginsv1alpha1.DeletionPolicyDelete
		Expect(k8sClient.Update(ctx, environment)).To(Succeed())
		Expect(k8sClient.Delete(ctx, claim)).To(Succeed())

		expectPoliciesAtDeletion(claim, pluginsv1alpha1.DeletionPolicyOrphan)
		expectGone(claim)
	})

	It("deletes the plane first, waits for a held request, and names it in Ready", func() {
		claim := readyClaim(newPlaneClaim())
		held := requestKey(claim, pluginstartup.SectionAccess)
		plugin.holdCleanup(held)
		DeferCleanup(plugin.releaseCleanup, held)

		Expect(k8sClient.Delete(ctx, claim)).To(Succeed())

		expectGone(
			&v1.CamundaManagementCluster{ObjectMeta: metav1.ObjectMeta{Name: claim.Name, Namespace: claim.Namespace}},
		)
		expectFalse(claim, v1.ConditionReady, fleetv1alpha1.ReasonDeleting, And(
			ContainSubstring("PluginRequest "+held.Name),
			Not(ContainSubstring("CamundaManagementCluster")),
		))
		Consistently(func() error {
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &fleetv1alpha1.CamundaManagementClusterClaim{})
		}, consistently, interval).Should(Succeed())

		plugin.releaseCleanup(held)

		expectGone(claim)
		var requests pluginsv1alpha1.PluginRequestList
		Expect(k8sClient.List(ctx, &requests, client.InNamespace(claim.Namespace))).To(Succeed())
		Expect(requests.Items).To(BeEmpty())
	})
})

// readyClaim creates claim and waits until its plane exists and the fake
// plugin holds both of its requests.
func readyClaim(claim *fleetv1alpha1.CamundaManagementClusterClaim) *fleetv1alpha1.CamundaManagementClusterClaim {
	GinkgoHelper()
	create(claim)

	eventuallyPlane(claim, nil)
	Eventually(func(g Gomega) {
		for _, section := range planeSections {
			request, err := requestOf(claim, section)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(request.Finalizers).To(ContainElement(pluginsv1alpha1.FinalizerCleanup))
		}
	}, timeout, interval).Should(Succeed())

	return claim
}

// expectPoliciesAtDeletion polls until the fake plugin saw the deletion of
// both requests of claim with policy.
func expectPoliciesAtDeletion(
	claim *fleetv1alpha1.CamundaManagementClusterClaim,
	policy pluginsv1alpha1.DeletionPolicy,
) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		for _, section := range planeSections {
			got, seen := plugin.deletionPolicyOf(requestKey(claim, section))
			g.Expect(seen).To(BeTrue(), section)
			g.Expect(got).To(Equal(policy), section)
		}
	}, timeout, interval).Should(Succeed())
}
