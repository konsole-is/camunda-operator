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
	"context"
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
)

// eventReasonPasswordRotated is recorded when the operator set a rotated
// admin password on the orchestration cluster.
const eventReasonPasswordRotated = "AdminPasswordRotated"

// adminCredential is the resolved admin credential of one reconcile: what
// the admin Secret publishes, the rotation value to record in status, and
// how an in-flight rotation went. The zero value is an OIDC cluster, which
// has no operator-owned credential.
type adminCredential struct {
	// password is the active password, published under the password key.
	password credentials.Password
	// published is the active password that the admin Secret already holds,
	// before this reconcile applies anything. The connectors config hash
	// takes this value and never password: connectors resolve the Secret
	// through a secretKeyRef when a pod starts, so a hash that ran ahead of
	// a Secret write that then failed would never roll them again.
	published string
	// pending is the requested password of an in-flight rotation, published
	// under the pending key. Empty when no rotation is in flight.
	pending string
	// rotation is the value of status.adminPassword.rotation to record.
	rotation string
	// failure is set when the user API rejected the rotation or did not
	// answer. The Secret keeps its active password, the controller reports
	// the failure on AdminSecretReady, and a timer retries: no watch fires
	// when the cluster recovers.
	failure *rotationFailure
}

// rotationFailure is a failed user API call, mapped to the AdminSecretReady
// reason that reports it.
type rotationFailure struct {
	reason  string
	message string
}

// recordRotation writes the adminPassword block on cluster, but only once
// the admin Secret publishes the promoted password. The Secret component
// reports its apply on AdminSecretReady, and it has already reconciled when
// the caller runs this. An apply that failed leaves the value that the
// status was read with: recording the rotation there would report it as
// complete while the Secret still publishes the old password. The pending
// password stays in the Secret, so the next reconcile promotes again.
func (c adminCredential) recordRotation(cluster *v1.CamundaCluster) {
	if c.rotation == "" {
		cluster.Status.AdminPassword = nil
		return
	}

	cond := meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionAdminSecretReady)
	if cond == nil || cond.Status != metav1.ConditionTrue {
		return
	}

	cluster.Status.AdminPassword = &v1.AdminPasswordStatus{Rotation: c.rotation}
}

// stageFailure overwrites the AdminSecretReady condition of cluster, in
// memory, with the failure of this reconcile. The admin Secret component has
// already reconciled and reported the unchanged Secret as healthy, but the
// requested rotation is not applied, so the component condition must say
// why. The deferred FlushStatus persists the staged value, and the Ready
// aggregate reads it from the in-memory cluster.
func (c adminCredential) stageFailure(cluster *v1.CamundaCluster) {
	if c.failure == nil {
		return
	}

	meta.SetStatusCondition(cluster.GetStatusConditions(), metav1.Condition{
		Type:               v1.ConditionAdminSecretReady,
		Status:             metav1.ConditionFalse,
		Reason:             c.failure.reason,
		Message:            conditions.BoundMessage(c.failure.message),
		ObservedGeneration: cluster.Generation,
	})
}

// resolveAdminCredential resolves what the admin Secret publishes this
// reconcile and drives a requested password rotation. A rotation takes two
// reconciles. The first one generates the new password and returns it as
// pending, so the component makes it durable in the Secret before any call;
// a crash can then never lose a password that the cluster may already hold.
// The second one sets the pending password on the admin user through the
// user API and, only on success, promotes it to the active password and
// records the rotation. A failed call comes back as a failure on the
// credential, never as an error: the caller reports it on AdminSecretReady
// and retries on a timer.
//
// A Secret without a password gets a new one. A cluster that never published
// an admin Secret records the requested rotation with it: that password
// seeds the initial user at first start, so there is nothing to update. A
// Secret that went away keeps the recorded rotation instead, because the
// cluster still holds the password of the deleted Secret; the request then
// goes to the user API and fails with Rejected there. A suspended cluster
// serves no user API, so a requested rotation waits, and an in-flight one
// stays pending, until the cluster resumes. An error is a transient read
// failure or an exhausted entropy source.
func (r *CamundaClusterReconciler) resolveAdminCredential(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	in components.Input,
) (adminCredential, error) {
	auth := components.ResolveAuth(in)
	if auth.Method != v1.AuthenticationMethodBasic {
		return adminCredential{}, nil
	}

	current, pending, err := r.readAdminSecret(ctx, cluster)
	if err != nil {
		return adminCredential{}, err
	}

	requested := ""
	if auth.Basic != nil {
		requested = auth.Basic.PasswordRotation
	}
	recorded := ""
	if cluster.Status.AdminPassword != nil {
		recorded = cluster.Status.AdminPassword.Rotation
	}

	if current.Value == "" {
		value, err := credentials.NewPassword()
		if err != nil {
			return adminCredential{}, err
		}

		// Only a cluster that never published an admin Secret seeds the
		// requested rotation: its password becomes the initial user at first
		// start, so there is nothing to update. A Secret that went away keeps
		// the recorded rotation. The cluster still holds the password of the
		// deleted Secret, so the next reconcile takes the request to the user
		// API and reports Rejected there.
		//
		// published stays empty: the Secret holds no password yet, so the
		// connectors of a new cluster hash on "" for one reconcile. They are
		// being created in that reconcile anyway.
		rotation := recorded
		if meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionAdminSecretReady) == nil {
			rotation = requested
		}

		return adminCredential{password: credentials.Password{Value: value}, rotation: rotation}, nil
	}

	if pending == "" && (requested == "" || requested == recorded) {
		return adminCredential{password: current, published: current.Value, rotation: recorded}, nil
	}

	if in.Effective.Suspend {
		return adminCredential{
			password: current, published: current.Value, pending: pending, rotation: recorded,
		}, nil
	}

	if pending == "" {
		value, err := credentials.NewPassword()
		if err != nil {
			return adminCredential{}, err
		}

		return adminCredential{password: current, published: current.Value, pending: value, rotation: recorded}, nil
	}

	if failure := r.updateAdminPassword(ctx, cluster, in, current.Value, pending); failure != nil {
		return adminCredential{
			password: current, published: current.Value, pending: pending, rotation: recorded, failure: failure,
		}, nil
	}

	r.EventRecorder.Eventf(
		cluster,
		nil,
		corev1.EventTypeNormal,
		eventReasonPasswordRotated,
		eventActionReconcile,
		"Rotated the admin password through the user API",
	)

	return adminCredential{
		password:  credentials.Password{Value: pending, SourceUID: current.SourceUID},
		published: current.Value,
		rotation:  requested,
	}, nil
}

// readAdminSecret reads the active and the pending password of the admin
// Secret without the cache. A missing Secret, key, or value is the zero
// password, never an error: an empty credential is replaced, not kept.
func (r *CamundaClusterReconciler) readAdminSecret(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (current credentials.Password, pending string, err error) {
	key := client.ObjectKey{Namespace: cluster.Namespace, Name: components.AdminSecretName(cluster)}

	var secret corev1.Secret
	if err := r.APIReader.Get(ctx, key, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return credentials.Password{}, "", nil
		}
		return credentials.Password{}, "", fmt.Errorf("reading Secret %q: %w", key, err)
	}

	value := string(secret.Data[components.AdminPasswordKey])
	if value == "" {
		return credentials.Password{}, "", nil
	}

	return credentials.Password{Value: value, SourceUID: secret.UID},
		string(secret.Data[components.AdminPendingPasswordKey]),
		nil
}

// updateAdminPassword sets pending as the password of the admin user through
// the user API, authenticated with the active password first and with
// pending second. The second try is what makes a crash re-entrant: when an
// earlier reconcile got its call accepted and crashed before the promote,
// the cluster already holds the pending password, so only pending
// authenticates, and setting it again changes nothing. It returns nil on
// success and the failure to report otherwise.
func (r *CamundaClusterReconciler) updateAdminPassword(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	in components.Input,
	active string,
	pending string,
) *rotationFailure {
	endpoint := components.RESTEndpoint(cluster, in.Effective)
	if r.RESTEndpoint != nil {
		endpoint = r.RESTEndpoint(cluster, in.Effective)
	}

	err := putAdminPassword(ctx, endpoint, in.Effective.Version, active, pending)
	if errors.Is(err, camundaadmin.ErrRejected) {
		if retryErr := putAdminPassword(ctx, endpoint, in.Effective.Version, pending, pending); retryErr == nil {
			return nil
		}
	}

	switch {
	case err == nil:
		return nil
	case errors.Is(err, camundaadmin.ErrUnreachable):
		return &rotationFailure{reason: v1.ReasonConnectionFailed, message: err.Error()}
	case errors.Is(err, camundaadmin.ErrRejected):
		return &rotationFailure{reason: v1.ReasonRejected, message: err.Error()}
	default:
		return &rotationFailure{reason: v1.ReasonInvalidReference, message: err.Error()}
	}
}

// putAdminPassword sends one update of the admin user, authenticated as that
// user with authPassword. The seeded name and email travel with it, because
// the endpoint replaces the whole profile.
func putAdminPassword(ctx context.Context, endpoint, version, authPassword, password string) error {
	users, err := camundaadmin.NewUserClient(camundaadmin.UserBinding{
		Endpoint: endpoint,
		Version:  version,
		Auth:     camundaadmin.Auth{Username: components.AdminUsername, Password: authPassword},
	})
	if err != nil {
		return err
	}

	return users.UpdateUserPassword(
		ctx, camundaadmin.User{
			Username: components.AdminUsername,
			Name:     components.AdminUsername,
			Email:    components.AdminEmail,
		}, password,
	)
}
