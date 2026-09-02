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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// externalKeycloakCluster is a management plane in the externalKeycloak mode
// that signs in to the realm of these tests at url with the Secret named
// secret.
func externalKeycloakCluster(url, secret string) *v1.CamundaManagementCluster {
	return &v1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "my-management-ns", Name: "my-management", UID: "management-uid",
		},
		Spec: v1.CamundaManagementClusterSpec{
			IdentityProvider: v1.IdentityProviderSpec{
				ExternalKeycloak: &v1.ExternalKeycloakSpec{
					URL:   url,
					Realm: "camunda-platform",
					AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
						Name:        secret,
						UsernameKey: "username",
						PasswordKey: "password",
					},
				},
			},
		},
	}
}

// A rotation of the administrator Secret changes no realm, so a record from
// before it can still name the Secret that was replaced. The deletion signs
// in with the Secret of the spec first, and it keeps the recorded one as the
// second try, for a spec whose Secret is the broken one.
func TestWithdrawalRealmsTriesTheSpecThenTheRecordOfOneRealm(t *testing.T) {
	t.Parallel()

	mc := externalKeycloakCluster("https://kc.example.com/auth", "rotated")
	mc.Status.CallbackRealm = &v1.KeycloakRealmTarget{
		URL:   "https://kc.example.com/auth",
		Realm: "camunda-platform",
		AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
			Name:        "replaced",
			UsernameKey: "username",
			PasswordKey: "password",
		},
	}

	r, _ := fakeReconciler(t, mc)

	realms, err := r.withdrawalRealms(context.Background(), mc)
	require.NoError(t, err)

	require.Len(t, realms, 2)
	require.NotNil(t, realms[0].AdminCredentials)
	assert.Equal(t, "rotated", realms[0].AdminCredentials.Name)
	require.NotNil(t, realms[1].AdminCredentials)
	assert.Equal(t, "replaced", realms[1].AdminCredentials.Name)
}

// A record that names what the spec names is the same try twice.
func TestWithdrawalRealmsOfAnUnchangedRecord(t *testing.T) {
	t.Parallel()

	mc := externalKeycloakCluster("https://kc.example.com/auth", "keycloak-admin")
	mc.Status.CallbackRealm = &v1.KeycloakRealmTarget{
		URL:   "https://kc.example.com/auth",
		Realm: "camunda-platform",
		AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
			Name:        "keycloak-admin",
			UsernameKey: "username",
			PasswordKey: "password",
		},
	}

	r, _ := fakeReconciler(t, mc)

	realms, err := r.withdrawalRealms(context.Background(), mc)
	require.NoError(t, err)

	require.Len(t, realms, 1)
	assert.Equal(t, "https://kc.example.com/auth", realms[0].KeycloakURL)
}

// The spec names another realm than the record, so the record is the only way
// back to the realm that holds the callbacks.
func TestWithdrawalRealmsKeepTheRecordOfAnotherRealm(t *testing.T) {
	t.Parallel()

	mc := externalKeycloakCluster("https://new.example.com/auth", "new-admin")
	mc.Status.CallbackRealm = &v1.KeycloakRealmTarget{
		URL:   "https://old.example.com/auth",
		Realm: "camunda-platform",
		AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
			Name:        "old-admin",
			UsernameKey: "username",
			PasswordKey: "password",
		},
	}

	r, _ := fakeReconciler(t, mc)

	realms, err := r.withdrawalRealms(context.Background(), mc)
	require.NoError(t, err)

	require.Len(t, realms, 1)
	assert.Equal(t, "https://old.example.com/auth", realms[0].KeycloakURL)
	require.NotNil(t, realms[0].AdminCredentials)
	assert.Equal(t, "old-admin", realms[0].AdminCredentials.Name)
}

// A plane that recorded no realm still runs a Management Identity against the
// realm of the spec, so that realm is the one to tidy. Its claim on that realm
// is what says it was ever in there.
func TestWithdrawalRealmsFallBackToTheSpec(t *testing.T) {
	mc := externalKeycloakCluster("https://kc.example.com/auth", "keycloak-admin")
	target := v1.KeycloakRealmTarget{URL: "https://kc.example.com/auth", Realm: "camunda-platform"}
	lease := components.NewRealmClaimLease(testClaimNamespace, target, mc)
	r, _ := fakeReconciler(t, mc, lease)

	realms, err := r.withdrawalRealms(context.Background(), mc)
	require.NoError(t, err)

	require.Len(t, realms, 1)
	assert.Equal(t, "https://kc.example.com/auth", realms[0].KeycloakURL)
	assert.Equal(t, "camunda-platform", realms[0].Realm)
}

// A plane parked on the realm of another plane keeps the contract of the realm
// it left, and its spec names a realm it never entered. Deleting it must leave
// the callbacks of the holder alone.
func TestWithdrawalRealmsLeaveARealmThisPlaneNeverClaimed(t *testing.T) {
	mc := externalKeycloakCluster("https://kc.example.com/auth", "keycloak-admin")
	r, _ := fakeReconciler(t, mc)

	realms, err := r.withdrawalRealms(context.Background(), mc)

	require.NoError(t, err)
	assert.Empty(t, realms)
}

// The operator runs the Keycloak of the keycloak mode and claims no realm in
// it, so the spec is the only way to a client that Identity made there.
func TestWithdrawalRealmsOfTheKeycloakModeNeedNoClaim(t *testing.T) {
	mc := &v1.CamundaManagementCluster{
		ObjectMeta: metav1.ObjectMeta{Namespace: "my-management-ns", Name: "my-management"},
		Spec: v1.CamundaManagementClusterSpec{
			IdentityProvider: v1.IdentityProviderSpec{Keycloak: &v1.ManagedKeycloakSpec{}},
		},
	}
	r, _ := fakeReconciler(t, mc)

	realms, err := r.withdrawalRealms(context.Background(), mc)

	require.NoError(t, err)
	assert.Len(t, realms, 1)
}

// A read that fails leaves the realms to withdraw from unknown, and the
// deletion releases the claim on every realm right after, so the pass stops
// instead of taking the failure for a plane that claimed nothing.
func TestWithdrawalRealmsFailWhenTheClaimCannotBeRead(t *testing.T) {
	mc := externalKeycloakCluster("https://kc.example.com/auth", "keycloak-admin")
	target := v1.KeycloakRealmTarget{URL: "https://kc.example.com/auth", Realm: "camunda-platform"}
	lease := components.NewRealmClaimLease(testClaimNamespace, target, mc)
	r, log := fakeReconciler(t, mc, lease)
	log.getFails = lease.Name

	_, err := r.withdrawalRealms(context.Background(), mc)

	require.Error(t, err)
}

// The oidc mode registers nothing, so a plane that recorded no realm has
// nothing to withdraw from.
func TestWithdrawalRealmsOfTheOIDCModeWithoutARecord(t *testing.T) {
	t.Parallel()

	mc := &v1.CamundaManagementCluster{
		Spec: v1.CamundaManagementClusterSpec{
			IdentityProvider: v1.IdentityProviderSpec{OIDC: &v1.ManagementOIDCSpec{}},
		},
	}

	r, _ := fakeReconciler(t, mc)

	realms, err := r.withdrawalRealms(context.Background(), mc)

	require.NoError(t, err)
	assert.Empty(t, realms)
}

// finalizerRealm is the realm that the specs below claim.
var finalizerRealm = v1.KeycloakRealmTarget{
	URL: "https://keycloak.example.com/auth", Realm: "camunda-platform",
}

// TestFinalizeStopsIdentityBeforeTheRealm covers the deletion of a management
// plane that never wrote a contract, or whose contract somebody removed. The
// contract says only whether this plane has login callbacks to withdraw. It
// says nothing about the Management Identity Deployment, which writes the
// clients of the realm and must be gone before another plane claims it.
func TestFinalizeStopsIdentityBeforeTheRealm(t *testing.T) {
	t.Run("the workloads go before the realm claim, with no contract", func(t *testing.T) {
		mc := finalizingCluster()
		identity := ownedIdentity(mc)
		set := ownedIdentitySet(mc, identity)
		lease := components.NewRealmClaimLease(testClaimNamespace, finalizerRealm, mc)
		r, deletes := fakeReconciler(t, mc, identity, set, lease)

		require.NoError(t, r.finalize(context.Background(), mc))

		assert.Equal(t, []string{identity.Name, set.Name, lease.Name}, deletes.names)
		assert.False(t, exists(t, r, identity))
		assert.False(t, exists(t, r, set))
		assert.False(t, exists(t, r, lease))
		assert.False(t, controllerutil.ContainsFinalizer(mc, Finalizer))
	})

	// The name of the Deployment is derived from the name of the management
	// cluster, and the labels of a ReplicaSet are the discovery labels of the
	// plane, so a workload of another owner can carry either. That workload
	// keeps writing the realm, and this pass cannot stop it, so the realm
	// stays claimed and the object stays.
	t.Run("the workloads of another owner stay, with the realm claim", func(t *testing.T) {
		mc := finalizingCluster()
		identity := ownedIdentity(mc)
		set := ownedIdentitySet(mc, identity)
		identity.OwnerReferences = nil
		lease := components.NewRealmClaimLease(testClaimNamespace, finalizerRealm, mc)
		r, _ := fakeReconciler(t, mc, identity, set, lease)

		require.Error(t, r.finalize(context.Background(), mc))

		assert.True(t, exists(t, r, identity))
		assert.True(t, exists(t, r, set))
		assert.True(t, exists(t, r, lease))
		assert.True(t, controllerutil.ContainsFinalizer(mc, Finalizer))
	})

	// A pass that stopped between the two deletions leaves ReplicaSets that
	// still start Management Identity. The next pass finds no Deployment, and
	// it must not hand the realm on with them still there.
	t.Run("a pass that stopped after the Deployment still stops the ReplicaSets", func(t *testing.T) {
		mc := finalizingCluster()
		set := ownedIdentitySet(mc, ownedIdentity(mc))
		lease := components.NewRealmClaimLease(testClaimNamespace, finalizerRealm, mc)
		r, deletes := fakeReconciler(t, mc, set, lease)

		require.NoError(t, r.finalize(context.Background(), mc))

		assert.Equal(t, []string{set.Name, lease.Name}, deletes.names)
		assert.False(t, exists(t, r, set))
	})

	// A workload that was replaced between the read and the delete fails the
	// precondition. The realm stays claimed until a pass has read what took
	// its place.
	t.Run("a replaced Deployment keeps the realm claim", func(t *testing.T) {
		mc := finalizingCluster()
		identity := ownedIdentity(mc)
		lease := components.NewRealmClaimLease(testClaimNamespace, finalizerRealm, mc)
		r, deletes := fakeReconciler(t, mc, identity, lease)
		deletes.conflictOn = identity.Name

		require.Error(t, r.finalize(context.Background(), mc))

		assert.True(t, exists(t, r, lease))
		assert.True(t, controllerutil.ContainsFinalizer(mc, Finalizer))
	})
}

// TestRegisteredCallbacksAuthorizesTheWithdrawal covers the two proofs that
// this management plane has login callbacks of its own to remove.
// status.callbackRealm is written before the components apply, so a plane
// whose contract write never landed still registered in the realm it recorded.
func TestRegisteredCallbacksAuthorizesTheWithdrawal(t *testing.T) {
	t.Run("a recorded realm authorizes it with no contract", func(t *testing.T) {
		mc := finalizingCluster()
		realm := finalizerRealm
		mc.Status.CallbackRealm = &realm
		r, _ := fakeReconciler(t, mc)

		assert.True(t, r.registeredCallbacks(context.Background(), mc))
	})

	// The keycloak mode records no realm, because the operator deletes that
	// Keycloak with the plane. The contract is what says it registered there.
	t.Run("a contract of this plane authorizes it with no record", func(t *testing.T) {
		mc := finalizingCluster()
		r, _ := fakeReconciler(t, mc, ownedContract(mc))

		assert.True(t, r.registeredCallbacks(context.Background(), mc))
	})

	// A plane with neither served no Optimize anywhere, and a deletion must
	// not remove a callback of another owner on a guess.
	t.Run("a plane with neither withdraws from nothing", func(t *testing.T) {
		mc := finalizingCluster()
		r, _ := fakeReconciler(t, mc)

		assert.False(t, r.registeredCallbacks(context.Background(), mc))
	})

	t.Run("a contract of another owner authorizes nothing", func(t *testing.T) {
		mc := finalizingCluster()
		contract := ownedContract(mc)
		contract.Labels[labels.ManagementClusterKey] = "elsewhere"
		r, _ := fakeReconciler(t, mc, contract)

		assert.False(t, r.registeredCallbacks(context.Background(), mc))
	})
}

// errUnread is what a read that the API server does not answer carries.
var errUnread = errors.New("the API server did not answer")

// errReplaced is what the conflict of a replaced object carries.
var errReplaced = errors.New("the object was replaced")

// finalizingCluster is a management cluster that carries the finalizer and no
// record of a contract.
func finalizingCluster() *v1.CamundaManagementCluster {
	return &v1.CamundaManagementCluster{ObjectMeta: metav1.ObjectMeta{
		Namespace:  "my-management-ns",
		Name:       "my-management",
		UID:        "management-uid",
		Finalizers: []string{Finalizer},
	}}
}

// ownedIdentity is the Management Identity Deployment of mc.
func ownedIdentity(mc *v1.CamundaManagementCluster) *appsv1.Deployment {
	controller := true

	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
		Namespace: mc.Namespace,
		Name:      components.IdentityName(mc),
		UID:       "identity-uid",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: v1.GroupVersion.String(),
			Kind:       "CamundaManagementCluster",
			Name:       mc.Name,
			UID:        mc.UID,
			Controller: &controller,
		}},
	}}
}

// ownedIdentitySet is a ReplicaSet of the Management Identity Deployment. It
// carries the discovery labels that the pod template of that Deployment has.
func ownedIdentitySet(
	mc *v1.CamundaManagementCluster,
	identity *appsv1.Deployment,
) *appsv1.ReplicaSet {
	controller := true

	return &appsv1.ReplicaSet{ObjectMeta: metav1.ObjectMeta{
		Namespace: mc.Namespace,
		Name:      identity.Name + "-7d9f8c",
		UID:       "identity-set-uid",
		Labels:    components.IdentityPodLabels(mc),
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: appsv1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
			Name:       identity.Name,
			UID:        identity.UID,
			Controller: &controller,
		}},
	}}
}

// pointIdentityAtRealm gives the Deployment a Management Identity container
// that names the realm of target, the way a rendered pod template does.
func pointIdentityAtRealm(identity *appsv1.Deployment, target v1.KeycloakRealmTarget) {
	identity.Spec.Template.Spec.Containers = []corev1.Container{{
		Name: "identity",
		Env: []corev1.EnvVar{
			{Name: "KEYCLOAK_URL", Value: target.URL},
			{Name: "KEYCLOAK_REALM", Value: target.Realm},
		},
	}}
}

// ownedContract is the ManagementAuthConfig that mc writes, with the owner
// labels that make it this plane's.
func ownedContract(mc *v1.CamundaManagementCluster) *v1.ManagementAuthConfig {
	return &v1.ManagementAuthConfig{ObjectMeta: metav1.ObjectMeta{
		Name: components.ContractName(mc),
		Labels: map[string]string{
			labels.ManagementClusterKey:          labels.OwnerName(mc.Name),
			labels.ManagementClusterNamespaceKey: mc.Namespace,
		},
	}}
}

// deleteLog records the name of every object that the reconciler deleted, in
// the order it deleted them.
type deleteLog struct {
	names []string
	// conflictOn answers the delete of the object of that name with a
	// conflict, the way the API server does when the object was replaced
	// between the read and the delete.
	conflictOn string
	// getFails answers every read of the object of that name with an error,
	// the way an API server that is not answering does.
	getFails string
}

// fakeReconciler builds a reconciler over a fake client that holds objects,
// and the log of what it deletes.
func fakeReconciler(t *testing.T, objects ...client.Object) (*Reconciler, *deleteLog) {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, appsv1.AddToScheme(scheme))
	require.NoError(t, corev1.AddToScheme(scheme))
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	deletes := &deleteLog{}
	c := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(objects...).
		WithInterceptorFuncs(interceptor.Funcs{
			Get: func(
				ctx context.Context,
				inner client.WithWatch,
				key client.ObjectKey,
				obj client.Object,
				opts ...client.GetOption,
			) error {
				if deletes.getFails != "" && key.Name == deletes.getFails {
					return apierrors.NewInternalError(errUnread)
				}

				return inner.Get(ctx, key, obj, opts...)
			},
			Delete: func(
				ctx context.Context,
				inner client.WithWatch,
				obj client.Object,
				opts ...client.DeleteOption,
			) error {
				deletes.names = append(deletes.names, obj.GetName())
				if obj.GetName() == deletes.conflictOn {
					return apierrors.NewConflict(
						schema.GroupResource{}, obj.GetName(), errReplaced,
					)
				}

				return inner.Delete(ctx, obj, opts...)
			},
		}).
		Build()

	return &Reconciler{
		Client:         c,
		APIReader:      c,
		Scheme:         scheme,
		EventRecorder:  events.NewFakeRecorder(16),
		ClaimNamespace: testClaimNamespace,
	}, deletes
}

// exists reports whether obj is still in the cluster of r.
func exists(t *testing.T, r *Reconciler, obj client.Object) bool {
	t.Helper()

	err := r.Get(context.Background(), client.ObjectKeyFromObject(obj), obj.DeepCopyObject().(client.Object))
	if apierrors.IsNotFound(err) {
		return false
	}
	require.NoError(t, err)

	return true
}
