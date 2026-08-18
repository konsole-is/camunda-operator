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

// Package pgbootstrap bootstraps logical databases and their users on an
// existing PostgreSQL server over plain, idempotent SQL. It is the SQL layer
// of the Database controller and lives outside the component framework. It
// only issues DDL through an admin connection and never touches Kubernetes.
package pgbootstrap

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/jackc/pgx/v5"
)

// maintenanceDatabase is the database that the admin connection attaches to
// for server-level statements. Per-database grants open their own connection.
const maintenanceDatabase = "postgres"

// Connection carries the values that open an admin connection to a PostgreSQL
// server. An empty SSLMode defaults to "prefer".
type Connection struct {
	// Host is the address of the server.
	Host string
	// Port is the port that the server listens on.
	Port int32
	// AdminUser is a role that can create databases and roles.
	AdminUser string
	// AdminPassword authenticates AdminUser.
	AdminPassword string
	// SSLMode is the libpq sslmode parameter for every connection that this
	// package opens.
	SSLMode string
}

// Bootstrapper is the idempotent SQL surface that the Database controller
// drives. Every operation is safe to run again. Names must be plain lowercase
// PostgreSQL identifiers (^[a-z_][a-z0-9_]{0,62}$), and other names are
// rejected.
type Bootstrapper interface {
	// EnsureDatabase creates the logical database if it does not exist. It
	// also revokes the default PUBLIC connect privilege, so only explicitly
	// granted roles can reach the database.
	EnsureDatabase(ctx context.Context, name string) error
	// EnsureUser creates the login role if it does not exist and always sets
	// its password. A rotated password then converges on the server before
	// it is published anywhere.
	EnsureUser(ctx context.Context, name, password string) error
	// GrantApplication grants user full privileges on database and makes it
	// the database owner. The owner has the schema DDL rights that migrations
	// need.
	GrantApplication(ctx context.Context, user, database string) error
	// EnsureBackupUser creates the backup login role and grants it read
	// access on all tables of database. Through ALTER DEFAULT PRIVILEGES the
	// grant includes tables that the application role creates later. It also
	// grants the schema DDL and table write rights that a restore needs. Call
	// it after GrantApplication, so the default privileges of the database
	// owner are altered for the application role.
	EnsureBackupUser(ctx context.Context, name, password, database string) error
	// Ping checks that the admin connection is alive.
	Ping(ctx context.Context) error
	// ServerVersion reads the major version of the server, for example "17".
	// A dump must run client tools of at least the server's major, so the
	// contract publishes it for the consumers that pick those tools.
	ServerVersion(ctx context.Context) (string, error)
	// Close releases the admin connection.
	Close()
}

// bootstrapper implements Bootstrapper over pgx connections. It holds one
// long-lived admin connection to the maintenance database, plus short-lived
// per-database connections for schema-level grants.
type bootstrapper struct {
	conn  Connection
	admin *pgx.Conn
}

// Connect opens the admin connection that c describes and returns the
// Bootstrapper over it. An unreachable server or rejected credentials cause an
// error here.
func Connect(ctx context.Context, c Connection) (Bootstrapper, error) {
	admin, err := dial(ctx, c, maintenanceDatabase)
	if err != nil {
		return nil, err
	}

	return &bootstrapper{conn: c, admin: admin}, nil
}

// dial opens a connection to database on the server that c describes.
func dial(ctx context.Context, c Connection, database string) (*pgx.Conn, error) {
	sslMode := c.SSLMode
	if sslMode == "" {
		sslMode = "prefer"
	}

	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(c.AdminUser, c.AdminPassword),
		Host:     fmt.Sprintf("%s:%d", c.Host, c.Port),
		Path:     "/" + database,
		RawQuery: url.Values{"sslmode": {sslMode}, "connect_timeout": {"5"}}.Encode(),
	}

	conn, err := pgx.Connect(ctx, u.String())
	if err != nil {
		return nil, fmt.Errorf("connecting to %s:%d/%s: %w", c.Host, c.Port, database, err)
	}

	return conn, nil
}

func (b *bootstrapper) EnsureDatabase(ctx context.Context, name string) error {
	if err := validateIdentifier(name); err != nil {
		return err
	}

	var exists bool
	err := b.admin.QueryRow(
		ctx,
		"SELECT EXISTS (SELECT FROM pg_database WHERE datname = $1)", name,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("checking database %q: %w", name, err)
	}

	if !exists {
		if _, err := b.admin.Exec(ctx, "CREATE DATABASE "+quoteIdentifier(name)); err != nil {
			return fmt.Errorf("creating database %q: %w", name, err)
		}
	}

	// A new database grants CONNECT to PUBLIC by default. The revoke keeps the
	// logical databases on a shared server isolated to their granted roles.
	if _, err := b.admin.Exec(
		ctx,
		"REVOKE CONNECT ON DATABASE "+quoteIdentifier(name)+" FROM PUBLIC",
	); err != nil {
		return fmt.Errorf("revoking public connect on database %q: %w", name, err)
	}

	return nil
}

func (b *bootstrapper) EnsureUser(ctx context.Context, name, password string) error {
	if err := validateIdentifier(name); err != nil {
		return err
	}

	var exists bool
	err := b.admin.QueryRow(
		ctx,
		"SELECT EXISTS (SELECT FROM pg_roles WHERE rolname = $1)", name,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("checking role %q: %w", name, err)
	}

	// CREATE ROLE and ALTER ROLE cannot take bind parameters, so the password
	// goes through quoteLiteral. ALTER runs also for an existing role, so a
	// rotated password always converges on the server.
	verb := "CREATE"
	if exists {
		verb = "ALTER"
	}
	if _, err := b.admin.Exec(
		ctx,
		verb+" ROLE "+quoteIdentifier(name)+" WITH LOGIN PASSWORD "+quoteLiteral(password),
	); err != nil {
		return fmt.Errorf("ensuring role %q: %w", name, err)
	}

	return nil
}

func (b *bootstrapper) GrantApplication(ctx context.Context, user, database string) error {
	if err := validateIdentifier(user); err != nil {
		return err
	}
	if err := validateIdentifier(database); err != nil {
		return err
	}

	if _, err := b.admin.Exec(
		ctx,
		"GRANT ALL PRIVILEGES ON DATABASE "+quoteIdentifier(database)+" TO "+quoteIdentifier(user),
	); err != nil {
		return fmt.Errorf("granting application privileges on %q to %q: %w", database, user, err)
	}

	owner, err := b.databaseOwner(ctx, b.admin, database)
	if err != nil {
		return err
	}
	if owner != user {
		// ALTER DATABASE OWNER requires that the admin is a member of the new
		// owning role. The membership is granted only for this statement.
		err := b.withRoleMembership(ctx, b.admin, user, func() error {
			_, err := b.admin.Exec(
				ctx,
				"ALTER DATABASE "+quoteIdentifier(database)+" OWNER TO "+quoteIdentifier(user),
			)
			return err
		})
		if err != nil {
			return fmt.Errorf("transferring ownership of %q to %q: %w", database, user, err)
		}
	}

	// Schema privileges are per database. On PostgreSQL 15 and later,
	// ownership already implies them, because schema public belongs to
	// pg_database_owner. The explicit grant keeps older servers working.
	db, err := dial(ctx, b.conn, database)
	if err != nil {
		return err
	}
	defer closeQuietly(db)

	if _, err := db.Exec(
		ctx,
		"GRANT USAGE, CREATE ON SCHEMA public TO "+quoteIdentifier(user),
	); err != nil {
		return fmt.Errorf("granting schema privileges on %q to %q: %w", database, user, err)
	}

	return nil
}

func (b *bootstrapper) EnsureBackupUser(ctx context.Context, name, password, database string) error {
	if err := validateIdentifier(name); err != nil {
		return err
	}
	if err := validateIdentifier(database); err != nil {
		return err
	}

	if err := b.EnsureUser(ctx, name, password); err != nil {
		return err
	}

	if _, err := b.admin.Exec(
		ctx,
		"GRANT CONNECT ON DATABASE "+quoteIdentifier(database)+" TO "+quoteIdentifier(name),
	); err != nil {
		return fmt.Errorf("granting connect on %q to %q: %w", database, name, err)
	}

	db, err := dial(ctx, b.conn, database)
	if err != nil {
		return err
	}
	defer closeQuietly(db)

	// Read access on all current tables, plus the DDL and write rights that a
	// restore needs: CREATE on the schema to recreate tables, and full DML on
	// existing tables to refill them.
	grants := []string{
		"GRANT USAGE, CREATE ON SCHEMA public TO " + quoteIdentifier(name),
		"GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE, REFERENCES, TRIGGER ON ALL TABLES IN SCHEMA public TO " +
			quoteIdentifier(name),
		"GRANT USAGE, SELECT, UPDATE ON ALL SEQUENCES IN SCHEMA public TO " + quoteIdentifier(name),
	}
	for _, stmt := range grants {
		if _, err := db.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("granting backup privileges on %q to %q: %w", database, name, err)
		}
	}

	// Tables created later belong to the database owner, which is the
	// application role. The altered default privileges of that role keep
	// them readable.
	owner, err := b.databaseOwner(ctx, db, database)
	if err != nil {
		return err
	}

	// ALTER DEFAULT PRIVILEGES FOR ROLE requires membership of that role. The
	// membership is granted only around these statements.
	err = b.withRoleMembership(ctx, db, owner, func() error {
		defaults := []string{
			"ALTER DEFAULT PRIVILEGES FOR ROLE " + quoteIdentifier(owner) +
				" IN SCHEMA public GRANT SELECT ON TABLES TO " + quoteIdentifier(name),
			"ALTER DEFAULT PRIVILEGES FOR ROLE " + quoteIdentifier(owner) +
				" IN SCHEMA public GRANT SELECT ON SEQUENCES TO " + quoteIdentifier(name),
		}
		for _, stmt := range defaults {
			if _, err := db.Exec(ctx, stmt); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("granting default read privileges on %q to %q: %w", database, name, err)
	}

	return nil
}

// databaseOwner resolves the owning role of database through conn. It
// validates the role as a plain identifier, so callers can interpolate it into
// DDL.
func (b *bootstrapper) databaseOwner(ctx context.Context, conn *pgx.Conn, database string) (string, error) {
	var owner string
	if err := conn.QueryRow(
		ctx,
		"SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname = $1", database,
	).Scan(&owner); err != nil {
		return "", fmt.Errorf("resolving owner of database %q: %w", database, err)
	}

	if err := validateIdentifier(owner); err != nil {
		return "", fmt.Errorf("database %q has an unexpected owner: %w", database, err)
	}

	return owner, nil
}

// withRoleMembership runs fn while the connected admin is a member of role,
// then restores the previous state. It grants the membership only when it is
// absent, that is, when the admin is not the role itself and holds no direct
// membership. It revokes the membership afterwards only when this call granted
// it, so an existing membership survives untouched.
func (b *bootstrapper) withRoleMembership(ctx context.Context, conn *pgx.Conn, role string, fn func() error) error {
	var member bool
	if err := conn.QueryRow(
		ctx,
		`SELECT to_regrole($1) = to_regrole(current_user::text) OR EXISTS (
			SELECT FROM pg_auth_members
			WHERE roleid = to_regrole($1) AND member = to_regrole(current_user::text)
		)`, role,
	).Scan(&member); err != nil {
		return fmt.Errorf("checking membership in role %q: %w", role, err)
	}

	if !member {
		if _, err := conn.Exec(ctx, "GRANT "+quoteIdentifier(role)+" TO CURRENT_USER"); err != nil {
			return fmt.Errorf("granting temporary membership in role %q: %w", role, err)
		}
	}

	fnErr := fn()

	if !member {
		if _, err := conn.Exec(ctx, "REVOKE "+quoteIdentifier(role)+" FROM CURRENT_USER"); err != nil {
			return errors.Join(fnErr, fmt.Errorf("revoking temporary membership in role %q: %w", role, err))
		}
	}

	return fnErr
}

func (b *bootstrapper) Ping(ctx context.Context) error {
	return b.admin.Ping(ctx)
}

func (b *bootstrapper) ServerVersion(ctx context.Context) (string, error) {
	var num string
	if err := b.admin.QueryRow(
		ctx, "SELECT current_setting('server_version_num')",
	).Scan(&num); err != nil {
		return "", fmt.Errorf("reading the server version: %w", err)
	}

	return majorVersion(num)
}

// majorVersion extracts the major from a server_version_num value, in the
// form that names the matching client tools. Since PostgreSQL 10 the number
// is MMmmmm, the major followed by four digits of the minor, and the major
// is everything before the last four digits ("170004" is "17"). Before 10
// the number is MMmmpp and the major has two components: "90624" is
// PostgreSQL 9.6.24, so the major is "9.6", never "9".
func majorVersion(num string) (string, error) {
	for _, r := range num {
		if r < '0' || r > '9' {
			return "", fmt.Errorf("unexpected server_version_num %q", num)
		}
	}
	switch {
	case len(num) == 5:
		minor := strings.TrimLeft(num[1:3], "0")
		if minor == "" {
			// 9.0.x is "900xx": an all-zero minor is still a minor.
			minor = "0"
		}

		return num[:1] + "." + minor, nil
	case len(num) > 5:
		return num[:len(num)-4], nil
	}

	return "", fmt.Errorf("unexpected server_version_num %q", num)
}

func (b *bootstrapper) Close() {
	closeQuietly(b.admin)
}

// closeQuietly closes conn with a fresh context. The shutdown then runs also
// when the reconcile context is already cancelled.
func closeQuietly(conn *pgx.Conn) {
	_ = conn.Close(context.Background())
}
