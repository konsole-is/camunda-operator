//go:build !no_docker

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

package databaseserver

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/konsole-is/camunda-operator/pkg/pgbootstrap"
)

// A Docker Hub pull first gets a token from auth.docker.io, and a reset of
// that request fails every spec. Google's mirror of Docker Hub needs no token.
const postgresImage = "mirror.gcr.io/library/postgres:17"

// startArchivingPostgres runs one PostgreSQL server whose archiver reports
// every segment as archived, and returns the connection that reaches it as
// the superuser.
func startArchivingPostgres(t *testing.T) pgbootstrap.Connection {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()

	container, err := postgres.Run(
		ctx, postgresImage,
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("admin-secret"),
		testcontainers.WithCmd(
			"postgres",
			"-c", "wal_level=replica",
			"-c", "archive_mode=on",
			"-c", "archive_command=true",
		),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)

	return pgbootstrap.Connection{
		Host:     host,
		Port:     int32(port.Num()),
		User:     "postgres",
		Password: "admin-secret",
		SSLMode:  "disable",
	}
}

func TestPrimaryWALAgainstPostgres(t *testing.T) {
	conn := startArchivingPostgres(t)
	ctx := t.Context()
	wal := primaryWAL{}

	var first string
	t.Run("closing returns the name of a segment", func(t *testing.T) {
		segment, err := wal.CloseSegment(ctx, conn)

		require.NoError(t, err)
		_, ok := segmentOf(segment)
		assert.True(t, ok, "%q names no segment", segment)
		first = segment
	})

	t.Run("each close ends a later segment", func(t *testing.T) {
		segment, err := wal.CloseSegment(ctx, conn)

		require.NoError(t, err)
		assert.True(t, archivedThrough(segment, first), "%q is not after %q", segment, first)
		assert.NotEqual(t, first, segment)
	})

	t.Run("the archive reaches the closed segment", func(t *testing.T) {
		segment, err := wal.CloseSegment(ctx, conn)
		require.NoError(t, err)

		archived := func() bool {
			last, err := wal.LastArchived(ctx, conn)

			return err == nil && archivedThrough(last, segment)
		}
		assert.Eventually(t, archived, 30*time.Second, 200*time.Millisecond)
	})

	t.Run("credentials that the server rejects are an error", func(t *testing.T) {
		rejected := conn
		rejected.Password = "wrong"

		_, closeErr := wal.CloseSegment(ctx, rejected)
		_, readErr := wal.LastArchived(ctx, rejected)

		assert.Error(t, closeErr)
		assert.Error(t, readErr)
	})
}
