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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin/camundaadmintest"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

var _ = Describe("Admin password rotation", func() {
	var users *camundaadmintest.UserAPI

	BeforeEach(func() {
		users = camundaadmintest.NewUserAPI()
		DeferCleanup(users.Close)
	})

	// serveCluster points the namespace of cluster at the fake of this spec.
	// A namespace that never registers has no user API, which is what a spec
	// wants while it drives a rotation that cannot reach the cluster.
	serveCluster := func(cluster *v1.CamundaCluster) {
		serveUserAPI(cluster.Namespace, users.URL())
	}

	// silenceCluster takes the user API of the namespace of cluster off the
	// network again.
	silenceCluster := func(cluster *v1.CamundaCluster) {
		serveUserAPI(cluster.Namespace, unreachableUserAPI)
	}

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
		users.SetUser(components.AdminUsername, components.AdminUsername, components.DefaultAdminEmail, password)
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
		serveCluster(cluster)

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
		silenceCluster(cluster)
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
		serveCluster(cluster)
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

	It("keeps the active password and reports InvalidCredentials until the cluster accepts it", func() {
		cluster := createDefaultCluster()
		serveCluster(cluster)
		password := fetchAdminPassword(cluster)
		seedClusterUser("changed-in-the-admin-app")

		requestRotation(cluster, "round-1")
		expectCondition(cluster, v1.ConditionAdminSecretReady, Equal(v1.ReasonInvalidCredentials))
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

	It("reports the failure of the retry, not the expected rejection of the first call", func() {
		cluster := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "cc-retry", Namespace: newNamespace()},
		}
		in := components.Input{
			Cluster:   cluster,
			Effective: components.Effective{CamundaClusterSpec: v1.CamundaClusterSpec{Version: "8.9.9"}},
		}
		serveCluster(cluster)
		reconciler := &CamundaClusterReconciler{
			APIReader: k8sClient,
			RESTEndpoint: func(cluster *v1.CamundaCluster, _ components.Effective) string {
				return clusterUserAPI(cluster)
			},
		}

		// The cluster holds the pending password already, so the call with
		// the active one is rejected and the retry is the real attempt. The
		// retry then finds the endpoint gone.
		seedClusterUser("pending-password")
		users.DropAfter("updateUser", 1, 1)

		failure := reconciler.updateAdminPassword(
			ctx,
			cluster,
			in,
			"active-password",
			"pending-password",
			components.DefaultAdminEmail,
		)
		Expect(failure).NotTo(BeNil())
		Expect(failure.reason).To(
			Equal(v1.ReasonConnectionFailed), "a cluster that went away must not read as bad credentials",
		)
	})

	It("records the rotation that staged the pending password, not the spec value of today", func() {
		ns := newNamespace()
		cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: "cc-bound", Namespace: ns}}
		createSecret(ns, components.AdminSecretName(cluster), map[string]string{
			components.AdminUsernameKey:        components.AdminUsername,
			components.AdminPasswordKey:        "active-password",
			components.AdminPendingPasswordKey: "pending-password",
			components.AdminPendingRotationKey: "round-1",
		})
		seedClusterUser("active-password")

		// The user cleared the field while the rotation was in flight. The
		// password still changes on the cluster, so the request that staged
		// it is what the operator must record.
		in := components.Input{
			Cluster: cluster,
			Effective: components.Effective{
				CamundaClusterSpec: v1.CamundaClusterSpec{
					Version: "8.9.9",
					Auth:    &v1.ClusterAuthSpec{Basic: &v1.BasicAuthSpec{PasswordRotation: ""}},
				},
			},
		}
		serveCluster(cluster)
		reconciler := &CamundaClusterReconciler{
			APIReader:     k8sClient,
			EventRecorder: events.NewFakeRecorder(10),
			RESTEndpoint: func(cluster *v1.CamundaCluster, _ components.Effective) string {
				return clusterUserAPI(cluster)
			},
		}

		cred, err := reconciler.resolveAdminCredential(ctx, cluster, in)
		Expect(err).NotTo(HaveOccurred())
		Expect(cred.failure).To(BeNil())
		Expect(cred.password.Value).To(Equal("pending-password"))
		Expect(cred.pending).To(BeEmpty())
		Expect(cred.rotation).To(Equal("round-1"), "a cleared field must not lose the applied request")
		Expect(cred.password.SourceUID).To(
			BeEmpty(),
			"the cluster holds this password now, so the apply must not be conditional on the old Secret",
		)
	})

	It("hashes the password it is about to publish on a cluster with no Secret", func() {
		cluster := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "cc-fresh", Namespace: newNamespace()},
		}
		in := components.Input{Cluster: cluster, Effective: components.Effective{}}
		reconciler := &CamundaClusterReconciler{APIReader: k8sClient}

		cred, err := reconciler.resolveAdminCredential(ctx, cluster, in)
		Expect(err).NotTo(HaveOccurred())
		Expect(cred.password.Value).NotTo(BeEmpty())
		Expect(cred.published).To(
			Equal(cred.password.Value),
			"connectors would otherwise roll again as soon as the first Secret lands",
		)
	})

	It("reports what the cluster refused, and retries only a stale password", func() {
		cluster := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "cc-refused", Namespace: newNamespace()},
		}
		in := components.Input{
			Cluster:   cluster,
			Effective: components.Effective{CamundaClusterSpec: v1.CamundaClusterSpec{Version: "8.9.9"}},
		}
		serveCluster(cluster)
		reconciler := &CamundaClusterReconciler{
			APIReader: k8sClient,
			RESTEndpoint: func(cluster *v1.CamundaCluster, _ components.Effective) string {
				return clusterUserAPI(cluster)
			},
		}

		// The cluster refuses the profile itself, not the credentials. The
		// operator must report that answer, not hide it behind the 401 of a
		// retry that cannot help here.
		seedClusterUser("active-password")
		users.RefuseNext(1, "the profile is not acceptable")

		failure := reconciler.updateAdminPassword(
			ctx,
			cluster,
			in,
			"active-password",
			"pending-password",
			components.DefaultAdminEmail,
		)
		Expect(failure).NotTo(BeNil())
		Expect(failure.reason).To(
			Equal(v1.ReasonRejected), "the credentials were good; the cluster refused the call itself",
		)
		Expect(failure.message).To(ContainSubstring("the profile is not acceptable"))
		Expect(users.UpdateCalls()).To(Equal(1), "a refusal that is not a stale password must not retry")
	})

	It("holds the connectors hash while it repairs a Secret that lost its password", func() {
		ns := newNamespace()
		cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: "cc-repair", Namespace: ns}}
		createSecret(ns, components.AdminSecretName(cluster), map[string]string{
			components.AdminUsernameKey: components.AdminUsername,
			components.AdminPasswordKey: "",
		})
		cluster.Status.AdminSecretPublished = true
		in := components.Input{Cluster: cluster, Effective: components.Effective{}}
		reconciler := &CamundaClusterReconciler{APIReader: k8sClient}

		// Connectors of this cluster are already running. A repair that
		// hashed the password it is about to write would roll them onto a
		// password the Secret may never hold, and again on every retry,
		// because each retry generates another one.
		first, err := reconciler.resolveAdminCredential(ctx, cluster, in)
		Expect(err).NotTo(HaveOccurred())
		second, err := reconciler.resolveAdminCredential(ctx, cluster, in)
		Expect(err).NotTo(HaveOccurred())

		Expect(first.password.Value).NotTo(BeEmpty())
		Expect(first.password.Value).NotTo(Equal(second.password.Value), "each attempt is a new password")

		// The hash of connectors is what matters, not the value behind it.
		// It must not move while the repair keeps failing, however many new
		// passwords the attempts generate.
		hashOf := func(cred adminCredential) string {
			hashed := in
			hashed.Effective.Version = "8.9.9"
			hashed.AdminPasswordHash = components.PasswordHash(cred.published)
			connectors := components.Process{Component: components.ComponentConnectors}
			return components.ConfigHash(hashed, connectors)
		}
		Expect(hashOf(second)).To(
			Equal(hashOf(first)), "a repair that keeps failing must not keep rolling connectors",
		)

		// It does move once, here, because an empty digest drops the admin
		// password out of that hash. Holding it still would need the digest
		// of the password the Secret no longer carries.
		settled := adminCredential{published: "the-password-it-used-to-publish"}
		Expect(hashOf(first)).NotTo(Equal(hashOf(settled)))
	})

	It("seeds a basic credential on a cluster that only ever ran OIDC", func() {
		cluster := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Name: "cc-switch", Namespace: newNamespace()},
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

		// An OIDC cluster reports the admin Secret component as Disabled
		// while it owns no Secret at all, so the condition says published
		// and the flag says the truth.
		meta.SetStatusCondition(cluster.GetStatusConditions(), metav1.Condition{
			Type:    v1.ConditionAdminSecretReady,
			Status:  metav1.ConditionFalse,
			Reason:  string(component.Disabled),
			Message: "Component is disabled.",
		})

		cred, err := reconciler.resolveAdminCredential(ctx, cluster, in)
		Expect(err).NotTo(HaveOccurred())
		Expect(cred.password.Value).NotTo(BeEmpty())
		Expect(cred.rotation).To(Equal("round-1"), "the first basic credential of a cluster is its seed")
		Expect(cred.appliedEmail).To(Equal(components.DefaultAdminEmail))
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
		Expect(cred.appliedEmail).To(
			Equal(components.DefaultAdminEmail), "the first Secret seeds the address it publishes",
		)

		By("keeping the recorded rotation once the cluster has published a Secret")
		cluster.Status.AdminSecretPublished = true

		cred, err = reconciler.resolveAdminCredential(ctx, cluster, in)
		Expect(err).NotTo(HaveOccurred())
		Expect(cred.password.Value).NotTo(BeEmpty())
		Expect(cred.rotation).To(BeEmpty(), "a rotation requested after a delete must reach the user API")
		Expect(cred.appliedEmail).To(
			BeEmpty(), "and the address of a replacement Secret is not applied either",
		)
		Expect(cred.email).To(
			Equal(components.DefaultAdminEmail), "while the processes still get a seed",
		)
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
		silenceCluster(cluster)
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
		serveCluster(cluster)
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

		By("reporting InvalidCredentials, because it holds no password that the cluster accepts")
		expectCondition(cluster, v1.ConditionAdminSecretReady, Equal(v1.ReasonInvalidCredentials))
	})

	It("retries a refused rotation on the timer, not in a hot loop", func() {
		cluster := createDefaultCluster()
		serveCluster(cluster)
		fetchAdminPassword(cluster)
		seedClusterUser("changed-in-the-admin-app")

		requestRotation(cluster, "round-1")
		expectCondition(cluster, v1.ConditionAdminSecretReady, Equal(v1.ReasonInvalidCredentials))

		By("counting the calls of a nine second window")
		before := users.UpdateCalls()
		time.Sleep(9 * time.Second)
		calls := users.UpdateCalls() - before

		// A coarse backstop only. The rate moves with the load of the suite,
		// from about 16 calls alone to about 28 beside the rest, so it
		// cannot separate a timer from one extra reconcile per retry. It
		// separates a timer from a spin, which is unbounded. The precise
		// guarantee belongs to `stages an unchanged failure as the condition
		// the server already holds`, which needs no clock.
		Expect(calls).To(
			BeNumerically("<=", 80), "the failed rotation is spinning instead of waiting for its timer",
		)
	})

	It("reports ConnectionFailed while the cluster does not answer, and retries", func() {
		// The namespace registers no user API, so the cluster answers nothing.
		cluster := createDefaultCluster()
		password := fetchAdminPassword(cluster)

		requestRotation(cluster, "round-1")
		expectCondition(cluster, v1.ConditionAdminSecretReady, Equal(v1.ReasonConnectionFailed))

		var secret corev1.Secret
		Expect(k8sClient.Get(ctx, adminSecretKey(cluster), &secret)).To(Succeed())
		Expect(string(secret.Data[components.AdminPasswordKey])).To(Equal(password))

		By("completing the rotation once the cluster answers")
		seedClusterUser(password)
		serveCluster(cluster)
		expectRotated(cluster, "round-1")
	})

	It("stages an unchanged failure as the condition the server already holds", func() {
		stamped := metav1.NewTime(time.Now().Add(-time.Hour).Truncate(time.Second))
		cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: "cc-same", Generation: 3}}
		failure := &rotationFailure{
			reason: v1.ReasonConnectionFailed, message: "dial tcp 10.0.0.1:8080: i/o timeout",
		}
		prior := &metav1.Condition{
			Type:               v1.ConditionAdminSecretReady,
			Status:             metav1.ConditionFalse,
			Reason:             failure.reason,
			Message:            conditions.BoundMessage(failure.message),
			ObservedGeneration: 3,
			LastTransitionTime: stamped,
		}

		meta.SetStatusCondition(cluster.GetStatusConditions(), metav1.Condition{
			Type:   v1.ConditionAdminSecretReady,
			Status: metav1.ConditionTrue,
			Reason: string(component.Healthy),
		})
		adminCredential{failure: failure}.stageFailure(cluster, prior)

		// Identical to the server copy means the flush writes nothing, which
		// is what keeps the retry on its timer instead of enqueuing itself.
		Expect(meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionAdminSecretReady)).To(
			Equal(prior), "an unchanged failure must stage no change at all",
		)
	})

	It("keeps the transition time of a failure that only changed its message", func() {
		stamped := metav1.NewTime(time.Now().Add(-time.Hour).Truncate(time.Second))
		cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: "cc-cond", Generation: 3}}
		prior := &metav1.Condition{
			Type:               v1.ConditionAdminSecretReady,
			Status:             metav1.ConditionFalse,
			Reason:             v1.ReasonConnectionFailed,
			Message:            "dial tcp 10.0.0.1:8080: i/o timeout",
			ObservedGeneration: 3,
			LastTransitionTime: stamped,
		}

		// The Secret component reports the unchanged Secret as healthy on
		// every reconcile, which is what makes the staged failure a flip.
		meta.SetStatusCondition(cluster.GetStatusConditions(), metav1.Condition{
			Type:   v1.ConditionAdminSecretReady,
			Status: metav1.ConditionTrue,
			Reason: string(component.Healthy),
		})

		cred := adminCredential{failure: &rotationFailure{
			reason: v1.ReasonConnectionFailed, message: "dial tcp 10.0.0.2:8080: i/o timeout",
		}}
		cred.stageFailure(cluster, prior)

		cond := meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionAdminSecretReady)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Message).To(ContainSubstring("10.0.0.2"), "the newest answer is reported")
		Expect(cond.LastTransitionTime).To(
			Equal(stamped), "the condition never left False, so it never transitioned",
		)
	})

	It("sets a changed admin email on the cluster without touching the password", func() {
		ns := newNamespace()
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		cluster.Spec.Auth = &v1.ClusterAuthSpec{
			Basic: &v1.BasicAuthSpec{AdminEmail: "first@example.com"},
		}
		createCluster(cluster)
		serveCluster(cluster)

		password := fetchAdminPassword(cluster)
		seedClusterUser(password)
		Eventually(func(g Gomega) {
			var secret corev1.Secret
			g.Expect(k8sClient.Get(ctx, adminSecretKey(cluster), &secret)).To(Succeed())
			g.Expect(string(secret.Data[components.AdminEmailKey])).To(Equal("first@example.com"))
			g.Expect(string(secret.Data[components.AdminAppliedEmailKey])).To(Equal("first@example.com"))
		}, timeout, interval).Should(Succeed())

		By("asking for another address")
		updateCluster(cluster, func(c *v1.CamundaCluster) {
			c.Spec.Auth = &v1.ClusterAuthSpec{Basic: &v1.BasicAuthSpec{AdminEmail: "second@example.com"}}
		})

		Eventually(func(g Gomega) {
			_, email := users.Profile(components.AdminUsername)
			g.Expect(email).To(Equal("second@example.com"), "the cluster holds the new address")

			var secret corev1.Secret
			g.Expect(k8sClient.Get(ctx, adminSecretKey(cluster), &secret)).To(Succeed())
			g.Expect(string(secret.Data[components.AdminAppliedEmailKey])).To(Equal("second@example.com"))
			g.Expect(string(secret.Data[components.AdminPasswordKey])).To(
				Equal(password), "a profile update never rotates the password",
			)
		}, timeout, interval).Should(Succeed())

		Expect(users.Password(components.AdminUsername)).To(Equal(password))
		expectEvent(cluster, "AdminProfileUpdated", corev1.EventTypeNormal)
	})

	It("keeps the applied admin email when the cluster refuses the new one", func() {
		ns := newNamespace()
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		cluster.Spec.Auth = &v1.ClusterAuthSpec{
			Basic: &v1.BasicAuthSpec{AdminEmail: "first@example.com"},
		}
		createCluster(cluster)
		serveCluster(cluster)

		password := fetchAdminPassword(cluster)
		seedClusterUser(password)
		expectCondition(cluster, v1.ConditionAdminSecretReady, Equal(string(component.Healthy)))

		By("asking for an address that the cluster refuses")
		users.RefuseNext(20, "the provided email is not valid")
		updateCluster(cluster, func(c *v1.CamundaCluster) {
			c.Spec.Auth = &v1.ClusterAuthSpec{Basic: &v1.BasicAuthSpec{AdminEmail: "second@example.com"}}
		})

		expectCondition(cluster, v1.ConditionAdminSecretReady, Equal(v1.ReasonRejected))
		Consistently(func(g Gomega) {
			var secret corev1.Secret
			g.Expect(k8sClient.Get(ctx, adminSecretKey(cluster), &secret)).To(Succeed())
			g.Expect(string(secret.Data[components.AdminAppliedEmailKey])).To(
				Equal("first@example.com"), "an address the cluster refused is never recorded as applied",
			)
			g.Expect(string(secret.Data[components.AdminEmailKey])).To(
				Equal("second@example.com"), "the processes still read a complete seed",
			)
		}, "3s", interval).Should(Succeed())
	})

	It("does not roll the unified processes when a preset asks for the rotation", func() {
		ns := newNamespace()
		preset := &v1.CamundaClusterPreset{
			ObjectMeta: metav1.ObjectMeta{Name: "ccp-" + utilrand.String(8)},
			Spec: v1.CamundaClusterPresetSpec{Cluster: v1.CamundaClusterSpec{
				Version: "8.9.9",
				Auth:    &v1.ClusterAuthSpec{Basic: &v1.BasicAuthSpec{PasswordRotation: "round-1"}},
			}},
		}
		Expect(k8sClient.Create(ctx, preset)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, preset) })

		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		cluster.Spec.PresetRef = preset.Name
		createCluster(cluster)
		serveCluster(cluster)

		zeebeKey := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-zeebe"}
		before := fetchStatefulSet(zeebeKey).Spec.Template.Annotations[components.ConfigHashAnnotation]
		Expect(before).NotTo(BeEmpty())

		By("asking for another rotation on the preset")
		Eventually(func(g Gomega) {
			var latest v1.CamundaClusterPreset
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(preset), &latest)).To(Succeed())
			latest.Spec.Cluster.Auth.Basic.PasswordRotation = "round-2"
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		By("keeping the brokers on their configuration")
		Consistently(func(g Gomega) {
			var sts appsv1.StatefulSet
			g.Expect(k8sClient.Get(ctx, zeebeKey, &sts)).To(Succeed())
			g.Expect(sts.Spec.Template.Annotations[components.ConfigHashAnnotation]).To(
				Equal(before), "a rotation renders nothing for the brokers",
			)
		}, "5s", interval).Should(Succeed())

		By("rolling them for a preset change that does render")
		Eventually(func(g Gomega) {
			var latest v1.CamundaClusterPreset
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(preset), &latest)).To(Succeed())
			latest.Spec.Cluster.PodLabels = map[string]string{"touch": "1"}
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Eventually(func(g Gomega) {
			var sts appsv1.StatefulSet
			g.Expect(k8sClient.Get(ctx, zeebeKey, &sts)).To(Succeed())
			g.Expect(sts.Spec.Template.Annotations[components.ConfigHashAnnotation]).NotTo(Equal(before))
		}, timeout, interval).Should(Succeed())
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
