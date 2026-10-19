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

package conformance

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestOutputProblems(t *testing.T) {
	declared := []pluginsv1alpha1.OutputDecl{{Key: "message"}, {Key: "url"}}

	tests := []struct {
		name     string
		outputs  map[string]string
		problems []string
	}{
		{"exact keys", map[string]string{"message": "hi", "url": "https://x"}, nil},
		{"a missing key", map[string]string{"message": "hi"}, []string{`output "url" is missing`}},
		{
			"an extra key",
			map[string]string{"message": "hi", "url": "https://x", "extra": "y"},
			[]string{`output "extra" is not declared in spec.outputs of the registration`},
		},
		{"an empty value", map[string]string{"message": "", "url": "https://x"}, []string{`output "message" is empty`}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.problems, outputProblems(declared, tt.outputs))
		})
	}
}

func grantWithReady(status metav1.ConditionStatus) pluginsv1alpha1.Grant {
	return pluginsv1alpha1.Grant{Status: pluginsv1alpha1.GrantStatus{
		Conditions: []metav1.Condition{{Type: v1.ConditionReady, Status: status, Reason: "Rejected"}},
	}}
}

func TestReadyVerdict(t *testing.T) {
	ready := func(status metav1.ConditionStatus, reason string) *metav1.Condition {
		return &metav1.Condition{Type: v1.ConditionReady, Status: status, Reason: reason, Message: "the echo is broken"}
	}

	tests := []struct {
		name    string
		ready   *metav1.Condition
		grants  []pluginsv1alpha1.Grant
		message string
	}{
		{name: "Ready=True", ready: ready(metav1.ConditionTrue, v1.ReasonHealthy)},
		{name: "no Ready", message: "has no Ready condition"},
		{
			name:    "Ready=False with another reason",
			ready:   ready(metav1.ConditionFalse, "EchoDefect"),
			message: "EchoDefect",
		},
		{
			name:  "GrantRejected with a failed Grant",
			ready: ready(metav1.ConditionFalse, pluginsv1alpha1.ReasonGrantRejected),
			grants: []pluginsv1alpha1.Grant{
				grantWithReady(metav1.ConditionTrue),
				grantWithReady(metav1.ConditionFalse),
			},
		},
		{
			name:    "GrantRejected without a failed Grant",
			ready:   ready(metav1.ConditionFalse, pluginsv1alpha1.ReasonGrantRejected),
			grants:  []pluginsv1alpha1.Grant{grantWithReady(metav1.ConditionTrue)},
			message: "no Grant of the request has Ready=False",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := readyVerdict(tt.ready, tt.grants)

			if tt.message == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.message)
		})
	}
}

// testRequestName is the name of the request of testRequest.
const testRequestName = "basic-delete"

func testRequest() *pluginsv1alpha1.PluginRequest {
	req := &pluginsv1alpha1.PluginRequest{}
	req.Namespace = "ns"
	req.Name = testRequestName
	req.UID = types.UID("request-uid")
	req.Spec.Registration = "echo-1"
	return req
}

func written(
	kind, namespace, name string,
	labels map[string]string,
	owners ...metav1.OwnerReference,
) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetKind(kind)
	obj.SetNamespace(namespace)
	obj.SetName(name)
	obj.SetLabels(labels)
	obj.SetOwnerReferences(owners)
	return obj
}

func writerLabels() map[string]string {
	return map[string]string{
		pluginsv1alpha1.LabelRegistration: "echo-1",
		pluginsv1alpha1.LabelRequest:      testRequestName,
	}
}

func ownerOf(req *pluginsv1alpha1.PluginRequest) metav1.OwnerReference {
	return metav1.OwnerReference{
		APIVersion: pluginsv1alpha1.GroupVersion.String(),
		Kind:       kindPluginRequest,
		Name:       req.Name,
		UID:        req.UID,
	}
}

func TestLabelProblems(t *testing.T) {
	req := testRequest()
	otherUID := ownerOf(req)
	otherUID.UID = "other-uid"

	tests := []struct {
		name     string
		obj      *unstructured.Unstructured
		contains []string
	}{
		{name: "a namespaced object with both labels and the owner reference",
			obj: written("ConfigMap", "ns", "x", writerLabels(), ownerOf(req))},
		{name: "a cluster-scoped object with both labels and no owner reference",
			obj: written("StorageClass", "", "x", writerLabels())},
		{
			name:     "a cluster-scoped object with an owner reference to the request",
			obj:      written("StorageClass", "", "x", writerLabels(), ownerOf(req)),
			contains: []string{"StorageClass x is cluster-scoped and has an owner reference to the request"},
		},
		{
			name:     "a namespaced object without the owner reference",
			obj:      written("ConfigMap", "ns", "x", writerLabels()),
			contains: []string{"ConfigMap ns/x has no owner reference to the request"},
		},
		{
			name: "an object without the registration label",
			obj: written(
				"ConfigMap", "ns", "x", map[string]string{pluginsv1alpha1.LabelRequest: testRequestName},
				ownerOf(req),
			),
			contains: []string{`ConfigMap ns/x has no label plugins.camunda.io/registration: "echo-1"`},
		},
		{
			name:     "an owner reference with another UID",
			obj:      written("ConfigMap", "ns", "x", writerLabels(), otherUID),
			contains: []string{"ConfigMap ns/x has no owner reference to the request"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problems := labelProblems(tt.obj, req)

			require.Len(t, problems, len(tt.contains), problems)
			for i, c := range tt.contains {
				assert.Contains(t, problems[i], c)
			}
		})
	}
}

func TestLabelProblemsNamesAnObjectOutsideTheRequestNamespace(t *testing.T) {
	problems := labelProblems(written("ConfigMap", "other", "x", writerLabels()), testRequest())

	assert.Equal(t, []string{"ConfigMap other/x is outside the namespace of the request"}, problems)
}

func TestResourceProblems(t *testing.T) {
	objs := []*unstructured.Unstructured{
		written("ConfigMap", "ns", "listed", nil),
		written("StorageClass", "", "unlisted", nil),
	}
	rows := []pluginsv1alpha1.ResourceRef{
		{Kind: "ConfigMap", Name: "listed", OnOrphan: pluginsv1alpha1.OnOrphanKept},
		{Kind: "S3Bucket", Name: "cloud-bucket", OnOrphan: pluginsv1alpha1.OnOrphanKept},
	}

	assert.Equal(t, []string{"StorageClass unlisted is not listed in status.resources"}, resourceProblems(objs, rows))
}

func TestResourceProblemsNamesAnIncompleteRow(t *testing.T) {
	rows := []pluginsv1alpha1.ResourceRef{{Kind: "S3Bucket", Name: "b"}}

	assert.Equal(t, []string{`status.resources row S3Bucket "b" has no onOrphan`}, resourceProblems(nil, rows))
}

func TestEventCount(t *testing.T) {
	events := []corev1.Event{
		{Count: 0},
		{Count: 3},
		{Count: 1, Series: &corev1.EventSeries{Count: 5}},
	}

	assert.Equal(t, 9, eventCount(events))
}

func TestDisplayProblems(t *testing.T) {
	now := metav1.NewTime(time.Now())
	full := func() pluginsv1alpha1.PluginRequestStatus {
		return pluginsv1alpha1.PluginRequestStatus{
			Summary: []pluginsv1alpha1.SummaryItem{
				{Label: "Message", Value: "hi", Severity: pluginsv1alpha1.SeverityInfo},
				{Label: "Note", Value: "no severity"},
			},
			SummaryTime: &now,
			Links:       []pluginsv1alpha1.Link{{Name: "Docs", URL: "https://example.com/echo"}},
		}
	}

	tests := []struct {
		name    string
		change  func(s *pluginsv1alpha1.PluginRequestStatus)
		message string
	}{
		{name: "a full status", change: func(*pluginsv1alpha1.PluginRequestStatus) {}},
		{name: "an empty summary", change: func(s *pluginsv1alpha1.PluginRequestStatus) { s.Summary = nil },
			message: "status.summary has no item"},
		{
			name:    "an unknown severity",
			change:  func(s *pluginsv1alpha1.PluginRequestStatus) { s.Summary[0].Severity = "Fatal" },
			message: `"Fatal"`,
		},
		{name: "no summaryTime", change: func(s *pluginsv1alpha1.PluginRequestStatus) { s.SummaryTime = nil },
			message: "status.summaryTime is not set"},
		{name: "a relative link", change: func(s *pluginsv1alpha1.PluginRequestStatus) { s.Links[0].URL = "/bucket" },
			message: `"/bucket"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status := full()
			tt.change(&status)

			problems := displayProblems(status)

			if tt.message == "" {
				assert.Empty(t, problems)
				return
			}
			require.Len(t, problems, 1)
			assert.Contains(t, problems[0], tt.message)
		})
	}
}

func TestLogProblems(t *testing.T) {
	long := "not json " + strings.Repeat("x", 200)

	tests := []struct {
		name    string
		lines   []string
		message string
	}{
		{name: "a line with the request name", lines: []string{`{"msg":"start"}`, `{"request":"basic-delete"}`, ""}},
		{name: "a line with the request object",
			lines: []string{`{"request":{"name":"basic-delete","namespace":"ns"}}`}},
		{name: "a line that is not JSON", lines: []string{`{"request":"basic-delete"}`, long},
			message: `"not json xxx`},
		{name: "a null line", lines: []string{`{"request":"basic-delete"}`, `null`}, message: `"null"`},
		{name: "JSON lines without the request", lines: []string{`{"request":"other"}`, `{"msg":"x"}`},
			message: `no log line carries the key request with the request name "basic-delete"`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			problems := logProblems(tt.lines, testRequestName)

			if tt.message == "" {
				assert.Empty(t, problems)
				return
			}
			require.Len(t, problems, 1)
			assert.Contains(t, problems[0], tt.message)
			assert.Less(t, len(problems[0]), 200, "a quoted line is cut")
		})
	}
}

func TestAcceptedProblemNamesAnOverwrittenMessageOrGeneration(t *testing.T) {
	accepted := &metav1.Condition{
		Type:    "Accepted",
		Status:  metav1.ConditionTrue,
		Reason:  "Healthy",
		Message: "Accepted",
	}

	message := accepted.DeepCopy()
	message.Message = "Served by echo"
	generation := accepted.DeepCopy()
	generation.ObservedGeneration = 7

	assert.ErrorContains(t, acceptedProblem(accepted, message), "Served by echo")
	assert.ErrorContains(t, acceptedProblem(accepted, generation), "generation 7")
}

func TestLateProblem(t *testing.T) {
	created := metav1.NewTime(time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC))
	at := func(after time.Duration, status metav1.ConditionStatus) *metav1.Condition {
		return &metav1.Condition{Type: "Ready", Status: status, LastTransitionTime: metav1.NewTime(created.Add(after))}
	}

	assert.NoError(t, lateProblem(at(20*time.Second, metav1.ConditionTrue), created, 20*time.Second))
	assert.NoError(t, lateProblem(at(time.Minute, metav1.ConditionFalse), created, 20*time.Second))
	assert.ErrorContains(
		t,
		lateProblem(at(25*time.Second, metav1.ConditionTrue), created, 20*time.Second),
		"Ready became True 25s after the request was created, which is more than the timeout of 20s",
	)
}

func TestAcceptedProblem(t *testing.T) {
	since := metav1.NewTime(time.Date(2026, 10, 18, 10, 0, 0, 0, time.UTC))
	accepted := &metav1.Condition{
		Type:               "Accepted",
		Status:             metav1.ConditionTrue,
		Reason:             "Healthy",
		LastTransitionTime: since,
	}
	changed := accepted.DeepCopy()
	changed.Reason = "SectionMissing"
	changed.Status = metav1.ConditionFalse

	assert.NoError(t, acceptedProblem(accepted, accepted.DeepCopy()))
	assert.ErrorContains(
		t,
		acceptedProblem(nil, accepted),
		"the registration had no Accepted condition when the run started",
	)
	assert.ErrorContains(t, acceptedProblem(accepted, nil), "the registration has no Accepted condition any more")
	assert.ErrorContains(t, acceptedProblem(accepted, changed), "False, reason SectionMissing")
}

func TestLifecycleChecksReportThePrepareError(t *testing.T) {
	prepareErr := errors.New("namespace could not be created")
	s := &suite{cfg: validConfig(), life: &lifecycle{err: prepareErr}}

	for _, c := range append(lifecycleChecks, deletionChecks...) {
		err := c.run(t.Context(), s)

		assert.ErrorIs(t, err, prepareErr, "check %s", c.ID)
	}
}
