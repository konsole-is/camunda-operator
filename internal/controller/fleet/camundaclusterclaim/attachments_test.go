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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaclusterclaim"
)

var _ = Describe("Cluster claim attachments", func() {
	It("creates the Optimize and the backup schedule after the cluster is ready", func() {
		claim := withAttachments(newClaim(newNamespace(nil)))
		create(claim)
		eventuallyCluster(claim)

		Consistently(func(g Gomega) {
			_, err := optimizeOf(claim)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "Optimize")
			_, err = scheduleOf(claim)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "schedule")
		}, consistently, interval).Should(Succeed())

		setClusterReady(claim, metav1.ConditionTrue, v1.ReasonHealthy, "")

		current := latest(Default, claim)
		Eventually(func(g Gomega) {
			optimize, err := optimizeOf(claim)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(optimize.Spec.ClusterRef).To(Equal(v1.ClusterRef{Name: claim.Name}))
			g.Expect(optimize.Spec.ManagementAuthRef).To(Equal("management"))
			g.Expect(optimize.Spec.Version).To(Equal("8.9.9"))
			g.Expect(optimize.Labels).To(HaveKeyWithValue(fleetv1alpha1.LabelClaim, claim.Name))
			g.Expect(metav1.IsControlledBy(optimize, current)).To(BeTrue())

			schedule, err := scheduleOf(claim)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(schedule.Spec.ClusterRef).To(Equal(v1.ClusterRef{Name: claim.Name}))
			g.Expect(schedule.Spec.Schedule).To(Equal("0 2 * * *"))
			g.Expect(metav1.IsControlledBy(schedule, current)).To(BeTrue())
		}, timeout, interval).Should(Succeed())
	})

	It("takes the Optimize URL from the access output", func() {
		claim := attachedClaim(selection(pluginstartup.SectionAccess, registrationAccess))

		Eventually(func(g Gomega) {
			optimize, err := optimizeOf(claim)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(optimize.Spec.ExternalURL).To(Equal(optimizeURL))
		}, timeout, interval).Should(Succeed())
	})

	It("moves the Optimize version with the release", func() {
		claim := attachedClaim()

		updateClaim(claim, func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.ReleaseRef = nextReleaseName })

		Eventually(func(g Gomega) {
			optimize, err := optimizeOf(claim)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(optimize.Spec.Version).To(Equal("8.9.10"))
			cluster, err := clusterOf(claim)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cluster.Spec.ReleaseRef).To(Equal(nextReleaseName))
		}, timeout, interval).Should(Succeed())
	})

	It("deletes an attachment that the claim removes", func() {
		claim := attachedClaim()

		updateClaim(claim, func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.Attachments.BackupSchedule = nil })

		Eventually(func() bool {
			_, err := scheduleOf(claim)
			return apierrors.IsNotFound(err)
		}, timeout, interval).Should(BeTrue())
		Consistently(func() error {
			_, err := optimizeOf(claim)
			return err
		}, consistently, interval).Should(Succeed())
	})

	It("counts a suspended Optimize as ready while the claim is suspended", func() {
		claim := attachedClaim()

		updateClaim(claim, func(c *fleetv1alpha1.CamundaClusterClaim) { c.Spec.Suspend = true })
		Eventually(func(g Gomega) {
			cluster, err := clusterOf(claim)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cluster.Spec.Suspend).To(BeTrue())
		}, timeout, interval).Should(Succeed())
		setClusterReady(claim, metav1.ConditionTrue, "Suspended", "")
		setOptimizeReady(claim, metav1.ConditionTrue, "Suspended")
		setScheduleReady(claim)

		Eventually(func(g Gomega) {
			conds := latest(g, claim).Status.Conditions
			g.Expect(meta.IsStatusConditionTrue(conds, fleetv1alpha1.ConditionAttachmentsReady)).To(BeTrue())
			g.Expect(meta.IsStatusConditionTrue(conds, v1.ConditionReady)).To(BeTrue())
		}, timeout, interval).Should(Succeed())
	})

	It("keeps a field that another writer owns", func() {
		claim := attachedClaim()
		Eventually(func() error {
			_, err := optimizeOf(claim)
			return err
		}, timeout, interval).Should(Succeed())

		other := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": v1.GroupVersion.String(),
			"kind":       "CamundaOptimize",
			"metadata": map[string]any{
				"name":      components.OptimizeName(claim.Name),
				"namespace": claim.Namespace,
			},
			"spec": map[string]any{"indexReplicas": int64(2)},
		}}
		//nolint:staticcheck // the repository applies through the deprecated client.Apply patch
		Expect(k8sClient.Patch(ctx, other, client.Apply, client.FieldOwner("other-writer"))).To(Succeed())
		expectNewPass(claim)
		expectNewPass(claim)

		Consistently(func(g Gomega) {
			optimize, err := optimizeOf(claim)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(optimize.Spec.IndexReplicas).NotTo(BeNil())
			g.Expect(*optimize.Spec.IndexReplicas).To(Equal(int32(2)))
		}, consistently, interval).Should(Succeed())
	})

	It("creates the Optimize once an Optimize of another owner is gone", func() {
		claim := withAttachments(newClaim(newNamespace(nil)))
		owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "other-owner", Namespace: claim.Namespace}}
		create(owner)
		foreign := &v1.CamundaOptimize{
			ObjectMeta: metav1.ObjectMeta{Name: components.OptimizeName(claim.Name), Namespace: claim.Namespace},
			Spec: v1.CamundaOptimizeSpec{
				Version:           "8.9.9",
				ManagementAuthRef: "management",
				ClusterRef:        v1.ClusterRef{Name: claim.Name},
			},
		}
		Expect(controllerutil.SetControllerReference(owner, foreign, k8sClient.Scheme())).To(Succeed())
		create(foreign)
		create(claim)
		eventuallyCluster(claim)
		setClusterReady(claim, metav1.ConditionTrue, v1.ReasonHealthy, "")
		expectFalse(claim, fleetv1alpha1.ConditionAttachmentsReady, "Blocked", Not(BeEmpty()))

		Expect(k8sClient.Delete(ctx, foreign)).To(Succeed())

		Eventually(func(g Gomega) {
			optimize, err := optimizeOf(claim)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(metav1.IsControlledBy(optimize, latest(g, claim))).To(BeTrue())
		}, timeout, interval).Should(Succeed())
	})

	It("reports AttachmentsReady in Ready", func() {
		claim := attachedClaim()

		setOptimizeReady(claim, metav1.ConditionFalse, "Creating")

		Eventually(func(g Gomega) {
			ready := meta.FindStatusCondition(latest(g, claim).Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(strings.HasPrefix(ready.Message, "attachments:")).To(BeTrue(), ready.Message)
		}, timeout, interval).Should(Succeed())
	})
})

// attachedClaim creates a claim with both attachments and selections, marks
// its cluster ready, and waits until both attachments exist.
func attachedClaim(selections ...fleetv1alpha1.PluginSelection) *fleetv1alpha1.CamundaClusterClaim {
	GinkgoHelper()
	claim := withAttachments(newClaim(newNamespace(nil), selections...))
	create(claim)
	eventuallyCluster(claim)
	setClusterReady(claim, metav1.ConditionTrue, v1.ReasonHealthy, "")

	Eventually(func(g Gomega) {
		_, err := optimizeOf(claim)
		g.Expect(err).NotTo(HaveOccurred())
		_, err = scheduleOf(claim)
		g.Expect(err).NotTo(HaveOccurred())
	}, timeout, interval).Should(Succeed())

	return claim
}
