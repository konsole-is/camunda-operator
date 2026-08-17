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
// dump Job runs as its main container: it streams one file to the backup
// bucket and exits. The environment is its whole interface, rendered by
// pkg/components/logicalbackuprdbms and read here; the Job succeeds only when
// the upload did.
package upload

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
)

// uploader is the slice of the bucket that the subcommand needs.
type uploader interface {
	Upload(ctx context.Context, key string, r io.Reader) error
	Close()
}

// Run reads the environment, opens the bucket, and uploads the file. It is
// the entry point of the subcommand; a non-nil error means a non-zero exit.
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

	cfg := &v1.ObjectStorageConfig{}
	cfg.Name = getenv(components.EnvUploadStorageName)
	if err := json.Unmarshal([]byte(getenv(components.EnvUploadStorageSpec)), &cfg.Spec); err != nil {
		return fmt.Errorf("decoding %s: %w", components.EnvUploadStorageSpec, err)
	}

	creds, err := credentials(cfg, getenv)
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

// credentials rebuilds the Secret data of the contract's static credentials
// from the indexed environment the Job projected, and hands it to the one
// mapping in objectstore.CredentialsFrom — the same call the operator makes,
// so every storage type resolves here exactly as it does there. Workload
// identity projects nothing and yields nil.
func credentials(
	cfg *v1.ObjectStorageConfig,
	getenv func(string) string,
) (*objectstore.Credentials, error) {
	names := getenv(components.EnvUploadCredentialKeys)
	if names == "" {
		return nil, nil
	}

	data := map[string][]byte{}
	for i, key := range strings.Split(names, ",") {
		value := getenv(components.EnvUploadCredentialPrefix + strconv.Itoa(i))
		if value == "" {
			return nil, fmt.Errorf(
				"credential key %q was projected without a value; the Secret may have lost it", key,
			)
		}
		data[key] = []byte(value)
	}

	creds, err := objectstore.CredentialsFrom(cfg, data)
	if err != nil {
		return nil, fmt.Errorf("resolving the projected credentials: %w", err)
	}

	return creds, nil
}

func openBucket(
	ctx context.Context,
	cfg *v1.ObjectStorageConfig,
	creds *objectstore.Credentials,
) (uploader, error) {
	return objectstore.Open(ctx, cfg, creds)
}
