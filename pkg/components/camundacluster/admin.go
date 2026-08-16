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

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/secret"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// adminComponentName is the ocf name of the admin Secret component.
const adminComponentName = "admin-secret"

// AdminSecretComponent renders the admin credentials Secret of a basic-auth
// cluster: <name>-camunda-admin with the keys username (admin) and password.
// The component is gated on enabled (basic authentication): when disabled it
// deletes the Secret and reports Disabled, so a switch to OIDC removes the
// credentials. The controller reads or generates the password with
// pkg/credentials and keeps it stable across reconciles; it passes an empty
// password when disabled. The component takes part in Ready only when
// enabled.
func AdminSecretComponent(
	cluster *v1.CamundaCluster,
	enabled bool,
	password string,
) (*component.Component, error) {
	admin, err := secret.NewBuilder(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      AdminSecretName(cluster),
			Namespace: cluster.Namespace,
			Labels:    labels.Managed(labels.Cluster(cluster.Name), adminComponentName),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			AdminUsernameKey: []byte(AdminUsername),
			AdminPasswordKey: []byte(password),
		},
	}).Build()
	if err != nil {
		return nil, fmt.Errorf("building %s component: %w", adminComponentName, err)
	}

	return component.NewComponentBuilder().
		WithName(adminComponentName).
		WithConditionType(component.ConditionType(v1.ConditionAdminSecretReady)).
		WithFeatureGate(feature.NewBooleanGate(enabled)).
		WithResource(admin).
		Build()
}
