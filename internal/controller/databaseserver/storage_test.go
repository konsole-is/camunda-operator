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

			got := r.keepAppliedWALSize(server, tt.requested, tt.existing)

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
