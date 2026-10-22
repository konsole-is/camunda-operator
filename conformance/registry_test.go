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
	"fmt"
	"regexp"
	"slices"
	"strings"
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

// The report, a skip, and the docs name a check by its ID, so an ID is
// unique, has the shape of the IDs in the spec, and comes with a
// description and a run function.
func TestRegistryChecksHaveUniqueIDsAndDescriptions(t *testing.T) {
	idShape := regexp.MustCompile(`^[A-Z]+(-[A-Z]+)+$`)
	seen := map[CheckID]bool{}

	for _, c := range Checks() {
		assert.Regexp(t, idShape, string(c.ID))
		assert.False(t, seen[c.ID], "check %s is registered more than once", c.ID)
		assert.NotEmpty(t, c.Description, "check %s has no description", c.ID)
		assert.NotNil(t, c.run, "check %s has no run function", c.ID)
		seen[c.ID] = true
	}
}

func TestChecksReturnsACopyOfTheRegistry(t *testing.T) {
	saved := checks
	t.Cleanup(func() { checks = saved })
	checks = []Check{{ID: "TEST-COPY", Description: "A check."}}

	got := Checks()
	got[0].ID = "TEST-CHANGED"

	assert.Equal(t, CheckID("TEST-COPY"), checks[0].ID)
}

func TestSkipReasonOfFindsTheReasonOfSkipCheck(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		reason string
		skip   bool
	}{
		{"skip", SkipCheck("no Deployment"), "no Deployment", true},
		{"wrapped skip", fmt.Errorf("checking pods: %w", SkipCheck("no Deployment")), "no Deployment", true},
		{"failure", errors.New("the plugin broke the contract"), "", false},
		{"pass", nil, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reason, skip := skipReasonOf(tt.err)

			assert.Equal(t, tt.skip, skip)
			assert.Equal(t, tt.reason, reason)
		})
	}
}

func TestRegistryRunsTheChecksInContractOrder(t *testing.T) {
	want := []CheckID{
		"REQ-FINALIZER", "REQ-OUTPUTS-READY", "REQ-OUTPUTS", "REQ-READY", "REQ-LABELS", "REQ-RESOURCES",
		"REQ-EVENTS", "REQ-DISPLAY", "REQ-LOGS", "REQ-ACCEPTED-KEPT",
		"GRANT-MATCH", "GRANT-REQUIRES", "GRANT-ADMISSION", "GRANT-READY",
		"ORDER-REGISTRATION-FIRST", "DELETE-DELETE", "DELETE-ORPHAN", "DELETE-ROUTES",
	}
	skippable := []CheckID{"REQ-LOGS", "GRANT-ADMISSION", "GRANT-READY", "ORDER-REGISTRATION-FIRST"}

	var got []CheckID
	lastStatic := -1
	firstLifecycle := -1
	for i, c := range Checks() {
		if strings.HasPrefix(string(c.ID), "REG-") {
			lastStatic = i
			continue
		}
		if firstLifecycle == -1 {
			firstLifecycle = i
		}
		got = append(got, c.ID)
		assert.Equal(t, slices.Contains(skippable, c.ID), c.Skippable, "check %s", c.ID)
	}

	assert.Equal(t, want, got)
	assert.Less(t, lastStatic, firstLifecycle, "the lifecycle checks run after every static check")
}

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
		&pluginsv1alpha1.Grant{},
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
