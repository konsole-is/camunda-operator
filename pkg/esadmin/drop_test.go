package esadmin_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/konsole-is/camunda-operator/pkg/esadmin"
	"github.com/konsole-is/camunda-operator/pkg/esadmin/esadmintest"
)

// A dropped connection is an unreachable endpoint, not a rejection. One
// operation can be dropped while another stays served, the way a broken
// proxy behaves.
func TestDroppedCallsAreUnreachableWhileOthersAreServed(t *testing.T) {
	ctx := context.Background()
	server := esadmintest.New()
	t.Cleanup(server.Close)
	client, err := esadmin.New(server.URL(), "camunda", "secret", nil)
	require.NoError(t, err)
	require.NoError(t, client.EnsureSnapshotRepository(ctx, "repo", esadmin.S3RepositoryConfig{Bucket: "b"}))

	server.DropNext("snapshotCreate", 1)
	err = client.CreateSnapshot(ctx, "repo", "snap", []string{"idx"})
	require.ErrorIs(t, err, esadmin.ErrUnreachable)

	_, err = client.SnapshotStatus(ctx, "repo", "snap")
	require.NoError(t, err, "the status query is served while creates are dropped")
	assert.NoError(t, client.CreateSnapshot(ctx, "repo", "snap", []string{"idx"}))
}
