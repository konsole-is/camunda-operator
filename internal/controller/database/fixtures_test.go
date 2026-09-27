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

package database

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

const (
	// schemaTestNamespace hosts the throwaway objects that the admission
	// specs create.
	schemaTestNamespace = "default"
	// notAResourceName is a value that the resource-name rules of the
	// schemas must reject.
	notAResourceName = "Not_A_Name"
)

// newDatabaseServerConfig returns the minimal example of the CRD doc with a
// unique name in namespace. Its admin credentials Secret is admin-creds of
// the same namespace, with the keys username and password. The caller creates
// that Secret.
func newDatabaseServerConfig(namespace string) *v1.DatabaseServerConfig {
	return &v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbsc-" + utilrand.String(8), Namespace: namespace},
		Spec: v1.DatabaseServerConfigSpec{
			Engine: v1.DatabaseEnginePostgres,
			Host:   "postgres.camunda-system.svc.cluster.local",
			Port:   5432,
			AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
				Name: "admin-creds", UsernameKey: "username", PasswordKey: "password",
			},
		},
	}
}
