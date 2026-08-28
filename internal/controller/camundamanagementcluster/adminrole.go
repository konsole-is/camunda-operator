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
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/keycloakadmin"
)

// optimizeRealmRole is the realm role that a person needs to open Optimize.
// Management Identity creates it with the Optimize client, in the preset that
// the first Optimize of a management plane switches on
// (https://docs.camunda.io/docs/self-managed/components/management-identity/miscellaneous/starting-configuration/).
const optimizeRealmRole = "Optimize"

// The event that the controller records when it grants the Optimize role.
const eventReasonAdminRoleGranted = "AdminOptimizeRoleGranted"

// grantAdminOptimizeRole gives the first administrator the Optimize realm role
// when the realm holds that role and the administrator does not hold it yet.
//
// It converges rather than running once, so a role that somebody takes away in
// Keycloak is given back by the next call. It serves the user that
// spec.identity.admin.username names and nobody else, and it adds a role
// without ever taking one away.
//
// The failure it returns names the state on OptimizeCallbacksReady, which the
// caller folds into Ready. Only a failure of the Kubernetes API comes back as
// an error.
func (r *Reconciler) grantAdminOptimizeRole(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	provider components.IdentityProvider,
) (*conditions.PreCheckFailure, error) {
	admin, failure, err := r.keycloakAdmin(ctx, mc, provider)
	if err != nil || failure != nil {
		return failure, err
	}

	username := mc.Spec.Identity.Admin.Username
	userID, err := admin.FindUser(ctx, username)
	if err != nil {
		return connectionFailed(err), nil
	}
	if userID == "" {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonAdminRoleGrantFailed,
			Message: fmt.Sprintf(
				"The realm holds no user %q, so that user holds no %q role. Management Identity "+
					"creates the first administrator while it starts for the first time",
				username, optimizeRealmRole,
			),
		}, nil
	}

	role, err := admin.FindRealmRole(ctx, optimizeRealmRole)
	if err != nil {
		return connectionFailed(err), nil
	}
	if role == nil {
		return &conditions.PreCheckFailure{
			Reason: v1.ReasonAdminRoleGrantFailed,
			Message: fmt.Sprintf(
				"The realm holds no %q role, so user %q cannot hold it. Management Identity "+
					"creates that role while it starts, so restart Management Identity to have "+
					"it created again",
				optimizeRealmRole, username,
			),
		}, nil
	}

	held, err := admin.UserRealmRoles(ctx, userID)
	if err != nil {
		return connectionFailed(err), nil
	}
	if slices.ContainsFunc(held, func(r keycloakadmin.RealmRole) bool {
		return r.Name == optimizeRealmRole
	}) {
		return nil, nil
	}

	if err := admin.AddUserRealmRole(ctx, userID, *role); err != nil {
		return &conditions.PreCheckFailure{
			Reason:  v1.ReasonAdminRoleGrantFailed,
			Message: conditions.BoundMessage(err.Error()),
		}, nil
	}
	r.EventRecorder.Eventf(
		mc,
		nil,
		corev1.EventTypeNormal,
		eventReasonAdminRoleGranted,
		eventActionUpdate,
		"User %q of realm %q now holds the %q role",
		username,
		provider.Realm,
		optimizeRealmRole,
	)

	return nil, nil
}

// connectionFailed reports a read of the realm that did not answer.
func connectionFailed(err error) *conditions.PreCheckFailure {
	return &conditions.PreCheckFailure{
		Reason:  v1.ReasonConnectionFailed,
		Message: conditions.BoundMessage(err.Error()),
	}
}
