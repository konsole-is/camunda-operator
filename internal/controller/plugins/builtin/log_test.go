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

package builtin

import (
	"encoding/json"
	"testing"

	"github.com/go-logr/logr/funcr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestRequestLogger(t *testing.T) {
	var lines []map[string]any
	base := funcr.NewJSON(func(obj string) {
		var line map[string]any
		require.NoError(t, json.Unmarshal([]byte(obj), &line))
		lines = append(lines, line)
	}, funcr.Options{})
	logger := requestLogger(base)

	logger(&reconcile.Request{NamespacedName: types.NamespacedName{Namespace: "ns", Name: "orders"}}).
		Info("Reconciler error")
	logger(nil).Info("Starting Controller")

	require.Len(t, lines, 2)
	assert.Equal(t, map[string]any{"name": "orders", "namespace": "ns"}, lines[0]["request"])
	assert.Equal(t, "builtin", lines[0]["controller"])
	assert.NotContains(t, lines[1], "request")
	assert.Equal(t, "builtin", lines[1]["controller"])
}

// TestLogReadyChange checks that the Ready line is written once per change of
// the status or the reason, not once per reconcile.
func TestLogReadyChange(t *testing.T) {
	ready := metav1.Condition{Type: v1.ConditionReady, Status: metav1.ConditionTrue, Reason: v1.ReasonHealthy}
	changed := func(f func(*metav1.Condition)) *metav1.Condition {
		c := ready
		f(&c)
		return &c
	}

	for name, tc := range map[string]struct {
		before metav1.Condition
		after  *metav1.Condition
		line   bool
	}{
		"unchanged": {
			before: ready, after: changed(func(c *metav1.Condition) { c.Message = "new" }), line: false,
		},
		"reason changed": {
			before: ready, after: changed(func(c *metav1.Condition) { c.Reason = "Blocked" }), line: true,
		},
		"status changed": {
			before: ready, after: changed(func(c *metav1.Condition) { c.Status = metav1.ConditionFalse }), line: true,
		},
		"first condition": {before: metav1.Condition{}, after: &ready, line: true},
		"no condition":    {before: ready, after: nil, line: false},
	} {
		t.Run(name, func(t *testing.T) {
			var lines []string
			sink := funcr.New(func(_, args string) { lines = append(lines, args) }, funcr.Options{})
			logReadyChange(sink, tc.before, tc.after)

			if !tc.line {
				assert.Empty(t, lines)
				return
			}
			require.Len(t, lines, 1)
			assert.Contains(t, lines[0], `"msg"="Set Ready of PluginRequest"`)
		})
	}
}
