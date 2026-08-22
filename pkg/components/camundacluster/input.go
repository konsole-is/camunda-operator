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

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"

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
	// Holder is set when another CamundaCluster, created earlier, uses the
	// backend of this binding. One CamundaCluster uses one backend, so the
	// controller renders a cluster with a Holder suspended and reports the
	// holder on Ready. Nil when this cluster holds its backend.
	Holder *StorageHolder
}

// StorageHolder is the CamundaCluster that holds a secondary storage backend
// and the backend it holds, as Backend.String of the contract wrapper names
// it.
type StorageHolder struct {
	Cluster types.NamespacedName
	Backend string
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
	// Backup is the ObjectStorageConfig that spec.backupStorageRef names,
	// with its credentials reference already pointed at the copy in the
	// cluster namespace. Nil when the cluster names no backup bucket; the
	// backup wiring is then not rendered and the store stays NONE.
	Backup *v1.ObjectStorageConfig
	// Documents is the ObjectStorageConfig that spec.documentStorageRef
	// names. Its workload identity and pod labels bind like the backup
	// bucket's; nothing else of it is wired yet.
	Documents *v1.ObjectStorageConfig
	// ServiceAccountAnnotations are the workload-identity annotations that
	// the referenced buckets derive, already checked for a conflict by the
	// controller. The annotations of spec.serviceAccount merge over them, so
	// an explicit user value on the same key wins.
	ServiceAccountAnnotations map[string]string
	// VolumeClaimSize is the storage request of the broker volume claim
	// template. A StatefulSet cannot change its claim template, so the
	// controller sets it to the size of the applied template. When nil, the
	// template requests the effective storage size.
	VolumeClaimSize *resource.Quantity
	// HashInputs are the resource versions and generations of the referenced
	// Secrets and custom resources, as "kind/namespace/name=version" strings.
	// ConfigHash sorts them, so the order does not matter.
	HashInputs []string
	// AdminPasswordHash is PasswordHash of the admin password that the admin
	// Secret of a basic-auth cluster publishes, or "" under OIDC. It is the
	// password the Secret already holds, never one that this reconcile is
	// promoting: connectors resolve the Secret when a pod starts, so a hash
	// that ran ahead of a Secret write that then failed would never roll
	// them again. A cluster whose Secret does not exist yet is the
	// exception, and the controller passes the password it is about to
	// publish there: a failed apply leaves no Secret at all, and the next
	// reconcile generates another password, so the hash keeps moving until
	// one lands. Only the hash of connectors consumes it; see ConfigHash.
	AdminPasswordHash string
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
	// Admin holds the members of the admin role. It comes from the effective
	// cluster auth and is set only when Method is oidc, because basic
	// authentication seeds its own administrator.
	Admin *v1.ClusterAdminSpec
	// Basic is the basic block of the effective cluster auth. It is set only
	// when Method is basic, because OIDC has no operator-owned credential.
	Basic *v1.BasicAuthSpec
}

// ResolveAdminEmail returns the address of the seeded admin user that auth
// asks for, or DefaultAdminEmail when it names none. It is meaningful under
// basic authentication only; OIDC seeds no user.
func ResolveAdminEmail(auth EffectiveAuth) string {
	if auth.Basic != nil && auth.Basic.AdminEmail != "" {
		return auth.Basic.AdminEmail
	}

	return DefaultAdminEmail
}

// ResolveAuth layers the authentication settings: the platform config gives
// the method and the identity provider connection, the effective cluster
// auth (preset then cluster) overrides the client id, the audience, and the
// client secret reference, and provides the members of the admin role.
// Under basic authentication the effective cluster auth provides the basic
// block instead. The platform spec is not mutated.
func ResolveAuth(in Input) EffectiveAuth {
	auth := EffectiveAuth{Method: in.Platform.Method()}
	if auth.Method != v1.AuthenticationMethodOIDC {
		if in.Effective.Auth != nil {
			auth.Basic = in.Effective.Auth.Basic
		}
		return auth
	}
	if in.Platform.Auth == nil || in.Platform.Auth.OIDC == nil {
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
		auth.Admin = override.Admin
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

	if registry := strings.TrimRight(in.Platform.ImageRegistry, "/"); registry != "" {
		return registry + "/" + image
	}
	return image
}
