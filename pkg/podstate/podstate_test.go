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

package podstate_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/podstate"
)

const (
	namespace = "camunda"
	work      = "the dump Job"
)

var selector = map[string]string{"camunda.io/component": "dump"}

func podScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	return scheme
}

func pod(name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: selector},
		Status:     corev1.PodStatus{Phase: corev1.PodPending},
	}
}

func waiting(name, container, reason, message string) *corev1.Pod {
	p := pod(name)
	p.Status.ContainerStatuses = []corev1.ContainerStatus{{
		Name:  container,
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: reason, Message: message}},
	}}

	return p
}

// The kubelet retries these waiting states without end. The Job that owns the
// pod stays active and consumes no backoff, so a controller that waits on the
// Job alone never learns about them.
func TestStuckClassifiesWaitingStates(t *testing.T) {
	t.Parallel()

	initWaiting := pod("init")
	initWaiting.Status.InitContainerStatuses = []corev1.ContainerStatus{{
		Name: "dump",
		State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
			Reason: "ImagePullBackOff", Message: "postgres:99 not found",
		}},
	}}

	unschedulable := pod("unbound")
	unschedulable.Status.Conditions = []corev1.PodCondition{{
		Type: corev1.PodScheduled, Status: corev1.ConditionFalse,
		Reason: corev1.PodReasonUnschedulable, Message: "unbound immediate PersistentVolumeClaims",
	}}

	succeeded := waiting("done", "upload", "ImagePullBackOff", "stale status")
	succeeded.Status.Phase = corev1.PodSucceeded

	cases := map[string]struct {
		pod    *corev1.Pod
		reason string
		text   string
	}{
		"config error": {
			pod:    waiting("config", "upload", "CreateContainerConfigError", "secret not found"),
			reason: v1.ReasonMissingSecret,
			text:   "CreateContainerConfigError",
		},
		"container error": {
			pod:    waiting("container", "upload", "CreateContainerError", "mount failed"),
			reason: v1.ReasonMissingSecret,
			text:   "CreateContainerError",
		},
		"image pull error": {
			pod:    waiting("pull", "upload", "ErrImagePull", "manifest unknown"),
			reason: v1.ReasonInvalidReference,
			text:   "ErrImagePull",
		},
		"image pull backoff": {
			pod:    waiting("backoff", "upload", "ImagePullBackOff", "back-off pulling"),
			reason: v1.ReasonInvalidReference,
			text:   "ImagePullBackOff",
		},
		"invalid image name": {
			pod:    waiting("invalid", "upload", "InvalidImageName", "couldn't parse image"),
			reason: v1.ReasonInvalidReference,
			text:   "InvalidImageName",
		},
		"init container": {
			pod:    initWaiting,
			reason: v1.ReasonInvalidReference,
			text:   "ImagePullBackOff",
		},
		"unschedulable": {
			pod:    unschedulable,
			reason: v1.ReasonProgressing,
			text:   "unbound",
		},
		"progressing": {
			pod: waiting("fine", "upload", "PodInitializing", ""),
		},
		"terminal pod": {pod: succeeded},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := fake.NewClientBuilder().WithScheme(podScheme(t)).WithObjects(tc.pod).Build()

			failure, err := podstate.Stuck(context.Background(), c, namespace, selector, work)
			require.NoError(t, err)
			if tc.reason == "" {
				assert.Nil(t, failure)

				return
			}
			require.NotNil(t, failure)
			assert.Equal(t, tc.reason, failure.Reason)
			assert.Contains(t, failure.Message, tc.text)
			assert.Contains(t, failure.Message, tc.pod.Name)
			assert.Contains(t, failure.Message, work)
		})
	}
}

// A pod outside the selector belongs to someone else. It never holds the
// caller's work.
func TestStuckIgnoresPodsThatTheSelectorDoesNotMatch(t *testing.T) {
	t.Parallel()

	stranger := waiting("stranger", "upload", "ImagePullBackOff", "not ours")
	stranger.Labels = map[string]string{"camunda.io/component": "other"}
	c := fake.NewClientBuilder().WithScheme(podScheme(t)).WithObjects(stranger).Build()

	failure, err := podstate.Stuck(context.Background(), c, namespace, selector, work)
	require.NoError(t, err)
	assert.Nil(t, failure)
}

// A transport error is not a pre-check failure. The caller retries it.
func TestStuckReportsAListErrorAsAnError(t *testing.T) {
	t.Parallel()

	boom := errors.New("boom")
	c := fake.NewClientBuilder().WithScheme(podScheme(t)).WithInterceptorFuncs(interceptor.Funcs{
		List: func(context.Context, client.WithWatch, client.ObjectList, ...client.ListOption) error {
			return boom
		},
	}).Build()

	failure, err := podstate.Stuck(context.Background(), c, namespace, selector, work)
	require.ErrorIs(t, err, boom)
	assert.Nil(t, failure)
}
