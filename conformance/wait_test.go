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
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestWaitForReturnsTheLastProblemOnTimeout(t *testing.T) {
	err := waitFor(t.Context(), time.Second, "OutputsReady=True", func(context.Context) error {
		return errors.New("OutputsReady is False")
	})

	assert.ErrorContains(t, err, "OutputsReady=True within 1s: OutputsReady is False")
}

func TestWaitForStopsWhenTheCheckPasses(t *testing.T) {
	calls := 0
	err := waitFor(t.Context(), time.Minute, "a pass", func(context.Context) error {
		calls++
		if calls < 2 {
			return errors.New("not yet")
		}
		return nil
	})

	assert.NoError(t, err)
	assert.Equal(t, 2, calls)
}

func TestWaitForReturnsAtOnceWhenTheContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	start := time.Now()
	err := waitFor(ctx, time.Minute, "a pass", func(context.Context) error {
		return errors.New("not yet")
	})

	assert.ErrorIs(t, err, context.Canceled)
	assert.Less(t, time.Since(start), 10*time.Second)
}

func TestWaitForGivesTheCheckTheContextOfTheCaller(t *testing.T) {
	err := waitFor(t.Context(), time.Second, "a pass", func(ctx context.Context) error {
		_, hasDeadline := ctx.Deadline()
		assert.False(t, hasDeadline, "a call that the timeout cuts short hides the last problem")
		return nil
	})

	assert.NoError(t, err)
}
