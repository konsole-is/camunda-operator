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
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
	"github.com/konsole-is/camunda-operator/pkg/clusterclaim"
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
	// brokerEnv is added to the broker container of the world, so a spec can
	// shape the environment that the restore reads its facts from.
	brokerEnv []corev1.EnvVar
	// brokerEnvFrom is added to the envFrom of the broker container, the way
	// spec.zeebe.extraEnvFrom reaches it.
	brokerEnvFrom []corev1.EnvFromSource
	claims        []*corev1.PersistentVolumeClaim
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

	return createWorldIn(newNamespace(), mutate...)
}

// createWorldIn is createWorld in a namespace that the spec already made, for
// a spec that has to create a resource of the cluster before the cluster.
func createWorldIn(namespace string, mutate ...func(*world)) *world {
	GinkgoHelper()
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
			PITR: &v1.PITRCapability{Enabled: true, RetentionPeriodDays: new(int32(7))},
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
			Replicas:    new(int32(0)),
			Selector:    &metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": name}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  components.ContainerCamunda,
						Image: "camunda/camunda:8.9.9",
						Env: append([]corev1.EnvVar{
							camundaconfig.Var(camundaconfig.KeyClusterSize, "2"),
							camundaconfig.Var(camundaconfig.KeyClusterPartitionCount, "3"),
						}, w.brokerEnv...),
						EnvFrom: w.brokerEnvFrom,
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
// duration behind the point that a restore of a spec asks for.
func positionsBehind(behind time.Duration) []v1.PartitionPosition {
	return positionsAt(restorePoint().Add(-behind))
}

// positionsAt returns one exporter position per partition, each at when.
func positionsAt(when time.Time) []v1.PartitionPosition {
	positions := make([]v1.PartitionPosition, 0, partitionCount)
	for partition := 1; partition <= partitionCount; partition++ {
		positions = append(positions, v1.PartitionPosition{
			PartitionID: int32(partition),
			LastUpdated: metav1.NewTime(when),
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
		g.Expect(current.Status.TargetClusterUID).To(Equal(w.cluster.UID))
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

// collectDeletedJobs plays the garbage collector of a real cluster: it clears
// the foreground finalizer that the API server puts on a Job which was deleted
// with foreground propagation, as the collector does once the pods of that Job
// are gone. Nothing does that in envtest, so such a Job would stay in place for
// ever.
func collectDeletedJobs(namespace string) {
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

			var jobs batchv1.JobList
			if err := k8sClient.List(ctx, &jobs, client.InNamespace(namespace)); err != nil {
				continue
			}
			for i := range jobs.Items {
				job := &jobs.Items[i]
				if job.DeletionTimestamp == nil || len(job.Finalizers) == 0 {
					continue
				}
				job.Finalizers = nil
				_ = k8sClient.Update(ctx, job)
			}
		}
	}()
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

	// The database records the exporter position with the wall clock of the
	// broker. A broker west of UTC would make a database that is ahead of the
	// requested point read as behind it, and the volumes would go.
	It("holds a cluster whose brokers do not run in UTC", func() {
		w := createWorld(func(w *world) {
			w.brokerEnv = []corev1.EnvVar{{Name: "TZ", Value: "America/New_York"}}
		})
		pitr := createRestore(w)

		message := expectHeld(pitr, v1.ReasonPitrUnavailable)
		Expect(message).To(ContainSubstring("America/New_York"))
		Expect(message).To(ContainSubstring("UTC"))
		expectClaimsUntouched(w)
	})

	It("holds a cluster whose brokers take their zone from a ConfigMap", func() {
		namespace := newNamespace()
		zones := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "broker-env", Namespace: namespace},
			Data:       map[string]string{"TZ": "Asia/Tokyo", "OTHER": "x"},
		}
		Expect(k8sClient.Create(ctx, zones)).To(Succeed())

		w := createWorldIn(namespace, func(w *world) {
			w.brokerEnvFrom = []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: zones.Name},
				},
			}}
		})
		pitr := createRestore(w)

		Expect(expectHeld(pitr, v1.ReasonPitrUnavailable)).To(ContainSubstring("Asia/Tokyo"))
		expectClaimsUntouched(w)
	})

	It("holds a cluster whose brokers take their zone from a prefixed Secret", func() {
		namespace := newNamespace()
		options := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "broker-options", Namespace: namespace},
			Data:       map[string][]byte{"OPTS": []byte("-Duser.timezone=Europe/Berlin")},
		}
		Expect(k8sClient.Create(ctx, options)).To(Succeed())

		w := createWorldIn(namespace, func(w *world) {
			w.brokerEnvFrom = []corev1.EnvFromSource{{
				Prefix: "JAVA_",
				SecretRef: &corev1.SecretEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: options.Name},
				},
			}}
		})
		pitr := createRestore(w)

		Expect(expectHeld(pitr, v1.ReasonPitrUnavailable)).To(ContainSubstring("Europe/Berlin"))
		expectClaimsUntouched(w)
	})

	// The kubelet applies envFrom first and lets env override it, so a cluster
	// that corrects the zone of a shared ConfigMap in extraEnv runs in UTC.
	It("admits a cluster whose extraEnv overrides the zone of a source with UTC", func() {
		namespace := newNamespace()
		zones := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "shared-env", Namespace: namespace},
			Data:       map[string]string{"TZ": "America/New_York"},
		}
		Expect(k8sClient.Create(ctx, zones)).To(Succeed())

		w := createWorldIn(namespace, func(w *world) {
			w.brokerEnvFrom = []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: zones.Name},
				},
			}}
			w.brokerEnv = []corev1.EnvVar{{Name: "TZ", Value: "UTC"}}
		})
		pitr := createRestore(w)

		expectAdmitted(pitr, w)
	})

	It("holds a cluster whose extraEnv overrides the zone of a source with another zone", func() {
		namespace := newNamespace()
		zones := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "shared-env", Namespace: namespace},
			Data:       map[string]string{"TZ": "UTC"},
		}
		Expect(k8sClient.Create(ctx, zones)).To(Succeed())

		w := createWorldIn(namespace, func(w *world) {
			w.brokerEnvFrom = []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: zones.Name},
				},
			}}
			w.brokerEnv = []corev1.EnvVar{{Name: "TZ", Value: "Asia/Tokyo"}}
		})
		pitr := createRestore(w)

		Expect(expectHeld(pitr, v1.ReasonPitrUnavailable)).To(ContainSubstring("Asia/Tokyo"))
		expectClaimsUntouched(w)
	})

	It("holds a cluster whose broker environment the operator cannot read", func() {
		w := createWorld(func(w *world) {
			w.brokerEnvFrom = []corev1.EnvFromSource{{
				ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "no-such-configmap"},
				},
			}}
		})
		pitr := createRestore(w)

		Expect(expectHeld(pitr, v1.ReasonPitrUnavailable)).To(ContainSubstring("no-such-configmap"))
		expectClaimsUntouched(w)
	})

	It("admits a cluster whose broker environment carries no zone", func() {
		namespace := newNamespace()
		other := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: "broker-env", Namespace: namespace},
			Data:       map[string]string{"SOMETHING": "else"},
		}
		Expect(k8sClient.Create(ctx, other)).To(Succeed())

		w := createWorldIn(namespace, func(w *world) {
			w.brokerEnvFrom = []corev1.EnvFromSource{
				{ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: other.Name},
				}},
				// A source that the cluster marks optional and that does not
				// exist carries no variable at all, so it hides no zone.
				{ConfigMapRef: &corev1.ConfigMapEnvSource{
					LocalObjectReference: corev1.LocalObjectReference{Name: "absent"},
					Optional:             new(true),
				}},
			}
		})
		pitr := createRestore(w)

		expectAdmitted(pitr, w)
	})

	// The Database resource is what proves that the server holds one database.
	// Without it the operator can prove nothing, and point-in-time recovery
	// rolls back the whole server, so it does not start.
	It("holds a server that no Database references", func() {
		w := createWorld(func(w *world) { w.database.Spec.ServerRef = "another-server" })
		pitr := createRestore(w)

		message := expectHeld(pitr, v1.ReasonInvalidReference)
		Expect(message).To(ContainSubstring(w.server.Name))
		Expect(message).To(ContainSubstring("Database"))
		expectClaimsUntouched(w)
	})

	// A restore that waits for a rule keeps the identity of the cluster it
	// read. A cluster that is deleted and created again under one name is
	// another cluster, with other primary storage.
	It("fails when the cluster it waits for is replaced under its name", func() {
		w := createWorld(func(w *world) { w.cluster.Spec.Suspend = false })
		pitr := createRestore(w)
		expectHeld(pitr, v1.ReasonClusterNotSuspended)

		Expect(k8sClient.Delete(ctx, w.cluster)).To(Succeed())
		Eventually(func(g Gomega) {
			var gone v1.CamundaCluster
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &gone)
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		replacement := &v1.CamundaCluster{
			ObjectMeta: metav1.ObjectMeta{Name: w.cluster.Name, Namespace: w.namespace},
			Spec:       *w.cluster.Spec.DeepCopy(),
		}
		replacement.Spec.Suspend = true
		Expect(k8sClient.Create(ctx, replacement)).To(Succeed())

		Eventually(func(g Gomega) {
			current := readRestore(pitr)
			g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreFailed))
			g.Expect(current.Status.FailureMessage).To(ContainSubstring("replaced"))
		}, timeout, interval).Should(Succeed())
		expectClaimsUntouched(w)
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

// holdCluster creates a relational backup of the cluster and gives it the
// claim on that cluster, as the backup controller does when its admission
// passes.
func holdCluster(w *world) *v1.LogicalBackupRDBMS {
	GinkgoHelper()
	backup := &v1.LogicalBackupRDBMS{
		ObjectMeta: metav1.ObjectMeta{
			Name: "lbrdbms-" + strings.ToLower(utilrand.String(6)), Namespace: w.namespace,
		},
		Spec: v1.LogicalBackupRDBMSSpec{ClusterRef: v1.ClusterRef{Name: w.cluster.Name}},
	}
	Expect(k8sClient.Create(ctx, backup)).To(Succeed())

	holder, err := clusterclaim.Claim(
		ctx, k8sClient, k8sClient, w.namespace, w.cluster.Name,
		clusterclaim.Claimant{Kind: backup.GetKind(), Name: backup.Name, UID: backup.UID},
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(holder).To(BeEmpty(), "the backup must hold the claim before the restore looks")

	return backup
}

// claimHolder returns the identity that the claim Lease of the cluster
// records, or "" when no Lease exists.
func claimHolder(w *world) string {
	GinkgoHelper()
	var lease coordinationv1.Lease
	key := types.NamespacedName{
		Namespace: w.namespace, Name: clusterclaim.ClaimLeaseName(w.cluster.Name),
	}
	err := k8sClient.Get(ctx, key, &lease)
	if apierrors.IsNotFound(err) {
		return ""
	}
	Expect(err).NotTo(HaveOccurred())

	annotations := lease.GetAnnotations()

	return annotations[clusterclaim.ClaimHolderKindAnnotation] + "/" +
		annotations[clusterclaim.ClaimHolderNameAnnotation]
}

var _ = Describe("PointInTimeRestore cluster claim", func() {
	// One backup or restore of a cluster runs at a time. A restore that
	// started while a backup runs would erase the volumes under it.
	It("holds a restore whose cluster another operation holds, and touches nothing", func() {
		w := createWorld()
		backup := holdCluster(w)
		pitr := createRestore(w)

		message := expectHeld(pitr, v1.ReasonClusterClaimed)
		Expect(message).To(ContainSubstring(backup.Name))
		expectClaimsUntouched(w)
		Expect(jobsOf(pitr)).To(BeEmpty())
		Expect(claimHolder(w)).To(Equal("LogicalBackupRDBMS/" + backup.Name))
	})

	// Nothing bounds the hold, and no spec change ends it. The restore takes
	// the claim over as soon as the holder reaches a terminal phase.
	It("starts on its own once the holder finishes", func() {
		w := createWorld()
		backup := holdCluster(w)
		pitr := createRestore(w)
		expectHeld(pitr, v1.ReasonClusterClaimed)

		Eventually(func(g Gomega) {
			var current v1.LogicalBackupRDBMS
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &current)).To(Succeed())
			current.Status.Phase = v1.LogicalBackupCompleted
			g.Expect(k8sClient.Status().Update(ctx, &current)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		expectAdmitted(pitr, w)
		Eventually(func() string {
			return claimHolder(w)
		}, timeout, interval).Should(Equal("PointInTimeRestore/" + pitr.Name))
	})

	// Foreground propagation keeps a Job in place until its last pod is gone,
	// and a pod that mounts a broker volume is what holds that volume. The
	// claim is what tells the next operation that the cluster is free, so it
	// waits for the pods and not for the delete call.
	It("keeps the claim while the Jobs of the completed restore still terminate", func() {
		w := createWorld()
		pitr := createRestore(w)
		expectJobs(pitr)

		for ordinal := range int32(brokerCount) {
			markJob(pitr, ordinal, batchv1.JobComplete)
		}

		By("reaching the terminal phase with every Job asked to go")
		Eventually(func(g Gomega) {
			g.Expect(readRestore(pitr).Status.Phase).To(Equal(v1.PointInTimeRestoreCompleted))
			jobs := jobsOf(pitr)
			g.Expect(jobs).To(HaveLen(brokerCount))
			for _, job := range jobs {
				g.Expect(job.DeletionTimestamp).NotTo(BeNil(), "Job %q", job.Name)
			}
		}, timeout, interval).Should(Succeed())

		By("holding the claim while the Jobs are still there")
		Consistently(func() string {
			return claimHolder(w)
		}, time.Second, interval).Should(Equal("PointInTimeRestore/" + pitr.Name))

		By("giving the claim back once the collector removed them")
		collectDeletedJobs(w.namespace)
		Eventually(func() string {
			return claimHolder(w)
		}, timeout, interval).Should(BeEmpty())
	})

	It("gives the claim back when it completes", func() {
		w := createWorld()
		collectDeletedJobs(w.namespace)
		pitr := createRestore(w)
		expectJobs(pitr)
		Expect(claimHolder(w)).To(Equal("PointInTimeRestore/" + pitr.Name))

		for ordinal := range int32(brokerCount) {
			markJob(pitr, ordinal, batchv1.JobComplete)
		}

		Eventually(func(g Gomega) {
			g.Expect(readRestore(pitr).Status.Phase).To(Equal(v1.PointInTimeRestoreCompleted))
			g.Expect(claimHolder(w)).To(BeEmpty())
		}, timeout, interval).Should(Succeed())
	})

	It("gives the claim back when it fails", func() {
		w := createWorld()
		pitr := createRestore(w)
		expectJobs(pitr)

		markJob(pitr, 0, batchv1.JobFailed)

		Eventually(func(g Gomega) {
			g.Expect(readRestore(pitr).Status.Phase).To(Equal(v1.PointInTimeRestoreFailed))
			g.Expect(claimHolder(w)).To(BeEmpty())
		}, timeout, interval).Should(Succeed())
	})
})

var _ = Describe("PointInTimeRestore storage identity", func() {
	// The restore validated one database. A cluster that is repointed at
	// another one after that is a different restore, and the volumes of this
	// one must not go on the strength of a check that ran elsewhere.
	It("fails when the cluster is repointed at another database", func() {
		w := createWorld()
		exporter.set(w.dbConfig.Spec.DatabaseName, answer{positions: positionsBehind(-2 * time.Minute)})
		pitr := createRestore(w)
		expectHeld(pitr, v1.ReasonDatabaseNotRestored)

		other := &v1.DatabaseConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "other-config", Namespace: w.namespace},
			Spec: v1.DatabaseConfigSpec{
				ServerRef:            w.server.Name,
				DatabaseName:         "other_database",
				CredentialsSecretRef: w.dbConfig.Spec.CredentialsSecretRef,
			},
		}
		Expect(k8sClient.Create(ctx, other)).To(Succeed())
		otherStorage := &v1.SecondaryStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "other-storage", Namespace: w.namespace},
			Spec: v1.SecondaryStorageConfigSpec{
				Type:  v1.SecondaryStorageTypeRDBMS,
				RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: other.Name},
			},
		}
		Expect(k8sClient.Create(ctx, otherStorage)).To(Succeed())
		// The database of the new chain would pass the check that the old one
		// holds on.
		exporter.set(other.Spec.DatabaseName, answer{positions: positionsBehind(time.Hour)})

		Eventually(func(g Gomega) {
			var cluster v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.cluster), &cluster)).To(Succeed())
			cluster.Spec.StorageRef = otherStorage.Name
			g.Expect(k8sClient.Update(ctx, &cluster)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		Eventually(func(g Gomega) {
			current := readRestore(pitr)
			g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreFailed))
			g.Expect(current.Status.FailureMessage).To(ContainSubstring("other-storage"))
		}, timeout, interval).Should(Succeed())
		expectClaimsUntouched(w)
		Expect(jobsOf(pitr)).To(BeEmpty())
	})

	It("pins what it validated", func() {
		w := createWorld()
		pitr := createRestore(w)

		Eventually(func(g Gomega) {
			pinned := readRestore(pitr).Status.Storage
			g.Expect(pinned).NotTo(BeNil())
			g.Expect(pinned.SecondaryStorageConfig).To(Equal(w.storage.Name))
			g.Expect(pinned.DatabaseConfig).To(Equal(w.dbConfig.Name))
			g.Expect(pinned.DatabaseServerConfig).To(Equal(w.server.Name))
			g.Expect(pinned.DatabaseName).To(Equal(w.dbConfig.Spec.DatabaseName))
			g.Expect(pinned.Endpoint).To(Equal("postgres.databases.svc:5432"))
		}, timeout, interval).Should(Succeed())
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
	markJobNamed(
		pitr.Namespace, restore.JobName(labels.PointInTimeRestore(pitr.Name), ordinal), kind,
	)
}

// markJobNamed is markJob for any Job name, for example a Job that another
// restore left behind.
func markJobNamed(namespace, name string, kind batchv1.JobConditionType) {
	GinkgoHelper()
	key := types.NamespacedName{Namespace: namespace, Name: name}
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
			Expect(claim.ManagedFields).To(
				ContainElement(HaveField("Manager", string(restore.FieldManagerPointInTimeRestore))),
				"the fields of a recreated volume carry the field manager of this kind",
			)
		}
		Expect(readRestore(pitr).Status.RecreatedClaims).To(ConsistOf(claimName(w, 0), claimName(w, 1)))

		By("running the restore application once per broker, at the requested point")
		jobs := expectJobs(pitr)
		for ordinal, job := range jobs {
			Expect(job.Spec.Template.Spec.Containers).To(HaveLen(1))
			container := job.Spec.Template.Spec.Containers[0]
			Expect(container.Command).To(Equal([]string{restore.RestoreEntrypoint}))
			Expect(container.Env).To(ContainElement(camundaconfig.Var(
				camundaconfig.KeyClusterNodeID, strconv.Itoa(ordinal),
			)))
			Expect(job.Labels).To(HaveKeyWithValue(labels.PointInTimeRestoreKey, pitr.Name))
			Expect(job.Labels).To(HaveKeyWithValue(labels.ComponentKey, restore.ComponentRestore))
			Expect(job.Labels).To(HaveKeyWithValue(labels.ClusterKey, w.cluster.Name))
			Expect(job.OwnerReferences).To(HaveLen(1), "deleting the restore must remove its Jobs")
			Expect(job.OwnerReferences[0].Name).To(Equal(pitr.Name))
			Expect(*job.OwnerReferences[0].Controller).To(BeTrue())
			Expect(job.ManagedFields).To(
				ContainElement(HaveField("Manager", string(restore.FieldManagerPointInTimeRestore))),
				"the fields of a restore Job carry the field manager of this kind",
			)
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
			g.Expect(current.Status.TerminalReason).To(Equal(v1.ReasonCompleted))
			condition := ready(current)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(condition.Reason).To(Equal(v1.ReasonCompleted))
		}, timeout, interval).Should(Succeed())
	})

	// A Job that completed keeps its pod, and a pod that mounts a broker
	// volume holds that volume under the pvc-protection finalizer. The next
	// operation on the cluster then waits on that volume without end.
	It("removes its restore Jobs when it completes", func() {
		w := createWorld()
		pitr := createRestore(w)
		expectJobs(pitr)

		for ordinal := range int32(brokerCount) {
			markJob(pitr, ordinal, batchv1.JobComplete)
		}

		By("asking for every recorded Job with foreground propagation")
		Eventually(func(g Gomega) {
			g.Expect(readRestore(pitr).Status.Phase).To(Equal(v1.PointInTimeRestoreCompleted))
			jobs := jobsOf(pitr)
			g.Expect(jobs).To(HaveLen(brokerCount))
			for _, job := range jobs {
				g.Expect(job.DeletionTimestamp).NotTo(BeNil(), "Job %q", job.Name)
				g.Expect(job.Finalizers).To(
					ContainElement(metav1.FinalizerDeleteDependents),
					"Job %q goes without foreground propagation, so its pods outlive it", job.Name,
				)
			}
		}, timeout, interval).Should(Succeed())

		By("leaving no Job behind once the pods of the Jobs are gone")
		collectDeletedJobs(pitr.Namespace)
		Eventually(func(g Gomega) {
			g.Expect(jobsOf(pitr)).To(BeEmpty())
		}, timeout, interval).Should(Succeed())
	})

	// The logs of a failed Job are the diagnosis of the failure, and only the
	// pod keeps them readable. A failed restore therefore holds the broker
	// volumes until somebody deletes the restore.
	It("keeps its restore Jobs when it fails", func() {
		w := createWorld()
		pitr := createRestore(w)
		expectJobs(pitr)

		markJob(pitr, 1, batchv1.JobFailed)

		Eventually(func(g Gomega) {
			g.Expect(readRestore(pitr).Status.Phase).To(Equal(v1.PointInTimeRestoreFailed))
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			jobs := jobsOf(pitr)
			g.Expect(jobs).To(HaveLen(brokerCount))
			for _, job := range jobs {
				g.Expect(job.DeletionTimestamp).To(BeNil(), "Job %q", job.Name)
			}
		}, time.Second, interval).Should(Succeed())
	})

	// The argument is the contract with the restore application, so the spec
	// names the exact string a broker receives instead of building it the way
	// the controller does.
	It("passes the requested point to the restore application as RFC 3339", func() {
		point := time.Date(2026, 7, 30, 14, 30, 0, 0, time.UTC)
		w := createWorld(func(w *world) {
			// The point lies further back than the retention of the other
			// worlds, and the rule that guards it has its own spec.
			w.server.Spec.PITR.RetentionPeriodDays = new(int32(3650))
		})
		exporter.set(w.dbConfig.Spec.DatabaseName, answer{positions: positionsAt(point.Add(-time.Hour))})
		pitr := createRestore(w, func(p *v1.PointInTimeRestore) {
			p.Spec.Timestamp = metav1.NewTime(point)
		})

		for _, job := range expectJobs(pitr) {
			Expect(job.Spec.Template.Spec.Containers[0].Args).To(
				Equal([]string{"--to=2026-07-30T14:30:00Z"}),
			)
		}
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
			// The terminal reason is recorded, so a write conflict on the
			// terminal flush cannot replace it with a weaker one.
			g.Expect(current.Status.TerminalReason).To(Equal(v1.ReasonFailed))
			condition := ready(current)
			g.Expect(condition).NotTo(BeNil())
			g.Expect(condition.Reason).To(Equal(current.Status.TerminalReason))
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

	It("refuses a Job that another restore left behind under its name", func() {
		w := createWorld()
		name := "pitr-" + strings.ToLower(utilrand.String(6))
		owner := labels.PointInTimeRestore(name)

		// The restore of a deleted resource of the same name. The garbage
		// collector has not removed it yet, and it carries the labels and the
		// name that this restore derives for its own broker 0.
		foreign := &batchv1.Job{
			ObjectMeta: metav1.ObjectMeta{
				Name:      restore.JobName(owner, 0),
				Namespace: w.namespace,
				Labels:    restore.JobLabels(owner, w.cluster.Name),
			},
			Spec: batchv1.JobSpec{
				Template: corev1.PodTemplateSpec{
					Spec: corev1.PodSpec{
						RestartPolicy: corev1.RestartPolicyNever,
						Containers: []corev1.Container{{
							Name: restore.ComponentRestore, Image: "camunda/camunda:8.9.9",
						}},
					},
				},
			},
		}
		Expect(k8sClient.Create(ctx, foreign)).To(Succeed())
		markJobNamed(w.namespace, foreign.Name, batchv1.JobComplete)

		pitr := createRestore(w, func(p *v1.PointInTimeRestore) { p.Name = name })

		Eventually(func(g Gomega) {
			current := readRestore(pitr)
			g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreFailed))
			g.Expect(current.Status.FailureMessage).To(ContainSubstring(foreign.Name))
		}, timeout, interval).Should(Succeed())

		// The restore never writes to a Job it does not own, so the Job of the
		// other restore keeps its own fields and its own owner.
		var untouched batchv1.Job
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(foreign), &untouched)).To(Succeed())
		Expect(untouched.UID).To(Equal(foreign.UID))
		Expect(untouched.OwnerReferences).To(BeEmpty())
		Expect(untouched.ManagedFields).NotTo(
			ContainElement(HaveField("Manager", string(restore.FieldManagerPointInTimeRestore))),
		)
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

	// The Jobs of a restore are the ones it recorded. A cluster that changes
	// its broker count while the restore runs must not add a Job on a volume
	// that this restore never emptied, and must not hold the restore open for
	// a broker it never started.
	It("tracks the Jobs it recorded when the cluster size changes under it", func() {
		w := createWorld()
		pitr := createRestore(w)
		expectJobs(pitr)

		Eventually(func(g Gomega) {
			var brokers appsv1.StatefulSet
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.brokers), &brokers)).To(Succeed())
			brokers.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
				camundaconfig.Var(camundaconfig.KeyClusterSize, "3"),
				camundaconfig.Var(camundaconfig.KeyClusterPartitionCount, "3"),
			}
			g.Expect(k8sClient.Update(ctx, &brokers)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		for ordinal := range int32(brokerCount) {
			markJob(pitr, ordinal, batchv1.JobComplete)
		}

		Eventually(func(g Gomega) {
			current := readRestore(pitr)
			g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreCompleted))
			g.Expect(current.Status.PrimaryJobNames).To(HaveLen(brokerCount))
			g.Expect(current.Status.Brokers).To(Equal(int32(brokerCount)))
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			var third batchv1.Job
			key := types.NamespacedName{
				Namespace: pitr.Namespace,
				Name:      restore.JobName(labels.PointInTimeRestore(pitr.Name), 2),
			}
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, key, &third))).To(BeTrue())
		}, time.Second, interval).Should(Succeed())
	})

	// The recorded name and the derived name are one truth. A restore that
	// polls for a Job it can never create waits for ever, so the mismatch ends
	// it instead.
	It("fails when a recorded Job name is not the name of that Job", func() {
		w := createWorld()
		pitr := createRestore(w)
		expectJobs(pitr)

		// The controller writes the status of the restore as well, so a single
		// edit can be overwritten before a reconcile reads it. The edit is
		// applied again on every look, until the restore reports the mismatch.
		const wrong = "not-the-name-of-this-job"
		Eventually(func(g Gomega) {
			current := readRestore(pitr)
			if current.Status.Phase != v1.PointInTimeRestoreFailed {
				current.Status.PrimaryJobNames[0] = wrong
				_ = k8sClient.Status().Update(ctx, current)
			}

			current = readRestore(pitr)
			g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreFailed))
			g.Expect(current.Status.FailureMessage).To(ContainSubstring(wrong))
		}, timeout, interval).Should(Succeed())
	})

	// A cluster that loses a broker while the restore waits cannot run the
	// restore that was recorded for it. The message names that, not the render
	// error underneath.
	It("fails when the cluster is resized to fewer brokers during the restore", func() {
		w := createWorld()
		exporter.set(w.dbConfig.Spec.DatabaseName, answer{positions: positionsBehind(-2 * time.Minute)})
		pitr := createRestore(w)
		expectHeld(pitr, v1.ReasonDatabaseNotRestored)

		Eventually(func(g Gomega) {
			var brokers appsv1.StatefulSet
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(w.brokers), &brokers)).To(Succeed())
			brokers.Spec.Template.Spec.Containers[0].Env = []corev1.EnvVar{
				camundaconfig.Var(camundaconfig.KeyClusterSize, "1"),
				camundaconfig.Var(camundaconfig.KeyClusterPartitionCount, "3"),
			}
			g.Expect(k8sClient.Update(ctx, &brokers)).To(Succeed())
		}, timeout, interval).Should(Succeed())

		exporter.set(w.dbConfig.Spec.DatabaseName, answer{positions: positionsBehind(time.Hour)})

		Eventually(func(g Gomega) {
			current := readRestore(pitr)
			g.Expect(current.Status.Phase).To(Equal(v1.PointInTimeRestoreFailed))
			g.Expect(current.Status.FailureMessage).To(ContainSubstring("resized"))
			g.Expect(current.Status.Brokers).To(Equal(int32(brokerCount)))
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
