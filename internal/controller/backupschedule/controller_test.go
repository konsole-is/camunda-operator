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

package backupschedule

import (
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	eventsv1 "k8s.io/api/events/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// world is a cluster that a schedule can back up: the referenced contracts
// exist and are schema-valid. No other controller runs, so the specs shape
// any deeper state by hand.
type world struct {
	namespace string
	cluster   *v1.CamundaCluster
}

func newNamespace() string {
	name := "bs-ns-" + utilrand.String(8)
	Expect(k8sClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	})).To(Succeed())

	return name
}

// createWorld builds a cluster whose secondary storage is of the given type,
// with a backup bucket. Mutators shape the cluster before it is created.
func createWorld(storageType v1.SecondaryStorageType, mutate ...func(*v1.CamundaCluster)) *world {
	namespace := newNamespace()
	suffix := utilrand.String(6)

	storage := &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "ssc-" + suffix, Namespace: namespace},
		Spec:       v1.SecondaryStorageConfigSpec{Type: storageType},
	}
	switch storageType {
	case v1.SecondaryStorageTypeElasticsearch:
		storage.Spec.Elasticsearch = &v1.ElasticsearchStorage{
			Endpoint: "http://elasticsearch.elastic.svc:9200",
			CredentialsSecretRef: v1.CredentialsSecretRef{
				Name: "es-user", Namespace: namespace,
				UsernameKey: "username", PasswordKey: "password",
			},
		}
	case v1.SecondaryStorageTypeRDBMS:
		storage.Spec.RDBMS = &v1.RDBMSStorage{DatabaseConfigRef: "camunda-db"}
	}
	Expect(k8sClient.Create(ctx, storage)).To(Succeed())

	bucket := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "osc-" + suffix},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "camunda-backups",
				Endpoint:   "http://minio.minio.svc:9000",
				Auth: v1.S3StorageAuth{
					Type: v1.ObjectStorageAuthTypeCredentials,
					Credentials: &v1.S3Credentials{
						SecretRef: v1.S3CredentialsSecretRef{
							Name: "minio-credentials", Namespace: namespace,
							AccessKeyIDKey: "accessKeyId", SecretAccessKeyKey: "secretAccessKey",
						},
					},
				},
			},
		},
	}
	Expect(k8sClient.Create(ctx, bucket)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, bucket) })

	platform := &v1.CamundaPlatformConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cpc-" + utilrand.String(8)},
		Spec: v1.CamundaPlatformConfigSpec{
			Auth: &v1.PlatformAuthSpec{Method: v1.AuthenticationMethodBasic},
		},
	}
	Expect(k8sClient.Create(ctx, platform)).To(Succeed())
	DeferCleanup(func() { _ = k8sClient.Delete(ctx, platform) })

	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Name: "cc-" + suffix, Namespace: namespace},
		Spec: v1.CamundaClusterSpec{
			Version:           "8.9.9",
			PlatformConfigRef: platform.Name,
			StorageRef:        storage.Name,
			BackupStorageRef:  bucket.Name,
		},
	}
	for _, m := range mutate {
		m(cluster)
	}
	Expect(k8sClient.Create(ctx, cluster)).To(Succeed())

	return &world{namespace: namespace, cluster: cluster}
}

// createSchedule creates a daily schedule for the cluster of w and returns it
// together with its first trigger, computed the way the controller computes
// it: the first 02:00 UTC after the server-stamped creation time.
func createSchedule(w *world, mutate ...func(*v1.BackupSchedule)) (*v1.BackupSchedule, time.Time) {
	GinkgoHelper()
	schedule := &v1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nightly-" + utilrand.String(6), Namespace: w.namespace,
		},
		Spec: v1.BackupScheduleSpec{
			ClusterRef: v1.ClusterRef{Name: w.cluster.Name},
			Schedule:   "0 2 * * *",
		},
	}
	for _, m := range mutate {
		m(schedule)
	}
	Expect(k8sClient.Create(ctx, schedule)).To(Succeed())

	sched, err := parseCron(schedule.Spec.Schedule)
	Expect(err).NotTo(HaveOccurred())

	return schedule, sched.Next(schedule.CreationTimestamp.Time)
}

// touch forces a reconcile of the schedule after the clock moved: the
// controller requeues in real time, so a spec that moves the fake clock
// wakes it through a watched update instead of waiting.
func touch(schedule *v1.BackupSchedule) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var current v1.BackupSchedule
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(schedule), &current)).To(Succeed())
		if current.Annotations == nil {
			current.Annotations = map[string]string{}
		}
		current.Annotations["test.camunda.io/touch"] = utilrand.String(6)
		g.Expect(k8sClient.Update(ctx, &current)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

// scheduledOf lists the backups of both kinds that carry the schedule label.
func scheduledOf(schedule *v1.BackupSchedule) []client.Object {
	GinkgoHelper()
	selector := client.MatchingLabels{labels.BackupScheduleKey: schedule.Name}

	var es v1.LogicalBackupElasticsearchList
	Expect(k8sClient.List(ctx, &es, client.InNamespace(schedule.Namespace), selector)).To(Succeed())
	var rdbms v1.LogicalBackupRDBMSList
	Expect(k8sClient.List(ctx, &rdbms, client.InNamespace(schedule.Namespace), selector)).To(Succeed())

	items := make([]client.Object, 0, len(es.Items)+len(rdbms.Items))
	for i := range es.Items {
		items = append(items, &es.Items[i])
	}
	for i := range rdbms.Items {
		items = append(items, &rdbms.Items[i])
	}

	return items
}

// eventReasons lists the reasons of the events about the schedule, paired
// with their notes, as "<reason>: <note>".
func eventReasons(schedule *v1.BackupSchedule) []string {
	GinkgoHelper()
	var events eventsv1.EventList
	Expect(k8sClient.List(ctx, &events, client.InNamespace(schedule.Namespace))).To(Succeed())

	entries := make([]string, 0, len(events.Items))
	for _, event := range events.Items {
		if event.Regarding.Name != schedule.Name {
			continue
		}
		entries = append(entries, event.Reason+": "+event.Note)
	}

	return entries
}

// scheduledRDBMS hand-creates a backup that carries the schedule's label, the
// way a trigger of the schedule would have, and moves it to the given phase.
// The backup controllers do not run in this suite, so the phase stays where
// the spec puts it.
func scheduledRDBMS(
	schedule *v1.BackupSchedule,
	phase v1.LogicalBackupPhase,
	completed time.Time,
) *v1.LogicalBackupRDBMS {
	GinkgoHelper()
	backup := &v1.LogicalBackupRDBMS{
		ObjectMeta: metav1.ObjectMeta{
			Name: "seeded-" + utilrand.String(6), Namespace: schedule.Namespace,
			Labels: map[string]string{
				labels.ClusterKey:        schedule.Spec.ClusterRef.Name,
				labels.BackupScheduleKey: schedule.Name,
			},
		},
		Spec: v1.LogicalBackupRDBMSSpec{ClusterRef: schedule.Spec.ClusterRef},
	}
	Expect(k8sClient.Create(ctx, backup)).To(Succeed())
	setPhase(backup, phase, completed)

	return backup
}

// setPhase moves a hand-created backup to the given phase, with the
// completion time that the pruning sorts by on a terminal phase.
func setPhase(backup *v1.LogicalBackupRDBMS, phase v1.LogicalBackupPhase, completed time.Time) {
	GinkgoHelper()
	if phase == "" {
		return
	}
	Eventually(func(g Gomega) {
		var current v1.LogicalBackupRDBMS
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(backup), &current)).To(Succeed())
		current.Status.Phase = phase
		if phase == v1.LogicalBackupCompleted || phase == v1.LogicalBackupFailed {
			current.Status.CompletionTime = ptr.To(metav1.NewTime(completed))
		}
		g.Expect(k8sClient.Status().Update(ctx, &current)).To(Succeed())
	}, timeout, interval).Should(Succeed())
}

var _ = Describe("BackupSchedule controller", func() {
	BeforeEach(func() {
		// Every spec starts on the real clock; the ones that need a trigger
		// move it forward from here.
		clock.Set(time.Now().UTC())
	})

	It("creates a LogicalBackupElasticsearch for an Elasticsearch cluster at the trigger", func() {
		w := createWorld(v1.SecondaryStorageTypeElasticsearch)
		schedule, trigger := createSchedule(w)

		By("reporting Healthy before the first trigger")
		Eventually(func(g Gomega) {
			var current v1.BackupSchedule
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(schedule), &current)).To(Succeed())
			ready := meta.FindStatusCondition(current.Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionTrue))
			g.Expect(ready.Reason).To(Equal(v1.ReasonHealthy))
			g.Expect(current.Status.LastScheduleTime).To(BeNil())
		}, timeout, interval).Should(Succeed())
		Expect(scheduledOf(schedule)).To(BeEmpty())

		By("creating the backup once the trigger passes")
		clock.Set(trigger.Add(30 * time.Second))
		touch(schedule)

		name := schedule.Name + "-" + strconv.FormatInt(trigger.Unix(), 10)
		Eventually(func(g Gomega) {
			var backup v1.LogicalBackupElasticsearch
			g.Expect(k8sClient.Get(
				ctx,
				client.ObjectKey{Namespace: w.namespace, Name: name},
				&backup,
			)).To(Succeed())
			g.Expect(backup.Labels).To(HaveKeyWithValue(labels.ClusterKey, w.cluster.Name))
			g.Expect(backup.Labels).To(HaveKeyWithValue(labels.BackupScheduleKey, schedule.Name))
			g.Expect(backup.OwnerReferences).To(BeEmpty())
			g.Expect(backup.Spec.ClusterRef.Name).To(Equal(w.cluster.Name))

			var current v1.BackupSchedule
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(schedule), &current)).To(Succeed())
			g.Expect(current.Status.LastBackupName).To(Equal(name))
			g.Expect(current.Status.LastScheduleTime).NotTo(BeNil())
			g.Expect(current.Status.LastScheduleTime.Time).To(BeTemporally("==", trigger))
		}, timeout, interval).Should(Succeed())
	})

	It("creates a LogicalBackupRDBMS for a relational cluster at the trigger", func() {
		w := createWorld(v1.SecondaryStorageTypeRDBMS)
		schedule, trigger := createSchedule(w)

		clock.Set(trigger.Add(30 * time.Second))
		touch(schedule)

		name := schedule.Name + "-" + strconv.FormatInt(trigger.Unix(), 10)
		Eventually(func(g Gomega) {
			var backup v1.LogicalBackupRDBMS
			g.Expect(k8sClient.Get(
				ctx,
				client.ObjectKey{Namespace: w.namespace, Name: name},
				&backup,
			)).To(Succeed())
			g.Expect(backup.Labels).To(HaveKeyWithValue(labels.BackupScheduleKey, schedule.Name))
			g.Expect(backup.OwnerReferences).To(BeEmpty())
		}, timeout, interval).Should(Succeed())
	})

	It("skips the trigger of a suspended cluster with an event and consumes it", func() {
		w := createWorld(v1.SecondaryStorageTypeRDBMS, func(c *v1.CamundaCluster) {
			c.Spec.Suspend = true
		})
		schedule, trigger := createSchedule(w)

		clock.Set(trigger.Add(30 * time.Second))
		touch(schedule)

		Eventually(func(g Gomega) {
			g.Expect(eventReasons(schedule)).To(ContainElement(ContainSubstring("TriggerSkipped")))
			g.Expect(eventReasons(schedule)).To(ContainElement(ContainSubstring("suspended")))

			var current v1.BackupSchedule
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(schedule), &current)).To(Succeed())
			g.Expect(current.Status.LastScheduleTime).NotTo(BeNil())
			g.Expect(current.Status.LastScheduleTime.Time).To(BeTemporally("==", trigger))
			g.Expect(current.Status.LastBackupName).To(BeEmpty())
		}, timeout, interval).Should(Succeed())
		Expect(scheduledOf(schedule)).To(BeEmpty())
	})

	It("skips the trigger while a backup of the schedule is not terminal", func() {
		w := createWorld(v1.SecondaryStorageTypeRDBMS)
		schedule, trigger := createSchedule(w)

		By("creating the first backup at the first trigger")
		clock.Set(trigger.Add(30 * time.Second))
		touch(schedule)
		Eventually(func(g Gomega) {
			g.Expect(scheduledOf(schedule)).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		By("skipping the second trigger while the first backup has no terminal phase")
		second := trigger.Add(24 * time.Hour)
		clock.Set(second.Add(30 * time.Second))
		touch(schedule)

		Eventually(func(g Gomega) {
			g.Expect(eventReasons(schedule)).To(ContainElement(ContainSubstring("TriggerSkipped")))

			var current v1.BackupSchedule
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(schedule), &current)).To(Succeed())
			g.Expect(current.Status.LastScheduleTime.Time).To(BeTemporally("==", second))
		}, timeout, interval).Should(Succeed())
		Expect(scheduledOf(schedule)).To(HaveLen(1))
	})

	It("reports InvalidReference while the cluster does not resolve", func() {
		namespace := newNamespace()
		schedule := &v1.BackupSchedule{
			ObjectMeta: metav1.ObjectMeta{Name: "orphan-" + utilrand.String(6), Namespace: namespace},
			Spec: v1.BackupScheduleSpec{
				ClusterRef: v1.ClusterRef{Name: "no-such-cluster"},
				Schedule:   "0 2 * * *",
			},
		}
		Expect(k8sClient.Create(ctx, schedule)).To(Succeed())

		Eventually(func(g Gomega) {
			var current v1.BackupSchedule
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(schedule), &current)).To(Succeed())
			ready := meta.FindStatusCondition(current.Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(ready.Reason).To(Equal(v1.ReasonInvalidReference))
		}, timeout, interval).Should(Succeed())
	})

	It("skips the trigger of a cluster without a backup bucket and reports why", func() {
		w := createWorld(v1.SecondaryStorageTypeRDBMS, func(c *v1.CamundaCluster) {
			c.Spec.BackupStorageRef = ""
		})
		schedule, trigger := createSchedule(w)

		clock.Set(trigger.Add(30 * time.Second))
		touch(schedule)

		Eventually(func(g Gomega) {
			var current v1.BackupSchedule
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(schedule), &current)).To(Succeed())
			ready := meta.FindStatusCondition(current.Status.Conditions, v1.ConditionReady)
			g.Expect(ready).NotTo(BeNil())
			g.Expect(ready.Reason).To(Equal(v1.ReasonInvalidReference))
			g.Expect(ready.Message).To(ContainSubstring("backupStorageRef"))
			g.Expect(current.Status.LastScheduleTime).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
		Expect(scheduledOf(schedule)).To(BeEmpty())
	})

	It("prunes the oldest terminal backups beyond both bounds and nothing else", func() {
		w := createWorld(v1.SecondaryStorageTypeRDBMS)
		schedule, _ := createSchedule(w, func(s *v1.BackupSchedule) {
			s.Spec.Retained = &v1.RetainedBackups{
				Completed: ptr.To(int32(2)), Failed: ptr.To(int32(1)),
			}
		})

		base := time.Now().UTC().Add(-time.Hour)
		oldestCompleted := scheduledRDBMS(schedule, v1.LogicalBackupCompleted, base)
		keptCompleted1 := scheduledRDBMS(schedule, v1.LogicalBackupCompleted, base.Add(time.Minute))
		keptCompleted2 := scheduledRDBMS(schedule, v1.LogicalBackupCompleted, base.Add(2*time.Minute))
		oldestFailed := scheduledRDBMS(schedule, v1.LogicalBackupFailed, base)
		keptFailed := scheduledRDBMS(schedule, v1.LogicalBackupFailed, base.Add(time.Minute))
		running := scheduledRDBMS(schedule, v1.LogicalBackupRunning, time.Time{})

		// A manual backup of the same cluster carries no schedule label and
		// must never be counted or pruned, however old it is.
		manual := &v1.LogicalBackupRDBMS{
			ObjectMeta: metav1.ObjectMeta{
				Name: "manual-" + utilrand.String(6), Namespace: schedule.Namespace,
			},
			Spec: v1.LogicalBackupRDBMSSpec{ClusterRef: schedule.Spec.ClusterRef},
		}
		Expect(k8sClient.Create(ctx, manual)).To(Succeed())
		setPhase(manual, v1.LogicalBackupCompleted, base.Add(-24*time.Hour))

		By("pruning the overflow of both terminal phases")
		Eventually(func(g Gomega) {
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKeyFromObject(oldestCompleted), &v1.LogicalBackupRDBMS{},
			)).NotTo(Succeed())
			g.Expect(k8sClient.Get(
				ctx, client.ObjectKeyFromObject(oldestFailed), &v1.LogicalBackupRDBMS{},
			)).NotTo(Succeed())
		}, timeout, interval).Should(Succeed())

		By("keeping the retained, the running, and the manual backups")
		for _, kept := range []*v1.LogicalBackupRDBMS{keptCompleted1, keptCompleted2, keptFailed, running, manual} {
			Expect(k8sClient.Get(
				ctx, client.ObjectKeyFromObject(kept), &v1.LogicalBackupRDBMS{},
			)).To(Succeed(), kept.Name)
		}

		By("having pruned on the phase changes alone, without a trigger")
		var current v1.BackupSchedule
		Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(schedule), &current)).To(Succeed())
		Expect(current.Status.LastScheduleTime).To(BeNil())
	})

	It("leaves the backups behind when the schedule is deleted", func() {
		w := createWorld(v1.SecondaryStorageTypeRDBMS)
		schedule, trigger := createSchedule(w)

		clock.Set(trigger.Add(30 * time.Second))
		touch(schedule)
		Eventually(func(g Gomega) {
			g.Expect(scheduledOf(schedule)).To(HaveLen(1))
		}, timeout, interval).Should(Succeed())

		Expect(k8sClient.Delete(ctx, schedule)).To(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(schedule), &v1.BackupSchedule{})
			g.Expect(client.IgnoreNotFound(err)).To(Succeed())
			g.Expect(err).To(HaveOccurred())
		}, timeout, interval).Should(Succeed())

		Consistently(func(g Gomega) {
			g.Expect(scheduledOf(schedule)).To(HaveLen(1))
		}, 2*time.Second, interval).Should(Succeed())
	})

	It("warns when the retained dumps outlive the primary-storage retention window", func() {
		w := createWorld(v1.SecondaryStorageTypeRDBMS)
		schedule, trigger := createSchedule(w, func(s *v1.BackupSchedule) {
			s.Spec.Retained = &v1.RetainedBackups{Completed: ptr.To(int32(200))}
		})

		clock.Set(trigger.Add(30 * time.Second))
		touch(schedule)

		Eventually(func(g Gomega) {
			g.Expect(scheduledOf(schedule)).To(HaveLen(1))
			g.Expect(eventReasons(schedule)).To(
				ContainElement(ContainSubstring("RetentionWindowExceeded")),
			)
		}, timeout, interval).Should(Succeed())
	})
})
