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

package elasticsearchcluster

import (
	"maps"
	"slices"
	"time"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/types"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// newElasticsearchClusterNamespace creates a uniquely named Namespace for one
// spec and registers its deletion.
// versionBelowFloor is an Elasticsearch version below the Camunda 8.9 floor.
const versionBelowFloor = "8.18.0"

func newElasticsearchClusterNamespace() string {
	GinkgoHelper()
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: "esc-ns-" + utilrand.String(8)},
	}
	Expect(k8sClient.Create(ctx, ns)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, ns) })
	return ns.Name
}

// smallClusterSpec is a single-node shape that presets use as their cluster
// block. It carries no version, which a preset rejects.
func smallClusterSpec() v1.ElasticsearchClusterSpec {
	return v1.ElasticsearchClusterSpec{
		Replicas:    new(int32(1)),
		StorageSize: new(resource.MustParse("1Gi")),
	}
}

// createElasticsearchClusterPreset creates a uniquely named preset with the
// given cluster baseline and registers its deletion.
func createElasticsearchClusterPreset(spec v1.ElasticsearchClusterSpec) *v1.ElasticsearchClusterPreset {
	GinkgoHelper()
	preset := &v1.ElasticsearchClusterPreset{
		ObjectMeta: metav1.ObjectMeta{Name: "escp-" + utilrand.String(8)},
		Spec:       v1.ElasticsearchClusterPresetSpec{Cluster: spec},
	}
	Expect(k8sClient.Create(ctx, preset)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, preset) })
	return preset
}

// createElasticsearchRelease creates a uniquely named CamundaRelease that
// names the given Elasticsearch version, and registers its deletion.
func createElasticsearchRelease(version string) *v1.CamundaRelease {
	GinkgoHelper()
	release := &v1.CamundaRelease{
		ObjectMeta: metav1.ObjectMeta{Name: "esr-" + utilrand.String(8)},
		Spec: v1.CamundaReleaseSpec{
			Version:       "8.9.18",
			Elasticsearch: &v1.ReleaseElasticsearchSpec{Version: version},
		},
	}
	Expect(k8sClient.Create(ctx, release)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, release) })
	return release
}

// createElasticsearchCluster creates cluster in its own fresh namespace,
// unless the test assigned one already, and registers its deletion.
func createElasticsearchCluster(cluster *v1.ElasticsearchCluster) {
	GinkgoHelper()
	if cluster.Namespace == "" {
		cluster.Namespace = newElasticsearchClusterNamespace()
	}
	Expect(k8sClient.Create(ctx, cluster)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, cluster) })
}

// expectElasticsearchClusterReady polls until the Ready condition of cluster
// has the given status and its reason and message match the given matchers.
func expectElasticsearchClusterReady(
	cluster *v1.ElasticsearchCluster,
	status metav1.ConditionStatus,
	reason, message types.GomegaMatcher,
) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.ElasticsearchCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Status).To(Equal(status))
		g.Expect(ready.Reason).To(reason)
		g.Expect(ready.Message).To(message)
	}, timeout, interval).Should(Succeed())
}

// fetchOwnedElasticsearch polls until the ECK CR with the name of cluster
// exists, and returns it.
func fetchOwnedElasticsearch(cluster *v1.ElasticsearchCluster) *esv1.Elasticsearch {
	GinkgoHelper()
	var es esv1.Elasticsearch
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).To(Succeed())
	}, timeout, interval).Should(Succeed())
	return &es
}

// updateECKStatus patches the status subresource of the ECK CR. It always
// stamps status.observedGeneration with the current generation of the CR, so
// the handlers of the wrapper trust the reported state. envtest runs no ECK
// operator, so the specs drive health transitions this way.
func updateECKStatus(cluster *v1.ElasticsearchCluster, mutate func(*esv1.Elasticsearch)) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var es esv1.Elasticsearch
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).To(Succeed())
		es.Status.ObservedGeneration = es.Generation
		mutate(&es)
		g.Expect(k8sClient.Status().Update(ctx, &es)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// expectRetainingPolicy polls until the applied Elasticsearch CR of cluster
// carries DeleteOnScaledownOnly: the suspend mutation switches the policy
// before the CR is deleted, and the specs stamp the ECK status only after
// that generation exists.
func expectRetainingPolicy(cluster *v1.ElasticsearchCluster) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest esv1.Elasticsearch
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		g.Expect(latest.Spec.VolumeClaimDeletePolicy).To(Equal(esv1.DeleteOnScaledownOnlyPolicy))
	}, timeout, interval).Should(Succeed())
}

// expectStorageShrinkIgnored polls until the controller has recorded the
// StorageShrinkIgnored event for cluster, then asserts that the applied ECK
// data volume claim still requests applied and that Ready does not report
// InvalidReference.
func expectStorageShrinkIgnored(cluster *v1.ElasticsearchCluster, applied string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var events corev1.EventList
		g.Expect(k8sClient.List(ctx, &events, client.InNamespace(cluster.Namespace))).To(Succeed())
		g.Expect(events.Items).To(ContainElement(SatisfyAll(
			HaveField("Reason", "StorageShrinkIgnored"),
			HaveField("InvolvedObject.Name", cluster.Name),
			HaveField("Type", corev1.EventTypeWarning),
		)))
	}, timeout, interval).Should(Succeed())

	// The event lands before the component applies the ECK CR in the same
	// reconcile, so the CR is polled too.
	Eventually(func(g Gomega) {
		var es esv1.Elasticsearch
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).To(Succeed())
		g.Expect(es.Spec.NodeSets[0].VolumeClaimTemplates[0].Spec.Resources.Requests[corev1.ResourceStorage]).
			To(Equal(resource.MustParse(applied)))
	}, timeout, interval).Should(Succeed())

	var latest v1.ElasticsearchCluster
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
	ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
	Expect(ready).NotTo(BeNil())
	Expect(ready.Reason).NotTo(Equal(v1.ReasonInvalidReference))
}

// expectVolumes polls until status.volumes of cluster lists exactly the
// given claim names with the given capacities, in name order.
func expectVolumes(cluster *v1.ElasticsearchCluster, want map[string]string) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.ElasticsearchCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		g.Expect(latest.Status.Volumes).To(HaveLen(len(want)))
		for i, volume := range latest.Status.Volumes {
			g.Expect(volume.Name).To(Equal(slices.Sorted(maps.Keys(want))[i]))
			g.Expect(volume.Capacity.Cmp(resource.MustParse(want[volume.Name]))).To(BeZero(), volume.Name)
		}
	}, timeout, interval).Should(Succeed())
}

// createDataClaim creates a data claim of cluster with the given ordinal, as
// ECK would name and label it, bound with the given capacity, and registers
// its deletion.
func createDataClaim(cluster *v1.ElasticsearchCluster, ordinal, capacity string) *corev1.PersistentVolumeClaim {
	GinkgoHelper()
	claim := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      components.DataVolumeClaimName + "-" + cluster.Name + "-es-default-" + ordinal,
			Namespace: cluster.Namespace,
			Labels:    map[string]string{components.ECKClusterNameLabel: cluster.Name},
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(capacity)},
			},
		},
	}
	Expect(k8sClient.Create(ctx, claim)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, claim) })
	claim.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(capacity)}
	claim.Status.Phase = corev1.ClaimBound
	Expect(k8sClient.Status().Update(ctx, claim)).To(Succeed())
	return claim
}

// s3BucketSpec returns an S3 bucket contract: with workload identity when
// credentials is nil, with the given static credentials otherwise.
func s3BucketSpec(credentials *v1.S3Credentials) v1.ObjectStorageConfigSpec {
	auth := v1.S3StorageAuth{
		Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
		WorkloadIdentity: &v1.S3WorkloadIdentity{RoleARN: "arn:aws:iam::123456789012:role/camunda"},
	}
	if credentials != nil {
		auth = v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeCredentials, Credentials: credentials}
	}

	return v1.ObjectStorageConfigSpec{
		Type: v1.ObjectStorageTypeS3,
		S3: &v1.S3Storage{
			BucketName: "camunda-backups",
			BasePath:   "clusters",
			Region:     "eu-west-1",
			Auth:       auth,
		},
	}
}

// gcsBucketSpec is the GCS counterpart of s3BucketSpec.
func gcsBucketSpec(credentials *v1.GCSCredentials) v1.ObjectStorageConfigSpec {
	auth := v1.GCSStorageAuth{
		Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
		WorkloadIdentity: &v1.GCSWorkloadIdentity{ServiceAccountEmail: "camunda@example.iam.gserviceaccount.com"},
	}
	if credentials != nil {
		auth = v1.GCSStorageAuth{Type: v1.ObjectStorageAuthTypeCredentials, Credentials: credentials}
	}

	return v1.ObjectStorageConfigSpec{
		Type: v1.ObjectStorageTypeGCS,
		GCS: &v1.GCSStorage{
			BucketName: "camunda-backups",
			BasePath:   "clusters",
			Auth:       auth,
		},
	}
}

// azureBucketSpec is the Azure counterpart of s3BucketSpec. An empty endpoint
// means the public Azure endpoint of the account.
func azureBucketSpec(endpoint string, credentials *v1.AzureBlobCredentials) v1.ObjectStorageConfigSpec {
	auth := v1.AzureBlobStorageAuth{
		Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
		WorkloadIdentity: &v1.AzureBlobWorkloadIdentity{ClientID: "00000000-0000-0000-0000-000000000000"},
	}
	if credentials != nil {
		auth = v1.AzureBlobStorageAuth{Type: v1.ObjectStorageAuthTypeCredentials, Credentials: credentials}
	}

	return v1.ObjectStorageConfigSpec{
		Type: v1.ObjectStorageTypeAzureBlob,
		AzureBlob: &v1.AzureBlobStorage{
			AccountName: "camundabackups",
			Container:   "camunda-backups",
			BasePath:    "clusters",
			Endpoint:    endpoint,
			Auth:        auth,
		},
	}
}

// servingClusterWithBucket creates a cluster in namespace that references the
// bucket and whose ECK Secrets are in place, so the reconciler reaches the
// registration.
func servingClusterWithBucket(namespace, bucket string) *v1.ElasticsearchCluster {
	GinkgoHelper()
	preset := createElasticsearchClusterPreset(smallClusterSpec())
	cluster := validElasticsearchCluster()
	cluster.Spec.PresetRef = preset.Name
	cluster.Spec.SnapshotStorageRef = bucket
	cluster.Namespace = namespace
	createECKSecrets(cluster)
	createElasticsearchCluster(cluster)

	return cluster
}

// expectRepositoryRegistered waits for the cluster to report a healthy
// snapshot repository.
func expectRepositoryRegistered(cluster *v1.ElasticsearchCluster) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var fetched v1.ElasticsearchCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &fetched)).To(Succeed())
		condition := meta.FindStatusCondition(
			fetched.Status.Conditions, components.ConditionSnapshotRepository,
		)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
		g.Expect(condition.Reason).To(Equal(v1.ReasonHealthy))
	}, timeout, interval).Should(Succeed())
}

// createObjectStorageConfig creates a bucket contract in namespace and
// removes it after the test.
func createObjectStorageConfig(namespace string, spec v1.ObjectStorageConfigSpec) *v1.ObjectStorageConfig {
	GinkgoHelper()
	config := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "bucket-" + utilrand.String(8), Namespace: namespace},
		Spec:       spec,
	}
	Expect(k8sClient.Create(ctx, config)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, config) })

	return config
}

// createECKSecrets creates the Secrets that ECK publishes for a serving
// cluster: the elastic user's password, and the CA that verifies the suite's
// TLS fake — the same CA path the reconciler walks against a real ECK
// cluster.
func createECKSecrets(cluster *v1.ElasticsearchCluster) {
	GinkgoHelper()
	for name, data := range map[string]map[string][]byte{
		esv1.ElasticUserSecret(cluster.Name):   {"elastic": []byte("elastic-password")},
		cluster.Name + "-es-http-certs-public": {"ca.crt": elasticsearch.CertificatePEM()},
	} {
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: cluster.Namespace},
			Data:       data,
		}
		Expect(k8sClient.Create(ctx, secret)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, secret) })
	}
}

// expectControlledBy asserts that obj carries a controller owner reference to
// cluster. Deletion then garbage-collects it without a finalizer.
func expectControlledBy(obj client.Object, cluster *v1.ElasticsearchCluster) {
	GinkgoHelper()
	controller := metav1.GetControllerOf(obj)
	Expect(controller).NotTo(BeNil())
	Expect(controller.Kind).To(Equal("ElasticsearchCluster"))
	Expect(controller.Name).To(Equal(cluster.Name))
}

var _ = Describe("ElasticsearchCluster controller", func() {
	It("publishes the ECK CR, user Secret and storage contract and mirrors the creating component", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)

		es := fetchOwnedElasticsearch(cluster)
		Expect(es.Spec.Version).To(Equal("9.2.4"))
		Expect(es.Spec.Auth.FileRealm).To(HaveLen(1))
		Expect(es.Spec.Auth.FileRealm[0].SecretName).To(Equal(cluster.Name + "-es-user"))
		Expect(es.Spec.NodeSets).To(HaveLen(1))
		Expect(es.Spec.NodeSets[0].Count).To(Equal(int32(1)))
		Expect(
			es.Spec.NodeSets[0].PodTemplate.Labels,
		).To(HaveKeyWithValue(labels.ElasticsearchClusterKey, cluster.Name))
		Expect(es.Spec.NodeSets[0].PodTemplate.Labels).To(HaveKeyWithValue(labels.ComponentKey, "elasticsearch"))
		Expect(es.Spec.NodeSets[0].VolumeClaimTemplates).To(HaveLen(1))
		claim := es.Spec.NodeSets[0].VolumeClaimTemplates[0]
		Expect(claim.Name).To(Equal("elasticsearch-data"))
		Expect(claim.Labels).To(HaveKeyWithValue(labels.ElasticsearchClusterKey, cluster.Name))
		Expect(claim.Spec.Resources.Requests[corev1.ResourceStorage]).To(Equal(resource.MustParse("1Gi")))
		expectControlledBy(es, cluster)

		var secret corev1.Secret
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{
					Namespace: cluster.Namespace, Name: cluster.Name + "-es-user",
				}, &secret,
			)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Expect(secret.Data["username"]).To(Equal([]byte("camunda")))
		Expect(secret.Data["roles"]).To(Equal([]byte("camunda")))
		Expect(secret.Data["password"]).To(HaveLen(32))
		expectControlledBy(&secret, cluster)

		var contract v1.SecondaryStorageConfig
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKey{
					Namespace: cluster.Namespace, Name: cluster.Spec.SecondaryStorageConfig,
				}, &contract,
			)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Expect(contract.Spec.Type).To(Equal(v1.SecondaryStorageTypeElasticsearch))
		Expect(contract.Spec.Elasticsearch).NotTo(BeNil())
		Expect(contract.Spec.Elasticsearch.Endpoint).To(Equal(
			"https://" + cluster.Name + "-es-http." + cluster.Namespace + ".svc:9200",
		))
		Expect(contract.Spec.Elasticsearch.CredentialsSecretRef).To(Equal(v1.LocalCredentialsSecretRef{
			Name:        cluster.Name + "-es-user",
			UsernameKey: "username", PasswordKey: "password",
		}))
		Expect(contract.Spec.Elasticsearch.CASecretRef).To(Equal(&v1.LocalSecretKeyRef{
			Name: cluster.Name + "-es-http-certs-public", Key: "ca.crt",
		}))
		expectControlledBy(&contract, cluster)

		expectElasticsearchClusterReady(
			cluster, metav1.ConditionFalse,
			// Before ECK reports health, the first reconcile sees Creating and
			// any later reconcile sees Failing (health unreported past the
			// first apply). envtest runs no ECK, so both are valid here.
			SatisfyAny(Equal(string(component.AliveCreating)), Equal(string(component.AliveFailing))),
			HavePrefix("elasticsearch: "),
		)
	})

	It("mirrors the Elasticsearch component onto Ready as ECK health transitions", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		fetchOwnedElasticsearch(cluster)

		updateECKStatus(cluster, func(es *esv1.Elasticsearch) {
			es.Status.Health = esv1.ElasticsearchGreenHealth
			es.Status.AvailableNodes = 1
		})
		expectElasticsearchClusterReady(
			cluster, metav1.ConditionTrue,
			Equal(v1.ReasonHealthy), HaveSuffix(": Component is healthy."),
		)

		updateECKStatus(cluster, func(es *esv1.Elasticsearch) {
			es.Status.Health = esv1.ElasticsearchRedHealth
		})
		expectElasticsearchClusterReady(
			cluster, metav1.ConditionFalse,
			Equal(string(component.AliveFailing)), Equal("elasticsearch: Elasticsearch reports red health"),
		)
	})

	It("reports InvalidReference for a dangling presetRef and applies nothing", func() {
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = "does-not-exist-" + utilrand.String(8)
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonInvalidReference), ContainSubstring(cluster.Spec.PresetRef),
		)

		var es esv1.Elasticsearch
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).NotTo(Succeed())
	})

	It("reports InvalidReference for a dangling snapshotStorageRef", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.SnapshotStorageRef = "no-such-bucket-" + utilrand.String(8)
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonInvalidReference), ContainSubstring(cluster.Spec.SnapshotStorageRef),
		)
	})

	// Elasticsearch addresses an azure account as https://<account>.blob.<suffix>
	// and configures only the suffix, so an endpoint that does not reduce to
	// one cannot be served. Saying so is better than registering a repository
	// against the public endpoint of the account, which is a different store
	// than the contract names.
	It("reports InvalidReference for an azure endpoint that is not a suffix", func() {
		namespace := newElasticsearchClusterNamespace()
		bucket := createObjectStorageConfig(namespace, azureBucketSpec(
			"http://azurite.azurite.svc:10000/devstoreaccount1", nil,
		))
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.SnapshotStorageRef = bucket.Name
		cluster.Namespace = namespace
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonInvalidReference), ContainSubstring("endpoint suffix"),
		)
	})

	It("reports MissingSecret when the snapshot bucket names a Secret that does not exist", func() {
		namespace := newElasticsearchClusterNamespace()
		bucket := createObjectStorageConfig(namespace, s3BucketSpec(&v1.S3Credentials{
			SecretRef: v1.S3CredentialsSecretRef{
				Name:               "absent-" + utilrand.String(8),
				AccessKeyIDKey:     "accessKeyId",
				SecretAccessKeyKey: "secretAccessKey",
			},
		}))
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.SnapshotStorageRef = bucket.Name
		cluster.Namespace = namespace
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonMissingSecret), ContainSubstring("not found"),
		)
	})

	// A ServiceAccount the operator does not own must exist before the pods
	// reference it; otherwise every pod stays unschedulable, which is a much
	// slower and less obvious failure than a reference that reports itself.
	It("reports InvalidReference for a pre-existing ServiceAccount that is absent", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		no := false
		cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{
			Name:   "platform-es-" + utilrand.String(8),
			Create: &no,
		}
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonInvalidReference),
			And(ContainSubstring(cluster.Spec.ServiceAccount.Name), ContainSubstring("create is false")),
		)
	})

	// The bucket identity reaches the pods without the user restating it in
	// serviceAccount.annotations, and the ServiceAccount is rendered even
	// though the spec asks for none.
	It("derives the workload-identity annotation of the bucket onto the ServiceAccount", func() {
		namespace := newElasticsearchClusterNamespace()
		bucket := createObjectStorageConfig(namespace, s3BucketSpec(nil))
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.SnapshotStorageRef = bucket.Name
		cluster.Namespace = namespace
		createElasticsearchCluster(cluster)

		var account corev1.ServiceAccount
		key := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-es"}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
			g.Expect(account.Annotations).To(HaveKeyWithValue(
				v1.IRSARoleARNAnnotation, "arn:aws:iam::123456789012:role/camunda",
			))
		}, timeout, interval).Should(Succeed())
	})

	// The contract names the repository only once its registration converged:
	// a consumer reading an earlier name would snapshot against a repository
	// that does not exist.
	It("publishes the repository name once registration converges, not before", func() {
		namespace := newElasticsearchClusterNamespace()
		bucket := createObjectStorageConfig(namespace, s3BucketSpec(nil))
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.SnapshotStorageRef = bucket.Name
		cluster.Namespace = namespace
		createElasticsearchCluster(cluster)

		// Without the Secrets that ECK publishes, registration cannot run and
		// the contract must not name the repository.
		var contract v1.SecondaryStorageConfig
		key := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Spec.SecondaryStorageConfig}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &contract)).To(Succeed())
			g.Expect(contract.Spec.Elasticsearch).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		Consistently(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &contract)).To(Succeed())
			g.Expect(contract.Spec.Elasticsearch.SnapshotRepository).To(BeEmpty())
		}, "2s", interval).Should(Succeed())

		createECKSecrets(cluster)

		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &contract)).To(Succeed())
			g.Expect(contract.Spec.Elasticsearch.SnapshotRepository).To(Equal(components.RepositoryName(cluster)))
		}, timeout, interval).Should(Succeed())

		// The published name comes from the record of what converged, so the
		// cluster reports the same repository the contract carries.
		var fetched v1.ElasticsearchCluster
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &fetched)).To(Succeed())
		Expect(fetched.Status.SnapshotRepository).To(Equal(components.RepositoryName(cluster)))
	})

	// Nothing reports that Elasticsearch started answering: it is not an
	// object, so no watch fires for it. A registration that failed to reach
	// the cluster must therefore come back on a requeue the controller asks
	// for itself, or the cluster stays not ready until something unrelated
	// happens to write its status.
	It("registers again after a registration that could not reach the cluster", func() {
		namespace := newElasticsearchClusterNamespace()
		bucket := createObjectStorageConfig(namespace, s3BucketSpec(nil))
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.SnapshotStorageRef = bucket.Name
		cluster.Namespace = namespace
		createECKSecrets(cluster)
		createElasticsearchCluster(cluster)

		By("waiting until registration converged and every watch is quiet")
		Eventually(func(g Gomega) {
			var fetched v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &fetched)).To(Succeed())
			condition := meta.FindStatusCondition(
				fetched.Status.Conditions, components.ConditionSnapshotRepository,
			)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
		}, timeout, interval).Should(Succeed())

		// Four attempts fail. That outlasts the reconciles that the
		// controller's own status write triggers: the second failure writes
		// the same condition, so nothing enqueues the cluster again.
		elasticsearch.DropNext("repository", 4)

		By("changing the bucket once, so the next reconcile really registers")
		Eventually(func(g Gomega) {
			var changed v1.ObjectStorageConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bucket), &changed)).To(Succeed())
			changed.Spec.S3.Endpoint = "http://minio.minio.svc:9000"
			changed.Spec.S3.ForcePathStyle = true
			g.Expect(k8sClient.Update(ctx, &changed)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		By("waiting for it to register on its own, with nothing else written")
		Eventually(func(g Gomega) {
			var fetched v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &fetched)).To(Succeed())
			condition := meta.FindStatusCondition(
				fetched.Status.Conditions, components.ConditionSnapshotRepository,
			)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(v1.ReasonHealthy))

			registered := elasticsearch.Repository(components.RepositoryName(cluster))
			g.Expect(registered).NotTo(BeNil())
			g.Expect(registered.Settings).To(
				HaveKeyWithValue("endpoint", "http://minio.minio.svc:9000"),
			)
		}, timeout, interval).Should(Succeed())
	})

	It("registers the snapshot repository and converges it idempotently", func() {
		namespace := newElasticsearchClusterNamespace()
		bucket := createObjectStorageConfig(namespace, s3BucketSpec(nil))
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.SnapshotStorageRef = bucket.Name
		cluster.Namespace = namespace
		createECKSecrets(cluster)
		createElasticsearchCluster(cluster)

		Eventually(func(g Gomega) {
			var fetched v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &fetched)).To(Succeed())
			condition := meta.FindStatusCondition(
				fetched.Status.Conditions, components.ConditionSnapshotRepository,
			)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(v1.ReasonHealthy))
		}, timeout, interval).Should(Succeed())

		repo := elasticsearch.Repository(components.RepositoryName(cluster))
		Expect(repo).NotTo(BeNil())
		Expect(repo.Type).To(Equal("s3"))
		Expect(repo.Settings).To(HaveKeyWithValue("bucket", "camunda-backups"))
		Expect(repo.Settings).To(HaveKeyWithValue(
			"base_path", "clusters/"+cluster.Namespace+"/"+cluster.Name,
		))
		// The workload-identity bucket names no endpoint and no path style,
		// so neither may leak into the repository settings.
		Expect(repo.Settings).NotTo(HaveKey("endpoint"))
		Expect(repo.Settings).NotTo(HaveKey("path_style_access"))

		// A converged, unchanged repository is not re-verified: Elasticsearch
		// makes every data node write a test blob per registration. A reconcile
		// that changes nothing must keep the condition without a new PUT.
		// The baseline is taken once the registration has quiesced. The
		// condition goes True on the reconcile that registered, and a reconcile
		// that started before it can still land a later PUT. A baseline taken
		// while one is in flight makes the check below fail on the reconcile
		// that was already on its way.
		var puts int
		Eventually(func(g Gomega) {
			before := elasticsearch.RepositoryPuts(components.RepositoryName(cluster))
			g.Expect(before).NotTo(BeZero())
			time.Sleep(interval)
			puts = elasticsearch.RepositoryPuts(components.RepositoryName(cluster))
			g.Expect(puts).To(Equal(before), "a registration is still in flight")
		}, timeout, interval).Should(Succeed())
		// The reconciler writes the cluster too, so the update re-reads on a
		// conflict.
		Eventually(func(g Gomega) {
			var fetched v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &fetched)).To(Succeed())
			fetched.Labels = map[string]string{"touched": "true"}
			g.Expect(k8sClient.Update(ctx, &fetched)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(elasticsearch.RepositoryPuts(components.RepositoryName(cluster))).To(Equal(puts))
		}, "2s", interval).Should(Succeed())

		// A changed bucket re-registers: the fingerprint no longer matches.
		Eventually(func(g Gomega) {
			var changed v1.ObjectStorageConfig
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(bucket), &changed)).To(Succeed())
			changed.Spec.S3.Endpoint = "http://minio.minio.svc:9000"
			changed.Spec.S3.ForcePathStyle = true
			g.Expect(k8sClient.Update(ctx, &changed)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(elasticsearch.RepositoryPuts(components.RepositoryName(cluster))).To(BeNumerically(">", puts))
			refreshed := elasticsearch.Repository(components.RepositoryName(cluster))
			g.Expect(refreshed).NotTo(BeNil())
			g.Expect(refreshed.Settings).To(HaveKeyWithValue("endpoint", "http://minio.minio.svc:9000"))
		}, timeout, interval).Should(Succeed())
	})

	// The suite runs one Elasticsearch behind every cluster, which is what a
	// contract that names the Elasticsearch of another namespace produces. Two
	// clusters of one name must not meet on one registration there: the second
	// would point the repository of the first at its own prefix, and a
	// consumer of either would read snapshots that the other wrote.
	It("gives two clusters of one name in two namespaces two repositories", func() {
		name := "esc-" + utilrand.String(8)
		serving := func() *v1.ElasticsearchCluster {
			namespace := newElasticsearchClusterNamespace()
			bucket := createObjectStorageConfig(namespace, s3BucketSpec(nil))
			preset := createElasticsearchClusterPreset(smallClusterSpec())
			cluster := validElasticsearchCluster()
			cluster.Name = name
			cluster.Namespace = namespace
			cluster.Spec.PresetRef = preset.Name
			cluster.Spec.SnapshotStorageRef = bucket.Name
			createECKSecrets(cluster)
			createElasticsearchCluster(cluster)
			expectRepositoryRegistered(cluster)

			return cluster
		}

		clusters := []*v1.ElasticsearchCluster{serving(), serving()}
		Expect(components.RepositoryName(clusters[0])).NotTo(Equal(components.RepositoryName(clusters[1])))

		// Both clusters keep reconciling. Neither registration moves to the
		// prefix of the other cluster, which is what a shared name produced.
		Consistently(func(g Gomega) {
			for _, cluster := range clusters {
				repo := elasticsearch.Repository(components.RepositoryName(cluster))
				g.Expect(repo).NotTo(BeNil(), cluster.Namespace)
				g.Expect(repo.Settings).To(HaveKeyWithValue(
					"base_path", "clusters/"+cluster.Namespace+"/"+cluster.Name,
				))
			}
		}, "2s", interval).Should(Succeed())
	})

	// A gcs bucket registers a gcs repository under the same per-cluster
	// prefix. Its credentials never appear in the settings: Elasticsearch
	// reads them from the node keystore alone, whatever the type.
	It("registers a gcs repository for a gcs bucket", func() {
		namespace := newElasticsearchClusterNamespace()
		bucket := createObjectStorageConfig(namespace, gcsBucketSpec(nil))
		cluster := servingClusterWithBucket(namespace, bucket.Name)

		expectRepositoryRegistered(cluster)

		repo := elasticsearch.Repository(components.RepositoryName(cluster))
		Expect(repo).NotTo(BeNil())
		Expect(repo.Type).To(Equal("gcs"))
		Expect(repo.Settings).To(HaveKeyWithValue("bucket", "camunda-backups"))
		Expect(repo.Settings).To(HaveKeyWithValue(
			"base_path", "clusters/"+cluster.Namespace+"/"+cluster.Name,
		))
	})

	// An azure repository addresses the blob container, and names it under
	// the container setting rather than bucket.
	It("registers an azure repository for an azure container", func() {
		namespace := newElasticsearchClusterNamespace()
		bucket := createObjectStorageConfig(namespace, azureBucketSpec("", nil))
		cluster := servingClusterWithBucket(namespace, bucket.Name)

		expectRepositoryRegistered(cluster)

		repo := elasticsearch.Repository(components.RepositoryName(cluster))
		Expect(repo).NotTo(BeNil())
		Expect(repo.Type).To(Equal("azure"))
		Expect(repo.Settings).To(HaveKeyWithValue("container", "camunda-backups"))
		Expect(repo.Settings).To(HaveKeyWithValue(
			"base_path", "clusters/"+cluster.Namespace+"/"+cluster.Name,
		))
		Expect(repo.Settings).NotTo(HaveKey("bucket"))
	})

	// The keystore Secret is rendered from the bucket's credentials Secret,
	// which carries no owner reference to the cluster. Only a watch brings
	// the controller back when it rotates; without one, the nodes keep the
	// old keys until an unrelated event.
	It("re-renders the keystore when the bucket credentials rotate", func() {
		namespace := newElasticsearchClusterNamespace()
		credentials := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "minio-credentials-" + utilrand.String(8), Namespace: namespace,
			},
			Data: map[string][]byte{
				"accessKeyId":     []byte("old-access"),
				"secretAccessKey": []byte("old-secret"),
			},
		}
		Expect(k8sClient.Create(ctx, credentials)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, credentials) })

		bucket := createObjectStorageConfig(namespace, s3BucketSpec(&v1.S3Credentials{
			SecretRef: v1.S3CredentialsSecretRef{
				Name:               credentials.Name,
				AccessKeyIDKey:     "accessKeyId",
				SecretAccessKeyKey: "secretAccessKey",
			},
		}))
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.SnapshotStorageRef = bucket.Name
		cluster.Namespace = namespace
		createECKSecrets(cluster)
		createElasticsearchCluster(cluster)

		keystoreKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-es-snapshot-keystore"}
		Eventually(func(g Gomega) {
			var keystore corev1.Secret
			g.Expect(k8sClient.Get(ctx, keystoreKey, &keystore)).To(Succeed())
			g.Expect(keystore.Data["s3.client.default.access_key"]).To(Equal([]byte("old-access")))
		}, timeout, interval).Should(Succeed())

		var fresh corev1.Secret
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(credentials), &fresh)).To(Succeed())
		fresh.Data["accessKeyId"] = []byte("new-access")
		fresh.Data["secretAccessKey"] = []byte("new-secret")
		Expect(k8sClient.Update(ctx, &fresh)).To(Succeed())

		Eventually(func(g Gomega) {
			var keystore corev1.Secret
			g.Expect(k8sClient.Get(ctx, keystoreKey, &keystore)).To(Succeed())
			g.Expect(keystore.Data["s3.client.default.access_key"]).To(Equal([]byte("new-access")))
			g.Expect(keystore.Data["s3.client.default.secret_key"]).To(Equal([]byte("new-secret")))
		}, timeout, interval).Should(Succeed())
	})

	// Suspension deletes the cluster, so there is nothing to register and
	// nothing to assert; a failure condition would drag Ready false, and
	// suspension is a Ready=True state by design.
	It("reports no repository condition while suspended, even with a bucket", func() {
		namespace := newElasticsearchClusterNamespace()
		bucket := createObjectStorageConfig(namespace, s3BucketSpec(nil))
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.SnapshotStorageRef = bucket.Name
		cluster.Spec.Suspend = true
		cluster.Namespace = namespace
		createElasticsearchCluster(cluster)

		Eventually(func(g Gomega) {
			var fetched v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &fetched)).To(Succeed())
			g.Expect(fetched.Status.Conditions).NotTo(BeEmpty())
			g.Expect(meta.FindStatusCondition(
				fetched.Status.Conditions, components.ConditionSnapshotRepository,
			)).To(BeNil())
		}, timeout, interval).Should(Succeed())
	})

	It("drops the repository condition when the bucket reference is removed", func() {
		namespace := newElasticsearchClusterNamespace()
		bucket := createObjectStorageConfig(namespace, s3BucketSpec(nil))
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.SnapshotStorageRef = bucket.Name
		cluster.Namespace = namespace
		createECKSecrets(cluster)
		createElasticsearchCluster(cluster)

		Eventually(func(g Gomega) {
			var fetched v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &fetched)).To(Succeed())
			g.Expect(meta.FindStatusCondition(
				fetched.Status.Conditions, components.ConditionSnapshotRepository,
			)).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			var fetched v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &fetched)).To(Succeed())
			fetched.Spec.SnapshotStorageRef = ""
			g.Expect(k8sClient.Update(ctx, &fetched)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			var after v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &after)).To(Succeed())
			g.Expect(meta.FindStatusCondition(
				after.Status.Conditions, components.ConditionSnapshotRepository,
			)).To(BeNil())
		}, timeout, interval).Should(Succeed())
	})

	// A suspended cluster is deleted and its pods do not run, so a bucket
	// Secret that disappears during the suspension must not flap Ready:
	// suspension reports Ready=True by design.
	It("stays Suspended when the bucket Secret disappears during suspension", func() {
		namespace := newElasticsearchClusterNamespace()
		credentials := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "minio-credentials-" + utilrand.String(8), Namespace: namespace,
			},
			Data: map[string][]byte{
				"accessKeyId":     []byte("access"),
				"secretAccessKey": []byte("secret"),
			},
		}
		Expect(k8sClient.Create(ctx, credentials)).To(Succeed())

		bucket := createObjectStorageConfig(namespace, s3BucketSpec(&v1.S3Credentials{
			SecretRef: v1.S3CredentialsSecretRef{
				Name:               credentials.Name,
				AccessKeyIDKey:     "accessKeyId",
				SecretAccessKeyKey: "secretAccessKey",
			},
		}))
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.SnapshotStorageRef = bucket.Name
		cluster.Spec.Suspend = true
		cluster.Namespace = namespace
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(
			cluster, metav1.ConditionTrue, Equal(string(component.Suspended)), HavePrefix("elasticsearch"),
		)

		Expect(k8sClient.Delete(ctx, credentials)).To(Succeed())

		Consistently(func(g Gomega) {
			var fetched v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &fetched)).To(Succeed())
			ready := meta.FindStatusCondition(fetched.Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(ready.Reason).To(Equal(string(component.Suspended)))
		}, "2s", interval).Should(Succeed())
	})

	// A True repository condition must not survive the loss of the bucket it
	// asserts: status would claim a registered repository nobody can verify.
	It("drops the repository condition when the bucket contract is deleted", func() {
		namespace := newElasticsearchClusterNamespace()
		bucket := createObjectStorageConfig(namespace, s3BucketSpec(nil))
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.SnapshotStorageRef = bucket.Name
		cluster.Namespace = namespace
		createECKSecrets(cluster)
		createElasticsearchCluster(cluster)

		Eventually(func(g Gomega) {
			var fetched v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &fetched)).To(Succeed())
			condition := meta.FindStatusCondition(
				fetched.Status.Conditions, components.ConditionSnapshotRepository,
			)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, bucket)).To(Succeed())

		Eventually(func(g Gomega) {
			var fetched v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &fetched)).To(Succeed())
			ready := meta.FindStatusCondition(fetched.Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(v1.ReasonInvalidReference))
			g.Expect(meta.FindStatusCondition(
				fetched.Status.Conditions, components.ConditionSnapshotRepository,
			)).To(BeNil())
		}, timeout, interval).Should(Succeed())
	})

	// A fleet cluster inherits its bucket from a preset, so the watches must
	// see through the preset: the raw spec never names the bucket.
	It("re-renders the keystore on rotation when the preset provides the bucket", func() {
		namespace := newElasticsearchClusterNamespace()
		credentials := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: "minio-credentials-" + utilrand.String(8), Namespace: namespace,
			},
			Data: map[string][]byte{
				"accessKeyId":     []byte("old-access"),
				"secretAccessKey": []byte("old-secret"),
			},
		}
		Expect(k8sClient.Create(ctx, credentials)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, credentials) })

		bucket := createObjectStorageConfig(namespace, s3BucketSpec(&v1.S3Credentials{
			SecretRef: v1.S3CredentialsSecretRef{
				Name:               credentials.Name,
				AccessKeyIDKey:     "accessKeyId",
				SecretAccessKeyKey: "secretAccessKey",
			},
		}))

		presetSpec := smallClusterSpec()
		presetSpec.SnapshotStorageRef = bucket.Name
		preset := createElasticsearchClusterPreset(presetSpec)

		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Namespace = namespace
		createECKSecrets(cluster)
		createElasticsearchCluster(cluster)

		keystoreKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-es-snapshot-keystore"}
		Eventually(func(g Gomega) {
			var keystore corev1.Secret
			g.Expect(k8sClient.Get(ctx, keystoreKey, &keystore)).To(Succeed())
			g.Expect(keystore.Data["s3.client.default.access_key"]).To(Equal([]byte("old-access")))
		}, timeout, interval).Should(Succeed())

		var fresh corev1.Secret
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(credentials), &fresh)).To(Succeed())
		fresh.Data["accessKeyId"] = []byte("new-access")
		Expect(k8sClient.Update(ctx, &fresh)).To(Succeed())

		Eventually(func(g Gomega) {
			var keystore corev1.Secret
			g.Expect(k8sClient.Get(ctx, keystoreKey, &keystore)).To(Succeed())
			g.Expect(keystore.Data["s3.client.default.access_key"]).To(Equal([]byte("new-access")))
		}, timeout, interval).Should(Succeed())
	})

	// A pre-existing ServiceAccount is the user's. In ocf a gated-off resource
	// is a deletion target, so holding the account behind a false gate would
	// delete the very object create: false promises never to touch.
	It("never deletes a pre-existing ServiceAccount", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Namespace = newElasticsearchClusterNamespace()

		foreign := &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:        "platform-sa",
				Namespace:   cluster.Namespace,
				Annotations: map[string]string{"platform.example.com/owner": "someone-else"},
			},
		}
		Expect(k8sClient.Create(ctx, foreign)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, foreign) })

		no := false
		cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Name: "platform-sa", Create: &no}
		createElasticsearchCluster(cluster)

		// The cluster reconciles and its pods reference the account.
		var es esv1.Elasticsearch
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).To(Succeed())
			g.Expect(es.Spec.NodeSets[0].PodTemplate.Spec.ServiceAccountName).To(Equal("platform-sa"))
		}, timeout, interval).Should(Succeed())

		// The account survives every reconcile: not deleted, not adopted, not
		// annotated.
		Consistently(func(g Gomega) {
			var account corev1.ServiceAccount
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKeyFromObject(foreign), &account,
			)).To(Succeed())
			g.Expect(account.OwnerReferences).To(BeEmpty())
			g.Expect(account.Annotations).To(Equal(map[string]string{
				"platform.example.com/owner": "someone-else",
			}))
			g.Expect(account.Labels).NotTo(HaveKey("app.kubernetes.io/managed-by"))
		}, "3s", interval).Should(Succeed())
	})

	// Pod Identity and Workload Identity Federation bind the ServiceAccount by
	// name on the cloud side. The documented principal must exist and be
	// referenced even though there is nothing to annotate onto it.
	It("renders and references the ServiceAccount for an annotation-less identity", func() {
		namespace := newElasticsearchClusterNamespace()
		bucket := createObjectStorageConfig(namespace, v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "camunda-backups",
				Region:     "eu-west-1",
				Auth:       v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
			},
		})
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.SnapshotStorageRef = bucket.Name
		cluster.Namespace = namespace
		createECKSecrets(cluster)
		createElasticsearchCluster(cluster)

		var account corev1.ServiceAccount
		key := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-es"}
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, key, &account)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Expect(account.Annotations).NotTo(HaveKey(v1.IRSARoleARNAnnotation))

		var es esv1.Elasticsearch
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).To(Succeed())
			g.Expect(es.Spec.NodeSets[0].PodTemplate.Spec.ServiceAccountName).To(Equal(cluster.Name + "-es"))
		}, timeout, interval).Should(Succeed())
	})

	// Without a bucket there is no repository, so the condition must not
	// appear at all rather than sit permanently unknown.
	It("reports no SnapshotRepositoryReady condition without a snapshot bucket", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)

		Eventually(func(g Gomega) {
			var fetched v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &fetched)).To(Succeed())
			g.Expect(fetched.Status.Conditions).NotTo(BeEmpty())
			g.Expect(meta.FindStatusCondition(
				fetched.Status.Conditions, components.ConditionSnapshotRepository,
			)).To(BeNil())
		}, timeout, interval).Should(Succeed())
	})

	It("names every missing field of an incomplete merge", func() {
		preset := createElasticsearchClusterPreset(v1.ElasticsearchClusterSpec{})
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonInvalidReference),
			And(ContainSubstring("replicas"), ContainSubstring("storageSize")),
		)
	})

	It("runs the version of the release when the cluster sets none", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		release := createElasticsearchRelease("9.2.4")
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.ReleaseRef = release.Name
		cluster.Spec.Version = ""
		createElasticsearchCluster(cluster)

		es := fetchOwnedElasticsearch(cluster)
		Expect(es.Spec.Version).To(Equal("9.2.4"))
	})

	It("runs its own version over the one the release names", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		release := createElasticsearchRelease("9.2.4")
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.ReleaseRef = release.Name
		cluster.Spec.Version = "9.2.5"
		createElasticsearchCluster(cluster)

		es := fetchOwnedElasticsearch(cluster)
		Expect(es.Spec.Version).To(Equal("9.2.5"))
	})

	It("reports InvalidReference for a dangling releaseRef and applies nothing", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.ReleaseRef = "does-not-exist-" + utilrand.String(8)
		cluster.Spec.Version = ""
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonInvalidReference), ContainSubstring(cluster.Spec.ReleaseRef),
		)

		var es esv1.Elasticsearch
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).NotTo(Succeed())
	})

	It("enforces the version floor on a version the release names", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		release := createElasticsearchRelease(versionBelowFloor)
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.ReleaseRef = release.Name
		cluster.Spec.Version = ""
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonInvalidReference), ContainSubstring(versionBelowFloor),
		)
	})

	It("flows a release edit to referencing clusters", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		release := createElasticsearchRelease("9.2.4")
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.ReleaseRef = release.Name
		cluster.Spec.Version = ""
		createElasticsearchCluster(cluster)
		fetchOwnedElasticsearch(cluster)

		Eventually(func(g Gomega) {
			var latest v1.CamundaRelease
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(release), &latest)).To(Succeed())
			latest.Spec.Elasticsearch.Version = "9.2.5"
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			var es esv1.Elasticsearch
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).To(Succeed())
			g.Expect(es.Spec.Version).To(Equal("9.2.5"))
		}, timeout, interval).Should(Succeed())
	})

	It("enforces the version floor on the merged result", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.Version = versionBelowFloor
		createElasticsearchCluster(cluster)

		expectElasticsearchClusterReady(
			cluster, metav1.ConditionFalse,
			Equal(v1.ReasonInvalidReference), ContainSubstring(versionBelowFloor),
		)
	})

	It("suspends by deleting the ECK CR with retained volumes, and recreates it on resume", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		es := fetchOwnedElasticsearch(cluster)
		// The default retention policy deletes the volumes with the cluster.
		Expect(es.Spec.VolumeClaimDeletePolicy).To(Equal(esv1.DeleteOnScaledownAndClusterDeletionPolicy))

		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			latest.Spec.Suspend = true
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// Suspension first switches the applied CR to the retaining policy.
		expectRetainingPolicy(cluster)

		// The CR is deleted only once ECK has observed the retaining policy.
		// envtest runs no ECK, so the spec stamps the observed generation.
		updateECKStatus(cluster, func(es *esv1.Elasticsearch) {
			es.Status.Phase = esv1.ElasticsearchReadyPhase
		})

		Eventually(func(g Gomega) {
			var latest esv1.Elasticsearch
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "the Elasticsearch CR must be deleted on suspension")
		}, timeout, interval).Should(Succeed())

		// Suspended is Ready=True: the cluster is in its desired state.
		expectElasticsearchClusterReady(
			cluster, metav1.ConditionTrue,
			Equal(string(component.Suspended)), HavePrefix("elasticsearch: "),
		)

		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			latest.Spec.Suspend = false
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// Resume recreates the CR; ECK reattaches the retained volumes by name.
		fetchOwnedElasticsearch(cluster)
	})

	It("renders the retaining volume policy when the retention policy is Retain", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.PersistentVolumeClaimRetentionPolicy = &v1.PersistentVolumeClaimRetentionPolicy{
			WhenDeleted: v1.RetainPersistentVolumeClaimRetentionPolicyType,
		}
		createElasticsearchCluster(cluster)

		es := fetchOwnedElasticsearch(cluster)
		Expect(es.Spec.VolumeClaimDeletePolicy).To(Equal(esv1.DeleteOnScaledownOnlyPolicy))
	})

	It("regenerates the password when the credentials Secret is deleted", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)

		secretKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-es-user"}
		var secret corev1.Secret
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, secretKey, &secret)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		oldPassword := string(secret.Data["password"])

		Expect(k8sClient.Delete(ctx, &secret)).To(Succeed())

		Eventually(func(g Gomega) {
			var recreated corev1.Secret
			g.Expect(k8sClient.Get(ctx, secretKey, &recreated)).To(Succeed())
			g.Expect(recreated.UID).NotTo(Equal(secret.UID))
			g.Expect(recreated.Data["password"]).To(HaveLen(32))
			g.Expect(string(recreated.Data["password"])).NotTo(Equal(oldPassword))
		}, timeout, interval).Should(Succeed())

		var contract v1.SecondaryStorageConfig
		Expect(k8sClient.Get(
			ctx, client.ObjectKey{
				Namespace: cluster.Namespace, Name: cluster.Spec.SecondaryStorageConfig,
			}, &contract,
		)).To(Succeed())
		Expect(contract.Spec.Elasticsearch.CredentialsSecretRef.Name).To(Equal(secretKey.Name))
	})

	It("flows a preset edit to referencing clusters without touching the CR", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		fetchOwnedElasticsearch(cluster)

		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchClusterPreset
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(preset), &latest)).To(Succeed())
			latest.Spec.Cluster.Replicas = new(int32(2))
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			var es esv1.Elasticsearch
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &es)).To(Succeed())
			g.Expect(es.Spec.NodeSets[0].Count).To(Equal(int32(2)))
		}, timeout, interval).Should(Succeed())
	})

	It("ignores a preset-driven storageSize shrink, keeps the applied size, and records an event", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		fetchOwnedElasticsearch(cluster)

		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchClusterPreset
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(preset), &latest)).To(Succeed())
			latest.Spec.Cluster.StorageSize = new(resource.MustParse("512Mi"))
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		expectStorageShrinkIgnored(cluster, "1Gi")
	})

	It("ignores an inline storageSize below the applied preset baseline", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		fetchOwnedElasticsearch(cluster)

		// Admission cannot catch this shrink. storageSize was unset inline
		// before, so the CEL transition rule does not fire.
		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			latest.Spec.StorageSize = new(resource.MustParse("512Mi"))
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		expectStorageShrinkIgnored(cluster, "1Gi")
	})

	It("ignores a preset shrink made during suspension and resumes with the retained volume size", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		fetchOwnedElasticsearch(cluster)

		// The retained data volume of a node. envtest runs no ECK, so the
		// spec creates the claim ECK would have created and stamps its size.
		claim := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      components.DataVolumeClaimName + "-" + cluster.Name + "-es-default-0",
				Namespace: cluster.Namespace,
				Labels:    map[string]string{components.ECKClusterNameLabel: cluster.Name},
			},
			Spec: corev1.PersistentVolumeClaimSpec{
				AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Resources: corev1.VolumeResourceRequirements{
					Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")},
				},
			},
		}
		Expect(k8sClient.Create(ctx, claim)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, claim) })
		claim.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("1Gi")}
		claim.Status.Phase = corev1.ClaimBound
		Expect(k8sClient.Status().Update(ctx, claim)).To(Succeed())

		// Suspend: the ECK CR goes away, the claim stays.
		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			latest.Spec.Suspend = true
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		expectRetainingPolicy(cluster)
		updateECKStatus(cluster, func(es *esv1.Elasticsearch) { es.Status.Phase = esv1.ElasticsearchReadyPhase })
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &esv1.Elasticsearch{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		// Shrink the preset while suspended, then resume.
		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchClusterPreset
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(preset), &latest)).To(Succeed())
			latest.Spec.Cluster.StorageSize = new(resource.MustParse("512Mi"))
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			latest.Spec.Suspend = false
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		// The recreated ECK CR requests the retained size, not the shrunk one.
		expectStorageShrinkIgnored(cluster, "1Gi")
	})

	It("reports every bound data volume with its capacity in status.volumes", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)
		fetchOwnedElasticsearch(cluster)

		// Before any claim reports a capacity, nothing is listed.
		expectVolumes(cluster, map[string]string{})

		// A data claim that ECK labels with the cluster name reports its
		// capacity, for example after a resize outside the spec. envtest runs
		// no ECK, so the spec creates the claims and stamps their capacities.
		// The claims grow apart, so the status must list them one by one.
		claim0 := createDataClaim(cluster, "0", "2Gi")
		expectVolumes(cluster, map[string]string{claim0.Name: "2Gi"})

		claim1 := createDataClaim(cluster, "1", "1Gi")
		expectVolumes(cluster, map[string]string{claim0.Name: "2Gi", claim1.Name: "1Gi"})
	})

	It("deploys the metrics exporter while monitoring is enabled and removes it when disabled", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		cluster.Spec.Monitoring = &v1.MonitoringSpec{ServiceMonitor: &v1.ServiceMonitorSpec{Enabled: true}}
		createElasticsearchCluster(cluster)
		fetchOwnedElasticsearch(cluster)

		exporterKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-es-exporter"}
		metricsKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-es-metrics"}

		var exporter appsv1.Deployment
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(ctx, exporterKey, &exporter)).To(Succeed())
		}, timeout, interval).Should(Succeed())
		expectControlledBy(&exporter, cluster)
		container := exporter.Spec.Template.Spec.Containers[0]
		Expect(container.Image).To(Equal(components.DefaultExporterImage))
		Expect(container.Args).To(ContainElement(
			"--es.uri=https://" + cluster.Name + "-es-http." + cluster.Namespace + ".svc:9200",
		))
		Expect(container.Env).To(ContainElement(HaveField("Name", "ES_PASSWORD")))

		var metrics corev1.Service
		Expect(k8sClient.Get(ctx, metricsKey, &metrics)).To(Succeed())
		expectControlledBy(&metrics, cluster)
		Expect(metrics.Spec.Ports).To(ConsistOf(HaveField("Port", int32(9114))))

		// Disabling monitoring flips the feature gate: the component deletes
		// its resources and reports Disabled, which Ready leaves out.
		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			latest.Spec.Monitoring.ServiceMonitor.Enabled = false
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, exporterKey, &appsv1.Deployment{}))).To(BeTrue())
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, metricsKey, &corev1.Service{}))).To(BeTrue())
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			metricsCond := meta.FindStatusCondition(latest.Status.Conditions, components.ConditionMetrics)
			g.Expect(metricsCond).NotTo(BeNil())
			g.Expect(metricsCond.Reason).To(Equal(string(component.Disabled)))
			ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Reason).NotTo(Equal(string(component.Disabled)))
		}, timeout, interval).Should(Succeed())
	})

	It("records the reconciled generation in status.observedGeneration", func() {
		preset := createElasticsearchClusterPreset(smallClusterSpec())
		cluster := validElasticsearchCluster()
		cluster.Spec.PresetRef = preset.Name
		createElasticsearchCluster(cluster)

		expectObservedGeneration := func() {
			GinkgoHelper()
			Eventually(func(g Gomega) {
				var latest v1.ElasticsearchCluster
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
				g.Expect(latest.Status.ObservedGeneration).To(Equal(latest.Generation))
			}, timeout, interval).Should(Succeed())
		}
		expectObservedGeneration()

		Eventually(func(g Gomega) {
			var latest v1.ElasticsearchCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			latest.Spec.PodLabels = map[string]string{"team": "platform"}
			g.Expect(k8sClient.Update(ctx, &latest)).To(Succeed())
			g.Expect(latest.Generation).To(BeNumerically(">", 1))
		}, timeout, interval).Should(Succeed())
		expectObservedGeneration()
	})

	// The rendered ECK CR must apply against the API server in envtest, so
	// the suite loads the ECK CRDs from the resolved module.
	It("accepts an ECK Elasticsearch resource in the test environment", func() {
		es := &esv1.Elasticsearch{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "eck-smoke-" + utilrand.String(8),
				Namespace: newElasticsearchClusterNamespace(),
			},
			Spec: esv1.ElasticsearchSpec{
				Version:  "9.2.4",
				NodeSets: []esv1.NodeSet{{Name: "default", Count: 1}},
			},
		}

		Expect(k8sClient.Create(ctx, es)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, es) })

		var fetched esv1.Elasticsearch
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(es), &fetched)).To(Succeed())
		Expect(fetched.Spec.Version).To(Equal("9.2.4"))
	})
})
