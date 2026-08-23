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
}

// secretsComponents renders the Secrets that the operator generates. The two
// Keycloak modes need them because Management Identity creates the clients
// and the first user itself, and it needs a credential to give each of them.
// The oidc mode generates none: the platform config names every client
// secret, and the first administrator is a token claim rather than a user
// with a password.
//
// A rotating Secret carries the apply precondition of the credential it
// publishes, so a delete of the Secret always rotates it. The controller must
// reconcile through credentials.NewApplyClient for that precondition to hold.
func secretsComponents(in Input) ([]*component.Component, error) {
	generated := generatedSecrets(in)
	if len(generated) == 0 {
		return nil, nil
	}

	builder := component.NewComponentBuilder().
		WithName(ComponentSecrets).
		WithConditionType(component.ConditionType(v1.ConditionSecretsReady))

	for _, gen := range generated {
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
			return nil, fmt.Errorf("building %s component: %w", ComponentSecrets, err)
		}
		builder = builder.WithResource(published)
	}

	comp, err := builder.Build()
	if err != nil {
		return nil, fmt.Errorf("building %s component: %w", ComponentSecrets, err)
	}

	return []*component.Component{comp}, nil
}

// generatedSecrets returns the Secrets that the operator generates for this
// management cluster, in the order it renders them. The administrator
// password is generated only while spec.identity.admin names no Secret of its
// own. The administrator password is the one credential that does not rotate.
func generatedSecrets(in Input) []generatedSecret {
	var generated []generatedSecret
	for _, gen := range []generatedSecret{
		{name: in.Secrets.IdentityClient, key: ClientSecretKey, rotates: true},
		{name: in.Secrets.OptimizeClient, key: ClientSecretKey, rotates: true},
		// Management Identity creates the Keycloak user with the
		// administrator password on its first start and never reads the
		// password again, so a new one would leave that user on the old one.
		// Without the precondition, a delete that races the apply republishes
		// the password the user holds. A delete that no apply follows loses
		// the password, which only a reset in Keycloak recovers.
		{name: in.Secrets.IdentityAdmin, key: PasswordKey},
	} {
		if gen.name != "" {
			generated = append(generated, gen)
		}
	}

	return generated
}
