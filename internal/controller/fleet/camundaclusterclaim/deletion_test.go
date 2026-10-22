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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/pkg/components/claimrequests"
)

// testHold is a finalizer that a spec puts on an object to keep it after
// its deletion. No controller of the suite removes it.
const testHold = "test.camunda.io/hold"

var _ = Describe("Deleting a cluster claim", func() {
	It("copies the claim policy to every request before it gets a deletion timestamp", func() {
		claim := readyClaim(newNamespace(nil))

		setOrphanPolicy(claim)
		Expect(k8sClient.Delete(ctx, claim)).To(Succeed())

		expectPoliciesAtDeletion(claim, pluginsv1alpha1.DeletionPolicyOrphan)
	})

	It("uses the default of the snapshot after the Environment changed", func() {
		environment := allowingEnvironment(map[string][]string{
			pluginstartup.SectionSecondaryStorage: {pluginstartup.RegistrationExistingSecondaryStorage},
			pluginstartup.SectionEncryptionKey:    {registrationKey},
		})
		environment.Spec.DeletionPolicy = pluginsv1alpha1.DeletionPolicyOrphan
		createEnvironment(environment)
		claim := readyClaim(newNamespace(map[string]string{fleetv1alpha1.LabelEnvironment: environment.Name}))
		Expect(latest(Default, claim).Status.ResolvedPolicy.DeletionPolicy).
			To(Equal(pluginsv1alpha1.DeletionPolicyOrphan))

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(environment), environment)).To(Succeed())
		environment.Spec.DeletionPolicy = pluginsv1alpha1.DeletionPolicyDelete
		Expect(k8sClient.Update(ctx, environment)).To(Succeed())
		Expect(k8sClient.Delete(ctx, claim)).To(Succeed())

		expectPoliciesAtDeletion(claim, pluginsv1alpha1.DeletionPolicyOrphan)
		expectGone(claim)
		expectGone(&v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: claim.Name, Namespace: claim.Namespace}})
	})

	It("waits for a held request, names it in Ready, and is gone once it is released", func() {
		claim := readyClaim(newNamespace(nil))
		held := requestKey(claim, pluginstartup.SectionEncryptionKey)
		plugin.holdCleanup(held)
		DeferCleanup(plugin.releaseCleanup, held)

		Expect(k8sClient.Delete(ctx, claim)).To(Succeed())

		expectFalse(claim, v1.ConditionReady, fleetv1alpha1.ReasonDeleting, And(
			ContainSubstring("PluginRequest "+held.Name),
			Not(ContainSubstring(requestKey(claim, pluginstartup.SectionSecondaryStorage).Name)),
		))
		Consistently(func() error {
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &fleetv1alpha1.CamundaClusterClaim{})
		}, consistently, interval).Should(Succeed())

		plugin.releaseCleanup(held)

		expectGone(claim)
		expectGone(&v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: claim.Name, Namespace: claim.Namespace}})
		var requests pluginsv1alpha1.PluginRequestList
		Expect(k8sClient.List(
			ctx,
			&requests,
			client.InNamespace(claim.Namespace),
			client.MatchingLabels{fleetv1alpha1.LabelClaim: claim.Name},
		)).To(Succeed())
		Expect(requests.Items).To(BeEmpty())
	})

	It("waits for a held request that lost its claim label", func() {
		claim := readyClaim(newNamespace(nil))
		held := requestKey(claim, pluginstartup.SectionEncryptionKey)
		plugin.holdCleanup(held)
		DeferCleanup(plugin.releaseCleanup, held)
		Expect(k8sClient.Delete(ctx, claim)).To(Succeed())
		expectFalse(
			claim,
			v1.ConditionReady,
			fleetv1alpha1.ReasonDeleting,
			ContainSubstring("PluginRequest "+held.Name),
		)

		Eventually(func() error {
			request := &pluginsv1alpha1.PluginRequest{}
			if err := k8sClient.Get(ctx, held, request); err != nil {
				return err
			}
			delete(request.Labels, fleetv1alpha1.LabelClaim)
			return k8sClient.Update(ctx, request)
		}, timeout, interval).Should(Succeed())

		Consistently(func() error {
			return k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &fleetv1alpha1.CamundaClusterClaim{})
		}, consistently, interval).Should(Succeed())
		plugin.releaseCleanup(held)
		expectGone(claim)
	})

	It("refuses the policy write on a request that replaced the one that it read", func() {
		claim := readyClaim(newNamespace(nil))
		request, err := requestOf(claim, pluginstartup.SectionEncryptionKey)
		Expect(err).NotTo(HaveOccurred())
		request.UID = "replaced"

		apply := claimrequests.DeletionPolicyApply(request, pluginsv1alpha1.DeletionPolicyOrphan)
		//nolint:staticcheck // the repository applies through the deprecated client.Apply patch
		err = k8sClient.Patch(ctx, apply, client.Apply, client.FieldOwner("test"), client.ForceOwnership)

		Expect(err).To(HaveOccurred())
		current, err := requestOf(claim, pluginstartup.SectionEncryptionKey)
		Expect(err).NotTo(HaveOccurred())
		Expect(current.Spec.DeletionPolicy).To(Equal(pluginsv1alpha1.DeletionPolicyDelete))
	})

	It("deletes the attachments, then the cluster, then the requests", func() {
		claim := readyClaim(newNamespace(nil))
		optimize := ownedOptimize(claim)
		schedule := &v1.BackupSchedule{
			ObjectMeta: metav1.ObjectMeta{Name: claim.Name + "-backups", Namespace: claim.Namespace},
			Spec:       v1.BackupScheduleSpec{ClusterRef: v1.ClusterRef{Name: claim.Name}, Schedule: "0 2 * * *"},
		}
		Expect(controllerutil.SetControllerReference(claim, schedule, k8sClient.Scheme())).To(Succeed())
		schedule.Finalizers = []string{testHold}
		create(schedule)
		cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: claim.Name, Namespace: claim.Namespace}}
		setTestHold(cluster, true)

		Expect(k8sClient.Delete(ctx, claim)).To(Succeed())

		By("deleting the attachments first")
		expectDeleting(optimize)
		expectDeleting(schedule)
		expectNotDeleting(cluster)

		By("deleting the cluster once the attachments are gone")
		setTestHold(optimize, false)
		setTestHold(schedule, false)
		expectDeleting(cluster)
		for _, section := range []string{pluginstartup.SectionSecondaryStorage, pluginstartup.SectionEncryptionKey} {
			expectNotDeleting(&pluginsv1alpha1.PluginRequest{ObjectMeta: objectMeta(requestKey(claim, section))})
		}

		By("deleting the requests once the cluster is gone")
		setTestHold(cluster, false)
		expectGone(claim)
		for _, section := range []string{pluginstartup.SectionSecondaryStorage, pluginstartup.SectionEncryptionKey} {
			_, seen := plugin.deletionPolicyOf(requestKey(claim, section))
			Expect(seen).To(BeTrue(), section)
		}
	})

	It("writes a policy change during the deletion on the requests that are not deleted yet", func() {
		claim := readyClaim(newNamespace(nil))
		cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: claim.Name, Namespace: claim.Namespace}}
		setTestHold(cluster, true)
		Expect(k8sClient.Delete(ctx, claim)).To(Succeed())
		expectDeleting(cluster)

		setOrphanPolicy(claim)
		Eventually(func(g Gomega) {
			for _, section := range []string{pluginstartup.SectionSecondaryStorage, pluginstartup.SectionEncryptionKey} {
				request, err := requestOf(claim, section)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(request.Spec.DeletionPolicy).To(Equal(pluginsv1alpha1.DeletionPolicyOrphan))
			}
		}, timeout, interval).Should(Succeed())
		setTestHold(cluster, false)

		expectPoliciesAtDeletion(claim, pluginsv1alpha1.DeletionPolicyOrphan)
	})

	It("keeps an object that the claim does not control", func() {
		claim := readyClaim(newNamespace(nil))
		optimize := &v1.CamundaOptimize{
			ObjectMeta: metav1.ObjectMeta{Name: claim.Name + "-optimize", Namespace: claim.Namespace},
			Spec: v1.CamundaOptimizeSpec{
				Version:           "8.9.9",
				ManagementAuthRef: "management",
				ClusterRef:        v1.ClusterRef{Name: claim.Name},
			},
		}
		create(optimize)

		Expect(k8sClient.Delete(ctx, claim)).To(Succeed())

		expectGone(claim)
		expectNotDeleting(optimize)
	})

	It("deletes a claim that has no snapshot", func() {
		claim := newClaim(newNamespace(nil))
		claim.Spec.PresetRef = uniqueName("missing-preset")
		create(claim)
		expectFalse(claim, v1.ConditionReady, v1.ReasonInvalidReference, Not(BeEmpty()))
		current := latest(Default, claim)
		Expect(current.Status.ResolvedPolicy).To(BeNil())
		controllerutil.AddFinalizer(current, fleetv1alpha1.FinalizerCleanup)
		Expect(k8sClient.Update(ctx, current)).To(Succeed())

		Expect(k8sClient.Delete(ctx, claim)).To(Succeed())

		expectGone(claim)
	})
})

var _ = Describe("A live cluster claim", func() {
	It("writes a policy change on every request", func() {
		claim := readyClaim(newNamespace(nil))

		setOrphanPolicy(claim)

		Eventually(func(g Gomega) {
			var requests pluginsv1alpha1.PluginRequestList
			g.Expect(k8sClient.List(
				ctx,
				&requests,
				client.InNamespace(claim.Namespace),
				client.MatchingLabels{fleetv1alpha1.LabelClaim: claim.Name},
			)).To(Succeed())
			g.Expect(requests.Items).To(HaveLen(2))
			for _, request := range requests.Items {
				g.Expect(request.Spec.DeletionPolicy).To(Equal(pluginsv1alpha1.DeletionPolicyOrphan))
			}
		}, timeout, interval).Should(Succeed())
	})
})

var _ = Describe("A blocked cluster claim", func() {
	It("writes a policy change on every request", func() {
		environment := allowingEnvironment(map[string][]string{
			pluginstartup.SectionSecondaryStorage: {pluginstartup.RegistrationExistingSecondaryStorage},
			pluginstartup.SectionEncryptionKey:    {registrationKey},
		})
		createEnvironment(environment)
		claim := readyClaim(newNamespace(map[string]string{fleetv1alpha1.LabelEnvironment: environment.Name}))
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(environment), environment)).To(Succeed())
		environment.Spec.Presets.CamundaCluster = []string{"other-preset"}
		Expect(k8sClient.Update(ctx, environment)).To(Succeed())
		expectFalse(claim, v1.ConditionReady, fleetv1alpha1.ReasonNotAllowed, Not(BeEmpty()))

		setOrphanPolicy(claim)

		Eventually(func(g Gomega) {
			for _, section := range []string{pluginstartup.SectionSecondaryStorage, pluginstartup.SectionEncryptionKey} {
				request, err := requestOf(claim, section)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(request.Spec.DeletionPolicy).To(Equal(pluginsv1alpha1.DeletionPolicyOrphan))
			}
		}, timeout, interval).Should(Succeed())
	})
})

var _ = Describe("A cluster claim with a registration that cannot orphan", func() {
	It("keeps the policy Delete on its requests when it asks for Orphan", func() {
		claim := newClaim(newNamespace(nil), selection(pluginstartup.SectionEncryptionKey, registrationDeleteOnly))
		create(claim)
		eventuallyCluster(claim)

		setOrphanPolicy(claim)
		expectFalse(claim, v1.ConditionReady, fleetv1alpha1.ReasonNotAllowed, ContainSubstring(registrationDeleteOnly))

		Consistently(func(g Gomega) {
			for _, section := range []string{pluginstartup.SectionSecondaryStorage, pluginstartup.SectionEncryptionKey} {
				request, err := requestOf(claim, section)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(request.Spec.DeletionPolicy).To(Equal(pluginsv1alpha1.DeletionPolicyDelete))
			}
		}, consistently, interval).Should(Succeed())
	})
})

// readyClaim creates a claim in namespace with the requests
// secondary-storage and encryption-key, and waits until its cluster exists
// and the fake plugin holds both requests.
func readyClaim(namespace string) *fleetv1alpha1.CamundaClusterClaim {
	GinkgoHelper()
	claim := newClaim(namespace, selection(pluginstartup.SectionEncryptionKey, registrationKey))
	create(claim)

	eventuallyCluster(claim)
	Eventually(func(g Gomega) {
		for _, section := range []string{pluginstartup.SectionSecondaryStorage, pluginstartup.SectionEncryptionKey} {
			request, err := requestOf(claim, section)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(request.Finalizers).To(ContainElement(pluginsv1alpha1.FinalizerCleanup))
		}
	}, timeout, interval).Should(Succeed())

	return claim
}

// ownedOptimize creates <claim>-optimize with a controller reference to
// claim and the finalizer testHold.
func ownedOptimize(claim *fleetv1alpha1.CamundaClusterClaim) *v1.CamundaOptimize {
	GinkgoHelper()
	optimize := &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{
			Name:       claim.Name + "-optimize",
			Namespace:  claim.Namespace,
			Finalizers: []string{testHold},
		},
		Spec: v1.CamundaOptimizeSpec{
			Version:           "8.9.9",
			ManagementAuthRef: "management",
			ClusterRef:        v1.ClusterRef{Name: claim.Name},
		},
	}
	Expect(controllerutil.SetControllerReference(claim, optimize, k8sClient.Scheme())).To(Succeed())
	create(optimize)

	return optimize
}

func requestKey(claim *fleetv1alpha1.CamundaClusterClaim, section string) types.NamespacedName {
	return types.NamespacedName{Namespace: claim.Namespace, Name: claim.Name + "-" + section}
}

func objectMeta(key types.NamespacedName) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace}
}

// setOrphanPolicy writes spec.deletionPolicy: Orphan on claim.
func setOrphanPolicy(claim *fleetv1alpha1.CamundaClusterClaim) {
	GinkgoHelper()
	Eventually(func() error {
		current := &fleetv1alpha1.CamundaClusterClaim{}
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), current); err != nil {
			return err
		}
		current.Spec.DeletionPolicy = pluginsv1alpha1.DeletionPolicyOrphan
		return k8sClient.Update(ctx, current)
	}, timeout, interval).Should(Succeed())
}

// setTestHold adds or removes the finalizer testHold on obj.
func setTestHold(obj client.Object, hold bool) {
	GinkgoHelper()
	Eventually(func() error {
		if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj); err != nil {
			return err
		}
		if hold {
			controllerutil.AddFinalizer(obj, testHold)
		} else {
			controllerutil.RemoveFinalizer(obj, testHold)
		}
		return k8sClient.Update(ctx, obj)
	}, timeout, interval).Should(Succeed())
}

// expectPoliciesAtDeletion polls until the fake plugin saw the deletion of
// both requests of claim with policy.
func expectPoliciesAtDeletion(claim *fleetv1alpha1.CamundaClusterClaim, policy pluginsv1alpha1.DeletionPolicy) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		for _, section := range []string{pluginstartup.SectionSecondaryStorage, pluginstartup.SectionEncryptionKey} {
			got, seen := plugin.deletionPolicyOf(requestKey(claim, section))
			g.Expect(seen).To(BeTrue(), section)
			g.Expect(got).To(Equal(policy), section)
		}
	}, timeout, interval).Should(Succeed())
}

// expectGone polls until obj does not exist.
func expectGone(obj client.Object) {
	GinkgoHelper()
	Eventually(func() bool {
		return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj))
	}, timeout, interval).Should(BeTrue())
}

// expectDeleting polls until obj has a deletion timestamp.
func expectDeleting(obj client.Object) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
		g.Expect(obj.GetDeletionTimestamp()).NotTo(BeNil())
	}, timeout, interval).Should(Succeed())
}

// expectNotDeleting checks for a while that obj exists without a deletion
// timestamp.
func expectNotDeleting(obj client.Object) {
	GinkgoHelper()
	Consistently(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(obj), obj)).To(Succeed())
		g.Expect(obj.GetDeletionTimestamp()).To(BeNil())
	}, consistently, interval).Should(Succeed())
}
