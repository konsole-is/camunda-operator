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

// Package deleteobject is the delete subcommand of camunda-operator-cli,
// which the cleanup Job of a deleted backup runs: it removes one object from
// the backup bucket and exits. It runs where the identity lives — under the
// cluster ServiceAccount that the storage contract binds — so a bucket on
// workload identity is cleaned by the identity that wrote it, never by the
// manager's. The environment is its whole interface, the same UPLOAD_*
// contract the upload subcommand reads; a key that does not exist is
// success, so a retried Job is idempotent.
package deleteobject

import (
	"context"
	"fmt"
	"os"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/cli/storageenv"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
)

// deleter is the slice of the bucket that the subcommand needs.
type deleter interface {
	Delete(ctx context.Context, key string) error
	Close()
}

// Run reads the environment, opens the bucket, and deletes the object. It is
// the entry point of the subcommand; a non-nil error means a non-zero exit.
func Run(ctx context.Context) error {
	return run(ctx, os.Getenv, openBucket)
}

func run(
	ctx context.Context,
	getenv func(string) string,
	open func(ctx context.Context, cfg *v1.ObjectStorageConfig, creds *objectstore.Credentials) (deleter, error),
) error {
	key := getenv(components.EnvUploadKey)
	if key == "" {
		return fmt.Errorf("%s must be set", components.EnvUploadKey)
	}

	cfg, creds, err := storageenv.Load(getenv)
	if err != nil {
		return err
	}

	bucket, err := open(ctx, cfg, creds)
	if err != nil {
		return fmt.Errorf("opening the bucket of %q: %w", cfg.Name, err)
	}
	defer bucket.Close()

	if err := bucket.Delete(ctx, key); err != nil {
		return fmt.Errorf("deleting %q: %w", key, err)
	}

	return nil
}

func openBucket(
	ctx context.Context,
	cfg *v1.ObjectStorageConfig,
	creds *objectstore.Credentials,
) (deleter, error) {
	return objectstore.Open(ctx, cfg, creds)
}
