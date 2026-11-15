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
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/konsole-is/camunda-operator/pkg/pgbootstrap"
)

// A Docker Hub pull first gets a token from auth.docker.io, and a reset of
// that request fails every spec. Google's mirror of Docker Hub needs no token.
const postgresImage = "mirror.gcr.io/library/postgres:17"

// startArchivingPostgres runs one PostgreSQL server that archives each
// segment into /archive of its container, and returns the container and the
// connection that reaches it as the superuser.
func startArchivingPostgres(t *testing.T) (testcontainers.Container, pgbootstrap.Connection) {
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
			"-c", "archive_command=cp %p /archive/%f",
			"-c", "autovacuum=off",
		),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })

	// The archiver retries a segment until its command succeeds, so the
	// directory can come after the server.
	inContainer(t, container, "sh", "-c", "mkdir -p /archive /work && chown postgres /archive /work")

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)

	return container, pgbootstrap.Connection{
		Host:     host,
		Port:     int32(port.Num()),
		User:     "postgres",
		Password: "admin-secret",
		SSLMode:  "disable",
	}
}

// inContainer runs command in container as root, fails the test when it
// exits with an error, and returns its output.
func inContainer(t *testing.T, container testcontainers.Container, command ...string) string {
	t.Helper()

	code, reader, err := container.Exec(t.Context(), command, tcexec.Multiplexed())
	require.NoError(t, err)
	output, err := io.ReadAll(reader)
	require.NoError(t, err)
	require.Zero(t, code, "%v: %s", command, output)

	return string(output)
}

func TestPrimaryWALAgainstPostgres(t *testing.T) {
	_, conn := startArchivingPostgres(t)
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

// A recovery to a time stops at the first commit after that time. A server
// that commits nothing after the target ends its recovery short of it, and
// PostgreSQL fails the recovery. The commit of CloseSegment is that commit.
func TestCloseSegmentLetsARecoveryReachAnIdleTarget(t *testing.T) {
	container, conn := startArchivingPostgres(t)
	ctx := t.Context()
	wal := primaryWAL{}

	db, err := pgbootstrap.Open(ctx, conn, maintenanceDatabase)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close(context.Background()) })

	inContainer(
		t, container,
		"gosu", "postgres", "pg_basebackup", "-h", "/var/run/postgresql", "-D", "/work/backup", "-X", "stream",
	)
	_, err = db.Exec(ctx, "CREATE TABLE rows (id int)")
	require.NoError(t, err)

	// commitThenIdle commits one row and returns a target after that commit,
	// with nothing committed between the two.
	commitThenIdle := func(id int) time.Time {
		_, err := db.Exec(ctx, "INSERT INTO rows VALUES ($1)", id)
		require.NoError(t, err)

		var target time.Time
		require.NoError(t, db.QueryRow(ctx, "SELECT clock_timestamp()").Scan(&target))
		time.Sleep(time.Second)

		return target
	}

	// archived waits until the archive holds segment.
	archived := func(segment string) {
		holds := func() bool {
			last, err := wal.LastArchived(ctx, conn)

			return err == nil && archivedThrough(last, segment)
		}
		require.Eventually(t, holds, 30*time.Second, 200*time.Millisecond)
	}

	t.Run("without a commit after the target, the recovery fails", func(t *testing.T) {
		target := commitThenIdle(1)

		var segment string
		require.NoError(t, db.QueryRow(ctx, "SELECT pg_walfile_name(pg_switch_wal())").Scan(&segment))
		archived(segment)

		recoverTo(t, container, "idle", 5433, target)

		const fatal = "recovery ended before configured recovery target was reached"
		failed := func() bool {
			return strings.Contains(inContainer(t, container, "cat", "/work/idle.log"), fatal)
		}
		require.Eventually(t, failed, time.Minute, time.Second)
	})

	t.Run("with the commit of CloseSegment, the recovery reaches the target", func(t *testing.T) {
		target := commitThenIdle(2)

		segment, err := wal.CloseSegment(ctx, conn)
		require.NoError(t, err)
		archived(segment)

		recoverTo(t, container, "closed", 5434, target)

		query := []string{
			"gosu", "postgres", "psql", "-h", "/var/run/postgresql", "-p", "5434", "-At",
			"-c", "SELECT pg_is_in_recovery()::text || ' ' || count(*) FROM rows",
		}
		promotedWithBothRows := func() bool {
			code, reader, err := container.Exec(ctx, query, tcexec.Multiplexed())
			if err != nil || code != 0 {
				return false
			}
			output, err := io.ReadAll(reader)

			return err == nil && strings.TrimSpace(string(output)) == "false 2"
		}
		require.Eventually(t, promotedWithBothRows, time.Minute, time.Second)
	})
}

// recoverTo starts a second server in container, on port, that recovers the
// base backup of the test from the archive to target and then promotes. Its
// data directory is /work/<name> and its log /work/<name>.log.
func recoverTo(t *testing.T, container testcontainers.Container, name string, port int, target time.Time) {
	t.Helper()

	dir := "/work/" + name
	options := fmt.Sprintf(
		"-p %d -c archive_mode=off -c restore_command='cp /archive/%%f %%p' "+
			"-c recovery_target_time='%s' -c recovery_target_action=promote",
		port, target.UTC().Format("2006-01-02 15:04:05.999999+00"),
	)
	inContainer(
		t, container, "gosu", "postgres", "sh", "-c",
		fmt.Sprintf(
			"cp -a /work/backup %[1]s && touch %[1]s/recovery.signal && "+
				"pg_ctl -D %[1]s -l %[1]s.log -W -o \"%[2]s\" start",
			dir, options,
		),
	)
}
