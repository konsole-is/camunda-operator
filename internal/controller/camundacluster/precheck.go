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
	"strings"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/mirror"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

// eventReasonDumpCredentials is recorded when the backup credentials of the
// database do not resolve. Only dump Jobs consume them, so the cluster warns
// and does not park.
const eventReasonDumpCredentials = "DumpCredentialsUnresolved"

// eventReasonTrustStoreOptions is recorded when a process needs the JVM trust
// store but reads JAVA_TOOL_OPTIONS from a reference. The operator cannot add
// its options to such an entry, and the referenced value can already name a
// trust store, so the cluster warns and does not park.
const eventReasonTrustStoreOptions = "TrustStoreOptionsNotApplied"

// mirroredSecrets are the copies of the referenced Secrets that live outside
// the cluster namespace: the copied keys and their data, by purpose.
type mirroredSecrets map[components.MirrorPurpose]map[string][]byte

// resolver accumulates what the pre-checks read: the hash inputs of every
// referenced object and the data of every Secret to mirror. Each resolve
// method fills exactly one part of the render input.
type resolver struct {
	reader client.Reader
	// writer writes the claim on the storage contract. Every read stays on
	// reader.
	writer   client.Writer
	scheme   *runtime.Scheme
	cluster  *v1.CamundaCluster
	recorder events.EventRecorder
	inputs   []string
	mirrors  mirroredSecrets
	// storage is the SecondaryStorageConfig that spec.storageRef names, set
	// by resolveStorage for the steps after it.
	storage *v1.SecondaryStorageConfig
}

// preCheck resolves every reference of cluster into the render input, in
// the documented order: the preset, the release and the merged spec, the
// platform config and its Secrets, the storage binding and its chain, the
// claim on the binding, the object storage references. Every Secret is
// checked for its keys through the uncached reader. A Secret outside the
// cluster namespace is copied into the returned mirrors, and the input
// references the copy, so the renderer only ever names Secrets of the
// cluster namespace. HashInputs carry the
// resource version of every Secret and the generation of every CR read,
// sorted, so a change to any of them rolls the pods. A failed check returns a
// *conditions.PreCheckFailure: InvalidReference for a dangling reference or
// an invalid effective spec, MissingSecret for a missing Secret or key. Any
// other error is a transient API failure.
func (r *CamundaClusterReconciler) preCheck(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (components.Input, mirroredSecrets, error) {
	res := &resolver{
		reader:   r.APIReader,
		writer:   r.Client,
		scheme:   r.Scheme,
		cluster:  cluster,
		recorder: r.EventRecorder,
		mirrors:  mirroredSecrets{},
	}
	in := components.Input{Cluster: cluster}

	steps := []func(context.Context, *components.Input) error{
		res.resolveEffective,
		res.resolvePlatform,
		res.resolveAuth,
		res.resolveStorage,
		res.claimStorage,
		res.warnReferencedJavaToolOptions,
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

// resolveEffective reads the preset that spec.presetRef names and the
// release that spec.releaseRef names, merges them under the cluster spec,
// validates the result, and sets in.Effective and in.Images. An invalid
// merged spec maps to InvalidReference with the fields named.
func (res *resolver) resolveEffective(ctx context.Context, in *components.Input) error {
	var preset *v1.CamundaClusterPresetSpec
	if res.cluster.Spec.PresetRef != "" {
		var obj v1.CamundaClusterPreset
		key := client.ObjectKey{Name: res.cluster.Spec.PresetRef}
		if err := res.exists(ctx, key, &obj); err != nil {
			return err
		}

		// The fingerprint, not the generation: the generation moves for
		// every spec change, and this input is hashed into every process, so
		// a passwordRotation set on a preset would restart the brokers of
		// every cluster that inherits it. Only connectors follow the admin
		// password, through Input.AdminPasswordHash.
		fingerprint, err := components.PresetFingerprint(obj.Spec)
		if err != nil {
			return err
		}
		res.inputs = append(res.inputs, res.objectKind(&obj)+"/"+objectPath(key)+"="+fingerprint)

		preset = &obj.Spec
	}

	var release *v1.CamundaReleaseSpec
	if res.cluster.Spec.ReleaseRef != "" {
		var obj v1.CamundaRelease
		if err := res.get(ctx, client.ObjectKey{Name: res.cluster.Spec.ReleaseRef}, &obj); err != nil {
			return err
		}
		release = &obj.Spec
	}

	merged := components.MergeSpec(res.cluster.Spec, preset, release)
	if err := components.ValidateMerged(merged); err != nil {
		return &conditions.PreCheckFailure{
			Reason:  v1.ReasonInvalidReference,
			Message: "invalid effective spec: " + err.Error(),
		}
	}
	in.Effective = components.NewEffective(merged)
	in.Images = components.ReleaseImages(merged, release)

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

// resolveAuth checks the client secret of the effective authentication. It
// needs in.Effective and in.Platform. Under oidc the effective client secret
// is the cluster one when the cluster (or its preset) sets it, otherwise the
// platform one. A cluster reference names a Secret of the cluster namespace.
// The platform config is cluster-scoped, so its Secret gets a local copy.
func (res *resolver) resolveAuth(ctx context.Context, in *components.Input) error {
	if auth := in.Effective.Auth; auth != nil && auth.ClientSecretRef != nil {
		return res.checkLocalSecret(ctx, auth.ClientSecretRef.Name, auth.ClientSecretRef.Key)
	}
	if components.ResolveAuth(*in).Method != v1.AuthenticationMethodOIDC {
		return nil
	}
	if in.Platform.Auth != nil && in.Platform.Auth.OIDC != nil {
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
	res.storage = &binding

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
// elasticsearch block of binding, after checking the Secrets it names.
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

	credentials := es.CredentialsSecretRef
	if err := res.checkLocalSecret(
		ctx, credentials.Name, credentials.UsernameKey, credentials.PasswordKey,
	); err != nil {
		return err
	}
	if ca := es.CASecretRef; ca != nil {
		if err := res.checkLocalSecret(ctx, ca.Name, ca.Key); err != nil {
			return err
		}
	}
	in.Storage.Elasticsearch = es

	return nil
}

// resolveRDBMSStorage follows the rdbms block of binding through its
// DatabaseConfig (same namespace as the binding) to the DatabaseServerConfig
// and sets in.Storage.RDBMS, after checking the credentials Secret.
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
	serverKey := client.ObjectKey{Namespace: dbConfig.Namespace, Name: dbConfig.Spec.ServerRef}
	if err := res.get(ctx, serverKey, &server); err != nil {
		return err
	}

	creds := dbConfig.Spec.CredentialsSecretRef
	if err := res.checkLocalSecret(ctx, creds.Name, creds.UsernameKey, creds.PasswordKey); err != nil {
		return err
	}

	if err := res.checkDumpCredentials(ctx, dbConfig.Spec.BackupCredentialsSecretRef); err != nil {
		return err
	}
	in.Storage.RDBMS = &components.RDBMSStorage{
		Host:        server.Spec.Host,
		Port:        server.Spec.Port,
		Database:    dbConfig.Spec.DatabaseName,
		Credentials: creds,
	}

	return nil
}

// checkDumpCredentials reports the backup user of the database as a Warning
// event when it does not resolve. Only dump Jobs consume it, so the cluster
// neither parks nor rolls its pods on it, and the backup that needs it parks
// on its own pre-check. The Secret is no hash input. A nil reference means
// that the database takes no dumps.
func (res *resolver) checkDumpCredentials(
	ctx context.Context,
	ref *v1.LocalCredentialsSecretRef,
) error {
	if ref == nil {
		return nil
	}

	key := client.ObjectKey{Namespace: res.cluster.Namespace, Name: ref.Name}
	msg, err := secretref.CheckKeys(ctx, res.reader, key, ref.UsernameKey, ref.PasswordKey)
	if err != nil {
		return fmt.Errorf("reading Secret %q: %w", key, err)
	}
	if msg != "" {
		res.recorder.Eventf(
			res.cluster,
			nil,
			corev1.EventTypeWarning,
			eventReasonDumpCredentials,
			eventActionReconcile,
			"The dump credentials do not resolve, so backups of this cluster will park: %s",
			msg,
		)
	}

	return nil
}

// warnReferencedJavaToolOptions warns about every process that needs the JVM
// trust store but reads JAVA_TOOL_OPTIONS from a reference. It needs
// in.Effective and in.Storage, and it never fails: the referenced value can
// already name a trust store that holds the certificate authority, so the
// combination is legal and only the operator options are lost.
func (res *resolver) warnReferencedJavaToolOptions(_ context.Context, in *components.Input) error {
	referenced := components.ReferencedJavaToolOptions(*in)
	if len(referenced) == 0 {
		return nil
	}

	res.recorder.Eventf(
		res.cluster,
		nil,
		corev1.EventTypeWarning,
		eventReasonTrustStoreOptions,
		eventActionReconcile,
		"Processes %s read JAVA_TOOL_OPTIONS from a reference, "+
			"so the operator cannot add the trust store options. "+
			"The operator builds the trust store at %s. "+
			"If the referenced value does not name it, the Elasticsearch export fails",
		strings.Join(referenced, ", "),
		components.TrustStorePath,
	)

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
func (res *resolver) localize(ctx context.Context, ref *v1.SecretKeyRef, purpose components.MirrorPurpose) error {
	local, err := res.secret(ctx, client.ObjectKey{Namespace: ref.Namespace, Name: ref.Name}, purpose, ref.Key)
	if err != nil {
		return err
	}
	ref.Name, ref.Namespace = local.Name, local.Namespace

	return nil
}

// checkLocalSecret checks that the Secret named name in the cluster namespace
// carries keys, and records its resource version as a render input. Every
// reference of a namespaced kind resolves here, so no copy is involved.
func (res *resolver) checkLocalSecret(ctx context.Context, name string, keys ...string) error {
	_, err := res.secret(ctx, client.ObjectKey{Namespace: res.cluster.Namespace, Name: name}, "", keys...)

	return err
}

// secret checks that the Secret at key carries every one of keys and records
// its resource version as a hash input. When purpose is set and the Secret
// lives outside the cluster namespace, it copies the keys into the mirror of
// that purpose and returns the key of the copy in the cluster namespace, the
// key that pkg/mirror resolves for a reader; otherwise it returns key
// unchanged. A missing Secret or key maps to MissingSecret.
func (res *resolver) secret(
	ctx context.Context,
	key client.ObjectKey,
	purpose components.MirrorPurpose,
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

	if purpose == "" || !mirror.Needed(res.cluster, key.Namespace) {
		return key, nil
	}

	data := make(map[string][]byte, len(keys))
	for _, k := range keys {
		data[k] = secret.Data[k]
	}
	res.mirrors[purpose] = data

	return client.ObjectKey{
		Namespace: res.cluster.Namespace,
		Name:      mirror.LocalSecretName(res.cluster, key.Namespace, key.Name, purpose),
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
