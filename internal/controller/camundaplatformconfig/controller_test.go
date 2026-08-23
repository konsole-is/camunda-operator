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

package camundaplatformconfig

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
)

// createPlatformConfig creates cfg and registers its deletion.
func createPlatformConfig(cfg *v1.CamundaPlatformConfig) {
	GinkgoHelper()
	Expect(k8sClient.Create(ctx, cfg)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, cfg) })
}

// createSecret creates a Secret with the given data in the schema test
// namespace and registers its deletion.
func createSecret(name string, data map[string]string) *corev1.Secret {
	GinkgoHelper()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: fixtures.SchemaTestNamespace},
		StringData: data,
	}
	Expect(k8sClient.Create(ctx, secret)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })
	return secret
}

// secretRef returns a reference into a uniquely named Secret in the schema
// test namespace.
func secretRef(prefix, key string) v1.SecretKeyRef {
	return v1.SecretKeyRef{
		Name: prefix + "-" + utilrand.String(8), Namespace: fixtures.SchemaTestNamespace, Key: key,
	}
}

// expectReady polls until the Ready condition of cfg matches the given status
// and reason, its message matches the matcher, and the observed generation is
// current.
func expectReady(
	cfg *v1.CamundaPlatformConfig,
	status metav1.ConditionStatus,
	reason string,
	message OmegaMatcher,
) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaPlatformConfig
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cfg), &latest)).To(Succeed())
		ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Status).To(Equal(status))
		g.Expect(ready.Reason).To(Equal(reason))
		g.Expect(ready.Message).To(message)
		g.Expect(ready.ObservedGeneration).To(Equal(latest.Generation))
		g.Expect(latest.Status.ObservedGeneration).To(Equal(latest.Generation))
	}, timeout, interval).Should(Succeed())
}

// notFoundMessage is the condition message for a Secret that does not exist.
func notFoundMessage(path string, ref v1.SecretKeyRef) string {
	return fmt.Sprintf("%s: Secret \"%s/%s\" not found", path, ref.Namespace, ref.Name)
}

// missingKeyMessage is the condition message for a Secret without the key.
func missingKeyMessage(path string, ref v1.SecretKeyRef) string {
	return fmt.Sprintf("%s: Secret \"%s/%s\" is missing key %q", path, ref.Namespace, ref.Name, ref.Key)
}

var _ = Describe("CamundaPlatformConfig controller", func() {
	It("reports Healthy for a basic config without secrets", func() {
		cfg := minimalPlatformConfig()
		createPlatformConfig(cfg)

		expectReady(cfg, metav1.ConditionTrue, v1.ReasonHealthy, Equal("All checks passed"))
	})

	It("reports Healthy for an empty spec", func() {
		cfg := minimalPlatformConfig()
		cfg.Spec = v1.CamundaPlatformConfigSpec{}
		createPlatformConfig(cfg)

		expectReady(cfg, metav1.ConditionTrue, v1.ReasonHealthy, Equal("All checks passed"))
	})

	It("reports MissingSecret naming the license reference, then Healthy once the Secret exists", func() {
		cfg := minimalPlatformConfig()
		ref := secretRef("lic", "license-key")
		cfg.Spec.LicenseSecretRef = &ref
		createPlatformConfig(cfg)

		expectReady(cfg, metav1.ConditionFalse, v1.ReasonMissingSecret, Equal(notFoundMessage("spec.licenseSecretRef", ref)))

		createSecret(ref.Name, map[string]string{ref.Key: "x"})

		expectReady(cfg, metav1.ConditionTrue, v1.ReasonHealthy, Equal("All checks passed"))
	})

	It("reports MissingSecret when the oidc client secret key is missing, then Healthy once the key exists", func() {
		cfg := realisticPlatformConfig()
		cfg.Spec.LicenseSecretRef = nil
		ref := secretRef("oidc", "client-secret")
		cfg.Spec.Auth.OIDC.ClientSecretRef = ref
		secret := createSecret(ref.Name, map[string]string{"other-key": "x"})
		createPlatformConfig(cfg)

		expectReady(
			cfg, metav1.ConditionFalse, v1.ReasonMissingSecret,
			Equal(missingKeyMessage("spec.auth.oidc.clientSecretRef", ref)),
		)

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(secret), secret)).To(Succeed())
		secret.Data[ref.Key] = []byte("s")
		Expect(k8sClient.Update(ctx, secret)).To(Succeed())

		expectReady(cfg, metav1.ConditionTrue, v1.ReasonHealthy, Equal("All checks passed"))
	})

	It("checks the license Secret once the oidc client Secret is present", func() {
		cfg := realisticPlatformConfig()
		clientRef := secretRef("oidc", "client-secret")
		licenseRef := secretRef("lic", "license-key")
		cfg.Spec.Auth.OIDC.ClientSecretRef = clientRef
		cfg.Spec.LicenseSecretRef = &licenseRef
		createSecret(clientRef.Name, map[string]string{clientRef.Key: "s"})
		createPlatformConfig(cfg)

		expectReady(cfg, metav1.ConditionFalse, v1.ReasonMissingSecret, Equal(notFoundMessage("spec.licenseSecretRef", licenseRef)))
	})

	It("flips back to MissingSecret when a referenced Secret is deleted", func() {
		cfg := minimalPlatformConfig()
		ref := secretRef("lic", "license-key")
		cfg.Spec.LicenseSecretRef = &ref
		secret := createSecret(ref.Name, map[string]string{ref.Key: "x"})
		createPlatformConfig(cfg)
		expectReady(cfg, metav1.ConditionTrue, v1.ReasonHealthy, Equal("All checks passed"))

		Expect(k8sClient.Delete(ctx, secret)).To(Succeed())

		expectReady(cfg, metav1.ConditionFalse, v1.ReasonMissingSecret, Equal(notFoundMessage("spec.licenseSecretRef", ref)))
	})

	It("reports MissingSecret naming the management identity client, then Healthy once the Secret exists", func() {
		cfg := realisticPlatformConfig()
		cfg.Spec.LicenseSecretRef = nil
		clientRef := secretRef("oidc", "client-secret")
		identityRef := secretRef("mgmt-identity", "client-secret")
		cfg.Spec.Auth.OIDC.ClientSecretRef = clientRef
		cfg.Spec.Auth.OIDC.Management = &v1.ManagementOIDCClientsSpec{
			Clients: v1.ManagementClients{
				Identity: &v1.ConfidentialClientSpec{
					ClientID: "camunda-identity", ClientSecretRef: identityRef,
				},
			},
		}
		createSecret(clientRef.Name, map[string]string{clientRef.Key: "s"})
		createPlatformConfig(cfg)

		expectReady(
			cfg, metav1.ConditionFalse, v1.ReasonMissingSecret,
			Equal(notFoundMessage("spec.auth.oidc.management.clients.identity.clientSecretRef", identityRef)),
		)

		createSecret(identityRef.Name, map[string]string{identityRef.Key: "s"})

		expectReady(cfg, metav1.ConditionTrue, v1.ReasonHealthy, Equal("All checks passed"))
	})

	It("reports MissingSecret naming the web modeler api client", func() {
		cfg := realisticPlatformConfig()
		cfg.Spec.LicenseSecretRef = nil
		clientRef := secretRef("oidc", "client-secret")
		apiRef := secretRef("mgmt-modeler-api", "client-secret")
		cfg.Spec.Auth.OIDC.ClientSecretRef = clientRef
		cfg.Spec.Auth.OIDC.Management = &v1.ManagementOIDCClientsSpec{
			Clients: v1.ManagementClients{
				Console:    &v1.PublicClientSpec{ClientID: "camunda-console"},
				WebModeler: &v1.PublicClientSpec{ClientID: "camunda-web-modeler"},
				WebModelerAPI: &v1.WebModelerAPIClientSpec{
					ConfidentialClientSpec: v1.ConfidentialClientSpec{
						ClientID: "camunda-web-modeler-api", ClientSecretRef: apiRef,
					},
				},
			},
		}
		createSecret(clientRef.Name, map[string]string{clientRef.Key: "s"})
		createPlatformConfig(cfg)

		expectReady(
			cfg, metav1.ConditionFalse, v1.ReasonMissingSecret,
			Equal(notFoundMessage("spec.auth.oidc.management.clients.webModelerApi.clientSecretRef", apiRef)),
		)
	})

	It("re-stamps observedGeneration after a spec update", func() {
		cfg := minimalPlatformConfig()
		createPlatformConfig(cfg)
		expectReady(cfg, metav1.ConditionTrue, v1.ReasonHealthy, Equal("All checks passed"))

		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cfg), cfg)).To(Succeed())
		cfg.Spec.ImageRegistry = "registry.example.com/camunda"
		Expect(k8sClient.Update(ctx, cfg)).To(Succeed())

		Eventually(func(g Gomega) {
			var updated v1.CamundaPlatformConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cfg), &updated)).To(Succeed())
			g.Expect(updated.Generation).To(BeNumerically(">", int64(1)))
			g.Expect(updated.Status.ObservedGeneration).To(Equal(updated.Generation))
		}, timeout, interval).Should(Succeed())
	})
})
