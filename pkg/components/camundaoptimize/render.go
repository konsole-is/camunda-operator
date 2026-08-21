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
	"net/url"
	"strconv"

	corev1 "k8s.io/api/core/v1"

	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
)

// The environment variables of the Optimize container. Optimize does not read
// the unified configuration of the orchestration cluster: it resolves its own
// service-config.yaml, where every variable below appears as a placeholder.
// The source is
// optimize/util/optimize-commons/src/main/resources/service-config.yaml of
// the camunda/camunda repository at 8.9, unless another file is named.
// render_source_test.go proves each one against a local checkout.
const (
	// envDatabase selects the database that Optimize stores its own indices
	// in (ConfigurationServiceConstants.CAMUNDA_OPTIMIZE_DATABASE). Optimize
	// supports elasticsearch and opensearch; this operator attaches only to
	// Elasticsearch secondary storage.
	envDatabase = "CAMUNDA_OPTIMIZE_DATABASE"
	// The Elasticsearch connection. Optimize reads a list of nodes under
	// es.connection.nodes, and these two variables describe one node of that
	// list. The operator gives it the host and the port of the storage
	// endpoint.
	//
	// envElasticsearchHost is es.connection.nodes[*].host.
	envElasticsearchHost = "OPTIMIZE_ELASTICSEARCH_HOST"
	// envElasticsearchPort is es.connection.nodes[*].httpPort.
	envElasticsearchPort = "OPTIMIZE_ELASTICSEARCH_HTTP_PORT"
	// envElasticsearchUsername is es.security.username.
	envElasticsearchUsername = "CAMUNDA_OPTIMIZE_ELASTICSEARCH_SECURITY_USERNAME"
	// envElasticsearchPassword is es.security.password.
	envElasticsearchPassword = "CAMUNDA_OPTIMIZE_ELASTICSEARCH_SECURITY_PASSWORD"
	// envElasticsearchSSLEnabled is es.security.ssl.enabled. Optimize takes a
	// host and a port rather than a URL, so this variable, and not the
	// endpoint, decides whether the connection is TLS.
	envElasticsearchSSLEnabled = "CAMUNDA_OPTIMIZE_ELASTICSEARCH_SSL_ENABLED"
	// envElasticsearchCAs is es.security.ssl.certificate_authorities, a list
	// of paths to PEM encoded CA certificates. A plain string is read as a
	// one-element list (ConfigurationServiceTest
	// .certificateAuthorizationStringIsConvertedToList), so the mount path
	// goes in unquoted.
	envElasticsearchCAs = "CAMUNDA_OPTIMIZE_ELASTICSEARCH_SECURITY_SSL_CERTIFICATE_AUTHORITIES"
	// envElasticsearchSelfSigned is es.security.ssl.selfSigned. The CA is
	// supplied, so the certificate is verified against it like any other.
	envElasticsearchSelfSigned = "CAMUNDA_OPTIMIZE_ELASTICSEARCH_SECURITY_SSL_SELF_SIGNED"
	// envZeebeEnabled is zeebe.enabled: whether this instance imports the
	// exported records of the cluster.
	envZeebeEnabled = "CAMUNDA_OPTIMIZE_ZEEBE_ENABLED"
	// envZeebeName is zeebe.name: the index prefix of the exported records.
	// It must match the prefix of the exporter.
	envZeebeName = "CAMUNDA_OPTIMIZE_ZEEBE_NAME"
	// envZeebePartitionCount is zeebe.partitionCount: the partition count of
	// the cluster whose records are imported.
	envZeebePartitionCount = "CAMUNDA_OPTIMIZE_ZEEBE_PARTITION_COUNT"
	// envIdentityIssuerURL is security.auth.ccsm.issuerUrl: the issuer that
	// the browser is redirected to.
	envIdentityIssuerURL = "CAMUNDA_OPTIMIZE_IDENTITY_ISSUER_URL"
	// envIdentityIssuerBackendURL is security.auth.ccsm.issuerBackendUrl: the
	// issuer that the container reaches from inside the Kubernetes cluster.
	envIdentityIssuerBackendURL = "CAMUNDA_OPTIMIZE_IDENTITY_ISSUER_BACKEND_URL"
	// envIdentityBaseURL is security.auth.ccsm.baseUrl: the base URL of the
	// Management Identity service.
	envIdentityBaseURL = "CAMUNDA_OPTIMIZE_IDENTITY_BASE_URL"
	// envIdentityClientID is security.auth.ccsm.clientId.
	envIdentityClientID = "CAMUNDA_OPTIMIZE_IDENTITY_CLIENTID"
	// envIdentityClientSecret is security.auth.ccsm.clientSecret.
	envIdentityClientSecret = "CAMUNDA_OPTIMIZE_IDENTITY_CLIENTSECRET"
	// envIdentityAudience is security.auth.ccsm.audience.
	envIdentityAudience = "CAMUNDA_OPTIMIZE_IDENTITY_AUDIENCE"
)

// The literal values the renderer sets.
const (
	// profileCCSM is the Spring profile of a Self-Managed Optimize
	// (ConfigurationServiceConstants.CCSM_PROFILE). It activates
	// application-ccsm.yaml, which binds the Management Identity settings.
	profileCCSM = "ccsm"
	// databaseElasticsearch is the CAMUNDA_OPTIMIZE_DATABASE value of an
	// Elasticsearch-backed Optimize (ConfigurationServiceConstants
	// .ELASTICSEARCH_DATABASE_PROPERTY).
	databaseElasticsearch = "elasticsearch"
	// readinessPath is the readiness endpoint on the HTTP port
	// (HealthRestService.java: the REST API path plus /readyz). It answers 200
	// only while Optimize is connected to its database, so it also gates the
	// startup probe: the first start builds the Optimize index schema.
	readinessPath = "/api/readyz"
	// livenessPath is the Spring Boot health endpoint on the management port.
	// Optimize exposes it (management.endpoints.web.exposure.include in
	// optimize/backend/src/main/resources/application.properties) and
	// registers no HealthIndicator of its own, and its build pulls in no
	// spring-boot-starter-data-elasticsearch, so nothing puts an
	// Elasticsearch check into the aggregate. The liveness probe therefore
	// answers while Elasticsearch is unreachable, and an Elasticsearch outage
	// takes the pods out of the Service through readinessPath instead of
	// restarting them in a loop.
	livenessPath = "/actuator/health"
	// metricsPath is the Prometheus endpoint on the management port. Optimize
	// exposes the prometheus actuator endpoint under /actuator
	// (optimize/backend/src/main/resources/application.properties).
	metricsPath = "/actuator/prometheus"
	// defaultHTTPSPort and defaultHTTPPort are the ports of an endpoint URL
	// that names none.
	defaultHTTPSPort = "443"
	defaultHTTPPort  = "80"
)

// baseEnv renders the environment of one Optimize container: the profile and
// the database, the Elasticsearch connection, the Zeebe import, the Management
// Identity connection, and the license. importEnabled turns the import of the
// exported cluster records on; the importer runs with it on and the webapp
// with it off, so only one instance imports.
func baseEnv(in Input, importEnabled bool) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{Name: camundaconfig.EnvSpringProfilesActive, Value: profileCCSM},
		{Name: envDatabase, Value: databaseElasticsearch},
	}
	env = append(env, elasticsearchEnv(in)...)
	env = append(env, zeebeEnv(in, importEnabled)...)
	env = append(env, identityEnv(in)...)

	if ref := in.Platform.LicenseSecretRef; ref != nil {
		env = append(env, corev1.EnvVar{
			Name:      camundaconfig.EnvLicenseKey,
			ValueFrom: secretSource(ref.Name, ref.Key),
		})
	}

	return env
}

// elasticsearchEnv renders the Elasticsearch connection: the host and the port
// of the endpoint, the basic-auth credentials, and the TLS settings. Optimize
// takes the host and the port apart, so the scheme of the endpoint becomes the
// TLS switch.
func elasticsearchEnv(in Input) []corev1.EnvVar {
	host, port, secure := endpointParts(in.Storage.Endpoint)
	creds := in.Storage.CredentialsSecretRef

	env := []corev1.EnvVar{
		{Name: envElasticsearchHost, Value: host},
		{Name: envElasticsearchPort, Value: port},
		{Name: envElasticsearchUsername, ValueFrom: secretSource(creds.Name, creds.UsernameKey)},
		{Name: envElasticsearchPassword, ValueFrom: secretSource(creds.Name, creds.PasswordKey)},
		{Name: envElasticsearchSSLEnabled, Value: strconv.FormatBool(secure)},
	}

	if ca := in.Storage.CASecretRef; ca != nil {
		env = append(
			env,
			corev1.EnvVar{Name: envElasticsearchCAs, Value: CAMountPath + "/" + ca.Key},
			corev1.EnvVar{Name: envElasticsearchSelfSigned, Value: "false"},
		)
	}

	return env
}

// zeebeEnv renders the import of the exported cluster records: the switch, the
// index prefix, and the partition count.
func zeebeEnv(in Input, importEnabled bool) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: envZeebeEnabled, Value: strconv.FormatBool(importEnabled)},
		{Name: envZeebeName, Value: ZeebeRecordPrefix},
		{Name: envZeebePartitionCount, Value: strconv.Itoa(int(in.Partitions))},
	}
}

// identityEnv renders the Management Identity connection. The backend issuer
// URL falls back to the issuer URL, which is what the contract documents for
// an empty issuerBackendUrl.
func identityEnv(in Input) []corev1.EnvVar {
	auth := in.Auth.Spec
	backend := auth.IssuerBackendURL
	if backend == "" {
		backend = auth.IssuerURL
	}

	return []corev1.EnvVar{
		{Name: envIdentityIssuerURL, Value: auth.IssuerURL},
		{Name: envIdentityIssuerBackendURL, Value: backend},
		{Name: envIdentityBaseURL, Value: auth.BaseURL},
		{Name: envIdentityClientID, Value: auth.ClientID},
		{
			Name:      envIdentityClientSecret,
			ValueFrom: secretSource(auth.ClientSecretRef.Name, auth.ClientSecretRef.Key),
		},
		{Name: envIdentityAudience, Value: auth.Audience},
	}
}

// endpointParts splits an Elasticsearch endpoint into the host, the port, and
// whether the connection is TLS. A URL without a port takes the default port
// of its scheme. An endpoint that does not parse yields empty parts; CEL on
// the storage contract rejects one, so this only guards the zero value.
func endpointParts(endpoint string) (host, port string, secure bool) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", "", false
	}

	secure = parsed.Scheme == "https"
	port = parsed.Port()
	if port == "" {
		port = defaultHTTPPort
		if secure {
			port = defaultHTTPSPort
		}
	}

	return parsed.Hostname(), port, secure
}

// caVolumes returns the volume that carries the Elasticsearch CA, or nothing
// when the storage contract names none.
func caVolumes(in Input) []corev1.Volume {
	ca := in.Storage.CASecretRef
	if ca == nil {
		return nil
	}

	return []corev1.Volume{{
		Name: caVolumeName,
		VolumeSource: corev1.VolumeSource{Secret: &corev1.SecretVolumeSource{
			SecretName: ca.Name,
			Items:      []corev1.KeyToPath{{Key: ca.Key, Path: ca.Key}},
		}},
	}}
}

// caMounts returns the mount of the Elasticsearch CA volume, or nothing when
// the storage contract names no CA.
func caMounts(in Input) []corev1.VolumeMount {
	if in.Storage.CASecretRef == nil {
		return nil
	}

	return []corev1.VolumeMount{{Name: caVolumeName, MountPath: CAMountPath, ReadOnly: true}}
}

// secretSource builds the source of an environment variable that reads one key
// of a Secret in the pod's namespace. Every reference in Input already points
// at a Secret of that namespace, because the controller copies the ones that
// live elsewhere.
func secretSource(name, key string) *corev1.EnvVarSource {
	return &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{
		LocalObjectReference: corev1.LocalObjectReference{Name: name},
		Key:                  key,
	}}
}
