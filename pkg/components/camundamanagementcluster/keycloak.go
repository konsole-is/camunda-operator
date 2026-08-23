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

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/images"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/keycloak"
)

// The Keycloak server options that the rendered Keycloak sets. Camunda
// documents both, together with the whole custom resource, in the guide to
// the operator-based infrastructure:
// https://docs.camunda.io/docs/self-managed/deployment/helm/configure/operator-based-infrastructure/
const (
	// keycloakOptionRelativePath serves Keycloak under /auth. The Camunda
	// build of Keycloak is built for that path, and the issuer URLs of the
	// realm carry it.
	keycloakOptionRelativePath = "http-relative-path"
	// keycloakOptionProxyHeaders makes Keycloak read the scheme and the host
	// of a request from the X-Forwarded headers, so the URLs it builds match
	// the address that the browser used to reach the ingress.
	keycloakOptionProxyHeaders = "proxy-headers"
	// keycloakProxyHeadersValue is the header set that Camunda documents.
	keycloakProxyHeadersValue = "xforwarded"
)

// keycloakDBVendor is the database that the operator runs Keycloak on, and
// keycloakDBSchema is the schema it opens there. Keycloak needs a PostgreSQL
// database of its own, and the reference deployment of Camunda names the
// schema rather than leaving it to the search path of the database user
// (the keycloak-instance manifests of
// https://github.com/camunda/camunda-deployment-references, under
// generic/kubernetes/operator-based/keycloak).
const (
	keycloakDBVendor = "postgres"
	keycloakDBSchema = "public"
)

// keycloakJDBCPrefix builds the JDBC URL of the Keycloak database. The
// Camunda build of Keycloak bundles the AWS JDBC wrapper, and the reference
// deployment of Camunda passes the whole URL rather than the host, the port,
// and the database name, because the optimized image is built for the wrapper
// driver
// (the keycloak-instance manifests of
// https://github.com/camunda/camunda-deployment-references, under
// generic/kubernetes/operator-based/keycloak).
const keycloakJDBCPrefix = "jdbc:aws-wrapper:postgresql://"

// keycloakComponents renders the Keycloak that the operator runs through the
// Keycloak Operator. The component is built in every mode and gated on the
// keycloak mode, so a move to externalKeycloak or to oidc deletes the custom
// resource. A Kubernetes cluster that does not serve the Keycloak kind gets
// no component at all: there is no resource to delete, and a delete would
// fail against an API that serves no such kind.
//
// The Keycloak Operator owns everything below the custom resource: the
// StatefulSet, the Service, and the Secret with the first administrator. This
// operator only writes the resource and reads its Ready condition.
func keycloakComponents(in Input) ([]*component.Component, error) {
	if !in.KeycloakCRDServed {
		return nil, nil
	}

	resource, err := keycloak.NewBuilder(keycloakCR(in)).Build()
	if err != nil {
		return nil, fmt.Errorf("building %s component: %w", ComponentKeycloak, err)
	}

	managed := feature.NewBooleanGate(in.Provider.Mode == ModeKeycloak)
	comp, err := component.NewComponentBuilder().
		WithName(ComponentKeycloak).
		WithConditionType(component.ConditionType(v1.ConditionKeycloakReady)).
		WithFeatureGate(managed).
		WithResource(resource, component.GatedBy(managed)).
		Suspend(in.Suspended).
		Build()
	if err != nil {
		return nil, fmt.Errorf("building %s component: %w", ComponentKeycloak, err)
	}

	return []*component.Component{comp}, nil
}

// keycloakCR renders the Keycloak custom resource. The Ingress of the
// Keycloak Operator stays off: the route to Keycloak is the one that
// spec.identityProvider.keycloak.externalUrl names, and it is yours to run.
//
// Another mode carries no Keycloak block, and the component is then gated off
// and deletes the resource, so the identity alone is rendered.
func keycloakCR(in Input) *keycloak.Keycloak {
	meta := metav1.ObjectMeta{
		Name:      KeycloakName(in.Cluster),
		Namespace: in.Cluster.Namespace,
		Labels:    managedLabels(in, ComponentKeycloak),
	}

	spec := in.Cluster.Spec.IdentityProvider.Keycloak
	if spec == nil {
		return &keycloak.Keycloak{ObjectMeta: meta}
	}

	return &keycloak.Keycloak{
		ObjectMeta: meta,
		Spec: keycloak.KeycloakSpec{
			Instances: new(in.replicas(ComponentKeycloak)),
			Image:     images.Resolve(in.Platform, images.Keycloak, spec.Version),
			DB:        keycloakDB(in),
			HTTP: &keycloak.KeycloakHTTPSpec{
				HTTPEnabled: new(true),
				HTTPPort:    new(KeycloakServicePort),
			},
			Hostname: &keycloak.KeycloakHostnameSpec{
				Hostname: spec.ExternalURL,
				Strict:   new(true),
			},
			Ingress: &keycloak.KeycloakIngressSpec{Enabled: new(false)},
			AdditionalOptions: []keycloak.KeycloakValueOrSecret{
				{Name: keycloakOptionRelativePath, Value: keycloakBasePath},
				{Name: keycloakOptionProxyHeaders, Value: keycloakProxyHeadersValue},
			},
			Unsupported: &keycloak.KeycloakUnsupportedSpec{
				PodTemplate: &corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: discoveryLabels(in, ComponentKeycloak),
					},
				},
			},
			Resources: spec.Resources.DeepCopy(),
		},
	}
}

// keycloakDB renders the database connection of Keycloak, from the resolved
// DatabaseConfig of spec.identityProvider.keycloak.
func keycloakDB(in Input) *keycloak.KeycloakDBSpec {
	db := in.Databases.Keycloak
	if db == nil {
		return nil
	}

	return &keycloak.KeycloakDBSpec{
		Vendor: keycloakDBVendor,
		URL:    fmt.Sprintf("%s%s:%d/%s", keycloakJDBCPrefix, db.Host, db.Port, db.Name),
		Schema: keycloakDBSchema,
		UsernameSecret: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: db.Credentials.Name},
			Key:                  db.Credentials.UsernameKey,
		},
		PasswordSecret: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: db.Credentials.Name},
			Key:                  db.Credentials.PasswordKey,
		},
	}
}
