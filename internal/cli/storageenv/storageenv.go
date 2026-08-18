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

// Package storageenv reads the bucket half of the environment contract that
// pkg/components/logicalbackuprdbms renders into the backup Jobs. That half
// is the ObjectStorageConfig spec as JSON and the projected static
// credentials. The upload and delete subcommands of camunda-operator-cli both
// read it here, so the two can never disagree with the Job or with each
// other.
package storageenv

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/logicalbackuprdbms"
	"github.com/konsole-is/camunda-operator/pkg/objectstore"
)

// Load reads the bucket contract and its credentials from the environment.
// Workload identity projects no credentials and yields nil. The provider
// chain then authenticates as the ServiceAccount of the pod.
func Load(getenv func(string) string) (*v1.ObjectStorageConfig, *objectstore.Credentials, error) {
	cfg := &v1.ObjectStorageConfig{}
	cfg.Name = getenv(components.EnvUploadStorageName)
	if err := json.Unmarshal([]byte(getenv(components.EnvUploadStorageSpec)), &cfg.Spec); err != nil {
		return nil, nil, fmt.Errorf("decoding %s: %w", components.EnvUploadStorageSpec, err)
	}

	creds, err := credentials(cfg, getenv)
	if err != nil {
		return nil, nil, err
	}

	return cfg, creds, nil
}

// credentials rebuilds the Secret data of the static credentials of the
// contract from the indexed environment that the Job projected. It hands the
// data to the one mapping in objectstore.CredentialsFrom, the same call that
// the operator makes. Every storage type therefore resolves here exactly as
// it does there.
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
