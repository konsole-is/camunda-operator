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

package pgbootstrap

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// adminConn is the shared connection target of the postgres:17 container
// started once per test binary by TestMain.
var adminConn Connection

func TestMain(m *testing.M) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	container, err := postgres.Run(ctx, "postgres:17",
		postgres.WithDatabase("postgres"),
		postgres.WithUsername("postgres"),
		postgres.WithPassword("admin-secret"),
		postgres.BasicWaitStrategies(),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "starting postgres container: %v\n", err)
		os.Exit(1)
	}

	host, err := container.Host(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolving container host: %v\n", err)
		os.Exit(1)
	}
	port, err := container.MappedPort(ctx, "5432/tcp")
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolving container port: %v\n", err)
		os.Exit(1)
	}

	adminConn = Connection{
		Host:          host,
		Port:          int32(port.Num()),
		AdminUser:     "postgres",
		AdminPassword: "admin-secret",
		SSLMode:       "disable",
	}

	code := m.Run()

	_ = testcontainers.TerminateContainer(container)
	os.Exit(code)
}

// connect returns a Bootstrapper against the shared container, closed on test
// cleanup.
func connect(t *testing.T) Bootstrapper {
	t.Helper()

	b, err := Connect(t.Context(), adminConn)
	require.NoError(t, err)
	t.Cleanup(b.Close)

	return b
}

// connectAs opens a direct connection to database as user, or returns the
// connection error.
func connectAs(ctx context.Context, user, password, database string) (*pgx.Conn, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable&connect_timeout=5",
		user, password, adminConn.Host, adminConn.Port, database)

	return pgx.Connect(ctx, url)
}

// mustConnectAs opens a direct connection to database as user, closed on test
// cleanup.
func mustConnectAs(t *testing.T, user, password, database string) *pgx.Conn {
	t.Helper()

	conn, err := connectAs(t.Context(), user, password, database)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close(context.Background()) })

	return conn
}

func TestConnectRejectsBadCredentials(t *testing.T) {
	bad := adminConn
	bad.AdminPassword = "wrong"

	b, err := Connect(t.Context(), bad)
	if err == nil {
		defer b.Close()
		err = b.Ping(t.Context())
	}
	require.Error(t, err)
}

func TestConnectRejectsUnreachableServer(t *testing.T) {
	bad := adminConn
	bad.Port = 1

	b, err := Connect(t.Context(), bad)
	if err == nil {
		defer b.Close()
		err = b.Ping(t.Context())
	}
	require.Error(t, err)
}

func TestPing(t *testing.T) {
	b := connect(t)
	assert.NoError(t, b.Ping(t.Context()))
}

func TestEnsureDatabaseRejectsInvalidIdentifier(t *testing.T) {
	b := connect(t)
	ctx := t.Context()

	assert.ErrorContains(t, b.EnsureDatabase(ctx, "Bad-Name"), "identifier")
	assert.ErrorContains(t, b.EnsureUser(ctx, `x";drop role y;--`, "pw"), "identifier")
	assert.ErrorContains(t, b.GrantApplication(ctx, "user", "Bad-Name"), "identifier")
	assert.ErrorContains(t, b.EnsureBackupUser(ctx, "Bad-Name", "pw", "db"), "identifier")
}

func TestBootstrapIsIdempotent(t *testing.T) {
	b := connect(t)
	ctx := t.Context()

	for range 2 {
		require.NoError(t, b.EnsureDatabase(ctx, "idem_db"))
		require.NoError(t, b.EnsureUser(ctx, "idem_app", "app-pw"))
		require.NoError(t, b.GrantApplication(ctx, "idem_app", "idem_db"))
		require.NoError(t, b.EnsureBackupUser(ctx, "idem_backup", "backup-pw", "idem_db"))
	}

	appDB := mustConnectAs(t, "idem_app", "app-pw", "idem_db")
	assert.NoError(t, appDB.Ping(t.Context()))

	backupDB := mustConnectAs(t, "idem_backup", "backup-pw", "idem_db")
	assert.NoError(t, backupDB.Ping(t.Context()))
}

func TestApplicationUserCanWriteOwnDatabaseOnly(t *testing.T) {
	b := connect(t)
	ctx := t.Context()

	require.NoError(t, b.EnsureDatabase(ctx, "iso_db"))
	require.NoError(t, b.EnsureDatabase(ctx, "iso_other_db"))
	require.NoError(t, b.EnsureUser(ctx, "iso_app", "app-pw"))
	require.NoError(t, b.GrantApplication(ctx, "iso_app", "iso_db"))

	appDB := mustConnectAs(t, "iso_app", "app-pw", "iso_db")
	_, err := appDB.Exec(ctx, "CREATE TABLE things (id int primary key, name text)")
	require.NoError(t, err)
	_, err = appDB.Exec(ctx, "INSERT INTO things VALUES (1, 'a'), (2, 'b')")
	require.NoError(t, err)
	_, err = appDB.Exec(ctx, "UPDATE things SET name = 'c' WHERE id = 1")
	require.NoError(t, err)
	_, err = appDB.Exec(ctx, "DELETE FROM things WHERE id = 2")
	require.NoError(t, err)

	var count int
	require.NoError(t, appDB.QueryRow(ctx, "SELECT count(*) FROM things").Scan(&count))
	assert.Equal(t, 1, count)

	_, err = connectAs(ctx, "iso_app", "app-pw", "iso_other_db")
	assert.Error(t, err, "application user must not connect to a database it was not granted")
}

func TestBackupUserReadsAllTablesIncludingFutureOnes(t *testing.T) {
	b := connect(t)
	ctx := t.Context()

	require.NoError(t, b.EnsureDatabase(ctx, "bak_db"))
	require.NoError(t, b.EnsureUser(ctx, "bak_app", "app-pw"))
	require.NoError(t, b.GrantApplication(ctx, "bak_app", "bak_db"))

	appDB := mustConnectAs(t, "bak_app", "app-pw", "bak_db")
	_, err := appDB.Exec(ctx, "CREATE TABLE before_grant (id int)")
	require.NoError(t, err)
	_, err = appDB.Exec(ctx, "INSERT INTO before_grant VALUES (1)")
	require.NoError(t, err)

	require.NoError(t, b.EnsureBackupUser(ctx, "bak_backup", "backup-pw", "bak_db"))

	_, err = appDB.Exec(ctx, "CREATE TABLE after_grant (id int)")
	require.NoError(t, err)
	_, err = appDB.Exec(ctx, "INSERT INTO after_grant VALUES (2)")
	require.NoError(t, err)

	backupDB := mustConnectAs(t, "bak_backup", "backup-pw", "bak_db")

	var id int
	require.NoError(t, backupDB.QueryRow(ctx, "SELECT id FROM before_grant").Scan(&id))
	assert.Equal(t, 1, id)
	require.NoError(t, backupDB.QueryRow(ctx, "SELECT id FROM after_grant").Scan(&id),
		"backup user must read tables created after the grant")
	assert.Equal(t, 2, id)
}

func TestBackupUserHoldsRestoreRights(t *testing.T) {
	b := connect(t)
	ctx := t.Context()

	require.NoError(t, b.EnsureDatabase(ctx, "restore_db"))
	require.NoError(t, b.EnsureUser(ctx, "restore_app", "app-pw"))
	require.NoError(t, b.GrantApplication(ctx, "restore_app", "restore_db"))
	require.NoError(t, b.EnsureBackupUser(ctx, "restore_backup", "backup-pw", "restore_db"))

	backupDB := mustConnectAs(t, "restore_backup", "backup-pw", "restore_db")
	_, err := backupDB.Exec(ctx, "CREATE TABLE restored (id int primary key)")
	require.NoError(t, err, "restore needs DDL rights in the schema")
	_, err = backupDB.Exec(ctx, "INSERT INTO restored VALUES (1)")
	require.NoError(t, err, "restore needs write rights on restored tables")
}

func TestEnsureUserRotatesPassword(t *testing.T) {
	b := connect(t)
	ctx := t.Context()

	require.NoError(t, b.EnsureDatabase(ctx, "rot_db"))
	require.NoError(t, b.EnsureUser(ctx, "rot_app", "old-pw"))
	require.NoError(t, b.GrantApplication(ctx, "rot_app", "rot_db"))

	old := mustConnectAs(t, "rot_app", "old-pw", "rot_db")
	require.NoError(t, old.Ping(ctx))

	require.NoError(t, b.EnsureUser(ctx, "rot_app", "new-pw"))

	rotated := mustConnectAs(t, "rot_app", "new-pw", "rot_db")
	assert.NoError(t, rotated.Ping(t.Context()))

	_, err := connectAs(ctx, "rot_app", "old-pw", "rot_db")
	assert.Error(t, err, "the old password must stop working after rotation")
}
