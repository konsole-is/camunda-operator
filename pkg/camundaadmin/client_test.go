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
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin/camundaadmintest"
)

// newClient builds a client against a fresh fake.
func newClient(t *testing.T) (*camundaadmin.Client, *camundaadmintest.Server) {
	t.Helper()

	server := camundaadmintest.New()
	t.Cleanup(server.Close)

	client, err := camundaadmin.New(camundaadmin.Binding{Endpoint: server.URL(), Version: "8.9.9"})
	require.NoError(t, err)

	return client, server
}

func TestNewRejectsUnknownVersions(t *testing.T) {
	tests := []struct {
		version string
		wantErr bool
	}{
		{"8.9.9", false},
		{"8.9", false},
		{"8.10.0", true},
		{"8.8.3", true},
		{"", true},
	}

	for _, tt := range tests {
		t.Run("version "+tt.version, func(t *testing.T) {
			_, err := camundaadmin.New(camundaadmin.Binding{Endpoint: "http://cluster:9600", Version: tt.version})
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.version)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNewRejectsEmptyEndpoint(t *testing.T) {
	_, err := camundaadmin.New(camundaadmin.Binding{Version: "8.9.9"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "endpoint")
}

func TestExportingPauseAndResume(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	require.NoError(t, client.PauseExporting(ctx, true))
	assert.Equal(t, "softPaused", server.Exporting())

	// Pausing again is success: the server-side operation is idempotent.
	require.NoError(t, client.PauseExporting(ctx, true))
	assert.Equal(t, 2, server.PauseCalls())

	require.NoError(t, client.ResumeExporting(ctx))
	assert.Equal(t, "running", server.Exporting())
}

func TestExportingErrorsAreRejected(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	server.FailNext("resume", 1)
	err := client.ResumeExporting(ctx)
	require.ErrorIs(t, err, camundaadmin.ErrRejected)
	assert.Contains(t, err.Error(), "injected resume failure")
}

func TestUnreachableEndpoint(t *testing.T) {
	ctx := context.Background()

	client, err := camundaadmin.New(camundaadmin.Binding{
		Endpoint: "http://127.0.0.1:1",
		Version:  "8.9.9",
	})
	require.NoError(t, err)

	err = client.PauseExporting(ctx, true)
	require.ErrorIs(t, err, camundaadmin.ErrUnreachable)
}

func TestStartHistoryBackupIsIdempotent(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	require.NoError(t, client.StartHistoryBackup(ctx, 42))

	// A second start of the same id hits the duplicate rejection of the
	// endpoint, and the client resolves it through the status: success.
	require.NoError(t, client.StartHistoryBackup(ctx, 42))
	assert.Equal(t, 1, server.HistoryStarts(42))
}

func TestHistoryBackupStatus(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	status, err := client.HistoryBackupStatus(ctx, 42)
	require.NoError(t, err)
	assert.Equal(t, camundaadmin.StateDoesNotExist, status.State)

	require.NoError(t, client.StartHistoryBackup(ctx, 42))
	server.SetHistoryState(42, "COMPLETED", "")

	status, err = client.HistoryBackupStatus(ctx, 42)
	require.NoError(t, err)
	assert.Equal(t, camundaadmin.StateCompleted, status.State)
	require.NotEmpty(t, status.Details)
	assert.Equal(t, "COMPLETED", status.Details[0].State)
}

func TestStartRuntimeBackupWithExplicitID(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	id := int64(1700000000)
	got, err := client.StartRuntimeBackup(ctx, &id)
	require.NoError(t, err)
	assert.Equal(t, id, got)

	// Re-entry: the endpoint answers 409, the client resolves it through
	// the status and reports the same id.
	got, err = client.StartRuntimeBackup(ctx, &id)
	require.NoError(t, err)
	assert.Equal(t, id, got)
	assert.Equal(t, 1, server.RuntimeStarts(id))
}

func TestStartRuntimeBackupConflictWithHigherIDFails(t *testing.T) {
	ctx := context.Background()
	client, _ := newClient(t)

	high := int64(2000)
	_, err := client.StartRuntimeBackup(ctx, &high)
	require.NoError(t, err)

	low := int64(1000)
	_, err = client.StartRuntimeBackup(ctx, &low)
	require.ErrorIs(t, err, camundaadmin.ErrRejected)
}

func TestStartRuntimeBackupGeneratesID(t *testing.T) {
	ctx := context.Background()
	client, _ := newClient(t)

	first, err := client.StartRuntimeBackup(ctx, nil)
	require.NoError(t, err)
	second, err := client.StartRuntimeBackup(ctx, nil)
	require.NoError(t, err)

	assert.Greater(t, second, first)
}

func TestRuntimeBackupStatus(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	id := int64(7)
	_, err := client.StartRuntimeBackup(ctx, &id)
	require.NoError(t, err)

	server.SetRuntimeState(id, "FAILED", "partition 1 lost quorum")

	status, err := client.RuntimeBackupStatus(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, camundaadmin.StateFailed, status.State)
	assert.Equal(t, "partition 1 lost quorum", status.FailureReason)
	require.NotEmpty(t, status.Details)
	assert.Equal(t, "1", status.Details[0].Name)
}

func TestDeleteRuntimeBackupIsIdempotent(t *testing.T) {
	ctx := context.Background()
	client, server := newClient(t)

	id := int64(9)
	_, err := client.StartRuntimeBackup(ctx, &id)
	require.NoError(t, err)

	require.NoError(t, client.DeleteRuntimeBackup(ctx, id))
	assert.Nil(t, server.RuntimeBackup(id))

	// Deleting an absent backup is success for the re-entrant finalizer.
	assert.NoError(t, client.DeleteRuntimeBackup(ctx, id))
}
