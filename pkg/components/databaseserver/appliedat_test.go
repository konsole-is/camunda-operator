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
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSpecAppliedAtTakesTheNewestApplyOfTheSpec(t *testing.T) {
	older := metav1.NewTime(time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC))
	newer := metav1.NewTime(older.Add(time.Minute))
	newest := metav1.NewTime(older.Add(time.Hour))

	cases := map[string]struct {
		fields []metav1.ManagedFieldsEntry
		want   metav1.Time
	}{
		"no stamp": {},
		"the newest apply of the spec": {
			fields: []metav1.ManagedFieldsEntry{
				{Operation: metav1.ManagedFieldsOperationApply, Time: &older},
				{Operation: metav1.ManagedFieldsOperationApply, Time: &newer},
			},
			want: newer,
		},
		"an update and an apply of the status do not count": {
			fields: []metav1.ManagedFieldsEntry{
				{Operation: metav1.ManagedFieldsOperationApply, Time: &older},
				{Operation: metav1.ManagedFieldsOperationUpdate, Time: &newest},
				{Operation: metav1.ManagedFieldsOperationApply, Subresource: "status", Time: &newest},
			},
			want: older,
		},
		"an apply with no time": {
			fields: []metav1.ManagedFieldsEntry{{Operation: metav1.ManagedFieldsOperationApply}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got := specAppliedAt(tc.fields)

			assert.True(t, tc.want.Equal(&got), "got %v, want %v", got, tc.want)
		})
	}
}
