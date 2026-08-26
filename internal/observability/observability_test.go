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

package observability

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestForgetRemovesTheSeriesOfOneOwnerOnly(t *testing.T) {
	rec := Recorder("testkind")
	gone := &metav1.ObjectMeta{Name: "gone", Namespace: "ns"}
	kept := &metav1.ObjectMeta{Name: "kept", Namespace: "ns"}
	rec.RecordConditionFor("TestKind", gone, "Ready", "False", "Failing", time.Now())
	rec.RecordConditionFor("TestKind", kept, "Ready", "True", "Healthy", time.Now())
	require.Equal(t, 2, testutil.CollectAndCount(conditions))

	Forget(rec, "TestKind", types.NamespacedName{Namespace: "ns", Name: "gone"})

	assert.Equal(t, 1, testutil.CollectAndCount(conditions))

	Forget(rec, "TestKind", types.NamespacedName{Namespace: "ns", Name: "kept"})
	assert.Equal(t, 0, testutil.CollectAndCount(conditions))
}

func TestForgetToleratesANilRecorder(t *testing.T) {
	assert.NotPanics(t, func() {
		Forget(nil, "TestKind", types.NamespacedName{Namespace: "ns", Name: "x"})
	})
}
