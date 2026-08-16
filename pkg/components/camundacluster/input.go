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

package camundacluster

import (
	"strings"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// Storage is the resolved secondary storage binding, filled by the
// controller from the SecondaryStorageConfig that spec.storageRef names.
type Storage struct {
	// Type selects which of the two blocks is set.
	Type v1.SecondaryStorageType
	// Elasticsearch is set when Type is elasticsearch.
	Elasticsearch *v1.ElasticsearchStorage
	// RDBMS is set when Type is rdbms.
	RDBMS *RDBMSStorage
}

// RDBMSStorage is the DatabaseConfig and DatabaseServerConfig chain of an
// rdbms binding, flattened to what the JDBC URL and the credentials need.
type RDBMSStorage struct {
	// Host and Port come from the DatabaseServerConfig.
	Host string
	Port int32
	// Database is the databaseName of the DatabaseConfig.
	Database string
	// Credentials is the credentialsSecretRef of the DatabaseConfig.
	Credentials v1.CredentialsSecretRef
}

// Input is everything the pure package needs to render one cluster.
type Input struct {
	// Cluster is the CamundaCluster as it was read. Only its name,
	// namespace, and instance-bound spec fields (externalUrl, extraEnv) are
	// read directly; the topology comes from Effective.
	Cluster *v1.CamundaCluster
	// Effective is the preset-merged spec with the defaults applied.
	Effective Effective
	// Platform is the spec of the referenced CamundaPlatformConfig.
	Platform v1.CamundaPlatformConfigSpec
	// Storage is the resolved secondary storage binding.
	Storage Storage
	// HashInputs are the resource versions and generations of the referenced
	// Secrets and custom resources, as "kind/namespace/name=version" strings.
	// ConfigHash sorts them, so the order does not matter.
	HashInputs []string
	// ServiceMonitorSupported reports whether the Kubernetes cluster serves
	// the ServiceMonitor kind. When false, no ServiceMonitor is rendered.
	ServiceMonitorSupported bool
}

// EffectiveAuth is the authentication source after the layering of the
// platform config, the preset auth, and the cluster auth.
type EffectiveAuth struct {
	// Method is basic or oidc.
	Method v1.AuthenticationMethod
	// OIDC is set when Method is oidc. The issuer and endpoint fields come
	// from the platform config. The client id, the audience, and the client
	// secret reference come from the cluster auth (already merged with the
	// preset auth) when set, otherwise from the platform config. The
	// audience defaults to the client id.
	OIDC *v1.OIDCSpec
}

// ResolveAuth layers the authentication settings: the platform config gives
// the method and the identity provider connection, the effective cluster
// auth (preset then cluster) overrides the client id, the audience, and the
// client secret reference. The platform spec is not mutated.
func ResolveAuth(in Input) EffectiveAuth {
	auth := EffectiveAuth{Method: in.Platform.Method()}
	if auth.Method != v1.AuthenticationMethodOIDC || in.Platform.Auth == nil || in.Platform.Auth.OIDC == nil {
		return auth
	}

	oidc := in.Platform.Auth.OIDC.DeepCopy()
	if override := in.Effective.Auth; override != nil {
		if override.ClientID != "" {
			oidc.ClientID = override.ClientID
			// The platform audience belongs to the platform client id.
			oidc.Audience = ""
		}
		if override.Audience != "" {
			oidc.Audience = override.Audience
		}
		if override.ClientSecretRef != nil {
			oidc.ClientSecretRef = *override.ClientSecretRef
		}
	}
	if oidc.Audience == "" {
		oidc.Audience = oidc.ClientID
	}
	auth.OIDC = oidc

	return auth
}

// Image returns the container image of a process: the platform registry
// prefix, when set, in front of camunda/camunda:<version> for a unified
// process or camunda/connectors-bundle:<connectors.version> for connectors.
func Image(in Input, p Process) string {
	image := CamundaImage + ":" + in.Effective.Version
	if p.Component == ComponentConnectors {
		version := ""
		if in.Effective.Connectors != nil {
			version = in.Effective.Connectors.Version
		}
		image = ConnectorsImage + ":" + version
	}

	if registry := strings.TrimSuffix(in.Platform.ImageRegistry, "/"); registry != "" {
		return registry + "/" + image
	}
	return image
}
