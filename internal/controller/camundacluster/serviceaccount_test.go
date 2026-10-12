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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

const (
	roleCamunda = "arn:aws:iam::123456789012:role/camunda"
	roleLater   = "arn:aws:iam::123456789012:role/later"
)

// expectObservedGeneration polls until the cluster reports the generation it
// has now as processed, which proves that a reconcile read the latest spec.
func expectObservedGeneration(cluster *v1.CamundaCluster) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		g.Expect(latest.Status.ObservedGeneration).To(Equal(latest.Generation))
	}, timeout, interval).Should(Succeed())
}

// ownedByCluster reports whether account carries an owner reference to the
// cluster named name.
func ownedByCluster(account *corev1.ServiceAccount, name string) bool {
	for _, ref := range account.OwnerReferences {
		if ref.Kind == "CamundaCluster" && ref.Name == name {
			return true
		}
	}
	return false
}

// expectAccountOwned polls until the account named key carries the
// controller owner reference of the cluster and the annotation value.
func expectAccountOwned(key client.ObjectKey, cluster *v1.CamundaCluster, annotation string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var account corev1.ServiceAccount
		g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
		g.Expect(metav1.GetControllerOf(&account)).NotTo(BeNil())
		g.Expect(metav1.GetControllerOf(&account).Name).To(Equal(cluster.Name))
		g.Expect(account.Annotations).To(HaveKeyWithValue(v1.IRSARoleARNAnnotation, annotation))
	}, timeout, interval).Should(Succeed())
}

// takeOverAccount plays the user who takes over a released account: it
// replaces the annotations and adds a label of its own.
func takeOverAccount(key client.ObjectKey) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var account corev1.ServiceAccount
		g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
		account.Annotations = map[string]string{"example.com/team": "user"}
		if account.Labels == nil {
			account.Labels = map[string]string{}
		}
		account.Labels["example.com/team"] = "user"
		g.Expect(k8sClient.Update(ctx, &account)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// expectReleasedAndUnchanged checks that the account named key stays without
// an owner reference to the cluster and keeps what the user set.
func expectReleasedAndUnchanged(key client.ObjectKey, cluster *v1.CamundaCluster) {
	GinkgoHelper()
	Consistently(func(g Gomega) {
		var account corev1.ServiceAccount
		g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
		g.Expect(ownedByCluster(&account, cluster.Name)).To(BeFalse())
		g.Expect(account.Annotations).To(Equal(map[string]string{"example.com/team": "user"}))
		g.Expect(account.Labels).To(HaveKeyWithValue("example.com/team", "user"))
	}, 2*time.Second, interval).Should(Succeed())
}

var _ = Describe("CamundaCluster ServiceAccount release", func() {
	// create: false says that the user owns the account. An owner reference
	// that stays makes the garbage collector delete the account with the
	// cluster.
	It("releases the cluster account when create switches to false, and adopts it back", func() {
		ns := newNamespace()
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{
			Annotations: map[string]string{v1.IRSARoleARNAnnotation: roleCamunda},
		}
		createCluster(cluster)
		key := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-camunda"}
		expectAccountOwned(key, cluster, roleCamunda)

		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.ServiceAccount.Create = new(false) })
		Eventually(func(g Gomega) {
			var account corev1.ServiceAccount
			g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
			g.Expect(ownedByCluster(&account, cluster.Name)).To(BeFalse())
		}, timeout, interval).Should(Succeed())

		// A spec change that the operator processed proves that it read the
		// spec and still left the account of the user alone.
		takeOverAccount(key)
		updateCluster(cluster, func(c *v1.CamundaCluster) {
			c.Spec.ServiceAccount.Annotations[v1.IRSARoleARNAnnotation] = roleLater
		})
		expectObservedGeneration(cluster)
		expectReleasedAndUnchanged(key, cluster)

		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.ServiceAccount.Create = nil })
		expectAccountOwned(key, cluster, roleLater)
	})

	It("releases the connectors account when create switches to false, and adopts it back", func() {
		ns := newNamespace()
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		cluster.Spec.Connectors = &v1.ConnectorsSpec{
			Enabled: new(true),
			Version: "8.9.7",
			ServiceAccount: &v1.ServiceAccountSpec{
				Annotations: map[string]string{v1.IRSARoleARNAnnotation: roleCamunda},
			},
		}
		createCluster(cluster)
		key := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-connectors"}
		expectAccountOwned(key, cluster, roleCamunda)

		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.Connectors.ServiceAccount.Create = new(false) })
		Eventually(func(g Gomega) {
			var account corev1.ServiceAccount
			g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
			g.Expect(ownedByCluster(&account, cluster.Name)).To(BeFalse())
		}, timeout, interval).Should(Succeed())

		takeOverAccount(key)
		updateCluster(cluster, func(c *v1.CamundaCluster) {
			c.Spec.Connectors.ServiceAccount.Annotations[v1.IRSARoleARNAnnotation] = roleLater
		})
		expectObservedGeneration(cluster)
		expectReleasedAndUnchanged(key, cluster)

		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.Connectors.ServiceAccount.Create = nil })
		expectAccountOwned(key, cluster, roleLater)
	})

	// Disabled connectors run no pod, but the account that the operator
	// created for them is still the user's once create is false.
	It("releases the connectors account when connectors are disabled in the same edit", func() {
		ns := newNamespace()
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		cluster.Spec.Connectors = &v1.ConnectorsSpec{
			Enabled: new(true),
			Version: "8.9.7",
			ServiceAccount: &v1.ServiceAccountSpec{
				Annotations: map[string]string{v1.IRSARoleARNAnnotation: roleCamunda},
			},
		}
		createCluster(cluster)
		key := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-connectors"}
		expectAccountOwned(key, cluster, roleCamunda)

		updateCluster(cluster, func(c *v1.CamundaCluster) {
			c.Spec.Connectors.Enabled = new(false)
			c.Spec.Connectors.ServiceAccount.Create = new(false)
		})
		Eventually(func(g Gomega) {
			var account corev1.ServiceAccount
			g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
			g.Expect(ownedByCluster(&account, cluster.Name)).To(BeFalse())
		}, timeout, interval).Should(Succeed())
	})

	// The operator reads an account of the user to see that it exists, and
	// never writes it.
	It("does not write an account that it never created", func() {
		ns := newNamespace()
		key := client.ObjectKey{Namespace: ns, Name: "platform-sa"}
		Expect(k8sClient.Create(ctx, &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: key.Name},
		})).To(Succeed())
		var created corev1.ServiceAccount
		Expect(k8sClient.Get(ctx, key, &created)).To(Succeed())

		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{
			Name:        key.Name,
			Create:      new(false),
			Annotations: map[string]string{v1.IRSARoleARNAnnotation: roleCamunda},
		}
		createCluster(cluster)
		expectObservedGeneration(cluster)
		updateCluster(cluster, func(c *v1.CamundaCluster) {
			c.Spec.ServiceAccount.Annotations[v1.IRSARoleARNAnnotation] = roleLater
		})
		expectObservedGeneration(cluster)

		Consistently(func(g Gomega) {
			var account corev1.ServiceAccount
			g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
			g.Expect(account.ResourceVersion).To(Equal(created.ResourceVersion))
		}, 2*time.Second, interval).Should(Succeed())
	})

	// The derived name belongs to the operator, which deletes an account of
	// that name while the cluster uses none. Connectors that name it as an
	// account of the user would lose it, so the cluster refuses them before
	// it applies anything.
	DescribeTable(
		"refuses connectors that name the reserved cluster account",
		func(enabled bool) {
			ns := newNamespace()
			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			key := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-camunda"}
			Expect(k8sClient.Create(ctx, &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: key.Name},
			})).To(Succeed())
			var created corev1.ServiceAccount
			Expect(k8sClient.Get(ctx, key, &created)).To(Succeed())

			cluster.Spec.Connectors = &v1.ConnectorsSpec{
				Enabled:        new(enabled),
				Version:        "8.9.7",
				ServiceAccount: &v1.ServiceAccountSpec{Name: key.Name, Create: new(false)},
			}
			createCluster(cluster)
			expectReady(
				cluster,
				metav1.ConditionFalse,
				Equal(v1.ReasonInvalidReference),
				ContainSubstring("reserved"),
			)

			Consistently(func(g Gomega) {
				var account corev1.ServiceAccount
				g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
				g.Expect(account.UID).To(Equal(created.UID))
			}, 2*time.Second, interval).Should(Succeed())
		},
		Entry("with connectors enabled", true),
		Entry("with connectors disabled", false),
	)
})
