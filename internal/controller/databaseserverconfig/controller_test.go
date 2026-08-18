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
	"k8s.io/apimachinery/pkg/types"
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

		serverConfig = fixtures.DatabaseServerConfig()
		serverConfig.Spec.AdminCredentialsSecretRef.Namespace = namespace
	})

	// createServerConfig submits the fixture CR and registers its deletion.
	createServerConfig := func() {
		Expect(k8sClient.Create(ctx, serverConfig)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, serverConfig) })
	}

	// adminSecret builds the referenced admin-creds Secret with the given keys.
	// The values are the admin credentials of the shared container, so a
	// complete Secret lets the probe reach the server.
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

	// pointAtServer aims the fixture at the shared PostgreSQL container, so
	// the probe finds a live server behind the admin credentials.
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
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: serverConfig.Name}, &got)).To(Succeed())
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
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: serverConfig.Name}, &got)).To(Succeed())
			g.Expect(got.Status.ServerVersion).To(Equal(major))
			g.Expect(got.Status.ProbedAt).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
	}

	notFoundMessage := func() string {
		return fmt.Sprintf("Secret %q not found", namespace+"/"+serverConfig.Spec.AdminCredentialsSecretRef.Name)
	}

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

	// The fixture's host is a service that does not exist here, so a
	// complete Secret alone is not enough anymore: Ready proves the server
	// answered, not that a Secret exists.
	It("reports ConnectionFailed when the server does not answer", func() {
		secret := adminSecret("username", "password")
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		createServerConfig()

		Eventually(func(g Gomega) {
			var got v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: serverConfig.Name}, &got)).To(Succeed())
			cond := meta.FindStatusCondition(got.Status.Conditions, v1.ConditionReady)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(cond.Reason).To(Equal(v1.ReasonConnectionFailed))
			g.Expect(cond.Message).To(ContainSubstring(serverConfig.Spec.Host))
			g.Expect(got.Status.ServerVersion).To(BeEmpty())
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
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: serverConfig.Name}, &got)).To(Succeed())
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
			"Secret %q is missing key %q",
			namespace+"/"+serverConfig.Spec.AdminCredentialsSecretRef.Name,
			serverConfig.Spec.AdminCredentialsSecretRef.PasswordKey,
		))
	})

	// A successful probe is not repeated on every reconcile: the status flush
	// of a fresh probe writes nothing new, so it wakes no watch, and the
	// interval is the cadence. A spec change or a Secret change probes again.
	It("probes once per interval, and again on a spec or Secret change", func() {
		pointAtServer()
		secret := adminSecret("username", "password")
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		before := probes.Load()
		createServerConfig()
		expectReady(metav1.ConditionTrue, v1.ReasonHealthy, "Reached the server; it runs major version 17")

		var got v1.DatabaseServerConfig
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: serverConfig.Name}, &got)).To(Succeed())
		probedAt := got.Status.ProbedAt.DeepCopy()
		count := probes.Load()
		Expect(count - before).To(Equal(int64(1)))

		By("leaving probedAt and the probe count alone while the probe is fresh")
		Consistently(func(g Gomega) {
			var again v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: serverConfig.Name}, &again)).To(Succeed())
			g.Expect(again.Status.ProbedAt.Equal(probedAt)).To(BeTrue())
			g.Expect(probes.Load()).To(Equal(count))
		}, "3s", interval).Should(Succeed())

		By("probing again after a spec change")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: serverConfig.Name}, &got)).To(Succeed())
			got.Spec.PITR = &v1.PITRCapability{Enabled: false}
			g.Expect(k8sClient.Update(ctx, &got)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Eventually(func(g Gomega) {
			g.Expect(probes.Load()).To(Equal(count + 1))
			var again v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: serverConfig.Name}, &again)).To(Succeed())
			g.Expect(again.Status.ProbedAt.After(probedAt.Time)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		By("probing again after the credentials Secret changes")
		Eventually(func(g Gomega) {
			var current corev1.Secret
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(secret), &current)).To(Succeed())
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

	It("catches status.observedGeneration up to metadata.generation after a spec update", func() {
		pointAtServer()
		secret := adminSecret("username", "password")
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })

		createServerConfig()
		expectReady(metav1.ConditionTrue, v1.ReasonHealthy, "Reached the server; it runs major version 17")

		var fetched v1.DatabaseServerConfig
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: serverConfig.Name}, &fetched)).To(Succeed())
		fetched.Spec.Host = "replica.camunda-system.svc.cluster.local"
		Expect(k8sClient.Update(ctx, &fetched)).To(Succeed())
		Expect(fetched.Generation).To(BeNumerically(">", 1))

		Eventually(func(g Gomega) {
			var got v1.DatabaseServerConfig
			g.Expect(k8sClient.Get(ctx, types.NamespacedName{Name: serverConfig.Name}, &got)).To(Succeed())
			g.Expect(got.Status.ObservedGeneration).To(Equal(fetched.Generation))
		}, timeout, interval).Should(Succeed())
	})
})
