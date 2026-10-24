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

package planedatabases

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComponentsKeepTheOrderOfAll(t *testing.T) {
	t.Parallel()

	assert.Equal(t, []Component{Keycloak, Identity, WebModeler}, Components())
}

// Two components with one name would publish one output, one Database, or
// one field twice.
func TestEachComponentHasNamesOfItsOwn(t *testing.T) {
	t.Parallel()

	columns := map[string]func(Database) string{
		"component":    func(d Database) string { return string(d.Component) },
		"title":        func(d Database) string { return d.Title },
		"output":       func(d Database) string { return d.Output },
		"name suffix":  func(d Database) string { return d.NameSuffix },
		"logical name": func(d Database) string { return d.LogicalName },
		"field path":   func(d Database) string { return d.FieldPath },
	}

	for column, value := range columns {
		seen := map[string]bool{}
		for _, d := range All {
			v := value(d)
			assert.NotEmpty(t, v, "%s of %s", column, d.Component)
			assert.False(t, seen[v], "%s %q of %s is not unique", column, v, d.Component)
			seen[v] = true
		}
	}
}

func TestOfReturnsTheDatabaseOfTheComponent(t *testing.T) {
	t.Parallel()

	d := Of(WebModeler)

	assert.Equal(t, WebModeler, d.Component)
	assert.Equal(t, "webModelerDatabaseConfig", d.Output)
	assert.Equal(t, "web-modeler", d.NameSuffix)
	assert.Equal(t, "web_modeler", d.LogicalName)
	assert.Equal(t, "spec.webModeler.databaseConfigRef", d.FieldPath)
}

func TestOfAnUnknownComponentIsTheZeroDatabase(t *testing.T) {
	t.Parallel()

	assert.Equal(t, Database{}, Of("console"))
}

func TestOutputDescriptionNamesTheTitle(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "The name of the DatabaseConfig of Management Identity.", Of(Identity).OutputDescription())
}
