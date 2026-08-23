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
	"strconv"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

// resolved is what the pre-checks produce: the render input and the name of
// the contract that this management cluster writes.
type resolved struct {
	// Input renders the management plane.
	Input components.Input
	// ContractName is the cluster-scoped ManagementAuthConfig that the
	// reconcile applies.
	ContractName string
}

// resolver accumulates what the pre-checks read: the data of every Secret to
// copy into the management namespace, and the hash inputs that roll the pods
// when a referenced object changes behind an unchanged reference. An object
// that one component alone reads goes into componentInputs under the name of
// that component.
type resolver struct {
	reader  client.Reader
	scheme  *runtime.Scheme
	mc      *v1.CamundaManagementCluster
	mirrors map[components.MirrorPurpose]map[string][]byte
	inputs  []string
	// componentInputs are the hash inputs of single components, by component
	// name. forComponent records a block of reads here instead of in inputs,
	// so what one component alone reads rolls that component alone.
	componentInputs map[string][]string
}

// preCheck resolves every reference of mc, in the documented order: the rules
// that the API server cannot check, the platform config and its license, the
// identity provider and its client secrets, the database of Management
// Identity, and what Web Modeler needs. A Secret outside the management
// namespace is copied into the returned mirrors, and the input references the
// copy, so the renderer only ever names Secrets of that namespace.
//
// A failed check returns a *conditions.PreCheckFailure: UnsupportedVersion for
// a version below its floor, InvalidReference for a dangling reference or a
// platform config that cannot serve the mode, MissingSecret for a missing
// Secret or key, and Conflict when another owner holds the contract name. Any
// other error is a transient API failure.
func (r *Reconciler) preCheck(ctx context.Context, mc *v1.CamundaManagementCluster) (resolved, error) {
	out := resolved{
		Input:        components.Input{Cluster: mc, Suspended: mc.Spec.Suspend, KeycloakCRDServed: r.keycloakServed},
		ContractName: components.ContractName(mc),
	}
	if failure := components.ValidateSpec(mc); failure != nil {
		return out, failure
	}

	res := &resolver{
		reader:          r.APIReader,
		scheme:          r.Scheme,
		mc:              mc,
		mirrors:         map[components.MirrorPurpose]map[string][]byte{},
		componentInputs: map[string][]string{},
	}

	if failure := r.checkKeycloakOperator(mc); failure != nil {
		return out, failure
	}
	if err := res.resolvePlatform(ctx, &out); err != nil {
		return out, err
	}
	if err := res.resolveKeycloakAdmin(ctx); err != nil {
		return out, err
	}
	if err := res.resolveGeneratedSecrets(ctx, &out); err != nil {
		return out, err
	}
	if err := res.resolveProvider(ctx, &out); err != nil {
		return out, err
	}
	if err := res.resolveDatabases(ctx, &out); err != nil {
		return out, err
	}
	if err := res.resolveWebModeler(ctx, &out); err != nil {
		return out, err
	}
	if err := r.checkContractOwner(ctx, mc, out.ContractName); err != nil {
		return out, err
	}

	out.Input.Mirrors = res.mirrors
	out.Input.HashInputs = res.inputs
	out.Input.ComponentHashInputs = res.componentInputs

	return out, nil
}

// checkKeycloakOperator refuses the keycloak mode on a Kubernetes cluster
// that does not serve the Keycloak kind. Nothing would create the Keycloak,
// and every other reference would resolve, so the management cluster would
// wait for a Keycloak that never arrives.
func (r *Reconciler) checkKeycloakOperator(mc *v1.CamundaManagementCluster) *conditions.PreCheckFailure {
	if components.Mode(mc) != components.ModeKeycloak || r.keycloakServed {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonKeycloakOperatorNotInstalled,
		Message: "spec.identityProvider selects keycloak and this Kubernetes cluster does not serve " +
			"k8s.keycloak.org/v2alpha1 Keycloak; install the Keycloak Operator, or select the " +
			"externalKeycloak or the oidc mode",
	}
}

// resolvePlatform reads the CamundaPlatformConfig that spec.platformConfigRef
// names and points the license reference at its local copy.
func (res *resolver) resolvePlatform(ctx context.Context, out *resolved) error {
	var cfg v1.CamundaPlatformConfig
	if err := res.get(ctx, client.ObjectKey{Name: res.mc.Spec.PlatformConfigRef}, &cfg); err != nil {
		return err
	}
	out.Input.Platform = cfg.Spec.DeepCopy()

	if ref := out.Input.Platform.LicenseSecretRef; ref != nil {
		return res.localize(ctx, ref, components.MirrorPurposeLicense)
	}

	return nil
}

// resolveKeycloakAdmin reads the Keycloak administrator that Management
// Identity bootstraps the realm with.
//
// The externalKeycloak mode names the Secret, so a missing one is a
// MissingSecret the user must correct, and the copy in the management
// namespace is what the Identity pods mount. The keycloak mode reads the
// Secret that the Keycloak Operator writes next to the Keycloak; that one is
// absent until the Keycloak Operator has acted, and refusing the reconcile
// over it would stop the very apply that creates the Keycloak, so it only
// contributes a hash input while it exists.
//
// Management Identity is the one component that signs in with the
// administrator, in either mode, so the input is its own.
func (res *resolver) resolveKeycloakAdmin(ctx context.Context) error {
	switch components.Mode(res.mc) {
	case components.ModeExternalKeycloak:
		// The rewritten reference is dropped: the render package derives the
		// same local name from LocalSecretName, so this call is here for the
		// check, the copy, and the hash input.
		ref := res.mc.Spec.IdentityProvider.ExternalKeycloak.AdminCredentialsSecretRef.DeepCopy()

		return res.forComponent(components.ComponentIdentity, func() error {
			return res.localizeCredentials(ctx, ref, components.MirrorPurposeKeycloakAdmin)
		})
	case components.ModeKeycloak:
		key := client.ObjectKey{
			Namespace: res.mc.Namespace,
			Name:      components.KeycloakInitialAdminSecretName(res.mc),
		}
		var secret corev1.Secret
		if err := res.reader.Get(ctx, key, &secret); err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			return fmt.Errorf("reading Secret %q: %w", key, err)
		}
		res.componentInputs[components.ComponentIdentity] = append(
			res.componentInputs[components.ComponentIdentity],
			"Secret/"+objectPath(key)+"="+secret.ResourceVersion,
		)

		return nil
	default:
		return nil
	}
}

// resolveGeneratedSecrets reads the credentials that the operator publishes
// itself: the Optimize client secret and the password of the first
// administrator. Only the two Keycloak modes need them, because Management
// Identity creates the clients and the user there. Identity makes its own
// client secret, so the operator publishes none for it.
//
// A credential that a Secret already holds is read back, so it stays stable
// after creation. Deleting the Secret is what rotates it.
//
// An administrator password of your own replaces the generated one. It is
// checked here, and copied into the management namespace when it lives
// outside it, because the Identity pods mount it. Management Identity is the
// one component that reads it, so the input is its own.
func (res *resolver) resolveGeneratedSecrets(ctx context.Context, out *resolved) error {
	if components.Mode(res.mc) == components.ModeOIDC {
		return nil
	}

	secrets := components.GeneratedSecrets{
		OptimizeClient: components.OptimizeClientSecretName(res.mc),
		Values:         map[string]credentials.Password{},
	}
	generated := map[string]string{
		secrets.OptimizeClient: components.ClientSecretKey,
	}
	if res.mc.Spec.Identity.Admin.PasswordSecretRef == nil {
		secrets.IdentityAdmin = components.IdentityAdminSecretName(res.mc)
		generated[secrets.IdentityAdmin] = components.PasswordKey
	}

	for name, field := range generated {
		key := client.ObjectKey{Namespace: res.mc.Namespace, Name: name}
		password, err := credentials.LookupOrNew(ctx, res.reader, key, field)
		if err != nil {
			return err
		}
		secrets.Values[name] = password
	}
	out.Input.Secrets = secrets

	if ref := res.mc.Spec.Identity.Admin.PasswordSecretRef; ref != nil {
		// The rewritten reference is dropped: the render package derives the
		// same local name from LocalSecretName, so this call is here for the
		// check, the copy, and the hash input.
		local := ref.DeepCopy()

		return res.forComponent(components.ComponentIdentity, func() error {
			return res.localize(ctx, local, components.MirrorPurposeIdentityAdmin)
		})
	}

	return nil
}

// resolveProvider builds the identity provider and resolves the client
// secrets it names. The Management Identity secret is pointed at its local
// copy, because the Identity pods mount it. The Optimize secret keeps the
// namespace it was declared in: the contract is cluster-scoped, and the
// CamundaOptimize that reads it makes a copy of its own.
//
// Only the oidc mode has secrets to resolve. In the two Keycloak modes
// Management Identity makes the client secret of every client it creates,
// and the Optimize one names a Secret that this operator generates in the
// management namespace. The Secrets component is what creates that Secret,
// so requiring it here would refuse the very reconcile that writes it.
func (res *resolver) resolveProvider(ctx context.Context, out *resolved) error {
	provider, err := components.ResolveIdentityProvider(out.Input)
	if err != nil {
		return err
	}
	out.Input.Provider = provider

	if components.Mode(res.mc) != components.ModeOIDC {
		return nil
	}

	if ref := out.Input.Provider.Clients.Identity.SecretRef; ref != nil {
		if err := res.localize(ctx, ref, components.MirrorPurposeIdentityClient); err != nil {
			return err
		}
	}
	if ref := out.Input.Provider.Clients.Optimize.SecretRef; ref != nil {
		key := client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}
		if _, err := res.secret(ctx, key, ref.Key); err != nil {
			return err
		}
	}

	return nil
}

// resolveDatabases reads the DatabaseConfig of Management Identity and of the
// Keycloak that the operator runs, and the server behind each. Management
// Identity is always deployed, so its database is never optional. Web Modeler
// reads its own database under its own component.
func (res *resolver) resolveDatabases(ctx context.Context, out *resolved) error {
	identity, err := res.resolveDatabase(
		ctx, res.mc.Spec.Identity.DatabaseConfigRef, components.MirrorPurposeIdentityDB,
	)
	if err != nil {
		return err
	}
	out.Input.Databases.Identity = identity

	if keycloak := res.mc.Spec.IdentityProvider.Keycloak; keycloak != nil {
		db, err := res.resolveDatabase(
			ctx, keycloak.DatabaseConfigRef, components.MirrorPurposeKeycloakDB,
		)
		if err != nil {
			return err
		}
		out.Input.Databases.Keycloak = &db
	}

	return nil
}

// resolveWebModeler reads what Web Modeler needs: the database of its own, the
// SMTP credentials, both pointed at their local copy, and the credentials that
// pair its two processes. It resolves nothing while the spec deploys no Web
// Modeler.
//
// The database and the SMTP server are read under the restapi component, so
// rotating one of their credentials rolls Web Modeler and leaves Management
// Identity where it is.
func (res *resolver) resolveWebModeler(ctx context.Context, out *resolved) error {
	webModeler := res.mc.Spec.WebModeler
	if webModeler == nil {
		return nil
	}

	err := res.forComponent(components.ComponentWebModelerRestapi, func() error {
		database, err := res.resolveDatabase(
			ctx, webModeler.DatabaseConfigRef, components.MirrorPurposeWebModelerDB,
		)
		if err != nil {
			return err
		}
		out.Input.Databases.WebModeler = &database

		ref := webModeler.Mail.CredentialsSecretRef
		if ref == nil {
			return nil
		}

		local := ref.DeepCopy()
		if err := res.localizeCredentials(ctx, local, components.MirrorPurposeWebModelerMail); err != nil {
			return err
		}
		out.Input.WebModelerMail = local

		return nil
	})
	if err != nil {
		return err
	}

	pusher, err := res.resolvePusher(ctx)
	if err != nil {
		return err
	}
	out.Input.Pusher = pusher

	return nil
}

// forComponent runs read and records every hash input it produces under comp
// rather than in the shared inputs. A failed read keeps what it recorded up to
// the failure, the way the shared inputs do.
func (res *resolver) forComponent(comp string, read func() error) error {
	shared := res.inputs
	res.inputs = nil

	err := read()

	res.componentInputs[comp] = append(res.componentInputs[comp], res.inputs...)
	res.inputs = shared

	return err
}

// resolvePusher reads the credentials that the two Web Modeler processes
// authenticate their WebSocket connection with, and generates them when the
// Secret that holds them is absent. Deleting that Secret therefore rotates
// them.
func (res *resolver) resolvePusher(ctx context.Context) (components.PusherCredentials, error) {
	key := client.ObjectKey{
		Namespace: res.mc.Namespace,
		Name:      components.PusherSecretName(res.mc),
	}

	appKey, err := credentials.LookupOrNew(ctx, res.reader, key, components.PusherAppKeyKey)
	if err != nil {
		return components.PusherCredentials{}, err
	}
	appSecret, err := credentials.LookupOrNew(ctx, res.reader, key, components.PusherAppSecretKey)
	if err != nil {
		return components.PusherCredentials{}, err
	}

	return components.PusherCredentials{Key: appKey, Secret: appSecret}, nil
}

// resolveDatabase reads one DatabaseConfig of the management namespace and the
// DatabaseServerConfig it names, and points its credentials at their local
// copy.
func (res *resolver) resolveDatabase(
	ctx context.Context,
	ref string,
	purpose components.MirrorPurpose,
) (components.Database, error) {
	var cfg v1.DatabaseConfig
	if err := res.get(ctx, client.ObjectKey{Namespace: res.mc.Namespace, Name: ref}, &cfg); err != nil {
		return components.Database{}, err
	}

	var server v1.DatabaseServerConfig
	if err := res.get(ctx, client.ObjectKey{Name: cfg.Spec.ServerRef}, &server); err != nil {
		return components.Database{}, err
	}

	secretRef := *cfg.Spec.CredentialsSecretRef.DeepCopy()
	if err := res.localizeCredentials(ctx, &secretRef, purpose); err != nil {
		return components.Database{}, err
	}

	return components.Database{
		Host:        server.Spec.Host,
		Port:        server.Spec.Port,
		Name:        cfg.Spec.DatabaseName,
		Credentials: secretRef,
	}, nil
}

// get reads the referenced object and records its generation as a hash input,
// so a change to that object rolls the pods. A missing object maps to
// InvalidReference, naming the kind and the reference. Any other error is a
// transient API failure.
func (res *resolver) get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	if err := res.reader.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return &conditions.PreCheckFailure{
				Reason:  v1.ReasonInvalidReference,
				Message: fmt.Sprintf("%s %q not found", res.objectKind(obj), objectPath(key)),
			}
		}
		return fmt.Errorf("reading %s %q: %w", res.objectKind(obj), key, err)
	}

	res.inputs = append(
		res.inputs,
		res.objectKind(obj)+"/"+objectPath(key)+"="+strconv.FormatInt(obj.GetGeneration(), 10),
	)

	return nil
}

// localize checks the Secret of ref through secret and rewrites ref to its
// local key.
func (res *resolver) localize(ctx context.Context, ref *v1.SecretKeyRef, purpose components.MirrorPurpose) error {
	local, err := res.secretFor(
		ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, purpose, ref.Key,
	)
	if err != nil {
		return err
	}
	ref.Name, ref.Namespace = local.Name, local.Namespace

	return nil
}

// localizeCredentials is localize for a username and password reference.
func (res *resolver) localizeCredentials(
	ctx context.Context,
	ref *v1.CredentialsSecretRef,
	purpose components.MirrorPurpose,
) error {
	local, err := res.secretFor(
		ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, purpose,
		ref.UsernameKey, ref.PasswordKey,
	)
	if err != nil {
		return err
	}
	ref.Name, ref.Namespace = local.Name, local.Namespace

	return nil
}

// secretFor is secret, plus the copy that a Secret outside the management
// namespace needs. It returns the key of the Secret that a pod of the
// management plane mounts.
func (res *resolver) secretFor(
	ctx context.Context,
	key client.ObjectKey,
	purpose components.MirrorPurpose,
	keys ...string,
) (client.ObjectKey, error) {
	found, err := res.secret(ctx, key, keys...)
	if err != nil {
		return client.ObjectKey{}, err
	}
	if key.Namespace != res.mc.Namespace {
		data := make(map[string][]byte, len(keys))
		for _, k := range keys {
			data[k] = found[k]
		}
		res.mirrors[purpose] = data
	}

	return client.ObjectKey{
		Namespace: res.mc.Namespace,
		Name:      components.LocalSecretName(res.mc, key.Namespace, key.Name, purpose),
	}, nil
}

// secret checks that the Secret at key carries every one of keys and returns
// its data. A missing Secret or key maps to MissingSecret. The resource
// version of the Secret goes into the hash inputs, so a rotated credential
// rolls the pods that read it.
func (res *resolver) secret(
	ctx context.Context,
	key client.ObjectKey,
	keys ...string,
) (map[string][]byte, error) {
	found, msg, err := secretref.Get(ctx, res.reader, key, keys...)
	if err != nil {
		return nil, fmt.Errorf("reading Secret %q: %w", key, err)
	}
	if msg != "" {
		return nil, &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: msg}
	}
	res.inputs = append(res.inputs, "Secret/"+objectPath(key)+"="+found.ResourceVersion)

	return found.Data, nil
}

// objectPath returns "<namespace>/<name>" for a namespaced key and "<name>"
// for a cluster-scoped one.
func objectPath(key client.ObjectKey) string {
	if key.Namespace == "" {
		return key.Name
	}

	return key.Namespace + "/" + key.Name
}

// objectKind returns the kind of obj from the scheme. A typed read leaves
// TypeMeta empty, so the object cannot tell.
func (res *resolver) objectKind(obj client.Object) string {
	gvk, err := apiutil.GVKForObject(obj, res.scheme)
	if err != nil {
		return fmt.Sprintf("%T", obj)
	}

	return gvk.Kind
}
