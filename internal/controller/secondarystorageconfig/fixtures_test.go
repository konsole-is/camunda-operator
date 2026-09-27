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

package secondarystorageconfig

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

const (
	// schemaTestNamespace hosts the throwaway objects that the admission
	// specs create.
	schemaTestNamespace = "default"
	// notAURL is a value that the URL rules of the schemas must reject.
	notAURL = "not a url"
)

// newDatabaseConfig returns the minimal example of the CRD doc with a unique
// name. The caller chooses the namespace.
func newDatabaseConfig() *v1.DatabaseConfig {
	return &v1.DatabaseConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbc-" + utilrand.String(8)},
		Spec: v1.DatabaseConfigSpec{
			ServerRef:    "my-db-server",
			DatabaseName: "camunda",
			CredentialsSecretRef: v1.LocalCredentialsSecretRef{
				Name: "my-camunda-db-credentials", UsernameKey: "username", PasswordKey: "password",
			},
		},
	}
}
