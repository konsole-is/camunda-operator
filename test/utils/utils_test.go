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
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"runtime"
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
	gone := func() bool { return stopped(child) }
	assert.Eventually(t, gone, 5*time.Second, 100*time.Millisecond, "the child of the command still runs")
}

// TestStoppedTreatsAZombieAsStopped proves that stopped tells a process that
// exited and was not reaped from one that runs.
func TestStoppedTreatsAZombieAsStopped(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("stopped reads the state of a zombie from /proc, which only Linux has")
	}

	running := exec.Command("sleep", "300")
	require.NoError(t, running.Start())
	defer func() { _ = running.Process.Kill(); _ = running.Wait() }()

	zombie := exec.Command("true")
	require.NoError(t, zombie.Start())
	defer func() { _ = zombie.Wait() }()

	assert.False(t, stopped(running.Process.Pid))
	assert.Eventually(t, func() bool { return stopped(zombie.Process.Pid) }, 5*time.Second, 50*time.Millisecond)
}

// stopped reports whether the process pid no longer runs. A killed child
// stays a zombie until its new parent reaps it, and some container inits
// never do.
func stopped(pid int) bool {
	if errors.Is(syscall.Kill(pid, 0), syscall.ESRCH) {
		return true
	}

	stat, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/stat")
	if err != nil {
		return false
	}
	// The state follows the command name, which is in parentheses.
	fields := strings.Fields(string(stat[bytes.LastIndexByte(stat, ')')+1:]))

	return len(fields) > 0 && fields[0] == "Z"
}
