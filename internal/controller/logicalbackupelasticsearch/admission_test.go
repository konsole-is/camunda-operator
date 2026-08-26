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

package logicalbackupelasticsearch

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// admissionRig is a reconciler over the fake client with every reference
// that admission resolves, and no manager. It drives admit directly, so a
// test controls which reconciles interleave.
type admissionRig struct {
	client client.Client
	r      *Reconciler
}

func newAdmissionRig(t *testing.T, backups ...*v1.LogicalBackupElasticsearch) *admissionRig {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	require.NoError(t, v1.AddToScheme(s))

	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cc"},
		Spec: v1.CamundaClusterSpec{
			PlatformConfigRef: "platform",
			Version:           "8.9.9",
			StorageRef:        "storage",
			BackupStorageRef:  "bucket",
		},
		Status: v1.CamundaClusterStatus{Management: &v1.ManagementBinding{
			// A closed port: nothing at admission calls it, and the size
			// probe of start fails fast and is best effort.
			Endpoint:         "http://127.0.0.1:1",
			Auth:             v1.ManagementAuth{Method: v1.ManagementAuthMethodNone},
			Version:          "8.9.9",
			Partitions:       3,
			BackupRepository: "cc",
		}},
	}
	storage := &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "storage"},
		Spec: v1.SecondaryStorageConfigSpec{
			Type: v1.SecondaryStorageTypeElasticsearch,
			Elasticsearch: &v1.ElasticsearchStorage{
				Endpoint: "http://127.0.0.1:1",
				CredentialsSecretRef: v1.CredentialsSecretRef{
					Name: "missing", Namespace: "ns", UsernameKey: "u", PasswordKey: "p",
				},
			},
		},
	}
	bucket := &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "bucket"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeS3,
			S3: &v1.S3Storage{
				BucketName: "b", Region: "r",
				Auth: v1.S3StorageAuth{Type: v1.ObjectStorageAuthTypeWorkloadIdentity},
			},
		},
	}
	objects := make([]client.Object, 0, 3+len(backups))
	objects = append(objects, cluster, storage, bucket)
	for _, backup := range backups {
		objects = append(objects, backup)
	}
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(objects...).
		WithStatusSubresource(&v1.LogicalBackupElasticsearch{}).Build()

	return &admissionRig{client: c, r: &Reconciler{
		Client:        c,
		APIReader:     c,
		Scheme:        s,
		EventRecorder: events.NewFakeRecorder(16),
	}}
}

// pendingBackup builds a pending backup of cluster cc, created at the given
// time, that the fake client holds under its own UID.
func pendingBackup(name, uid string, created time.Time) *v1.LogicalBackupElasticsearch {
	return &v1.LogicalBackupElasticsearch{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "ns", Name: name, UID: types.UID(uid),
			CreationTimestamp: metav1.NewTime(created),
		},
		Spec: v1.LogicalBackupElasticsearchSpec{ClusterRef: v1.ClusterRef{Name: "cc"}},
	}
}

func readyReason(backup *v1.LogicalBackupElasticsearch) string {
	ready := meta.FindStatusCondition(backup.Status.Conditions, v1.ConditionReady)
	if ready == nil {
		return ""
	}
	return ready.Reason
}

// The race that the tie-break alone cannot close: A passed the pre-filter
// while alone and has not flushed its ID yet. B is created in the same
// second with a smaller name, so the tie-break lets B through as well. The
// claim is what stops B.
func TestAdmissionSerializesOnTheClaimNotOnTheTieBreak(t *testing.T) {
	created := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	a := pendingBackup("z-later-name", "uid-a", created)
	rig := newAdmissionRig(t, a)

	By := func(step string) { t.Log(step) }

	By("A admits alone: it takes the claim and allocates its ID, but the flush has not happened")
	_, err := rig.r.admit(t.Context(), a)
	require.NoError(t, err)
	require.NotZero(t, a.Status.BackupID)
	// The status of A in the API is still the pending one: nothing was
	// flushed. B reads that.

	By("B arrives in the same second with a smaller name")
	b := pendingBackup("a-earlier-name", "uid-b", created)
	require.NoError(t, rig.client.Create(t.Context(), b))

	By("B passes the tie-break: A looks pending and has the larger name")
	var seen v1.LogicalBackupElasticsearch
	require.NoError(t, rig.client.Get(t.Context(), client.ObjectKeyFromObject(a), &seen))
	require.Zero(t, seen.Status.BackupID, "the API still shows A as pending")
	require.True(t, seen.CreationTimestamp.Equal(&b.CreationTimestamp), "the same second")
	require.False(t, blocks(&seen, b), "the tie-break lets B through")

	_, err = rig.r.admit(t.Context(), b)
	require.NoError(t, err)

	By("B is stopped by the claim, and it names A")
	assert.Zero(t, b.Status.BackupID)
	assert.Equal(t, v1.LogicalBackupPending, b.Status.Phase)
	assert.Equal(t, v1.ReasonBackupInProgress, readyReason(b))
	assert.Contains(
		t, meta.FindStatusCondition(b.Status.Conditions, v1.ConditionReady).Message,
		"LogicalBackupElasticsearch/z-later-name",
	)

	By("A re-enters after a failed flush and still holds the claim")
	again := pendingBackup("z-later-name", "uid-a", created)
	_, err = rig.r.admit(t.Context(), again)
	require.NoError(t, err)
	assert.NotZero(t, again.Status.BackupID)
}
