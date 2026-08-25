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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

var _ = Describe("CamundaManagementCluster controller", func() {
	Context("with an external OIDC provider", func() {
		It("deploys Management Identity and writes the contract", func() {
			s := newScenario()

			key := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
			Eventually(func(g Gomega) {
				var workload appsv1.Deployment
				g.Expect(k8sClient.Get(ctx, key, &workload)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			expectReadyWhileStamping(s.mc, key)

			var contract v1.ManagementAuthConfig
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: s.mc.Name}, &contract)).To(Succeed())
			Expect(contract.Spec.BaseURL).To(Equal(identityExternalURL))
			Expect(contract.Spec.IssuerURL).To(Equal(issuerURL))
			Expect(contract.Spec.ClientID).To(Equal("optimize"))
			Expect(contract.Spec.Audience).To(Equal("optimize-api"))
			Expect(contract.Spec.ClientSecretRef.Namespace).To(Equal(s.namespace))
			Expect(contract.Labels).To(HaveKeyWithValue(labels.ManagementClusterKey, s.mc.Name))
			Expect(contract.Labels).To(HaveKeyWithValue(labels.ManagementClusterNamespaceKey, s.namespace))
		})

		It("records the administrator claim that the Identity pod started with", func() {
			s := newScenario()

			key := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
			Eventually(func(g Gomega) {
				var workload appsv1.Deployment
				g.Expect(k8sClient.Get(ctx, key, &workload)).To(Succeed())
			}, timeout, interval).Should(Succeed())
			startIdentityPod(key)

			expectReadyWhileStamping(s.mc, key)

			Eventually(func(g Gomega) {
				g.Expect(readManagementCluster(g, s.mc).Annotations).To(
					HaveKeyWithValue(components.InitialClaimAnnotation, "oid=admin-oid"),
				)
			}, timeout, interval).Should(Succeed())
		})

		// A management plane whose Identity never ran holds no administrator
		// in its database. A wrong claim that the user corrects before the
		// first start must leave no record behind.
		It("records no administrator claim while no Identity pod has started", func() {
			s := newScenario()

			key := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
			expectReadyWhileStamping(s.mc, key)

			Consistently(func(g Gomega) {
				g.Expect(readManagementCluster(g, s.mc).Annotations).NotTo(
					HaveKey(components.InitialClaimAnnotation),
				)
			}, "2s", interval).Should(Succeed())
		})

		// Management Identity reads the administrator claim as it boots and
		// stores it in its database, which is before the pod is ready. A
		// change in that window records the claim the pod started with, not
		// the one the spec now asks for.
		It("reports ImmutableAfterStart when the administrator claim changes after the start", func() {
			s := newScenario()

			key := client.ObjectKey{Namespace: s.namespace, Name: components.IdentityName(s.mc)}
			Eventually(func(g Gomega) {
				var workload appsv1.Deployment
				g.Expect(k8sClient.Get(ctx, key, &workload)).To(Succeed())
			}, timeout, interval).Should(Succeed())
			startIdentityPod(key)

			Eventually(func(g Gomega) {
				latest := readManagementCluster(g, s.mc)
				latest.Spec.Identity.Admin.ClaimValue = "second-admin"
				g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				g.Expect(readManagementCluster(g, s.mc).Annotations).To(
					HaveKeyWithValue(components.InitialClaimAnnotation, "oid=admin-oid"),
				)

				identity := conditionOf(g, s.mc, v1.ConditionIdentityReady)
				g.Expect(identity.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(identity.Reason).To(Equal(v1.ReasonImmutableAfterStart))

				ready := conditionOf(g, s.mc, v1.ConditionReady)
				g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(ready.Reason).To(Equal(v1.ReasonImmutableAfterStart))
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				var workload appsv1.Deployment
				g.Expect(k8sClient.Get(ctx, key, &workload)).To(Succeed())
				g.Expect(workload.Spec.Template.Spec.Containers[0].Env).To(ContainElement(
					corev1.EnvVar{Name: "IDENTITY_INITIAL_CLAIM_VALUE", Value: "admin-oid"},
				))
			}, timeout, interval).Should(Succeed())
		})

		It("refuses a platform config that authenticates with basic", func() {
			s := newScenario(func(f *fixture) {
				f.platform.Spec.Auth = &v1.PlatformAuthSpec{Method: v1.AuthenticationMethodBasic}
			})

			expectReadyReason(s.mc, v1.ReasonInvalidReference)
		})

		It("refuses a platform config that declares no Identity client", func() {
			s := newScenario(func(f *fixture) {
				f.platform.Spec.Auth.OIDC.Management.Clients.Identity = nil
			})

			expectReadyReason(s.mc, v1.ReasonInvalidReference)
		})

		It("reports a missing client Secret", func() {
			s := newScenario(func(f *fixture) { f.withClientSecret = false })

			expectReadyReason(s.mc, v1.ReasonMissingSecret)
		})

		It("refuses a version below the floor", func() {
			s := newScenario(func(f *fixture) { f.mc.Spec.Identity.Version = "8.8.0" })

			expectReadyReason(s.mc, v1.ReasonUnsupportedVersion)
		})

		It("leaves a contract that another owner holds alone", func() {
			name := "mac-" + utilrand.String(8)
			createForeignContract(name, map[string]string{
				labels.ManagementClusterKey: "another-management",
			})

			s := newScenario(func(f *fixture) { f.mc.Spec.ManagementAuthConfigName = name })

			expectReadyReason(s.mc, v1.ReasonConflict)

			var unchanged v1.ManagementAuthConfig
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name}, &unchanged)).To(Succeed())
			Expect(unchanged.Spec.ClientID).To(Equal("elsewhere"))
		})

		// Two management clusters of the same name in two namespaces ask for
		// the same contract name, so the owner name alone does not tell them
		// apart.
		It("refuses a contract that a management cluster of another namespace holds", func() {
			name := "cmc-" + utilrand.String(8)
			createForeignContract(name, map[string]string{
				labels.ManagementClusterKey:          name,
				labels.ManagementClusterNamespaceKey: "another-namespace",
			})

			s := newScenario(func(f *fixture) { f.mc.Name = name })

			Eventually(func(g Gomega) {
				ready := conditionOf(g, s.mc, v1.ConditionReady)
				g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
				g.Expect(ready.Reason).To(Equal(v1.ReasonConflict))
				g.Expect(ready.Message).To(ContainSubstring("another-namespace/" + name))
			}, timeout, interval).Should(Succeed())

			var unchanged v1.ManagementAuthConfig
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name}, &unchanged)).To(Succeed())
			Expect(unchanged.Spec.ClientID).To(Equal("elsewhere"))
		})

		It("reports WriteFailed when the API server refuses the contract", func() {
			s := newScenario(func(f *fixture) { f.mc.Spec.ManagementAuthConfigName = "Not A Name" })

			expectReadyReason(s.mc, v1.ReasonWriteFailed)
		})

		It("leaves a contract that another owner created in the meantime alone", func() {
			// Two management clusters that ask for one free name at the same
			// time both pass the pre-check. The API server creates the
			// contract for one of them only; the other must not take it over.
			s := newScenario()
			name := s.mc.Name + "-raced"
			var written v1.ManagementAuthConfig
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: s.mc.Name}, &written)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			other := &v1.ManagementAuthConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name: name,
					Labels: map[string]string{
						labels.ManagementClusterKey:          "other-management",
						labels.ManagementClusterNamespaceKey: "other-ns",
					},
				},
				Spec: *written.Spec.DeepCopy(),
			}
			other.Spec.ClientID = "other"
			Expect(k8sClient.Create(ctx, other)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, other) })

			r := &Reconciler{Client: k8sClient, APIReader: k8sClient, Scheme: k8sClient.Scheme()}
			racer := s.mc.DeepCopy()
			racer.Spec.ManagementAuthConfigName = name
			err := r.applyContract(ctx, racer, written.Spec)
			Expect(err).To(MatchError(ContainSubstring("belongs to")))

			var kept v1.ManagementAuthConfig
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: name}, &kept)).To(Succeed())
			Expect(kept.Spec.ClientID).To(Equal("other"))
			Expect(kept.Labels).To(HaveKeyWithValue(labels.ManagementClusterKey, "other-management"))
		})

		It("withdraws the old contract when the spec renames it", func() {
			s := newScenario()

			oldName := client.ObjectKey{Name: s.mc.Name}
			Eventually(func(g Gomega) {
				var written v1.ManagementAuthConfig
				g.Expect(k8sClient.Get(ctx, oldName, &written)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			renamed := s.mc.Name + "-renamed"
			Eventually(func(g Gomega) {
				latest := readManagementCluster(g, s.mc)
				latest.Spec.ManagementAuthConfigName = renamed
				g.Expect(k8sClient.Update(ctx, latest)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Eventually(func(g Gomega) {
				var written v1.ManagementAuthConfig
				g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: renamed}, &written)).To(Succeed())
				g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, oldName, &written))).To(BeTrue())
				g.Expect(readManagementCluster(g, s.mc).Status.ManagementAuthConfig).To(Equal(renamed))
			}, timeout, interval).Should(Succeed())
		})

		It("removes the contract with the management cluster", func() {
			s := newScenario()

			contract := client.ObjectKey{Name: s.mc.Name}
			Eventually(func(g Gomega) {
				var written v1.ManagementAuthConfig
				g.Expect(k8sClient.Get(ctx, contract, &written)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			Expect(k8sClient.Delete(ctx, s.mc)).To(Succeed())

			Eventually(func(g Gomega) {
				var written v1.ManagementAuthConfig
				g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, contract, &written))).To(BeTrue())
			}, timeout, interval).Should(Succeed())
		})
	})
})

// The identities that every scenario shares.
const (
	issuerURL = "https://login.example.com"
	// otherIssuerURL is an identity provider that the management plane does
	// not sign anybody in to.
	otherIssuerURL      = "https://login.elsewhere.example.com"
	identityExternalURL = "https://identity.example.com"
	// keycloakAdminSecret holds the administrator of a Keycloak that the user
	// runs.
	keycloakAdminSecret = "keycloak-admin"
)

// fixture is the set of objects that a scenario creates, before they reach the
// API server. A spec changes what it needs through the mutator of newScenario.
type fixture struct {
	platform *v1.CamundaPlatformConfig
	mc       *v1.CamundaManagementCluster
	// withClientSecret creates the Secret that the platform config names.
	// A spec turns it off to exercise MissingSecret.
	withClientSecret bool
	// keycloakDatabase creates a second DatabaseConfig and points
	// spec.identityProvider.keycloak at it.
	keycloakDatabase bool
	// withKeycloakAdmin creates the Secret with the administrator of a
	// Keycloak that the user runs. A spec turns it off to exercise
	// MissingSecret.
	withKeycloakAdmin bool
}

// scenario is a created fixture: the namespace it lives in and the management
// cluster under test.
type scenario struct {
	namespace string
	mc        *v1.CamundaManagementCluster
}

// newScenario creates a namespace, a database, a platform config with an
// external OIDC provider, and a management cluster in the oidc mode, with
// mutate applied to the objects before they are created.
func newScenario(mutate ...func(f *fixture)) scenario {
	GinkgoHelper()

	namespace := newNamespace()
	database := createDatabase(namespace)

	f := &fixture{
		platform:         newPlatformConfig(namespace),
		mc:               newManagementCluster(namespace, database),
		withClientSecret: true,
	}
	for _, m := range mutate {
		m(f)
	}
	f.mc.Spec.PlatformConfigRef = f.platform.Name
	if f.keycloakDatabase {
		f.mc.Spec.IdentityProvider.Keycloak.DatabaseConfigRef = createDatabase(namespace)
	}
	if f.withKeycloakAdmin {
		createSecret(namespace, keycloakAdminSecret, map[string]string{
			"username": "keycloak-admin", "password": "keycloak-s3cret",
		})
	}

	Expect(k8sClient.Create(ctx, f.platform)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, f.platform) })
	if f.withClientSecret {
		createSecret(namespace, "oidc-credentials", map[string]string{
			"identity-client-secret": "identity-s3cret",
			"optimize-client-secret": "optimize-s3cret",
		})
	}

	Expect(k8sClient.Create(ctx, f.mc)).To(Succeed())
	DeferCleanup(func() { _ = deleteManagementCluster(f.mc) })

	return scenario{namespace: namespace, mc: f.mc}
}

// newPlatformConfig returns a platform config with an external OIDC provider
// and the two clients that the management plane always needs. Their Secret
// lives in namespace.
func newPlatformConfig(namespace string) *v1.CamundaPlatformConfig {
	secretRef := func(key string) v1.SecretKeyRef {
		return v1.SecretKeyRef{Name: "oidc-credentials", Namespace: namespace, Key: key}
	}

	return &v1.CamundaPlatformConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cpc-" + utilrand.String(8)},
		Spec: v1.CamundaPlatformConfigSpec{Auth: &v1.PlatformAuthSpec{
			Method: v1.AuthenticationMethodOIDC,
			OIDC: &v1.OIDCSpec{
				IssuerURL:       issuerURL,
				AuthURL:         issuerURL + "/oauth/authorize",
				TokenURL:        issuerURL + "/oauth/token",
				JWKSURL:         issuerURL + "/.well-known/jwks.json",
				ClientID:        "camunda",
				ClientSecretRef: secretRef("identity-client-secret"),
				Management: &v1.ManagementOIDCClientsSpec{Clients: v1.ManagementClients{
					Identity: &v1.ConfidentialClientSpec{
						ClientID:        "management-identity",
						ClientSecretRef: secretRef("identity-client-secret"),
					},
					Optimize: &v1.ConfidentialClientSpec{
						ClientID:        "optimize",
						Audience:        "optimize-api",
						ClientSecretRef: secretRef("optimize-client-secret"),
					},
				}},
			},
		}},
	}
}

// newManagementCluster returns a management cluster in the oidc mode that
// stores Management Identity in the given DatabaseConfig.
func newManagementCluster(namespace, database string) *v1.CamundaManagementCluster {
	return &v1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cmc-" + utilrand.String(8), Namespace: namespace},
		Spec: v1.CamundaManagementClusterSpec{
			IdentityProvider: v1.IdentityProviderSpec{OIDC: &v1.ManagementOIDCSpec{}},
			Identity: v1.IdentitySpec{
				Version:           "8.9.4",
				ExternalURL:       identityExternalURL,
				DatabaseConfigRef: database,
				Admin:             v1.IdentityAdminSpec{ClaimName: "oid", ClaimValue: "admin-oid"},
			},
		},
	}
}

// createDatabase creates a DatabaseServerConfig, a DatabaseConfig of
// namespace, and the credentials Secret they name, and returns the name of the
// DatabaseConfig. The Secret is named after the DatabaseConfig, so a scenario
// that needs a second database in one namespace gets a second Secret.
func createDatabase(namespace string) string {
	GinkgoHelper()

	credentials := "db-credentials-" + utilrand.String(8)

	server := &v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsc-" + utilrand.String(8), Namespace: namespace},
		Spec: v1.DatabaseServerConfigSpec{
			Engine: v1.DatabaseEnginePostgres,
			Host:   "postgres." + namespace + ".svc",
			Port:   5432,
			AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
				Name: credentials, UsernameKey: "username", PasswordKey: "password",
			},
		},
	}
	Expect(k8sClient.Create(ctx, server)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, server) })

	database := &v1.DatabaseConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbc-" + utilrand.String(8), Namespace: namespace},
		Spec: v1.DatabaseConfigSpec{
			ServerRef:    server.Name,
			DatabaseName: "identity",
			CredentialsSecretRef: v1.CredentialsSecretRef{
				Name: credentials, Namespace: namespace,
				UsernameKey: "username", PasswordKey: "password",
			},
		},
	}
	Expect(k8sClient.Create(ctx, database)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, database) })
	createSecret(namespace, credentials, map[string]string{
		"username": "identity", "password": "db-s3cret",
	})

	return database.Name
}

// newNamespace creates a uniquely named Namespace and registers its deletion.
func newNamespace() string {
	GinkgoHelper()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "cmc-ns-" + utilrand.String(8)}}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })

	return ns.Name
}

// createSecret creates a Secret with the given string data and registers its
// deletion.
func createSecret(namespace, name string, data map[string]string) {
	GinkgoHelper()

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		StringData: data,
	}
	Expect(k8sClient.Create(ctx, secret)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })
}

// createForeignContract creates a ManagementAuthConfig that carries the given
// owner labels and registers its deletion. Its client tells it apart from the
// one a management cluster writes.
func createForeignContract(name string, ownerLabels map[string]string) {
	GinkgoHelper()

	contract := &v1.ManagementAuthConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: ownerLabels},
		Spec: v1.ManagementAuthConfigSpec{
			BaseURL:         "https://elsewhere.example.com",
			IssuerURL:       issuerURL,
			AuthURL:         issuerURL + "/oauth/authorize",
			TokenURL:        issuerURL + "/oauth/token",
			JwksURL:         issuerURL + "/.well-known/jwks.json",
			ClientID:        "elsewhere",
			Audience:        "elsewhere",
			ClientSecretRef: v1.SecretKeyRef{Name: "s", Namespace: "s", Key: "s"},
		},
	}
	Expect(k8sClient.Create(ctx, contract)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, contract) })
}

// deleteManagementCluster deletes a management cluster and waits for its
// finalizer to release it. A namespace that is deleted with a finalized object
// in it never terminates, and the next spec would then reuse a stale claim.
func deleteManagementCluster(mc *v1.CamundaManagementCluster) error {
	if err := k8sClient.Delete(ctx, mc); err != nil {
		return client.IgnoreNotFound(err)
	}

	Eventually(func(g Gomega) {
		var latest v1.CamundaManagementCluster
		err := k8sClient.Get(ctx, client.ObjectKeyFromObject(mc), &latest)
		g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
	}, timeout, interval).Should(Succeed())

	return nil
}

// readManagementCluster reads the management cluster as the API server holds it now.
func readManagementCluster(g Gomega, mc *v1.CamundaManagementCluster) *v1.CamundaManagementCluster {
	var latest v1.CamundaManagementCluster
	g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(mc), &latest)).To(Succeed())

	return &latest
}

// conditionOf reads one condition of the management cluster and asserts that
// it is reported.
func conditionOf(g Gomega, mc *v1.CamundaManagementCluster, conditionType string) *metav1.Condition {
	condition := meta.FindStatusCondition(readManagementCluster(g, mc).Status.Conditions, conditionType)
	g.Expect(condition).NotTo(BeNil())

	return condition
}

// expectReadyReason polls until the management cluster reports Ready=False
// with the given reason.
func expectReadyReason(mc *v1.CamundaManagementCluster, reason string) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		ready := conditionOf(g, mc, v1.ConditionReady)
		g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
		g.Expect(ready.Reason).To(Equal(reason))
	}, timeout, interval).Should(Succeed())
}

// expectReadyWhileStamping polls until the management cluster reports
// Ready=True/Healthy, stamping every Deployment again on each attempt. A stamp
// names the generation it saw, so a re-render after a single stamp leaves that
// stamp stale and the components never read the Deployments as up to date.
// Only a real controller keeps up with a rolling generation, and envtest runs
// none.
func expectReadyWhileStamping(mc *v1.CamundaManagementCluster, keys ...client.ObjectKey) {
	GinkgoHelper()

	Eventually(func(g Gomega) {
		for _, key := range keys {
			stampDeploymentReady(g, key)
		}

		ready := conditionOf(g, mc, v1.ConditionReady)
		g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		g.Expect(ready.Reason).To(Equal(v1.ReasonHealthy))
	}, timeout, interval).Should(Succeed())
}

// startIdentityPod creates the pod that the Management Identity Deployment
// describes and reports its container as running. Envtest runs no Deployment
// controller and no kubelet, so a spec that needs a started Identity is what
// puts the pod behind the Deployment.
func startIdentityPod(key client.ObjectKey) {
	GinkgoHelper()

	var workload appsv1.Deployment
	Expect(k8sClient.Get(ctx, key, &workload)).To(Succeed())

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      key.Name + "-" + utilrand.String(5),
			Namespace: key.Namespace,
			Labels:    workload.Spec.Template.Labels,
		},
		Spec: workload.Spec.Template.Spec,
	}
	Expect(k8sClient.Create(ctx, pod)).To(Succeed())
	// A pod that no kubelet removes stays terminating for its whole grace
	// period, and the next list would still read it.
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, pod, client.GracePeriodSeconds(0)) })

	pod.Status.Phase = corev1.PodRunning
	pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name:  pod.Spec.Containers[0].Name,
		State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()}},
	}}
	Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
}

// stampDeploymentReady writes the status that a running Deployment controller
// would write: every replica up to date and available at the current
// generation.
func stampDeploymentReady(g Gomega, key client.ObjectKey) {
	var workload appsv1.Deployment
	g.Expect(k8sClient.Get(ctx, key, &workload)).To(Succeed())
	replicas := *workload.Spec.Replicas
	workload.Status.ObservedGeneration = workload.Generation
	workload.Status.Replicas = replicas
	workload.Status.ReadyReplicas = replicas
	workload.Status.UpdatedReplicas = replicas
	workload.Status.AvailableReplicas = replicas
	g.Expect(k8sClient.Status().Update(ctx, &workload)).To(Succeed())
}
