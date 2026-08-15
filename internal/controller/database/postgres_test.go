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

package database

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

// testPostgresInfo locates the shared PostgreSQL container and its admin
// credentials.
type testPostgresInfo struct {
	Host          string
	Port          int32
	AdminUser     string
	AdminPassword string
}

var (
	testPostgresOnce   sync.Once
	testPostgresShared testPostgresInfo
	testPostgresErr    error
)

// testPostgres starts the shared postgres:17 container on first use and
// returns its coordinates. The testcontainers reaper removes the container
// when the test binary exits.
func testPostgres() (testPostgresInfo, error) {
	testPostgresOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		container, err := postgres.Run(ctx, "postgres:17",
			postgres.WithDatabase("postgres"),
			postgres.WithUsername("postgres"),
			postgres.WithPassword("admin-secret"),
			postgres.BasicWaitStrategies(),
		)
		if err != nil {
			testPostgresErr = fmt.Errorf("starting postgres container: %w", err)
			return
		}

		host, err := container.Host(ctx)
		if err != nil {
			testPostgresErr = fmt.Errorf("resolving container host: %w", err)
			return
		}
		port, err := container.MappedPort(ctx, "5432/tcp")
		if err != nil {
			testPostgresErr = fmt.Errorf("resolving container port: %w", err)
			return
		}

		testPostgresShared = testPostgresInfo{
			Host:          host,
			Port:          int32(port.Num()),
			AdminUser:     "postgres",
			AdminPassword: "admin-secret",
		}
	})

	return testPostgresShared, testPostgresErr
}

// pgConnect opens a direct connection to database on the shared container as
// user, or returns the connection error.
func pgConnect(ctx context.Context, user, password, database string) (*pgx.Conn, error) {
	pg, err := testPostgres()
	if err != nil {
		return nil, err
	}

	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable&connect_timeout=5",
		user, password, pg.Host, pg.Port, database)

	return pgx.Connect(ctx, url)
}
