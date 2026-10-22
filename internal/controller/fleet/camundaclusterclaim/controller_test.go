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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

// consistently bounds the Consistently checks of an absence.
const consistently = 2 * time.Second

var _ = Describe("CamundaClusterClaim controller", func() {
	It("writes no PluginRequest before status.resolvedPolicy is recorded", func() {
		claim := newClaim(newNamespace(nil), selection(pluginstartup.SectionEncryptionKey, registrationKey))
		create(claim)

		Eventually(func() error {
			_, err := requestOf(claim, pluginstartup.SectionEncryptionKey)
			return err
		}, timeout, interval).Should(Succeed())
		recorded := witness.recorded(client.ObjectKeyFromObject(claim))
		Expect(recorded).NotTo(BeEmpty())
		Expect(recorded).To(HaveEach(BeTrue()))
	})

	It("writes a request with an input only after its producer has outputs", func() {
		plugin.hold(registrationKey)
		DeferCleanup(plugin.release, registrationKey)
		claim := newClaim(
			newNamespace(nil),
			selection(pluginstartup.SectionVolumeEncryption, registrationVolumes),
			selection(pluginstartup.SectionEncryptionKey, registrationKey),
		)
		create(claim)

		expectFalse(
			claim,
			fleetv1alpha1.ConditionRequestsReady,
			"Blocked",
			ContainSubstring("waiting for the outputs of encryption-key"),
		)
		Consistently(func() bool {
			_, err := requestOf(claim, pluginstartup.SectionVolumeEncryption)
			return apierrors.IsNotFound(err)
		}, consistently, interval).Should(BeTrue())

		plugin.release(registrationKey)

		Eventually(func(g Gomega) {
			request, err := requestOf(claim, pluginstartup.SectionVolumeEncryption)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(request.Spec.Inputs).To(HaveKeyWithValue("keyRef", keyARN))
		}, timeout, interval).Should(Succeed())
	})

	It("lets the cloud identity trust the Elasticsearch ServiceAccount", func() {
		claim := newClaim(newNamespace(nil), selection(pluginstartup.SectionCloudIdentity, registrationIdentity))
		claim.Spec.Plugins[0].Registration = pluginstartup.RegistrationElasticsearch
		create(claim)

		Eventually(func(g Gomega) {
			request, err := requestOf(claim, pluginstartup.SectionCloudIdentity)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(request.Spec.Target.AdditionalServiceAccounts).To(Equal([]string{"orders-es-es"}))
		}, timeout, interval).Should(Succeed())
	})

	It("adds no account without built-in storage", func() {
		claim := newClaim(newNamespace(nil), selection(pluginstartup.SectionCloudIdentity, registrationIdentity))
		create(claim)

		Eventually(func(g Gomega) {
			request, err := requestOf(claim, pluginstartup.SectionCloudIdentity)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(request.Spec.Target.ServiceAccountName).To(Equal("orders-camunda"))
			g.Expect(request.Spec.Target.AdditionalServiceAccounts).To(BeEmpty())
		}, timeout, interval).Should(Succeed())
	})

	It("gives a request whose registration declares releaseRef the release of the claim", func() {
		claim := newClaim(newNamespace(nil), selection(pluginstartup.SectionEncryptionKey, registrationVersioned))
		create(claim)

		Eventually(func(g Gomega) {
			request, err := requestOf(claim, pluginstartup.SectionEncryptionKey)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(request.Spec.Parameters).NotTo(BeNil())
			g.Expect(string(request.Spec.Parameters.Raw)).To(MatchJSON(`{"releaseRef":"` + releaseName + `"}`))
		}, timeout, interval).Should(Succeed())
	})

	It("writes the CamundaCluster only after every slot request has outputs", func() {
		plugin.hold(registrationIdentity)
		DeferCleanup(plugin.release, registrationIdentity)
		claim := newClaim(newNamespace(nil), selection(pluginstartup.SectionCloudIdentity, registrationIdentity))
		create(claim)

		expectFalse(
			claim, v1.ConditionReady, "PrerequisiteNotMet", ContainSubstring("cloud-identity"),
		)
		Consistently(func() bool {
			_, err := clusterOf(claim)
			return apierrors.IsNotFound(err)
		}, consistently, interval).Should(BeTrue())

		plugin.release(registrationIdentity)

		eventuallyCluster(claim)
	})

	It("does not wait for a request without a slot", func() {
		plugin.hold(registrationKey)
		DeferCleanup(plugin.release, registrationKey)
		claim := newClaim(newNamespace(nil), selection(pluginstartup.SectionEncryptionKey, registrationKey))
		create(claim)

		eventuallyCluster(claim)
		request, err := requestOf(claim, pluginstartup.SectionEncryptionKey)
		Expect(err).NotTo(HaveOccurred())
		Expect(meta.FindStatusCondition(request.Status.Conditions, pluginsv1alpha1.ConditionOutputsReady)).To(BeNil())
	})

	It("gives the Connectors their own account and annotations", func() {
		claim := newClaim(
			newNamespace(nil),
			selection(pluginstartup.SectionConnectorsPermissions, registrationConnectors),
			selection(pluginstartup.SectionCloudIdentity, registrationIdentity),
		)
		create(claim)

		cluster := eventuallyCluster(claim)
		request, err := requestOf(claim, pluginstartup.SectionConnectorsPermissions)
		Expect(err).NotTo(HaveOccurred())
		Expect(request.Spec.Target.ServiceAccountName).To(Equal("orders-connectors"))
		Expect(cluster.Spec.Connectors.ServiceAccount.Name).To(Equal("orders-connectors"))
		Expect(cluster.Spec.Connectors.ServiceAccount.Annotations).
			To(Equal(map[string]string{"eks.amazonaws.com/role-arn": "arn:connectors"}))
		Expect(cluster.Spec.ServiceAccount.Annotations).
			To(Equal(map[string]string{"eks.amazonaws.com/role-arn": "arn:camunda"}))
	})

	It("writes the slot fields, spec.cluster, the label, and the owner reference", func() {
		claim := newClaim(
			newNamespace(nil),
			selection(pluginstartup.SectionAccess, registrationAccess),
			selection(pluginstartup.SectionVolumeEncryption, registrationVolumes),
			selection(pluginstartup.SectionEncryptionKey, registrationKey),
		)
		claim.Spec.Cluster = &v1.CamundaClusterSpec{Zeebe: &v1.ZeebeSpec{Partitions: ptr.To[int32](3)}}
		create(claim)

		cluster := eventuallyCluster(claim)
		Expect(cluster.Spec.ExternalURL).To(Equal(accessURL))
		Expect(cluster.Spec.StorageRef).To(Equal(existingStorageName))
		Expect(cluster.Spec.Zeebe.StorageClassName).To(HaveValue(Equal(encryptedClass)))
		Expect(cluster.Spec.Zeebe.Partitions).To(HaveValue(Equal(int32(3))))
		Expect(cluster.Spec.PresetRef).To(Equal(presetName))
		Expect(cluster.Spec.ReleaseRef).To(Equal(releaseName))
		Expect(cluster.Spec.PlatformConfigRef).To(Equal(platformConfigName))
		Expect(cluster.Spec.ServiceAccount.Name).To(Equal("orders-camunda"))
		Expect(cluster.Labels).To(HaveKeyWithValue(fleetv1alpha1.LabelClaim, claim.Name))
		owner := metav1.GetControllerOf(cluster)
		Expect(owner).NotTo(BeNil())
		Expect(owner.Kind).To(Equal("CamundaClusterClaim"))
		Expect(owner.Name).To(Equal(claim.Name))
	})

	It("shows the Ready reason and message of the cluster on ClusterReady", func() {
		claim := newClaim(newNamespace(nil))
		create(claim)
		eventuallyCluster(claim)

		setClusterReady(
			claim,
			metav1.ConditionFalse,
			v1.ReasonInvalidReference,
			`CamundaPlatformConfig "x" does not exist`,
		)

		Eventually(func(g Gomega) {
			ready := meta.FindStatusCondition(latest(g, claim).Status.Conditions, fleetv1alpha1.ConditionClusterReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Message).To(Equal(`InvalidReference: CamundaPlatformConfig "x" does not exist`))
		}, timeout, interval).Should(Succeed())
	})

	It("turns Ready True when the requests and the cluster are ready", func() {
		claim := newClaim(newNamespace(nil), selection(pluginstartup.SectionEncryptionKey, registrationKey))
		create(claim)
		eventuallyCluster(claim)

		setClusterReady(claim, metav1.ConditionTrue, v1.ReasonHealthy, "The cluster is ready")

		Eventually(func(g Gomega) {
			current := latest(g, claim)
			for _, conditionType := range []string{
				v1.ConditionReady, fleetv1alpha1.ConditionRequestsReady, fleetv1alpha1.ConditionClusterReady,
			} {
				g.Expect(meta.IsStatusConditionTrue(current.Status.Conditions, conditionType)).
					To(BeTrue(), conditionType)
			}
			g.Expect(current.Status.Cluster).To(Equal(&v1.ClusterRef{Name: claim.Name}))
			g.Expect(current.Status.Requests).To(ConsistOf(
				fleetv1alpha1.RequestStatus{
					Section:      pluginstartup.SectionSecondaryStorage,
					Registration: pluginstartup.RegistrationExistingSecondaryStorage,
					Name:         "orders-secondary-storage",
					OutputsReady: metav1.ConditionTrue,
					Ready:        metav1.ConditionTrue,
				},
				fleetv1alpha1.RequestStatus{
					Section:      pluginstartup.SectionEncryptionKey,
					Registration: registrationKey,
					Name:         "orders-encryption-key",
					OutputsReady: metav1.ConditionTrue,
					Ready:        metav1.ConditionTrue,
				},
			))
			g.Expect(current.Status.ObservedGeneration).To(Equal(current.Generation))
		}, timeout, interval).Should(Succeed())
	})

	It("keeps status.resolvedPolicy when the Environment changes", func() {
		environment := allowingEnvironment(map[string][]string{
			pluginstartup.SectionSecondaryStorage: {pluginstartup.RegistrationExistingSecondaryStorage},
		})
		createEnvironment(environment)
		claim := newClaim(newNamespace(map[string]string{fleetv1alpha1.LabelEnvironment: environment.Name}))
		create(claim)

		var recorded *fleetv1alpha1.ResolvedPolicy
		Eventually(func(g Gomega) {
			current := latest(g, claim)
			g.Expect(current.Status.ResolvedPolicy).NotTo(BeNil())
			g.Expect(current.Status.Environment).To(Equal(environment.Name))
			recorded = current.Status.ResolvedPolicy
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(environment), environment)).To(Succeed())
		environment.Spec.DeletionPolicy = pluginsv1alpha1.DeletionPolicyOrphan
		environment.Spec.Sections[pluginstartup.SectionSecondaryStorage] = fleetv1alpha1.SectionPolicy{
			Allowed: []string{
				pluginstartup.RegistrationElasticsearch, pluginstartup.RegistrationExistingSecondaryStorage,
			},
		}
		Expect(k8sClient.Update(ctx, environment)).To(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(latest(g, claim).Status.ResolvedPolicy).To(Equal(recorded))
		}, consistently, interval).Should(Succeed())
	})

	It("gives a refused claim no finalizer, so it deletes at once", func() {
		claim := newClaim(newNamespace(nil))
		claim.Spec.PresetRef = uniqueName("missing-preset")
		create(claim)
		expectFalse(claim, v1.ConditionReady, v1.ReasonInvalidReference, Not(BeEmpty()))

		Expect(latest(Default, claim).Finalizers).To(BeEmpty())
		Expect(k8sClient.Delete(ctx, claim)).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), claim))
		}, timeout, interval).Should(BeTrue())
	})

	It("writes the same requests on each pass when the Environment gives a parameter null", func() {
		environment := allowingEnvironment(map[string][]string{
			pluginstartup.SectionSecondaryStorage: {pluginstartup.RegistrationExistingSecondaryStorage},
			pluginstartup.SectionEncryptionKey:    {registrationKey},
		})
		environment.Spec.Parameters = map[string]fleetv1alpha1.ParameterPolicies{
			registrationKey: {
				"region": {Value: &apiextensionsv1.JSON{Raw: []byte(`"eu"`)}},
				"tier":   {Value: &apiextensionsv1.JSON{Raw: []byte(`null`)}},
			},
		}
		createEnvironment(environment)
		claim := newClaim(
			newNamespace(map[string]string{fleetv1alpha1.LabelEnvironment: environment.Name}),
			selection(pluginstartup.SectionEncryptionKey, registrationKey),
		)
		create(claim)

		var first *pluginsv1alpha1.PluginRequest
		Eventually(func(g Gomega) {
			request, err := requestOf(claim, pluginstartup.SectionEncryptionKey)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(request.Spec.Parameters).NotTo(BeNil())
			first = request
		}, timeout, interval).Should(Succeed())
		Expect(string(first.Spec.Parameters.Raw)).To(MatchJSON(`{"region":"eu"}`))

		current := latest(Default, claim)
		current.Annotations = map[string]string{"test.camunda.io/touch": "1"}
		Expect(k8sClient.Update(ctx, current)).To(Succeed())

		Consistently(func(g Gomega) {
			request, err := requestOf(claim, pluginstartup.SectionEncryptionKey)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(request.Generation).To(Equal(first.Generation))
			g.Expect(request.Spec).To(Equal(first.Spec))
		}, consistently, interval).Should(Succeed())
	})

	It("reports the refused CamundaCluster on ClusterReady", func() {
		claim := newClaim(newNamespace(nil), selection(pluginstartup.SectionEncryptionKey, registrationKey))
		claim.Spec.Plugins = claim.Spec.Plugins[1:]
		create(claim)

		Eventually(func(g Gomega) {
			ready := meta.FindStatusCondition(latest(g, claim).Status.Conditions, fleetv1alpha1.ConditionClusterReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Message).To(ContainSubstring("spec.storageRef is required"))
		}, timeout, interval).Should(Succeed())
	})

	It("does not take over a CamundaCluster that another owner controls", func() {
		claim := newClaim(newNamespace(nil))
		owner := foreignOwner(claim.Namespace)
		foreign := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:            claim.Name,
				Namespace:       claim.Namespace,
				OwnerReferences: []metav1.OwnerReference{controllerReference(owner)},
			},
			Spec: v1.CamundaClusterSpec{PresetRef: "other", PlatformConfigRef: "other", StorageRef: "other"},
		}
		create(foreign)
		create(claim)

		expectFalse(
			claim,
			fleetv1alpha1.ConditionClusterReady,
			"Blocked",
			ContainSubstring(owner.Name),
		)
		cluster, err := clusterOf(claim)
		Expect(err).NotTo(HaveOccurred())
		Expect(cluster.Spec.PresetRef).To(Equal("other"))
		Consistently(func(g Gomega) {
			g.Expect(latest(g, claim).Status.Cluster).To(BeNil())
		}, consistently, interval).Should(Succeed())

		By("writing its own cluster once the foreign one is gone")
		Expect(k8sClient.Delete(ctx, foreign)).To(Succeed())
		Eventually(func(g Gomega) {
			cluster, err := clusterOf(claim)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(cluster.Spec.PresetRef).To(Equal(presetName))
			g.Expect(metav1.GetControllerOf(cluster).Name).To(Equal(claim.Name))
			g.Expect(latest(g, claim).Status.Cluster).To(Equal(&v1.ClusterRef{Name: claim.Name}))
		}, timeout, interval).Should(Succeed())
	})

	DescribeTable(
		"does not take over a request that another claim controls",
		func(claimRef pluginsv1alpha1.ClaimRef) {
			claim := newClaim(newNamespace(nil), selection(pluginstartup.SectionEncryptionKey, registrationKey))
			owner := foreignOwner(claim.Namespace)
			foreign := &pluginsv1alpha1.PluginRequest{
				ObjectMeta: metav1.ObjectMeta{
					Name:            claim.Name + "-" + pluginstartup.SectionEncryptionKey,
					Namespace:       claim.Namespace,
					Labels:          map[string]string{fleetv1alpha1.LabelClaim: claimRef.Name},
					OwnerReferences: []metav1.OwnerReference{controllerReference(owner)},
				},
				Spec: pluginsv1alpha1.PluginRequestSpec{
					Section:        pluginstartup.SectionEncryptionKey,
					Registration:   registrationKey,
					ClaimRef:       claimRef,
					Target:         pluginsv1alpha1.TargetRef{Kind: "CamundaCluster", Name: claimRef.Name},
					DeletionPolicy: pluginsv1alpha1.DeletionPolicyDelete,
				},
			}
			create(foreign)
			create(claim)

			expectFalse(
				claim,
				fleetv1alpha1.ConditionRequestsReady,
				"Blocked",
				ContainSubstring(owner.Name),
			)
			request, err := requestOf(claim, pluginstartup.SectionEncryptionKey)
			Expect(err).NotTo(HaveOccurred())
			Expect(request.Spec.ClaimRef).To(Equal(claimRef))

			By("writing its own request once the foreign one is gone")
			Expect(k8sClient.Delete(ctx, foreign)).To(Succeed())
			Eventually(func(g Gomega) {
				request, err := requestOf(claim, pluginstartup.SectionEncryptionKey)
				g.Expect(err).NotTo(HaveOccurred())
				g.Expect(request.Spec.ClaimRef).To(Equal(pluginsv1alpha1.ClaimRef{
					Kind: pluginsv1alpha1.ClaimKindCamundaClusterClaim, Name: claim.Name,
				}))
			}, timeout, interval).Should(Succeed())
		},
		Entry("a management cluster claim of the same name", pluginsv1alpha1.ClaimRef{
			Kind: pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim, Name: "orders",
		}),
		// orders-encryption with a section key also writes orders-encryption-key.
		Entry("a cluster claim of another name", pluginsv1alpha1.ClaimRef{
			Kind: pluginsv1alpha1.ClaimKindCamundaClusterClaim, Name: "orders-encryption",
		}),
	)
})

// foreignOwner creates a ConfigMap in namespace that controls an object the
// claim would write.
func foreignOwner(namespace string) *corev1.ConfigMap {
	GinkgoHelper()
	owner := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: uniqueName("owner"), Namespace: namespace}}
	create(owner)
	return owner
}

func controllerReference(owner *corev1.ConfigMap) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Name:       owner.Name,
		UID:        owner.UID,
		Controller: new(true),
	}
}
