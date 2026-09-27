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

package camundaoptimize

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

const (
	// schemaTestNamespace hosts the throwaway objects that the admission
	// specs create.
	schemaTestNamespace = "default"
)

// newSecondaryStorageConfigElasticsearch returns an Elasticsearch binding
// with a unique name in namespace. Its credentials Secret is
// <name>-credentials in the same namespace, with the keys username and
// password. The caller creates that Secret.
func newSecondaryStorageConfigElasticsearch(namespace string) *v1.SecondaryStorageConfig {
	name := "ssc-" + utilrand.String(8)
	return &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
		Spec: v1.SecondaryStorageConfigSpec{
			Type: v1.SecondaryStorageTypeElasticsearch,
			Elasticsearch: &v1.ElasticsearchStorage{
				Endpoint: "https://" + name + "-es-http." + namespace + ".svc:9200",
				CredentialsSecretRef: v1.LocalCredentialsSecretRef{
					Name: name + "-credentials", UsernameKey: "username", PasswordKey: "password",
				},
			},
		},
	}
}

// newCamundaPlatformConfigBasic returns a platform config with basic
// authentication, no license, and no registry, with a unique name.
func newCamundaPlatformConfigBasic() *v1.CamundaPlatformConfig {
	return &v1.CamundaPlatformConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "cpc-" + utilrand.String(8)},
		Spec: v1.CamundaPlatformConfigSpec{
			Auth: &v1.PlatformAuthSpec{Method: v1.AuthenticationMethodBasic},
		},
	}
}
