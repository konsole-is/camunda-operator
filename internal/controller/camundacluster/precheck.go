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
	"fmt"
	"slices"
	"strconv"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

// mirroredSecrets are the copies of the referenced Secrets that live outside
// the cluster namespace: the copied keys and their data, by purpose.
type mirroredSecrets map[string]map[string][]byte

// resolver accumulates what the pre-checks read: the hash inputs of every
// referenced object and the data of every Secret to mirror. Each resolve
// method fills exactly one part of the render input.
type resolver struct {
	reader  client.Reader
	scheme  *runtime.Scheme
	cluster *v1.CamundaCluster
	inputs  []string
	mirrors mirroredSecrets
}

// preCheck resolves every reference of cluster into the render input, in the
// documented order: the preset and the merged spec, the platform config and
// its Secrets, the storage binding and its chain, the object storage
// references. Every Secret is checked for its keys through the uncached
// reader. A Secret outside the cluster namespace is copied into the returned
// mirrors, and the input references the copy, so the renderer only ever
// names Secrets of the cluster namespace. HashInputs carry the resource
// version of every Secret and the generation of every CR read, sorted, so a
// change to any of them rolls the pods. A failed check returns a
// *conditions.PreCheckFailure: InvalidReference for a dangling reference or
// an invalid effective spec, MissingSecret for a missing Secret or key. Any
// other error is a transient API failure.
func (r *CamundaClusterReconciler) preCheck(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (components.Input, mirroredSecrets, error) {
	res := &resolver{reader: r.APIReader, scheme: r.Scheme, cluster: cluster, mirrors: mirroredSecrets{}}
	in := components.Input{Cluster: cluster}

	steps := []func(context.Context, *components.Input) error{
		res.resolveEffective,
		res.resolvePlatform,
		res.resolveAuth,
		res.resolveStorage,
		res.resolveObjectStorage,
	}
	for _, step := range steps {
		if err := step(ctx, &in); err != nil {
			return in, nil, err
		}
	}

	slices.Sort(res.inputs)
	in.HashInputs = res.inputs

	return in, res.mirrors, nil
}

// resolveEffective reads the preset that spec.presetRef names, merges it
// under the cluster spec, validates the result, and sets in.Effective. An
// invalid merged spec maps to InvalidReference with the fields named.
func (res *resolver) resolveEffective(ctx context.Context, in *components.Input) error {
	var preset *v1.CamundaClusterPresetSpec
	if res.cluster.Spec.PresetRef != "" {
		var obj v1.CamundaClusterPreset
		if err := res.get(ctx, client.ObjectKey{Name: res.cluster.Spec.PresetRef}, &obj); err != nil {
			return err
		}
		preset = &obj.Spec
	}

	merged := components.MergePreset(res.cluster.Spec, preset)
	if err := components.ValidateMerged(merged); err != nil {
		return &conditions.PreCheckFailure{
			Reason:  v1.ReasonInvalidReference,
			Message: "invalid effective spec: " + err.Error(),
		}
	}
	in.Effective = components.NewEffective(merged)

	return nil
}

// resolvePlatform reads the CamundaPlatformConfig that spec.platformConfigRef
// names into in.Platform and points the license reference at its local copy.
func (res *resolver) resolvePlatform(ctx context.Context, in *components.Input) error {
	var cfg v1.CamundaPlatformConfig
	if err := res.get(ctx, client.ObjectKey{Name: res.cluster.Spec.PlatformConfigRef}, &cfg); err != nil {
		return err
	}
	in.Platform = *cfg.Spec.DeepCopy()

	if in.Platform.LicenseSecretRef != nil {
		return res.localize(ctx, in.Platform.LicenseSecretRef, components.MirrorPurposeLicense)
	}

	return nil
}

// resolveAuth checks the client secret of the effective authentication and
// points the reference at its local copy. It needs in.Effective and
// in.Platform. Under oidc the effective client secret is the cluster one when
// the cluster (or its preset) sets it, otherwise the platform one. Under basic
// a cluster client secret reference is checked but not mirrored, because
// nothing renders it.
func (res *resolver) resolveAuth(ctx context.Context, in *components.Input) error {
	clusterRef := in.Effective.Auth != nil && in.Effective.Auth.ClientSecretRef != nil

	switch {
	case components.ResolveAuth(*in).Method != v1.AuthenticationMethodOIDC:
		if clusterRef {
			ref := in.Effective.Auth.ClientSecretRef
			_, err := res.secret(ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, "", ref.Key)
			return err
		}
	case clusterRef:
		return res.localize(ctx, in.Effective.Auth.ClientSecretRef, components.MirrorPurposeAuthClient)
	case in.Platform.Auth != nil && in.Platform.Auth.OIDC != nil:
		return res.localize(ctx, &in.Platform.Auth.OIDC.ClientSecretRef, components.MirrorPurposeOIDCClient)
	}

	return nil
}

// resolveStorage reads the SecondaryStorageConfig of the cluster namespace
// that spec.storageRef names and resolves its backend into in.Storage.
func (res *resolver) resolveStorage(ctx context.Context, in *components.Input) error {
	var binding v1.SecondaryStorageConfig
	key := client.ObjectKey{Namespace: res.cluster.Namespace, Name: res.cluster.Spec.StorageRef}
	if err := res.get(ctx, key, &binding); err != nil {
		return err
	}
	in.Storage.Type = binding.Spec.Type

	switch binding.Spec.Type {
	case v1.SecondaryStorageTypeElasticsearch:
		return res.resolveElasticsearchStorage(ctx, in, &binding)
	case v1.SecondaryStorageTypeRDBMS:
		return res.resolveRDBMSStorage(ctx, in, &binding)
	default:
		return fmt.Errorf("SecondaryStorageConfig %q has unsupported type %q", key, binding.Spec.Type)
	}
}

// resolveElasticsearchStorage sets in.Storage.Elasticsearch from the
// elasticsearch block of binding, with the credentials and CA references
// pointed at their local copies.
func (res *resolver) resolveElasticsearchStorage(
	ctx context.Context,
	in *components.Input,
	binding *v1.SecondaryStorageConfig,
) error {
	if binding.Spec.Elasticsearch == nil {
		return fmt.Errorf(
			"SecondaryStorageConfig %q has type elasticsearch and no elasticsearch block",
			client.ObjectKeyFromObject(binding),
		)
	}
	es := binding.Spec.Elasticsearch.DeepCopy()

	if err := res.localizeCredentials(
		ctx,
		&es.CredentialsSecretRef,
		components.MirrorPurposeESCredentials,
	); err != nil {
		return err
	}
	if es.CASecretRef != nil {
		if err := res.localize(ctx, es.CASecretRef, components.MirrorPurposeESCA); err != nil {
			return err
		}
	}
	in.Storage.Elasticsearch = es

	return nil
}

// resolveRDBMSStorage follows the rdbms block of binding through its
// DatabaseConfig (same namespace as the binding) to the DatabaseServerConfig
// and sets in.Storage.RDBMS, with the DatabaseConfig credentials pointed at
// their local copy.
func (res *resolver) resolveRDBMSStorage(
	ctx context.Context,
	in *components.Input,
	binding *v1.SecondaryStorageConfig,
) error {
	if binding.Spec.RDBMS == nil {
		return fmt.Errorf(
			"SecondaryStorageConfig %q has type rdbms and no rdbms block",
			client.ObjectKeyFromObject(binding),
		)
	}

	var dbConfig v1.DatabaseConfig
	dbKey := client.ObjectKey{Namespace: binding.Namespace, Name: binding.Spec.RDBMS.DatabaseConfigRef}
	if err := res.get(ctx, dbKey, &dbConfig); err != nil {
		return err
	}

	var server v1.DatabaseServerConfig
	if err := res.get(ctx, client.ObjectKey{Name: dbConfig.Spec.ServerRef}, &server); err != nil {
		return err
	}

	creds := dbConfig.Spec.CredentialsSecretRef
	if err := res.localizeCredentials(ctx, &creds, components.MirrorPurposeDBCredentials); err != nil {
		return err
	}

	// The dump Job of a LogicalBackupRDBMS mounts the backup user of the
	// database, so its credentials get a local copy the same way: the
	// backup controller consumes the copy and never writes one itself.
	if dbConfig.Spec.BackupCredentialsSecretRef != nil {
		dump := *dbConfig.Spec.BackupCredentialsSecretRef
		if err := res.localizeCredentials(ctx, &dump, components.MirrorPurposeDumpCredentials); err != nil {
			return err
		}
	}
	in.Storage.RDBMS = &components.RDBMSStorage{
		Host:        server.Spec.Host,
		Port:        server.Spec.Port,
		Database:    dbConfig.Spec.DatabaseName,
		Credentials: creds,
	}

	return nil
}

// get reads the referenced object without the cache and records its
// generation as a hash input. A missing object maps to InvalidReference,
// naming the kind and the reference.
func (res *resolver) get(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	if err := res.exists(ctx, key, obj); err != nil {
		return err
	}

	res.inputs = append(
		res.inputs,
		res.objectKind(obj)+"/"+objectPath(key)+"="+strconv.FormatInt(obj.GetGeneration(), 10),
	)

	return nil
}

// exists reads the referenced object without the cache. A missing object maps
// to InvalidReference, naming the kind and the reference (with its namespace
// for a namespaced kind). Any other error is a transient API failure.
func (res *resolver) exists(ctx context.Context, key client.ObjectKey, obj client.Object) error {
	if err := res.reader.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return &conditions.PreCheckFailure{
				Reason:  v1.ReasonInvalidReference,
				Message: fmt.Sprintf("%s %q not found", res.objectKind(obj), objectPath(key)),
			}
		}
		return fmt.Errorf("reading %s %q: %w", res.objectKind(obj), key, err)
	}

	return nil
}

// localize checks the Secret of ref through secret and rewrites ref to its
// local key.
func (res *resolver) localize(ctx context.Context, ref *v1.SecretKeyRef, purpose string) error {
	local, err := res.secret(ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, purpose, ref.Key)
	if err != nil {
		return err
	}
	ref.Name, ref.Namespace = local.Name, local.Namespace

	return nil
}

// localizeCredentials is localize for a username/password reference.
func (res *resolver) localizeCredentials(
	ctx context.Context,
	ref *v1.CredentialsSecretRef,
	purpose string,
) error {
	local, err := res.secret(
		ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, purpose,
		ref.UsernameKey, ref.PasswordKey,
	)
	if err != nil {
		return err
	}
	ref.Name, ref.Namespace = local.Name, local.Namespace

	return nil
}

// secret checks that the Secret at key carries every one of keys and records
// its resource version as a hash input. When purpose is set and the Secret
// lives outside the cluster namespace, it copies the keys into the mirror of
// that purpose and returns the key of the copy in the cluster namespace;
// otherwise it returns key unchanged. A missing Secret or key maps to
// MissingSecret.
func (res *resolver) secret(
	ctx context.Context,
	key client.ObjectKey,
	purpose string,
	keys ...string,
) (client.ObjectKey, error) {
	secret, msg, err := secretref.Get(ctx, res.reader, key, keys...)
	if err != nil {
		return client.ObjectKey{}, fmt.Errorf("reading Secret %q: %w", key, err)
	}
	if msg != "" {
		return client.ObjectKey{}, &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: msg}
	}
	res.inputs = append(res.inputs, "Secret/"+objectPath(key)+"="+secret.ResourceVersion)

	if purpose == "" || key.Namespace == res.cluster.Namespace {
		return key, nil
	}

	data := make(map[string][]byte, len(keys))
	for _, k := range keys {
		data[k] = secret.Data[k]
	}
	res.mirrors[purpose] = data

	return client.ObjectKey{
		Namespace: res.cluster.Namespace,
		Name:      components.MirroredSecretName(res.cluster, purpose),
	}, nil
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
