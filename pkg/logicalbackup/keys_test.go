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

package logicalbackup_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
)

func TestAllocateBackupID(t *testing.T) {
	at := metav1.NewTime(time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC))

	assert.Equal(t, at.UnixMilli(), logicalbackup.AllocateBackupID(at))
}

// Camunda rejects an id that is not greater than every id the cluster already
// holds, and the pre-checks do not prevent two backups from starting within
// the same second. At second resolution those two would collide.
func TestAllocateBackupIDAfterFollowsTheClockWhenItIsAhead(t *testing.T) {
	at := metav1.NewTime(time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC))

	assert.Equal(t, at.UnixMilli(), logicalbackup.AllocateBackupIDAfter(at, at.UnixMilli()-1))
	assert.Equal(t, at.UnixMilli(), logicalbackup.AllocateBackupIDAfter(at, 0))
}

func TestAllocateBackupIDAfterStepsPastAHigherID(t *testing.T) {
	// The clock stepped back: a sibling holds an id from later than now.
	at := metav1.NewTime(time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC))
	highest := at.UnixMilli() + 5_000

	assert.Equal(t, highest+1, logicalbackup.AllocateBackupIDAfter(at, highest))
	assert.Equal(t, at.UnixMilli()+1, logicalbackup.AllocateBackupIDAfter(at, at.UnixMilli()))
}

func TestAllocateBackupIDSeparatesBackupsWithinOneSecond(t *testing.T) {
	base := time.Date(2026, 8, 16, 12, 0, 0, 0, time.UTC)

	first := logicalbackup.AllocateBackupID(metav1.NewTime(base))
	second := logicalbackup.AllocateBackupID(metav1.NewTime(base.Add(500 * time.Millisecond)))

	assert.Greater(t, second, first)
}

// ClusterPrefix is the one definition of the bucket layout; the repository
// base_path and every object key build on it, so a leading or trailing slash
// in the contract must not fork the layout.
func TestClusterPrefix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		basePath string
		want     string
	}{
		{name: "no base path", basePath: "", want: "camunda/my-cluster"},
		{name: "plain base path", basePath: "backups", want: "backups/camunda/my-cluster"},
		{name: "leading slash trimmed", basePath: "/backups", want: "backups/camunda/my-cluster"},
		{name: "trailing slash trimmed", basePath: "backups/", want: "backups/camunda/my-cluster"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, test.want, logicalbackup.ClusterPrefix(test.basePath, "camunda", "my-cluster"))
		})
	}
}

func TestObjectKeyPrefix(t *testing.T) {
	tests := []struct {
		name     string
		basePath string
		expected string
	}{
		{
			name:     "base path prefixes every key",
			basePath: "clusters",
			expected: "clusters/my-ns/my-cluster/1748937221",
		},
		{
			name:     "empty base path means the bucket root",
			basePath: "",
			expected: "my-ns/my-cluster/1748937221",
		},
		{
			name:     "surrounding slashes of the base path do not double up",
			basePath: "/clusters/",
			expected: "clusters/my-ns/my-cluster/1748937221",
		},
		{
			name:     "a nested base path is kept as it is",
			basePath: "camunda/backups",
			expected: "camunda/backups/my-ns/my-cluster/1748937221",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(
				t,
				tt.expected,
				logicalbackup.ObjectKeyPrefix(tt.basePath, "my-ns", "my-cluster", 1748937221),
			)
		})
	}
}

// The finalizer is API surface: a user who deletes a backup while its store
// is unreachable finds this string on the object.
func TestFinalizer(t *testing.T) {
	assert.Equal(t, "core.camunda.io/backup-artifacts", logicalbackup.Finalizer)
}
