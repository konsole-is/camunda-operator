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

package camundamanagementcluster

import (
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The settings of one cluster in the Web Modeler cluster list. Web Modeler
// numbers the entries from zero, so every name below carries the index of its
// cluster
// (https://docs.camunda.io/docs/self-managed/components/modeler/web-modeler/configuration/#clusters).
const (
	clusterEnvPrefix = "CAMUNDA_MODELER_CLUSTERS_"
	// The identity of the cluster, common to every cluster version.
	clusterEnvID      = "_ID"
	clusterEnvName    = "_NAME"
	clusterEnvVersion = "_VERSION"
	// clusterEnvAuthentication is how Web Modeler authenticates its calls.
	clusterEnvAuthentication = "_AUTHENTICATION"
	// The addresses of a cluster of version 8.8 or later. The gRPC and the
	// REST address are reached from inside the Kubernetes cluster; the web
	// application address is the one a browser follows.
	clusterEnvURLGRPC   = "_URL_GRPC"
	clusterEnvURLREST   = "_URL_REST"
	clusterEnvURLWebapp = "_URL_WEBAPP"
	// clusterEnvAuthorizationsEnabled makes Web Modeler warn a person that
	// their own authorizations decide what a deployment may do.
	clusterEnvAuthorizationsEnabled = "_AUTHORIZATIONS_ENABLED"
)

// The authentication methods that Web Modeler offers for a cluster of version
// 8.8 or later, from "Available authentication methods" on the configuration
// page:
// https://docs.camunda.io/docs/self-managed/components/modeler/web-modeler/configuration/
const (
	// clusterAuthBearerToken sends the token of the person who is signed in.
	// It needs a cluster on the identity provider of the management plane.
	clusterAuthBearerToken = "BEARER_TOKEN"
	// clusterAuthBasic sends a user name and a password. Web Modeler asks the
	// person for them in the deploy dialog; no setting carries them.
	clusterAuthBasic = "BASIC"
)

// grpcScheme is what Web Modeler expects in front of a gRPC address. The
// gateway binding of a CamundaCluster publishes a host and a port, so the
// scheme is added here.
const grpcScheme = "grpc://"

// clustersEnv renders the cluster list of Web Modeler: one numbered block per
// cluster that Web Modeler can deploy to, in the order of clusters.
//
// A cluster that publishes no gateway endpoints is left out, and the numbering
// closes over it, because Web Modeler stops reading at the first index it does
// not find. Such a cluster appears in status.clusters with the reason NotReady
// instead.
//
// An oidc cluster takes the token of the person who is signed in. A basic-auth
// cluster takes a user name and a password that Web Modeler asks the person
// for; the operator publishes a user for that in
// WebModelerClusterUserSecretName, because no setting can carry them.
func clustersEnv(clusters []AttachedCluster) []corev1.EnvVar {
	var env []corev1.EnvVar
	index := 0
	for _, cluster := range clusters {
		if cluster.GRPCEndpoint == "" || cluster.RESTEndpoint == "" {
			continue
		}
		env = append(env, clusterEnv(cluster, index)...)
		index++
	}

	return env
}

// clusterEnv renders one numbered block of the cluster list.
func clusterEnv(cluster AttachedCluster, index int) []corev1.EnvVar {
	prefix := clusterEnvPrefix + strconv.Itoa(index)

	env := []corev1.EnvVar{
		{Name: prefix + clusterEnvID, Value: string(cluster.UID)},
		{Name: prefix + clusterEnvName, Value: cluster.Namespace + "/" + cluster.Name},
		{Name: prefix + clusterEnvVersion, Value: cluster.Version},
		{Name: prefix + clusterEnvAuthentication, Value: clusterAuthentication(cluster.AuthMethod)},
		{Name: prefix + clusterEnvURLGRPC, Value: grpcURL(cluster.GRPCEndpoint)},
		{Name: prefix + clusterEnvURLREST, Value: cluster.RESTEndpoint},
		{Name: prefix + clusterEnvAuthorizationsEnabled, Value: strconv.FormatBool(true)},
	}
	if cluster.ExternalURL != "" {
		env = append(
			env, corev1.EnvVar{Name: prefix + clusterEnvURLWebapp, Value: cluster.ExternalURL},
		)
	}

	return env
}

// clusterAuthentication returns the Web Modeler authentication method of a
// cluster.
func clusterAuthentication(method v1.AuthenticationMethod) string {
	if method == v1.AuthenticationMethodOIDC {
		return clusterAuthBearerToken
	}

	return clusterAuthBasic
}

// grpcURL returns the gRPC address as a URL. An endpoint that already carries
// a scheme keeps it, so a cluster that publishes one is not given a second.
func grpcURL(endpoint string) string {
	if strings.Contains(endpoint, "://") {
		return endpoint
	}

	return grpcScheme + endpoint
}
