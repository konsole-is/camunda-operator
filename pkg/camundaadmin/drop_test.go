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

package camundaadmin_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
)

// A dropped connection is an unreachable endpoint, not a rejection. One
// operation can be dropped while another stays served, the way a broken
// proxy behaves.
func TestDroppedCallsAreUnreachableWhileOthersAreServed(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)
	server.SetRuntimeState(7, "COMPLETED", "")

	server.DropNext("runtimeStatus", 1)
	_, err := client.RuntimeBackupStatus(ctx, 7)
	require.ErrorIs(t, err, camundaadmin.ErrUnreachable)

	require.NoError(t, client.ResumeExporting(ctx), "exporting is served while status queries are dropped")

	status, err := client.RuntimeBackupStatus(ctx, 7)
	require.NoError(t, err, "one drop, then reachable again")
	assert.Equal(t, camundaadmin.StateCompleted, status.State)
}
