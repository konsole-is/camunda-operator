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
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
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
// when a referenced object changes behind an unchanged reference.
type resolver struct {
	reader  client.Reader
	scheme  *runtime.Scheme
	mc      *v1.CamundaManagementCluster
	mirrors map[components.MirrorPurpose]map[string][]byte
	inputs  []string
}

// preCheck resolves every reference of mc, in the documented order: the rules
// that the API server cannot check, the platform config and its license, the
// identity provider and its client secrets, and the database of Management
// Identity. A Secret outside the management namespace is copied into the
// returned mirrors, and the input references the copy, so the renderer only
// ever names Secrets of that namespace.
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
		reader:  r.APIReader,
		scheme:  r.Scheme,
		mc:      mc,
		mirrors: map[components.MirrorPurpose]map[string][]byte{},
	}

	if err := res.resolvePlatform(ctx, &out); err != nil {
		return out, err
	}
	if err := res.resolveProvider(ctx, &out); err != nil {
		return out, err
	}
	if err := res.resolveDatabases(ctx, &out); err != nil {
		return out, err
	}
	if err := r.checkContractOwner(ctx, mc, out.ContractName); err != nil {
		return out, err
	}

	out.Input.Mirrors = res.mirrors
	slices.Sort(res.inputs)
	out.Input.HashInputs = res.inputs

	return out, nil
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

// resolveProvider builds the identity provider and resolves the client
// secrets it names. The Management Identity secret is pointed at its local
// copy, because the Identity pods mount it. The Optimize secret keeps the
// namespace it was declared in: the contract is cluster-scoped, and the
// CamundaOptimize that reads it makes a copy of its own.
func (res *resolver) resolveProvider(ctx context.Context, out *resolved) error {
	provider, err := components.ResolveIdentityProvider(out.Input)
	if err != nil {
		return err
	}
	out.Input.Provider = provider

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

// resolveDatabases reads the DatabaseConfig of every component that needs one
// and the server behind it. The oidc mode deploys Management Identity alone,
// so it needs one database.
func (res *resolver) resolveDatabases(ctx context.Context, out *resolved) error {
	identity, err := res.resolveDatabase(
		ctx, res.mc.Spec.Identity.DatabaseConfigRef, components.MirrorPurposeIdentityDB,
	)
	if err != nil {
		return err
	}
	out.Input.Databases.Identity = identity

	return nil
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

	credentials := *cfg.Spec.CredentialsSecretRef.DeepCopy()
	if err := res.localizeCredentials(ctx, &credentials, purpose); err != nil {
		return components.Database{}, err
	}

	return components.Database{
		Host:        server.Spec.Host,
		Port:        server.Spec.Port,
		Name:        cfg.Spec.DatabaseName,
		Credentials: credentials,
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
