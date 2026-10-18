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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	authorizationv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestNewSuiteFillsTheTimeoutDefaultsAndKeepsTheConnection(t *testing.T) {
	restCfg := &rest.Config{Host: "https://cluster.example"}
	c := fake.NewClientBuilder().Build()

	s := newSuite(validConfig(), restCfg, c)

	assert.Equal(t, 10*time.Minute, s.cfg.Timeouts.Ready)
	assert.Same(t, restCfg, s.restConfig)
	assert.Same(t, c, s.client)
}

// The checks read and write the plugin kinds, the core kinds, and the
// Kubernetes kinds that a plugin creates.
func TestNewSchemeKnowsTheKindsOfTheChecks(t *testing.T) {
	scheme, err := newScheme()
	require.NoError(t, err)

	for _, obj := range []runtime.Object{
		&pluginsv1alpha1.PluginRegistration{},
		&pluginsv1alpha1.PluginSection{},
		&pluginsv1alpha1.PluginRequest{},
		&v1.CamundaCluster{},
		&corev1.ConfigMap{},
		&appsv1.Deployment{},
		&authorizationv1.SubjectAccessReview{},
	} {
		_, _, err := scheme.ObjectKinds(obj)
		assert.NoError(t, err, "%T", obj)
	}
}

// Port 1 on the loopback address refuses each connection.
func TestConnectFailsWhenTheClusterDoesNotAnswer(t *testing.T) {
	_, _, err := connect(t.Context(), &rest.Config{Host: "https://127.0.0.1:1"})

	assert.ErrorContains(t, err, "127.0.0.1:1")
}
