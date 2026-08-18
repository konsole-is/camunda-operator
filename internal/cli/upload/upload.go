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

// Package upload is the upload subcommand of camunda-operator-cli, which the
// dump Job runs as its main container. It streams one file to the backup
// bucket and exits. The environment is its whole interface. It is rendered by
// pkg/components/logicalbackuprdbms and read here. The Job succeeds only when
// the upload succeeded.
package upload

import (
	"context"
	"fmt"
	"io"
	"os"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/cli/storageenv"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
)

// uploader is the slice of the bucket that the subcommand needs.
type uploader interface {
	Upload(ctx context.Context, key string, r io.Reader) error
	Close()
}

// Run reads the environment, opens the bucket, and uploads the file. It is
// the entry point of the subcommand. A non-nil error means a non-zero exit.
func Run(ctx context.Context) error {
	return run(ctx, os.Getenv, openBucket)
}

func run(
	ctx context.Context,
	getenv func(string) string,
	open func(ctx context.Context, cfg *v1.ObjectStorageConfig, creds *objectstore.Credentials) (uploader, error),
) error {
	file := getenv(components.EnvUploadFile)
	key := getenv(components.EnvUploadKey)
	if file == "" || key == "" {
		return fmt.Errorf(
			"%s and %s must both be set", components.EnvUploadFile, components.EnvUploadKey,
		)
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

	source, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("opening the dump: %w", err)
	}
	defer func() { _ = source.Close() }()

	if err := bucket.Upload(ctx, key, source); err != nil {
		return fmt.Errorf("uploading the dump to %q: %w", key, err)
	}

	return nil
}

func openBucket(
	ctx context.Context,
	cfg *v1.ObjectStorageConfig,
	creds *objectstore.Credentials,
) (uploader, error) {
	return objectstore.Open(ctx, cfg, creds)
}
