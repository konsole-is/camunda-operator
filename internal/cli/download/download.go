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

// Package download is the download subcommand of camunda-operator-cli, which
// the restore Job runs as its init container. It streams one object from the
// backup bucket into a file and exits. The environment is its whole interface.
// It is rendered by pkg/components/logicalrestore and read here. The DOWNLOAD_*
// half names the object and the file. The bucket half is the same UPLOAD_*
// contract that the upload and delete subcommands read, so the three can never
// disagree.
package download

import (
	"context"
	"fmt"
	"io"
	"os"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/cli/storageenv"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalrestore"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
)

// downloader is the slice of the bucket that the subcommand needs.
type downloader interface {
	Download(ctx context.Context, key string, w io.Writer) error
	Close()
}

// fileMode is the mode of the downloaded archive. The group bit is what lets
// the restore container read it. The two containers run as different users,
// and the fsGroup of the pod puts the file in the group of the postgres user.
const fileMode = 0o640

// Run reads the environment, opens the bucket, and downloads the object. It is
// the entry point of the subcommand. A non-nil error means a non-zero exit.
func Run(ctx context.Context) error {
	return run(ctx, os.Getenv, openBucket)
}

func run(
	ctx context.Context,
	getenv func(string) string,
	open func(ctx context.Context, cfg *v1.ObjectStorageConfig, creds *objectstore.Credentials) (downloader, error),
) error {
	file := getenv(components.EnvDownloadFile)
	key := getenv(components.EnvDownloadKey)
	if file == "" || key == "" {
		return fmt.Errorf(
			"%s and %s must both be set", components.EnvDownloadFile, components.EnvDownloadKey,
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

	target, err := os.OpenFile(file, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, fileMode)
	if err != nil {
		return fmt.Errorf("creating the dump file %q: %w", file, err)
	}

	// A transfer that breaks partway leaves a truncated archive. pg_restore
	// reads such a file a long way before it fails, so the file is removed
	// here and the Job fails now.
	if err := bucket.Download(ctx, key, target); err != nil {
		_ = target.Close()
		_ = os.Remove(file)

		return fmt.Errorf("downloading the dump from %q: %w", key, err)
	}

	if err := target.Close(); err != nil {
		_ = os.Remove(file)

		return fmt.Errorf("finishing the dump file %q: %w", file, err)
	}

	return nil
}

func openBucket(
	ctx context.Context,
	cfg *v1.ObjectStorageConfig,
	creds *objectstore.Credentials,
) (downloader, error) {
	return objectstore.Open(ctx, cfg, creds)
}
