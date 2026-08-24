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
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// TestProbeEndsWithinItsDeadline pins that the probe as a whole is bounded.
// The server accepts the connection and never answers. connect_timeout does
// not end this test on its own: the deadline of the probe does, well before
// the 5 seconds that connect_timeout allows.
func TestProbeEndsWithinItsDeadline(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			// Hold the connection open and say nothing until the test ends.
			t.Cleanup(func() { _ = conn.Close() })
		}
	}()

	addr := listener.Addr().(*net.TCPAddr)
	cfg := &v1.DatabaseServerConfig{Spec: v1.DatabaseServerConfigSpec{
		Engine: v1.DatabaseEnginePostgres, Host: addr.IP.String(), Port: int32(addr.Port),
	}}

	started := time.Now()
	_, _, err = probeWithin(context.Background(), 300*time.Millisecond, cfg, "admin", "secret")
	require.Error(t, err)
	assert.Less(t, time.Since(started), 3*time.Second, "the deadline of the probe ends it, not connect_timeout")
}
