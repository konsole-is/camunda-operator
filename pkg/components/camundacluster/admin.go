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
	"github.com/konsole-is/camunda-operator/pkg/credentials"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// adminComponentName is the ocf name of the admin Secret component.
const adminComponentName = "admin-secret"

// AdminSecretState is everything the admin Secret publishes about the admin
// credential: the active password, and the bookkeeping of a rotation. The
// four travel in one apply, so the record of which request produced a
// password can never disagree with the password beside it. An empty
// bookkeeping value leaves its key out.
type AdminSecretState struct {
	// Password is the active password, under AdminPasswordKey.
	Password credentials.Password
	// Email is the address that the orchestration cluster holds for the
	// admin user, under AdminEmailKey. The processes read their seed from
	// this key, so a changed address never touches a pod template. An empty
	// value leaves the key out: the operator records an address only once
	// the cluster has accepted it, and never one of its own invention.
	Email string
	// PendingPassword is the requested password of a rotation in flight,
	// under AdminPendingPasswordKey.
	PendingPassword string
	// PendingRotation is the rotation value that staged PendingPassword,
	// under AdminPendingRotationKey.
	PendingRotation string
	// Rotation is the rotation value that produced Password, under
	// AdminRotationKey.
	Rotation string
}

// AdminSecretComponent renders the admin credentials Secret of a basic-auth
// cluster: <name>-camunda-admin with the keys username (admin) and password.
// The component is gated on enabled (basic authentication): when disabled it
// deletes the Secret and reports Disabled, so a switch to OIDC removes the
// credentials. The controller reads or generates the password with
// pkg/credentials and keeps it stable across reconciles; it passes the zero
// password when disabled. The component takes part in Ready only when
// enabled.
//
// While a password rotation is in flight, pendingPassword carries the
// requested password and the Secret publishes it under password-pending,
// next to the active one. The key makes the requested password durable
// before the controller sets it through the user API, so a crash between
// the call and the publish can never lose it. An empty pendingPassword
// leaves the key out, which removes it when the rotation completes.
//
// rotation is the spec.auth.basic.passwordRotation value that produced the
// password, published under password-rotation. It travels in the same apply
// as the password it answers, so the record of which request is applied is
// durable and can never disagree with the password beside it. The
// controller reads it back to decide whether a rotation is still needed.
//
// A reused password carries its apply precondition onto the Secret, so a
// delete of the Secret always rotates the password. The controller must
// reconcile the component through credentials.NewApplyClient for the
// precondition to hold.
func AdminSecretComponent(
	cluster *v1.CamundaCluster,
	enabled bool,
	state AdminSecretState,
) (*component.Component, error) {
	data := map[string][]byte{
		AdminUsernameKey: []byte(AdminUsername),
		AdminPasswordKey: []byte(state.Password.Value),
	}
	for key, value := range map[string]string{
		AdminEmailKey:           state.Email,
		AdminPendingPasswordKey: state.PendingPassword,
		AdminPendingRotationKey: state.PendingRotation,
		AdminRotationKey:        state.Rotation,
	} {
		if value != "" {
			data[key] = []byte(value)
		}
	}

	admin, err := secret.NewBuilder(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:        AdminSecretName(cluster),
			Namespace:   cluster.Namespace,
			Labels:      labels.Managed(labels.Cluster(cluster.Name), adminComponentName),
			Annotations: state.Password.PreconditionAnnotations(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
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
