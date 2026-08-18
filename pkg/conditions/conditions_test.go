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

package conditions

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestReadyBuildsConditionVerbatim(t *testing.T) {
	cond := Ready(metav1.ConditionFalse, v1.ReasonMissingSecret, `Secret "ns/name" not found`, 3)

	assert.Equal(t, v1.ConditionReady, cond.Type)
	assert.Equal(t, metav1.ConditionFalse, cond.Status)
	assert.Equal(t, v1.ReasonMissingSecret, cond.Reason)
	assert.Equal(t, `Secret "ns/name" not found`, cond.Message)
	assert.Equal(t, int64(3), cond.ObservedGeneration)
}

func TestReadyBoundsAnOversizedMessage(t *testing.T) {
	message := strings.Repeat("x", 100_000)

	cond := Ready(metav1.ConditionFalse, v1.ReasonFailed, message, 1)

	assert.Less(t, len(cond.Message), MaxMessageLength+100)
	assert.True(t, strings.HasPrefix(cond.Message, strings.Repeat("x", MaxMessageLength)))
	assert.True(t, strings.HasSuffix(cond.Message, "... (truncated, 100000 bytes)"), cond.Message[len(cond.Message)-40:])
}

func TestBoundMessageCutsOnARuneBoundary(t *testing.T) {
	// A message of multi-byte runes: no cut point but a rune boundary is a
	// valid string, and a truncated message must stay valid.
	message := strings.Repeat("é", MaxMessageLength)

	bounded := BoundMessage(message)

	assert.Contains(t, bounded, "(truncated, ")
	assert.True(t, strings.HasPrefix(message, strings.TrimSuffix(bounded, bounded[strings.LastIndex(bounded, "..."):])))
	assert.Less(t, len(bounded), MaxMessageLength+100)
}

func TestBoundMessageKeepsAMessageWithinTheBound(t *testing.T) {
	message := strings.Repeat("x", MaxMessageLength)

	assert.Equal(t, message, BoundMessage(message))
	assert.Equal(t, "short", BoundMessage("short"))
}

func TestStageBoundsAConditionBuiltElsewhere(t *testing.T) {
	owner := &v1.SecondaryStorageConfig{}
	owner.Generation = 4
	oversized := metav1.Condition{
		Type: v1.ConditionReady, Status: metav1.ConditionFalse, Reason: v1.ReasonFailed,
		Message: strings.Repeat("y", 3*MaxMessageLength),
	}

	Stage(owner, oversized)

	staged := (*owner.GetStatusConditions())[0]
	assert.Less(t, len(staged.Message), MaxMessageLength+100)
	assert.Contains(t, staged.Message, "(truncated, ")
	assert.Equal(t, int64(4), owner.Status.ObservedGeneration)
}
