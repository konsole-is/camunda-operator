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

// Package fixtures holds the sample CRs and schema probe values that more than
// one controller suite needs. A fixture that one package uses stays in the
// tests of that package. Only the fixtures that cross a package boundary live
// here, for example a contract that another controller references. Each of
// them then has exactly one definition.
package fixtures

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

const (
	// SchemaTestNamespace hosts the throwaway objects that the admission specs
	// create.
	SchemaTestNamespace = "default"
	// NotAResourceName is a value that the resource-name rules of the schemas
	// must reject.
	NotAResourceName = "Not_A_Name"
	// NotAURL is a value that the URL rules of the schemas must reject.
	NotAURL = "not a url"
)

// DatabaseServerConfig returns the minimal example of the CRD doc with a
// unique name in namespace. Its admin credentials Secret is admin-creds of
// the same namespace, with the keys username and password. The caller creates
// that Secret.
func DatabaseServerConfig(namespace string) *v1.DatabaseServerConfig {
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

// DatabaseConfig returns the minimal example of the CRD doc with a unique
// name. The caller chooses the namespace.
func DatabaseConfig() *v1.DatabaseConfig {
	return &v1.DatabaseConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "dbc-" + utilrand.String(8)},
		Spec: v1.DatabaseConfigSpec{
			ServerRef:    "my-db-server",
			DatabaseName: "camunda",
			CredentialsSecretRef: v1.CredentialsSecretRef{
				Name: "my-camunda-db-credentials", Namespace: "my-cluster-ns",
				UsernameKey: "username", PasswordKey: "password",
			},
		},
	}
}

// SecondaryStorageConfigElasticsearch returns an Elasticsearch binding with a
// unique name in namespace. Its credentials Secret is <name>-credentials in
// the same namespace, with the keys username and password. The caller creates
// that Secret.
func SecondaryStorageConfigElasticsearch(namespace string) *v1.SecondaryStorageConfig {
	name := "ssc-" + utilrand.String(8)
	return &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1.SecondaryStorageConfigSpec{
			Type: v1.SecondaryStorageTypeElasticsearch,
			Elasticsearch: &v1.ElasticsearchStorage{
				Endpoint: "https://" + name + "-es-http." + namespace + ".svc:9200",
				CredentialsSecretRef: v1.CredentialsSecretRef{
					Name: name + "-credentials", Namespace: namespace,
					UsernameKey: "username", PasswordKey: "password",
				},
			},
		},
	}
}

// CamundaPlatformConfigBasic returns a platform config with basic
// authentication, no license, and no registry, with a unique name.
func CamundaPlatformConfigBasic() *v1.CamundaPlatformConfig {
	return &v1.CamundaPlatformConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cpc-" + utilrand.String(8)},
		Spec: v1.CamundaPlatformConfigSpec{
			Auth: &v1.PlatformAuthSpec{Method: v1.AuthenticationMethodBasic},
		},
	}
}
