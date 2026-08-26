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
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
)

// createBucket creates an S3 ObjectStorageConfig with workload identity in
// namespace and registers its deletion. A cluster resolves the reference in
// its own namespace, so the contract belongs next to the cluster. The role is
// unique per bucket unless roleARN is given, so a second bucket can be made to
// conflict.
func createBucket(namespace, roleARN string) *v1.ObjectStorageConfig {
	GinkgoHelper()
	bucket := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "osc-" + utilrand.String(8), Namespace: namespace},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "camunda-backups",
				Region:     "eu-west-1",
				Auth: v1.S3StorageAuth{
					Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
					WorkloadIdentity: &v1.S3WorkloadIdentity{RoleARN: roleARN},
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, bucket)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, bucket) })
	return bucket
}

// createStaticBucket creates an S3 ObjectStorageConfig in namespace that
// authenticates with the static keys of the named Secret, and registers its
// deletion. The Secret resolves in the same namespace as the bucket, so a
// cluster on this bucket carries no cloud identity.
func createStaticBucket(namespace, secretName string) *v1.ObjectStorageConfig {
	GinkgoHelper()
	bucket := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "osc-" + utilrand.String(8), Namespace: namespace},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "camunda-backups",
				Endpoint:   "http://minio.minio.svc:9000",
				Auth: v1.S3StorageAuth{
					Type: v1.ObjectStorageAuthTypeCredentials,
					Credentials: &v1.S3Credentials{SecretRef: v1.S3CredentialsSecretRef{
						Name:               secretName,
						AccessKeyIDKey:     "accessKeyId",
						SecretAccessKeyKey: "secretAccessKey",
					}},
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, bucket)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, bucket) })

	return bucket
}

// setSnapshotRepository puts a snapshot repository name on the storage
// contract, which an Elasticsearch cluster that takes backups needs.
func setSnapshotRepository(binding *v1.SecondaryStorageConfig, name string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.SecondaryStorageConfig
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(binding), &latest)).To(Succeed())
		latest.Spec.Elasticsearch.SnapshotRepository = name
		g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// expectBinding polls until the published management binding satisfies match.
func expectBinding(cluster *v1.CamundaCluster, match func(Gomega, *v1.ManagementBinding)) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		match(g, latest.Status.Management)
	}, timeout, interval).Should(Succeed())
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

var _ = Describe("CamundaCluster backup wiring", func() {
	Describe("the published management binding", func() {
		// The default topology runs a standalone gateway, which hosts the web
		// applications and serves the actuator endpoints of the cluster.
		It("names the endpoint, the version, and the partition count", func() {
			cluster := createDefaultCluster()

			expectBinding(cluster, func(g Gomega, binding *v1.ManagementBinding) {
				g.Expect(binding).NotTo(BeNil())
				g.Expect(binding.Endpoint).To(Equal(
					"http://" + cluster.Name + "-gateway." + cluster.Namespace + ".svc:9600",
				))
				g.Expect(binding.Version).To(Equal("8.9.9"))
				g.Expect(binding.Partitions).To(Equal(int32(1)))
			})
		})

		// With an embedded gateway the brokers serve the same endpoints, so
		// the binding follows the topology instead of naming a Service that
		// the cluster does not render.
		It("names the broker Service when the gateway is embedded", func() {
			ns := newNamespace()
			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			cluster.Spec.Gateway = &v1.GatewaySpec{Mode: v1.ComponentModeEmbedded}
			createCluster(cluster)

			expectBinding(cluster, func(g Gomega, binding *v1.ManagementBinding) {
				g.Expect(binding).NotTo(BeNil())
				g.Expect(binding.Endpoint).To(Equal(
					"http://" + cluster.Name + "-zeebe." + cluster.Namespace + ".svc:9600",
				))
			})
		})

		// Camunda 8.9 leaves the actuator endpoints of the management port
		// unauthenticated, so a consumer needs no credentials to call them.
		It("reports that the management port needs no credentials", func() {
			cluster := createDefaultCluster()

			expectBinding(cluster, func(g Gomega, binding *v1.ManagementBinding) {
				g.Expect(binding).NotTo(BeNil())
				g.Expect(binding.Auth.Method).To(Equal(v1.ManagementAuthMethodNone))
				g.Expect(binding.Auth.CredentialsSecretRef).To(BeNil())
			})
		})

		// A suspended cluster has every workload scaled to zero. A consumer
		// that kept calling the endpoint would report a connection failure
		// instead of a suspended cluster, so the binding goes away.
		It("is cleared while the cluster is suspended, and returns after", func() {
			cluster := createDefaultCluster()
			expectBinding(cluster, func(g Gomega, binding *v1.ManagementBinding) {
				g.Expect(binding).NotTo(BeNil())
			})

			updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.Suspend = true })
			expectBinding(cluster, func(g Gomega, binding *v1.ManagementBinding) {
				g.Expect(binding).To(BeNil())
			})

			updateCluster(cluster, func(c *v1.CamundaCluster) { c.Spec.Suspend = false })
			expectBinding(cluster, func(g Gomega, binding *v1.ManagementBinding) {
				g.Expect(binding).NotTo(BeNil())
			})
		})

		// The distinct name proves that the value comes from this contract,
		// not from the shared fixture name of the other tests.
		// Without a backupStorageRef the components are wired to no
		// repository, so the binding must not name one.
		It("omits the repository when the cluster takes no backups", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			setSnapshotRepository(binding, "repository-of-this-contract")
			cluster := newCluster(ns, createPlatformConfig(), binding)
			createCluster(cluster)

			expectBinding(cluster, func(g Gomega, published *v1.ManagementBinding) {
				g.Expect(published).NotTo(BeNil())
				g.Expect(published.BackupRepository).To(BeEmpty())
			})
		})

		It("carries the snapshot repository of the storage contract", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			setSnapshotRepository(binding, "repository-of-this-contract")
			cluster := newCluster(ns, createPlatformConfig(), binding)
			cluster.Spec.BackupStorageRef = createBucket(ns, "arn:aws:iam::123456789012:role/camunda").Name
			createCluster(cluster)

			expectBinding(cluster, func(g Gomega, published *v1.ManagementBinding) {
				g.Expect(published).NotTo(BeNil())
				g.Expect(published.BackupRepository).To(Equal("repository-of-this-contract"))
			})
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

	Describe("the backup bucket", func() {
		// Without a repository the web applications have nowhere to write
		// their snapshots, so the reference is incomplete rather than the
		// backup silently failing later.
		It("rejects an Elasticsearch cluster whose contract has no snapshot repository", func() {
			ns := newNamespace()
			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			cluster.Spec.BackupStorageRef = createBucket(ns, "arn:aws:iam::123456789012:role/camunda").Name
			createCluster(cluster)

			expectReady(
				cluster,
				metav1.ConditionFalse,
				Equal(v1.ReasonInvalidReference),
				ContainSubstring("elasticsearch.snapshotRepository"),
			)
		})

		// A cluster reads its bucket contract in its own namespace. A
		// contract of the same name next door belongs to another tenant.
		It("does not resolve a bucket of another namespace", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			setSnapshotRepository(binding, "my-repository")
			elsewhere := createBucket(newNamespace(), "arn:aws:iam::123456789012:role/camunda")

			cluster := newCluster(ns, createPlatformConfig(), binding)
			cluster.Spec.BackupStorageRef = elsewhere.Name
			createCluster(cluster)

			expectReady(
				cluster,
				metav1.ConditionFalse,
				Equal(v1.ReasonInvalidReference),
				ContainSubstring(elsewhere.Name),
			)
		})

		// A pod has one ServiceAccount, so the operator cannot honor two
		// identities of one cloud. Failing here is clearer than annotating
		// with whichever bucket was read last.
		It("rejects two buckets that name different identities of one cloud", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			setSnapshotRepository(binding, "my-repository")
			cluster := newCluster(ns, createPlatformConfig(), binding)
			cluster.Spec.BackupStorageRef = createBucket(ns, "arn:aws:iam::123456789012:role/backup").Name
			cluster.Spec.DocumentStorageRef = createBucket(ns, "arn:aws:iam::123456789012:role/documents").Name
			createCluster(cluster)

			expectReady(
				cluster,
				metav1.ConditionFalse,
				Equal(v1.ReasonInvalidReference),
				ContainSubstring("one cluster has one ServiceAccount"),
			)
		})

		// The azure store has no container field: base-path IS the
		// container, so the per-cluster prefix of S3 and GCS does not exist
		// and a second cluster on the contract would write into the first
		// cluster's container. The oldest cluster keeps the contract.
		It("rejects a second cluster on one Azure contract, oldest wins", func() {
			ns := newNamespace()
			bucket := &v1.ObjectStorageConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "osc-" + utilrand.String(8), Namespace: ns},
				Spec: v1.ObjectStorageConfigSpec{
					Type: v1.ObjectStorageTypeAzureBlob,
					AzureBlob: &v1.AzureBlobStorage{
						AccountName: "camundabackups",
						Container:   "backups",
					},
				},
			}
			Expect(k8sClient.Create(ctx, bucket)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, bucket) })

			first := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			first.Spec.BackupStorageRef = bucket.Name
			createCluster(first)

			second := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			second.Spec.BackupStorageRef = bucket.Name
			createCluster(second)

			// The API server stamps creation times with one-second
			// granularity, so which cluster is older is not for the test to
			// say. The invariant is that exactly one yields, and its message
			// names the one that keeps the contract.
			rejected := func(cluster *v1.CamundaCluster, sibling *v1.CamundaCluster) bool {
				var latest v1.CamundaCluster
				if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest); err != nil {
					return false
				}
				ready := meta.FindStatusCondition(latest.Status.Conditions, "Ready")
				return ready != nil && ready.Status == metav1.ConditionFalse &&
					ready.Reason == v1.ReasonInvalidReference &&
					strings.Contains(ready.Message, sibling.Name)
			}
			Eventually(func(g Gomega) {
				firstRejected := rejected(first, second)
				secondRejected := rejected(second, first)
				g.Expect(firstRejected).NotTo(
					Equal(secondRejected),
					"exactly one cluster must yield the contract; first=%t second=%t",
					firstRejected, secondRejected,
				)
			}, timeout, interval).Should(Succeed())
		})

		It("accepts two buckets that name the same identity", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			setSnapshotRepository(binding, "my-repository")
			cluster := newCluster(ns, createPlatformConfig(), binding)
			role := "arn:aws:iam::123456789012:role/camunda"
			cluster.Spec.BackupStorageRef = createBucket(ns, role).Name
			cluster.Spec.DocumentStorageRef = createBucket(ns, role).Name
			createCluster(cluster)

			expectBinding(cluster, func(g Gomega, published *v1.ManagementBinding) {
				g.Expect(published).NotTo(BeNil())
			})
		})

		// A pre-existing ServiceAccount is a reference, not a resource: the
		// operator must not adopt it, because an owned account is deleted
		// with the cluster. Its absence fails fast instead of leaving the
		// pods unschedulable.
		It("requires a pre-existing ServiceAccount to exist", func() {
			ns := newNamespace()
			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{
				Name:   "platform-sa",
				Create: new(false),
			}
			createCluster(cluster)

			expectReady(
				cluster,
				metav1.ConditionFalse,
				Equal(v1.ReasonInvalidReference),
				ContainSubstring("platform-sa"),
			)

			Expect(k8sClient.Create(ctx, &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "platform-sa"},
			})).To(Succeed())

			// Nothing watches the foreign account, so the recovery proves
			// the unwatched pre-check comes back on its own.
			Eventually(func(g Gomega) {
				var latest v1.CamundaCluster
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
				ready := meta.FindStatusCondition(latest.Status.Conditions, "Ready")
				g.Expect(ready).NotTo(BeNil())
				g.Expect(ready.Reason).NotTo(Equal(v1.ReasonInvalidReference))
			}, timeout, interval).Should(Succeed())

			// The operator neither owns nor deletes the foreign account: an
			// excluded resource is no deletion target, so it survives the
			// reconciles that just ran, unannotated and unowned.
			var account corev1.ServiceAccount
			key := client.ObjectKey{Namespace: ns, Name: "platform-sa"}
			Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
			Expect(account.Annotations).NotTo(HaveKey(v1.IRSARoleARNAnnotation))
			Expect(account.OwnerReferences).To(BeEmpty())
		})

		It("renders and annotates the ServiceAccount under a custom name", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			setSnapshotRepository(binding, "my-repository")
			cluster := newCluster(ns, createPlatformConfig(), binding)
			cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Name: "camunda-prod"}
			cluster.Spec.BackupStorageRef = createBucket(ns, "arn:aws:iam::123456789012:role/camunda").Name
			createCluster(cluster)

			Eventually(func(g Gomega) {
				var account corev1.ServiceAccount
				key := client.ObjectKey{Namespace: cluster.Namespace, Name: "camunda-prod"}
				g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
				g.Expect(account.Annotations).To(HaveKeyWithValue(
					v1.IRSARoleARNAnnotation,
					"arn:aws:iam::123456789012:role/camunda",
				))
			}, timeout, interval).Should(Succeed())
		})

		It("annotates the ServiceAccount with the identity of the bucket", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			setSnapshotRepository(binding, "my-repository")
			cluster := newCluster(ns, createPlatformConfig(), binding)
			cluster.Spec.BackupStorageRef = createBucket(ns, "arn:aws:iam::123456789012:role/camunda").Name
			createCluster(cluster)

			Eventually(func(g Gomega) {
				var account corev1.ServiceAccount
				key := client.ObjectKey{
					Namespace: cluster.Namespace,
					Name:      components.ServiceAccountName(cluster, components.NewEffective(cluster.Spec)),
				}
				g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
				g.Expect(account.Annotations).To(HaveKeyWithValue(
					v1.IRSARoleARNAnnotation,
					"arn:aws:iam::123456789012:role/camunda",
				))
			}, timeout, interval).Should(Succeed())
		})

		// Static keys reach the pods directly: a bucket resolves its
		// credentials Secret in its own namespace, which is the cluster
		// namespace, so no copy is involved.
		It("resolves the static credentials of a bucket in the cluster namespace", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			setSnapshotRepository(binding, "my-repository")
			createSecret(ns, "minio-keys", map[string]string{
				"accessKeyId": "minio", "secretAccessKey": "minio123",
			})

			cluster := newCluster(ns, createPlatformConfig(), binding)
			cluster.Spec.BackupStorageRef = createStaticBucket(ns, "minio-keys").Name
			createCluster(cluster)

			stampStatefulSetReady(client.ObjectKey{Namespace: ns, Name: cluster.Name + "-zeebe"})
			stampDeploymentReady(client.ObjectKey{Namespace: ns, Name: cluster.Name + "-gateway"})
			expectReady(cluster, metav1.ConditionTrue, Equal(v1.ReasonHealthy), Not(BeEmpty()))
		})

		// Only dump Jobs consume the backup user of the database, so a
		// dangling reference must not park the whole cluster. The cluster
		// warns here, and the backup that needs it parks in its own pre-check.
		It("warns on dangling dump credentials instead of parking the cluster", func() {
			ns := newNamespace()

			server := fixtures.DatabaseServerConfig(ns)
			Expect(k8sClient.Create(ctx, server)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, server) })

			createSecret(ns, "app-user", map[string]string{
				"username": "camunda", "password": "app-secret",
			})

			dbConfig := fixtures.DatabaseConfig()
			dbConfig.Namespace = ns
			dbConfig.Spec.ServerRef = server.Name
			dbConfig.Spec.CredentialsSecretRef = v1.LocalCredentialsSecretRef{
				Name:        "app-user",
				UsernameKey: "username", PasswordKey: "password",
			}
			dbConfig.Spec.BackupCredentialsSecretRef = &v1.LocalCredentialsSecretRef{
				Name:        "gone-user",
				UsernameKey: "username", PasswordKey: "password",
			}
			Expect(k8sClient.Create(ctx, dbConfig)).To(Succeed())

			storage := &v1.SecondaryStorageConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "rdbms-" + utilrand.String(8), Namespace: ns},
				Spec: v1.SecondaryStorageConfigSpec{
					Type:  v1.SecondaryStorageTypeRDBMS,
					RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: dbConfig.Name},
				},
			}
			Expect(k8sClient.Create(ctx, storage)).To(Succeed())

			cluster := newCluster(ns, createPlatformConfig(), storage)
			createCluster(cluster)

			By("rendering the workloads: the pre-check did not park")
			Eventually(func(g Gomega) {
				g.Expect(k8sClient.Get(
					ctx, client.ObjectKey{Namespace: ns, Name: cluster.Name + "-zeebe"},
					&appsv1.StatefulSet{},
				)).To(Succeed())
			}, timeout, interval).Should(Succeed())

			By("warning about the unresolved dump credentials")
			Eventually(func(g Gomega) {
				var events eventsv1.EventList
				g.Expect(k8sClient.List(ctx, &events, client.InNamespace(ns))).To(Succeed())
				reasons := make([]string, 0, len(events.Items))
				for _, event := range events.Items {
					reasons = append(reasons, event.Reason)
				}
				g.Expect(reasons).To(ContainElement("DumpCredentialsUnresolved"))
			}, timeout, interval).Should(Succeed())
		})

		// The dump Job of a LogicalBackupRDBMS mounts the backup user of the
		// database from its own namespace, so a Secret there resolves
		// directly and raises no warning.
		It("resolves local dump credentials with no warning", func() {
			ns := newNamespace()

			server := fixtures.DatabaseServerConfig(ns)
			Expect(k8sClient.Create(ctx, server)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, server) })

			createSecret(ns, "app-user", map[string]string{
				"username": "camunda", "password": "app-secret",
			})
			createSecret(ns, "backup-user", map[string]string{
				"username": "backup", "password": "dump-secret",
			})

			dbConfig := fixtures.DatabaseConfig()
			dbConfig.Namespace = ns
			dbConfig.Spec.ServerRef = server.Name
			dbConfig.Spec.CredentialsSecretRef = v1.LocalCredentialsSecretRef{
				Name:        "app-user",
				UsernameKey: "username", PasswordKey: "password",
			}
			dbConfig.Spec.BackupCredentialsSecretRef = &v1.LocalCredentialsSecretRef{
				Name:        "backup-user",
				UsernameKey: "username", PasswordKey: "password",
			}
			Expect(k8sClient.Create(ctx, dbConfig)).To(Succeed())

			storage := &v1.SecondaryStorageConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "rdbms-" + utilrand.String(8), Namespace: ns},
				Spec: v1.SecondaryStorageConfigSpec{
					Type:  v1.SecondaryStorageTypeRDBMS,
					RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: dbConfig.Name},
				},
			}
			Expect(k8sClient.Create(ctx, storage)).To(Succeed())

			cluster := newCluster(ns, createPlatformConfig(), storage)
			createCluster(cluster)

			stampStatefulSetReady(client.ObjectKey{Namespace: ns, Name: cluster.Name + "-zeebe"})
			stampDeploymentReady(client.ObjectKey{Namespace: ns, Name: cluster.Name + "-gateway"})
			expectReady(cluster, metav1.ConditionTrue, Equal(v1.ReasonHealthy), Not(BeEmpty()))

			Consistently(func(g Gomega) {
				var events eventsv1.EventList
				g.Expect(k8sClient.List(ctx, &events, client.InNamespace(ns))).To(Succeed())
				reasons := make([]string, 0, len(events.Items))
				for _, event := range events.Items {
					reasons = append(reasons, event.Reason)
				}
				g.Expect(reasons).NotTo(ContainElement("DumpCredentialsUnresolved"))
			}, 2*time.Second, interval).Should(Succeed())
		})

		It("reports MissingSecret when the credentials of the bucket are absent", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			setSnapshotRepository(binding, "my-repository")

			bucket := &v1.ObjectStorageConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "osc-" + utilrand.String(8), Namespace: ns},
				Spec: v1.ObjectStorageConfigSpec{
					Type: v1.ObjectStorageTypeS3,
					S3: &v1.S3Storage{
						BucketName: "camunda-backups",
						Endpoint:   "http://minio.minio.svc:9000",
						Auth: v1.S3StorageAuth{
							Type: v1.ObjectStorageAuthTypeCredentials,
							Credentials: &v1.S3Credentials{SecretRef: v1.S3CredentialsSecretRef{
								Name:               "absent-keys",
								AccessKeyIDKey:     "accessKeyId",
								SecretAccessKeyKey: "secretAccessKey",
							}},
						},
					},
				},
			}
			Expect(k8sClient.Create(ctx, bucket)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, bucket) })

			cluster := newCluster(ns, createPlatformConfig(), binding)
			cluster.Spec.BackupStorageRef = bucket.Name
			createCluster(cluster)

			expectReady(
				cluster,
				metav1.ConditionFalse,
				Equal(v1.ReasonMissingSecret),
				ContainSubstring("absent-keys"),
			)
		})
	})
})

var _ = Describe("CamundaCluster backup schema", func() {
	It("accepts the backup block of the doc", func() {
		cluster := minimalCamundaCluster()
		cluster.Spec.Backup = &v1.ClusterBackupSpec{
			PrimaryStorage: &v1.PrimaryStorageBackupSpec{
				Continuous:         new(true),
				Schedule:           "PT1H",
				CheckpointInterval: "PT15M",
				Retention: &v1.PrimaryStorageRetentionSpec{
					Window:          "P7D",
					CleanupSchedule: "PT1H",
				},
			},
			Dump: &v1.BackupDumpSpec{DumpPodSpec: v1.DumpPodSpec{
				PodAnnotations: map[string]string{"sidecar.istio.io/inject": "false"},
				ScratchVolume:  &v1.ScratchVolumeSpec{SizeLimit: new(resource.MustParse("50Gi"))},
			}},
		}

		Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, cluster) })
	})

	// A CRON expression is legal for the two schedules, so only the pure
	// duration fields carry a pattern.
	It("accepts a CRON schedule and the none keyword", func() {
		cluster := minimalCamundaCluster()
		cluster.Spec.Backup = &v1.ClusterBackupSpec{
			PrimaryStorage: &v1.PrimaryStorageBackupSpec{
				Schedule:  "0 */5 * * * *",
				Retention: &v1.PrimaryStorageRetentionSpec{CleanupSchedule: "none"},
			},
		}

		Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, cluster) })
	})

	It("rejects a checkpoint interval that is not a duration", func() {
		cluster := minimalCamundaCluster()
		cluster.Spec.Backup = &v1.ClusterBackupSpec{
			PrimaryStorage: &v1.PrimaryStorageBackupSpec{CheckpointInterval: "15 minutes"},
		}

		Expect(k8sClient.Create(ctx, cluster)).NotTo(Succeed())
	})

	// Camunda parses durations of days and time only. P1W, P1M, and P1Y are
	// valid ISO 8601 but crash-loop the broker, so admission rejects them.
	It("rejects duration units that Camunda does not parse", func() {
		for _, window := range []string{"one week", "P1W", "P1M", "P1Y", "P", "PT"} {
			cluster := minimalCamundaCluster()
			cluster.Spec.Backup = &v1.ClusterBackupSpec{
				PrimaryStorage: &v1.PrimaryStorageBackupSpec{
					Retention: &v1.PrimaryStorageRetentionSpec{Window: window},
				},
			}

			Expect(k8sClient.Create(ctx, cluster)).NotTo(Succeed(), window)
		}

		cluster := minimalCamundaCluster()
		cluster.Spec.Backup = &v1.ClusterBackupSpec{
			PrimaryStorage: &v1.PrimaryStorageBackupSpec{CheckpointInterval: "P1W"},
		}
		Expect(k8sClient.Create(ctx, cluster)).NotTo(Succeed())
	})

	It("accepts day and time durations", func() {
		cluster := minimalCamundaCluster()
		cluster.Spec.Backup = &v1.ClusterBackupSpec{
			PrimaryStorage: &v1.PrimaryStorageBackupSpec{
				CheckpointInterval: "PT0.5S",
				Retention:          &v1.PrimaryStorageRetentionSpec{Window: "P2DT12H"},
			},
		}

		Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, cluster) })
	})

	// The backup block is policy, so a preset may carry it. Where backups go
	// is instance-bound and stays on the cluster.
	It("accepts the backup block in a preset but not backupStorageRef", func() {
		preset := &v1.CamundaClusterPreset{
			ObjectMeta: metav1.ObjectMeta{Name: "ccp-" + utilrand.String(8)},
			Spec: v1.CamundaClusterPresetSpec{Cluster: v1.CamundaClusterSpec{
				Version: "8.9.0",
				Backup: &v1.ClusterBackupSpec{
					PrimaryStorage: &v1.PrimaryStorageBackupSpec{Schedule: "PT2H"},
				},
			}},
		}
		Expect(k8sClient.Create(ctx, preset)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, preset) })

		rejected := &v1.CamundaClusterPreset{
			ObjectMeta: metav1.ObjectMeta{Name: "ccp-" + utilrand.String(8)},
			Spec: v1.CamundaClusterPresetSpec{Cluster: v1.CamundaClusterSpec{
				Version:          "8.9.0",
				BackupStorageRef: "my-backup-config",
			}},
		}
		Expect(k8sClient.Create(ctx, rejected)).NotTo(Succeed())
	})
})
