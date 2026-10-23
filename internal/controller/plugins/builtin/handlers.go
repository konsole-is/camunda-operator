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

package builtin

import (
	"maps"
	"slices"

	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/adapter"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/elasticsearch"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/existing"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/managementdatabase"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/postgresql"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

// Served returns the sorted names of the built-in registrations whose
// requests the controller serves.
func Served() []string {
	return slices.Sorted(maps.Keys(handlers()))
}

// handlers returns the handler of each built-in registration that the
// controller serves, by registration name.
func handlers() map[string]adapter.Handler {
	return map[string]adapter.Handler{
		pluginstartup.RegistrationDatabaseServer: postgresql.Handler{},
		pluginstartup.RegistrationElasticsearch:  elasticsearch.Handler{},
		pluginstartup.RegistrationExistingObjectStorage: existing.OneContract(
			pluginstartup.OutputObjectStorageConfig,
			adapter.KindObjectStorageConfig,
		),
		pluginstartup.RegistrationExistingSecondaryStorage: existing.OneContract(
			pluginstartup.OutputSecondaryStorageConfig,
			adapter.KindSecondaryStorageConfig,
		),
		pluginstartup.RegistrationExistingDatabaseConfigs:  existing.DatabaseConfigs(),
		pluginstartup.RegistrationManagementDatabaseServer: managementdatabase.Handler{},
	}
}
