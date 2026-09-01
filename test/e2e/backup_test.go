//go:build e2e
// +build e2e

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

package e2e

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	escomponents "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
	"github.com/konsole-is/camunda-operator/test/utils"
)

const (
	// minioNamespace holds the object store of the backup flows. It is a
	// namespace of its own, so every flow reaches one store from a namespace
	// of its own, which is how an installation places a store next to several
	// clusters.
	minioNamespace = "minio-e2e"
	// backupStorage is the ObjectStorageConfig that every flow writes
	// through: a MinIO bucket addressed with an endpoint, path-style
	// addressing, and static credentials. The kind is namespaced, so each
	// flow gets a contract of this name in its own namespace.
	backupStorage = "camunda-backup-e2e"
	// backupBasePath is the key prefix of the contract. The operator writes
	// every artifact under <basePath>/<namespace>/<cluster>, so a non-empty
	// prefix proves the layout rather than the bucket root.
	backupBasePath = "camunda"
	// esBackupName and rdbmsBackupName are the backups of the two flows.
	esBackupName    = "camunda-es-backup"
	rdbmsBackupName = "camunda-rdbms-backup"

	oscResource     = "objectstorageconfigs.core.camunda.io"
	lbesResource    = "logicalbackupelasticsearches.core.camunda.io"
	lbrdbmsResource = "logicalbackuprdbmses.core.camunda.io"

	// backupTimeout bounds one backup procedure: the management calls, the
	// Elasticsearch snapshots, and on the relational path the pull of the
	// postgres image and the dump Job.
	backupTimeout = 15 * time.Minute
	// storeTimeout bounds one call against the object store.
	storeTimeout = 2 * time.Minute
)

// exporterPhaseRunning is the phase that the brokers report while exporting
// runs. The management endpoint of Camunda 8.9 has no read operation for the
// pause state, so the exporter phase of every partition is the evidence that
// exporting resumed.
const exporterPhaseRunning = "EXPORTING"

// createBackupStorage creates the access-key pair and the bucket contract in
// namespace, and waits for the contract to report Ready. Every cluster that
// writes to the store needs both of its own: the kind is namespaced, and it
// names its Secret in its own namespace.
func createBackupStorage(namespace string) {
	By("creating the bucket credentials and the ObjectStorageConfig of " + namespace)
	Expect(apply(minioCredentials(namespace))).To(Succeed(), "Failed to create the bucket credentials")
	Expect(apply(backupObjectStorage(namespace))).To(Succeed(), "Failed to create the ObjectStorageConfig")
	Eventually(func(g Gomega) {
		expectReady(g, oscResource, backupStorage, namespace, v1.ReasonHealthy)
	}, 2*time.Minute).Should(Succeed())
}

// minioCredentials returns the access-key pair of the store, in namespace. It
// holds the same values as the pair that testdata/minio.yaml creates for the
// server itself.
func minioCredentials(namespace string) *corev1.Secret {
	return &corev1.Secret{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{Name: utils.MinIOCredentialsSecret, Namespace: namespace},
		StringData: map[string]string{
			utils.MinIOAccessKeyIDKey:     utils.MinIOAccessKeyID,
			utils.MinIOSecretAccessKeyKey: utils.MinIOSecretAccessKey,
		},
	}
}

// backupObjectStorage returns the bucket contract of namespace. It is the
// shape that an S3-compatible store needs: the endpoint of the store,
// path-style addressing, and an access-key pair in a Secret.
func backupObjectStorage(namespace string) *v1.ObjectStorageConfig {
	return &v1.ObjectStorageConfig{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "ObjectStorageConfig"},
		ObjectMeta: metav1.ObjectMeta{Name: backupStorage, Namespace: namespace},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName:     utils.MinIOBucket,
				BasePath:       backupBasePath,
				Endpoint:       utils.MinIOEndpoint(minioNamespace),
				ForcePathStyle: true,
				Auth: v1.S3StorageAuth{
					Type: v1.ObjectStorageAuthTypeCredentials,
					Credentials: &v1.S3Credentials{
						SecretRef: v1.S3CredentialsSecretRef{
							Name:               utils.MinIOCredentialsSecret,
							AccessKeyIDKey:     utils.MinIOAccessKeyIDKey,
							SecretAccessKeyKey: utils.MinIOSecretAccessKeyKey,
						},
					},
				},
			},
		},
	}
}

// itBacksUpTheElasticsearchCluster registers the backup specs of the
// Elasticsearch flow. The CamundaCluster flow calls it while its cluster is
// healthy, so the specs run against a cluster that has exported a process
// instance.
//
// The specs are ordered: the repository has to be registered before a backup
// can run, and the deletion spec removes what the backup spec created.
func itBacksUpTheElasticsearchCluster(cluster *v1.CamundaCluster, elasticsearch, storageConfig string) {
	// backup is the completed backup, as the deletion spec observed it.
	var backup v1.LogicalBackupElasticsearch

	It("registers the snapshot repository and publishes it to the cluster", func() {
		By("waiting for SnapshotRepositoryReady on the ElasticsearchCluster")
		Eventually(func(g Gomega) {
			expectCondition(
				g, esResource, elasticsearch, cluster.Namespace,
				escomponents.ConditionSnapshotRepository, v1.ReasonHealthy,
			)
		}, esReadyTimeout, 5*time.Second).Should(Succeed())

		By("reading the repository name from the storage contract")
		var contract v1.SecondaryStorageConfig
		Expect(utils.Get(sscResource, storageConfig, cluster.Namespace, &contract)).To(Succeed())
		Expect(contract.Spec.Elasticsearch).NotTo(BeNil())
		Expect(contract.Spec.Elasticsearch.SnapshotRepository).To(Equal(esRepository(cluster, elasticsearch)))

		By("waiting for the cluster to publish the repository in its management binding")
		Eventually(func(g Gomega) {
			var got v1.CamundaCluster
			g.Expect(utils.Get(ccResource, cluster.Name, cluster.Namespace, &got)).To(Succeed())
			g.Expect(got.Status.Management).NotTo(BeNil())
			g.Expect(got.Status.Management.BackupRepository).To(Equal(esRepository(cluster, elasticsearch)))
		}, ccAPITimeout, 5*time.Second).Should(Succeed())
	})

	It("completes a LogicalBackupElasticsearch with its snapshots and its runtime objects", func() {
		By("creating the LogicalBackupElasticsearch")
		Expect(apply(&v1.LogicalBackupElasticsearch{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "LogicalBackupElasticsearch",
			},
			ObjectMeta: metav1.ObjectMeta{Name: esBackupName, Namespace: cluster.Namespace},
			Spec: v1.LogicalBackupElasticsearchSpec{
				ClusterRef: v1.ClusterRef{Name: cluster.Name},
			},
		})).To(Succeed())

		By("waiting for Ready Completed")
		Eventually(func(g Gomega) {
			expectReady(g, lbesResource, esBackupName, cluster.Namespace, v1.ReasonCompleted)
		}, backupTimeout, 5*time.Second).Should(Succeed())

		By("reading the identity of the backup set")
		Expect(utils.Get(lbesResource, esBackupName, cluster.Namespace, &backup)).To(Succeed())
		Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupCompleted))
		Expect(backup.Status.BackupID).NotTo(BeZero())
		Expect(backup.Status.Repository).To(Equal(esRepository(cluster, elasticsearch)))
		Expect(backup.Status.HistorySnapshots).NotTo(BeEmpty())
		Expect(backup.Status.History.State).To(Equal(v1.BackupPartCompleted))
		Expect(backup.Status.Records.State).To(Equal(v1.BackupPartCompleted))
		Expect(backup.Status.Runtime.State).To(Equal(v1.BackupPartCompleted))

		By("finding the web-application and record snapshots in Elasticsearch")
		Eventually(func(g Gomega) {
			names, err := snapshotNames(cluster.Namespace, storageConfig, backup.Status.Repository)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(names).To(ContainElements(toAny(backup.Status.HistorySnapshots)...))
			g.Expect(names).To(ContainElement(logicalbackup.RecordsSnapshotName(backup.Status.BackupID)))
		}, 3*time.Minute, 5*time.Second).Should(Succeed())

		By("finding the partition backup objects in MinIO")
		objects, err := runtimeBackupObjects(cluster, backup.Status.BackupID)
		Expect(err).NotTo(HaveOccurred())
		Expect(objects).NotTo(BeEmpty(), "the Zeebe backup wrote no object under the prefix of the cluster")
	})

	It("runs exporting again once the backup is done", func() {
		Eventually(func(g Gomega) {
			phases, err := exporterPhases(cluster)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(phases).NotTo(BeEmpty(), "no partition reported an exporter phase")
			g.Expect(phases).To(HaveEach(exporterPhaseRunning))
		}, ccAPITimeout, 5*time.Second).Should(Succeed())
	})

	It("removes its snapshots and its runtime objects when the backup is deleted", func() {
		By("deleting the LogicalBackupElasticsearch")
		_, err := utils.Kubectl(
			"delete", lbesResource, esBackupName, "-n", cluster.Namespace, "--wait=false",
		)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the finalizer to release the resource")
		Eventually(func(g Gomega) {
			expectGone(g, lbesResource, esBackupName, cluster.Namespace)
		}, 5*time.Minute, 5*time.Second).Should(Succeed())

		By("checking that the snapshots are gone from Elasticsearch")
		names, err := snapshotNames(cluster.Namespace, storageConfig, backup.Status.Repository)
		Expect(err).NotTo(HaveOccurred())
		for _, name := range backup.Status.HistorySnapshots {
			Expect(names).NotTo(ContainElement(name))
		}
		Expect(names).NotTo(ContainElement(logicalbackup.RecordsSnapshotName(backup.Status.BackupID)))

		By("checking that the partition backup objects are gone from MinIO")
		objects, err := runtimeBackupObjects(cluster, backup.Status.BackupID)
		Expect(err).NotTo(HaveOccurred())
		Expect(objects).To(BeEmpty())
	})
}

// itBacksUpTheRelationalCluster registers the backup specs of the RDBMS flow.
// The CamundaCluster flow calls it while its cluster is healthy.
func itBacksUpTheRelationalCluster(cluster *v1.CamundaCluster) {
	// backup is the completed backup, as the deletion spec observed it.
	var backup v1.LogicalBackupRDBMS

	It("completes a LogicalBackupRDBMS with a non-empty dump and a Zeebe backup", func() {
		// The dump Job runs under the account the cluster publishes. The
		// bucket of this flow holds static credentials, so the cluster binds
		// no cloud identity and renders no account. A Job that named one
		// here would name an account that does not exist, and the API server
		// would never create its pod.
		By("reading the empty account that a cluster on static credentials publishes")
		var published v1.CamundaCluster
		Expect(utils.Get(ccResource, cluster.Name, cluster.Namespace, &published)).To(Succeed())
		Expect(published.Status.ServiceAccountName).To(BeEmpty())

		By("creating the LogicalBackupRDBMS")
		Expect(apply(&v1.LogicalBackupRDBMS{
			TypeMeta: metav1.TypeMeta{
				APIVersion: v1.GroupVersion.String(),
				Kind:       "LogicalBackupRDBMS",
			},
			ObjectMeta: metav1.ObjectMeta{Name: rdbmsBackupName, Namespace: cluster.Namespace},
			Spec: v1.LogicalBackupRDBMSSpec{
				ClusterRef: v1.ClusterRef{Name: cluster.Name},
			},
		})).To(Succeed())

		By("waiting for Ready Completed")
		Eventually(func(g Gomega) {
			expectReady(g, lbrdbmsResource, rdbmsBackupName, cluster.Namespace, v1.ReasonCompleted)
		}, backupTimeout, 5*time.Second).Should(Succeed())

		By("reading the identity of the backup")
		Expect(utils.Get(lbrdbmsResource, rdbmsBackupName, cluster.Namespace, &backup)).To(Succeed())
		Expect(backup.Status.Phase).To(Equal(v1.LogicalBackupCompleted))
		Expect(backup.Status.BackupID).NotTo(BeZero())
		Expect(backup.Status.ObjectKey).NotTo(BeEmpty())
		Expect(backup.Status.BucketRef).To(Equal(backupStorage))
		Expect(backup.Status.ZeebeBackupID).NotTo(BeNil())
		Expect(*backup.Status.ZeebeBackupID).NotTo(BeZero())

		By("finding a non-empty dump in MinIO")
		objects, err := utils.MinIOObjectsWithPrefix(minioNamespace, backup.Status.ObjectKey, storeTimeout)
		Expect(err).NotTo(HaveOccurred())
		Expect(objects).To(HaveLen(1), "expected exactly the dump at %q", backup.Status.ObjectKey)
		Expect(objects[0].Key).To(Equal(backup.Status.ObjectKey))
		Expect(objects[0].Size).To(BeNumerically(">", 0), "the dump is empty")
	})

	It("removes the dump when the backup is deleted", func() {
		By("deleting the LogicalBackupRDBMS")
		_, err := utils.Kubectl(
			"delete", lbrdbmsResource, rdbmsBackupName, "-n", cluster.Namespace, "--wait=false",
		)
		Expect(err).NotTo(HaveOccurred())

		By("waiting for the finalizer to release the resource")
		Eventually(func(g Gomega) {
			expectGone(g, lbrdbmsResource, rdbmsBackupName, cluster.Namespace)
		}, 5*time.Minute, 5*time.Second).Should(Succeed())

		By("checking that the dump is gone from MinIO")
		objects, err := utils.MinIOObjectsWithPrefix(minioNamespace, backup.Status.ObjectKey, storeTimeout)
		Expect(err).NotTo(HaveOccurred())
		Expect(objects).To(BeEmpty())
	})
}

// esRepository returns the snapshot repository that the ElasticsearchCluster
// named elasticsearch registers. Both flows run their Elasticsearch in the
// namespace of their cluster, and the repository name carries that namespace
// so that two clusters of one name never meet on one registration.
func esRepository(cluster *v1.CamundaCluster, elasticsearch string) string {
	return cluster.Namespace + "." + elasticsearch
}

// runtimeBackupObjects returns the objects that the Zeebe backup of id wrote
// into the bucket. The backup store of Zeebe owns the layout under the prefix
// of the cluster and carries the backup id as one path segment of every key,
// so the id selects the set without this suite restating that layout.
func runtimeBackupObjects(cluster *v1.CamundaCluster, id int64) ([]utils.MinIOObject, error) {
	prefix := logicalbackup.ClusterPrefix(backupBasePath, cluster.Namespace, cluster.Name)
	objects, err := utils.MinIOObjectsWithPrefix(minioNamespace, prefix+"/", storeTimeout)
	if err != nil {
		return nil, err
	}

	segment := "/" + strconv.FormatInt(id, 10) + "/"
	var matched []utils.MinIOObject
	for _, object := range objects {
		if strings.Contains(object.Key, segment) {
			matched = append(matched, object)
		}
	}

	return matched, nil
}

// snapshotNames returns the names of every snapshot that the repository
// holds, read through the published storage contract the way a consumer
// reads it.
func snapshotNames(namespace, storageConfig, repository string) ([]string, error) {
	var contract v1.SecondaryStorageConfig
	if err := utils.Get(sscResource, storageConfig, namespace, &contract); err != nil {
		return nil, err
	}

	out, err := curlElasticsearch(&contract, "snapshots", "/_snapshot/"+repository+"/_all")
	if err != nil {
		return nil, err
	}

	var body struct {
		Snapshots []struct {
			Snapshot string `json:"snapshot"`
		} `json:"snapshots"`
	}
	if err := json.Unmarshal([]byte(out), &body); err != nil {
		return nil, fmt.Errorf("decoding the snapshots of repository %q: %w", repository, err)
	}

	names := make([]string, 0, len(body.Snapshots))
	for _, snapshot := range body.Snapshots {
		names = append(names, snapshot.Snapshot)
	}

	return names, nil
}

// exporterPhases returns the exporter phase of every partition that the
// brokers of cluster report, by partition id. Camunda 8.9 serves no read
// operation for the pause state of exporting. The partitions endpoint of the
// broker management port is the one actuator that answers it.
//
// A follower reports no phase at all. Such a partition is left out, so a
// caller that finds no phase has reached a cluster without a leader.
func exporterPhases(cluster *v1.CamundaCluster) (map[string]string, error) {
	url := fmt.Sprintf(
		"http://%s.%s.svc:%d/actuator/partitions",
		components.WorkloadName(cluster, components.ComponentZeebe),
		cluster.Namespace,
		components.PortManagement,
	)

	out, err := utils.RunPod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "curl-partitions-" + utilrand.String(5),
			Namespace: cluster.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  "curl",
				Image: utils.CurlImage,
				Args:  []string{"-fsS", url},
			}},
		},
	}, podTimeout)
	if err != nil {
		return nil, err
	}

	var partitions map[string]struct {
		ExporterPhase *string `json:"exporterPhase"`
	}
	if err := json.Unmarshal([]byte(out), &partitions); err != nil {
		return nil, fmt.Errorf("decoding the partitions of %s/%s: %w", cluster.Namespace, cluster.Name, err)
	}

	phases := map[string]string{}
	for id, partition := range partitions {
		if partition.ExporterPhase != nil {
			phases[id] = *partition.ExporterPhase
		}
	}

	return phases, nil
}

// toAny widens a slice of names for the Gomega matchers that take variadic
// interface arguments.
func toAny(values []string) []any {
	widened := make([]any, 0, len(values))
	for _, value := range values {
		widened = append(widened, value)
	}

	return widened
}
