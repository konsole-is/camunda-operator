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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
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

// createUserAccount creates the ServiceAccount named key without an owner, as a
// user does, and returns it as created.
func createUserAccount(key client.ObjectKey) corev1.ServiceAccount {
	GinkgoHelper()
	Expect(k8sClient.Create(ctx, &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{Namespace: key.Namespace, Name: key.Name},
	})).To(Succeed())
	var created corev1.ServiceAccount
	Expect(k8sClient.Get(ctx, key, &created)).To(Succeed())
	return created
}

// expectUserAccountKept checks that the account named key stays the object
// that the user created, without an owner reference to the cluster.
func expectUserAccountKept(key client.ObjectKey, created corev1.ServiceAccount, cluster *v1.CamundaCluster) {
	GinkgoHelper()
	Consistently(func(g Gomega) {
		var account corev1.ServiceAccount
		g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
		g.Expect(account.UID).To(Equal(created.UID))
		g.Expect(ownedByCluster(&account, cluster.Name)).To(BeFalse())
	}, 2*time.Second, interval).Should(Succeed())
}

// expectPublishedAccount polls until the cluster publishes name as the
// ServiceAccount its pods run under. An empty name is a cluster whose pods
// run under the default account of the namespace.
func expectPublishedAccount(cluster *v1.CamundaCluster, name string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		g.Expect(latest.Status.ServiceAccountName).To(Equal(name))
	}, timeout, interval).Should(Succeed())
}

// expectPublishedConnectorsAccount polls until the cluster publishes name as
// the ServiceAccount its connectors run under. An empty name is the default
// account of the namespace.
func expectPublishedConnectorsAccount(cluster *v1.CamundaCluster, name string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		g.Expect(latest.Status.ConnectorsServiceAccountName).To(Equal(name))
	}, timeout, interval).Should(Succeed())
}

// expectDeploymentAccount polls until the Deployment named key runs its pods
// under account.
func expectDeploymentAccount(key client.ObjectKey, account string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var deployment appsv1.Deployment
		g.Expect(k8sClient.Get(ctx, key, &deployment)).To(Succeed())
		g.Expect(deployment.Spec.Template.Spec.ServiceAccountName).To(Equal(account))
	}, timeout, interval).Should(Succeed())
}

// statefulSetAccountHash polls until the StatefulSet named key carries a
// ServiceAccount hash on its pod template, and returns it.
func statefulSetAccountHash(key client.ObjectKey) string {
	GinkgoHelper()
	var hash string
	Eventually(func(g Gomega) {
		var sts appsv1.StatefulSet
		g.Expect(k8sClient.Get(ctx, key, &sts)).To(Succeed())
		hash = sts.Spec.Template.Annotations[components.ServiceAccountHashAnnotation]
		g.Expect(hash).NotTo(BeEmpty())
	}, timeout, interval).Should(Succeed())
	return hash
}

// deploymentAccountHash polls until the Deployment named key carries a
// ServiceAccount hash on its pod template, and returns it.
func deploymentAccountHash(key client.ObjectKey) string {
	GinkgoHelper()
	var hash string
	Eventually(func(g Gomega) {
		var deployment appsv1.Deployment
		g.Expect(k8sClient.Get(ctx, key, &deployment)).To(Succeed())
		hash = deployment.Spec.Template.Annotations[components.ServiceAccountHashAnnotation]
		g.Expect(hash).NotTo(BeEmpty())
	}, timeout, interval).Should(Succeed())
	return hash
}

var _ = Describe("CamundaCluster ServiceAccount wiring", func() {
	Describe("the connectors ServiceAccount", func() {
		connectors := func() *v1.ConnectorsSpec {
			return &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"}
		}

		It("keeps the connectors off the account of the cluster", func() {
			ns := newNamespace()
			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Name: "platform-sa"}
			cluster.Spec.Connectors = connectors()
			createCluster(cluster)

			expectPublishedAccount(cluster, "platform-sa")
			expectPublishedConnectorsAccount(cluster, "")
			expectDeploymentAccount(client.ObjectKey{Namespace: ns, Name: cluster.Name + "-gateway"}, "platform-sa")
			expectDeploymentAccount(client.ObjectKey{Namespace: ns, Name: cluster.Name + "-connectors"}, "default")
		})

		// A pre-existing account is a reference: its absence fails fast
		// instead of leaving the connectors pods unschedulable.
		It("requires a pre-existing connectors account to exist", func() {
			ns := newNamespace()
			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			cluster.Spec.Connectors = connectors()
			cluster.Spec.Connectors.ServiceAccount = &v1.ServiceAccountSpec{
				Name:   "platform-connectors",
				Create: new(false),
			}
			createCluster(cluster)

			expectReady(
				cluster,
				metav1.ConditionFalse,
				Equal(v1.ReasonInvalidReference),
				And(
					ContainSubstring("platform-connectors"),
					ContainSubstring("connectors.serviceAccount.create is false"),
				),
			)

			Expect(k8sClient.Create(ctx, &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "platform-connectors"},
			})).To(Succeed())

			// Nothing watches the foreign account, and a failed pre-check
			// publishes no account, so the published name proves that the
			// pre-check came back on its own and passed.
			expectPublishedConnectorsAccount(cluster, "platform-connectors")
			expectDeploymentAccount(
				client.ObjectKey{Namespace: ns, Name: cluster.Name + "-connectors"},
				"platform-connectors",
			)

			var account corev1.ServiceAccount
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: "platform-connectors"}, &account)).
				To(Succeed())
			Expect(account.OwnerReferences).To(BeEmpty())
		})

		// The schema refuses two equal names of the spec, so only the derived
		// name of the cluster account reaches the pre-check.
		It("refuses a connectors account that is the cluster account", func() {
			ns := newNamespace()
			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{}
			cluster.Spec.Connectors = connectors()
			cluster.Spec.Connectors.ServiceAccount = &v1.ServiceAccountSpec{Name: cluster.Name + "-camunda"}
			createCluster(cluster)

			expectReady(
				cluster,
				metav1.ConditionFalse,
				Equal(v1.ReasonInvalidReference),
				ContainSubstring("connectors.serviceAccount.name"),
			)
		})

		It("merges the connectors account annotations of the preset", func() {
			ns := newNamespace()
			preset := minimalPreset()
			preset.Spec.Cluster.Connectors = &v1.ConnectorsSpec{
				ServiceAccount: &v1.ServiceAccountSpec{Annotations: map[string]string{
					v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/preset-connectors",
					"example.com/team":       "preset",
				}},
			}
			Expect(k8sClient.Create(ctx, preset)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, preset) })

			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			cluster.Spec.PresetRef = preset.Name
			cluster.Spec.Connectors = connectors()
			cluster.Spec.Connectors.ServiceAccount = &v1.ServiceAccountSpec{Annotations: map[string]string{
				"example.com/team": "cluster",
			}}
			createCluster(cluster)

			expectPublishedConnectorsAccount(cluster, cluster.Name+"-connectors")
			Eventually(func(g Gomega) {
				var account corev1.ServiceAccount
				key := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-connectors"}
				g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
				g.Expect(account.Annotations).To(HaveKeyWithValue(
					v1.IRSARoleARNAnnotation, "arn:aws:iam::123456789012:role/preset-connectors",
				))
				g.Expect(account.Annotations).To(HaveKeyWithValue("example.com/team", "cluster"))
			}, timeout, interval).Should(Succeed())
		})

		// Without the block the operator names no connectors account, so an
		// account with the derived name is not its own to delete.
		It("leaves an account of the user with the derived name in place", func() {
			ns := newNamespace()
			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			cluster.Spec.Connectors = connectors()
			key := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-connectors"}
			Expect(k8sClient.Create(ctx, &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: key.Name},
			})).To(Succeed())
			createCluster(cluster)

			expectDeploymentAccount(key, "default")
			Consistently(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, &corev1.ServiceAccount{})).To(Succeed())
			}, 2*time.Second, interval).Should(Succeed())
		})

		// Disabled connectors run no pod, so the operator names no account
		// for them, also when the block is set.
		It("leaves an account with the derived name in place while connectors are off", func() {
			ns := newNamespace()
			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			cluster.Spec.Connectors = &v1.ConnectorsSpec{
				Enabled:        new(false),
				ServiceAccount: &v1.ServiceAccountSpec{},
			}
			key := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-connectors"}
			Expect(k8sClient.Create(ctx, &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: key.Name},
			})).To(Succeed())
			createCluster(cluster)

			expectDeploymentAccount(client.ObjectKey{Namespace: ns, Name: cluster.Name + "-gateway"}, "")
			Consistently(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, &corev1.ServiceAccount{})).To(Succeed())
			}, 2*time.Second, interval).Should(Succeed())
		})

		It("moves the connectors back to the default account when the block goes", func() {
			ns := newNamespace()
			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			cluster.Spec.Connectors = connectors()
			cluster.Spec.Connectors.ServiceAccount = &v1.ServiceAccountSpec{}
			createCluster(cluster)

			key := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-connectors"}
			expectDeploymentAccount(key, cluster.Name+"-connectors")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(ctx, key, &corev1.ServiceAccount{})).To(Succeed())
			}, timeout, interval).Should(Succeed())

			updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.Connectors.ServiceAccount = nil })

			expectPublishedConnectorsAccount(cluster, "")
			expectDeploymentAccount(key, "default")

			// The account stays with the cluster and goes when the cluster
			// goes.
			var account corev1.ServiceAccount
			Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
			Expect(metav1.GetControllerOf(&account)).NotTo(BeNil())
			Expect(metav1.GetControllerOf(&account).Name).To(Equal(cluster.Name))
		})

		// Workload identity reads the annotations of the account when a pod
		// starts, so the pods of the changed account restart, and only those.
		It("rolls the pods of an account whose annotations change", func() {
			ns := newNamespace()
			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Annotations: map[string]string{
				v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/old",
			}}
			cluster.Spec.Connectors = connectors()
			cluster.Spec.Connectors.ServiceAccount = &v1.ServiceAccountSpec{Annotations: map[string]string{
				v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/connectors",
			}}
			createCluster(cluster)

			zeebe := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-zeebe"}
			gateway := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-gateway"}
			connectorsKey := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-connectors"}
			zeebeHash := statefulSetAccountHash(zeebe)
			gatewayHash := deploymentAccountHash(gateway)
			deploymentAccountHash(connectorsKey)
			var connectorsBefore appsv1.Deployment
			Expect(k8sClient.Get(ctx, connectorsKey, &connectorsBefore)).To(Succeed())

			updateCluster(cluster, func(c *v1.CamundaCluster) {
				c.Spec.ServiceAccount.Annotations[v1.IRSARoleARNAnnotation] = "arn:aws:iam::123456789012:role/new"
			})

			Eventually(func(g Gomega) {
				var sts appsv1.StatefulSet
				g.Expect(k8sClient.Get(ctx, zeebe, &sts)).To(Succeed())
				g.Expect(sts.Spec.Template.Annotations[components.ServiceAccountHashAnnotation]).
					NotTo(Equal(zeebeHash))
				var deployment appsv1.Deployment
				g.Expect(k8sClient.Get(ctx, gateway, &deployment)).To(Succeed())
				g.Expect(deployment.Spec.Template.Annotations[components.ServiceAccountHashAnnotation]).
					NotTo(Equal(gatewayHash))
			}, timeout, interval).Should(Succeed())

			// A backup Job starts its pod under the published account, which
			// already carries the new annotation.
			expectPublishedAccount(cluster, cluster.Name+"-camunda")
			var account corev1.ServiceAccount
			Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: ns, Name: cluster.Name + "-camunda"}, &account)).
				To(Succeed())
			Expect(account.Annotations).To(HaveKeyWithValue(
				v1.IRSARoleARNAnnotation, "arn:aws:iam::123456789012:role/new",
			))

			Consistently(func(g Gomega) {
				var connectorsNow appsv1.Deployment
				g.Expect(k8sClient.Get(ctx, connectorsKey, &connectorsNow)).To(Succeed())
				g.Expect(connectorsNow.Spec.Template).To(Equal(connectorsBefore.Spec.Template))
			}, 2*time.Second, interval).Should(Succeed())
		})

		It("rolls only the connectors when their account annotations change", func() {
			ns := newNamespace()
			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Annotations: map[string]string{
				v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/camunda",
			}}
			cluster.Spec.Connectors = connectors()
			cluster.Spec.Connectors.ServiceAccount = &v1.ServiceAccountSpec{Annotations: map[string]string{
				v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/old",
			}}
			createCluster(cluster)

			zeebe := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-zeebe"}
			gateway := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-gateway"}
			connectorsKey := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-connectors"}
			statefulSetAccountHash(zeebe)
			deploymentAccountHash(gateway)
			connectorsHash := deploymentAccountHash(connectorsKey)
			var zeebeBefore appsv1.StatefulSet
			Expect(k8sClient.Get(ctx, zeebe, &zeebeBefore)).To(Succeed())
			var gatewayBefore appsv1.Deployment
			Expect(k8sClient.Get(ctx, gateway, &gatewayBefore)).To(Succeed())

			updateCluster(cluster, func(c *v1.CamundaCluster) {
				c.Spec.Connectors.ServiceAccount.Annotations[v1.IRSARoleARNAnnotation] =
					"arn:aws:iam::123456789012:role/new"
			})

			Eventually(func(g Gomega) {
				var deployment appsv1.Deployment
				g.Expect(k8sClient.Get(ctx, connectorsKey, &deployment)).To(Succeed())
				g.Expect(deployment.Spec.Template.Annotations[components.ServiceAccountHashAnnotation]).
					NotTo(Equal(connectorsHash))
			}, timeout, interval).Should(Succeed())

			Consistently(func(g Gomega) {
				var sts appsv1.StatefulSet
				g.Expect(k8sClient.Get(ctx, zeebe, &sts)).To(Succeed())
				g.Expect(sts.Spec.Template).To(Equal(zeebeBefore.Spec.Template))
				var deployment appsv1.Deployment
				g.Expect(k8sClient.Get(ctx, gateway, &deployment)).To(Succeed())
				g.Expect(deployment.Spec.Template).To(Equal(gatewayBefore.Spec.Template))
			}, 2*time.Second, interval).Should(Succeed())
		})
	})

	// A consumer that renders a pod against the cluster reads this field.
	// Rebuilding the rule from the spec and every bucket the cluster
	// references is what shipped a dump Job that named an account the
	// cluster never rendered.
	Describe("the published ServiceAccount", func() {
		It("names the account of the spec", func() {
			ns := newNamespace()
			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Name: "platform-sa"}
			createCluster(cluster)

			expectPublishedAccount(cluster, "platform-sa")
		})

		// A workload-identity bucket binds the account by name on the cloud
		// side, so the cluster renders one even though the spec asks for
		// none.
		It("names the derived account of a workload-identity bucket", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			setSnapshotRepository(binding, "my-repository")
			cluster := newCluster(ns, createPlatformConfig(), binding)
			cluster.Spec.BackupStorageRef = createBucket(ns, "arn:aws:iam::123456789012:role/camunda").Name
			createCluster(cluster)

			expectPublishedAccount(cluster, cluster.Name+"-camunda")
		})

		// Static keys reach the pods in a Secret, so the pods carry no cloud
		// identity and the operator renders no account. A consumer that named
		// one here would name an account that does not exist.
		It("is empty for a bucket with static credentials", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			setSnapshotRepository(binding, "my-repository")
			keys := createSecret(ns, "minio-keys", map[string]string{
				"accessKeyId": "minio", "secretAccessKey": "minio123",
			})
			cluster := newCluster(ns, createPlatformConfig(), binding)
			cluster.Spec.BackupStorageRef = createStaticBucket(ns, keys.Name).Name
			createCluster(cluster)

			// The binding proves the cluster reconciled. Without it an empty
			// account would also match a status that was never written.
			expectBinding(cluster, func(g Gomega, published *v1.ManagementBinding) {
				g.Expect(published).NotTo(BeNil())
			})
			expectPublishedAccount(cluster, "")
		})

		// The account outlives a suspension: the ServiceAccount stays, and a
		// cleanup Job of a deleted backup still needs the identity it holds.
		It("stays published while the cluster is suspended", func() {
			ns := newNamespace()
			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Name: "platform-sa"}
			createCluster(cluster)
			expectPublishedAccount(cluster, "platform-sa")

			updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.Suspend = true })
			expectBinding(cluster, func(g Gomega, published *v1.ManagementBinding) {
				g.Expect(published).To(BeNil())
			})
			expectPublishedAccount(cluster, "platform-sa")
		})
	})
})

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

	It("deletes the account that it created once the cluster no longer uses it", func() {
		ns := newNamespace()
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{
			Annotations: map[string]string{v1.IRSARoleARNAnnotation: roleCamunda},
		}
		createCluster(cluster)
		key := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-camunda"}
		expectAccountOwned(key, cluster, roleCamunda)

		updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.ServiceAccount = nil })

		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, key, &corev1.ServiceAccount{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())
	})

	// The cluster deletes only an account that it owns, so an account of the
	// user with the derived name survives while the cluster uses none.
	It("keeps an account of the user with the derived name while the cluster uses none", func() {
		ns := newNamespace()
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		key := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-camunda"}
		created := createUserAccount(key)
		createCluster(cluster)

		// The gateway is applied after the brokers, whose component deletes
		// the account it does not use.
		expectDeploymentAccount(client.ObjectKey{Namespace: ns, Name: cluster.Name + "-gateway"}, "")
		expectPublishedAccount(cluster, "")
		expectUserAccountKept(key, created, cluster)
	})

	It("takes over an account of the user with the derived name once the cluster uses it", func() {
		ns := newNamespace()
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{
			Annotations: map[string]string{v1.IRSARoleARNAnnotation: roleCamunda},
		}
		key := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-camunda"}
		created := createUserAccount(key)
		createCluster(cluster)

		expectAccountOwned(key, cluster, roleCamunda)
		var account corev1.ServiceAccount
		Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
		Expect(account.UID).To(Equal(created.UID))
	})

	// Connectors that are off run no pod, and the cluster that uses no account
	// deletes none that it does not own, so the spec is accepted.
	It("accepts disabled connectors that name an account of the user with the derived cluster name", func() {
		ns := newNamespace()
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		key := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-camunda"}
		created := createUserAccount(key)
		cluster.Spec.Connectors = &v1.ConnectorsSpec{
			Enabled:        new(false),
			Version:        "8.9.7",
			ServiceAccount: &v1.ServiceAccountSpec{Name: key.Name, Create: new(false)},
		}
		createCluster(cluster)

		// A failed pre-check applies nothing, so the gateway proves that the
		// spec passed it.
		expectDeploymentAccount(client.ObjectKey{Namespace: ns, Name: cluster.Name + "-gateway"}, "")
		expectUserAccountKept(key, created, cluster)
	})

	// Enabled connectors would run under the cluster identity as soon as the
	// cluster uses its account.
	It("refuses enabled connectors that name an account of the user with the derived cluster name", func() {
		ns := newNamespace()
		cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
		key := client.ObjectKey{Namespace: ns, Name: cluster.Name + "-camunda"}
		created := createUserAccount(key)
		cluster.Spec.Connectors = &v1.ConnectorsSpec{
			Enabled:        new(true),
			Version:        "8.9.7",
			ServiceAccount: &v1.ServiceAccountSpec{Name: key.Name, Create: new(false)},
		}
		createCluster(cluster)

		expectReady(
			cluster,
			metav1.ConditionFalse,
			Equal(v1.ReasonInvalidReference),
			ContainSubstring("is the ServiceAccount of the cluster"),
		)
		expectUserAccountKept(key, created, cluster)
	})
})
