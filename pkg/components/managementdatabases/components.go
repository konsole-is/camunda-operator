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
	"github.com/konsole-is/camunda-operator/pkg/wrappers/database"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/databaseserver"
)

// Backing is the component that applies the server and the listed databases,
// and deletes the Database of each component that is not listed.
type Backing struct {
	Component *component.Component
	// Server holds the DatabaseServer once it is read.
	Server *concepts.Data[v1.DatabaseServer]
	// Databases holds the Ready condition of the Database of each listed
	// component once it is read. The zero condition means it has none.
	Databases map[Component]*concepts.Data[metav1.Condition]
}

// NewBacking returns the component with condition type conditionType. Every
// object is deleted only if the owner owns it.
func NewBacking(v Values, conditionType string) (Backing, error) {
	backing := Backing{
		Server:    concepts.NewData[v1.DatabaseServer]("server"),
		Databases: map[Component]*concepts.Data[metav1.Condition]{},
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
		WithResource(server, component.DeleteOnlyIfOwned())
	for _, c := range Components {
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
		b.WithResource(res, component.DeleteWhen(!listed), component.DeleteOnlyIfOwned())
	}

	backing.Component, err = b.Build()
	if err != nil {
		return Backing{}, fmt.Errorf("building the backing component: %w", err)
	}

	return backing, nil
}

// DeleteDatabases returns the component that deletes the Database of every
// component, if the owner owns it.
func DeleteDatabases(v Values) (*component.Component, error) {
	b := component.NewComponentBuilder().
		WithName("databases").
		WithConditionType(v1.ConditionReady)
	for _, c := range Components {
		res, err := database.NewBuilder(Database(v, c)).Build()
		if err != nil {
			return nil, fmt.Errorf("building the Database of %s: %w", c, err)
		}
		b.WithResource(res, component.Delete(), component.DeleteOnlyIfOwned())
	}

	return b.Build()
}

// DeleteServer returns the component that deletes the DatabaseServer, if the
// owner owns it.
func DeleteServer(v Values) (*component.Component, error) {
	res, err := databaseserver.NewBuilder(Server(v)).Build()
	if err != nil {
		return nil, fmt.Errorf("building the DatabaseServer: %w", err)
	}

	return component.NewComponentBuilder().
		WithName("server").
		WithConditionType(v1.ConditionReady).
		WithResource(res, component.Delete(), component.DeleteOnlyIfOwned()).
		Build()
}

// Orphan returns the component that removes the owner reference from the
// server and from the Database of every component, and keeps the objects.
func Orphan(v Values) (*component.Component, error) {
	server, err := databaseserver.NewBuilder(Server(v)).Build()
	if err != nil {
		return nil, fmt.Errorf("building the DatabaseServer: %w", err)
	}

	b := component.NewComponentBuilder().
		WithName("orphan").
		WithConditionType(v1.ConditionReady).
		WithResource(server, component.OrphanWhen(true))
	for _, c := range Components {
		res, err := database.NewBuilder(Database(v, c)).Build()
		if err != nil {
			return nil, fmt.Errorf("building the Database of %s: %w", c, err)
		}
		b.WithResource(res, component.OrphanWhen(true))
	}

	return b.Build()
}
