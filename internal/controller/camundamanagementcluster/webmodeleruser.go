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
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
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
	// webModelerUserApplied is the value under
	// WebModelerClusterUserAppliedKey once the cluster holds the user with
	// the password beside it.
	webModelerUserApplied = "true"
)

// authorizationSearchLimit bounds the authorizations of the Web Modeler user
// that one converge reads. The user is granted two of them, so the bound is
// only reached on a cluster where somebody else granted it hundreds more.
const authorizationSearchLimit = 1000

// defaultConvergeInterval is how long a cluster that holds the user is left
// alone before the operator reads it again. The management plane reconciles on
// every event of every selected cluster, so without a bound the check would
// run several times a second on a busy Kubernetes cluster.
const defaultConvergeInterval = 10 * time.Minute

// The events that this file records.
const (
	// eventActionRemoveUser is the action of the events about the Web Modeler
	// user on an orchestration cluster.
	eventActionRemoveUser = "RemoveWebModelerUser"
	// eventReasonUserRemovalFailed is recorded when the finalizer could not
	// remove the user. The management cluster is deleted either way, so an
	// administrator has to remove it by hand.
	eventReasonUserRemovalFailed = "WebModelerUserRemovalFailed"
	// eventReasonUserLeftBehind is recorded when the operator stops trying to
	// remove the user, because nothing on that cluster authenticates with it.
	eventReasonUserLeftBehind = "WebModelerUserLeftBehind"
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

// syncWebModelerUsers gives Web Modeler a user of its own on every attached
// basic-auth cluster, publishes its password in a Secret of the management
// namespace, and withdraws the user from every cluster the management plane
// no longer serves.
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
func (r *Reconciler) syncWebModelerUsers(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	clusters []v1.CamundaCluster,
	attached []components.AttachedCluster,
	rows []v1.AttachedClusterStatus,
) error {
	served := map[types.UID]bool{}

	var errs []error
	if mc.Spec.WebModeler != nil {
		for _, cluster := range attached {
			if cluster.AuthMethod != v1.AuthenticationMethodBasic {
				continue
			}
			served[cluster.UID] = true

			failure, err := r.webModelerUser(ctx, mc, cluster)
			if err != nil {
				errs = append(errs, err)
			}
			if failure != "" {
				markCluster(rows, cluster, v1.ReasonBasicAuthUserFailed, failure)
			}
		}
	}

	return errors.Join(append(errs, r.withdrawUnservedUsers(ctx, mc, clusters, served, false))...)
}

// convergesUsers reports whether the reconcile has a Web Modeler user to keep
// converging. Nothing watches the user API of a cluster, so a user that
// somebody removed there is found by the next read and by nothing else.
func convergesUsers(mc *v1.CamundaManagementCluster, attached []components.AttachedCluster) bool {
	if mc.Spec.WebModeler == nil {
		return false
	}

	for _, cluster := range attached {
		if cluster.AuthMethod == v1.AuthenticationMethodBasic {
			return true
		}
	}

	return false
}

// convergeInterval returns ConvergeInterval, or defaultConvergeInterval when
// unset.
func (r *Reconciler) convergeInterval() time.Duration {
	if r.ConvergeInterval > 0 {
		return r.ConvergeInterval
	}

	return defaultConvergeInterval
}

// withdrawWebModelerUsers removes the Web Modeler user from every
// orchestration cluster. The finalizer calls it, so a deleted management plane
// leaves no user behind that nobody rotates. A cluster that cannot be reached
// at that moment does not hold the deletion: the user stays, with a warning
// event that names it, because the Secret that would retry it goes with the
// management plane.
func (r *Reconciler) withdrawWebModelerUsers(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	clusters []v1.CamundaCluster,
) error {
	return r.withdrawUnservedUsers(ctx, mc, clusters, nil, true)
}

// withdrawUnservedUsers removes the user of every published Secret whose
// cluster is not in served, and deletes the Secret with it.
//
// The Secrets are the record of which users exist, so one withdrawal covers
// every way a cluster stops being served: it left the selector, it moved to
// oidc, the spec dropped Web Modeler, or Kubernetes no longer holds the
// cluster at all. A cluster is never read one at a time; clusters is the list
// the reconcile already made, and it names the cluster to call.
//
// A removal that fails keeps its Secret and returns the error, so the next
// reconcile tries again; bestEffort drops the Secret all the same, for the
// finalizer.
func (r *Reconciler) withdrawUnservedUsers(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	clusters []v1.CamundaCluster,
	served map[types.UID]bool,
	bestEffort bool,
) error {
	var users corev1.SecretList
	if err := r.APIReader.List(
		ctx, &users,
		client.InNamespace(mc.Namespace),
		client.MatchingLabels(
			labels.Managed(labels.ManagementCluster(mc.Name), components.ComponentWebModelerClusterUser),
		),
	); err != nil {
		return fmt.Errorf("listing the Web Modeler user Secrets: %w", err)
	}

	byUID := make(map[types.UID]*v1.CamundaCluster, len(clusters))
	for i := range clusters {
		byUID[clusters[i].UID] = &clusters[i]
	}

	var errs []error
	for i := range users.Items {
		published := &users.Items[i]

		uid := types.UID(published.Labels[labels.ClusterUIDKey])
		if served[uid] {
			continue
		}
		errs = append(errs, r.withdrawWebModelerUser(ctx, mc, published, byUID[uid], bestEffort))
	}

	return errors.Join(errs...)
}

// withdrawWebModelerUser removes the user from one cluster and deletes the
// Secret that published its password. A nil cluster is one that Kubernetes no
// longer holds, and leaves nothing to remove the user from. A removal that
// fails keeps the Secret, which is what makes the next reconcile try again,
// unless bestEffort says the Secret goes regardless.
func (r *Reconciler) withdrawWebModelerUser(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	published *corev1.Secret,
	cluster *v1.CamundaCluster,
	bestEffort bool,
) error {
	if cluster != nil {
		if err := r.removeWebModelerUser(ctx, mc, cluster); err != nil && !bestEffort {
			return err
		}
	}

	// The password goes with the user. A Secret that outlived the user would
	// let a later reconcile trust a credential the cluster no longer holds.
	if err := r.Delete(ctx, published); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf(
			"deleting Secret %q: %w", client.ObjectKeyFromObject(published), err,
		)
	}

	return nil
}

// removeWebModelerUser removes the Web Modeler user from one cluster.
//
// A cluster that no longer accepts basic authentication holds nothing to
// remove: no credential of that cluster authenticates any more, so the user
// row grants nobody anything and no call can delete it. Such a removal reports
// a normal event and succeeds, which is what lets the claim of the cluster go
// free. Every other failure is a warning event and an error, so the caller
// decides whether it holds the withdrawal: the sync path keeps the Secret and
// tries again, the finalizer lets the deletion go on.
func (r *Reconciler) removeWebModelerUser(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	cluster *v1.CamundaCluster,
) error {
	attached := components.AttachedCluster{
		Name:      cluster.Name,
		Namespace: cluster.Namespace,
		Version:   clusterVersion(cluster),
	}
	if cluster.Status.Gateway != nil {
		attached.RESTEndpoint = cluster.Status.Gateway.RESTEndpoint
	}

	users, failure, err := r.clusterUserClient(ctx, attached)
	if err != nil {
		return err
	}
	if failure == "" {
		if err := users.DeleteUser(ctx, components.WebModelerClusterUsername); err != nil {
			failure = err.Error()
		}
	}
	if failure == "" {
		return nil
	}

	inert, why, err := r.basicAuthGone(ctx, cluster)
	if err != nil {
		return err
	}
	name := cluster.Namespace + "/" + cluster.Name

	if inert {
		r.EventRecorder.Eventf(
			mc,
			nil,
			corev1.EventTypeNormal,
			eventReasonUserLeftBehind,
			eventActionRemoveUser,
			"The user %q stays on CamundaCluster %q, because %s. The removal reported: %s",
			components.WebModelerClusterUsername, name, why, failure,
		)

		return nil
	}

	r.EventRecorder.Eventf(
		mc,
		nil,
		corev1.EventTypeWarning,
		eventReasonUserRemovalFailed,
		eventActionRemoveUser,
		"Could not remove the user %q from CamundaCluster %q: %s",
		components.WebModelerClusterUsername, name, failure,
	)

	return fmt.Errorf(
		"removing the user %q from CamundaCluster %q: %s",
		components.WebModelerClusterUsername, name, failure,
	)
}

// basicAuthGone reports whether the Web Modeler user of a cluster
// authenticates nobody any more, and names the state that the cluster is in. A
// cluster that authenticates with oidc is such a state, and so is one whose
// CamundaPlatformConfig is gone. Neither of them takes the password that the
// operator published. It returns an error only for a failure of the Kubernetes
// API.
func (r *Reconciler) basicAuthGone(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (bool, string, error) {
	method, failure, err := r.clusterAuthMethod(ctx, cluster)
	switch {
	case err != nil:
		return false, "", err
	case failure != "":
		return true, failure, nil
	case method != v1.AuthenticationMethodBasic:
		return true, fmt.Sprintf("the cluster authenticates with %q and nobody signs in with the user", method), nil
	default:
		return false, "", nil
	}
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

// webModelerUser converges the user on one cluster. It returns the message of
// the row of that cluster when the cluster refused the work, and an error when
// the Kubernetes API did.
//
// Before the first success the password is published, then the cluster is
// called, and the Secret is marked only once the cluster holds the user. A
// crash between the two therefore leaves a Secret without the marker, and the
// next reconcile sets the password it already published rather than a second
// one that nothing has seen.
//
// A marked Secret is converged again once its converge interval has passed:
// the cluster is read, and a user that is gone there is created again under
// the published password.
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
	if published.applied && time.Since(published.convergedAt) < r.convergeInterval() {
		return "", nil
	}

	password := published.password
	applied := published.applied
	if password.Value == "" {
		// Nothing was ever published, so nothing on the cluster can hold it.
		applied = false
		if password, err = credentials.LookupOrNew(
			ctx, r.APIReader, key, components.WebModelerClusterUserPasswordKey,
		); err != nil {
			return "", err
		}
	}

	if !applied {
		if password, err = r.publishUserSecret(
			ctx, mc, cluster, key, password, time.Time{},
		); err != nil {
			return "", err
		}
	}

	users, failure, err := r.clusterUserClient(ctx, cluster)
	if err != nil || failure != "" {
		return failure, err
	}

	if failure := convergeClusterUser(ctx, users, password.Value, applied); failure != "" {
		return failure, nil
	}

	_, err = r.publishUserSecret(ctx, mc, cluster, key, password, time.Now())

	return "", err
}

// publishedUserSecret is the Web Modeler user Secret of one cluster as a
// reconcile found it.
type publishedUserSecret struct {
	// password is the published password and the Secret it came from, so a
	// republish can carry the apply precondition of a reused credential.
	password credentials.Password
	// applied reports that the cluster held the user under that password, with
	// its authorizations, at least once.
	applied bool
	// convergedAt is when the cluster last answered that it holds the user. It
	// is the zero time when the Secret carries no such stamp, which makes the
	// next reconcile converge the cluster.
	convergedAt time.Time
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

	// A stamp that no reconcile of this operator wrote parses to the zero
	// time, which converges the cluster. That is the safe answer.
	convergedAt, _ := time.Parse(
		time.RFC3339, found.Annotations[components.WebModelerClusterUserConvergedAnnotation],
	)

	return publishedUserSecret{
		password: credentials.Password{
			Value:     string(found.Data[components.WebModelerClusterUserPasswordKey]),
			SourceUID: found.UID,
		},
		applied:     string(found.Data[components.WebModelerClusterUserAppliedKey]) == webModelerUserApplied,
		convergedAt: convergedAt,
	}, nil
}

// publishUserSecret applies the Secret that publishes the password of the user
// on one cluster, and returns the password bound to the Secret the server now
// holds, so that a later apply of the same Secret carries the rotation
// precondition of the credential.
//
// converged is when the cluster answered that it holds the user under this
// password, with its authorizations. It writes the applied marker and the
// stamp that the next converge is due from. The zero time publishes the
// password alone, which is what the operator does before it calls the cluster.
func (r *Reconciler) publishUserSecret(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	cluster components.AttachedCluster,
	key client.ObjectKey,
	password credentials.Password,
	converged time.Time,
) (credentials.Password, error) {
	data := map[string][]byte{
		components.WebModelerClusterUserPasswordKey: []byte(password.Value),
	}
	annotations := password.PreconditionAnnotations()
	if !converged.IsZero() {
		data[components.WebModelerClusterUserAppliedKey] = []byte(webModelerUserApplied)
		if annotations == nil {
			annotations = map[string]string{}
		}
		annotations[components.WebModelerClusterUserConvergedAnnotation] =
			converged.UTC().Format(time.RFC3339)
	}

	secret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        key.Name,
			Namespace:   key.Namespace,
			Labels:      webModelerUserLabels(mc, cluster),
			Annotations: annotations,
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
	if err := ctrl.SetControllerReference(mc, secret, r.Scheme); err != nil {
		return password, fmt.Errorf("owning Secret %q: %w", key, err)
	}

	//nolint:staticcheck // the repository applies through the deprecated client.Apply patch
	if err := r.componentClient.Patch(
		ctx, secret, client.Apply,
		client.FieldOwner(components.FieldManager), client.ForceOwnership,
	); err != nil {
		return password, fmt.Errorf("publishing Secret %q: %w", key, err)
	}
	password.SourceUID = secret.UID

	return password, nil
}

// webModelerUserLabels returns the labels of a user Secret: the ones every
// managed resource carries, and the cluster the user belongs to. The name of
// the Secret holds only a hash of the cluster UID, so without them nothing on
// the object says which cluster it serves, and the withdrawal has nothing to
// select on.
func webModelerUserLabels(
	mc *v1.CamundaManagementCluster,
	cluster components.AttachedCluster,
) map[string]string {
	set := labels.Managed(labels.ManagementCluster(mc.Name), components.ComponentWebModelerClusterUser)
	set[labels.ClusterKey] = labels.OwnerName(cluster.Name)
	set[labels.ClusterNamespaceKey] = cluster.Namespace
	set[labels.ClusterUIDKey] = string(cluster.UID)

	return set
}

// convergeClusterUser makes the cluster hold the Web Modeler user under
// password, with the authorizations it needs. applied says that the cluster
// answered the same thing before, which is what tells a first pass from a
// repair.
//
// It returns the message of the row of the cluster, or an empty string when
// the cluster holds the user.
func convergeClusterUser(
	ctx context.Context,
	users *camundaadmin.UserClient,
	password string,
	applied bool,
) string {
	user := camundaadmin.User{
		Username: components.WebModelerClusterUsername,
		Name:     components.WebModelerClusterUsername,
		Email:    webModelerUserEmail,
	}

	if failure := clusterUser(ctx, users, user, password, applied); failure != "" {
		return failure
	}

	return clusterAuthorizations(ctx, users, user.Username)
}

// clusterUser makes the cluster hold user under password.
//
// Until the cluster answered once, every pass sets the password. That takes
// over a user of the same name that the cluster held before the operator did,
// and one that a failed pass left behind. Afterwards the cluster already holds
// the published password, so a pass reads, and creates the user again only
// when the cluster lost it.
func clusterUser(
	ctx context.Context,
	users *camundaadmin.UserClient,
	user camundaadmin.User,
	password string,
	applied bool,
) string {
	if applied {
		// The read is eventually consistent, so it can report a user absent
		// that is there. The create below then reports the name as taken, and
		// the password is set to the published one, which it already is.
		_, exists, err := users.GetUser(ctx, user.Username)
		if err != nil {
			return fmt.Sprintf("Reading the user %q: %v", user.Username, err)
		}
		if exists {
			return ""
		}
	}

	switch err := users.CreateUser(ctx, user, password); {
	case errors.Is(err, camundaadmin.ErrAlreadyExists):
		if err := users.UpdateUserPassword(ctx, user, password); err != nil {
			return fmt.Sprintf("Setting the password of the user %q: %v", user.Username, err)
		}
	case err != nil:
		return fmt.Sprintf("Creating the user %q: %v", user.Username, err)
	}

	return ""
}

// clusterAuthorizations grants username the permissions of
// webModelerAuthorizations that the cluster does not already hold. The grant
// endpoint creates rather than converges, so a permission granted twice leaves
// two rows and no more access. The search is what keeps the grants to the ones
// that are missing.
func clusterAuthorizations(ctx context.Context, users *camundaadmin.UserClient, username string) string {
	granted, err := users.SearchAuthorizations(
		ctx, camundaadmin.OwnerUser, username, authorizationSearchLimit,
	)
	if err != nil {
		return fmt.Sprintf("Reading the authorizations of the user %q: %v", username, err)
	}

	desired := make([]camundaadmin.Authorization, 0, len(webModelerAuthorizations))
	for _, authorization := range webModelerAuthorizations {
		authorization.OwnerID = username
		desired = append(desired, authorization)
	}

	for _, missing := range camundaadmin.MissingAuthorizations(desired, granted) {
		if err := users.CreateAuthorization(ctx, missing); err != nil {
			return fmt.Sprintf(
				"Granting %v on %s to the user %q: %v",
				missing.PermissionTypes, missing.ResourceType, username, err,
			)
		}
	}

	return ""
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
