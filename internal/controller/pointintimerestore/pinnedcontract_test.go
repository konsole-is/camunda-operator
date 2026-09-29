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
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
)

func TestPinnedContractKeysLeaveOutFinishedRestores(t *testing.T) {
	restoreIn := func(phase v1.PointInTimeRestorePhase, storage *v1.PointInTimeRestoreStorage) *v1.PointInTimeRestore {
		return &v1.PointInTimeRestore{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "pitr"},
			Status:     v1.PointInTimeRestoreStatus{Phase: phase, Storage: storage},
		}
	}
	pinned := &v1.PointInTimeRestoreStorage{DatabaseServerConfig: "server"}
	key := refindex.NamespacedKey("ns", "server")

	cases := []struct {
		name  string
		pitr  *v1.PointInTimeRestore
		wants []string
	}{
		{"a running restore", restoreIn(v1.PointInTimeRestoreRestoringDatabase, pinned), []string{key}},
		{"a completed restore", restoreIn(v1.PointInTimeRestoreCompleted, pinned), nil},
		{"a failed restore", restoreIn(v1.PointInTimeRestoreFailed, pinned), nil},
		{"a restore that pinned nothing", restoreIn(v1.PointInTimeRestorePending, nil), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.wants, pinnedContractKeys(c.pitr))
		})
	}
}
