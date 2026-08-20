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

package pointintimerestore

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/restore"
)

// world is the resolved fixture set of one spec: a suspended relational
// cluster, its whole storage chain, the one Database that owns the server, and
// the live broker StatefulSet with its data volumes.
type world struct {
	namespace   string
	cluster     *v1.CamundaCluster
	storage     *v1.SecondaryStorageConfig
	dbConfig    *v1.DatabaseConfig
	server      *v1.DatabaseServerConfig
	database    *v1.Database
	credentials *corev1.Secret
	brokers     *appsv1.StatefulSet
	claims      []*corev1.PersistentVolumeClaim
}

// brokerCount is how many brokers every world runs. It is what
// CAMUNDA_CLUSTER_SIZE on the broker container says, not spec.replicas: a
// suspended StatefulSet runs at zero replicas.
const brokerCount = 2

// partitionCount is what CAMUNDA_CLUSTER_PARTITIONCOUNT on the broker
// container says. The database-state check needs a position for each of them.
const partitionCount = 3

func newNamespace() string {
	GinkgoHelper()
	name := "pitr-ns-" + utilrand.String(8)
	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	})).To(Succeed())

	return name
}

// createWorld builds the state that a point-in-time restore is admitted
// against: a suspended relational cluster, a database server that declares
// point-in-time recovery with a seven-day retention, exactly one Database on
// that server, and the broker StatefulSet that the restore reads its facts
// from. The mutators shape the resources before any of them is created.
//
// The exporter reader answers for the logical database of this world with
// positions that lie behind the timestamp of a restore, so a world passes the
// database-state check unless a spec says otherwise.
func createWorld(mutate ...func(*world)) *world {
	GinkgoHelper()
	namespace := newNamespace()
	suffix := strings.ToLower(utilrand.String(6))

	platform := fixtures.CamundaPlatformConfigBasic()
	Expect(k8sClient.Create(ctx, platform)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, platform) })

	w := &world{namespace: namespace}

	w.credentials = &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "app-user", Namespace: namespace},
		Data:       map[string][]byte{"username": []byte("camunda"), "password": []byte("s3cr3t")},
	}

	w.server = &v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsc-" + suffix},
		Spec: v1.DatabaseServerConfigSpec{
			Engine: v1.DatabaseEnginePostgres,
			Host:   "postgres.databases.svc",
			Port:   5432,
			AdminCredentialsSecretRef: v1.CredentialsSecretRef{
				Name: "admin", Namespace: namespace,
				UsernameKey: "username", PasswordKey: "password",
			},
			PITR: &v1.PITRCapability{Enabled: true, RetentionPeriodDays: ptr(int32(7))},
		},
	}

	w.database = &v1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "db-" + suffix, Namespace: namespace},
		Spec: v1.DatabaseSpec{
			ServerRef:       w.server.Name,
			DatabaseName:    "camunda_" + suffix,
			TargetNamespace: namespace,
		},
	}

	w.dbConfig = &v1.DatabaseConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbc-" + suffix, Namespace: namespace},
		Spec: v1.DatabaseConfigSpec{
			ServerRef:    w.server.Name,
			DatabaseName: "camunda_" + suffix,
			CredentialsSecretRef: v1.CredentialsSecretRef{
				Name: "app-user", Namespace: namespace,
				UsernameKey: "username", PasswordKey: "password",
			},
		},
	}

	w.storage = &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "ssc-" + suffix, Namespace: namespace},
		Spec: v1.SecondaryStorageConfigSpec{
			Type:  v1.SecondaryStorageTypeRDBMS,
			RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: w.dbConfig.Name},
		},
	}

	w.cluster = &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cc-" + suffix, Namespace: namespace},
		Spec: v1.CamundaClusterSpec{
			Version:           "8.9.9",
			PlatformConfigRef: platform.Name,
			StorageRef:        w.storage.Name,
			BackupStorageRef:  "backups",
			Suspend:           true,
		},
	}

	for _, m := range mutate {
		m(w)
	}

	Expect(k8sClient.Create(ctx, w.credentials)).To(Succeed())
	Expect(k8sClient.Create(ctx, w.server)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, w.server) })
	Expect(k8sClient.Create(ctx, w.database)).To(Succeed())
	Expect(k8sClient.Create(ctx, w.dbConfig)).To(Succeed())
	Expect(k8sClient.Create(ctx, w.storage)).To(Succeed())
	Expect(k8sClient.Create(ctx, w.cluster)).To(Succeed())

	createBrokers(w)
	releaseTerminatingClaims(namespace)
	exporter.set(w.dbConfig.Spec.DatabaseName, answer{positions: positionsBehind(time.Hour)})

	return w
}

// createBrokers creates the broker StatefulSet of the world and the data
// volume of every broker, as the CamundaCluster controller and the StatefulSet
// controller leave them behind on a suspended cluster.
func createBrokers(w *world) {
	GinkgoHelper()
	name := components.WorkloadName(w.cluster, components.ComponentZeebe)
	w.brokers = &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: w.namespace},
		Spec: appsv1.StatefulSetSpec{
			ServiceName: name,
			Replicas:    ptr(int32(0)),
			Selector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  components.ContainerCamunda,
						Image: "camunda/camunda:8.9.9",
						Env: []corev1.EnvVar{
							camundaconfig.Var(camundaconfig.KeyClusterSize, "2"),
							camundaconfig.Var(camundaconfig.KeyClusterPartitionCount, "3"),
						},
						VolumeMounts: []corev1.VolumeMount{{
							Name: components.DataVolumeName, MountPath: "/usr/local/camunda/data",
						}},
					}},
				},
			},
			VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
				ObjectMeta: metav1.ObjectMeta{
					Name:      components.DataVolumeName,
					Namespace: w.namespace,
					Labels:    map[string]string{"app": name},
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("10Gi")},
					},
				},
			}},
		},
	}
	Expect(k8sClient.Create(ctx, w.brokers)).To(Succeed())

	for ordinal := range brokerCount {
		claim := &corev1.PersistentVolumeClaim{
			ObjectMeta: metav1.ObjectMeta{
				Name:      claimName(w, ordinal),
				Namespace: w.namespace,
				Labels:    map[string]string{"app": name},
			},
			Spec: *w.brokers.Spec.VolumeClaimTemplates[0].Spec.DeepCopy(),
		}
		Expect(k8sClient.Create(ctx, claim)).To(Succeed())
		w.claims = append(w.claims, claim)
	}
}

// releaseTerminatingClaims plays the volume protection of a real cluster: it
// clears the protection finalizer that the API server puts on every claim, as
// the cluster does once no pod holds the volume any more. Nothing does that in
// envtest, so a deleted claim would stay in place for ever and no restore
// would get past its first volume.
func releaseTerminatingClaims(namespace string) {
	stop := make(chan struct{})
	DeferCleanup(func() { close(stop) })

	go func() {
		defer GinkgoRecover()
		for {
			select {
			case <-stop:
				return
			case <-time.After(50 * time.Millisecond):
			}

			var claims corev1.PersistentVolumeClaimList
			if err := k8sClient.List(ctx, &claims, client.InNamespace(namespace)); err != nil {
				continue
			}
			for i := range claims.Items {
				claim := &claims.Items[i]
				if claim.DeletionTimestamp == nil || len(claim.Finalizers) == 0 {
					continue
				}
				claim.Finalizers = nil
				_ = k8sClient.Update(ctx, claim)
			}
		}
	}()
}

func claimName(w *world, ordinal int) string {
	return components.DataVolumeName + "-" +
		components.WorkloadName(w.cluster, components.ComponentZeebe) + "-" +
		strconv.Itoa(ordinal)
}

// positionsBehind returns one exporter position per partition, each the given
// duration behind the timestamp that restoreAt uses.
func positionsBehind(behind time.Duration) []v1.PartitionPosition {
	positions := make([]v1.PartitionPosition, 0, partitionCount)
	for partition := 1; partition <= partitionCount; partition++ {
		positions = append(positions, v1.PartitionPosition{
			PartitionID: int32(partition),
			LastUpdated: metav1.NewTime(restorePoint().Add(-behind)),
		})
	}

	return positions
}

// restorePoint is the point that every restore of a spec asks for, unless the
// spec says otherwise: one hour ago, which lies inside the retention period of
// the world and not in the future.
func restorePoint() time.Time {
	return suiteStart.Add(-time.Hour)
}

// suiteStart pins the clock of the fixtures, so the position of a fake
// exporter and the timestamp of a restore keep their distance while a spec
// runs.
var suiteStart = time.Now().Truncate(time.Second)

func createRestore(w *world, mutate ...func(*v1.PointInTimeRestore)) *v1.PointInTimeRestore {
	GinkgoHelper()
	pitr := &v1.PointInTimeRestore{
		ObjectMeta: metav1.ObjectMeta{
			Name: "pitr-" + strings.ToLower(utilrand.String(6)), Namespace: w.namespace,
		},
		Spec: v1.PointInTimeRestoreSpec{
			ClusterRef: v1.ClusterRef{Name: w.cluster.Name},
			Timestamp:  metav1.NewTime(restorePoint()),
		},
	}
	for _, m := range mutate {
		m(pitr)
	}
	Expect(k8sClient.Create(ctx, pitr)).To(Succeed())

	return pitr
}

func readRestore(pitr *v1.PointInTimeRestore) *v1.PointInTimeRestore {
	GinkgoHelper()
	var current v1.PointInTimeRestore
	Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(pitr), &current)).To(Succeed())

	return &current
}

func ready(pitr *v1.PointInTimeRestore) *metav1.Condition {
	return meta.FindStatusCondition(pitr.Status.Conditions, v1.ConditionReady)
}

// expectHeld asserts that the restore waits in Pending with the given reason,
// and returns the message it reported.
func expectHeld(pitr *v1.PointInTimeRestore, reason string) string {
	GinkgoHelper()
	var message string
	Eventually(func(g Gomega) {
		current := readRestore(pitr)
		g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestorePending))
		condition := ready(current)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Status).To(Equal(metav1.ConditionFalse))
		g.Expect(condition.Reason).To(Equal(reason))
		message = condition.Message
	}, timeout, interval).Should(Succeed())

	return message
}

// expectAdmitted asserts that the restore passed every admission rule: it left
// Pending and pinned the identity of its cluster.
func expectAdmitted(pitr *v1.PointInTimeRestore, w *world) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		current := readRestore(pitr)
		g.Expect(current.Status.Phase).NotTo(Equal(v1.PointInTimeRestorePending))
		g.Expect(current.Status.Phase).NotTo(Equal(v1.PointInTimeRestoreFailed))
		g.Expect(current.Status.ClusterUID).To(Equal(w.cluster.UID))
	}, timeout, interval).Should(Succeed())
}

// expectStartedHold asserts that the restore holds in the database-state
// phase with the given reason, and returns the message it reported. A restore
// holds there only while it waits for something that the mid-run grace bounds.
func expectStartedHold(pitr *v1.PointInTimeRestore, reason string) string {
	GinkgoHelper()
	var message string
	Eventually(func(g Gomega) {
		current := readRestore(pitr)
		g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreValidatingDatabaseState))
		condition := ready(current)
		g.Expect(condition).NotTo(BeNil())
		g.Expect(condition.Reason).To(Equal(reason))
		message = condition.Message
	}, timeout, interval).Should(Succeed())

	return message
}

// jobsOf lists the restore Jobs of pitr, selected the way every consumer of
// these Jobs must select them.
func jobsOf(pitr *v1.PointInTimeRestore) []batchv1.Job {
	GinkgoHelper()
	var jobs batchv1.JobList
	Expect(k8sClient.List(
		ctx, &jobs,
		client.InNamespace(pitr.Namespace),
		client.MatchingLabels(restore.JobSelector(labels.PointInTimeRestore(pitr.Name))),
	)).To(Succeed())

	return jobs.Items
}

// expectClaimsUntouched asserts that every broker volume is still the volume
// the world created.
func expectClaimsUntouched(w *world) {
	GinkgoHelper()
	for _, claim := range w.claims {
		var current corev1.PersistentVolumeClaim
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &current)).To(Succeed())
		Expect(current.UID).To(Equal(claim.UID), "the broker volume was deleted and created again")
		Expect(current.DeletionTimestamp).To(BeNil(), "the broker volume is being deleted")
	}
}

func ptr[T any](value T) *T { return &value }

var _ = Describe("PointInTimeRestore admission", func() {
	It("holds a restore whose cluster still runs, and touches nothing", func() {
		w := createWorld(func(w *world) { w.cluster.Spec.Suspend = false })
		pitr := createRestore(w)

		message := expectHeld(pitr, v1.ReasonClusterNotSuspended)
		Expect(message).To(ContainSubstring(w.cluster.Name))
		Consistently(func(g Gomega) {
			g.Expect(readRestore(pitr).Status.Phase).To(Equal(v1.PointInTimeRestorePending))
		}, time.Second, interval).Should(Succeed())
		expectClaimsUntouched(w)
	})

	It("wakes a held restore when the cluster is suspended", func() {
		w := createWorld(func(w *world) { w.cluster.Spec.Suspend = false })
		pitr := createRestore(w)
		expectHeld(pitr, v1.ReasonClusterNotSuspended)

		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &cluster)).To(Succeed())
			cluster.Spec.Suspend = true
			g.Expect(k8sClient.Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		expectAdmitted(pitr, w)
	})

	It("holds a restore whose clusterRef names no cluster", func() {
		w := createWorld()
		pitr := createRestore(w, func(p *v1.PointInTimeRestore) {
			p.Spec.ClusterRef.Name = "no-such-cluster"
		})

		Expect(expectHeld(pitr, v1.ReasonInvalidReference)).To(ContainSubstring("no-such-cluster"))
	})

	// The schema of CamundaCluster requires spec.storageRef, so a cluster
	// without one cannot exist. A cluster whose storage contract is gone is
	// the reachable shape of the same hold.
	It("holds a SecondaryStorageConfig that does not exist", func() {
		w := createWorld(func(w *world) { w.cluster.Spec.StorageRef = "no-such-storage" })
		pitr := createRestore(w)

		Expect(expectHeld(pitr, v1.ReasonInvalidReference)).To(ContainSubstring("no-such-storage"))
	})

	It("holds a cluster that names no backup storage", func() {
		w := createWorld(func(w *world) { w.cluster.Spec.BackupStorageRef = "" })
		pitr := createRestore(w)

		Expect(expectHeld(pitr, v1.ReasonInvalidReference)).To(ContainSubstring("backupStorageRef"))
	})

	It("holds a cluster whose secondary storage is Elasticsearch", func() {
		w := createWorld(func(w *world) {
			w.storage.Spec.Type = v1.SecondaryStorageTypeElasticsearch
			w.storage.Spec.RDBMS = nil
			w.storage.Spec.Elasticsearch = &v1.ElasticsearchStorage{
				Endpoint: "http://elasticsearch.es.svc:9200",
				CredentialsSecretRef: v1.CredentialsSecretRef{
					Name: "es", Namespace: w.namespace,
					UsernameKey: "username", PasswordKey: "password",
				},
			}
		})
		pitr := createRestore(w)

		message := expectHeld(pitr, v1.ReasonInvalidReference)
		Expect(message).To(ContainSubstring("point-in-time restore"))
		Expect(message).To(ContainSubstring("Elasticsearch"))
	})

	It("holds a DatabaseConfig that does not resolve", func() {
		w := createWorld(func(w *world) { w.storage.Spec.RDBMS.DatabaseConfigRef = "no-such-config" })
		pitr := createRestore(w)

		Expect(expectHeld(pitr, v1.ReasonInvalidReference)).To(ContainSubstring("no-such-config"))
	})

	It("holds a DatabaseServerConfig that does not resolve", func() {
		w := createWorld(func(w *world) { w.dbConfig.Spec.ServerRef = "no-such-server" })
		pitr := createRestore(w)

		Expect(expectHeld(pitr, v1.ReasonInvalidReference)).To(ContainSubstring("no-such-server"))
	})

	It("holds a server that declares no point-in-time recovery", func() {
		w := createWorld(func(w *world) { w.server.Spec.PITR = nil })
		pitr := createRestore(w)

		Expect(expectHeld(pitr, v1.ReasonPitrUnavailable)).To(ContainSubstring(w.server.Name))
	})

	It("holds a server whose point-in-time recovery is disabled", func() {
		w := createWorld(func(w *world) { w.server.Spec.PITR = &v1.PITRCapability{Enabled: false} })
		pitr := createRestore(w)

		Expect(expectHeld(pitr, v1.ReasonPitrUnavailable)).To(ContainSubstring(w.server.Name))
	})

	It("holds a timestamp that lies outside the retention period", func() {
		w := createWorld()
		pitr := createRestore(w, func(p *v1.PointInTimeRestore) {
			p.Spec.Timestamp = metav1.NewTime(suiteStart.Add(-8 * 24 * time.Hour))
		})

		Expect(expectHeld(pitr, v1.ReasonPitrUnavailable)).To(ContainSubstring("7 days"))
	})

	It("holds a timestamp that lies in the future", func() {
		w := createWorld()
		pitr := createRestore(w, func(p *v1.PointInTimeRestore) {
			p.Spec.Timestamp = metav1.NewTime(suiteStart.Add(time.Hour))
		})

		Expect(expectHeld(pitr, v1.ReasonPitrUnavailable)).To(ContainSubstring("future"))
	})

	It("holds a server that a second Database references", func() {
		w := createWorld()
		second := &v1.Database{
			ObjectMeta: metav1.ObjectMeta{Name: "other-" + w.database.Name, Namespace: newNamespace()},
			Spec: v1.DatabaseSpec{
				ServerRef:       w.server.Name,
				DatabaseName:    "other_database",
				TargetNamespace: w.namespace,
			},
		}
		Expect(k8sClient.Create(ctx, second)).To(Succeed())
		pitr := createRestore(w)

		message := expectHeld(pitr, v1.ReasonSharedServer)
		Expect(message).To(ContainSubstring(w.database.Name))
		Expect(message).To(ContainSubstring(second.Name))
		expectClaimsUntouched(w)
	})

	It("admits a suspended cluster on a dedicated server", func() {
		w := createWorld()
		pitr := createRestore(w)

		expectAdmitted(pitr, w)
	})
})

var _ = Describe("PointInTimeRestore database-state check", func() {
	// The refusal is the whole point of the phase: the database of the
	// cluster still holds state that the requested point does not, so the
	// broker volumes must survive untouched.
	It("refuses a database that is ahead of the requested point, and touches nothing", func() {
		w := createWorld()
		exporter.set(w.dbConfig.Spec.DatabaseName, answer{positions: positionsBehind(-2 * time.Minute)})
		pitr := createRestore(w)

		message := expectHeld(pitr, v1.ReasonDatabaseNotRestored)
		Expect(message).To(ContainSubstring("ahead"))
		expectClaimsUntouched(w)
		Expect(jobsOf(pitr)).To(BeEmpty())
		Expect(readRestore(pitr).Status.ObservedPositions).To(HaveLen(partitionCount))
	})

	It("refuses a database that reports no position for a partition", func() {
		w := createWorld()
		exporter.set(w.dbConfig.Spec.DatabaseName, answer{
			positions: positionsBehind(time.Hour)[:1],
		})
		pitr := createRestore(w)

		Expect(expectHeld(pitr, v1.ReasonDatabaseNotRestored)).To(ContainSubstring("partition 2"))
		expectClaimsUntouched(w)
	})

	It("continues on its own once the database is restored to the requested point", func() {
		w := createWorld()
		exporter.set(w.dbConfig.Spec.DatabaseName, answer{positions: positionsBehind(-2 * time.Minute)})
		pitr := createRestore(w)
		expectHeld(pitr, v1.ReasonDatabaseNotRestored)

		exporter.set(w.dbConfig.Spec.DatabaseName, answer{positions: positionsBehind(time.Hour)})

		Eventually(func(g Gomega) {
			current := readRestore(pitr)
			g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreRestoringPrimaryStorage))
			g.Expect(current.Status.ObservedPositions).To(HaveLen(partitionCount))
			g.Expect(current.Status.Brokers).To(Equal(int32(brokerCount)))
		}, timeout, interval).Should(Succeed())
	})

	It("holds a database that the operator cannot read, then fails past the grace", func() {
		w := createWorld()
		exporter.set(w.dbConfig.Spec.DatabaseName, answer{
			err: errors.New("password authentication failed for user \"camunda\""),
		})
		pitr := createRestore(w)

		Expect(expectStartedHold(pitr, v1.ReasonConnectionFailed)).To(ContainSubstring("password"))
		expectClaimsUntouched(w)

		Eventually(func(g Gomega) {
			current := readRestore(pitr)
			g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreFailed))
			g.Expect(current.Status.FailureMessage).To(ContainSubstring("did not recover"))
		}, timeout, interval).Should(Succeed())
		expectClaimsUntouched(w)
	})

	It("holds a restore whose application credentials are missing", func() {
		w := createWorld(func(w *world) {
			w.dbConfig.Spec.CredentialsSecretRef.Name = "no-such-secret"
		})
		pitr := createRestore(w)

		Expect(expectStartedHold(pitr, v1.ReasonMissingSecret)).To(ContainSubstring("no-such-secret"))
		expectClaimsUntouched(w)
	})

	It("refuses a database whose Camunda schema does not exist", func() {
		w := createWorld()
		exporter.set(w.dbConfig.Spec.DatabaseName, answer{
			err: fmt.Errorf("%w: relation \"exporter_position\" does not exist", errNoExporterTable),
		})
		pitr := createRestore(w)

		Expect(expectHeld(pitr, v1.ReasonDatabaseNotRestored)).To(ContainSubstring("exporter position"))
		expectClaimsUntouched(w)
	})
})

// markJob writes onto the restore Job of one broker the bookkeeping that a
// finished Job carries. No Job controller runs in envtest, so the suite plays
// it.
func markJob(pitr *v1.PointInTimeRestore, ordinal int32, kind batchv1.JobConditionType) {
	GinkgoHelper()
	key := types.NamespacedName{
		Namespace: pitr.Namespace,
		Name:      restore.JobName(labels.PointInTimeRestore(pitr.Name), ordinal),
	}
	var job batchv1.Job
	Eventually(func(g Gomega) {
		g.Expect(k8sClient.Get(ctx, key, &job)).To(Succeed())
	}, timeout, interval).Should(Succeed())

	// The API server demands the full bookkeeping of a finished Job: the
	// precursor condition, the start time, and, for a completed one, the
	// completion time.
	now := metav1.Now()
	precursor := batchv1.JobSuccessCriteriaMet
	if kind == batchv1.JobFailed {
		precursor = batchv1.JobFailureTarget
	}
	job.Status.StartTime = &now
	job.Status.Conditions = append(
		job.Status.Conditions,
		batchv1.JobCondition{
			Type: precursor, Status: corev1.ConditionTrue,
			Reason: "Test", Message: "marked by the suite",
		},
		batchv1.JobCondition{
			Type: kind, Status: corev1.ConditionTrue,
			Reason: "Test", Message: "marked by the suite",
		},
	)
	if kind == batchv1.JobComplete {
		job.Status.Succeeded = 1
		job.Status.CompletionTime = &now
	}
	Expect(k8sClient.Status().Update(ctx, &job)).To(Succeed())
}

// expectRecreatedClaims waits until every broker volume of the world is a new,
// empty volume and returns the volumes as they are now.
func expectRecreatedClaims(w *world, pitr *v1.PointInTimeRestore) []corev1.PersistentVolumeClaim {
	GinkgoHelper()
	current := make([]corev1.PersistentVolumeClaim, brokerCount)
	Eventually(func(g Gomega) {
		for ordinal, claim := range w.claims {
			var live corev1.PersistentVolumeClaim
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(claim), &live)).To(Succeed())
			g.Expect(live.UID).NotTo(Equal(claim.UID), "the broker volume still holds its old data")
			current[ordinal] = live
		}
		g.Expect(readRestore(pitr).Status.RecreatedClaims).To(HaveLen(brokerCount))
	}, timeout, interval).Should(Succeed())

	return current
}

// expectJobs waits for one restore Job per broker and returns them in broker
// order.
func expectJobs(pitr *v1.PointInTimeRestore) []batchv1.Job {
	GinkgoHelper()
	jobs := make([]batchv1.Job, brokerCount)
	Eventually(func(g Gomega) {
		for ordinal := range int32(brokerCount) {
			var job batchv1.Job
			key := types.NamespacedName{
				Namespace: pitr.Namespace,
				Name:      restore.JobName(labels.PointInTimeRestore(pitr.Name), ordinal),
			}
			g.Expect(k8sClient.Get(ctx, key, &job)).To(Succeed())
			jobs[ordinal] = job
		}
	}, timeout, interval).Should(Succeed())

	return jobs
}

var _ = Describe("PointInTimeRestore primary storage", func() {
	It("recreates the broker volumes, runs one Job per broker, and completes", func() {
		w := createWorld()
		pitr := createRestore(w)

		By("recording the broker count of the live StatefulSet before it deletes anything")
		Eventually(func(g Gomega) {
			g.Expect(readRestore(pitr).Status.Brokers).To(Equal(int32(brokerCount)))
		}, timeout, interval).Should(Succeed())

		By("deleting and creating every broker volume, at the size of the claim template")
		claims := expectRecreatedClaims(w, pitr)
		for _, claim := range claims {
			Expect(claim.Spec.Resources.Requests.Storage().String()).To(Equal("10Gi"))
			Expect(claim.Spec.AccessModes).To(Equal(w.brokers.Spec.VolumeClaimTemplates[0].Spec.AccessModes))
			Expect(claim.Labels).To(HaveKeyWithValue("app", w.brokers.Name))
			Expect(claim.OwnerReferences).To(
				BeEmpty(), "the StatefulSet owns the broker volumes, never the restore",
			)
		}
		Expect(readRestore(pitr).Status.RecreatedClaims).To(ConsistOf(claimName(w, 0), claimName(w, 1)))

		By("running the restore application once per broker, at the requested point")
		jobs := expectJobs(pitr)
		for ordinal, job := range jobs {
			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := job.Spec.Template.Spec.Containers[0]
			Expect(container.Command).To(Equal([]string{restore.RestoreEntrypoint}))
			Expect(container.Args).To(Equal([]string{
				"--to=" + pitr.Spec.Timestamp.UTC().Format(time.RFC3339),
			}))
			Expect(container.Env).To(ContainElement(camundaconfig.Var(
				camundaconfig.KeyClusterNodeID, strconv.Itoa(ordinal),
			)))
			Expect(job.Labels).To(HaveKeyWithValue(labels.PointInTimeRestoreKey, pitr.Name))
			Expect(job.Labels).To(HaveKeyWithValue(labels.ComponentKey, restore.ComponentRestore))
			Expect(job.Labels).To(HaveKeyWithValue(labels.ClusterKey, w.cluster.Name))
			Expect(job.OwnerReferences).To(HaveLen(1), "deleting the restore must remove its Jobs")
			Expect(job.OwnerReferences[0].Name).To(Equal(pitr.Name))
			Expect(*job.OwnerReferences[0].Controller).To(BeTrue())
		}
		Expect(readRestore(pitr).Status.PrimaryJobNames).To(HaveLen(brokerCount))

		By("keeping the volumes it created while the Jobs run")
		Consistently(func(g Gomega) {
			for ordinal, claim := range claims {
				var live corev1.PersistentVolumeClaim
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&claims[ordinal]), &live)).To(Succeed())
				g.Expect(live.UID).To(Equal(claim.UID))
			}
		}, time.Second, interval).Should(Succeed())

		By("completing once every broker restored")
		for ordinal := range int32(brokerCount) {
			markJob(pitr, ordinal, batchv1.JobComplete)
		}
		Eventually(func(g Gomega) {
			current := readRestore(pitr)
			g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreCompleted))
			g.Expect(current.Status.CompletionTime).NotTo(BeNil())
			condition := ready(current)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(v1.ReasonCompleted))
		}, timeout, interval).Should(Succeed())
	})

	It("fails when a broker cannot restore, and names the broker", func() {
		w := createWorld()
		pitr := createRestore(w)
		expectJobs(pitr)

		markJob(pitr, 1, batchv1.JobFailed)

		Eventually(func(g Gomega) {
			current := readRestore(pitr)
			g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreFailed))
			g.Expect(current.Status.FailureMessage).To(ContainSubstring("broker 1"))
		}, timeout, interval).Should(Succeed())
	})

	It("holds a restore whose cluster was unsuspended under it", func() {
		w := createWorld()
		pitr := createRestore(w)
		expectJobs(pitr)

		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &cluster)).To(Succeed())
			cluster.Spec.Suspend = false
			g.Expect(k8sClient.Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			condition := ready(readRestore(pitr))
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Reason).To(Equal(v1.ReasonClusterNotSuspended))
		}, timeout, interval).Should(Succeed())
	})

	It("holds a restore whose pod cannot start, then fails past the grace", func() {
		w := createWorld()
		pitr := createRestore(w)
		expectJobs(pitr)

		// No Job controller runs in envtest, so the suite creates the pod that
		// a Job creates, in the state that a missing Secret leaves it in.
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "restore-pod-" + strings.ToLower(utilrand.String(6)),
				Namespace: w.namespace,
				Labels:    restore.JobSelector(labels.PointInTimeRestore(pitr.Name)),
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers: []corev1.Container{{
					Name: restore.ComponentRestore, Image: "camunda/camunda:8.9.9",
				}},
			},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		pod.Status.Phase = corev1.PodPending
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
			Name: restore.ComponentRestore,
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
				Reason:  "CreateContainerConfigError",
				Message: "secret \"camunda-credentials\" not found",
			}},
		}}
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())

		Eventually(func(g Gomega) {
			condition := ready(readRestore(pitr))
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Reason).To(Equal(v1.ReasonMissingSecret))
			g.Expect(condition.Message).To(ContainSubstring(pod.Name))
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			g.Expect(readRestore(pitr).Status.Phase).To(Equal(v1.PointInTimeRestoreFailed))
		}, timeout, interval).Should(Succeed())
	})

	It("never recreates a broker volume once the Jobs exist", func() {
		w := createWorld()
		pitr := createRestore(w)
		claims := expectRecreatedClaims(w, pitr)
		expectJobs(pitr)

		Consistently(func(g Gomega) {
			for ordinal := range claims {
				var live corev1.PersistentVolumeClaim
				g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(&claims[ordinal]), &live)).To(Succeed())
				g.Expect(live.UID).To(Equal(claims[ordinal].UID))
				g.Expect(live.DeletionTimestamp).To(BeNil())
			}
		}, 2*time.Second, interval).Should(Succeed())
	})
})
