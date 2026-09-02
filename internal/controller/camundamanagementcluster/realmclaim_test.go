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
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
)

var _ = Describe("CamundaManagementCluster controller and the claim on the realm", func() {
	It("parks the second plane that names the realm of the first, and frees it on deletion", func() {
		first := newScenario(withExternalKeycloak)
		expectReadyWhileStamping(first.mc, identityKey(first))
		expectRealmClaimHolder(externalRealmTarget(), first.mc)

		second := newScenario(withExternalKeycloak)

		Eventually(func(g Gomega) {
			ready := conditionOf(g, second.mc, v1.ConditionReady)
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(v1.ReasonRealmClaimedElsewhere))
			g.Expect(ready.Message).To(ContainSubstring(first.namespace + "/" + first.mc.Name))
		}, timeout, interval).Should(Succeed())

		By("rendering nothing that would touch the realm")
		Consistently(func(g Gomega) {
			var workload appsv1.Deployment
			err := k8sClient.Get(ctx, identityKey(second), &workload)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, "2s", interval).Should(Succeed())

		By("letting the parked plane proceed once the holder is deleted")
		Expect(deleteManagementCluster(first.mc)).To(Succeed())

		expectReadyWhileStamping(second.mc, identityKey(second))
		expectRealmClaimHolder(externalRealmTarget(), second.mc)
	})

	// A Lease at the name of the realm that this operator did not write has
	// no provenance. It blocks without a takeover, and only its removal by
	// hand frees the realm.
	It("parks on a Lease that names no management cluster until it is removed", func() {
		foreign := &coordinationv1.Lease{ObjectMeta: metav1.ObjectMeta{
			Namespace: testClaimNamespace,
			Name:      realmLeaseKey(externalRealmTarget()).Name,
		}}
		Expect(k8sClient.Create(ctx, foreign)).To(Succeed())
		DeferCleanup(func() {
			_ = client.IgnoreNotFound(k8sClient.Delete(ctx, foreign))
		})

		s := newScenario(withExternalKeycloak)

		Eventually(func(g Gomega) {
			ready := conditionOf(g, s.mc, v1.ConditionReady)
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(v1.ReasonRealmClaimedElsewhere))
			g.Expect(ready.Message).To(ContainSubstring("names no CamundaManagementCluster"))
			g.Expect(ready.Message).To(ContainSubstring(foreign.Name))
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, foreign)).To(Succeed())

		expectReadyWhileStamping(s.mc, identityKey(s))
		expectRealmClaimHolder(externalRealmTarget(), s.mc)
	})

	// A crash between the claim and the release leaves a Lease whose holder
	// is gone. It must not block the realm forever.
	It("takes over the Lease of a holder that no longer exists", func() {
		ghost := &v1.CamundaManagementCluster{ObjectMeta: metav1.ObjectMeta{
			Namespace: "gone-ns", Name: "gone", UID: "gone-uid",
		}}
		stale := components.NewRealmClaimLease(testClaimNamespace, externalRealmTarget(), ghost)
		Expect(k8sClient.Create(ctx, stale)).To(Succeed())
		DeferCleanup(func() {
			_ = client.IgnoreNotFound(k8sClient.Delete(ctx, stale))
		})

		s := newScenario(withExternalKeycloak)

		expectReadyWhileStamping(s.mc, identityKey(s))
		expectRealmClaimHolder(externalRealmTarget(), s.mc)
	})

	// The Lease of the old realm goes only after the callbacks left it. A
	// release before that would let another plane register in a realm this
	// one still tidies.
	It("releases the realm it leaves only after the callbacks left it", func() {
		first := startFakeKeycloak(withOptimizeClient())
		second := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(first))
		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(HaveLen(1))
			g.Expect(readManagementCluster(g, s.mc).Status.CallbackRealm).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		expectRealmClaimHolder(fakeRealmTarget(first), s.mc)

		first.setRefuseUpdate(true)
		retargetKeycloak(s.mc, second.url)

		By("keeping the old claim while the old Keycloak refuses the withdrawal")
		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(v1.ReasonWriteFailed))
		}, timeout, interval).Should(Succeed())
		expectRealmClaimHolder(fakeRealmTarget(second), s.mc)
		Consistently(func(g Gomega) {
			var lease coordinationv1.Lease
			g.Expect(k8sClient.Get(ctx, realmLeaseKey(fakeRealmTarget(first)), &lease)).To(Succeed())
		}, "2s", interval).Should(Succeed())

		By("releasing the old claim once the withdrawal went through")
		first.setRefuseUpdate(false)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(BeEmpty())
			var lease coordinationv1.Lease
			err := k8sClient.Get(ctx, realmLeaseKey(fakeRealmTarget(first)), &lease)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())
		expectRealmClaimHolder(fakeRealmTarget(second), s.mc)
	})

	// A parked plane still gives back the realm it left, once the callbacks
	// left it. A claim kept there would block every later claimant of the
	// old realm for as long as the park stands.
	It("releases the realm it left even while parked on the next one", func() {
		holder := newScenario(withExternalKeycloak)
		expectReadyWhileStamping(holder.mc, identityKey(holder))
		expectRealmClaimHolder(externalRealmTarget(), holder.mc)

		old := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(old))
		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(old.redirectURIs()).To(HaveLen(1))
			g.Expect(readManagementCluster(g, s.mc).Status.CallbackRealm).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		expectRealmClaimHolder(fakeRealmTarget(old), s.mc)

		retargetKeycloak(s.mc, keycloakExternalURL)

		Eventually(func(g Gomega) {
			ready := conditionOf(g, s.mc, v1.ConditionReady)
			g.Expect(ready.Reason).To(Equal(v1.ReasonRealmClaimedElsewhere))

			g.Expect(old.redirectURIs()).To(BeEmpty())
		}, timeout, interval).Should(Succeed())

		// The claim on the old realm goes once nothing of the plane points
		// there: the record is cleared by the withdrawal above, and the
		// Deployment of that realm is the last thing left.
		By("releasing the old claim once nothing of the plane points there")
		deleteIdentityDeployment(s)
		nudge(s.mc)

		Eventually(func(g Gomega) {
			var lease coordinationv1.Lease
			err := k8sClient.Get(ctx, realmLeaseKey(fakeRealmTarget(old)), &lease)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		expectRealmClaimHolder(externalRealmTarget(), holder.mc)
	})

	// A Management Identity that points at a realm writes its clients again
	// whenever a pod of it starts, so the realm of a running workload is never
	// left for another plane, not even by a plane parked on the realm it wants
	// next.
	It("keeps the claim while a Management Identity workload points at the realm", func() {
		holder := newScenario(withExternalKeycloak)
		expectReadyWhileStamping(holder.mc, identityKey(holder))
		expectRealmClaimHolder(externalRealmTarget(), holder.mc)

		old := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(old))
		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(old.redirectURIs()).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())
		startIdentityPod(identityKey(s))
		// A pod that is not ready is one that still starts, and the
		// withdrawal of the callbacks waits for it. This pod ran long ago.
		markIdentityPodsReady(s)
		expectRealmClaimHolder(fakeRealmTarget(old), s.mc)

		retargetKeycloak(s.mc, keycloakExternalURL)

		Eventually(func(g Gomega) {
			ready := conditionOf(g, s.mc, v1.ConditionReady)
			g.Expect(ready.Reason).To(Equal(v1.ReasonRealmClaimedElsewhere))

			g.Expect(old.redirectURIs()).To(BeEmpty())
		}, timeout, interval).Should(Succeed())

		By("keeping the claim while the pod of the old realm runs")
		Consistently(func(g Gomega) {
			var lease coordinationv1.Lease
			g.Expect(k8sClient.Get(ctx, realmLeaseKey(fakeRealmTarget(old)), &lease)).To(Succeed())
		}, "2s", interval).Should(Succeed())

		// The withdrawal deleted the Deployment of the old realm before it
		// cleared the record, so the pod is the last thing that points there.
		By("releasing the claim once no workload of the plane points at it")
		deleteIdentityPods(s)
		deleteIdentityDeployment(s)
		nudge(s.mc)

		Eventually(func(g Gomega) {
			var lease coordinationv1.Lease
			err := k8sClient.Get(ctx, realmLeaseKey(fakeRealmTarget(old)), &lease)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())
	})

	// The sweep runs on the failed pre-check path too. A reference that stops
	// resolving must not strand the claim on a realm that nothing of the plane
	// names any more.
	It("releases the realm it left even when the spec no longer resolves", func() {
		old := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(old))
		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(old.redirectURIs()).To(HaveLen(1))
			g.Expect(readManagementCluster(g, s.mc).Status.CallbackRealm).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		expectRealmClaimHolder(fakeRealmTarget(old), s.mc)

		By("pointing the spec at a Keycloak whose administrator Secret is missing")
		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.IdentityProvider.ExternalKeycloak.URL = "https://second.example.com/auth"
			latest.Spec.IdentityProvider.ExternalKeycloak.AdminCredentialsSecretRef.Name = "gone"
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			ready := conditionOf(g, s.mc, v1.ConditionReady)
			g.Expect(ready.Reason).To(Equal(v1.ReasonMissingSecret))

			g.Expect(old.redirectURIs()).To(BeEmpty())
		}, timeout, interval).Should(Succeed())

		By("releasing the old claim once nothing of the plane points there")
		deleteIdentityDeployment(s)
		nudge(s.mc)

		Eventually(func(g Gomega) {
			var lease coordinationv1.Lease
			err := k8sClient.Get(ctx, realmLeaseKey(fakeRealmTarget(old)), &lease)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())
	})

	// The oidc mode administers no realm, so a plane that moves there keeps
	// no claim on the realm it administered before.
	It("releases the realm when the spec moves to the oidc mode", func() {
		s := newScenario(withExternalKeycloak)
		expectReadyWhileStamping(s.mc, identityKey(s))
		expectRealmClaimHolder(externalRealmTarget(), s.mc)

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.IdentityProvider = v1.IdentityProviderSpec{OIDC: &v1.ManagementOIDCSpec{}}
			latest.Spec.Identity.Admin = v1.IdentityAdminSpec{
				ClaimName: "oid", ClaimValue: "admin-oid",
			}
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			var lease coordinationv1.Lease
			err := k8sClient.Get(ctx, realmLeaseKey(externalRealmTarget()), &lease)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())
	})

	// A suspended plane touches no claim: the realm it holds stays held, and
	// a realm the spec now names is claimed on resume.
	It("keeps its claim and takes none while suspended", func() {
		s := newScenario(withExternalKeycloak)
		expectReadyWhileStamping(s.mc, identityKey(s))
		expectRealmClaimHolder(externalRealmTarget(), s.mc)

		retargeted := v1.KeycloakRealmTarget{
			URL: "https://second.example.com/auth", Realm: "camunda-platform",
		}
		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.Suspend = true
			latest.Spec.IdentityProvider.ExternalKeycloak.URL = retargeted.URL
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			// Envtest runs no Deployment controller, so the scale to zero is
			// stamped the way a rollout is.
			stampDeploymentReady(g, identityKey(s))

			ready := conditionOf(g, s.mc, v1.ConditionReady)
			g.Expect(ready.Reason).To(Equal(string(component.Suspended)))
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			var lease coordinationv1.Lease
			g.Expect(k8sClient.Get(ctx, realmLeaseKey(externalRealmTarget()), &lease)).To(Succeed())
			err := k8sClient.Get(ctx, realmLeaseKey(retargeted), &lease)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, "2s", interval).Should(Succeed())
	})

	// A suspended plane touches no claim on any path. A pre-check that stops
	// failing passes while it sleeps must not give its realm back either.
	It("keeps its claim while suspended even when the spec no longer resolves", func() {
		s := newScenario(withExternalKeycloak)
		expectReadyWhileStamping(s.mc, identityKey(s))
		expectRealmClaimHolder(externalRealmTarget(), s.mc)

		By("suspending the plane, which scales its Management Identity to zero")
		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.Suspend = true
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			// Envtest runs no Deployment controller, so the scale to zero is
			// stamped the way a rollout is.
			stampDeploymentReady(g, identityKey(s))

			ready := conditionOf(g, s.mc, v1.ConditionReady)
			g.Expect(ready.Reason).To(Equal(string(component.Suspended)))
		}, timeout, interval).Should(Succeed())

		By("pointing the sleeping plane at a Keycloak whose administrator Secret is missing")
		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.IdentityProvider.ExternalKeycloak.URL = "https://second.example.com/auth"
			latest.Spec.IdentityProvider.ExternalKeycloak.AdminCredentialsSecretRef.Name = "gone"
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			ready := conditionOf(g, s.mc, v1.ConditionReady)
			g.Expect(ready.Reason).To(Equal(v1.ReasonMissingSecret))
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			var lease coordinationv1.Lease
			g.Expect(k8sClient.Get(ctx, realmLeaseKey(externalRealmTarget()), &lease)).To(Succeed())
		}, "2s", interval).Should(Succeed())
	})

	// The keycloak mode owns the Keycloak it runs, and deletes it with the
	// plane, so the realm needs no claim there.
	It("claims no realm in the keycloak mode", func() {
		s := newScenario(withManagedKeycloak)

		Eventually(func(g Gomega) {
			conditionOf(g, s.mc, v1.ConditionKeycloakReady)
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			var leases coordinationv1.LeaseList
			g.Expect(k8sClient.List(
				ctx, &leases,
				client.InNamespace(testClaimNamespace),
				client.MatchingLabels(components.RealmClaimLeaseLabels(s.mc.Name)),
			)).To(Succeed())
			g.Expect(leases.Items).To(BeEmpty())
		}, "2s", interval).Should(Succeed())
	})
})

// An entry of extraEnv with the name of a rendered variable replaces it, so a
// spec that sets the Keycloak of Management Identity there would administer a
// realm that this plane never claimed.
var _ = Describe("the schema of the Management Identity block", func() {
	It("refuses an extraEnv entry that names the Keycloak of the realm", func() {
		namespace := newNamespace()
		mc := newManagementCluster(namespace, "any-database-config")
		mc.Spec.PlatformConfigRef = "any-platform-config"
		mc.Spec.Identity.ExtraEnv = []corev1.EnvVar{
			{Name: "KEYCLOAK_URL", Value: "https://another.example.com/auth"},
		}

		err := k8sClient.Create(ctx, mc)

		Expect(err).To(MatchError(ContainSubstring(
			"extraEnv must not set KEYCLOAK_URL or KEYCLOAK_REALM",
		)))
	})
})

// Only the externalKeycloak mode names a realm from the spec, because it is
// the only mode that claims one. A plane that moves to the keycloak mode
// therefore gives back a realm whose identity the Keycloak it now runs
// happens to share.
func TestReleaseUnusedRealmsNamesTheSpecRealmOnlyWhereItClaims(t *testing.T) {
	mc := &v1.CamundaManagementCluster{ObjectMeta: metav1.ObjectMeta{
		Namespace: "my-management-ns", Name: "my-management", UID: "management-uid",
	}}
	mc.Spec.IdentityProvider.Keycloak = &v1.ManagedKeycloakSpec{}

	provider, err := components.ResolveIdentityProvider(components.Input{Cluster: mc})
	require.NoError(t, err)
	target := components.RealmTarget(provider)
	require.NotNil(t, target)

	lease := components.NewRealmClaimLease(testClaimNamespace, *target, mc)
	r, _ := fakeReconciler(t, mc, lease)

	held, err := r.releaseUnusedRealms(context.Background(), mc)

	require.NoError(t, err)
	assert.False(t, held)
	assert.False(t, exists(t, r, lease), "the keycloak mode claims no realm")
}

// markIdentityPodsReady stamps the Ready condition on every Management
// Identity pod of a scenario, the way a kubelet does once the containers
// pass their probes. Envtest runs none.
func markIdentityPodsReady(s scenario) {
	GinkgoHelper()

	var pods corev1.PodList
	Expect(k8sClient.List(
		ctx,
		&pods,
		client.InNamespace(s.namespace),
		client.MatchingLabels(components.IdentityPodLabels(s.mc)),
	)).To(Succeed())
	for i := range pods.Items {
		pod := &pods.Items[i]
		pod.Status.Conditions = append(pod.Status.Conditions, corev1.PodCondition{
			Type: corev1.PodReady, Status: corev1.ConditionTrue,
		})
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
	}
}

// deleteIdentityPods removes every Management Identity pod of a scenario at
// once. Envtest runs no kubelet, so a graceful deletion would leave the pod
// terminating for its whole grace period.
func deleteIdentityPods(s scenario) {
	GinkgoHelper()

	var pods corev1.PodList
	Expect(k8sClient.List(
		ctx,
		&pods,
		client.InNamespace(s.namespace),
		client.MatchingLabels(components.IdentityPodLabels(s.mc)),
	)).To(Succeed())
	for i := range pods.Items {
		Expect(k8sClient.Delete(ctx, &pods.Items[i], client.GracePeriodSeconds(0))).To(Succeed())
	}
}

// deleteIdentityDeployment removes the Management Identity Deployment of a
// scenario. A parked plane renders nothing, so nothing writes it back.
func deleteIdentityDeployment(s scenario) {
	GinkgoHelper()

	Expect(client.IgnoreNotFound(k8sClient.Delete(ctx, &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Namespace: s.namespace, Name: components.IdentityName(s.mc)},
	}))).To(Succeed())
}

// identityKey is the key of the Management Identity Deployment of a scenario.
func identityKey(s scenario) client.ObjectKey {
	return client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
}

// externalRealmTarget is the realm that withExternalKeycloak names. The realm
// of the spec is empty and resolves to camunda-platform.
func externalRealmTarget() v1.KeycloakRealmTarget {
	return v1.KeycloakRealmTarget{URL: keycloakExternalURL, Realm: "camunda-platform"}
}

// fakeRealmTarget is the realm of a fake Keycloak.
func fakeRealmTarget(keycloak *fakeKeycloak) v1.KeycloakRealmTarget {
	return v1.KeycloakRealmTarget{URL: keycloak.url, Realm: "camunda-platform"}
}

// realmLeaseKey is the key of the Lease that claims target.
func realmLeaseKey(target v1.KeycloakRealmTarget) client.ObjectKey {
	return client.ObjectKey{
		Namespace: testClaimNamespace,
		Name:      components.RealmClaimLeaseName(components.RealmIdentity(target)),
	}
}

// expectRealmClaimHolder polls until the Lease of target names mc as its
// holder.
func expectRealmClaimHolder(target v1.KeycloakRealmTarget, mc *v1.CamundaManagementCluster) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var lease coordinationv1.Lease
		g.Expect(k8sClient.Get(ctx, realmLeaseKey(target), &lease)).To(Succeed())

		holder, ours := components.RealmClaimHolderOf(&lease)
		g.Expect(ours).To(BeTrue())
		g.Expect(holder.Namespace).To(Equal(mc.Namespace))
		g.Expect(holder.Name).To(Equal(mc.Name))
		g.Expect(holder.UID).To(Equal(mc.UID))
	}, timeout, interval).Should(Succeed())
}

// The claim sweep reads the Deployment at the derived name the way it reads
// the labelled ReplicaSets and pods: a workload that runs Management Identity
// against a realm writes its clients whoever owns it, so its claim stays.
func TestIdentityRealmsReadsADeploymentOfAnotherOwner(t *testing.T) {
	mc := finalizingCluster()
	mc.Spec = externalKeycloakCluster("https://new.example.com/auth", "new-admin").Spec

	identity := ownedIdentity(mc)
	identity.OwnerReferences = nil
	pointIdentityAtRealm(identity, finalizerRealm)
	lease := components.NewRealmClaimLease(testClaimNamespace, finalizerRealm, mc)
	r, _ := fakeReconciler(t, mc, identity, lease)

	realms, unknown, err := r.identityRealms(context.Background(), mc)

	require.NoError(t, err)
	assert.False(t, unknown)
	require.Len(t, realms, 1)
	assert.True(t, components.SameRealm(finalizerRealm, realms[0]))

	held, err := r.releaseUnusedRealms(context.Background(), mc)

	require.NoError(t, err)
	assert.True(t, held, "the workload holds the claim of a realm the spec left")
	assert.True(t, exists(t, r, lease))
}
