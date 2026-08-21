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
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// dnsName returns a DNS subdomain of exactly length characters.
func dnsName(prefix string, length int) string {
	return prefix + strings.Repeat("x", length-len(prefix))
}

// scheduleNamed returns a schedule of the given name that references a
// cluster of the same length, so both derived label values meet the same
// bound.
func scheduleNamed(name string) *v1.BackupSchedule {
	const clusterPrefix = "cluster-"

	return &v1.BackupSchedule{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "backups"},
		Spec: v1.BackupScheduleSpec{
			ClusterRef: v1.ClusterRef{Name: dnsName(clusterPrefix, max(len(name), len(clusterPrefix)+1))},
			Schedule:   "0 2 * * *",
		},
	}
}

func TestNewBackupBoundsTheDerivedNames(t *testing.T) {
	due := time.Unix(1787104800, 0).UTC()

	for _, storageType := range []v1.SecondaryStorageType{
		v1.SecondaryStorageTypeElasticsearch,
		v1.SecondaryStorageTypeRDBMS,
	} {
		for _, length := range []int{63, 100, validation.DNS1123SubdomainMaxLength} {
			t.Run(fmt.Sprintf("%s at %d characters", storageType, length), func(t *testing.T) {
				schedule := scheduleNamed(dnsName("nightly-", length))
				backup, _ := newBackup(schedule, storageType, due)

				assert.Empty(t, validation.IsDNS1123Subdomain(backup.GetName()))
				for key, value := range backup.GetLabels() {
					assert.Empty(t, validation.IsValidLabelValue(value), "label %q", key)
				}

				assert.Equal(
					t,
					labels.OwnerName(schedule.Name),
					backup.GetLabels()[labels.BackupScheduleKey],
				)
				assert.Equal(
					t,
					labels.OwnerName(schedule.Spec.ClusterRef.Name),
					backup.GetLabels()[labels.ClusterKey],
				)
				assert.True(
					t,
					strings.HasSuffix(backup.GetName(), "-1787104800"),
					"the name carries the trigger time: %q", backup.GetName(),
				)
			})
		}
	}
}

func TestNewBackupKeepsTwoLongSchedulesApart(t *testing.T) {
	due := time.Unix(1787104800, 0).UTC()
	head := dnsName("nightly-", 250)

	first, _ := newBackup(scheduleNamed(head+"-a"), v1.SecondaryStorageTypeRDBMS, due)
	second, _ := newBackup(scheduleNamed(head+"-b"), v1.SecondaryStorageTypeRDBMS, due)

	assert.NotEqual(t, first.GetName(), second.GetName())
	assert.NotEqual(
		t,
		first.GetLabels()[labels.BackupScheduleKey],
		second.GetLabels()[labels.BackupScheduleKey],
	)
}

// backupLabeled returns a backup that carries the given schedule label value,
// the way a trigger of that schedule writes it.
func backupLabeled(namespace, value string) *v1.LogicalBackupRDBMS {
	return &v1.LogicalBackupRDBMS{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "backup",
			Namespace: namespace,
			Labels:    map[string]string{labels.BackupScheduleKey: value},
		},
	}
}

func TestEnqueueSchedule(t *testing.T) {
	scheme, err := v1.SchemeBuilder.Build()
	require.NoError(t, err)

	short := "nightly"
	long := dnsName("nightly-", 100)

	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			scheduleNamed(short),
			scheduleNamed(long),
			scheduleNamed(dnsName("elsewhere-", 100)),
		).
		Build()

	h := enqueueSchedule(c)

	requests := func(backup *v1.LogicalBackupRDBMS) []reconcile.Request {
		t.Helper()
		q := workqueue.NewTypedRateLimitingQueue(
			workqueue.DefaultTypedControllerRateLimiter[reconcile.Request](),
		)
		h.Create(context.Background(), event.CreateEvent{Object: backup}, q)

		var reqs []reconcile.Request
		for q.Len() > 0 {
			req, _ := q.Get()
			reqs = append(reqs, req)
			q.Done(req)
		}

		return reqs
	}

	t.Run("maps a backup of a short-named schedule", func(t *testing.T) {
		assert.Equal(
			t,
			[]reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: "backups", Name: short}}},
			requests(backupLabeled("backups", short)),
		)
	})

	t.Run("maps a backup of a long-named schedule back to the full name", func(t *testing.T) {
		value := labels.OwnerName(long)
		require.NotEqual(t, long, value)

		assert.Equal(
			t,
			[]reconcile.Request{{NamespacedName: types.NamespacedName{Namespace: "backups", Name: long}}},
			requests(backupLabeled("backups", value)),
		)
	})

	t.Run("maps a manual backup to nothing", func(t *testing.T) {
		assert.Empty(t, requests(backupLabeled("backups", "")))
	})

	t.Run("maps a label that no schedule of the namespace carries to nothing", func(t *testing.T) {
		assert.Empty(t, requests(backupLabeled("backups", "no-such-schedule")))
		assert.Empty(t, requests(backupLabeled("other", labels.OwnerName(long))))
	})
}
