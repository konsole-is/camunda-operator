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

package utils

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRunContextEndsWithItsContext proves that a command whose child holds
// the output pipe returns when its context ends, and that the child is killed
// with it. The teardown of the e2e suite runs make, and make runs kubectl,
// which can wait without end.
func TestRunContextEndsWithItsContext(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	start := time.Now()
	out, err := RunContext(ctx, "sh", "-c", "sleep 300 & echo $!; wait")

	assert.Error(t, err)
	assert.Less(t, time.Since(start), 5*time.Second)

	child, convErr := strconv.Atoi(strings.TrimSpace(out))
	require.NoError(t, convErr, out)
	gone := func() bool { return errors.Is(syscall.Kill(child, 0), syscall.ESRCH) }
	assert.Eventually(t, gone, 5*time.Second, 100*time.Millisecond, "the child of the command still runs")
}
