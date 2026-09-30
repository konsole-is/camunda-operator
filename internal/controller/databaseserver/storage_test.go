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

package databaseserver

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
)

// The write-ahead log volume answers two edits that the data volume never
// sees: a size lowered under the server, and the field cleared altogether.
// CloudNativePG refuses a cluster that gives the volume up, so the second one
// keeps the volume rather than removing it.
func TestKeepAppliedWALSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		requested *resource.Quantity
		existing  *resource.Quantity
		want      string
		reason    string
	}{
		{
			name:      "no volume asked for, and none there",
			requested: nil,
			existing:  nil,
		},
		{
			name:      "the volume is no longer asked for",
			requested: nil,
			existing:  new(resource.MustParse("4Gi")),
			want:      "4Gi",
			reason:    eventReasonWALStorageKept,
		},
		{
			name:      "a smaller volume is asked for",
			requested: new(resource.MustParse("1Gi")),
			existing:  new(resource.MustParse("4Gi")),
			want:      "4Gi",
			reason:    eventReasonStorageShrinkIgnored,
		},
		{
			name:      "a larger volume is asked for",
			requested: new(resource.MustParse("8Gi")),
			existing:  new(resource.MustParse("4Gi")),
			want:      "8Gi",
		},
		{
			name:      "the volume is asked for and none is there",
			requested: new(resource.MustParse("8Gi")),
			existing:  nil,
			want:      "8Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := events.NewFakeRecorder(4)
			r := &DatabaseServerReconciler{EventRecorder: recorder}
			server := &v1.DatabaseServer{ObjectMeta: metav1.ObjectMeta{Name: "my-db", Namespace: "ns"}}

			got := r.keepAppliedWALSize(server, tt.requested, tt.existing, false)

			if tt.want == "" {
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.want, got.String())
			}

			if tt.reason == "" {
				assert.Empty(t, recorder.Events)
				return
			}

			require.Len(t, recorder.Events, 1)
			assert.Contains(t, <-recorder.Events, tt.reason)
		})
	}
}

// A kept volume is reported once per requested size: the cluster carries the
// request it applied, and a request that matches it was reported before.
func TestKeepAppliedStorageSizeReportsOncePerRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		merged    v1.DatabaseServerSpec
		requested map[string]string
		hold      bool
		reason    string
	}{
		{
			name: "a smaller data volume, first asked for",
			merged: v1.DatabaseServerSpec{
				StorageSize:    new(resource.MustParse("1Gi")),
				WALStorageSize: new(resource.MustParse("2Gi")),
			},
			requested: map[string]string{components.RequestedStorageSizeAnnotation: "4Gi"},
			reason:    eventReasonStorageShrinkIgnored,
		},
		{
			name: "a smaller data volume, already applied",
			merged: v1.DatabaseServerSpec{
				StorageSize:    new(resource.MustParse("1Gi")),
				WALStorageSize: new(resource.MustParse("2Gi")),
			},
			requested: map[string]string{components.RequestedStorageSizeAnnotation: "1024Mi"},
		},
		{
			name: "a smaller data volume, with no cluster applied",
			merged: v1.DatabaseServerSpec{
				StorageSize:    new(resource.MustParse("1Gi")),
				WALStorageSize: new(resource.MustParse("2Gi")),
			},
			requested: nil,
			reason:    eventReasonStorageShrinkIgnored,
		},
		{
			name: "a smaller data volume, while the server is held for suspension",
			merged: v1.DatabaseServerSpec{
				StorageSize:    new(resource.MustParse("1Gi")),
				WALStorageSize: new(resource.MustParse("2Gi")),
			},
			requested: map[string]string{components.RequestedStorageSizeAnnotation: "4Gi"},
			hold:      true,
		},
		{
			name: "no write-ahead log volume, first asked for",
			merged: v1.DatabaseServerSpec{
				StorageSize: new(resource.MustParse("4Gi")),
			},
			requested: map[string]string{
				components.RequestedStorageSizeAnnotation:    "4Gi",
				components.RequestedWALStorageSizeAnnotation: "2Gi",
			},
			reason: eventReasonWALStorageKept,
		},
		{
			name: "no write-ahead log volume, already applied",
			merged: v1.DatabaseServerSpec{
				StorageSize: new(resource.MustParse("4Gi")),
			},
			requested: map[string]string{
				components.RequestedStorageSizeAnnotation:    "4Gi",
				components.RequestedWALStorageSizeAnnotation: "",
			},
		},
		{
			name: "a smaller write-ahead log volume, already applied",
			merged: v1.DatabaseServerSpec{
				StorageSize:    new(resource.MustParse("4Gi")),
				WALStorageSize: new(resource.MustParse("1Gi")),
			},
			requested: map[string]string{
				components.RequestedStorageSizeAnnotation:    "4Gi",
				components.RequestedWALStorageSizeAnnotation: "1Gi",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			recorder := events.NewFakeRecorder(4)
			r := &DatabaseServerReconciler{EventRecorder: recorder}
			server := &v1.DatabaseServer{ObjectMeta: metav1.ObjectMeta{Name: "my-db", Namespace: "ns"}}
			volumes := serverVolumes{
				appliedData: new(resource.MustParse("4Gi")),
				appliedWAL:  new(resource.MustParse("2Gi")),
				requested:   tt.requested,
			}
			resolved := resolvedSpec{merged: tt.merged, holdForSuspension: tt.hold}

			r.keepAppliedStorageSize(server, &resolved, volumes)

			assert.Equal(t, "4Gi", resolved.merged.StorageSize.String())
			assert.Equal(t, "2Gi", resolved.merged.WALStorageSize.String())
			assert.Equal(t, tt.merged.StorageSize, resolved.requested.Data)
			assert.Equal(t, tt.merged.WALStorageSize, resolved.requested.WAL)

			if tt.reason == "" {
				assert.Empty(t, recorder.Events)
				return
			}

			require.Len(t, recorder.Events, 1)
			assert.Contains(t, <-recorder.Events, tt.reason)
		})
	}
}
