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

package managementdatabases

import (
	"fmt"
	"slices"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/planedatabases"
	"github.com/konsole-is/camunda-operator/pkg/pluginbacking"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/database"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/databaseserver"
)

// Backing is the component that writes the server and the Database of every
// component for one lifecycle of the request.
type Backing struct {
	Component *component.Component
	// Server holds the DatabaseServer once it is read.
	Server *concepts.Data[v1.DatabaseServer]
	// Databases holds the Ready condition of the Database of each listed
	// component once it is read. The zero condition means it has none.
	Databases map[planedatabases.Component]*concepts.Data[metav1.Condition]
}

// NewBacking returns the component with condition type conditionType. While
// the request lives, it applies the server and the listed databases, and
// deletes the Database of each component that is not listed. While the
// request is deleted, it applies the deletion policy to the server and to the
// Database of every component. Every object is deleted only if the owner owns
// it.
func NewBacking(v Values, lifecycle pluginbacking.Lifecycle, conditionType string) (Backing, error) {
	backing := Backing{
		Server:    concepts.NewData[v1.DatabaseServer]("server"),
		Databases: map[planedatabases.Component]*concepts.Data[metav1.Condition]{},
	}

	sb := databaseserver.NewBuilder(Server(v))
	databaseserver.ExtractInto(sb, backing.Server, func(s v1.DatabaseServer) (v1.DatabaseServer, error) {
		return s, nil
	})
	server, err := sb.Build()
	if err != nil {
		return Backing{}, fmt.Errorf("building the DatabaseServer: %w", err)
	}

	b := component.NewComponentBuilder().
		WithName("backing").
		WithConditionType(component.ConditionType(conditionType)).
		WithResource(server, lifecycle.Options(false)...)
	for _, c := range planedatabases.Components() {
		db := database.NewBuilder(Database(v, c))
		listed := slices.Contains(v.Databases, c)
		if listed {
			cell := concepts.NewData[metav1.Condition](string(c))
			database.ExtractInto(db, cell, func(d v1.Database) (metav1.Condition, error) {
				if ready := meta.FindStatusCondition(d.Status.Conditions, v1.ConditionReady); ready != nil {
					return *ready, nil
				}
				return metav1.Condition{}, nil
			})
			backing.Databases[c] = cell
		}

		res, err := db.Build()
		if err != nil {
			return Backing{}, fmt.Errorf("building the Database of %s: %w", c, err)
		}
		b.WithResource(res, lifecycle.Options(!listed)...)
	}

	backing.Component, err = b.Build()
	if err != nil {
		return Backing{}, fmt.Errorf("building the backing component: %w", err)
	}

	return backing, nil
}
