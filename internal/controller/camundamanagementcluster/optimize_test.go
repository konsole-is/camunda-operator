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
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/keycloakadmin"
)

// The two Optimize addresses of this file. Each one belongs to a
// CamundaOptimize that names the contract of the management plane.
const (
	blueOptimizeURL  = "https://optimize.blue.example.com"
	greenOptimizeURL = "https://optimize.green.example.com"
)

// keycloakCASecret holds the certificate authority of a Keycloak that serves
// https, and caBundleKey is the key it holds it under.
const (
	keycloakCASecret = "keycloak-ca"
	caBundleKey      = "ca.crt"
)

var _ = Describe("CamundaManagementCluster controller and the Optimize instances behind it", func() {
	It("discovers a CamundaOptimize that names its contract", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			rows := readManagementCluster(g, s.mc).Status.Optimize
			g.Expect(rows).To(HaveLen(1))
			g.Expect(rows[0].Namespace).To(Equal(s.namespace))
			g.Expect(rows[0].ExternalURL).To(Equal(blueOptimizeURL))
		}, timeout, interval).Should(Succeed())

		urls := client.ObjectKey{
			Namespace: s.namespace, Name: components.IdentityOptimizeURLsName(s.mc),
		}
		Eventually(func(g Gomega) {
			g.Expect(readConfigMap(g, urls)).To(Equal(blueOptimizeURL))
		}, timeout, interval).Should(Succeed())
	})

	It("withdraws a CamundaOptimize that is deleted", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		optimize := createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			g.Expect(readManagementCluster(g, s.mc).Status.Optimize).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, optimize)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(readManagementCluster(g, s.mc).Status.Optimize).To(BeEmpty())
			g.Expect(identityEnv(g, s)).NotTo(HaveKey("KEYCLOAK_INIT_OPTIMIZE_ROOT_URL"))
		}, timeout, interval).Should(Succeed())
	})

	// The root URLs live in a ConfigMap that the container refers to, so a
	// second Optimize changes that object and leaves the pod template alone.
	// Nothing restarts, and the callback reaches the realm all the same.
	It("adds a second Optimize without rolling Management Identity", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		identity := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
		urls := client.ObjectKey{
			Namespace: s.namespace, Name: components.IdentityOptimizeURLsName(s.mc),
		}

		var before appsv1.Deployment
		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))
			g.Expect(k8sClient.Get(ctx, identity, &before)).To(Succeed())
			g.Expect(readConfigMap(g, urls)).To(Equal(blueOptimizeURL))
		}, timeout, interval).Should(Succeed())

		createOptimize(s.namespace, s.mc.Name, greenOptimizeURL)

		// The rows are ordered by namespace and name, and the names are
		// generated, so the list holds both in whichever order that gives.
		Eventually(func(g Gomega) {
			g.Expect(strings.Split(readConfigMap(g, urls), ",")).To(ConsistOf(
				blueOptimizeURL, greenOptimizeURL,
			))
			g.Expect(keycloak.redirectURIs()).To(ConsistOf(
				blueOptimizeURL+components.OptimizeCallbackPath,
				greenOptimizeURL+components.OptimizeCallbackPath,
			))
		}, timeout, interval).Should(Succeed())

		// The pod template is what Kubernetes rolls on, so it must be the same
		// object it was before the second Optimize arrived.
		var after appsv1.Deployment
		Expect(k8sClient.Get(ctx, identity, &after)).To(Succeed())
		Expect(after.Spec.Template).To(Equal(before.Spec.Template))
		Expect(after.Generation).To(Equal(before.Generation))
	})

	It("adds the missing callback of a second Optimize", func() {
		keycloak := startFakeKeycloak(withOptimizeClient(
			blueOptimizeURL + components.OptimizeCallbackPath,
		))
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)
		createOptimize(s.namespace, s.mc.Name, greenOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(ConsistOf(
				blueOptimizeURL+components.OptimizeCallbackPath,
				greenOptimizeURL+components.OptimizeCallbackPath,
			))

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(v1.ReasonHealthy))
		}, timeout, interval).Should(Succeed())
	})

	// A redirect URI that a person registered by hand does not end in the
	// login path of Optimize, so the operator leaves it where it is.
	It("keeps a redirect URI that this operator does not own", func() {
		keycloak := startFakeKeycloak(withOptimizeClient("https://legacy.example.com/*"))
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(Equal([]string{
				"https://legacy.example.com/*",
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))
		}, timeout, interval).Should(Succeed())
	})

	// The realm is bootstrapped by Management Identity, so the workload has to
	// be up before a missing client is the fault worth reporting. Ready only
	// takes the callback reason once every component is True.
	It("reports OptimizeClientMissing while the realm holds no Optimize client", func() {
		keycloak := startFakeKeycloak(nil)
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(v1.ReasonOptimizeClientMissing))

			ready := conditionOf(g, s.mc, v1.ConditionReady)
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(v1.ReasonOptimizeClientMissing))
		}, timeout, interval).Should(Succeed())
	})

	It("reports NoCallbacks while no Optimize names a URL", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(v1.ReasonNoCallbacks))

			g.Expect(identityEnv(g, s)).NotTo(HaveKey("KEYCLOAK_INIT_OPTIMIZE_ROOT_URL"))
		}, timeout, interval).Should(Succeed())

		// A management plane that has reported an empty realm stops calling
		// Keycloak, so a plane that serves no Optimize costs it nothing. It
		// records no realm either: Management Identity creates the Optimize
		// client from the preset of the Optimize instances the plane serves,
		// and a record that is written and taken away again would keep the
		// status moving for as long as the plane runs.
		at := keycloak.requests()
		Consistently(func(g Gomega) {
			g.Expect(keycloak.requests()).To(Equal(at))
			g.Expect(readManagementCluster(g, s.mc).Status.CallbackRealm).To(BeNil())
		}, "2s", interval).Should(Succeed())
	})

	// A plane that serves no Optimize renders no preset, so Management
	// Identity never creates the client. That absence is the resting state and
	// not a fault to retry.
	It("reports NoCallbacks while the realm holds no Optimize client either", func() {
		keycloak := startFakeKeycloak(nil)
		s := newScenario(withFakeKeycloak(keycloak))

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(v1.ReasonNoCallbacks))
		}, timeout, interval).Should(Succeed())

		at := keycloak.requests()
		Consistently(func(g Gomega) {
			g.Expect(keycloak.requests()).To(Equal(at))
		}, "2s", interval).Should(Succeed())
	})

	It("reports WriteFailed while Keycloak refuses the change to the client", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		keycloak.refuseUpdate = true
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(v1.ReasonWriteFailed))
		}, timeout, interval).Should(Succeed())

		Expect(keycloak.redirectURIs()).To(BeEmpty())
	})

	// A suspended management plane is in its desired state, and the Keycloak
	// it runs is scaled to zero, so the realm is left alone.
	It("leaves the realm alone while the management cluster is suspended", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.Suspend = true
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(string(component.Suspended)))
		}, timeout, interval).Should(Succeed())

		at := keycloak.requests()
		Consistently(func(g Gomega) {
			g.Expect(keycloak.requests()).To(Equal(at))
		}, "2s", interval).Should(Succeed())
	})

	// Nothing else removes the callback of an Optimize that went away. The
	// rendered environment of a management plane that serves none carries no
	// Optimize preset, so Management Identity never rewrites the client.
	It("withdraws the callback of the last Optimize and then rests", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		optimize := createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, optimize)).To(Succeed())

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(BeEmpty())

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Reason).To(Equal(v1.ReasonNoCallbacks))
		}, timeout, interval).Should(Succeed())
	})

	// Management Identity writes the whole client while it starts, and this
	// step writes the same object back, so the two must not overlap.
	It("waits for Management Identity before it touches the client", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(string(component.PrerequisiteNotMet)))
		}, timeout, interval).Should(Succeed())

		Expect(keycloak.redirectURIs()).To(BeEmpty())

		// The component that is still starting is the cause, so Ready names it
		// and not the realm it has not bootstrapped yet.
		Expect(conditionOf(Default, s.mc, v1.ConditionReady).Reason).NotTo(
			Equal(string(component.PrerequisiteNotMet)),
		)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))
		}, timeout, interval).Should(Succeed())
	})

	// A Keycloak that the user runs outlives this resource, so a callback left
	// behind would point at an Optimize that is gone.
	It("withdraws the callbacks when the management cluster is deleted", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))
		}, timeout, interval).Should(Succeed())

		identity := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
		Expect(deleteManagementCluster(s.mc)).To(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(keycloak.redirectURIs()).To(BeEmpty())
		}, timeout, interval).Should(Succeed())

		// The writer is stopped before the realm is tidied, so no pod of it
		// starts afterwards and puts the callbacks back.
		var workload appsv1.Deployment
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, identity, &workload))).To(BeTrue())
	})

	// A realm that the spec no longer names is out of reach of every later
	// reconcile, so the callbacks have to leave it in the pass that finds the
	// change.
	It("moves the callbacks to the Keycloak that the spec now names", func() {
		first := startFakeKeycloak(withOptimizeClient())
		second := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(first))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))
			g.Expect(readManagementCluster(g, s.mc).Status.CallbackRealm).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		retargetKeycloak(s.mc, second.url)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(BeEmpty())
			g.Expect(second.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))

			recorded := readManagementCluster(g, s.mc).Status.CallbackRealm
			g.Expect(recorded).NotTo(BeNil())
			g.Expect(recorded.URL).To(Equal(second.url))

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(v1.ReasonHealthy))
		}, timeout, interval).Should(Succeed())
	})

	// The oidc mode registers nothing, so the realm of the Keycloak mode before
	// it would keep callbacks that no later reconcile can reach.
	It("withdraws the callbacks when the spec changes to the oidc mode", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.IdentityProvider = v1.IdentityProviderSpec{OIDC: &v1.ManagementOIDCSpec{}}
			latest.Spec.Identity.Admin = v1.IdentityAdminSpec{
				ClaimName: "oid", ClaimValue: "admin-oid",
			}
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(BeEmpty())
			g.Expect(readManagementCluster(g, s.mc).Status.CallbackRealm).To(BeNil())

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(string(component.Disabled)))
		}, timeout, interval).Should(Succeed())
	})

	// The new realm waits until the old one is clear. Registering in both would
	// leave the users of the old realm signing in to Optimize as before.
	It("reports the realm it cannot withdraw from and registers nothing new", func() {
		first := startFakeKeycloak(withOptimizeClient())
		second := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(first))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		first.setRefuseUpdate(true)
		retargetKeycloak(s.mc, second.url)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(v1.ReasonWriteFailed))
			g.Expect(condition.Message).To(ContainSubstring(first.url))

			g.Expect(second.redirectURIs()).To(BeEmpty())

			// The whole plane waits, not only this operator's writes: a
			// rendered Management Identity would register in the new realm
			// itself as its pod starts.
			g.Expect(identityEnv(g, s)["KEYCLOAK_URL"]).To(Equal(first.url))
			g.Expect(conditionOf(g, s.mc, v1.ConditionReady).Reason).To(Equal(v1.ReasonWriteFailed))
		}, timeout, interval).Should(Succeed())

		first.setRefuseUpdate(false)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(BeEmpty())
			g.Expect(second.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))
		}, timeout, interval).Should(Succeed())
	})

	// A Keycloak that is gone for good never answers, and the record is then
	// what keeps the new realm waiting. The annotation is the way to let go of
	// it, and the callbacks stay where they are.
	It("lets go of a realm the annotation names and registers in the new one", func() {
		first := startFakeKeycloak(withOptimizeClient())
		second := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(first))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		first.setRefuseUpdate(true)
		retargetKeycloak(s.mc, second.url)

		var recorded v1.KeycloakRealmTarget
		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Message).To(ContainSubstring(components.ForgetCallbackRealmAnnotation))

			latest := readManagementCluster(g, s.mc)
			g.Expect(latest.Status.CallbackRealm).NotTo(BeNil())
			recorded = *latest.Status.CallbackRealm
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			metav1.SetMetaDataAnnotation(
				&latest.ObjectMeta, components.ForgetCallbackRealmAnnotation, components.RealmIdentity(recorded),
			)
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(second.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))

			latest := readManagementCluster(g, s.mc)
			g.Expect(latest.Annotations).NotTo(HaveKey(components.ForgetCallbackRealmAnnotation))
			g.Expect(latest.Status.CallbackRealm).NotTo(BeNil())
			g.Expect(latest.Status.CallbackRealm.URL).To(Equal(second.url))

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Reason).To(Equal(v1.ReasonHealthy))

			// The event is the one trace of the callbacks that stay behind.
			g.Expect(eventReasons(g, s.mc)).To(ContainElement(eventReasonCallbacksLeftBehind))
		}, timeout, interval).Should(Succeed())

		// The old realm keeps what the annotation asked to leave there.
		Expect(first.redirectURIs()).To(HaveLen(1))
	})

	// The realm to leave is in the record, so a spec that cannot be resolved
	// yet still tidies it. Without this, a retarget whose new administrator
	// Secret is not created yet would leave the callbacks where no later
	// reconcile can reach them.
	It("withdraws from the old realm while the Secret of the new one is missing", func() {
		first := startFakeKeycloak(withOptimizeClient())
		second := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(first))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		const missingSecret = "missing-keycloak-admin"
		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.IdentityProvider.ExternalKeycloak.URL = second.url
			latest.Spec.IdentityProvider.ExternalKeycloak.AdminCredentialsSecretRef.Name = missingSecret
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(first.redirectURIs()).To(BeEmpty())
			g.Expect(readManagementCluster(g, s.mc).Status.CallbackRealm).To(BeNil())

			ready := conditionOf(g, s.mc, v1.ConditionReady)
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(v1.ReasonMissingSecret))
		}, timeout, interval).Should(Succeed())

		// The Secret arrives, and the callbacks reach the realm it unlocks.
		createSecret(s.namespace, missingSecret, map[string]string{
			"username": "keycloak-admin", "password": "keycloak-s3cret",
		})

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(second.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))
		}, timeout, interval).Should(Succeed())
	})

	// A new Keycloak that Management Identity cannot reach keeps the rollout
	// from ever finishing. The withdrawal from the old realm must not wait on
	// that: only a pod that starts against the old realm can write it, and
	// there is none.
	It("withdraws from the old realm while the new Identity never becomes ready", func() {
		first := startFakeKeycloak(withOptimizeClient())
		second := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(first))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		retargetKeycloak(s.mc, second.url)

		// Nothing stamps the Deployment after the change, so Management
		// Identity stays mid-rollout for the rest of the spec. The old realm
		// is tidied all the same, the record moves to the realm the plane now
		// points Identity at, and only the registration keeps waiting.
		Eventually(func(g Gomega) {
			g.Expect(first.redirectURIs()).To(BeEmpty())

			recorded := readManagementCluster(g, s.mc).Status.CallbackRealm
			g.Expect(recorded).NotTo(BeNil())
			g.Expect(recorded.URL).To(Equal(second.url))

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Reason).To(Equal(string(component.PrerequisiteNotMet)))
			g.Expect(second.redirectURIs()).To(BeEmpty())
		}, timeout, interval).Should(Succeed())
	})

	// Management Identity writes the login callbacks itself while it starts,
	// before this operator reaches the realm at all. The realm is recorded
	// from the moment the plane points Identity at it, so a retarget during
	// that first start still finds the realm to empty.
	It("empties the first realm after a retarget during the first Identity start", func() {
		// The client holds the callback that Management Identity wrote as it
		// started, and the rollout it is in never finishes.
		first := startFakeKeycloak(withOptimizeClient(
			blueOptimizeURL + components.OptimizeCallbackPath,
		))
		second := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(first))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			recorded := readManagementCluster(g, s.mc).Status.CallbackRealm
			g.Expect(recorded).NotTo(BeNil())
			g.Expect(recorded.URL).To(Equal(first.url))

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Reason).To(Equal(string(component.PrerequisiteNotMet)))
		}, timeout, interval).Should(Succeed())

		retargetKeycloak(s.mc, second.url)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(BeEmpty())
			g.Expect(second.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))

			recorded := readManagementCluster(g, s.mc).Status.CallbackRealm
			g.Expect(recorded).NotTo(BeNil())
			g.Expect(recorded.URL).To(Equal(second.url))
		}, timeout, interval).Should(Succeed())
	})

	// An Identity pod of the revision before the change can still be starting
	// while the spec moves on, and its initializer writes the old realm. The
	// withdrawal waits for exactly that pod, and the record outlives it: even
	// a ready pod can restart and register the old callbacks again.
	It("waits for an Identity pod that is starting against the old realm", func() {
		first := startFakeKeycloak(withOptimizeClient())
		second := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(first))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		identity := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		// The pod is created from the template that still names the first
		// Keycloak, and it is running and not ready: a start in flight.
		pod := startIdentityPod(identity)

		retargetKeycloak(s.mc, second.url)

		Eventually(func(g Gomega) {
			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(string(component.PrerequisiteNotMet)))
			g.Expect(condition.Message).To(ContainSubstring(first.url))
		}, timeout, interval).Should(Succeed())
		// The starting pod can rewrite the old client at any moment, so
		// nothing was written and the plane did not move.
		Expect(first.redirectURIs()).To(HaveLen(1))

		markPodReady(pod)

		// A ready pod is past its start, so the old realm is emptied. It can
		// still restart, so the record stays, the old Management Identity is
		// stopped rather than moved, and the new realm waits: nothing may
		// write it while a pod of the old realm can come back.
		Eventually(func(g Gomega) {
			g.Expect(first.redirectURIs()).To(BeEmpty())

			recorded := readManagementCluster(g, s.mc).Status.CallbackRealm
			g.Expect(recorded).NotTo(BeNil())
			g.Expect(recorded.URL).To(Equal(first.url))
			g.Expect(second.redirectURIs()).To(BeEmpty())

			var workload appsv1.Deployment
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, identity, &workload))).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		// The pod goes, and the record and the registration follow it.
		Expect(k8sClient.Delete(ctx, pod, client.GracePeriodSeconds(0))).To(Succeed())

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(second.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))

			recorded := readManagementCluster(g, s.mc).Status.CallbackRealm
			g.Expect(recorded).NotTo(BeNil())
			g.Expect(recorded.URL).To(Equal(second.url))
		}, timeout, interval).Should(Succeed())
	})

	// The annotation is read on a pass that a failed pre-check stops too. The
	// callbacks stay in the old realm then, so the condition must say that
	// they are still there and not that they left.
	It("says the callbacks stay behind when a failed pre-check reads the annotation", func() {
		first := startFakeKeycloak(withOptimizeClient())
		second := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(first))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		// The new Keycloak names an administrator Secret that is not there,
		// so the pre-check stops every pass, and the old Keycloak refuses to
		// let the callbacks go.
		first.setRefuseUpdate(true)
		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.IdentityProvider.ExternalKeycloak.URL = second.url
			latest.Spec.IdentityProvider.ExternalKeycloak.AdminCredentialsSecretRef.Name = "missing-keycloak-admin"
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		var recorded v1.KeycloakRealmTarget
		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			g.Expect(latest.Status.CallbackRealm).NotTo(BeNil())
			recorded = *latest.Status.CallbackRealm

			g.Expect(conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady).Message).To(
				ContainSubstring(components.ForgetCallbackRealmAnnotation),
			)
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			metav1.SetMetaDataAnnotation(
				&latest.ObjectMeta, components.ForgetCallbackRealmAnnotation, components.RealmIdentity(recorded),
			)
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			g.Expect(latest.Status.CallbackRealm).To(BeNil())
			g.Expect(latest.Annotations).NotTo(HaveKey(components.ForgetCallbackRealmAnnotation))

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Message).To(ContainSubstring("keeps the login callbacks"))
			g.Expect(condition.Message).To(ContainSubstring(first.url))
		}, timeout, interval).Should(Succeed())

		// The old realm keeps what the annotation asked to leave there.
		Expect(first.redirectURIs()).To(HaveLen(1))
	})

	// A Deployment keeps the ReplicaSet of every revision it rolled over, at
	// zero replicas. Such a revision starts no pod, so it must not hold the
	// record: a plane whose withdrawal failed once would never leave the old
	// realm again, however long the old Keycloak answers.
	It("moves on while a retired ReplicaSet of the old realm is kept", func() {
		first := startFakeKeycloak(withOptimizeClient())
		second := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(first))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		identity := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		createIdentityReplicaSet(identity, 0)

		retargetKeycloak(s.mc, second.url)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(BeEmpty())
			g.Expect(second.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))

			recorded := readManagementCluster(g, s.mc).Status.CallbackRealm
			g.Expect(recorded).NotTo(BeNil())
			g.Expect(recorded.URL).To(Equal(second.url))
		}, timeout, interval).Should(Succeed())
	})

	// A ReplicaSet of the old realm that still wants a replica starts one
	// after any list, and that pod writes the old realm from its environment.
	// The record outlives it, and the new realm gets nothing until it is
	// scaled down.
	It("keeps the record while a ReplicaSet of the old realm wants a replica", func() {
		first := startFakeKeycloak(withOptimizeClient())
		second := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(first))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		identity := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		set := createIdentityReplicaSet(identity, 1)

		retargetKeycloak(s.mc, second.url)

		Eventually(func(g Gomega) {
			g.Expect(first.redirectURIs()).To(BeEmpty())

			recorded := readManagementCluster(g, s.mc).Status.CallbackRealm
			g.Expect(recorded).NotTo(BeNil())
			g.Expect(recorded.URL).To(Equal(first.url))
			g.Expect(second.redirectURIs()).To(BeEmpty())
		}, timeout, interval).Should(Succeed())

		scaleReplicaSet(set, 0)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(second.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))

			recorded := readManagementCluster(g, s.mc).Status.CallbackRealm
			g.Expect(recorded).NotTo(BeNil())
			g.Expect(recorded.URL).To(Equal(second.url))
		}, timeout, interval).Should(Succeed())
	})

	// A plane that serves no Optimize fills no realm, so a failing withdrawal
	// holds nothing back: the workloads move, Ready stays with them, and only
	// the condition keeps naming the realm still to be emptied.
	It("keeps a plane that serves no Optimize ready while the old realm refuses", func() {
		first := startFakeKeycloak(withOptimizeClient())
		second := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(first))

		optimize := createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		first.setRefuseUpdate(true)
		Expect(k8sClient.Delete(ctx, optimize)).To(Succeed())
		retargetKeycloak(s.mc, second.url)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Message).To(ContainSubstring(first.url))

			// The workloads moved and the plane is ready without the realm.
			g.Expect(identityEnv(g, s)["KEYCLOAK_URL"]).To(Equal(second.url))
			g.Expect(conditionOf(g, s.mc, v1.ConditionReady).Reason).To(Equal(v1.ReasonHealthy))
		}, timeout, interval).Should(Succeed())

		first.setRefuseUpdate(false)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(BeEmpty())
			g.Expect(readManagementCluster(g, s.mc).Status.CallbackRealm).To(BeNil())
		}, timeout, interval).Should(Succeed())
	})

	// An annotation that names another realm than the recorded one sanctions
	// nothing. It is removed with a Warning event, so a typo does not sit
	// armed until a later retarget happens to match it.
	// The annotation is written by hand, and a Keycloak URL admits a user with
	// a password, so the event that reports the removal prints the folded
	// identity of the value and never the value.
	It("removes a forget annotation that names no recorded realm", func() {
		first := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(first))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(readManagementCluster(g, s.mc).Status.CallbackRealm).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			metav1.SetMetaDataAnnotation(
				&latest.ObjectMeta,
				components.ForgetCallbackRealmAnnotation,
				"https://someone:hunter2@elsewhere.example.com/auth/realms/other",
			)
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			g.Expect(latest.Annotations).NotTo(HaveKey(components.ForgetCallbackRealmAnnotation))
			g.Expect(latest.Status.CallbackRealm).NotTo(BeNil())
			g.Expect(eventReasons(g, s.mc)).To(ContainElement(eventReasonForgetIgnored))
			g.Expect(eventNotes(g, s.mc)).To(ContainElement(And(
				ContainSubstring("https://elsewhere.example.com/auth/realms/other"),
				Not(ContainSubstring("hunter2")),
			)))
		}, timeout, interval).Should(Succeed())

		Expect(first.redirectURIs()).To(HaveLen(1))
	})

	// The hatch leaves the callbacks behind for good, so it answers to the
	// exact value the condition message prints. A value that carries a query
	// is no realm identity, whatever it folds to, and it is removed unused.
	It("refuses a forget annotation that carries a query", func() {
		first := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(first))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		var recorded string
		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			realm := readManagementCluster(g, s.mc).Status.CallbackRealm
			g.Expect(realm).NotTo(BeNil())
			recorded = components.RealmIdentity(*realm)
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			metav1.SetMetaDataAnnotation(
				&latest.ObjectMeta,
				components.ForgetCallbackRealmAnnotation,
				recorded+"?typo=1",
			)
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			g.Expect(latest.Annotations).NotTo(HaveKey(components.ForgetCallbackRealmAnnotation))
			g.Expect(latest.Status.CallbackRealm).NotTo(BeNil())
			g.Expect(eventReasons(g, s.mc)).To(ContainElement(eventReasonForgetIgnored))
		}, timeout, interval).Should(Succeed())

		Expect(first.redirectURIs()).To(HaveLen(1))
	})

	// Suspension holds the realms, not an annotation that sanctions nothing.
	// A typo that waits until the plane resumes is a typo that a later
	// retarget can meet, so it goes while the plane sleeps.
	It("removes a forget annotation of another realm while it is suspended", func() {
		first := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(first))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(readManagementCluster(g, s.mc).Status.CallbackRealm).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.Suspend = true
			metav1.SetMetaDataAnnotation(
				&latest.ObjectMeta,
				components.ForgetCallbackRealmAnnotation,
				"https://elsewhere.example.com/auth/realms/other",
			)
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			g.Expect(latest.Annotations).NotTo(HaveKey(components.ForgetCallbackRealmAnnotation))
			g.Expect(latest.Status.CallbackRealm).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		// The realm of the record is untouched, which is what suspension
		// promises.
		Expect(first.redirectURIs()).To(HaveLen(1))
	})

	// A suspended management plane leaves every realm as it is, so a spec that
	// is retargeted while it sleeps still has the old realm to tidy on
	// deletion.
	It("withdraws from the realm it registered in when it is deleted", func() {
		first := startFakeKeycloak(withOptimizeClient())
		second := startFakeKeycloak(withOptimizeClient(
			greenOptimizeURL + components.OptimizeCallbackPath,
		))
		s := newScenario(withFakeKeycloak(first))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(first.redirectURIs()).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			latest := readManagementCluster(g, s.mc)
			latest.Spec.Suspend = true
			latest.Spec.IdentityProvider.ExternalKeycloak.URL = second.url
			g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady).Reason).To(
				Equal(string(component.Suspended)),
			)

			recorded := readManagementCluster(g, s.mc).Status.CallbackRealm
			g.Expect(recorded).NotTo(BeNil())
			g.Expect(recorded.URL).To(Equal(first.url))
		}, timeout, interval).Should(Succeed())

		Expect(deleteManagementCluster(s.mc)).To(Succeed())

		Expect(first.redirectURIs()).To(BeEmpty())
		Expect(second.redirectURIs()).To(Equal([]string{
			greenOptimizeURL + components.OptimizeCallbackPath,
		}))
	})

	// Two planes can share one external Keycloak realm. A plane parked on a
	// name another owner holds never served those Optimize instances, so its
	// deletion must not take the holder's callbacks with it.
	It("leaves the realm alone when a plane that owns no contract is deleted", func() {
		keycloak := startFakeKeycloak(withOptimizeClient(
			blueOptimizeURL + components.OptimizeCallbackPath,
		))
		name := "mac-" + utilrand.String(8)
		createForeignContract(name, map[string]string{"camunda.io/management-cluster": "elsewhere"})

		s := newScenario(withFakeKeycloak(keycloak), func(f *fixture) {
			f.mc.Spec.ManagementAuthConfigName = name
		})

		Eventually(func(g Gomega) {
			g.Expect(conditionOf(g, s.mc, v1.ConditionReady).Reason).To(Equal(v1.ReasonConflict))
		}, timeout, interval).Should(Succeed())

		Expect(deleteManagementCluster(s.mc)).To(Succeed())

		Expect(keycloak.redirectURIs()).To(Equal([]string{
			blueOptimizeURL + components.OptimizeCallbackPath,
		}))
	})

	// A Deployment whose old pod is still ready satisfies IdentityReady while
	// the new pod runs its initializer against the realm. Only a finished
	// rollout leaves no Management Identity writing to the client.
	It("waits through a rollout that an old ready pod would hide", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		identity := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
		Eventually(func(g Gomega) {
			stampMidRollout(g, identity)

			g.Expect(conditionOf(g, s.mc, v1.ConditionIdentityReady).Status).To(
				Equal(metav1.ConditionTrue),
			)
			g.Expect(conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady).Reason).To(
				Equal(string(component.PrerequisiteNotMet)),
			)

			// No component reports the wait, because the pod of the previous
			// revision satisfies IdentityReady. Ready therefore has to carry
			// it, or it reads Healthy over a callback that nobody can sign in
			// through yet.
			ready := conditionOf(g, s.mc, v1.ConditionReady)
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(string(component.PrerequisiteNotMet)))
		}, timeout, interval).Should(Succeed())

		Expect(keycloak.redirectURIs()).To(BeEmpty())

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))
		}, timeout, interval).Should(Succeed())
	})

	// A plane that never wrote a contract never served an Optimize behind it,
	// so an absent contract is no licence to clear the realm.
	It("leaves the realm alone when a plane with no contract is deleted", func() {
		keycloak := startFakeKeycloak(withOptimizeClient(
			blueOptimizeURL + components.OptimizeCallbackPath,
		))
		s := newScenario(withFakeKeycloak(keycloak), func(f *fixture) {
			// A dangling DatabaseConfig fails the pre-checks, so the reconcile
			// stops before it ever writes the contract.
			f.mc.Spec.Identity.DatabaseConfigRef = "dbc-does-not-exist"
		})

		Eventually(func(g Gomega) {
			g.Expect(conditionOf(g, s.mc, v1.ConditionReady).Reason).To(
				Equal(v1.ReasonInvalidReference),
			)
		}, timeout, interval).Should(Succeed())

		Expect(deleteManagementCluster(s.mc)).To(Succeed())

		Expect(keycloak.redirectURIs()).To(Equal([]string{
			blueOptimizeURL + components.OptimizeCallbackPath,
		}))
	})

	// A plane parked on a name another owner holds does not know which
	// Optimize instances are its own, so it writes nothing to the realm.
	It("writes nothing to the realm while another owner holds the contract", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		name := "mac-" + utilrand.String(8)
		createForeignContract(name, map[string]string{"camunda.io/management-cluster": "elsewhere"})

		s := newScenario(withFakeKeycloak(keycloak), func(f *fixture) {
			f.mc.Spec.ManagementAuthConfigName = name
		})
		createOptimize(s.namespace, name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			g.Expect(conditionOf(g, s.mc, v1.ConditionReady).Reason).To(Equal(v1.ReasonConflict))
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(keycloak.redirectURIs()).To(BeEmpty())
		}, "2s", interval).Should(Succeed())
	})

	// Nothing watches the Optimize client, so an entry removed in Keycloak
	// itself is found by the next converge and by nothing else.
	It("puts a callback back that somebody removed in Keycloak", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))
		}, timeout, interval).Should(Succeed())

		keycloak.setRedirectURIs(nil)

		Eventually(func(g Gomega) {
			g.Expect(keycloak.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))
		}, timeout, interval).Should(Succeed())
	})

	// A withdrawal that Keycloak refuses is reported and tried again, and it
	// never holds Ready back: nobody can sign in to an Optimize that is gone.
	It("retries a refused withdrawal without holding Ready back", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		s := newScenario(withFakeKeycloak(keycloak))

		optimize := createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		keycloak.setRefuseUpdate(true)
		Expect(k8sClient.Delete(ctx, optimize)).To(Succeed())

		// Removing the last Optimize changes the environment of Management
		// Identity, so its pods roll and the step waits for them again.
		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(v1.ReasonWriteFailed))

			g.Expect(conditionOf(g, s.mc, v1.ConditionReady).Reason).NotTo(
				Equal(v1.ReasonWriteFailed),
			)
		}, timeout, interval).Should(Succeed())

		keycloak.setRefuseUpdate(false)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(BeEmpty())
			g.Expect(conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady).Reason).To(
				Equal(v1.ReasonNoCallbacks),
			)
		}, timeout, interval).Should(Succeed())
	})

	// The identity provider of the platform config holds the callback URLs of
	// the oidc mode, so a CamundaOptimize there is none of this resource's
	// business and gets no row.
	It("discovers no Optimize in the oidc mode", func() {
		s := newScenario()

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			g.Expect(conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady).Reason).To(
				Equal(string(component.Disabled)),
			)
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(readManagementCluster(g, s.mc).Status.Optimize).To(BeEmpty())
		}, "2s", interval).Should(Succeed())
	})

	It("reports ConnectionFailed while Keycloak refuses the administrator", func() {
		keycloak := startFakeKeycloak(withOptimizeClient())
		keycloak.refuse = true
		s := newScenario(withFakeKeycloak(keycloak))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(v1.ReasonConnectionFailed))
		}, timeout, interval).Should(Succeed())
	})

	// A Keycloak that you run can serve a certificate of an authority that
	// the operator image does not carry. The bundle of the spec is what makes
	// the handshake succeed.
	It("registers the callbacks at a Keycloak behind a private authority", func() {
		keycloak, bundle := startFakeKeycloakTLS(withOptimizeClient())
		s := newScenario(withFakeKeycloakTLS(keycloak, string(bundle)))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			g.Expect(keycloak.redirectURIs()).To(Equal([]string{
				blueOptimizeURL + components.OptimizeCallbackPath,
			}))

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(v1.ReasonHealthy))
		}, timeout, interval).Should(Succeed())
	})

	It("reports ConnectionFailed while it does not trust the certificate of Keycloak", func() {
		keycloak, _ := startFakeKeycloakTLS(withOptimizeClient())
		s := newScenario(withFakeKeycloakTLS(keycloak, ""))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(v1.ReasonConnectionFailed))
		}, timeout, interval).Should(Succeed())

		Expect(keycloak.redirectURIs()).To(BeEmpty())
	})

	It("reports InvalidCABundle while the Secret holds no certificate", func() {
		keycloak, _ := startFakeKeycloakTLS(withOptimizeClient())
		s := newScenario(withFakeKeycloakTLS(keycloak, "not a certificate"))

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(v1.ReasonInvalidCABundle))
			g.Expect(condition.Message).To(ContainSubstring(caBundleKey))
		}, timeout, interval).Should(Succeed())

		Expect(keycloak.redirectURIs()).To(BeEmpty())
	})

	It("reports MissingSecret while the bundle Secret is absent", func() {
		keycloak, _ := startFakeKeycloakTLS(withOptimizeClient())
		s := newScenario(withFakeKeycloakTLS(keycloak, ""), func(f *fixture) {
			f.mc.Spec.IdentityProvider.ExternalKeycloak.CABundleSecretRef = &v1.LocalSecretKeyRef{
				Name: keycloakCASecret, Key: caBundleKey,
			}
		})

		createOptimize(s.namespace, s.mc.Name, blueOptimizeURL)

		Eventually(func(g Gomega) {
			stampIdentityReady(g, s)

			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(condition.Reason).To(Equal(v1.ReasonMissingSecret))
		}, timeout, interval).Should(Succeed())
	})

	// The identity provider of the platform config holds the callback URLs of
	// the oidc mode, so the operator administers no client there.
	It("reports Disabled in the oidc mode", func() {
		s := newScenario()

		Eventually(func(g Gomega) {
			condition := conditionOf(g, s.mc, v1.ConditionOptimizeCallbacksReady)
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(string(component.Disabled)))
		}, timeout, interval).Should(Succeed())
	})
})

// createOptimize creates a CamundaOptimize at externalURL that names contract,
// and registers its deletion.
func createOptimize(namespace, contract, externalURL string) *v1.CamundaOptimize {
	GinkgoHelper()

	optimize := &v1.CamundaOptimize{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "optimize-" + utilrand.String(8),
			Namespace: namespace,
		},
		Spec: v1.CamundaOptimizeSpec{
			Version:           "8.9.4",
			ManagementAuthRef: contract,
			ExternalURL:       externalURL,
			ClusterRef:        v1.ClusterRef{Name: "my-cluster"},
		},
	}
	Expect(k8sClient.Create(ctx, optimize)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, optimize) })

	return optimize
}

// retargetKeycloak points the spec of a management cluster in the
// externalKeycloak mode at another Keycloak.
func retargetKeycloak(mc *v1.CamundaManagementCluster, url string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		latest := readManagementCluster(g, mc)
		latest.Spec.IdentityProvider.ExternalKeycloak.URL = url
		g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// markPodReady reports the containers of pod as passing their readiness
// probes, the way the kubelet does.
func markPodReady(pod *corev1.Pod) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var latest corev1.Pod
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), &latest)).To(Succeed())
		latest.Status.Conditions = []corev1.PodCondition{{
			Type: corev1.PodReady, Status: corev1.ConditionTrue,
		}}
		g.Expect(k8sClient.Status().Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// createIdentityReplicaSet creates the ReplicaSet that a Deployment controller
// creates for the Management Identity Deployment as it is now, at replicas
// replicas. Envtest runs no Deployment controller, so a spec that needs the
// revision of an earlier template is what creates it.
func createIdentityReplicaSet(key client.ObjectKey, replicas int32) *appsv1.ReplicaSet {
	GinkgoHelper()

	var workload appsv1.Deployment
	Expect(k8sClient.Get(ctx, key, &workload)).To(Succeed())

	set := &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name + "-" + utilrand.String(5),
			Namespace: key.Namespace,
			Labels:    workload.Spec.Template.Labels,
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &replicas,
			Selector: workload.Spec.Selector,
			Template: workload.Spec.Template,
		},
	}
	Expect(k8sClient.Create(ctx, set)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, set) })

	return set
}

// scaleReplicaSet writes the replica count of set.
func scaleReplicaSet(set *appsv1.ReplicaSet, replicas int32) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		var latest appsv1.ReplicaSet
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(set), &latest)).To(Succeed())
		latest.Spec.Replicas = &replicas
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// eventReasons returns the reason of every event that regards mc.
func eventReasons(g Gomega, mc *v1.CamundaManagementCluster) []string {
	var events eventsv1.EventList
	g.Expect(k8sClient.List(ctx, &events, client.InNamespace(mc.Namespace))).To(Succeed())

	reasons := make([]string, 0, len(events.Items))
	for _, event := range events.Items {
		if event.Regarding.Name == mc.Name {
			reasons = append(reasons, event.Reason)
		}
	}

	return reasons
}

// eventNotes returns the message of every event that names mc.
func eventNotes(g Gomega, mc *v1.CamundaManagementCluster) []string {
	var events eventsv1.EventList
	g.Expect(k8sClient.List(ctx, &events, client.InNamespace(mc.Namespace))).To(Succeed())

	notes := make([]string, 0, len(events.Items))
	for _, event := range events.Items {
		if event.Regarding.Name == mc.Name {
			notes = append(notes, event.Note)
		}
	}

	return notes
}

// readConfigMap returns the Optimize root URLs that a ConfigMap holds.
func readConfigMap(g Gomega, key client.ObjectKey) string {
	var urls corev1.ConfigMap
	g.Expect(k8sClient.Get(ctx, key, &urls)).To(Succeed())

	return urls.Data[components.OptimizeRootURLKey]
}

// stampMidRollout reports the Deployment the way Kubernetes does in the middle
// of a rolling update: the old replica is ready, so the ocf handler reads the
// workload as ready, while the updated replica is not there yet.
func stampMidRollout(g Gomega, key client.ObjectKey) {
	var workload appsv1.Deployment
	g.Expect(k8sClient.Get(ctx, key, &workload)).To(Succeed())
	replicas := *workload.Spec.Replicas
	workload.Status.ObservedGeneration = workload.Generation
	workload.Status.Replicas = replicas
	workload.Status.ReadyReplicas = replicas
	workload.Status.AvailableReplicas = replicas
	workload.Status.UpdatedReplicas = 0
	g.Expect(k8sClient.Status().Update(ctx, &workload)).To(Succeed())
}

// stampIdentityReady reports the Management Identity Deployment as ready. The
// callback step waits for it, because Management Identity owns the Optimize
// client while it starts. A re-render moves the generation, so every polling
// loop stamps again.
func stampIdentityReady(g Gomega, s scenario) {
	stampDeploymentReady(
		g, client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)},
	)
}

// identityEnv returns the rendered environment of the Management Identity
// container as a map from name to literal value.
func identityEnv(g Gomega, s scenario) map[string]string {
	var workload appsv1.Deployment
	key := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
	g.Expect(k8sClient.Get(ctx, key, &workload)).To(Succeed())
	g.Expect(workload.Spec.Template.Spec.Containers).NotTo(BeEmpty())

	env := map[string]string{}
	for _, entry := range workload.Spec.Template.Spec.Containers[0].Env {
		env[entry.Name] = entry.Value
	}

	return env
}

// withFakeKeycloak turns a scenario into the externalKeycloak mode against the
// fake Keycloak. The mode carries the administrator credentials in the spec,
// so the base URL of the administration API is the URL of the spec and no test
// hook is needed.
func withFakeKeycloak(keycloak *fakeKeycloak) func(f *fixture) {
	return func(f *fixture) {
		withExternalKeycloak(f)
		f.mc.Spec.IdentityProvider.ExternalKeycloak.URL = keycloak.url
	}
}

// fakeKeycloak is a Keycloak that serves the token endpoint, the Optimize
// client of one realm, and the users and realm roles of that realm. It records
// the redirect URIs and the role mappings that the operator writes.

// withFakeKeycloakTLS turns a scenario into the externalKeycloak mode against
// a fake Keycloak that serves https. bundle is what the Secret of
// caBundleSecretRef holds, and an empty one names no Secret at all, so a spec
// can leave the operator with the trust store of its image alone.
func withFakeKeycloakTLS(keycloak *fakeKeycloak, bundle string) func(f *fixture) {
	return func(f *fixture) {
		withFakeKeycloak(keycloak)(f)
		if bundle == "" {
			return
		}

		createSecret(f.mc.Namespace, keycloakCASecret, map[string]string{caBundleKey: bundle})
		f.mc.Spec.IdentityProvider.ExternalKeycloak.CABundleSecretRef = &v1.LocalSecretKeyRef{
			Name: keycloakCASecret,
			Key:  caBundleKey,
		}
	}
}

// fakeKeycloak is a Keycloak that serves the token endpoint and the Optimize
// client of one realm. It records the redirect URIs that the operator writes.
type fakeKeycloak struct {
	url string
	// refuse answers every administration call with 401, the way a Keycloak
	// that does not know the administrator does.
	refuse bool
	// refuseUpdate answers the client update with 403, the way a Keycloak does
	// for an administrator that cannot change a client of the realm.
	refuseUpdate bool

	mu       sync.Mutex
	stored   keycloakadmin.Representation
	answered int
	// users maps the username of every user of the realm to the internal id
	// that a role mapping is written by.
	users map[string]string
	// realmRoles maps the name of every realm role to its internal id.
	realmRoles map[string]string
	// granted maps the internal id of a user to the realm roles mapped to it
	// directly.
	granted map[string][]keycloakadmin.RealmRole
	// inherited maps the internal id of a user to the realm roles that the
	// user holds through a group. Only the composite read carries them.
	inherited map[string][]keycloakadmin.RealmRole
}

// withOptimizeClient returns the Optimize client that the fake Keycloak starts
// with, carrying the given redirect URIs.
func withOptimizeClient(redirectURIs ...string) keycloakadmin.Representation {
	entries := make([]any, 0, len(redirectURIs))
	for _, uri := range redirectURIs {
		entries = append(entries, uri)
	}

	return keycloakadmin.Representation{
		"id":           "6c4c0c5c",
		"clientId":     "optimize",
		"webOrigins":   []any{"+"},
		"redirectUris": entries,
	}
}

// startFakeKeycloak runs a fake Keycloak that holds stored as its Optimize
// client, or no client at all when stored is nil.
//
// The realm holds the first administrator from the start, the way one that
// Management Identity bootstrapped does. It holds the Optimize role only
// beside an Optimize client, because Management Identity creates the two in
// the same preset, and it runs that preset only for a management plane that
// serves an Optimize.
func startFakeKeycloak(stored keycloakadmin.Representation) *fakeKeycloak {
	GinkgoHelper()

	fake := newFakeKeycloak(stored)
	server := httptest.NewServer(http.HandlerFunc(fake.serve))
	DeferCleanup(server.Close)
	fake.url = server.URL

	return fake
}

// newFakeKeycloak builds the realm of a fake Keycloak: the first administrator
// as a user, and the Optimize role when stored holds the Optimize client, the
// way Management Identity leaves a realm after its optimize preset ran.
func newFakeKeycloak(stored keycloakadmin.Representation) *fakeKeycloak {
	fake := &fakeKeycloak{
		stored:     stored,
		users:      map[string]string{adminUsername: "9f2a"},
		realmRoles: map[string]string{},
		granted:    map[string][]keycloakadmin.RealmRole{},
		inherited:  map[string][]keycloakadmin.RealmRole{},
	}
	if stored != nil {
		fake.realmRoles[optimizeRealmRole] = "1b7c"
	}

	return fake
}

// runOptimizePreset creates the Optimize client and the Optimize role, the way
// Management Identity does when it starts for a management plane that serves
// its first Optimize.
func (f *fakeKeycloak) runOptimizePreset() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.stored = withOptimizeClient()
	f.realmRoles[optimizeRealmRole] = "1b7c"
}

// adminRealmRoles returns the names of the realm roles that are mapped to the
// first administrator directly, which is what this operator writes.
func (f *fakeKeycloak) adminRealmRoles() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	held := f.granted[f.users[adminUsername]]
	names := make([]string, 0, len(held))
	for _, role := range held {
		names = append(names, role.Name)
	}

	return names
}

// putAdminInAnOptimizeGroup gives the first administrator the Optimize role
// through a group, the way an administrator of the realm does for a team. The
// user holds the role, and no realm role is mapped to the user directly.
func (f *fakeKeycloak) putAdminInAnOptimizeGroup() {
	f.mu.Lock()
	defer f.mu.Unlock()

	id := f.users[adminUsername]
	f.inherited[id] = append(f.inherited[id], keycloakadmin.RealmRole{
		ID: f.realmRoles[optimizeRealmRole], Name: optimizeRealmRole,
	})
}

// revokeAdminRealmRoles takes every realm role away from the first
// administrator, the way somebody editing the realm in Keycloak would.
func (f *fakeKeycloak) revokeAdminRealmRoles() {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.granted, f.users[adminUsername])
}

// removeAdminUser takes the first administrator out of the realm.
func (f *fakeKeycloak) removeAdminUser() {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.users, adminUsername)
}

// removeRealmRole takes the realm role out of the realm.
func (f *fakeKeycloak) removeRealmRole(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.realmRoles, name)
}

// startFakeKeycloakTLS runs a fake Keycloak that serves https with a
// certificate of its own. It returns the fake and that certificate as a PEM
// bundle, which is what the certificate authority of a private Keycloak
// reaches the operator as.
func startFakeKeycloakTLS(stored keycloakadmin.Representation) (*fakeKeycloak, []byte) {
	GinkgoHelper()

	fake := newFakeKeycloak(stored)
	server := httptest.NewTLSServer(http.HandlerFunc(fake.serve))
	DeferCleanup(server.Close)
	fake.url = server.URL

	bundle := pem.EncodeToMemory(
		&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw},
	)

	return fake, bundle
}

// redirectURIs returns the redirect URIs of the stored Optimize client.
func (f *fakeKeycloak) redirectURIs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.stored.RedirectURIs()
}

// setRedirectURIs replaces the redirect URIs of the stored Optimize client,
// the way somebody editing the realm in Keycloak would.
func (f *fakeKeycloak) setRedirectURIs(uris []string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.stored.SetRedirectURIs(uris)
}

// setRefuseUpdate turns the refusal of the client update on and off.
func (f *fakeKeycloak) setRefuseUpdate(refuse bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.refuseUpdate = refuse
}

// requests returns how many requests the fake Keycloak has answered.
func (f *fakeKeycloak) requests() int {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.answered
}

func (f *fakeKeycloak) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	f.answered++
	f.mu.Unlock()

	if strings.HasSuffix(r.URL.Path, "/protocol/openid-connect/token") {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"an-access-token"}`))

		return
	}
	if f.refuse {
		w.WriteHeader(http.StatusUnauthorized)

		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	switch {
	case strings.Contains(r.URL.Path, "/role-mappings/realm"):
		f.serveRoleMappings(w, r)

		return
	case strings.Contains(r.URL.Path, "/roles/"):
		f.serveRealmRole(w, r)

		return
	case strings.HasSuffix(r.URL.Path, "/users"):
		f.serveUsers(w, r)

		return
	}

	if r.Method == http.MethodGet {
		found := []keycloakadmin.Representation{}
		if f.stored != nil {
			found = append(found, f.stored)
		}
		_ = json.NewEncoder(w).Encode(found)

		return
	}

	if f.refuseUpdate {
		w.WriteHeader(http.StatusForbidden)

		return
	}

	var update keycloakadmin.Representation
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		w.WriteHeader(http.StatusBadRequest)

		return
	}
	f.stored = update
	w.WriteHeader(http.StatusNoContent)
}

// serveUsers answers the exact lookup of one user of the realm. The caller
// holds the lock.
func (f *fakeKeycloak) serveUsers(w http.ResponseWriter, r *http.Request) {
	found := []map[string]string{}
	if id, ok := f.users[r.URL.Query().Get("username")]; ok {
		found = append(found, map[string]string{"id": id})
	}
	_ = json.NewEncoder(w).Encode(found)
}

// serveRealmRole answers the lookup of one realm role by name, with the 404
// that Keycloak answers for a realm that holds no such role. The caller holds
// the lock.
func (f *fakeKeycloak) serveRealmRole(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Path[strings.LastIndex(r.URL.Path, "/roles/")+len("/roles/"):]

	id, ok := f.realmRoles[name]
	if !ok {
		w.WriteHeader(http.StatusNotFound)

		return
	}
	_ = json.NewEncoder(w).Encode(keycloakadmin.RealmRole{ID: id, Name: name})
}

// serveRoleMappings reads and writes the realm roles of one user. The caller
// holds the lock.
func (f *fakeKeycloak) serveRoleMappings(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimSuffix(r.URL.Path, "/composite")
	path = strings.TrimSuffix(path, "/role-mappings/realm")
	userID := path[strings.LastIndex(path, "/")+1:]

	// The composite read is what a user holds, so it carries the roles of the
	// groups of the user as well. The write below adds a direct mapping only.
	if r.Method == http.MethodGet {
		held := []keycloakadmin.RealmRole{}
		held = append(held, f.granted[userID]...)
		if strings.HasSuffix(r.URL.Path, "/composite") {
			held = append(held, f.inherited[userID]...)
		}
		_ = json.NewEncoder(w).Encode(held)

		return
	}

	var added []keycloakadmin.RealmRole
	if err := json.NewDecoder(r.Body).Decode(&added); err != nil {
		w.WriteHeader(http.StatusBadRequest)

		return
	}
	f.granted[userID] = append(f.granted[userID], added...)
	w.WriteHeader(http.StatusNoContent)
}

// The wait on a starting pod must not outlive the Deployment that makes that
// pod. A Management Identity that never becomes ready, because the Keycloak of
// the recorded realm is gone, would otherwise keep the plane in that realm for
// good, and the wait would name no way out.
func TestWithdrawRetargetedStopsTheIdentityThatFeedsTheWait(t *testing.T) {
	mc := externalKeycloakCluster("https://new.example.com/auth", "new-admin")
	recorded := finalizerRealm
	mc.Status.CallbackRealm = &recorded
	identity := ownedIdentity(mc)
	pointIdentityAtRealm(identity, finalizerRealm)
	pod := startingIdentityPod(mc, finalizerRealm)
	r, deletes := fakeReconciler(t, mc, identity, pod)

	target, err := specRealmTarget(mc)
	require.NoError(t, err)

	failure, consumed, err := r.withdrawRetargeted(context.Background(), mc, target, false)

	require.NoError(t, err)
	assert.False(t, consumed)
	require.NotNil(t, failure)
	assert.Equal(t, string(component.PrerequisiteNotMet), failure.Reason)
	assert.Contains(
		t, failure.Message, components.ForgetCallbackRealmAnnotation,
		"the wait names the way out of a Keycloak that is gone for good",
	)
	assert.Equal(t, []string{identity.Name}, deletes.names)
	assert.False(t, exists(t, r, identity))
	assert.NotNil(
		t, mc.Status.CallbackRealm, "the record stays while a pod can still write the realm",
	)
}

// A Deployment at the derived Management Identity name starts a pod against
// the realm its template names whoever owns it, so it holds the record of
// that realm. Only the delete asks who owns it.
func TestStopOldIdentityWritersReadsEveryDeploymentAtTheName(t *testing.T) {
	t.Run("a Deployment of another owner holds the record and stays", func(t *testing.T) {
		mc := finalizingCluster()
		identity := ownedIdentity(mc)
		identity.OwnerReferences = nil
		pointIdentityAtRealm(identity, finalizerRealm)
		r, deletes := fakeReconciler(t, mc, identity)

		writers, err := r.stopOldIdentityWriters(context.Background(), mc, finalizerRealm)

		require.NoError(t, err)
		assert.True(t, writers)
		assert.Empty(t, deletes.names)
		assert.True(t, exists(t, r, identity))
	})

	t.Run("a Deployment of this plane holds the record and goes", func(t *testing.T) {
		mc := finalizingCluster()
		identity := ownedIdentity(mc)
		pointIdentityAtRealm(identity, finalizerRealm)
		r, deletes := fakeReconciler(t, mc, identity)

		writers, err := r.stopOldIdentityWriters(context.Background(), mc, finalizerRealm)

		require.NoError(t, err)
		assert.True(t, writers)
		assert.Equal(t, []string{identity.Name}, deletes.names)
	})

	// A Deployment that points somewhere else writes another realm, and the
	// record of this one is free to go.
	t.Run("a Deployment of another realm holds nothing", func(t *testing.T) {
		mc := finalizingCluster()
		identity := ownedIdentity(mc)
		identity.OwnerReferences = nil
		pointIdentityAtRealm(identity, v1.KeycloakRealmTarget{
			URL: "https://other.example.com/auth", Realm: "camunda-platform",
		})
		r, _ := fakeReconciler(t, mc, identity)

		writers, err := r.stopOldIdentityWriters(context.Background(), mc, finalizerRealm)

		require.NoError(t, err)
		assert.False(t, writers)
	})
}

// TestDropSpentForgetAnnotationLeavesAnEvent covers the removal of a
// ForgetCallbackRealmAnnotation that lets go of nothing. Somebody set it by
// hand, so it never vanishes without a word.
func TestDropSpentForgetAnnotationLeavesAnEvent(t *testing.T) {
	// The pass after the annotation was consumed finds the record gone. The
	// realm went on the pass before, so the event is a Normal one.
	t.Run("a plane that records no realm", func(t *testing.T) {
		mc := externalKeycloakCluster("https://kc.example.com/auth", "keycloak-admin")
		mc.Annotations = map[string]string{
			components.ForgetCallbackRealmAnnotation: "https://old.example.com/auth/realms/other",
		}
		r, _ := fakeReconciler(t, mc)
		recorder := readableEvents(r)

		require.NoError(t, r.dropSpentForgetAnnotation(context.Background(), mc))

		assert.NotContains(t, mc.Annotations, components.ForgetCallbackRealmAnnotation)
		require.Len(t, recorder.Events, 1)
		event := <-recorder.Events
		assert.Contains(t, event, eventReasonForgetRemoved)
		assert.Contains(t, event, "https://old.example.com/auth/realms/other")
	})

	// An annotation that names another realm than the recorded one asked for
	// something and got nothing, so it is a Warning.
	t.Run("a plane that records another realm", func(t *testing.T) {
		mc := externalKeycloakCluster("https://kc.example.com/auth", "keycloak-admin")
		recorded := finalizerRealm
		mc.Status.CallbackRealm = &recorded
		mc.Annotations = map[string]string{
			components.ForgetCallbackRealmAnnotation: "https://old.example.com/auth/realms/other",
		}
		r, _ := fakeReconciler(t, mc)
		recorder := readableEvents(r)

		require.NoError(t, r.dropSpentForgetAnnotation(context.Background(), mc))

		assert.NotContains(t, mc.Annotations, components.ForgetCallbackRealmAnnotation)
		require.Len(t, recorder.Events, 1)
		assert.Contains(t, <-recorder.Events, eventReasonForgetIgnored)
	})
}
