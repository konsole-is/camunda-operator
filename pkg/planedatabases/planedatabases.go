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

// Package planedatabases lists the components of a management plane that get
// a database of their own, with each name that the plugin contract publishes
// for that database. It imports nothing, so the packages under pkg/ and
// internal/ can all read it.
package planedatabases

// Component is a component of a management plane that gets a database of its
// own. Its value is the key of the component in the parameters of the
// management database registrations.
type Component string

// The components of a management plane that get a database.
const (
	Keycloak   Component = "keycloak"
	Identity   Component = "identity"
	WebModeler Component = "webModeler"
)

// Database holds the names of the database of one component.
type Database struct {
	Component Component
	// Title names the component in the descriptions of the plugin contract.
	Title string
	// Output is the output key that holds the name of the DatabaseConfig.
	Output string
	// NameSuffix ends the name of the Database: <plane>-<NameSuffix>.
	NameSuffix string
	// LogicalName is the logical database on the server.
	LogicalName string
	// FieldPath is the field of the CamundaManagementCluster that the output
	// fills, in the field path grammar of the fleet manager.
	FieldPath string
}

// OutputDescription returns the description of the output of d.
func (d Database) OutputDescription() string {
	return "The name of the DatabaseConfig of " + d.Title + "."
}

// All are the databases of all components, in the order keycloak, identity,
// webModeler. The schemas, the outputs, and the Databases follow this order.
var All = []Database{
	{
		Component:   Keycloak,
		Title:       "Keycloak",
		Output:      "keycloakDatabaseConfig",
		NameSuffix:  "keycloak",
		LogicalName: "keycloak",
		FieldPath:   "spec.identityProvider.keycloak.databaseConfigRef",
	},
	{
		Component:   Identity,
		Title:       "Management Identity",
		Output:      "identityDatabaseConfig",
		NameSuffix:  "identity",
		LogicalName: "identity",
		FieldPath:   "spec.identity.databaseConfigRef",
	},
	{
		Component:   WebModeler,
		Title:       "Web Modeler",
		Output:      "webModelerDatabaseConfig",
		NameSuffix:  "web-modeler",
		LogicalName: "web_modeler",
		FieldPath:   "spec.webModeler.databaseConfigRef",
	},
}

// Of returns the database of c. It returns the zero Database when c is not
// in All.
func Of(c Component) Database {
	for _, d := range All {
		if d.Component == c {
			return d
		}
	}

	return Database{}
}

// Components returns the component of each database of All, in its order.
func Components() []Component {
	components := make([]Component, 0, len(All))
	for _, d := range All {
		components = append(components, d.Component)
	}

	return components
}
