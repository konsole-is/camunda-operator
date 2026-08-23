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
	"errors"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaadmin"
	clustercomponents "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// The identity of the Web Modeler user on an orchestration cluster.
const (
	// webModelerUserEmail is the address the user is created with. The domain
	// is the one RFC 2606 reserves for documentation, so the operator never
	// claims an address that somebody owns.
	webModelerUserEmail = "web-modeler@example.com"
	// webModelerUserComponent is the component label of the Secrets that hold
	// the passwords of these users.
	webModelerUserComponent = "web-modeler-cluster-user"
	// webModelerUserApplied is the value under
	// WebModelerClusterUserAppliedKey once the cluster holds the user with
	// the password beside it.
	webModelerUserApplied = "true"
)

// The events that this file records.
const (
	// eventActionRemoveUser is the action of the events about the Web Modeler
	// user on an orchestration cluster.
	eventActionRemoveUser = "RemoveWebModelerUser"
	// eventReasonUserRemovalFailed is recorded when the finalizer could not
	// remove the user. The management cluster is deleted either way, so an
	// administrator has to remove it by hand.
	eventReasonUserRemovalFailed = "WebModelerUserRemovalFailed"
)

// webModelerAuthorizations are the permissions that the Web Modeler user
// needs on an orchestration cluster that enables authorizations. They are the
// set the Camunda documentation names for the two things a person does from
// Web Modeler.
//
// Deploying a diagram needs CREATE on the RESOURCE resource type
// (https://docs.camunda.io/docs/components/modeler/web-modeler/run-or-publish-your-process/#before-deploying-a-process).
// Starting an instance needs CREATE_PROCESS_INSTANCE on PROCESS_DEFINITION
// (https://docs.camunda.io/docs/components/modeler/web-modeler/idp/idp-configuration/#cluster-requirements).
// The two read permissions let the person see the definition they deployed
// and the instance they started
// (https://docs.camunda.io/docs/components/modeler/web-modeler/run-or-publish-your-process/#run-manually-from-modeler).
//
// The owner id is empty here and filled with the username of each cluster.
var webModelerAuthorizations = []camundaadmin.Authorization{
	{
		OwnerType:       camundaadmin.OwnerUser,
		ResourceType:    "RESOURCE",
		ResourceID:      "*",
		PermissionTypes: []string{"CREATE"},
	},
	{
		OwnerType:    camundaadmin.OwnerUser,
		ResourceType: "PROCESS_DEFINITION",
		ResourceID:   "*",
		PermissionTypes: []string{
			"CREATE_PROCESS_INSTANCE", "READ_PROCESS_DEFINITION", "READ_PROCESS_INSTANCE",
		},
	},
}

// webModelerUsers gives Web Modeler a user of its own on every attached
// basic-auth cluster and publishes its password in a Secret of the management
// namespace.
//
// Web Modeler asks a person for the credentials of a basic-auth cluster in its
// deploy dialog: no setting of Web Modeler carries them. This user is what
// they type, so that deploying from Web Modeler never needs the administrator
// of the orchestration cluster.
//
// What one cluster answers is a row of that cluster: a refused call reports
// BasicAuthUserFailed in status.clusters and the reconcile carries on. Only a
// failure of the Kubernetes API comes back as an error, because the operator
// cannot publish the password it just set.
func (r *Reconciler) webModelerUsers(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	clusters []components.AttachedCluster,
	rows []v1.AttachedClusterStatus,
) error {
	if mc.Spec.WebModeler == nil {
		return nil
	}

	var firstErr error
	for _, cluster := range clusters {
		if cluster.AuthMethod != v1.AuthenticationMethodBasic {
			continue
		}

		failure, err := r.webModelerUser(ctx, mc, cluster)
		if err != nil && firstErr == nil {
			firstErr = err
		}
		if failure != "" {
			markCluster(rows, cluster, v1.ReasonBasicAuthUserFailed, failure)
		}
	}

	return firstErr
}

// markCluster records a reason and a message on the row of one cluster. It
// leaves Attached alone. The management plane still serves the cluster and
// Web Modeler still lists it. Only the user of Web Modeler is missing.
func markCluster(
	rows []v1.AttachedClusterStatus,
	cluster components.AttachedCluster,
	reason, message string,
) {
	for i := range rows {
		if rows[i].Name == cluster.Name && rows[i].Namespace == cluster.Namespace {
			rows[i].Reason = reason
			rows[i].Message = message

			return
		}
	}
}

// webModelerUser converges the user on one cluster. It returns the message of
// the row of that cluster when the cluster refused the work, and an error when
// the Kubernetes API did.
//
// The password is published only after the cluster holds it, under a marker
// that says so. A crash between the two therefore leaves a Secret without the
// marker, and the next reconcile generates another password and sets that one
// instead of trusting a value the cluster may never have seen.
func (r *Reconciler) webModelerUser(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	cluster components.AttachedCluster,
) (string, error) {
	key := client.ObjectKey{
		Namespace: mc.Namespace,
		Name:      components.WebModelerClusterUserSecretName(mc, cluster.UID),
	}

	published, err := r.publishedUser(ctx, key)
	if err != nil {
		return "", err
	}
	if published.applied {
		return "", nil
	}

	password := published.password
	if password.Value == "" {
		if password, err = credentials.LookupOrNew(
			ctx, r.APIReader, key, components.WebModelerClusterUserPasswordKey,
		); err != nil {
			return "", err
		}
	}

	users, failure, err := r.clusterUserClient(ctx, cluster)
	if err != nil || failure != "" {
		return failure, err
	}

	if failure := ensureClusterUser(ctx, users, password.Value); failure != "" {
		return failure, nil
	}

	return "", r.publishUserSecret(ctx, mc, key, password)
}

// publishedUserSecret is the Web Modeler user Secret of one cluster as a
// reconcile found it.
type publishedUserSecret struct {
	// password is the published password and the Secret it came from, so a
	// republish can carry the apply precondition of a reused credential.
	password credentials.Password
	// applied reports that the cluster holds the user under that password,
	// with its authorizations.
	applied bool
}

// publishedUser reads the user Secret at key. A Secret that is not there is
// the zero value, which means that nothing has been published yet.
func (r *Reconciler) publishedUser(
	ctx context.Context,
	key client.ObjectKey,
) (publishedUserSecret, error) {
	var found corev1.Secret
	if err := r.APIReader.Get(ctx, key, &found); err != nil {
		if apierrors.IsNotFound(err) {
			return publishedUserSecret{}, nil
		}
		return publishedUserSecret{}, fmt.Errorf("reading Secret %q: %w", key, err)
	}

	return publishedUserSecret{
		password: credentials.Password{
			Value:     string(found.Data[components.WebModelerClusterUserPasswordKey]),
			SourceUID: found.UID,
		},
		applied: string(found.Data[components.WebModelerClusterUserAppliedKey]) == webModelerUserApplied,
	}, nil
}

// clusterUserClient builds a client for the user API of one cluster,
// authenticated as the administrator of that cluster. A missing administrator
// credential is a message for the row of the cluster: the cluster publishes
// its own state, and one broken cluster must not stop the management plane.
func (r *Reconciler) clusterUserClient(
	ctx context.Context,
	cluster components.AttachedCluster,
) (*camundaadmin.UserClient, string, error) {
	key := client.ObjectKey{
		Namespace: cluster.Namespace,
		// The name of the admin Secret follows from the cluster name alone.
		Name: clustercomponents.AdminSecretName(
			&v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: cluster.Name}},
		),
	}

	var admin corev1.Secret
	if err := r.APIReader.Get(ctx, key, &admin); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, fmt.Sprintf("Secret %q with the cluster administrator not found", key), nil
		}
		return nil, "", fmt.Errorf("reading Secret %q: %w", key, err)
	}

	users, err := camundaadmin.NewUserClient(camundaadmin.UserBinding{
		Endpoint: cluster.RESTEndpoint,
		Version:  cluster.Version,
		Auth: camundaadmin.Auth{
			Username: string(admin.Data[clustercomponents.AdminUsernameKey]),
			Password: string(admin.Data[clustercomponents.AdminPasswordKey]),
		},
	})
	if err != nil {
		return nil, err.Error(), nil
	}

	return users, "", nil
}

// ensureClusterUser creates the Web Modeler user with password and grants it
// the authorizations it needs. A user that the cluster already holds takes the
// new password and the same grants again. The caller reaches this function
// only while the published Secret carries no applied marker, so a user that a
// failed pass left without permissions, and one that was there before the
// operator, both end up with them. A repeated grant adds an authorization row
// and no access, because the endpoint creates rather than converges.
//
// It returns the message of the row of the cluster, or an empty string when
// the cluster holds the user.
func ensureClusterUser(ctx context.Context, users *camundaadmin.UserClient, password string) string {
	user := camundaadmin.User{
		Username: components.WebModelerClusterUsername,
		Name:     components.WebModelerClusterUsername,
		Email:    webModelerUserEmail,
	}

	switch err := users.CreateUser(ctx, user, password); {
	case errors.Is(err, camundaadmin.ErrAlreadyExists):
		if err := users.UpdateUserPassword(ctx, user, password); err != nil {
			return fmt.Sprintf("Setting the password of the user %q: %v", user.Username, err)
		}
	case err != nil:
		return fmt.Sprintf("Creating the user %q: %v", user.Username, err)
	}

	for _, authorization := range webModelerAuthorizations {
		authorization.OwnerID = user.Username
		if err := users.CreateAuthorization(ctx, authorization); err != nil {
			return fmt.Sprintf(
				"Granting %v on %s to the user %q: %v",
				authorization.PermissionTypes, authorization.ResourceType, user.Username, err,
			)
		}
	}

	return ""
}

// publishUserSecret writes the password of the user together with the marker
// that says the cluster holds it. A reused password carries its apply
// precondition, so a delete of the Secret rotates the credential instead of
// republishing one the operator can no longer prove.
func (r *Reconciler) publishUserSecret(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	key client.ObjectKey,
	password credentials.Password,
) error {
	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        key.Name,
			Namespace:   key.Namespace,
			Labels:      labels.Managed(labels.ManagementCluster(mc.Name), webModelerUserComponent),
			Annotations: password.PreconditionAnnotations(),
		},
		Type: corev1.SecretTypeOpaque,
		Data: map[string][]byte{
			components.WebModelerClusterUserPasswordKey: []byte(password.Value),
			components.WebModelerClusterUserAppliedKey:  []byte(webModelerUserApplied),
		},
	}
	if err := ctrl.SetControllerReference(mc, secret, r.Scheme); err != nil {
		return fmt.Errorf("owning Secret %q: %w", key, err)
	}

	//nolint:staticcheck // the repository applies through the deprecated client.Apply patch
	if err := r.componentClient.Patch(
		ctx, secret, client.Apply,
		client.FieldOwner(components.FieldManager), client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("publishing Secret %q: %w", key, err)
	}

	return nil
}

// removeWebModelerUsers removes the Web Modeler user from every orchestration
// cluster this management cluster created one on. The finalizer calls it, so a
// deleted management plane leaves no user behind that nobody rotates.
//
// It is best effort. A cluster that is gone, unreachable, or no longer holds
// its administrator credential records an event and does not hold the deletion
// of the management cluster: the credential travels with the Secrets, which
// Kubernetes collects with the CamundaManagementCluster.
func (r *Reconciler) removeWebModelerUsers(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) error {
	clusters, err := r.listClusters(ctx)
	if err != nil {
		return err
	}

	for i := range clusters {
		cluster := &clusters[i]
		key := client.ObjectKey{
			Namespace: mc.Namespace,
			Name:      components.WebModelerClusterUserSecretName(mc, cluster.UID),
		}

		var published corev1.Secret
		if err := r.APIReader.Get(ctx, key, &published); err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("reading Secret %q: %w", key, err)
		}

		r.removeWebModelerUser(ctx, mc, cluster)
	}

	return nil
}

// removeWebModelerUser removes the user from one cluster and records an event
// when it cannot.
func (r *Reconciler) removeWebModelerUser(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	cluster *v1.CamundaCluster,
) {
	attached := components.AttachedCluster{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
		Version:   clusterVersion(cluster),
	}
	if cluster.Status.Gateway != nil {
		attached.RESTEndpoint = cluster.Status.Gateway.RESTEndpoint
	}

	users, failure, err := r.clusterUserClient(ctx, attached)
	if err == nil && failure == "" {
		err = users.DeleteUser(ctx, components.WebModelerClusterUsername)
	}
	if err == nil && failure == "" {
		return
	}
	if failure == "" {
		failure = err.Error()
	}

	r.EventRecorder.Eventf(
		mc,
		nil,
		corev1.EventTypeWarning,
		eventReasonUserRemovalFailed,
		eventActionRemoveUser,
		"Could not remove the user %q from CamundaCluster %q: %s",
		components.WebModelerClusterUsername, cluster.Namespace+"/"+cluster.Name, failure,
	)
}
