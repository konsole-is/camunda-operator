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
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/secret"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// generatedSecret is one credential that the operator generates and publishes
// in a Secret of its own.
type generatedSecret struct {
	// name is the name of the Secret in the management namespace.
	name string
	// key is the key that holds the credential.
	key string
	// rotates reports whether deleting the Secret rotates the credential
	// everywhere it is used.
	rotates bool
	// published reports whether this management cluster generates the
	// credential. A Secret that is not published is gated off, so what an
	// earlier spec published is deleted rather than left behind.
	published bool
}

// secretsComponents renders the Secrets that the operator generates. The two
// Keycloak modes need them because Management Identity creates the clients
// and the first user itself, and it needs a credential to give each of them.
// The oidc mode generates none: the platform config names every client
// secret, and the first administrator is a token claim rather than a user
// with a password.
//
// The component is built in every mode and gated on the two Keycloak modes,
// so a move to oidc deletes what a Keycloak mode published instead of leaving
// it behind under a SecretsReady that no longer says anything. The
// administrator password carries a gate of its own, because a password of
// your own replaces the generated one.
//
// A rotating Secret carries the apply precondition of the credential it
// publishes, so a delete of the Secret always rotates it. The controller must
// reconcile through credentials.NewApplyClient for that precondition to hold.
func secretsComponents(in Input) (Built, error) {
	keycloakMode := Mode(in.Cluster) != ModeOIDC

	builder := component.NewComponentBuilder().
		WithName(ComponentSecrets).
		WithConditionType(component.ConditionType(v1.ConditionSecretsReady)).
		WithFeatureGate(feature.NewBooleanGate(keycloakMode))

	for _, gen := range generatedSecrets(in) {
		password := in.Secrets.Values[gen.name]
		var precondition map[string]string
		if gen.rotates {
			precondition = password.PreconditionAnnotations()
		}
		published, err := secret.NewBuilder(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        gen.name,
				Namespace:   in.Cluster.Namespace,
				Labels:      managedLabels(in, ComponentSecrets),
				Annotations: precondition,
			},
			Type: corev1.SecretTypeOpaque,
			Data: map[string][]byte{gen.key: []byte(password.Value)},
		}).Build()
		if err != nil {
			return Built{}, fmt.Errorf("building %s component: %w", ComponentSecrets, err)
		}
		builder = builder.WithResource(published, component.GatedBy(feature.NewBooleanGate(gen.published)))
	}

	comp, err := builder.Build()
	if err != nil {
		return Built{}, fmt.Errorf("building %s component: %w", ComponentSecrets, err)
	}

	built := Built{Components: []*component.Component{comp}}
	if keycloakMode {
		built.Ready = built.Components
	}

	return built, nil
}

// generatedSecrets returns the Secrets that the operator generates for this
// management cluster, in the order it renders them, each with the modes and
// the spec that publish it. The oidc mode publishes none, and the
// administrator password is published only while identity.admin names no
// Secret of its own. The administrator password is the one credential that
// does not rotate.
func generatedSecrets(in Input) []generatedSecret {
	mc := in.Cluster
	keycloakMode := Mode(mc) != ModeOIDC

	return []generatedSecret{
		{
			name:      OptimizeClientSecretName(mc),
			key:       ClientSecretKey,
			rotates:   true,
			published: keycloakMode,
		},
		// Management Identity creates the Keycloak user with the
		// administrator password on its first start and never reads the
		// password again, so a new one would leave that user on the old one.
		// Without the precondition, a delete that races the apply republishes
		// the password the user holds. A delete that no apply follows
		// re-creates the Secret with a password that the Keycloak user does
		// not hold, which only a reset in Keycloak recovers.
		{
			name:      IdentityAdminSecretName(mc),
			key:       PasswordKey,
			published: keycloakMode && mc.Spec.Identity.Admin.PasswordSecretRef == nil,
		},
	}
}
