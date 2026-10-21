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

// Package managementdatabases builds the DatabaseServer and the Database
// objects that the built-in registration management-database-server applies
// for a management plane.
package managementdatabases

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// Component is a component of a management plane that gets its own database.
type Component string

// The components of a management plane that get a database.
const (
	Keycloak   Component = "keycloak"
	Identity   Component = "identity"
	WebModeler Component = "webModeler"
)

// Components are all components, in the order keycloak, identity, webModeler.
var Components = []Component{Keycloak, Identity, WebModeler}

// Values are the inputs of the backing resources of one request.
type Values struct {
	Namespace string
	// Plane is the name of the plane claim.
	Plane string
	// Server is the name of the DatabaseServer and of its DatabaseServerConfig.
	Server string
	// Preset is the name of a DatabaseServerPreset.
	Preset string
	// Version is the PostgreSQL major version of the server.
	Version string
	// StorageClassName is the StorageClass of the server volumes. Empty
	// keeps the default StorageClass.
	StorageClassName string
	// Databases are the listed components, without duplicates, in the order
	// of Components.
	Databases []Component
	// Labels are the labels of every object.
	Labels map[string]string
}

// Server returns the DatabaseServer of v.
func Server(v Values) *v1.DatabaseServer {
	server := &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{Name: v.Server, Namespace: v.Namespace, Labels: v.Labels},
		Spec: v1.DatabaseServerSpec{
			PresetRef:            v.Preset,
			Version:              v.Version,
			DatabaseServerConfig: v.Server,
		},
	}
	if v.StorageClassName != "" {
		server.Spec.StorageClassName = &v.StorageClassName
	}

	return server
}

// Database returns the Database of c on the server of v. It publishes its
// DatabaseConfig under its own name.
func Database(v Values, c Component) *v1.Database {
	name := DatabaseName(v.Plane, c)
	return &v1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: v.Namespace, Labels: v.Labels},
		Spec: v1.DatabaseSpec{
			ServerRef:      v.Server,
			DatabaseName:   LogicalName(c),
			DatabaseConfig: name,
		},
	}
}

// DatabaseName returns the name of the Database of c for the plane:
// <plane>-keycloak, <plane>-identity, or <plane>-web-modeler.
func DatabaseName(plane string, c Component) string {
	if c == WebModeler {
		return plane + "-web-modeler"
	}

	return plane + "-" + string(c)
}

// LogicalName returns the logical database of c on the server: keycloak,
// identity, or web_modeler.
func LogicalName(c Component) string {
	if c == WebModeler {
		return "web_modeler"
	}

	return string(c)
}
