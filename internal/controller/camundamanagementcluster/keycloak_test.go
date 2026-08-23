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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/keycloak"
)

var _ = Describe("CamundaManagementCluster controller in the Keycloak modes", func() {
	Context("with a Keycloak that the operator runs", func() {
		It("creates the Keycloak, the generated Secrets, and the contract", func() {
			s := newScenario(withManagedKeycloak)

			var kc keycloak.Keycloak
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, keycloakKey(s), &kc)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Expect(kc.Spec.Image).To(Equal("camunda/keycloak:quay-optimized-26.4.1"))
			Expect(*kc.Spec.Instances).To(Equal(int32(1)))
			Expect(kc.Spec.DB.URL).To(HavePrefix("jdbc:aws-wrapper:postgresql://"))
			Expect(kc.Spec.Hostname.Hostname).To(Equal(keycloakExternalURL))
			Expect(*kc.Spec.Ingress.Enabled).To(BeFalse())
			Expect(kc.Spec.AdditionalOptions).To(ContainElement(
				keycloak.KeycloakValueOrSecret{Name: "http-relative-path", Value: "/auth"},
			))
			Expect(kc.Spec.Unsupported.PodTemplate.Labels).To(
				HaveKeyWithValue("camunda.io/component", components.ComponentKeycloak),
			)

			for _, name := range []string{
				components.IdentityClientSecretName(s.mc),
				components.OptimizeClientSecretName(s.mc),
				components.IdentityAdminSecretName(s.mc),
			} {
				var generated corev1.Secret
				key := client.ObjectKey{Namespace: s.namespace, Name: name}
				Eventually(func(g Gomega) {
					g.Expect(k8sClient.Get(ctx, key, &generated)).To(Succeed())
				}, timeout, interval).Should(Succeed())
				Expect(generated.Data).NotTo(BeEmpty())
			}

			var contract v1.ManagementAuthConfig
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: s.mc.Name}, &contract)).To(Succeed())
			}, timeout, interval).Should(Succeed())
			Expect(contract.Spec.IssuerURL).To(Equal(keycloakExternalURL + "/realms/camunda-platform"))
			Expect(contract.Spec.ClientSecretRef.Name).To(
				Equal(components.OptimizeClientSecretName(s.mc)),
			)
		})

		It("follows the Ready condition of the Keycloak", func() {
			s := newScenario(withManagedKeycloak)

			Eventually(func(g Gomega) {
				condition := conditionOf(g, s.mc, v1.ConditionKeycloakReady)
				g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
			}, timeout, interval).Should(Succeed())

			identity := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
			Eventually(func(g Gomega) {
				stampKeycloakReady(g, keycloakKey(s))
				stampDeploymentReady(g, identity)

				ready := conditionOf(g, s.mc, v1.ConditionReady)
				g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
				g.Expect(ready.Reason).To(Equal(v1.ReasonHealthy))
			}, timeout, interval).Should(Succeed())
		})

		// The claim annotation belongs to the oidc mode. A Keycloak mode
		// carries no claim, so recording one would stamp "=" and read as a
		// claim that Management Identity started with.
		It("records no initial administrator claim", func() {
			s := newScenario(withManagedKeycloak)

			identity := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
			Eventually(func(g Gomega) {
				stampKeycloakReady(g, keycloakKey(s))
				stampDeploymentReady(g, identity)

				g.Expect(conditionOf(g, s.mc, v1.ConditionIdentityReady).Status).To(
					Equal(metav1.ConditionTrue),
				)
			}, timeout, interval).Should(Succeed())

			Consistently(func(g Gomega) {
				var current v1.CamundaManagementCluster
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(s.mc), &current)).To(Succeed())
				g.Expect(current.Annotations).NotTo(HaveKey(components.InitialClaimAnnotation))
			}, "2s", interval).Should(Succeed())
		})

		It("scales the Keycloak to zero when the management cluster is suspended", func() {
			s := newScenario(withManagedKeycloak)

			Eventually(func(g Gomega) {
				var kc keycloak.Keycloak
				g.Expect(k8sClient.Get(ctx, keycloakKey(s), &kc)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				latest := readManagementCluster(g, s.mc)
				latest.Spec.Suspend = true
				g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				var kc keycloak.Keycloak
				g.Expect(k8sClient.Get(ctx, keycloakKey(s), &kc)).To(Succeed())
				g.Expect(kc.Spec.Instances).NotTo(BeNil())
				g.Expect(*kc.Spec.Instances).To(Equal(int32(0)))
			}, timeout, interval).Should(Succeed())
		})

		It("generates a new credential after the Secret is deleted", func() {
			s := newScenario(withManagedKeycloak)

			key := client.ObjectKey{
				Namespace: s.namespace, Name: components.IdentityClientSecretName(s.mc),
			}
			var first corev1.Secret
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, &first)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Expect(k8sClient.Delete(ctx, &first)).To(Succeed())

			Eventually(func(g Gomega) {
				var second corev1.Secret
				g.Expect(k8sClient.Get(ctx, key, &second)).To(Succeed())
				g.Expect(second.UID).NotTo(Equal(first.UID))
				g.Expect(second.Data).NotTo(Equal(first.Data))
			}, timeout, interval).Should(Succeed())
		})
	})

	Context("when the identity provider mode changes", func() {
		It("deletes the Keycloak of a management cluster that moves to an existing one", func() {
			s := newScenario(withManagedKeycloak, func(f *fixture) { f.withKeycloakAdmin = true })

			Eventually(func(g Gomega) {
				var kc keycloak.Keycloak
				g.Expect(k8sClient.Get(ctx, keycloakKey(s), &kc)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				latest := readManagementCluster(g, s.mc)
				latest.Spec.IdentityProvider = v1.IdentityProviderSpec{
					ExternalKeycloak: &v1.ExternalKeycloakSpec{
						URL: keycloakExternalURL,
						AdminCredentialsSecretRef: v1.CredentialsSecretRef{
							Name:        keycloakAdminSecret,
							Namespace:   s.namespace,
							UsernameKey: "username",
							PasswordKey: "password",
						},
					},
				}
				g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				var kc keycloak.Keycloak
				g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, keycloakKey(s), &kc))).To(BeTrue())

				// The component is still built, so the condition stays and
				// says the Keycloak is off rather than going missing.
				condition := conditionOf(g, s.mc, v1.ConditionKeycloakReady)
				g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
				g.Expect(condition.Reason).To(Equal(string(component.Disabled)))
			}, timeout, interval).Should(Succeed())

			identity := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
			expectReadyWhileStamping(s.mc, identity)
		})
	})

	Context("with a Keycloak that the user runs", func() {
		It("reads the administrator Secret and creates no Keycloak", func() {
			s := newScenario(withExternalKeycloak)

			identity := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
			expectReadyWhileStamping(s.mc, identity)

			var kc keycloak.Keycloak
			Expect(apierrors.IsNotFound(k8sClient.Get(ctx, keycloakKey(s), &kc))).To(BeTrue())

			var workload appsv1.Deployment
			Expect(k8sClient.Get(ctx, identity, &workload)).To(Succeed())
			Expect(workload.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
				corev1.EnvVar{
					Name: "KEYCLOAK_SETUP_PASSWORD",
					ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: keycloakAdminSecret},
						Key:                  "password",
					}},
				},
			))
		})

		It("reports a missing administrator Secret", func() {
			s := newScenario(withExternalKeycloak, func(f *fixture) { f.withKeycloakAdmin = false })

			expectReadyReason(s.mc, v1.ReasonMissingSecret)
		})
	})
})

// The Keycloak Operator is an optional prerequisite. Without its CRD the
// keycloak mode cannot converge, and the reason says which operator to
// install rather than reporting a dangling reference.
var _ = Describe("the Keycloak kind probe", func() {
	It("reports the keycloak mode as unsupported while the kind is not served", func() {
		mc := &v1.CamundaManagementCluster{
			Spec: v1.CamundaManagementClusterSpec{
				IdentityProvider: v1.IdentityProviderSpec{
					Keycloak: &v1.ManagedKeycloakSpec{Version: "26.4.1"},
				},
			},
		}

		failure := (&Reconciler{keycloakServed: false}).checkKeycloakOperator(mc)

		Expect(failure).To(HaveOccurred())
		Expect(failure.Reason).To(Equal(v1.ReasonKeycloakOperatorNotInstalled))
		Expect((&Reconciler{keycloakServed: true}).checkKeycloakOperator(mc)).To(Succeed())
	})

	It("reads the kind from the RESTMapper", func() {
		empty := meta.NewDefaultRESTMapper(nil)
		Expect(keycloakKindServed(empty)).To(BeFalse())

		served := meta.NewDefaultRESTMapper([]schema.GroupVersion{keycloak.GroupVersion})
		served.Add(keycloak.GroupVersion.WithKind(keycloakKind), meta.RESTScopeNamespace)
		Expect(keycloakKindServed(served)).To(BeTrue())
	})
})

// keycloakExternalURL is the address that browsers reach the Keycloak of a
// scenario at.
const keycloakExternalURL = "https://keycloak.example.com/auth"

// keycloakKey is the key of the Keycloak custom resource of a scenario.
func keycloakKey(s scenario) client.ObjectKey {
	return client.ObjectKey{Namespace: s.namespace, Name: components.KeycloakName(s.mc)}
}

// withManagedKeycloak turns a scenario into the keycloak mode: the operator
// runs Keycloak on a database of its own.
func withManagedKeycloak(f *fixture) {
	f.keycloakDatabase = true
	f.mc.Spec.IdentityProvider = v1.IdentityProviderSpec{Keycloak: &v1.ManagedKeycloakSpec{
		Version:     "26.4.1",
		ExternalURL: keycloakExternalURL,
	}}
	withKeycloakAdministrator(f)
}

// withExternalKeycloak turns a scenario into the externalKeycloak mode: the
// user runs Keycloak and names the administrator of it.
func withExternalKeycloak(f *fixture) {
	f.withKeycloakAdmin = true
	f.mc.Spec.IdentityProvider = v1.IdentityProviderSpec{
		ExternalKeycloak: &v1.ExternalKeycloakSpec{
			URL: keycloakExternalURL,
			AdminCredentialsSecretRef: v1.CredentialsSecretRef{
				Name:        keycloakAdminSecret,
				Namespace:   f.mc.Namespace,
				UsernameKey: "username",
				PasswordKey: "password",
			},
		},
	}
	withKeycloakAdministrator(f)
}

// withKeycloakAdministrator replaces the initial claim of the oidc mode with
// the first Keycloak user, and declares the Optimize that the realm carries a
// client for.
func withKeycloakAdministrator(f *fixture) {
	f.mc.Spec.Identity.Admin = v1.IdentityAdminSpec{Username: "platform-admin"}
	f.mc.Spec.Optimize = &v1.ManagementOptimizeSpec{ExternalURL: "https://optimize.example.com"}
}

// stampKeycloakReady writes the status that the Keycloak Operator would write
// for a Keycloak that serves requests.
func stampKeycloakReady(g Gomega, key client.ObjectKey) {
	var kc keycloak.Keycloak
	g.Expect(k8sClient.Get(ctx, key, &kc)).To(Succeed())
	kc.Status = keycloak.KeycloakStatus{
		Conditions:         []keycloak.KeycloakCondition{{Type: keycloak.ConditionReady, Status: "True"}},
		Instances:          1,
		ObservedGeneration: kc.Generation,
	}
	g.Expect(k8sClient.Status().Update(ctx, &kc)).To(Succeed())
}
