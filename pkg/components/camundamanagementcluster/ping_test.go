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

package camundamanagementcluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// A cluster before 8.10 reads the ping under camunda.console.ping.
func TestPingEnvOfACamunda89Cluster(t *testing.T) {
	t.Parallel()

	env := PingEnv("http://my-management-console.camunda.svc:80", "production", "8.9.9")

	assert.Equal(
		t, []corev1.EnvVar{
			{Name: "CAMUNDA_CONSOLE_PING_ENABLED", Value: "true"},
			{Name: "CAMUNDA_CONSOLE_PING_ENDPOINT", Value: "http://my-management-console.camunda.svc:80"},
			{Name: "CAMUNDA_CONSOLE_PING_CLUSTERNAME", Value: "production"},
			{Name: "CAMUNDA_CONSOLE_PING_PINGPERIOD", Value: "1h"},
		}, env,
	)
}

// Camunda 8.10 renamed Console to Hub, and a cluster of that version reads the
// ping under camunda.hub.ping.
func TestPingEnvOfACamunda810Cluster(t *testing.T) {
	t.Parallel()

	for _, version := range []string{"8.10.0", "8.10.3", "9.0.0"} {
		t.Run(version, func(t *testing.T) {
			t.Parallel()

			env := PingEnv("http://console.camunda.svc:80", "production", version)

			assert.Equal(
				t, []corev1.EnvVar{
					{Name: "CAMUNDA_HUB_PING_ENABLED", Value: "true"},
					{Name: "CAMUNDA_HUB_PING_ENDPOINT", Value: "http://console.camunda.svc:80"},
					{Name: "CAMUNDA_HUB_PING_CLUSTERNAME", Value: "production"},
					{Name: "CAMUNDA_HUB_PING_PINGPERIOD", Value: "1h"},
				}, env,
			)
		})
	}
}

// A cluster that publishes no version yet reads the names that every supported
// cluster understands.
func TestPingEnvOfAClusterWithoutAVersion(t *testing.T) {
	t.Parallel()

	env := PingEnv("http://console.camunda.svc:80", "production", "")

	assert.Equal(t, "CAMUNDA_CONSOLE_PING_ENABLED", env[0].Name)
}

// The endpoint tells the entries of one management plane from those of
// another, under either key set.
func TestPingsConsoleReadsTheEndpoint(t *testing.T) {
	t.Parallel()

	ours := "http://my-management-console.camunda.svc:80"

	assert.True(t, PingsConsole(PingEnv(ours, "production", "8.9.9"), ours))
	assert.True(t, PingsConsole(PingEnv(ours, "production", "8.10.0"), ours))
	assert.False(t, PingsConsole(PingEnv("http://other-console.other.svc:80", "production", "8.9.9"), ours))
	assert.False(t, PingsConsole(nil, ours))
	assert.False(t, PingsConsole([]corev1.EnvVar{{Name: "ZEEBE_BROKER_CLUSTER_SIZE", Value: "3"}}, ours))
}

// The management plane owns the ping names, so an entry under one of them
// that carries valueFrom collides with the value it writes. Every other entry
// is left alone.
func TestPingCollisionsReadsTheEntriesUnderAPingName(t *testing.T) {
	t.Parallel()

	fromConfigMap := &corev1.EnvVarSource{
		ConfigMapKeyRef: &corev1.ConfigMapKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "console"},
			Key:                  "endpoint",
		},
	}
	extraEnv := []corev1.EnvVar{
		{Name: "USER_SETTING", ValueFrom: fromConfigMap},
		{Name: "CAMUNDA_CONSOLE_PING_ENABLED", Value: "false"},
		{Name: "CAMUNDA_CONSOLE_PING_ENDPOINT", ValueFrom: fromConfigMap},
		{Name: "CAMUNDA_CONSOLE_PING_PINGPERIOD", ValueFrom: fromConfigMap},
		{Name: "CAMUNDA_HUB_PING_CLUSTERNAME", ValueFrom: fromConfigMap},
	}
	cluster := &v1.CamundaCluster{Spec: v1.CamundaClusterSpec{ExtraEnv: extraEnv}}

	// The last entry comes first, so a caller that removes them in this order
	// keeps the index of the entries it has not reached yet. The Hub entry is
	// not a setting an 8.9 cluster reads, so it stays.
	assert.Equal(
		t, []PingCollision{
			{Index: 3, Name: "CAMUNDA_CONSOLE_PING_PINGPERIOD"},
			{Index: 2, Name: "CAMUNDA_CONSOLE_PING_ENDPOINT"},
		}, PingCollisions(cluster, "8.9.9"),
	)

	// An 8.10 cluster reads the Hub key set, and the Console entries stay.
	assert.Equal(
		t, []PingCollision{{Index: 4, Name: "CAMUNDA_HUB_PING_CLUSTERNAME"}}, PingCollisions(cluster, "8.10.0"),
	)
}

func TestPingCollisionsOfAClusterWithoutOne(t *testing.T) {
	t.Parallel()

	cluster := &v1.CamundaCluster{Spec: v1.CamundaClusterSpec{ExtraEnv: PingEnv(
		"http://console.camunda.svc:80", "production", "8.9.9",
	)}}

	assert.Empty(t, PingCollisions(cluster, "8.9.9"))
	assert.Empty(t, PingCollisions(&v1.CamundaCluster{}, "8.9.9"))
}

// The entry is gone once the management plane replaces it, so the field
// manager that wrote it is what tells the user where it came from.
func TestPingCollisionsNameTheFieldManagerOfValueFrom(t *testing.T) {
	t.Parallel()

	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{ManagedFields: []metav1.ManagedFieldsEntry{
			{
				Manager: "camunda-operator/camundamanagementcluster-ping",
				FieldsV1: metav1.NewFieldsV1(
					`{"f:spec":{"f:extraEnv":{"k:{\"name\":\"OTHER\"}":{"f:value":{}}}}}`,
				),
			},
			{
				Manager:     "a-person",
				Subresource: "status",
				FieldsV1: metav1.NewFieldsV1(
					`{"f:spec":{"f:extraEnv":{"k:{\"name\":\"CAMUNDA_CONSOLE_PING_ENDPOINT\"}":{"f:valueFrom":{}}}}}`,
				),
			},
			{
				Manager: "kubectl-client-side-apply",
				FieldsV1: metav1.NewFieldsV1(
					`{"f:spec":{"f:extraEnv":{"k:{\"name\":\"CAMUNDA_CONSOLE_PING_ENDPOINT\"}":{"f:valueFrom":{}}}}}`,
				),
			},
		}},
		Spec: v1.CamundaClusterSpec{ExtraEnv: []corev1.EnvVar{{
			Name:      "CAMUNDA_CONSOLE_PING_ENDPOINT",
			ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}},
		}}},
	}

	collisions := PingCollisions(cluster, "8.9.9")

	// The entry of a subresource says nothing about spec.extraEnv, and the
	// manager that owns another name is not the one that wrote this entry.
	assert.Equal(
		t, []PingCollision{
			{Index: 0, Name: "CAMUNDA_CONSOLE_PING_ENDPOINT", Manager: "kubectl-client-side-apply"},
		}, collisions,
	)
}

// A cluster whose managedFields no writer of the entry claims still reports
// the collision, so the entry is removed either way.
func TestPingCollisionsWithoutAFieldManager(t *testing.T) {
	t.Parallel()

	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{ManagedFields: []metav1.ManagedFieldsEntry{
			{Manager: "kubectl", FieldsV1: metav1.NewFieldsV1(`{"f:spec":{"f:version":{}}}`)},
			{Manager: "broken", FieldsV1: metav1.NewFieldsV1("not json")},
			{Manager: "empty"},
		}},
		Spec: v1.CamundaClusterSpec{ExtraEnv: []corev1.EnvVar{{
			Name:      "CAMUNDA_HUB_PING_PINGPERIOD",
			ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}},
		}}},
	}

	assert.Equal(
		t,
		[]PingCollision{{Index: 0, Name: "CAMUNDA_HUB_PING_PINGPERIOD"}},
		PingCollisions(cluster, "8.10.0"),
	)
}
