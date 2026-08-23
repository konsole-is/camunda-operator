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
	"strconv"
	"strings"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/deployment"
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

// The environment variables of the Management Identity container. The source
// of each one is the configuration variables page, unless its comment names
// another page below by title.
//
// Configuration variables:
// https://docs.camunda.io/docs/self-managed/components/management-identity/miscellaneous/configuration-variables/
//
// Connect Management Identity to an identity provider:
// https://docs.camunda.io/docs/self-managed/components/management-identity/configuration/connect-to-an-oidc-provider/
//
// Connect Camunda to Microsoft Entra ID:
// https://docs.camunda.io/docs/self-managed/deployment/helm/configure/authentication-and-authorization/microsoft-entra/
const (
	// identityEnvURL is the URL of the Identity service itself. Identity
	// registers it as the redirect URI of its own client.
	identityEnvURL = "IDENTITY_URL"
	// The database connection. Identity stores its authorizations, its
	// tenants, and, under an external identity provider, its users in it.
	identityEnvDatabaseHost     = "IDENTITY_DATABASE_HOST"
	identityEnvDatabasePort     = "IDENTITY_DATABASE_PORT"
	identityEnvDatabaseName     = "IDENTITY_DATABASE_NAME"
	identityEnvDatabaseUsername = "IDENTITY_DATABASE_USERNAME"
	identityEnvDatabasePassword = "IDENTITY_DATABASE_PASSWORD"
	// identityEnvType selects the kind of identity provider: KEYCLOAK,
	// GENERIC, or MICROSOFT.
	identityEnvType = "CAMUNDA_IDENTITY_TYPE"
	// identityEnvBaseURL is the base URL of Identity that the Identity SDK of
	// every other component reaches.
	identityEnvBaseURL = "CAMUNDA_IDENTITY_BASE_URL"
	// identityEnvIssuer is the front-channel issuer, used for the login
	// redirect and the logout.
	identityEnvIssuer = "CAMUNDA_IDENTITY_ISSUER"
	// identityEnvIssuerBackendURL is the back-channel issuer, used for token
	// verification from inside the Kubernetes cluster.
	identityEnvIssuerBackendURL = "CAMUNDA_IDENTITY_ISSUER_BACKEND_URL"
	// identityEnvClientID and identityEnvClientSecret are the credentials of
	// the Identity client at the identity provider.
	identityEnvClientID     = "CAMUNDA_IDENTITY_CLIENT_ID"
	identityEnvClientSecret = "CAMUNDA_IDENTITY_CLIENT_SECRET"
	// identityEnvAudience is the audience that access tokens must carry.
	// Identity refuses to start without it ("Connect Camunda to Microsoft
	// Entra ID").
	identityEnvAudience = "CAMUNDA_IDENTITY_AUDIENCE"
	// identityEnvUsernameClaim is the token claim that holds the username of
	// a person. Identity defaults it per provider type, so the operator sets
	// it only when the platform config names one.
	identityEnvUsernameClaim = "CAMUNDA_IDENTITY_USERNAMECLAIM"
	// identityEnvInitialClaimName and identityEnvInitialClaimValue name the
	// first administrator of the management plane. Identity reads them on its
	// first start only ("Connect Management Identity to an identity
	// provider").
	identityEnvInitialClaimName  = "IDENTITY_INITIAL_CLAIM_NAME"
	identityEnvInitialClaimValue = "IDENTITY_INITIAL_CLAIM_VALUE"
)

// The literal values the renderer sets. The health endpoint and the port it
// answers on come from the monitoring page:
// https://docs.camunda.io/docs/self-managed/components/management-identity/miscellaneous/application-monitoring/
const (
	// identityProfileOIDC and identityProfileKeycloak are the Spring profiles
	// that bind the settings of an external identity provider and of a
	// Keycloak ("Connect Management Identity to an identity provider").
	identityProfileOIDC     = "oidc"
	identityProfileKeycloak = "keycloak"
	// The CAMUNDA_IDENTITY_TYPE values of the three provider types: a
	// Keycloak, and the two external ones that the platform config offers.
	identityTypeKeycloak  = "KEYCLOAK"
	identityTypeGeneric   = "GENERIC"
	identityTypeMicrosoft = "MICROSOFT"
	// identityHealthPath is the health endpoint on the management port. It is
	// the probe path of the 8.9 Helm chart too.
	identityHealthPath = "/actuator/health"
	// identityContainer is the container of the Management Identity
	// Deployment.
	identityContainer = "identity"
	// initialClaimSeparator divides the name and the value of the initial
	// administrator claim in the annotation that records it. A claim name is
	// a token claim, which carries no "=".
	initialClaimSeparator = "="
)

// identityComponents renders Management Identity: its Deployment and its
// Service, in one component under the IdentityReady condition. Identity is
// always deployed, so this renders on every management plane and always takes
// part in Ready. In the keycloak mode it waits for the Keycloak to become
// ready.
func identityComponents(in Input) (Built, error) {
	workload, err := deployment.NewBuilder(identityDeployment(in)).
		WithMutation(workloadmutations.Mutations(in.workload(ComponentIdentity), identityContainer)...).
		Build()
	if err != nil {
		return Built{}, fmt.Errorf("building the %s component: %w", ComponentIdentity, err)
	}

	svc, err := service.NewBuilder(identityService(in)).Build()
	if err != nil {
		return Built{}, fmt.Errorf("building the %s component: %w", ComponentIdentity, err)
	}

	builder := component.NewComponentBuilder().
		WithName(ComponentIdentity).
		WithConditionType(component.ConditionType(v1.ConditionIdentityReady)).
		WithResource(workload).
		WithResource(svc).
		Suspend(in.Suspended)
	if in.Provider.Mode == ModeKeycloak {
		// The Keycloak Operator writes the administrator Secret that
		// KEYCLOAK_SETUP_USER and KEYCLOAK_SETUP_PASSWORD name, so an
		// Identity pod cannot start before the Keycloak is ready.
		builder = builder.WithPrerequisite(
			component.DependsOn(component.ConditionType(v1.ConditionKeycloakReady)),
		)
	}

	comp, err := builder.Build()
	if err != nil {
		return Built{}, fmt.Errorf("building the %s component: %w", ComponentIdentity, err)
	}

	comps := []*component.Component{comp}

	return Built{Components: comps, Ready: comps}, nil
}

// identityDeployment renders the base Deployment. workloadmutations.Mutations
// layers the override surfaces of spec.identity on top.
func identityDeployment(in Input) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      IdentityName(in.Cluster),
			Namespace: in.Cluster.Namespace,
			Labels:    managedLabels(in, ComponentIdentity),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(in.replicas(ComponentIdentity)),
			Selector: &metav1.LabelSelector{MatchLabels: discoveryLabels(in, ComponentIdentity)},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      discoveryLabels(in, ComponentIdentity),
					Annotations: map[string]string{ConfigHashAnnotation: ConfigHash(in, ComponentIdentity)},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{identityContainerSpec(in)}},
			},
		},
	}
}

// identityContainerSpec renders the Management Identity container.
//
// It carries no liveness probe. The health endpoint aggregates the check of
// the Identity datasource, so a liveness probe on it would restart every pod
// in a loop while the database is unreachable. The readiness probe takes the
// pods out of the Service instead, which is also what the 8.9 Helm chart does:
// it enables the readiness probe and leaves the liveness probe off.
func identityContainerSpec(in Input) corev1.Container {
	return corev1.Container{
		Name:  identityContainer,
		Image: images.Resolve(in.Platform, images.Identity, in.Cluster.Spec.Identity.Version),
		Env:   identityEnv(in),
		Ports: []corev1.ContainerPort{
			{Name: portNameHTTP, ContainerPort: IdentityPortHTTP, Protocol: corev1.ProtocolTCP},
			{Name: portNameManagement, ContainerPort: IdentityPortManagement, Protocol: corev1.ProtocolTCP},
		},
		StartupProbe: probe(
			portNameManagement, identityHealthPath, startupPeriodSeconds, startupFailureThreshold,
		),
		ReadinessProbe: probe(
			portNameManagement, identityHealthPath, readinessPeriodSeconds, 0,
		),
	}
}

// identityEnv renders the environment of the Management Identity container:
// the identity provider, the URL of Identity itself, its database, and the
// license.
func identityEnv(in Input) []corev1.EnvVar {
	env := identityProviderEnv(in)
	env = append(env, corev1.EnvVar{Name: identityEnvURL, Value: in.Cluster.Spec.Identity.ExternalURL})
	env = append(env, identityDatabaseEnv(in.Databases.Identity)...)

	if ref := in.Platform.LicenseSecretRef; ref != nil {
		env = append(env, corev1.EnvVar{
			Name:      camundaconfig.EnvLicenseKey,
			ValueFrom: secretSource(ref.Name, ref.Key),
		})
	}

	return env
}

// identityProviderEnv renders the connection to the identity provider. The
// two modes carry two settings: an external provider is bound by the oidc
// profile of Management Identity, and a Keycloak by the keycloak profile,
// which also bootstraps the realm.
func identityProviderEnv(in Input) []corev1.EnvVar {
	if in.Provider.Mode != ModeOIDC {
		return keycloakProviderEnv(in)
	}

	return identityOIDCProviderEnv(in)
}

// identityOIDCProviderEnv renders the connection to an identity provider that
// the operator does not run.
func identityOIDCProviderEnv(in Input) []corev1.EnvVar {
	provider := in.Provider
	client := provider.Clients.Identity

	var env []corev1.EnvVar
	if provider.SpringProfile != "" {
		env = append(env, corev1.EnvVar{
			Name:  camundaconfig.EnvSpringProfilesActive,
			Value: provider.SpringProfile,
		})
	}

	env = append(
		env,
		corev1.EnvVar{Name: identityEnvType, Value: provider.Type},
		corev1.EnvVar{Name: identityEnvBaseURL, Value: in.Cluster.Spec.Identity.ExternalURL},
		corev1.EnvVar{Name: identityEnvIssuer, Value: provider.IssuerURL},
		corev1.EnvVar{Name: identityEnvIssuerBackendURL, Value: provider.IssuerBackendURL},
		corev1.EnvVar{Name: identityEnvClientID, Value: client.ID},
		corev1.EnvVar{Name: identityEnvAudience, Value: client.Audience},
	)
	if ref := client.SecretRef; ref != nil {
		env = append(env, corev1.EnvVar{
			Name:      identityEnvClientSecret,
			ValueFrom: secretSource(ref.Name, ref.Key),
		})
	}
	if provider.UsernameClaim != "" {
		env = append(env, corev1.EnvVar{Name: identityEnvUsernameClaim, Value: provider.UsernameClaim})
	}

	name, value := initialClaim(in.Cluster)

	return append(
		env,
		corev1.EnvVar{Name: identityEnvInitialClaimName, Value: name},
		corev1.EnvVar{Name: identityEnvInitialClaimValue, Value: value},
	)
}

// initialClaim returns the initial administrator claim that the rendered
// environment carries: the one that Identity started with once it recorded
// one, and spec.identity.admin until then. Identity stores the first value in
// its database and ignores the setting on every later start, so a rendered
// change would only make the resource lie about what the management plane
// does.
func initialClaim(mc *v1.CamundaManagementCluster) (name, value string) {
	recorded := RecordedInitialClaim(mc)
	if recorded == "" {
		recorded = SpecInitialClaim(mc)
	}
	name, value, _ = strings.Cut(recorded, initialClaimSeparator)

	return name, value
}

// SpecInitialClaim returns the initial administrator claim of the spec, in the
// form the annotation records: "<claimName>=<claimValue>". The controller
// compares it against the recorded one to find a change that Identity can no
// longer act on.
func SpecInitialClaim(mc *v1.CamundaManagementCluster) string {
	admin := mc.Spec.Identity.Admin

	return admin.ClaimName + initialClaimSeparator + admin.ClaimValue
}

// RecordedInitialClaim returns the initial administrator claim that
// Management Identity started with, or empty while it has not started yet.
func RecordedInitialClaim(mc *v1.CamundaManagementCluster) string {
	return mc.Annotations[InitialClaimAnnotation]
}

// StartedInitialClaim returns the initial administrator claim that Management
// Identity started with, read from the environment of the Identity container
// that started first among pods. It returns empty while none of them has run.
//
// Pass every pod of the Identity Deployment. A rollout runs two of them at
// once, and the older one is the one whose claim Identity wrote into its
// database.
func StartedInitialClaim(pods []corev1.Pod) string {
	var claim, name string
	var first *metav1.Time

	// Two pods can share a start time, as the kubelet records it in seconds,
	// and the list has no order. The name breaks the tie, so the pick does not
	// change between reconciles.
	for i := range pods {
		started := identityStartedAt(&pods[i])
		if started == nil {
			continue
		}
		if first == nil || started.Before(first) || (started.Equal(first) && pods[i].Name < name) {
			first, name, claim = started, pods[i].Name, identityClaimEnv(&pods[i])
		}
	}

	return claim
}

// identityStartedAt returns when the pod of the Management Identity container
// began, or nil while the container never ran.
//
// The started field of the container status is not the signal. The container
// carries a startup probe, so the kubelet sets that field only after the probe
// passes on the health endpoint, which is just before the pod is ready.
// Identity reads the administrator claim as it boots, well before that.
//
// The container states hold the latest run and the one before it only, so a
// container that restarted twice has lost its first run. The start time of
// the pod is older than every run and stays, so it orders the pods once the
// container ran at least once. A pod without one falls back to the earliest
// run its container states still hold.
func identityStartedAt(pod *corev1.Pod) *metav1.Time {
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name != identityContainer {
			continue
		}
		var first *metav1.Time
		for _, started := range []*metav1.Time{
			runStartedAt(status.State),
			runStartedAt(status.LastTerminationState),
		} {
			if started != nil && (first == nil || started.Before(first)) {
				first = started
			}
		}
		if first != nil && pod.Status.StartTime != nil {
			return pod.Status.StartTime
		}

		return first
	}

	return nil
}

// runStartedAt returns when the run that state describes began, or nil for a
// container that waits and has not run.
func runStartedAt(state corev1.ContainerState) *metav1.Time {
	if state.Running != nil {
		return &state.Running.StartedAt
	}
	if state.Terminated != nil {
		return &state.Terminated.StartedAt
	}

	return nil
}

// identityClaimEnv returns the initial administrator claim in the environment
// of the Management Identity container of pod, or empty when it carries none.
func identityClaimEnv(pod *corev1.Pod) string {
	var name, value string
	for _, container := range pod.Spec.Containers {
		if container.Name != identityContainer {
			continue
		}
		for _, env := range container.Env {
			switch env.Name {
			case identityEnvInitialClaimName:
				name = env.Value
			case identityEnvInitialClaimValue:
				value = env.Value
			}
		}
	}
	if name == "" {
		return ""
	}

	return name + initialClaimSeparator + value
}

// identityDatabaseEnv renders the connection to the Identity database.
func identityDatabaseEnv(db Database) []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: identityEnvDatabaseHost, Value: db.Host},
		{Name: identityEnvDatabasePort, Value: strconv.Itoa(int(db.Port))},
		{Name: identityEnvDatabaseName, Value: db.Name},
		{
			Name:      identityEnvDatabaseUsername,
			ValueFrom: secretSource(db.Credentials.Name, db.Credentials.UsernameKey),
		},
		{
			Name:      identityEnvDatabasePassword,
			ValueFrom: secretSource(db.Credentials.Name, db.Credentials.PasswordKey),
		},
	}
}

// identityService renders the Service of Management Identity. Both ports are
// exposed: the HTTP port serves the user interface and the API, the management
// port the actuator endpoints.
func identityService(in Input) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      IdentityName(in.Cluster),
			Namespace: in.Cluster.Namespace,
			Labels:    managedLabels(in, ComponentIdentity),
		},
		Spec: corev1.ServiceSpec{
			Selector: discoveryLabels(in, ComponentIdentity),
			Ports: []corev1.ServicePort{
				{
					Name:       portNameHTTP,
					Port:       IdentityServicePortHTTP,
					TargetPort: intstr.FromString(portNameHTTP),
					Protocol:   corev1.ProtocolTCP,
				},
				{
					Name:       portNameManagement,
					Port:       IdentityServicePortManagement,
					TargetPort: intstr.FromString(portNameManagement),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}
