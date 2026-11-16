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

package echo

import (
	"context"
	"encoding/json"
	"slices"
	"testing"

	"github.com/go-logr/logr/funcr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// newStatusController is newFakeController with the status subresource of
// PluginRequest and a recorder, so that writeStatus can run on req.
func newStatusController(t *testing.T, req *pluginsv1alpha1.PluginRequest) *Controller {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, pluginsv1alpha1.AddToScheme(scheme))

	c, err := New(validOptions())
	require.NoError(t, err)
	c.client = fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(req).
		WithStatusSubresource(&pluginsv1alpha1.PluginRequest{}).
		Build()
	c.recorder = events.NewFakeRecorder(10)

	return c
}

// jsonLines returns a context whose logger appends each line, as JSON, to lines.
func jsonLines(t *testing.T, lines *[]map[string]any) context.Context {
	t.Helper()

	logger := funcr.NewJSON(func(obj string) {
		var line map[string]any
		require.NoError(t, json.Unmarshal([]byte(obj), &line))
		*lines = append(*lines, line)
	}, funcr.Options{})

	return log.IntoContext(t.Context(), logger)
}

// stored reads req back, so that a write carries its resourceVersion.
func stored(t *testing.T, c *Controller, req *pluginsv1alpha1.PluginRequest) *pluginsv1alpha1.PluginRequest {
	t.Helper()

	var got pluginsv1alpha1.PluginRequest
	require.NoError(t, c.client.Get(t.Context(), client.ObjectKeyFromObject(req), &got))

	return &got
}

func TestWriteStatusLogsTheRequestWithItsNamespace(t *testing.T) {
	req := grantRequest(map[string]string{"source": "hello"})
	c := newStatusController(t, req)
	var lines []map[string]any

	_, err := c.writeStatus(jsonLines(t, &lines), stored(t, c, req))

	require.NoError(t, err)
	require.NotEmpty(t, lines)
	for _, line := range lines {
		assert.Equal(t, map[string]any{"name": requestName, "namespace": "ns"}, line["request"], "line %v", line)
	}
}

func TestWriteStatusLogsTheFirstReadyOnce(t *testing.T) {
	req := grantRequest(map[string]string{"source": "hello"})
	c := newStatusController(t, req)
	var lines []map[string]any
	ctx := jsonLines(t, &lines)

	_, err := c.writeStatus(ctx, stored(t, c, req))
	require.NoError(t, err)

	_, err = c.writeStatus(ctx, stored(t, c, req))
	require.NoError(t, err)

	ready := slices.DeleteFunc(slices.Clone(lines), func(l map[string]any) bool {
		return l["msg"] != "Set Ready of PluginRequest"
	})
	require.Len(t, ready, 1)
	assert.Equal(t, "True", ready[0]["status"])
	assert.Equal(t, v1.ReasonHealthy, ready[0]["reason"])
}

// TestLogReadyChange checks that the Ready line is written once per change of
// the status or the reason, not once per status write.
func TestLogReadyChange(t *testing.T) {
	ready := metav1.Condition{Type: v1.ConditionReady, Status: metav1.ConditionTrue, Reason: v1.ReasonHealthy}
	changed := func(f func(*metav1.Condition)) *metav1.Condition {
		c := ready
		f(&c)
		return &c
	}

	for name, tc := range map[string]struct {
		before *metav1.Condition
		after  *metav1.Condition
		line   bool
	}{
		"first condition": {before: nil, after: &ready, line: true},
		"reason changed": {
			before: &ready, after: changed(func(c *metav1.Condition) { c.Reason = "Blocked" }), line: true,
		},
		"status changed": {
			before: &ready, after: changed(func(c *metav1.Condition) { c.Status = metav1.ConditionFalse }), line: true,
		},
		"message changed": {
			before: &ready, after: changed(func(c *metav1.Condition) { c.Message = "new" }), line: false,
		},
		"no condition after": {before: &ready, after: nil, line: false},
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
