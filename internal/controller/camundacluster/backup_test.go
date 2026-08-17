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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
)

// createBucket creates an S3 ObjectStorageConfig with workload identity and
// registers its deletion. The role is unique per bucket unless roleARN is
// given, so a second bucket can be made to conflict.
func createBucket(roleARN string) *v1.ObjectStorageConfig {
	GinkgoHelper()
	bucket := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "osc-" + utilrand.String(8)},
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

var _ = Describe("CamundaCluster backup wiring", func() {
	Describe("the published management binding", func() {
		It("names the endpoint, the version, and the partition count", func() {
			cluster := createDefaultCluster()

			expectBinding(cluster, func(g Gomega, binding *v1.ManagementBinding) {
				g.Expect(binding).NotTo(BeNil())
				g.Expect(binding.Endpoint).To(Equal(
					"http://" + cluster.Name + "-zeebe." + cluster.Namespace + ".svc:9600",
				))
				g.Expect(binding.Version).To(Equal("8.9.9"))
				g.Expect(binding.Partitions).To(Equal(int32(1)))
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

		It("carries the snapshot repository of the storage contract", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			setSnapshotRepository(binding, "my-repository")
			cluster := newCluster(ns, createPlatformConfig(), binding)
			cluster.Spec.BackupStorageRef = createBucket("arn:aws:iam::123456789012:role/camunda").Name
			createCluster(cluster)

			expectBinding(cluster, func(g Gomega, published *v1.ManagementBinding) {
				g.Expect(published).NotTo(BeNil())
				g.Expect(published.BackupRepository).To(Equal("my-repository"))
			})
		})
	})

	Describe("the backup bucket", func() {
		// Without a repository the web applications have nowhere to write
		// their snapshots, so the reference is incomplete rather than the
		// backup silently failing later.
		It("rejects an Elasticsearch cluster whose contract has no snapshot repository", func() {
			ns := newNamespace()
			cluster := newCluster(ns, createPlatformConfig(), createBinding(ns, true))
			cluster.Spec.BackupStorageRef = createBucket("arn:aws:iam::123456789012:role/camunda").Name
			createCluster(cluster)

			expectReady(
				cluster,
				metav1.ConditionFalse,
				Equal(v1.ReasonInvalidReference),
				ContainSubstring("elasticsearch.snapshotRepository"),
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
			cluster.Spec.BackupStorageRef = createBucket("arn:aws:iam::123456789012:role/backup").Name
			cluster.Spec.DocumentStorageRef = createBucket("arn:aws:iam::123456789012:role/documents").Name
			createCluster(cluster)

			expectReady(
				cluster,
				metav1.ConditionFalse,
				Equal(v1.ReasonInvalidReference),
				ContainSubstring("one cluster has one ServiceAccount"),
			)
		})

		It("accepts two buckets that name the same identity", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			setSnapshotRepository(binding, "my-repository")
			cluster := newCluster(ns, createPlatformConfig(), binding)
			role := "arn:aws:iam::123456789012:role/camunda"
			cluster.Spec.BackupStorageRef = createBucket(role).Name
			cluster.Spec.DocumentStorageRef = createBucket(role).Name
			createCluster(cluster)

			expectBinding(cluster, func(g Gomega, published *v1.ManagementBinding) {
				g.Expect(published).NotTo(BeNil())
			})
		})

		It("annotates the ServiceAccount with the identity of the bucket", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			setSnapshotRepository(binding, "my-repository")
			cluster := newCluster(ns, createPlatformConfig(), binding)
			cluster.Spec.BackupStorageRef = createBucket("arn:aws:iam::123456789012:role/camunda").Name
			createCluster(cluster)

			Eventually(func(g Gomega) {
				var account corev1.ServiceAccount
				key := client.ObjectKey{
					Namespace: cluster.Namespace,
					Name:      components.ServiceAccountName(cluster),
				}
				g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
				g.Expect(account.Annotations).To(HaveKeyWithValue(
					components.AWSRoleARNAnnotation,
					"arn:aws:iam::123456789012:role/camunda",
				))
			}, timeout, interval).Should(Succeed())
		})

		// Static keys may live anywhere, and a pod reads only Secrets of its
		// own namespace, so the copy is what the workloads reference.
		It("copies the static credentials of a bucket into the cluster namespace", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			setSnapshotRepository(binding, "my-repository")
			createSecret("default", "minio-keys", map[string]string{
				"accessKeyId": "minio", "secretAccessKey": "minio123",
			})

			bucket := &v1.ObjectStorageConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "osc-" + utilrand.String(8)},
				Spec: v1.ObjectStorageConfigSpec{
					Type: v1.ObjectStorageTypeS3,
					S3: &v1.S3Storage{
						BucketName: "camunda-backups",
						Endpoint:   "http://minio.minio.svc:9000",
						Auth: v1.S3StorageAuth{
							Type: v1.ObjectStorageAuthTypeCredentials,
							Credentials: &v1.S3Credentials{SecretRef: v1.S3CredentialsSecretRef{
								Name:               "minio-keys",
								Namespace:          "default",
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

			Eventually(func(g Gomega) {
				var copied corev1.Secret
				key := client.ObjectKey{
					Namespace: cluster.Namespace,
					Name: components.MirroredSecretName(
						cluster,
						components.MirrorPurposeBackupCredentials,
					),
				}
				g.Expect(k8sClient.Get(ctx, key, &copied)).To(Succeed())
				g.Expect(copied.Data).To(HaveKeyWithValue("accessKeyId", []byte("minio")))
				g.Expect(copied.Data).To(HaveKeyWithValue("secretAccessKey", []byte("minio123")))
			}, timeout, interval).Should(Succeed())
		})

		It("reports MissingSecret when the credentials of the bucket are absent", func() {
			ns := newNamespace()
			binding := createBinding(ns, true)
			setSnapshotRepository(binding, "my-repository")

			bucket := &v1.ObjectStorageConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "osc-" + utilrand.String(8)},
				Spec: v1.ObjectStorageConfigSpec{
					Type: v1.ObjectStorageTypeS3,
					S3: &v1.S3Storage{
						BucketName: "camunda-backups",
						Endpoint:   "http://minio.minio.svc:9000",
						Auth: v1.S3StorageAuth{
							Type: v1.ObjectStorageAuthTypeCredentials,
							Credentials: &v1.S3Credentials{SecretRef: v1.S3CredentialsSecretRef{
								Name:               "absent-keys",
								Namespace:          ns,
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
