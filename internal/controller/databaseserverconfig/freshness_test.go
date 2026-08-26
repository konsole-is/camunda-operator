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

package databaseserverconfig

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// TestProbeIsFresh pins when the recorded probe stands. It stands when it
// succeeded within the interval, against the server and the Secret that the
// spec names now. In every other case the controller probes again.
func TestProbeIsFresh(t *testing.T) {
	t.Parallel()

	now := time.Now()
	probed := func(age time.Duration, secretVersion string, ready metav1.ConditionStatus) *v1.DatabaseServerConfig {
		cfg := &v1.DatabaseServerConfig{}
		cfg.Generation = 3
		cfg.Status.ObservedGeneration = 3
		cfg.Spec.Host = "postgres.databases.svc"
		cfg.Spec.Port = 5432
		cfg.Spec.AdminCredentialsSecretRef.Name = "admin"
		cfg.Spec.AdminCredentialsSecretRef.UsernameKey = "username"
		cfg.Spec.AdminCredentialsSecretRef.PasswordKey = "password"
		cfg.Status.ServerVersion = "17"
		at := metav1.NewTime(now.Add(-age))
		cfg.Status.ProbedAt = &at
		cfg.Status.ProbedEndpoint = "postgres.databases.svc:5432"
		cfg.Status.ProbedSecretName = "admin"
		cfg.Status.ProbedSecretKeys = "username/password"
		cfg.Status.ProbedSecretVersion = secretVersion
		cfg.Status.Conditions = []metav1.Condition{{
			Type: v1.ConditionReady, Status: ready, Reason: v1.ReasonHealthy, ObservedGeneration: 3,
		}}

		return cfg
	}

	fresh, remaining := probeIsFresh(probed(time.Minute, "rv-1", metav1.ConditionTrue), "rv-1", now)
	assert.True(t, fresh)
	assert.InDelta(t, (probeInterval - time.Minute).Seconds(), remaining.Seconds(), 1)

	// A spec change that names the same server leaves the probe standing. Every
	// recovery request and every answer is such a change, and they are written
	// on a contract whose databases are running.
	unrelated := probed(time.Minute, "rv-1", metav1.ConditionTrue)
	unrelated.Generation = 4
	unrelated.Spec.PITR = &v1.PITRCapability{Enabled: true}
	fresh, _ = probeIsFresh(unrelated, "rv-1", now)
	assert.True(t, fresh, "a spec change that names the same server")

	movedHost := probed(time.Minute, "rv-1", metav1.ConditionTrue)
	movedHost.Spec.Host = "postgres-r1.databases.svc"

	movedPort := probed(time.Minute, "rv-1", metav1.ConditionTrue)
	movedPort.Spec.Port = 5433

	movedSecret := probed(time.Minute, "rv-1", metav1.ConditionTrue)
	movedSecret.Spec.AdminCredentialsSecretRef.Name = "admin-copy"

	// One Secret can hold the credentials of more than one user.
	movedKeys := probed(time.Minute, "rv-1", metav1.ConditionTrue)
	movedKeys.Spec.AdminCredentialsSecretRef.UsernameKey = "readonly-username"

	cases := map[string]*v1.DatabaseServerConfig{
		"never probed":       {},
		"stale":              probed(probeInterval, "rv-1", metav1.ConditionTrue),
		"host moved":         movedHost,
		"port moved":         movedPort,
		"admin secret moved": movedSecret,
		"admin keys moved":   movedKeys,
		"secret changed":     probed(time.Minute, "rv-0", metav1.ConditionTrue),
		"last probe failed":  probed(time.Minute, "rv-1", metav1.ConditionFalse),
	}
	for name, cfg := range cases {
		fresh, _ := probeIsFresh(cfg, "rv-1", now)
		assert.False(t, fresh, name)
	}
}
