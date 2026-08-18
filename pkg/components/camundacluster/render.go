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
	"fmt"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
)

// The literal values the renderer sets. Each one comes from the source named
// next to it.
const (
	// grpcBindAddress binds the gRPC server to every interface
	// (defaults.yaml:15 documents the address property).
	grpcBindAddress = "0.0.0.0"
	// javaToolOptions makes the JVM exit on OutOfMemoryError, so the kubelet
	// restarts the pod instead of leaving a broken process (helm chart
	// 14.8.3 values.yaml:2697-2701 sets the same flag).
	javaToolOptions = "-XX:+ExitOnOutOfMemoryError"
	// adminEmail is the email of the seeded admin user; ConfiguredUser.java
	// requires none, and no mail is ever sent.
	adminEmail = "admin@localhost"
	// jdbcPostgres is the JDBC URL prefix of the bundled PostgreSQL driver
	// (dist/pom.xml:356-359).
	jdbcPostgres = "jdbc:postgresql://"
	// vendorPostgres is the database-vendor-id of PostgreSQL
	// (RdbmsDatabaseIdProvider.java:18-25).
	vendorPostgres = "postgresql"
	// clientModeSelfManaged is the camunda.client.mode of a self-managed
	// cluster (CamundaClientProperties.java:341-344).
	clientModeSelfManaged = "selfManaged"
	// ssoCallbackPath is the OIDC redirect path of the binary
	// (WebSecurityConfig.java:154, ClientRegistrationFactory.java:26).
	ssoCallbackPath = "/sso-callback"
	// caVolumeName is the volume that carries the Elasticsearch CA.
	caVolumeName = "es-ca"
	// nodeIDCommand exports the broker node id from the pod ordinal before
	// it starts the binary; camunda.cluster.node-id has no ordinal provider
	// (NodeIdProvider.java:274-277). The Helm chart derives the id from the
	// pod name the same way (templates/orchestration/configmap.yaml:9-12).
	nodeIDCommand = "export %s=${HOSTNAME##*-}; exec " + CamundaEntrypoint
)

// rendered is the environment, volumes, and mounts of one process.
type rendered struct {
	env     []corev1.EnvVar
	envFrom []corev1.EnvFromSource
	volumes []corev1.Volume
	mounts  []corev1.VolumeMount
	// command is the node-id wrapper of the brokers; nil for every other
	// process, which runs the image entrypoint.
	command []string
}

// render produces the environment of a process in layers; a later layer wins
// by name: cluster identity, secondary storage, authentication and platform
// settings, process role, then user overrides (global extraEnv, then the
// extraEnv of the embedded gateway and web applications the process hosts,
// then the extraEnv of the process's own component). Connectors get only the client
// layer, the license, and the user overrides.
func render(in Input, p Process) rendered {
	if p.Component == ComponentConnectors {
		return rendered{
			env:     dedupeEnv(append(connectorsEnv(in), userEnv(in, p)...)),
			envFrom: userEnvFrom(in, p),
		}
	}

	r := storageEnv(in)
	backup := backupEnv(in, p)
	r.volumes = append(r.volumes, backup.volumes...)
	r.mounts = append(r.mounts, backup.mounts...)

	env := identityEnv(in, p)
	env = append(env, r.env...)
	env = append(env, backup.env...)
	env = append(env, authEnv(in)...)
	env = append(env, roleEnv(p)...)
	env = append(env, userEnv(in, p)...)
	r.env = dedupeEnv(env)
	r.envFrom = userEnvFrom(in, p)

	if p.Component == ComponentZeebe {
		r.command = []string{"bash", "-c", fmt.Sprintf(nodeIDCommand, camundaconfig.KeyClusterNodeID.Env())}
	}

	return r
}

// identityEnv is the cluster identity layer: name, size, partitions,
// replication factor, the contact points of every broker, the gRPC bind
// address, and the gateway id of every Deployment-backed unified process
// (the gateway and the standalone web applications, which run the gateway
// code path).
func identityEnv(in Input, p Process) []corev1.EnvVar {
	e := in.Effective
	env := []corev1.EnvVar{
		camundaconfig.Var(camundaconfig.KeyClusterName, in.Cluster.Name),
		camundaconfig.Var(camundaconfig.KeyClusterSize, strconv.Itoa(int(e.ZeebeReplicas()))),
		camundaconfig.Var(camundaconfig.KeyClusterPartitionCount, strconv.Itoa(int(e.Partitions()))),
		camundaconfig.Var(camundaconfig.KeyClusterReplicationFactor, strconv.Itoa(int(e.ReplicationFactor()))),
		camundaconfig.Var(camundaconfig.KeyClusterInitialContacts, strings.Join(contactPoints(in), ",")),
		camundaconfig.Var(camundaconfig.KeyGRPCAddress, grpcBindAddress),
	}

	if p.Kind == ProcessDeployment {
		env = append(env, camundaconfig.VarFrom(camundaconfig.KeyClusterGatewayID, &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"},
		}))
	}

	return env
}

// contactPoints returns the internal address of every broker pod through the
// headless zeebe Service.
func contactPoints(in Input) []string {
	zeebe := WorkloadName(in.Cluster, ComponentZeebe)
	points := make([]string, 0, in.Effective.ZeebeReplicas())
	for i := range in.Effective.ZeebeReplicas() {
		points = append(
			points,
			zeebe+"-"+strconv.Itoa(int(i))+"."+zeebe+"."+in.Cluster.Namespace+".svc:"+strconv.Itoa(int(PortInternal)),
		)
	}
	return points
}

// storageEnv is the secondary storage layer. It returns the volume and mount
// of the Elasticsearch CA when the binding names one.
func storageEnv(in Input) rendered {
	var r rendered
	r.env = append(r.env, camundaconfig.Var(camundaconfig.KeySecondaryStorageType, string(in.Storage.Type)))

	switch {
	case in.Storage.Type == v1.SecondaryStorageTypeElasticsearch && in.Storage.Elasticsearch != nil:
		es := in.Storage.Elasticsearch
		r.env = append(
			r.env,
			camundaconfig.Var(camundaconfig.KeyElasticsearchURL, es.Endpoint),
			camundaconfig.VarFrom(
				camundaconfig.KeyElasticsearchUsername,
				secretSource(es.CredentialsSecretRef.Name, es.CredentialsSecretRef.UsernameKey),
			),
			camundaconfig.VarFrom(
				camundaconfig.KeyElasticsearchPassword,
				secretSource(es.CredentialsSecretRef.Name, es.CredentialsSecretRef.PasswordKey),
			),
		)
		if es.CASecretRef != nil {
			r.env = append(
				r.env,
				camundaconfig.Var(camundaconfig.KeyElasticsearchSecurityEnabled, "true"),
				camundaconfig.Var(
					camundaconfig.KeyElasticsearchSecurityCertificatePath,
					CAMountPath+"/"+es.CASecretRef.Key,
				),
				camundaconfig.Var(camundaconfig.KeyElasticsearchSecurityVerifyHostname, "true"),
				camundaconfig.Var(camundaconfig.KeyElasticsearchSecuritySelfSigned, "false"),
			)
			r.volumes = []corev1.Volume{{
				Name: caVolumeName,
				VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
					SecretName: es.CASecretRef.Name,
					Items:      []corev1.KeyToPath{{Key: es.CASecretRef.Key, Path: es.CASecretRef.Key}},
				}},
			}}
			r.mounts = []corev1.VolumeMount{{Name: caVolumeName, MountPath: CAMountPath, ReadOnly: true}}
		}
	case in.Storage.Type == v1.SecondaryStorageTypeRDBMS && in.Storage.RDBMS != nil:
		db := in.Storage.RDBMS
		r.env = append(
			r.env,
			camundaconfig.Var(camundaconfig.KeyRDBMSURL, RDBMSURL(db.Host, db.Port, db.Database)),
			camundaconfig.VarFrom(
				camundaconfig.KeyRDBMSUsername,
				secretSource(db.Credentials.Name, db.Credentials.UsernameKey),
			),
			camundaconfig.VarFrom(
				camundaconfig.KeyRDBMSPassword,
				secretSource(db.Credentials.Name, db.Credentials.PasswordKey),
			),
			camundaconfig.Var(camundaconfig.KeyRDBMSDatabaseVendorID, vendorPostgres),
		)
	}

	return r
}

// authEnv is the authentication and platform layer: the method, the OIDC
// connection or the seeded admin user, and the license.
func authEnv(in Input) []corev1.EnvVar {
	auth := ResolveAuth(in)
	env := []corev1.EnvVar{camundaconfig.Var(camundaconfig.KeyAuthenticationMethod, string(auth.Method))}

	if auth.OIDC != nil {
		env = append(env, oidcEnv(in, auth)...)
	} else {
		env = append(env, adminUserEnv(in)...)
	}

	if in.Platform.LicenseSecretRef != nil {
		env = append(env, camundaconfig.VarFrom(
			camundaconfig.KeyLicenseKey,
			secretSource(in.Platform.LicenseSecretRef.Name, in.Platform.LicenseSecretRef.Key),
		))
	}

	return env
}

// oidcEnv renders the OIDC connection and the role membership that
// authorizes callers. The discovery endpoints and the claim names are set
// only when the platform config carries them, and the redirect only when the
// cluster has an externalUrl; otherwise the binary derives them.
func oidcEnv(in Input, auth EffectiveAuth) []corev1.EnvVar {
	oidc := auth.OIDC
	env := []corev1.EnvVar{
		camundaconfig.Var(camundaconfig.KeyOIDCIssuerURI, oidc.IssuerURL),
		camundaconfig.Var(camundaconfig.KeyOIDCClientID, oidc.ClientID),
		camundaconfig.VarFrom(
			camundaconfig.KeyOIDCClientSecret,
			secretSource(oidc.ClientSecretRef.Name, oidc.ClientSecretRef.Key),
		),
		camundaconfig.Var(camundaconfig.KeyOIDCAudiences, oidc.Audience),
	}

	optional := []struct {
		key   camundaconfig.Key
		value string
	}{
		{camundaconfig.KeyOIDCJWKSetURI, oidc.JWKSURL},
		{camundaconfig.KeyOIDCTokenURI, oidc.TokenURL},
		{camundaconfig.KeyOIDCAuthorizationURI, oidc.AuthURL},
		{camundaconfig.KeyOIDCUsernameClaim, oidc.UsernameClaim},
		{camundaconfig.KeyOIDCClientIDClaim, oidc.ClientIDClaim},
	}
	for _, o := range optional {
		if o.value != "" {
			env = append(env, camundaconfig.Var(o.key, o.value))
		}
	}

	if externalURL := strings.TrimRight(in.Cluster.Spec.ExternalURL, "/"); externalURL != "" {
		env = append(env, camundaconfig.Var(camundaconfig.KeyOIDCRedirectURI, externalURL+ssoCallbackPath))
	}

	env = append(env, adminRoleEnv(auth.Admin)...)

	return append(env, connectorsRoleEnv(in, oidc)...)
}

// adminRoleEnv makes every member of admin a member of the admin role, and
// declares the mapping rules that the block references. Camunda seeds no
// administrator under OIDC, so an empty block leaves the cluster without one.
func adminRoleEnv(admin *v1.ClusterAdminSpec) []corev1.EnvVar {
	if admin == nil {
		return nil
	}

	var env []corev1.EnvVar
	for i, user := range admin.Users {
		env = append(env, camundaconfig.Var(
			camundaconfig.Index(camundaconfig.KeyDefaultRolesAdminUsers, i, ""), user,
		))
	}

	for i, client := range admin.Clients {
		env = append(env, camundaconfig.Var(
			camundaconfig.Index(camundaconfig.KeyDefaultRolesAdminClients, i, ""), client,
		))
	}

	// The rule definition and its membership carry the same index, so both
	// come from one pass over the slice.
	for i, rule := range admin.MappingRules {
		env = append(
			env,
			camundaconfig.Var(
				camundaconfig.Index(camundaconfig.KeyDefaultRolesAdminMappingRules, i, ""), rule.ID,
			),
			camundaconfig.Var(
				camundaconfig.Index(camundaconfig.KeyInitializationMappingRules, i, "mapping-rule-id"), rule.ID,
			),
			camundaconfig.Var(
				camundaconfig.Index(camundaconfig.KeyInitializationMappingRules, i, "claim-name"), rule.ClaimName,
			),
			camundaconfig.Var(
				camundaconfig.Index(camundaconfig.KeyInitializationMappingRules, i, "claim-value"), rule.ClaimValue,
			),
		)
	}

	return env
}

// connectorsRoleEnv gives the connectors role to the OIDC client of the
// cluster, which is the client the connectors runtime authenticates with.
// Without a client id claim that runtime becomes a person, and a client member
// would never match it, so the grant is left out.
func connectorsRoleEnv(in Input, oidc *v1.OIDCSpec) []corev1.EnvVar {
	if oidc.ClientIDClaim == "" || !in.Effective.ConnectorsEnabled() {
		return nil
	}

	return []corev1.EnvVar{camundaconfig.Var(
		camundaconfig.Index(camundaconfig.KeyDefaultRolesConnectorsClients, 0, ""), oidc.ClientID,
	)}
}

// adminUserEnv seeds the initial admin user of a basic-auth cluster from
// the admin Secret and makes it a member of the admin role. The binary seeds
// no user on its own (InitializationConfiguration.java:23).
func adminUserEnv(in Input) []corev1.EnvVar {
	users := camundaconfig.KeyInitializationUsers
	return []corev1.EnvVar{
		camundaconfig.Var(camundaconfig.Index(users, 0, "username"), AdminUsername),
		camundaconfig.VarFrom(
			camundaconfig.Index(users, 0, "password"),
			secretSource(AdminSecretName(in.Cluster), AdminPasswordKey),
		),
		camundaconfig.Var(camundaconfig.Index(users, 0, "name"), AdminUsername),
		camundaconfig.Var(camundaconfig.Index(users, 0, "email"), adminEmail),
		camundaconfig.Var(camundaconfig.Index(camundaconfig.KeyDefaultRolesAdminUsers, 0, ""), AdminUsername),
	}
}

// roleEnv is the process role layer: the Spring profiles, the embedded
// gateway switch of the brokers, and the JVM options of every unified
// process.
func roleEnv(p Process) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{Name: camundaconfig.EnvSpringProfilesActive, Value: strings.Join(p.Profiles, ",")},
		{Name: camundaconfig.EnvJavaToolOptions, Value: javaToolOptions},
	}
	if p.Component == ComponentZeebe {
		env = append(
			env,
			camundaconfig.Var(camundaconfig.KeyBrokerGatewayEnable, strconv.FormatBool(p.EmbeddedGateway)),
		)
	}
	return env
}

// connectorsEnv is the client layer of the connectors runtime: the mode, the
// gateway addresses, the credentials of the admin user or the OIDC client,
// and the license.
func connectorsEnv(in Input) []corev1.EnvVar {
	host := GatewayHost(in.Cluster, in.Effective) + "." + in.Cluster.Namespace + ".svc"
	env := []corev1.EnvVar{
		camundaconfig.Var(camundaconfig.KeyClientMode, clientModeSelfManaged),
		camundaconfig.Var(camundaconfig.KeyClientGRPCAddress, "http://"+host+":"+strconv.Itoa(int(PortGRPC))),
		camundaconfig.Var(camundaconfig.KeyClientRESTAddress, "http://"+host+":"+strconv.Itoa(int(PortHTTP))),
	}

	auth := ResolveAuth(in)
	env = append(env, camundaconfig.Var(camundaconfig.KeyClientAuthMethod, string(auth.Method)))
	if auth.OIDC != nil {
		env = append(
			env,
			camundaconfig.Var(camundaconfig.KeyClientAuthClientID, auth.OIDC.ClientID),
			camundaconfig.VarFrom(
				camundaconfig.KeyClientAuthClientSecret,
				secretSource(auth.OIDC.ClientSecretRef.Name, auth.OIDC.ClientSecretRef.Key),
			),
			camundaconfig.Var(camundaconfig.KeyClientAuthIssuerURL, auth.OIDC.IssuerURL),
			camundaconfig.Var(camundaconfig.KeyClientAuthAudience, auth.OIDC.Audience),
		)
	} else {
		env = append(
			env,
			camundaconfig.Var(camundaconfig.KeyClientAuthUsername, AdminUsername),
			camundaconfig.VarFrom(
				camundaconfig.KeyClientAuthPassword,
				secretSource(AdminSecretName(in.Cluster), AdminPasswordKey),
			),
		)
	}

	if in.Platform.LicenseSecretRef != nil {
		env = append(env, corev1.EnvVar{
			Name:      camundaconfig.EnvLicenseKeyConnectors,
			ValueFrom: secretSource(in.Platform.LicenseSecretRef.Name, in.Platform.LicenseSecretRef.Key),
		})
	}

	return env
}

// envSources returns the component names whose extraEnv and extraEnvFrom
// apply to a process, in the order they are layered: the embedded gateway
// (on the brokers), the embedded web applications the process hosts, then
// the process's own component.
func envSources(p Process) []string {
	sources := []string{}
	if p.EmbeddedGateway {
		sources = append(sources, ComponentGateway)
	}
	for _, app := range webApps {
		if app.component != p.Component && slices.Contains(p.Profiles, app.profile) {
			sources = append(sources, app.component)
		}
	}
	return append(sources, p.Component)
}

// userEnv is the user override layer: the global extraEnv, then the extraEnv
// of every component that envSources names. dedupeEnv makes the last one
// win.
func userEnv(in Input, p Process) []corev1.EnvVar {
	env := slices.Clone(in.Effective.ExtraEnv)
	for _, component := range envSources(p) {
		env = append(env, in.Effective.Workload(component).ExtraEnv...)
	}
	return env
}

// userEnvFrom concatenates the global extraEnvFrom and the extraEnvFrom of
// every component that envSources names.
func userEnvFrom(in Input, p Process) []corev1.EnvFromSource {
	envFrom := slices.Clone(in.Effective.ExtraEnvFrom)
	for _, component := range envSources(p) {
		envFrom = append(envFrom, in.Effective.Workload(component).ExtraEnvFrom...)
	}
	return envFrom
}

// dedupeEnv keeps the last occurrence of every name and preserves the order
// of first appearance.
func dedupeEnv(env []corev1.EnvVar) []corev1.EnvVar {
	last := make(map[string]corev1.EnvVar, len(env))
	order := make([]string, 0, len(env))
	for _, e := range env {
		if _, seen := last[e.Name]; !seen {
			order = append(order, e.Name)
		}
		last[e.Name] = e
	}

	out := make([]corev1.EnvVar, 0, len(order))
	for _, name := range order {
		out = append(out, last[name])
	}
	return out
}

// secretSource builds the source of an environment variable that reads one
// key of a Secret in the pod's namespace.
func secretSource(name, key string) *corev1.EnvVarSource {
	return &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: name},
		Key:                  key,
	}}
}

// RDBMSURL returns the JDBC URL that the orchestration cluster is configured
// with for its relational secondary storage: the value the rendered pod
// template carries under camundaconfig.KeyRDBMSURL. It is the one definition
// of that value, so a consumer that reads it from a live template — the
// RDBMS backup, to prove the dump targets the database Zeebe runs — compares
// against the same rendering.
func RDBMSURL(host string, port int32, database string) string {
	return jdbcPostgres + host + ":" + strconv.Itoa(int(port)) + "/" + database
}
