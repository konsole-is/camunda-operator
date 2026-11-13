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

package logicalrestorerdbms

import (
	"context"
	"io"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcexec "github.com/testcontainers/testcontainers-go/exec"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	backup "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/pgbootstrap"
)

// A Docker Hub pull first gets a token from auth.docker.io, and a reset of
// that request fails every spec. Google's mirror of Docker Hub needs no token.
// It is the image that both Jobs render for server version 17.
const postgresImage = "mirror.gcr.io/library/postgres:17"

// The roles of the target database, as the Database controller sets them up
// through pkg/pgbootstrap: the dump connects as the backup role, and the
// restore connects as the application role that owns the database.
const (
	targetDatabase    = "camunda"
	applicationRole   = "camunda_app"
	applicationSecret = "app-credentials"
	backupRole        = "camunda_backup"
	backupSecret      = "backup-credentials"
	rolePassword      = "role-secret"
)

// olderSchema is the schema that the dump holds. It has the shape of a
// Liquibase-managed schema: a changelog table that lists the changesets that
// ran, and a table with an index and a sequence.
const olderSchema = `
CREATE TABLE databasechangelog (
	id VARCHAR(255) NOT NULL,
	filename VARCHAR(255) NOT NULL,
	orderexecuted INT NOT NULL
);
CREATE TABLE process_instance (process_instance_key BIGINT PRIMARY KEY, state VARCHAR(20));
CREATE INDEX idx_process_instance_state ON process_instance (state);
CREATE SEQUENCE process_instance_seq;
INSERT INTO process_instance VALUES (1, 'ACTIVE');
INSERT INTO databasechangelog VALUES ('create_process_instance_table', '8.9.0.xml', 1);
`

// olderChangesets are the changelog rows of olderSchema.
var olderChangesets = []string{"create_process_instance_table"}

func TestRestoreRebuildsTheObjectsOfTheDump(t *testing.T) {
	t.Parallel()

	target := startTarget(t)
	target.exec(t, olderSchema)
	dumped := target.objects(t)
	target.dump(t)

	target.exec(t, `
ALTER TABLE process_instance ADD COLUMN suspended_date TIMESTAMP;
CREATE INDEX idx_process_instance_suspended_date ON process_instance (suspended_date);
UPDATE process_instance SET state = 'COMPLETED';
INSERT INTO databasechangelog VALUES ('add_process_instance_suspended_date_column', '8.10.0.xml', 2);
`)
	code, output := target.restore(t)

	require.Zero(t, code, output)
	assert.Equal(t, dumped, target.objects(t))
	assert.Equal(t, olderChangesets, target.changesets(t))
	assert.Equal(t, "ACTIVE", target.queryString(t, "SELECT state FROM process_instance"))
}

// TestRestoreLeavesTheObjectsThatTheDumpDoesNotHold records what a restore
// leaves today. A restore must leave only the objects of the dump (#1034).
// The fix empties the list of objects that stay.
func TestRestoreLeavesTheObjectsThatTheDumpDoesNotHold(t *testing.T) {
	t.Parallel()

	target := startTarget(t)
	target.exec(t, olderSchema)
	dumped := target.objects(t)
	target.dump(t)

	target.exec(t, `
CREATE TABLE deployed_resource (resource_key BIGINT PRIMARY KEY, resource_id VARCHAR(255));
CREATE INDEX idx_deployed_resource_id ON deployed_resource (resource_id);
CREATE SEQUENCE deployed_resource_seq;
INSERT INTO deployed_resource VALUES (2, 'written after the backup');
INSERT INTO databasechangelog VALUES ('create_deployed_resource_table', '8.10.0.xml', 2);
`)
	code, output := target.restore(t)

	require.Zero(t, code, output)
	assert.Equal(t, olderChangesets, target.changesets(t))
	restored := target.objects(t)
	assert.Subset(t, restored, dumped)

	stays := slices.DeleteFunc(restored, func(o string) bool { return slices.Contains(dumped, o) })
	expected := []string{
		"column deployed_resource.resource_id",
		"column deployed_resource.resource_key",
		"index deployed_resource_pkey",
		"index idx_deployed_resource_id",
		"sequence deployed_resource_seq",
		"table deployed_resource",
	}
	assert.Equal(t, expected, stays)
	assert.Equal(t, "1", target.queryString(t, "SELECT count(*)::text FROM deployed_resource"))
}

// TestRestoreFailsWhenANewerTableReferencesATableOfTheDump records what a
// restore does today when a table that the dump does not hold has a foreign
// key into a table that it holds. Camunda's 8.10 schema has one:
// CLUSTER_VARIABLE_METADATA references CLUSTER_VARIABLE of 8.9. A restore must
// succeed and leave only the objects of the dump (#1034).
func TestRestoreFailsWhenANewerTableReferencesATableOfTheDump(t *testing.T) {
	t.Parallel()

	target := startTarget(t)
	target.exec(t, olderSchema)
	target.dump(t)

	target.exec(t, `
ALTER TABLE process_instance ADD COLUMN suspended_date TIMESTAMP;
UPDATE process_instance SET state = 'COMPLETED';
CREATE TABLE process_instance_metadata (
	process_instance_key BIGINT PRIMARY KEY REFERENCES process_instance ON DELETE CASCADE
);
INSERT INTO process_instance_metadata VALUES (1);
`)
	code, output := target.restore(t)

	assert.Equal(t, 1, code)
	assert.Contains(t, output, "cannot drop table public.process_instance because other objects depend on it")
	assert.Contains(t, target.objects(t), "column process_instance.suspended_date")
	assert.Equal(t, "COMPLETED", target.queryString(t, "SELECT state FROM process_instance"))
}

// restoreTarget is one PostgreSQL server with the target database and its
// roles, and the commands of both Jobs run against it.
type restoreTarget struct {
	container *postgres.PostgresContainer
	// app is a connection to the target database as the application role.
	app *pgx.Conn
	// secrets holds the keys of the credential Secrets that the Jobs read.
	secrets map[string]map[string]string
}

// startTarget runs one PostgreSQL server for the test and sets up the target
// database and its roles with pkg/pgbootstrap.
func startTarget(t *testing.T) *restoreTarget {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Minute)
	defer cancel()

	container, err := postgres.Run(
		ctx, postgresImage,
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("admin-secret"),
		postgres.BasicWaitStrategies(),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = testcontainers.TerminateContainer(container) })

	host, err := container.Host(ctx)
	require.NoError(t, err)
	port, err := container.MappedPort(ctx, "5432/tcp")
	require.NoError(t, err)
	admin := pgbootstrap.Connection{
		Host:     host,
		Port:     int32(port.Num()),
		User:     "postgres",
		Password: "admin-secret",
		SSLMode:  "disable",
	}

	bootstrap, err := pgbootstrap.Connect(ctx, admin)
	require.NoError(t, err)
	defer bootstrap.Close()
	require.NoError(t, bootstrap.EnsureDatabase(ctx, targetDatabase))
	require.NoError(t, bootstrap.EnsureUser(ctx, applicationRole, rolePassword))
	require.NoError(t, bootstrap.GrantApplication(ctx, applicationRole, targetDatabase))
	require.NoError(t, bootstrap.EnsureBackupUser(ctx, backupRole, rolePassword, targetDatabase))

	app := admin
	app.User = applicationRole
	app.Password = rolePassword
	conn, err := pgbootstrap.Open(ctx, app, targetDatabase)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	// The Jobs write and read the dump under the mount path of the scratch
	// volume.
	code, _, err := container.Exec(ctx, []string{"mkdir", "-p", scratchMountPath})
	require.NoError(t, err)
	require.Zero(t, code)

	return &restoreTarget{
		container: container,
		app:       conn,
		secrets: map[string]map[string]string{
			applicationSecret: {"username": applicationRole, "password": rolePassword},
			backupSecret:      {"username": backupRole, "password": rolePassword},
		},
	}
}

// exec runs sql in the target database as the application role, as Camunda's
// schema migration does.
func (r *restoreTarget) exec(t *testing.T, sql string) {
	t.Helper()

	_, err := r.app.Exec(t.Context(), sql)
	require.NoError(t, err)
}

// dump runs the dump container of the dump Job and requires it to succeed.
func (r *restoreTarget) dump(t *testing.T) {
	t.Helper()

	job, err := backup.BuildJob(backup.JobInput{
		Backup: &v1.LogicalBackupRDBMS{
			ObjectMeta: metav1.ObjectMeta{Name: "my-cluster-backup", Namespace: "my-cluster-ns", UID: "backup-uid"},
		},
		ClusterName:   "my-cluster",
		Bucket:        s3Bucket(s3Credentials()),
		DBSecretName:  backupSecret,
		DBUsernameKey: "username",
		DBPasswordKey: "password",
		ServerVersion: "17",
		Host:          "127.0.0.1",
		Port:          5432,
		Database:      targetDatabase,
		CLIImage:      "ghcr.io/konsole-is/camunda-operator-cli:0.1.0",
	})
	require.NoError(t, err)

	code, output := r.run(t, job.Spec.Template.Spec.InitContainers[0])
	require.Zero(t, code, output)
}

// restore runs the restore container of the restore Job and returns its exit
// code and its output.
func (r *restoreTarget) restore(t *testing.T) (int, string) {
	t.Helper()

	in := input()
	in.DBSecretName = applicationSecret
	in.Host = "127.0.0.1"
	in.Database = targetDatabase
	job, err := BuildJob(in)
	require.NoError(t, err)

	return r.run(t, job.Spec.Template.Spec.Containers[0])
}

// run executes the command of container inside the PostgreSQL container, with
// the environment of container and its Secret references resolved.
func (r *restoreTarget) run(t *testing.T, container corev1.Container) (int, string) {
	t.Helper()

	env := make([]string, 0, len(container.Env))
	for _, e := range container.Env {
		value := e.Value
		if e.ValueFrom != nil {
			ref := e.ValueFrom.SecretKeyRef
			value = r.secrets[ref.Name][ref.Key]
		}
		env = append(env, e.Name+"="+value)
	}

	code, reader, err := r.container.Exec(
		t.Context(), container.Command, tcexec.Multiplexed(), tcexec.WithEnv(env),
	)
	require.NoError(t, err)
	output, err := io.ReadAll(reader)
	require.NoError(t, err)

	return code, string(output)
}

// objects lists the tables, columns, indexes, and sequences of the public
// schema, sorted.
func (r *restoreTarget) objects(t *testing.T) []string {
	t.Helper()

	rows, err := r.app.Query(t.Context(), `
SELECT object FROM (
	SELECT 'table ' || tablename FROM pg_tables WHERE schemaname = 'public'
	UNION ALL
	SELECT 'column ' || table_name || '.' || column_name FROM information_schema.columns WHERE table_schema = 'public'
	UNION ALL
	SELECT 'index ' || indexname FROM pg_indexes WHERE schemaname = 'public'
	UNION ALL
	SELECT 'sequence ' || sequencename FROM pg_sequences WHERE schemaname = 'public'
) AS objects (object)
ORDER BY object COLLATE "C"`)
	require.NoError(t, err)
	objects, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)

	return objects
}

// changesets lists the ids of the changelog rows in the order they ran.
func (r *restoreTarget) changesets(t *testing.T) []string {
	t.Helper()

	rows, err := r.app.Query(t.Context(), "SELECT id FROM databasechangelog ORDER BY orderexecuted")
	require.NoError(t, err)
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	require.NoError(t, err)

	return ids
}

// queryString runs a query that returns one text value.
func (r *restoreTarget) queryString(t *testing.T, sql string) string {
	t.Helper()

	var value string
	require.NoError(t, r.app.QueryRow(t.Context(), sql).Scan(&value))

	return value
}
