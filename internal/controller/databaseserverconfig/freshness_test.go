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
// succeeded within the interval for the current generation and Secret. In
// every other case the controller probes again.
func TestProbeIsFresh(t *testing.T) {
	t.Parallel()

	now := time.Now()
	probed := func(age time.Duration, generation, observed int64, secretVersion string, ready metav1.ConditionStatus) *v1.DatabaseServerConfig {
		cfg := &v1.DatabaseServerConfig{}
		cfg.Generation = generation
		cfg.Status.ObservedGeneration = observed
		cfg.Status.ServerVersion = "17"
		at := metav1.NewTime(now.Add(-age))
		cfg.Status.ProbedAt = &at
		cfg.Status.ProbedSecretVersion = secretVersion
		cfg.Status.Conditions = []metav1.Condition{{
			Type: v1.ConditionReady, Status: ready, Reason: v1.ReasonHealthy, ObservedGeneration: observed,
		}}

		return cfg
	}

	fresh, remaining := probeIsFresh(probed(time.Minute, 3, 3, "rv-1", metav1.ConditionTrue), "rv-1", now)
	assert.True(t, fresh)
	assert.InDelta(t, (probeInterval - time.Minute).Seconds(), remaining.Seconds(), 1)

	cases := map[string]*v1.DatabaseServerConfig{
		"never probed":      {},
		"stale":             probed(probeInterval, 3, 3, "rv-1", metav1.ConditionTrue),
		"spec changed":      probed(time.Minute, 4, 3, "rv-1", metav1.ConditionTrue),
		"secret changed":    probed(time.Minute, 3, 3, "rv-0", metav1.ConditionTrue),
		"last probe failed": probed(time.Minute, 3, 3, "rv-1", metav1.ConditionFalse),
	}
	for name, cfg := range cases {
		fresh, _ := probeIsFresh(cfg, "rv-1", now)
		assert.False(t, fresh, name)
	}
}
