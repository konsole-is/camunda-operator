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

package camundacluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin/camundaadmintest"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
)

var _ = Describe("Admin password rotation", func() {
	var users *camundaadmintest.UserAPI

	BeforeEach(func() {
		users = camundaadmintest.NewUserAPI()
		DeferCleanup(users.Close)
		userAPIEndpoint.Store(users.URL())
		DeferCleanup(func() { userAPIEndpoint.Store("http://127.0.0.1:1") })
	})

	// adminSecretKey locates the admin Secret of cluster.
	adminSecretKey := func(cluster *v1.CamundaCluster) client.ObjectKey {
		return client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-camunda-admin"}
	}

	// fetchAdminPassword waits for the admin Secret and returns the active
	// password.
	fetchAdminPassword := func(cluster *v1.CamundaCluster) string {
		GinkgoHelper()
		var secret corev1.Secret
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, adminSecretKey(cluster), &secret)).To(Succeed())
			g.Expect(secret.Data[components.AdminPasswordKey]).NotTo(BeEmpty())
		}, timeout, interval).Should(Succeed())
		return string(secret.Data[components.AdminPasswordKey])
	}

	// seedClusterUser tells the fake that the running cluster holds the
	// given admin password, the way the initial user seed does.
	seedClusterUser := func(password string) {
		users.SetUser(components.AdminUsername, components.AdminUsername, components.AdminEmail, password)
	}

	// requestRotation sets spec.auth.basic.passwordRotation on cluster.
	requestRotation := func(cluster *v1.CamundaCluster, value string) {
		GinkgoHelper()
		updateCluster(cluster, func(c *v1.CamundaCluster) {
			c.Spec.Auth = &v1.ClusterAuthSpec{Basic: &v1.BasicAuthSpec{PasswordRotation: value}}
		})
	}

	// expectRotated waits until cluster records the rotation and its Secret
	// holds the password of the fake, without a pending key.
	expectRotated := func(cluster *v1.CamundaCluster, rotation string) {
		GinkgoHelper()
		Eventually(func(g Gomega) {
			var latest v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			g.Expect(latest.Status.AdminPassword).NotTo(BeNil())
			g.Expect(latest.Status.AdminPassword.Rotation).To(Equal(rotation))

			var secret corev1.Secret
			g.Expect(k8sClient.Get(ctx, adminSecretKey(cluster), &secret)).To(Succeed())
			g.Expect(string(secret.Data[components.AdminPasswordKey])).To(
				Equal(users.Password(components.AdminUsername)),
			)
			g.Expect(secret.Data).NotTo(HaveKey(components.AdminPendingPasswordKey))
		}, timeout, interval).Should(Succeed())
	}

	It("rotates the password through the user API, records it, and rolls connectors", func() {
		ns := newNamespace()
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		enabled := true
		cluster.Spec.Connectors = &v1.ConnectorsSpec{Enabled: &enabled, Version: "8.9.7"}
		createCluster(cluster)

		password := fetchAdminPassword(cluster)
		seedClusterUser(password)

		connectorsKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-connectors"}
		oldHash := fetchDeployment(connectorsKey).Spec.Template.Annotations[components.ConfigHashAnnotation]
		Expect(oldHash).NotTo(BeEmpty())

		requestRotation(cluster, "2026-08")
		expectRotated(cluster, "2026-08")

		rotated := users.Password(components.AdminUsername)
		Expect(rotated).NotTo(Equal(password))
		name, email := users.Profile(components.AdminUsername)
		Expect(name).To(Equal("admin"))
		Expect(email).To(Equal("admin@example.com"))
		expectEvent(cluster, "AdminPasswordRotated", corev1.EventTypeNormal)

		By("rolling the connectors Deployment on the new password")
		Eventually(func(g Gomega) {
			var connectors appsv1.Deployment
			g.Expect(k8sClient.Get(ctx, connectorsKey, &connectors)).To(Succeed())
			g.Expect(connectors.Spec.Template.Annotations[components.ConfigHashAnnotation]).NotTo(Equal(oldHash))
		}, timeout, interval).Should(Succeed())

		By("not rotating again for the same value")
		calls := users.UpdateCalls()
		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.PodLabels = map[string]string{"touch": "1"} })
		Eventually(func(g Gomega) {
			var sts appsv1.StatefulSet
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"}, &sts,
			)).To(Succeed())
			g.Expect(sts.Spec.Template.Labels).To(HaveKeyWithValue("touch", "1"))
		}, timeout, interval).Should(Succeed())
		Expect(users.UpdateCalls()).To(Equal(calls))
	})

	It("keeps the connectors hash on the published password when the Secret write fails", func() {
		ns := newNamespace()
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		enabled := true
		cluster.Spec.Connectors = &v1.ConnectorsSpec{Enabled: &enabled, Version: "8.9.7"}
		createCluster(cluster)

		password := fetchAdminPassword(cluster)
		seedClusterUser(password)

		By("stalling the rotation on the pending password")
		userAPIEndpoint.Store("http://127.0.0.1:1")
		requestRotation(cluster, "round-1")
		expectCondition(cluster, v1.ConditionAdminSecretReady, Equal(v1.ReasonConnectionFailed))

		var secret corev1.Secret
		Expect(k8sClient.Get(ctx, adminSecretKey(cluster), &secret)).To(Succeed())
		pending := string(secret.Data[components.AdminPendingPasswordKey])
		Expect(pending).NotTo(BeEmpty())

		connectorsKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-connectors"}
		beforeHash := fetchDeployment(connectorsKey).Spec.Template.Annotations[components.ConfigHashAnnotation]
		Expect(beforeHash).NotTo(BeEmpty())

		By("making every further write of the admin Secret fail")
		secret.Immutable = new(true)
		Expect(k8sClient.Update(ctx, &secret)).To(Succeed())

		By("letting the user API accept the call")
		userAPIEndpoint.Store(users.URL())
		Eventually(func(g Gomega) {
			g.Expect(users.Password(components.AdminUsername)).To(Equal(pending))
		}, timeout, interval).Should(Succeed())

		By("keeping the connectors hash and the recorded rotation on what the Secret still publishes")
		Consistently(func(g Gomega) {
			var latest corev1.Secret
			g.Expect(k8sClient.Get(ctx, adminSecretKey(cluster), &latest)).To(Succeed())
			g.Expect(string(latest.Data[components.AdminPasswordKey])).To(Equal(password))

			var connectors appsv1.Deployment
			g.Expect(k8sClient.Get(ctx, connectorsKey, &connectors)).To(Succeed())
			g.Expect(connectors.Spec.Template.Annotations[components.ConfigHashAnnotation]).To(Equal(beforeHash))

			var recorded v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &recorded)).To(Succeed())
			g.Expect(recorded.Status.AdminPassword).To(
				BeNil(), "a rotation is complete only once the Secret publishes the promoted password",
			)
		}, "5s", interval).Should(Succeed())
	})

	It("keeps the active password and reports Rejected until the cluster accepts the call", func() {
		cluster := createDefaultCluster()
		password := fetchAdminPassword(cluster)
		seedClusterUser("changed-in-the-admin-app")

		requestRotation(cluster, "round-1")
		expectCondition(cluster, v1.ConditionAdminSecretReady, Equal(v1.ReasonRejected))
		Eventually(func(g Gomega) {
			var latest v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			cond := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionAdminSecretReady)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(cond.Message).To(ContainSubstring("bad credentials"))
			g.Expect(latest.Status.AdminPassword).To(BeNil(), "a failed rotation records nothing")
		}, timeout, interval).Should(Succeed())

		var secret corev1.Secret
		Expect(k8sClient.Get(ctx, adminSecretKey(cluster), &secret)).To(Succeed())
		Expect(string(secret.Data[components.AdminPasswordKey])).To(Equal(password))

		By("completing the rotation once the passwords match again")
		seedClusterUser(password)
		expectRotated(cluster, "round-1")
		expectCondition(cluster, v1.ConditionAdminSecretReady, Equal(string(component.Healthy)))
	})

	It("records a requested rotation only while the cluster never published an admin Secret", func() {
		cluster := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "cc-seam", Namespace: newNamespace()},
		}
		in := components.Input{
			Cluster: cluster,
			Effective: components.Effective{
				CamundaClusterSpec: v1.CamundaClusterSpec{
					Auth: &v1.ClusterAuthSpec{Basic: &v1.BasicAuthSpec{PasswordRotation: "round-1"}},
				},
			},
		}
		reconciler := &CamundaClusterReconciler{APIReader: k8sClient}

		By("seeding the initial user of a new cluster with the requested rotation")
		cred, err := reconciler.resolveAdminCredential(ctx, cluster, in)
		Expect(err).NotTo(HaveOccurred())
		Expect(cred.password.Value).NotTo(BeEmpty())
		Expect(cred.rotation).To(Equal("round-1"))

		By("keeping the recorded rotation once the cluster has published a Secret")
		meta.SetStatusCondition(cluster.GetStatusConditions(), metav1.Condition{
			Type:    v1.ConditionAdminSecretReady,
			Status:  metav1.ConditionTrue,
			Reason:  string(component.Healthy),
			Message: "the admin Secret is published",
		})

		cred, err = reconciler.resolveAdminCredential(ctx, cluster, in)
		Expect(err).NotTo(HaveOccurred())
		Expect(cred.password.Value).NotTo(BeEmpty())
		Expect(cred.rotation).To(BeEmpty(), "a rotation requested after a delete must reach the user API")
	})

	It("reads the applied rotation from the Secret, so a lost status never rotates twice", func() {
		ns := newNamespace()
		cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: "cc-token", Namespace: ns}}
		createSecret(ns, components.AdminSecretName(cluster), map[string]string{
			components.AdminUsernameKey: components.AdminUsername,
			components.AdminPasswordKey: "the-active-password",
			components.AdminRotationKey: "round-1",
		})
		in := components.Input{
			Cluster: cluster,
			Effective: components.Effective{
				CamundaClusterSpec: v1.CamundaClusterSpec{
					Auth: &v1.ClusterAuthSpec{Basic: &v1.BasicAuthSpec{PasswordRotation: "round-1"}},
				},
			},
		}
		reconciler := &CamundaClusterReconciler{APIReader: k8sClient}

		// status.adminPassword is empty here, which is what a lost status
		// flush or a cache that lags one reconcile behind looks like.
		cred, err := reconciler.resolveAdminCredential(ctx, cluster, in)
		Expect(err).NotTo(HaveOccurred())
		Expect(cred.pending).To(BeEmpty(), "the Secret already records this rotation")
		Expect(cred.password.Value).To(Equal("the-active-password"))
		Expect(cred.rotation).To(Equal("round-1"))
	})

	It("refuses a rotation that was requested while the admin Secret was gone", func() {
		cluster := createDefaultCluster()
		password := fetchAdminPassword(cluster)
		seedClusterUser(password)

		By("stalling the rotation on the pending password")
		userAPIEndpoint.Store("http://127.0.0.1:1")
		requestRotation(cluster, "round-1")
		expectCondition(cluster, v1.ConditionAdminSecretReady, Equal(v1.ReasonConnectionFailed))

		By("pausing the cluster, so that no reconcile republishes the Secret")
		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.Pause = true })
		expectEvent(cluster, "Paused", corev1.EventTypeNormal)

		By("deleting the admin Secret")
		var secret corev1.Secret
		Expect(k8sClient.Get(ctx, adminSecretKey(cluster), &secret)).To(Succeed())
		Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())
		Consistently(func(g Gomega) {
			err := k8sClient.Get(ctx, adminSecretKey(cluster), &corev1.Secret{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "the paused cluster republished its admin Secret")
		}, "3s", interval).Should(Succeed())

		By("resuming on a cluster that still holds the first password")
		userAPIEndpoint.Store(users.URL())
		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.Pause = false })

		By("publishing a password that the cluster does not hold")
		Eventually(func(g Gomega) {
			var latest corev1.Secret
			g.Expect(k8sClient.Get(ctx, adminSecretKey(cluster), &latest)).To(Succeed())
			g.Expect(string(latest.Data[components.AdminPasswordKey])).NotTo(BeEmpty())
			g.Expect(string(latest.Data[components.AdminPasswordKey])).NotTo(Equal(password))
		}, timeout, interval).Should(Succeed())

		By("recording no rotation")
		Consistently(func(g Gomega) {
			var latest v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			g.Expect(latest.Status.AdminPassword).To(BeNil())
		}, "5s", interval).Should(Succeed())

		By("reporting Rejected, because the operator holds no password that the cluster accepts")
		expectCondition(cluster, v1.ConditionAdminSecretReady, Equal(v1.ReasonRejected))
	})

	It("reports ConnectionFailed while the cluster does not answer, and retries", func() {
		userAPIEndpoint.Store("http://127.0.0.1:1")
		cluster := createDefaultCluster()
		password := fetchAdminPassword(cluster)

		requestRotation(cluster, "round-1")
		expectCondition(cluster, v1.ConditionAdminSecretReady, Equal(v1.ReasonConnectionFailed))

		var secret corev1.Secret
		Expect(k8sClient.Get(ctx, adminSecretKey(cluster), &secret)).To(Succeed())
		Expect(string(secret.Data[components.AdminPasswordKey])).To(Equal(password))

		By("completing the rotation once the cluster answers")
		seedClusterUser(password)
		userAPIEndpoint.Store(users.URL())
		expectRotated(cluster, "round-1")
	})

	It("ignores the field under OIDC and rejects nothing", func() {
		ns := newNamespace()
		cfg := createPlatformConfig()
		oidcSecret := createSecret(ns, "oidc", map[string]string{"client-secret": "s"})
		Eventually(func(g Gomega) {
			var latest v1.CamundaPlatformConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKey{Name: cfg.Name}, &latest)).To(Succeed())
			latest.Spec.Auth = &v1.PlatformAuthSpec{
				Method: v1.AuthenticationMethodOIDC,
				OIDC: &v1.OIDCSpec{
					IssuerURL: "https://idp.example.com/realms/camunda",
					ClientID:  "platform-client",
					ClientSecretRef: v1.SecretKeyRef{
						Name: oidcSecret.Name, Namespace: ns, Key: "client-secret",
					},
				},
			}
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		cluster := newCluster(ns, cfg, createBinding(ns, true))
		cluster.Spec.Auth = &v1.ClusterAuthSpec{Basic: &v1.BasicAuthSpec{PasswordRotation: "2026-08"}}
		createCluster(cluster)

		expectCondition(cluster, v1.ConditionAdminSecretReady, Equal(string(component.Disabled)))
		var latest v1.CamundaCluster
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		Expect(latest.Status.AdminPassword).To(BeNil())
		Expect(apierrors.IsNotFound(k8sClient.Get(ctx, adminSecretKey(cluster), &corev1.Secret{}))).To(BeTrue())
		Expect(users.UpdateCalls()).To(BeZero())
	})
})
