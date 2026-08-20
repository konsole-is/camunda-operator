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
	// pendingRotation is the rotation value that staged pending. It travels
	// with it, so a promote records the request that produced the password
	// it promotes even when the spec changed, or was cleared, while the
	// rotation was in flight.
	pendingRotation string
	// rotation is the passwordRotation value that the admin Secret publishes
	// after this reconcile, under the rotation key, beside the password it
	// answers. The Secret component writes the two together.
	rotation string
	// publishedRotation is the rotation value that the admin Secret already
	// holds. status.adminPassword.rotation projects it, so the status
	// follows the Secret exactly as the connectors hash does: a rotation is
	// reported complete only once its password is durable, and a lost status
	// write is rebuilt from the Secret on the next reconcile.
	publishedRotation string
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

// recordRotation writes the adminPassword block on cluster from the rotation
// that the admin Secret already publishes. A rotation whose Secret apply
// failed is therefore never reported as complete, and a status write that
// was lost is rebuilt from the Secret on the next reconcile.
func (c adminCredential) recordRotation(cluster *v1.CamundaCluster) {
	if c.publishedRotation == "" {
		cluster.Status.AdminPassword = nil
		return
	}

	cluster.Status.AdminPassword = &v1.AdminPasswordStatus{Rotation: c.publishedRotation}
}

// stageFailure overwrites the AdminSecretReady condition of cluster, in
// memory, with the failure of this reconcile. prior is that condition as the
// reconcile read it, before the components staged their own. The admin Secret component has
// already reconciled and reported the unchanged Secret as healthy, but the
// requested rotation is not applied, so the component condition must say
// why. The deferred FlushStatus persists the staged value, and the Ready
// aggregate reads it from the in-memory cluster.
func (c adminCredential) stageFailure(cluster *v1.CamundaCluster, prior *metav1.Condition) {
	if c.failure == nil {
		return
	}

	cond := metav1.Condition{
		Type:               v1.ConditionAdminSecretReady,
		Status:             metav1.ConditionFalse,
		Reason:             c.failure.reason,
		Message:            conditions.BoundMessage(c.failure.message),
		ObservedGeneration: cluster.Generation,
	}
	// The component reported the unchanged Secret as healthy a moment ago,
	// so staging the failure flips the condition back and would stamp a new
	// transition time on it. The time belongs to the status, and the status
	// never left False, so it keeps the one it carries. A failure that is
	// unchanged in every field then stages a status that matches the server
	// exactly, the flush writes nothing, and the retry waits for its timer
	// instead of being enqueued again by its own status write. A new reason
	// or message still travels, and still writes.
	if prior != nil && prior.Status == cond.Status {
		cond.LastTransitionTime = prior.LastTransitionTime
	}

	meta.SetStatusCondition(cluster.GetStatusConditions(), cond)
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

	current, pending, applied, err := r.readAdminSecret(ctx, cluster)
	if err != nil {
		return adminCredential{}, err
	}

	requested := ""
	if auth.Basic != nil {
		requested = auth.Basic.PasswordRotation
	}

	if current.Value == "" {
		value, err := credentials.NewPassword()
		if err != nil {
			return adminCredential{}, err
		}

		// Only a cluster that never published an admin Secret seeds the
		// requested rotation: its password becomes the initial user at first
		// start, so there is nothing to update. A Secret that went away took
		// its applied rotation with it, so this replacement records none.
		// The orchestration cluster still holds the password of the deleted
		// Secret, which nobody has any more, so the next reconcile takes the
		// request to the user API and reports Rejected there. That is what
		// the authentication guide promises, and it is the point: a
		// replacement that kept the applied value would reach the steady
		// branch and report a healthy Secret that the cluster does not
		// accept.
		//
		// published is the password of this apply, not the empty value that
		// the Secret holds now: connectors would otherwise hash on "" and
		// roll again as soon as the first Secret lands. Hashing it early is
		// safe only here. A failed apply leaves no Secret at all, and the
		// next reconcile takes this branch again and generates another
		// password, so the hash keeps moving until one lands. It can never
		// stall on a password that the Secret does not hold.
		rotation := ""
		if meta.FindStatusCondition(cluster.Status.Conditions, v1.ConditionAdminSecretReady) == nil {
			rotation = requested
		}

		return adminCredential{
			password: credentials.Password{Value: value}, published: value, rotation: rotation,
		}, nil
	}

	if pending.password == "" && (requested == "" || requested == applied) {
		return adminCredential{
			password: current, published: current.Value, rotation: applied, publishedRotation: applied,
		}, nil
	}

	if in.Effective.Suspend {
		return adminCredential{
			password:          current,
			published:         current.Value,
			pending:           pending.password,
			pendingRotation:   pending.rotation,
			rotation:          applied,
			publishedRotation: applied,
		}, nil
	}

	if pending.password == "" {
		value, err := credentials.NewPassword()
		if err != nil {
			return adminCredential{}, err
		}

		return adminCredential{
			password:          current,
			published:         current.Value,
			pending:           value,
			pendingRotation:   requested,
			rotation:          applied,
			publishedRotation: applied,
		}, nil
	}

	if failure := r.updateAdminPassword(ctx, cluster, in, current.Value, pending.password); failure != nil {
		return adminCredential{
			password:          current,
			published:         current.Value,
			pending:           pending.password,
			pendingRotation:   pending.rotation,
			rotation:          applied,
			publishedRotation: applied,
			failure:           failure,
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

	// The rotation that staged the pending password is what this promote
	// applies, whatever the spec asks for now. A value that changed, or was
	// cleared, while the call was in flight never loses the request that the
	// cluster just accepted.
	return adminCredential{
		password:          credentials.Password{Value: pending.password, SourceUID: current.SourceUID},
		published:         current.Value,
		rotation:          pending.rotation,
		publishedRotation: applied,
	}, nil
}

// readAdminSecret reads the active password, the pending password, and the
// applied rotation of the admin Secret without the cache. The three travel
// in one object, so the applied rotation always describes the password
// beside it. A missing Secret, key, or value is the zero password, never an
// error: an empty credential is replaced, not kept.
func (r *CamundaClusterReconciler) readAdminSecret(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (current credentials.Password, pending pendingRotation, applied string, err error) {
	key := client.ObjectKey{Namespace: cluster.Namespace, Name: components.AdminSecretName(cluster)}

	var secret corev1.Secret
	if err := r.APIReader.Get(ctx, key, &secret); err != nil {
		if apierrors.IsNotFound(err) {
			return credentials.Password{}, pendingRotation{}, "", nil
		}
		return credentials.Password{}, pendingRotation{}, "", fmt.Errorf("reading Secret %q: %w", key, err)
	}

	value := string(secret.Data[components.AdminPasswordKey])
	if value == "" {
		return credentials.Password{}, pendingRotation{}, "", nil
	}

	return credentials.Password{Value: value, SourceUID: secret.UID},
		pendingRotation{
			password: string(secret.Data[components.AdminPendingPasswordKey]),
			rotation: string(secret.Data[components.AdminPendingRotationKey]),
		},
		string(secret.Data[components.AdminRotationKey]),
		nil
}

// pendingRotation is the rotation that the admin Secret has staged but not
// applied: the requested password and the value that asked for it. The zero
// value is no rotation in flight.
type pendingRotation struct {
	password string
	rotation string
}

// updateAdminPassword sets pending as the password of the admin user through
// the user API, authenticated with the active password first and with
// pending second. The second try is what makes a crash re-entrant: when an
// earlier reconcile got its call accepted and crashed before the promote,
// the cluster already holds the pending password, so only pending
// authenticates, and setting it again changes nothing. The outcome of the
// retry replaces the rejection that provoked it, so the reported reason is
// always the last thing the operator learned about the cluster. It returns
// nil on success and the failure to report otherwise.
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
	if errors.Is(err, camundaadmin.ErrUnauthenticated) {
		// Only a refused credential is worth a second password: the cluster
		// already holds the pending one when an earlier reconcile got its
		// call accepted and died before the promote. Every other refusal,
		// such as a profile that the cluster does not accept, answers the
		// same way to both passwords, and retrying would replace its message
		// with the bad credentials of a password that is not active yet.
		//
		// The retry decides this reconcile, so its own failure is the one to
		// report: a cluster that goes away during the retry must read
		// ConnectionFailed and clear on its own.
		err = putAdminPassword(ctx, endpoint, in.Effective.Version, pending, pending)
	}

	return rotationFailureFor(err)
}

// rotationFailureFor maps the error of a user API call onto the
// AdminSecretReady reason that reports it. A nil error is no failure. The
// reasons separate the three answers a caller acts on differently: no answer
// at all, an answer that refused the credentials, and an answer that refused
// the call.
func rotationFailureFor(err error) *rotationFailure {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, camundaadmin.ErrUnreachable):
		return &rotationFailure{reason: v1.ReasonConnectionFailed, message: err.Error()}
	// Before ErrRejected, which it travels with: a refused credential is the
	// one refusal that a new password recovers, and it reads differently to
	// a caller than a call the cluster refused on its content.
	case errors.Is(err, camundaadmin.ErrUnauthenticated):
		return &rotationFailure{reason: v1.ReasonInvalidCredentials, message: err.Error()}
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
