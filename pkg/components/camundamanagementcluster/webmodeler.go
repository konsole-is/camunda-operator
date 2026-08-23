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
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/deployment"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/secret"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/service"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
	"github.com/konsole-is/camunda-operator/pkg/images"
	"github.com/konsole-is/camunda-operator/pkg/workloadmutations"
)

// The environment variables of the Web Modeler restapi container. Every one of
// them is on the configuration page, in the section its comment names.
//
// Configuration of the restapi component:
// https://docs.camunda.io/docs/self-managed/components/modeler/web-modeler/configuration/
const (
	// The database connection. Web Modeler needs a PostgreSQL database of its
	// own ("Database").
	webModelerEnvDatasourceURL      = "SPRING_DATASOURCE_URL"
	webModelerEnvDatasourceUsername = "SPRING_DATASOURCE_USERNAME"
	webModelerEnvDatasourcePassword = "SPRING_DATASOURCE_PASSWORD"
	// The SMTP server that Web Modeler sends its notifications through. It
	// does not start without one ("SMTP / email").
	webModelerEnvMailHost        = "RESTAPI_MAIL_HOST"
	webModelerEnvMailPort        = "RESTAPI_MAIL_PORT"
	webModelerEnvMailUser        = "RESTAPI_MAIL_USER"
	webModelerEnvMailPassword    = "RESTAPI_MAIL_PASSWORD"
	webModelerEnvMailEnableTLS   = "RESTAPI_MAIL_ENABLE_TLS"
	webModelerEnvMailFromAddress = "RESTAPI_MAIL_FROM_ADDRESS"
	webModelerEnvMailFromName    = "RESTAPI_MAIL_FROM_NAME"
	// webModelerEnvServerURL is the URL that a browser reaches Web Modeler
	// at. Web Modeler builds its login redirects and the links in its
	// notification emails from it ("General").
	webModelerEnvServerURL = "RESTAPI_SERVER_URL"
	// webModelerEnvContextPath is required when the server URL does not point
	// at the root of its domain ("General").
	webModelerEnvContextPath = "SERVER_SERVLET_CONTEXTPATH"
	// webModelerEnvHTTPSOnly redirects a browser from http to https. It
	// defaults to true, so an http external URL needs it turned off
	// ("General").
	webModelerEnvHTTPSOnly = "SERVER_HTTPS_ONLY"
	// webModelerEnvClientID is the client of the Web Modeler user interface
	// ("Identity / Keycloak").
	webModelerEnvClientID = "OAUTH2_CLIENT_ID"
	// webModelerEnvIdentityBaseURL is the in-cluster URL of Management
	// Identity, which Web Modeler reads its users from ("Identity /
	// Keycloak").
	webModelerEnvIdentityBaseURL = "CAMUNDA_IDENTITY_BASEURL"
	// webModelerEnvIdentityType selects the kind of identity provider, the
	// CAMUNDA_IDENTITY_TYPE of Management Identity.
	webModelerEnvIdentityType = "CAMUNDA_IDENTITY_TYPE"
	// The two audiences that Web Modeler validates: the internal one in the
	// tokens of a person, the public one in the tokens of an application that
	// calls the Web Modeler API ("Identity / Keycloak").
	webModelerEnvAudienceInternalAPI = "CAMUNDA_MODELER_SECURITY_JWT_AUDIENCE_INTERNAL_API"
	webModelerEnvAudiencePublicAPI   = "CAMUNDA_MODELER_SECURITY_JWT_AUDIENCE_PUBLIC_API"
	// The issuer of the tokens, for validation ("Identity / Keycloak"). The
	// backend URL is the one that Web Modeler requests the provider
	// configuration from, from inside the Kubernetes cluster.
	webModelerEnvIssuerURI        = "SPRING_SECURITY_OAUTH2_RESOURCESERVER_JWT_ISSUER_URI"
	webModelerEnvJWKSetURI        = "SPRING_SECURITY_OAUTH2_RESOURCESERVER_JWT_JWK_SET_URI"
	webModelerEnvIssuerBackendURL = "RESTAPI_OAUTH2_TOKEN_ISSUER_BACKEND_URL"
	// webModelerEnvUsernameClaim is the token claim that Web Modeler reads a
	// username from. It defaults to "name" ("Identity / Keycloak").
	webModelerEnvUsernameClaim = "CAMUNDA_MODELER_OAUTH2_TOKEN_USERNAMECLAIM"
	// The WebSocket server as the restapi process reaches it ("WebSocket").
	// The three credentials must match the ones of the websockets process.
	webModelerEnvPusherHost   = "RESTAPI_PUSHER_HOST"
	webModelerEnvPusherPort   = "RESTAPI_PUSHER_PORT"
	webModelerEnvPusherAppID  = "RESTAPI_PUSHER_APP_ID"
	webModelerEnvPusherKey    = "RESTAPI_PUSHER_KEY"
	webModelerEnvPusherSecret = "RESTAPI_PUSHER_SECRET"
	// The WebSocket server as a browser reaches it ("WebSocket").
	webModelerEnvClientPusherHost     = "CLIENT_PUSHER_HOST"
	webModelerEnvClientPusherPort     = "CLIENT_PUSHER_PORT"
	webModelerEnvClientPusherPath     = "CLIENT_PUSHER_PATH"
	webModelerEnvClientPusherForceTLS = "CLIENT_PUSHER_FORCE_TLS"
)

// The environment variables of the Web Modeler websockets container. They are
// on the same page, under "Configuration of the websocket component".
const (
	webModelerEnvAppID   = "PUSHER_APP_ID"
	webModelerEnvAppKey  = "PUSHER_APP_KEY"
	webModelerEnvAppPath = "PUSHER_APP_PATH"
	// webModelerEnvAppSecret has no _APP_ in the restapi name, so the two
	// sides of the pair spell it differently.
	webModelerEnvAppSecret = "PUSHER_APP_SECRET"
)

// The literal values and the identities that the renderer sets.
const (
	// webModelerDefaultPublicAPIAudience is what Web Modeler validates in the
	// tokens of the public API when the platform config names no audience of
	// its own ("Identity / Keycloak").
	webModelerDefaultPublicAPIAudience = "web-modeler-public-api"
	// webModelerRestapiHealthPath is the readiness endpoint on the management
	// port, and webModelerWebsocketsHealthPath the one of the websockets
	// process on its only port
	// (https://docs.camunda.io/docs/self-managed/components/modeler/web-modeler/monitoring/).
	webModelerRestapiHealthPath    = "/health/readiness"
	webModelerWebsocketsHealthPath = "/up"
	// The containers of the two Deployments.
	webModelerRestapiContainer    = "restapi"
	webModelerWebsocketsContainer = "websockets"
	// defaultPusherPath is the WebSocket base path when the external URL
	// names none.
	defaultPusherPath = "/"
)

// The scheme and the default ports that an external URL is read against. The
// CRD accepts http and https only.
const (
	schemeHTTPS = "https"
	portHTTPS   = 443
	portHTTP    = 80
)

// webModelerComponents renders Web Modeler as one component under
// WebModelerReady: the Secret that pairs the two processes, then the restapi
// and the websockets Deployment with their Services. Both processes have to
// answer for the product to work, so one condition covers them, and the Secret
// is generated for them alone, so it goes when they go.
//
// The component is built while spec.webModeler is unset too, gated off. A
// management cluster that drops Web Modeler then has its workloads deleted
// instead of left running, and the gate keeps the Disabled condition out of
// Ready.
func webModelerComponents(in Input) (Built, error) {
	deployed := in.Cluster.Spec.WebModeler != nil
	gate := component.GatedBy(feature.NewBooleanGate(deployed))

	pusher, err := pusherSecret(in)
	if err != nil {
		return Built{}, err
	}

	restapi, err := webModelerDeploymentResource(
		in, ComponentWebModelerRestapi, webModelerRestapiContainer,
		WebModelerRestapiName(in.Cluster), webModelerRestapiContainerSpec(in),
	)
	if err != nil {
		return Built{}, err
	}

	websockets, err := webModelerDeploymentResource(
		in, ComponentWebModelerWebsockets, webModelerWebsocketsContainer,
		WebModelerWebsocketsName(in.Cluster), webModelerWebsocketsContainerSpec(in),
	)
	if err != nil {
		return Built{}, err
	}

	restapiService, err := service.NewBuilder(webModelerRestapiService(in)).Build()
	if err != nil {
		return Built{}, fmt.Errorf("building the %s component: %w", ComponentWebModelerRestapi, err)
	}

	websocketsService, err := service.NewBuilder(webModelerWebsocketsService(in)).Build()
	if err != nil {
		return Built{}, fmt.Errorf("building the %s component: %w", ComponentWebModelerWebsockets, err)
	}

	comp, err := component.NewComponentBuilder().
		WithName(ComponentWebModeler).
		WithConditionType(component.ConditionType(v1.ConditionWebModelerReady)).
		WithFeatureGate(feature.NewBooleanGate(deployed)).
		WithResource(pusher, gate).
		WithResource(restapi, gate).
		WithResource(restapiService, gate).
		WithResource(websockets, gate).
		WithResource(websocketsService, gate).
		Suspend(in.Suspended).
		Build()
	if err != nil {
		return Built{}, fmt.Errorf("building the %s component: %w", ComponentWebModeler, err)
	}

	built := Built{Components: []*component.Component{comp}}
	if deployed {
		built.Ready = built.Components
	}

	return built, nil
}

// pusherSecret renders the generated credentials that both Web Modeler
// processes authenticate the WebSocket connection with. An ocf Secret models
// no suspension, so it stays as it is while the workloads are scaled to zero.
//
// A reused credential carries its apply precondition onto the Secret, so a
// delete of the Secret always rotates it. The controller must reconcile this
// component through credentials.NewApplyClient for that to hold.
func pusherSecret(in Input) (*secret.Resource, error) {
	pusher, err := secret.NewBuilder(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        PusherSecretName(in.Cluster),
			Namespace:   in.Cluster.Namespace,
			Labels:      managedLabels(in, ComponentWebModeler),
			Annotations: in.Pusher.Key.PreconditionAnnotations(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			PusherAppIDKey:     []byte(PusherAppID),
			PusherAppKeyKey:    []byte(in.Pusher.Key.Value),
			PusherAppSecretKey: []byte(in.Pusher.Secret.Value),
		},
	}).Build()
	if err != nil {
		return nil, fmt.Errorf("building the %s component: %w", ComponentWebModeler, err)
	}

	return pusher, nil
}

// webModelerDeploymentResource builds one Web Modeler Deployment with the
// override surfaces of its block layered on top.
func webModelerDeploymentResource(
	in Input,
	comp string,
	container string,
	name string,
	spec corev1.Container,
) (*deployment.Resource, error) {
	workload, err := deployment.NewBuilder(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: in.Cluster.Namespace,
			Labels:    managedLabels(in, comp),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(in.replicas(comp)),
			Selector: &metav1.LabelSelector{MatchLabels: discoveryLabels(in, comp)},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      discoveryLabels(in, comp),
					Annotations: map[string]string{ConfigHashAnnotation: ConfigHash(in, comp)},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{spec}},
			},
		},
	}).
		WithMutation(workloadmutations.Mutations(in.workload(comp), container)...).
		Build()
	if err != nil {
		return nil, fmt.Errorf("building the %s component: %w", comp, err)
	}

	return workload, nil
}

// webModelerRestapiContainerSpec renders the restapi container. Like
// Management Identity it carries no liveness probe: its health endpoint
// aggregates the check of the Web Modeler datasource, so a liveness probe
// would restart every pod in a loop while the database is unreachable.
func webModelerRestapiContainerSpec(in Input) corev1.Container {
	return corev1.Container{
		Name: webModelerRestapiContainer,
		Image: images.Resolve(
			in.Platform, images.WebModelerRestapi, in.webModeler().Version,
		),
		Env: webModelerRestapiEnv(in),
		Ports: []corev1.ContainerPort{
			{
				Name:          portNameHTTP,
				ContainerPort: WebModelerRestapiPortHTTP,
				Protocol:      corev1.ProtocolTCP,
			},
			{
				Name:          portNameManagement,
				ContainerPort: WebModelerRestapiPortManagement,
				Protocol:      corev1.ProtocolTCP,
			},
		},
		StartupProbe: probe(
			portNameManagement, webModelerRestapiHealthPath,
			startupPeriodSeconds, startupFailureThreshold,
		),
		ReadinessProbe: probe(
			portNameManagement, webModelerRestapiHealthPath, readinessPeriodSeconds, 0,
		),
	}
}

// webModelerRestapiEnv renders the environment of the restapi container: its
// database, the SMTP server, where a browser reaches it, the identity
// provider, the WebSocket pairing, the license, and the attached clusters.
func webModelerRestapiEnv(in Input) []corev1.EnvVar {
	spec := in.webModeler()

	env := webModelerDatabaseEnv(in.Databases.WebModeler)
	env = append(env, webModelerMailEnv(spec.Mail, in.WebModelerMail)...)
	env = append(env, webModelerServerEnv(spec)...)
	env = append(env, webModelerProviderEnv(in)...)
	env = append(env, webModelerPusherEnv(in)...)

	if ref := in.Platform.LicenseSecretRef; ref != nil {
		env = append(env, corev1.EnvVar{
			Name:      camundaconfig.EnvLicenseKey,
			ValueFrom: secretSource(ref.Name, ref.Key),
		})
	}

	return append(env, ClustersEnv(in.Clusters)...)
}

// webModelerDatabaseEnv renders the connection to the Web Modeler database. It
// is nil while the controller has resolved none, which is every management
// cluster that does not deploy Web Modeler.
func webModelerDatabaseEnv(db *Database) []corev1.EnvVar {
	if db == nil {
		return nil
	}

	return []corev1.EnvVar{
		{
			Name: webModelerEnvDatasourceURL,
			Value: fmt.Sprintf(
				"jdbc:postgresql://%s:%d/%s", db.Host, db.Port, db.Name,
			),
		},
		{
			Name:      webModelerEnvDatasourceUsername,
			ValueFrom: secretSource(db.Credentials.Name, db.Credentials.UsernameKey),
		},
		{
			Name:      webModelerEnvDatasourcePassword,
			ValueFrom: secretSource(db.Credentials.Name, db.Credentials.PasswordKey),
		},
	}
}

// webModelerMailEnv renders the SMTP server. The user and the password are
// left out for a server that needs no credentials.
func webModelerMailEnv(mail v1.WebModelerMailSpec, ref *v1.CredentialsSecretRef) []corev1.EnvVar {
	env := []corev1.EnvVar{
		{Name: webModelerEnvMailHost, Value: mail.SMTPHost},
		{Name: webModelerEnvMailPort, Value: strconv.Itoa(int(mail.SMTPPort))},
		{Name: webModelerEnvMailEnableTLS, Value: strconv.FormatBool(mail.TLS == nil || *mail.TLS)},
		{Name: webModelerEnvMailFromAddress, Value: mail.FromAddress},
	}
	if mail.FromName != "" {
		env = append(env, corev1.EnvVar{Name: webModelerEnvMailFromName, Value: mail.FromName})
	}
	if ref != nil {
		env = append(
			env,
			corev1.EnvVar{
				Name:      webModelerEnvMailUser,
				ValueFrom: secretSource(ref.Name, ref.UsernameKey),
			},
			corev1.EnvVar{
				Name:      webModelerEnvMailPassword,
				ValueFrom: secretSource(ref.Name, ref.PasswordKey),
			},
		)
	}

	return env
}

// webModelerServerEnv renders where a browser reaches Web Modeler. A URL under
// a path needs the context path, and an http URL needs the redirect to https
// turned off, which is on by default.
func webModelerServerEnv(spec v1.WebModelerSpec) []corev1.EnvVar {
	external := parseURL(spec.ExternalURL)

	env := []corev1.EnvVar{
		{Name: webModelerEnvServerURL, Value: spec.ExternalURL},
		{Name: webModelerEnvHTTPSOnly, Value: strconv.FormatBool(external.Scheme == schemeHTTPS)},
	}
	if path := strings.TrimSuffix(external.Path, "/"); path != "" {
		env = append(env, corev1.EnvVar{Name: webModelerEnvContextPath, Value: path})
	}

	return env
}

// parseURL reads an external URL of the spec. The CRD validates every one of
// them as an http or https URL with a host, so a URL that does not parse
// cannot reach the renderer. It yields the empty URL rather than an error the
// caller could not act on.
func parseURL(raw string) *url.URL {
	parsed, err := url.Parse(raw)
	if err != nil {
		return &url.URL{}
	}

	return parsed
}

// webModelerProviderEnv renders the identity provider and Management Identity.
func webModelerProviderEnv(in Input) []corev1.EnvVar {
	provider := in.Provider

	env := []corev1.EnvVar{
		{Name: webModelerEnvClientID, Value: provider.Clients.WebModeler.ID},
		{Name: webModelerEnvIdentityBaseURL, Value: IdentityServiceURL(in.Cluster)},
		{Name: webModelerEnvIdentityType, Value: provider.Type},
		{Name: webModelerEnvAudienceInternalAPI, Value: provider.Clients.WebModelerAPI.Audience},
		{Name: webModelerEnvAudiencePublicAPI, Value: provider.Clients.WebModelerPublicAPIAudience},
		{Name: webModelerEnvIssuerURI, Value: provider.IssuerURL},
		{Name: webModelerEnvIssuerBackendURL, Value: provider.IssuerBackendURL},
	}
	if provider.JwksURL != "" {
		env = append(env, corev1.EnvVar{Name: webModelerEnvJWKSetURI, Value: provider.JwksURL})
	}
	if provider.UsernameClaim != "" {
		env = append(
			env, corev1.EnvVar{Name: webModelerEnvUsernameClaim, Value: provider.UsernameClaim},
		)
	}

	return env
}

// webModelerPusherEnv renders both sides of the WebSocket pairing as the
// restapi process needs them: where the WebSocket server is inside the
// Kubernetes cluster, and where a browser reaches it.
func webModelerPusherEnv(in Input) []corev1.EnvVar {
	external := parseURL(in.webModeler().WebsocketsExternalURL)

	return []corev1.EnvVar{
		{
			Name: webModelerEnvPusherHost,
			Value: fmt.Sprintf(
				"%s.%s.svc", WebModelerWebsocketsName(in.Cluster), in.Cluster.Namespace,
			),
		},
		{
			Name:  webModelerEnvPusherPort,
			Value: strconv.Itoa(int(WebModelerWebsocketsServicePortHTTP)),
		},
		{Name: webModelerEnvPusherAppID, Value: PusherAppID},
		{
			Name:      webModelerEnvPusherKey,
			ValueFrom: secretSource(PusherSecretName(in.Cluster), PusherAppKeyKey),
		},
		{
			Name:      webModelerEnvPusherSecret,
			ValueFrom: secretSource(PusherSecretName(in.Cluster), PusherAppSecretKey),
		},
		{Name: webModelerEnvClientPusherHost, Value: external.Hostname()},
		{Name: webModelerEnvClientPusherPort, Value: externalPort(external)},
		{Name: webModelerEnvClientPusherPath, Value: pusherPath(external)},
		{
			Name:  webModelerEnvClientPusherForceTLS,
			Value: strconv.FormatBool(external.Scheme == schemeHTTPS),
		},
	}
}

// webModelerWebsocketsContainerSpec renders the websockets container. It reads
// the same pairing credentials as the restapi container and nothing else.
func webModelerWebsocketsContainerSpec(in Input) corev1.Container {
	return corev1.Container{
		Name: webModelerWebsocketsContainer,
		Image: images.Resolve(
			in.Platform, images.WebModelerWebsockets, in.webModeler().Version,
		),
		Env: webModelerWebsocketsEnv(in),
		Ports: []corev1.ContainerPort{{
			Name:          portNameHTTP,
			ContainerPort: WebModelerWebsocketsPortHTTP,
			Protocol:      corev1.ProtocolTCP,
		}},
		StartupProbe: probe(
			portNameHTTP, webModelerWebsocketsHealthPath,
			startupPeriodSeconds, startupFailureThreshold,
		),
		ReadinessProbe: probe(
			portNameHTTP, webModelerWebsocketsHealthPath, readinessPeriodSeconds, 0,
		),
	}
}

// webModelerWebsocketsEnv renders the environment of the websockets
// container. The base path travels with the credentials, because the browser
// is told the same one through CLIENT_PUSHER_PATH.
func webModelerWebsocketsEnv(in Input) []corev1.EnvVar {
	external := parseURL(in.webModeler().WebsocketsExternalURL)

	return []corev1.EnvVar{
		{Name: webModelerEnvAppID, Value: PusherAppID},
		{
			Name:      webModelerEnvAppKey,
			ValueFrom: secretSource(PusherSecretName(in.Cluster), PusherAppKeyKey),
		},
		{
			Name:      webModelerEnvAppSecret,
			ValueFrom: secretSource(PusherSecretName(in.Cluster), PusherAppSecretKey),
		},
		{Name: webModelerEnvAppPath, Value: pusherPath(external)},
	}
}

// webModelerRestapiService renders the Service of the restapi process. Both
// ports are exposed: the HTTP port serves the user interface and the API, the
// management port the actuator endpoints.
func webModelerRestapiService(in Input) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WebModelerRestapiName(in.Cluster),
			Namespace: in.Cluster.Namespace,
			Labels:    managedLabels(in, ComponentWebModelerRestapi),
		},
		Spec: corev1.ServiceSpec{
			Selector: discoveryLabels(in, ComponentWebModelerRestapi),
			Ports: []corev1.ServicePort{
				{
					Name:       portNameHTTP,
					Port:       WebModelerRestapiServicePortHTTP,
					TargetPort: intstr.FromString(portNameHTTP),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       portNameManagement,
					Port:       WebModelerRestapiServicePortManagement,
					TargetPort: intstr.FromString(portNameManagement),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// webModelerWebsocketsService renders the Service of the websockets process.
// The restapi container and the ingress of the browser both reach it here.
func webModelerWebsocketsService(in Input) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WebModelerWebsocketsName(in.Cluster),
			Namespace: in.Cluster.Namespace,
			Labels:    managedLabels(in, ComponentWebModelerWebsockets),
		},
		Spec: corev1.ServiceSpec{
			Selector: discoveryLabels(in, ComponentWebModelerWebsockets),
			Ports: []corev1.ServicePort{{
				Name:       portNameHTTP,
				Port:       WebModelerWebsocketsServicePortHTTP,
				TargetPort: intstr.FromString(portNameHTTP),
				Protocol:   corev1.ProtocolTCP,
			}},
		},
	}
}

// pusherPath returns the base path of the WebSocket endpoint, which both sides
// of the pairing must agree on. A URL that names no path uses the root.
func pusherPath(external *url.URL) string {
	if path := strings.TrimSuffix(external.Path, "/"); path != "" {
		return path
	}

	return defaultPusherPath
}

// externalPort returns the port a browser connects to: the one the URL names,
// or the default of its scheme.
func externalPort(external *url.URL) string {
	if port := external.Port(); port != "" {
		return port
	}
	if external.Scheme == schemeHTTPS {
		return strconv.Itoa(portHTTPS)
	}

	return strconv.Itoa(portHTTP)
}
