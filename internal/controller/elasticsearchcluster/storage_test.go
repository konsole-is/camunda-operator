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
	"testing"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
)

// The request annotation is read live. The cache can still hold the ECK CR
// from before the last apply, and an old request there reports the same
// shrink again.
func TestDataVolumesReadsTheRequestLive(t *testing.T) {
	t.Parallel()

	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	require.NoError(t, esv1.AddToScheme(s))

	cluster := &v1.ElasticsearchCluster{ObjectMeta: metav1.ObjectMeta{Name: "es", Namespace: "ns"}}
	stale := &esv1.Elasticsearch{ObjectMeta: metav1.ObjectMeta{Name: "es", Namespace: "ns"}}
	applied := stale.DeepCopy()
	applied.Annotations = map[string]string{
		components.RequestedStorageSizeAnnotation:  "512Mi",
		components.RequestedStorageClassAnnotation: "class-b",
	}

	r := &ElasticsearchClusterReconciler{
		Client:    fake.NewClientBuilder().WithScheme(s).WithObjects(stale).Build(),
		APIReader: fake.NewClientBuilder().WithScheme(s).WithObjects(applied).Build(),
	}

	volumes, err := r.dataVolumes(t.Context(), cluster)
	require.NoError(t, err)
	assert.Equal(t, "512Mi", volumes.requested)
	assert.Equal(t, new("class-b"), volumes.requestedClass)
}

// An ECK CR of the same name that another owner controls never takes the
// request of this cluster, so a shrink under it records no event.
func TestKeepAppliedStorageSizeRecordsNothingUnderAForeignCR(t *testing.T) {
	t.Parallel()

	s := runtime.NewScheme()
	require.NoError(t, scheme.AddToScheme(s))
	require.NoError(t, esv1.AddToScheme(s))
	require.NoError(t, v1.AddToScheme(s))

	cluster := &v1.ElasticsearchCluster{ObjectMeta: metav1.ObjectMeta{Name: "es", Namespace: "ns", UID: "cluster-uid"}}
	foreign := &esv1.Elasticsearch{ObjectMeta: metav1.ObjectMeta{
		Name:      "es",
		Namespace: "ns",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: v1.GroupVersion.String(),
			Kind:       "ElasticsearchCluster",
			Name:       "other",
			UID:        "other-uid",
			Controller: new(true),
		}},
	}}
	foreign.Spec.NodeSets = []esv1.NodeSet{{
		Name: "default",
		VolumeClaimTemplates: []corev1.PersistentVolumeClaim{{
			ObjectMeta: metav1.ObjectMeta{Name: components.DataVolumeClaimName},
			Spec: corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse("4Gi")},
			}},
		}},
	}}

	recorder := events.NewFakeRecorder(4)
	c := fake.NewClientBuilder().WithScheme(s).WithObjects(foreign).Build()
	r := &ElasticsearchClusterReconciler{Client: c, APIReader: c, EventRecorder: recorder}

	volumes, err := r.dataVolumes(t.Context(), cluster)
	require.NoError(t, err)

	merged := v1.ElasticsearchClusterSpec{StorageSize: new(resource.MustParse("1Gi"))}
	r.keepAppliedStorageSize(cluster, &merged, volumes)

	assert.Empty(t, recorder.Events)
}

// The data volume claim keeps the largest size, and a requested shrink records
// an event while the applied ECK CR does not carry that request.
func TestKeepAppliedStorageSize(t *testing.T) {
	t.Parallel()

	applied := func(size, requested string) dataVolumes {
		return dataVolumes{
			applied: &corev1.PersistentVolumeClaimSpec{Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)},
			}},
			requested: requested,
		}
	}
	retained := dataVolumes{volumes: []v1.VolumeStatus{{Name: "data-0", Capacity: resource.MustParse("1Gi")}}}

	tests := []struct {
		name          string
		volumes       dataVolumes
		suspend       bool
		requested     string
		wantSize      string
		wantRequested string
		wantEvent     bool
	}{
		{
			name:          "a size above the applied one",
			volumes:       applied("1Gi", "1Gi"),
			requested:     "2Gi",
			wantSize:      "2Gi",
			wantRequested: "2Gi",
		},
		{
			name:          "a shrink, first asked for",
			volumes:       applied("1Gi", "1Gi"),
			requested:     "512Mi",
			wantSize:      "1Gi",
			wantRequested: "512Mi",
			wantEvent:     true,
		},
		{
			name:          "a shrink, already asked for",
			volumes:       applied("1Gi", "512Mi"),
			requested:     "512Mi",
			wantSize:      "1Gi",
			wantRequested: "512Mi",
		},
		{
			name:          "suspended with the ECK CR still there",
			volumes:       applied("1Gi", "1Gi"),
			suspend:       true,
			requested:     "512Mi",
			wantSize:      "1Gi",
			wantRequested: "1Gi",
		},
		{
			name:      "suspended with only the retained volumes",
			volumes:   retained,
			suspend:   true,
			requested: "512Mi",
			wantSize:  "1Gi",
		},
		{
			name:          "the retained volumes after a resume",
			volumes:       retained,
			requested:     "512Mi",
			wantSize:      "1Gi",
			wantRequested: "512Mi",
			wantEvent:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := events.NewFakeRecorder(1)
			r := &ElasticsearchClusterReconciler{EventRecorder: recorder}
			cluster := &v1.ElasticsearchCluster{ObjectMeta: metav1.ObjectMeta{Name: "es", Namespace: "ns"}}
			merged := v1.ElasticsearchClusterSpec{StorageSize: new(resource.MustParse(tt.requested)), Suspend: tt.suspend}

			requested := r.keepAppliedStorageSize(cluster, &merged, tt.volumes)

			if tt.wantRequested == "" {
				assert.Nil(t, requested)
			} else {
				require.NotNil(t, requested)
				assert.Equal(t, tt.wantRequested, requested.String())
			}
			assert.Equal(t, tt.wantSize, merged.StorageSize.String())
			if !tt.wantEvent {
				assert.Empty(t, recorder.Events)
				return
			}
			require.Len(t, recorder.Events, 1)
			assert.Contains(t, <-recorder.Events, eventReasonStorageShrinkIgnored)
		})
	}
}

// The data volume claim keeps the class of the applied ECK CR, and a
// requested class records at most one event.
func TestKeepAppliedStorageClass(t *testing.T) {
	t.Parallel()

	applied := func(class *string, requested *string) dataVolumes {
		return dataVolumes{
			applied:        &corev1.PersistentVolumeClaimSpec{StorageClassName: class},
			requestedClass: requested,
		}
	}

	tests := []struct {
		name      string
		volumes   dataVolumes
		suspend   bool
		requested *string
		wantClass *string
		wantEvent bool
		// keepsRequest expects the request that the applied CR carries in
		// place of the requested class.
		keepsRequest bool
	}{
		{
			name:      "no applied ECK CR",
			volumes:   dataVolumes{},
			requested: new("class-b"),
			wantClass: new("class-b"),
		},
		{
			name:      "the applied class",
			volumes:   applied(new("class-a"), new("class-a")),
			requested: new("class-a"),
			wantClass: new("class-a"),
		},
		{
			name:      "another class, first asked for",
			volumes:   applied(new("class-a"), new("class-a")),
			requested: new("class-b"),
			wantClass: new("class-a"),
			wantEvent: true,
		},
		{
			name:      "another class, already asked for",
			volumes:   applied(new("class-a"), new("class-b")),
			requested: new("class-b"),
			wantClass: new("class-a"),
		},
		{
			name:      "a class on a claim without one",
			volumes:   applied(nil, nil),
			requested: new("class-b"),
			wantEvent: true,
		},
		{
			name:      "no class on a claim with one",
			volumes:   applied(new("class-a"), new("class-a")),
			requested: nil,
			wantClass: new("class-a"),
			wantEvent: true,
		},
		{
			name:      "no class, already asked for",
			volumes:   applied(new("class-a"), nil),
			requested: nil,
			wantClass: new("class-a"),
		},
		{
			name:         "suspended with the ECK CR still there",
			volumes:      applied(new("class-a"), new("class-a")),
			suspend:      true,
			requested:    new("class-b"),
			wantClass:    new("class-a"),
			keepsRequest: true,
		},
		{
			name: "an ECK CR that another owner controls",
			volumes: dataVolumes{
				applied:        &corev1.PersistentVolumeClaimSpec{StorageClassName: new("class-a")},
				requestedClass: new("class-a"),
				foreign:        true,
			},
			requested: new("class-b"),
			wantClass: new("class-a"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := events.NewFakeRecorder(1)
			r := &ElasticsearchClusterReconciler{EventRecorder: recorder}
			cluster := &v1.ElasticsearchCluster{ObjectMeta: metav1.ObjectMeta{Name: "es", Namespace: "ns"}}
			merged := v1.ElasticsearchClusterSpec{StorageClassName: tt.requested, Suspend: tt.suspend}

			requested := r.keepAppliedStorageClass(cluster, &merged, tt.volumes)

			wantRequested := tt.requested
			if tt.keepsRequest {
				wantRequested = tt.volumes.requestedClass
			}
			assert.Equal(t, wantRequested, requested)
			assert.Equal(t, tt.wantClass, merged.StorageClassName)
			if !tt.wantEvent {
				assert.Empty(t, recorder.Events)
				return
			}
			require.Len(t, recorder.Events, 1)
			assert.Contains(t, <-recorder.Events, eventReasonStorageClassChangeIgnored)
		})
	}
}
