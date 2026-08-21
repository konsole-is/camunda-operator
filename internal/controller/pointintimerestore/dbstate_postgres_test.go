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

package pointintimerestore

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/konsole-is/camunda-operator/pkg/pgbootstrap"
)

// exporterPositionDDL creates the table the way Camunda creates it. Liquibase
// runs the changeset with no quoting strategy, so PostgreSQL folds every name
// to lower case, and the column types are what Liquibase maps NUMBER and
// DATETIME to: numeric, and a timestamp that carries no zone.
const exporterPositionDDL = `
CREATE TABLE EXPORTER_POSITION (
	PARTITION_ID NUMERIC PRIMARY KEY,
	EXPORTER VARCHAR(200),
	LAST_EXPORTED_POSITION BIGINT,
	CREATED TIMESTAMP WITHOUT TIME ZONE,
	LAST_UPDATED TIMESTAMP WITHOUT TIME ZONE
)`

// startPostgres runs one PostgreSQL server for the test and returns the
// connection that reaches it. The reader of the exporter position is the one
// part of this controller that no fake can cover: the table name, the column
// names, the scan, and the answer of a database that carries no Camunda schema
// all live in the server.
func startPostgres(t *testing.T) pgbootstrap.Connection {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()

	container, err := postgres.Run(
		ctx, "postgres:17",
		postgres.WithDatabase("camunda"),
		postgres.WithUsername("camunda"),
		postgres.WithPassword("app-secret"),
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
		User:     "camunda",
		Password: "app-secret",
		SSLMode:  "disable",
	}
}

func TestReadPositionsAgainstPostgres(t *testing.T) {
	conn := startPostgres(t)
	ctx := t.Context()

	t.Run("a database with no Camunda schema reports no exporter table", func(t *testing.T) {
		positions, err := readPositions(ctx, conn, "camunda")

		assert.Nil(t, positions)
		require.Error(t, err)
		assert.True(
			t, errors.Is(err, errNoExporterTable),
			"an empty database is the state the check exists for, got %v", err,
		)
	})

	db, err := pgbootstrap.Open(ctx, conn, "camunda")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close(context.Background()) })

	_, err = db.Exec(ctx, exporterPositionDDL)
	require.NoError(t, err)

	t.Run("an empty table reports no position", func(t *testing.T) {
		positions, err := readPositions(ctx, conn, "camunda")

		require.NoError(t, err)
		assert.Empty(t, positions)
	})

	t.Run("every partition reports its last update, in partition order", func(t *testing.T) {
		// The exporter writes the wall clock of the broker into a column that
		// carries no zone, so the values go in and come back as they are.
		first := time.Date(2026, 7, 30, 14, 30, 0, 0, time.UTC)
		second := time.Date(2026, 7, 30, 14, 31, 15, 0, time.UTC)

		_, err := db.Exec(
			ctx, `
			INSERT INTO exporter_position
				(partition_id, exporter, last_exported_position, created, last_updated)
			VALUES (2, 'RdbmsExporter', 4711, $1, $2), (1, 'RdbmsExporter', 4242, $1, $1)`,
			first, second,
		)
		require.NoError(t, err)

		positions, err := readPositions(ctx, conn, "camunda")

		require.NoError(t, err)
		require.Len(t, positions, 2)
		assert.Equal(t, int32(1), positions[0].PartitionID)
		assert.Equal(t, first.UTC(), positions[0].LastUpdated.UTC())
		assert.Equal(t, int32(2), positions[1].PartitionID)
		assert.Equal(t, second.UTC(), positions[1].LastUpdated.UTC())
	})

	t.Run("credentials that the server rejects are not a missing table", func(t *testing.T) {
		rejected := conn
		rejected.Password = "wrong"

		positions, err := readPositions(ctx, rejected, "camunda")

		assert.Nil(t, positions)
		require.Error(t, err)
		assert.False(t, errors.Is(err, errNoExporterTable))
	})
}
