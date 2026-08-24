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

package databaseserverconfig

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

var _ = Describe("DatabaseServerConfig controller", func() {
	var (
		namespace    string
		serverConfig *v1.DatabaseServerConfig
	)

	BeforeEach(func() {
		namespace = "dbsc-ns-" + utilrand.String(8)
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
		Expect(k8sClient.Create(ctx, ns)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })

		serverConfig = fixtures.DatabaseServerConfig(namespace)
	})

	// createServerConfig submits the fixture CR and registers its deletion.
	createServerConfig := func() {
		Expect(k8sClient.Create(ctx, serverConfig)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, serverConfig) })
	}

	// adminSecret builds the referenced admin-creds Secret with the given keys.
	// The values are the admin credentials of the shared container. A complete
	// Secret therefore lets the probe reach the server.
	adminSecret := func(keys ...string) *corev1.Secret {
		pg, err := testPostgres()
		Expect(err).NotTo(HaveOccurred())
		values := map[string]string{"username": pg.AdminUser, "password": pg.AdminPassword}
		data := map[string][]byte{}
		for _, key := range keys {
			data[key] = []byte(values[key])
		}
		return &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      serverConfig.Spec.AdminCredentialsSecretRef.Name,
				Namespace: namespace,
			},
			Data: data,
		}
	}

	// pointAtServer aims the fixture at the shared PostgreSQL container. The
	// probe then finds a live server behind the admin credentials.
	pointAtServer := func() {
		pg, err := testPostgres()
		Expect(err).NotTo(HaveOccurred())
		serverConfig.Spec.Host = pg.Host
		serverConfig.Spec.Port = pg.Port
	}

	// expectReady polls until the Ready condition of the CR matches exactly.
	expectReady := func(status metav1.ConditionStatus, reason, message string) {
		GinkgoHelper()
		Eventually(func(g Gomega) {
			var got v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &got)).To(Succeed())
			cond := meta.FindStatusCondition(got.Status.Conditions, v1.ConditionReady)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(status))
			g.Expect(cond.Reason).To(Equal(reason))
			g.Expect(cond.Message).To(Equal(message))
		}, timeout, interval).Should(Succeed())
	}

	// expectServerVersion polls until the probed major and its timestamp are
	// published.
	expectServerVersion := func(major string) {
		GinkgoHelper()
		Eventually(func(g Gomega) {
			var got v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &got)).To(Succeed())
			g.Expect(got.Status.ServerVersion).To(Equal(major))
			g.Expect(got.Status.ProbedAt).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
	}

	// expectSystemIdentifier polls until the published identity of the server
	// is the one the container reports, and returns it.
	expectSystemIdentifier := func() string {
		GinkgoHelper()
		want, err := testPostgresSystemIdentifier()
		Expect(err).NotTo(HaveOccurred())
		Expect(want).NotTo(BeEmpty())
		Eventually(func(g Gomega) {
			var got v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &got)).To(Succeed())
			g.Expect(got.Status.SystemIdentifier).To(Equal(want))
		}, timeout, interval).Should(Succeed())

		return want
	}

	notFoundMessage := func() string {
		return fmt.Sprintf("Secret %s not found", namespace+"/"+serverConfig.Spec.AdminCredentialsSecretRef.Name)
	}

	// A contract of one namespace never reaches the admin Secret of another.
	// The Secret reference carries no namespace, so the lookup is local.
	It("ignores a Secret of the same name in another namespace", func() {
		pointAtServer()
		other := "dbsc-other-" + utilrand.String(8)
		otherNS := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: other}}
		Expect(k8sClient.Create(ctx, otherNS)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, otherNS) })

		secret := adminSecret("username", "password")
		secret.Namespace = other
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		createServerConfig()

		expectReady(metav1.ConditionFalse, v1.ReasonMissingSecret, notFoundMessage())
	})

	It("reports MissingSecret when the admin credentials Secret does not exist", func() {
		createServerConfig()

		expectReady(metav1.ConditionFalse, v1.ReasonMissingSecret, notFoundMessage())
	})

	It("flips to Healthy and publishes the server version when the Secret appears", func() {
		pointAtServer()
		createServerConfig()
		expectReady(metav1.ConditionFalse, v1.ReasonMissingSecret, notFoundMessage())

		secret := adminSecret("username", "password")
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		expectReady(metav1.ConditionTrue, v1.ReasonHealthy, "Reached the server; it runs major version 17")
		expectServerVersion("17")
		expectSystemIdentifier()
	})

	It("flips back to MissingSecret when the Secret is deleted", func() {
		pointAtServer()
		secret := adminSecret("username", "password")
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())

		createServerConfig()
		expectReady(metav1.ConditionTrue, v1.ReasonHealthy, "Reached the server; it runs major version 17")

		Expect(k8sClient.Delete(ctx, secret)).To(Succeed())

		expectReady(metav1.ConditionFalse, v1.ReasonMissingSecret, notFoundMessage())
	})

	// The host of the fixture is a service that does not exist here, so a
	// complete Secret alone is not enough. Ready proves that the server
	// answered, not that a Secret exists.
	It("reports ConnectionFailed when the server does not answer", func() {
		secret := adminSecret("username", "password")
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		createServerConfig()

		Eventually(func(g Gomega) {
			var got v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &got)).To(Succeed())
			cond := meta.FindStatusCondition(got.Status.Conditions, v1.ConditionReady)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(cond.Reason).To(Equal(v1.ReasonConnectionFailed))
			g.Expect(cond.Message).To(ContainSubstring(serverConfig.Spec.Host))
			g.Expect(got.Status.ServerVersion).To(BeEmpty())
			g.Expect(got.Status.SystemIdentifier).To(BeEmpty())
		}, timeout, interval).Should(Succeed())
	})

	It("reports ConnectionFailed when the credentials are rejected", func() {
		pointAtServer()
		secret := adminSecret("username", "password")
		secret.Data["password"] = []byte("wrong")
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		createServerConfig()

		Eventually(func(g Gomega) {
			var got v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &got)).To(Succeed())
			cond := meta.FindStatusCondition(got.Status.Conditions, v1.ConditionReady)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Reason).To(Equal(v1.ReasonConnectionFailed))
		}, timeout, interval).Should(Succeed())
	})

	It("reports MissingSecret naming the key when the Secret lacks the password key", func() {
		secret := adminSecret("username")
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		createServerConfig()

		expectReady(metav1.ConditionFalse, v1.ReasonMissingSecret, fmt.Sprintf(
			"Secret %s is missing key %q",
			namespace+"/"+serverConfig.Spec.AdminCredentialsSecretRef.Name,
			serverConfig.Spec.AdminCredentialsSecretRef.PasswordKey,
		))
	})

	// The controller does not repeat a successful probe on every reconcile.
	// The status flush of a fresh probe writes nothing new, so it wakes no
	// watch, and the interval sets the cadence. A spec change or a Secret
	// change probes again.
	It("probes once per interval, and again when the spec names another server", func() {
		pointAtServer()
		secret := adminSecret("username", "password")
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		before := probes.Load()
		createServerConfig()
		expectReady(metav1.ConditionTrue, v1.ReasonHealthy, "Reached the server; it runs major version 17")

		var got v1.DatabaseServerConfig
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &got)).To(Succeed())
		probedAt := got.Status.ProbedAt.DeepCopy()
		count := probes.Load()
		Expect(count - before).To(Equal(int64(1)))

		By("leaving probedAt and the probe count alone while the probe is fresh")
		Consistently(func(g Gomega) {
			var again v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &again)).To(Succeed())
			g.Expect(again.Status.ProbedAt.Equal(probedAt)).To(BeTrue())
			g.Expect(probes.Load()).To(Equal(count))
		}, "3s", interval).Should(Succeed())

		By("leaving the probe alone for a spec change that names the same server")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &got)).To(Succeed())
			got.Spec.PITR = &v1.PITRCapability{Enabled: false}
			g.Expect(k8sClient.Update(ctx, &got)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			var again v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &again)).To(Succeed())
			g.Expect(again.Status.ProbedAt.Equal(probedAt)).To(BeTrue())
			g.Expect(probes.Load()).To(Equal(count))
		}, "2s", interval).Should(Succeed())

		By("probing again when the spec names another admin Secret")
		copied := adminSecret("username", "password")
		copied.Name = "admin-copy"
		Expect(k8sClient.Create(ctx, copied)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, copied) })

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &got)).To(Succeed())
			got.Spec.AdminCredentialsSecretRef.Name = copied.Name
			g.Expect(k8sClient.Update(ctx, &got)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(probes.Load()).To(Equal(count + 1))
			var again v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &again)).To(Succeed())
			g.Expect(again.Status.ProbedAt.After(probedAt.Time)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		By("probing again after the credentials Secret changes")
		Eventually(func(g Gomega) {
			var current corev1.Secret
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(copied), &current)).To(Succeed())
			if current.Annotations == nil {
				current.Annotations = map[string]string{}
			}
			current.Annotations["rotated"] = "now"
			g.Expect(k8sClient.Update(ctx, &current)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(probes.Load()).To(Equal(count + 2))
		}, timeout, interval).Should(Succeed())
	})

	// The published identity belongs to the endpoint the operator reached. A
	// spec that names another endpoint publishes nothing until the probe says
	// what is behind it, so a consumer never keys on the identity of a server
	// this contract no longer describes.
	It("clears the version and the identity when the spec is repointed", func() {
		pointAtServer()
		secret := adminSecret("username", "password")
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		createServerConfig()
		expectReady(metav1.ConditionTrue, v1.ReasonHealthy, "Reached the server; it runs major version 17")
		expectSystemIdentifier()

		By("repointing the host at an address that answers nothing")
		Eventually(func(g Gomega) {
			var current v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &current)).To(Succeed())
			current.Spec.Host = "127.0.0.1"
			current.Spec.Port = 1
			g.Expect(k8sClient.Update(ctx, &current)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			var got v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &got)).To(Succeed())
			cond := meta.FindStatusCondition(got.Status.Conditions, v1.ConditionReady)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Reason).To(Equal(v1.ReasonConnectionFailed))
			g.Expect(got.Status.SystemIdentifier).To(BeEmpty())
			g.Expect(got.Status.ServerVersion).To(BeEmpty())
			g.Expect(got.Status.ProbedAt).To(BeNil())
			g.Expect(got.Status.ProbedEndpoint).To(BeEmpty())
			g.Expect(got.Status.ProbedSecretName).To(BeEmpty())
			g.Expect(got.Status.ProbedSecretVersion).To(BeEmpty())
		}, timeout, interval).Should(Succeed())
	})

	// A recovery request and its answer are written on the spec of a contract
	// whose server is running, and they name no endpoint. Clearing the
	// identity for them would take this contract, and every Database on it,
	// out of Ready for a write that cannot move the server.
	It("keeps the identity when a spec change names the same server", func() {
		pointAtServer()
		secret := adminSecret("username", "password")
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		createServerConfig()
		expectReady(metav1.ConditionTrue, v1.ReasonHealthy, "Reached the server; it runs major version 17")
		identifier := expectSystemIdentifier()

		var probed v1.DatabaseServerConfig
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &probed)).To(Succeed())
		probedAt := probed.Status.ProbedAt.DeepCopy()

		By("writing a recovery request on the contract")
		Eventually(func(g Gomega) {
			var current v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &current)).To(Succeed())
			current.Spec.Recovery = &v1.RecoveryRequest{
				RequestID:   "3f2b1c4d-5e6a-4b7c-8d9e-0f1a2b3c4d5e",
				RequestedBy: current.Namespace + "/pitr-1",
				TargetTime:  "2026-08-20T14:30:00Z",
			}
			g.Expect(k8sClient.Update(ctx, &current)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			var got v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &got)).To(Succeed())
			g.Expect(got.Status.ObservedGeneration).To(Equal(got.Generation))
			cond := meta.FindStatusCondition(got.Status.Conditions, v1.ConditionReady)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(cond.ObservedGeneration).To(Equal(got.Generation))
			g.Expect(got.Status.SystemIdentifier).To(Equal(identifier))
			g.Expect(got.Status.ProbedAt).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		// The record of the probe is not cleared and taken again: it stands.
		// A clear and a re-probe would take the contract, and every Database
		// on it, out of Ready for as long as the probe takes.
		Consistently(func(g Gomega) {
			var got v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &got)).To(Succeed())
			g.Expect(got.Status.SystemIdentifier).To(Equal(identifier))
			g.Expect(got.Status.ProbedAt.Equal(probedAt)).To(BeTrue())
		}, "2s", interval).Should(Succeed())
	})

	It("catches status.observedGeneration up to metadata.generation after a spec update", func() {
		pointAtServer()
		secret := adminSecret("username", "password")
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		createServerConfig()
		expectReady(metav1.ConditionTrue, v1.ReasonHealthy, "Reached the server; it runs major version 17")

		var fetched v1.DatabaseServerConfig
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &fetched)).To(Succeed())
		fetched.Spec.Host = "replica.camunda-system.svc.cluster.local"
		Expect(k8sClient.Update(ctx, &fetched)).To(Succeed())
		Expect(fetched.Generation).To(BeNumerically(">", 1))

		Eventually(func(g Gomega) {
			var got v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(serverConfig), &got)).To(Succeed())
			g.Expect(got.Status.ObservedGeneration).To(Equal(fetched.Generation))
		}, timeout, interval).Should(Succeed())
	})
})
