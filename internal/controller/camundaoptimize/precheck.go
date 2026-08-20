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

package camundaoptimize

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	clustercomponents "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

// mirroredSecrets are the copies of the referenced Secrets that live outside
// the CamundaOptimize namespace: the copied keys and their data, by purpose.
type mirroredSecrets map[string]map[string][]byte

// resolved is what the pre-checks produce: the render input, the Secrets to
// copy into the CamundaOptimize namespace, and what the exporter patch needs.
type resolved struct {
	// Input renders the Optimize workloads.
	Input components.Input
	// Mirrors are the copies of the referenced Secrets that live elsewhere.
	Mirrors mirroredSecrets
	// ClusterKey is the CamundaCluster that the exporter patch targets.
	ClusterKey client.ObjectKey
	// ExporterStorage is the storage contract with the credentials reference
	// as the cluster resolves it. The exporter runs in the broker container,
	// so it reads the copy that the cluster's own controller makes, not the
	// copy that this controller makes for the Optimize pods.
	ExporterStorage v1.ElasticsearchStorage
}

// resolver accumulates what the pre-checks read: the data of every Secret to
// copy into the CamundaOptimize namespace.
type resolver struct {
	reader   client.Reader
	scheme   *runtime.Scheme
	optimize *v1.CamundaOptimize
	mirrors  mirroredSecrets
	// inputs are the hash inputs of the objects that this controller reads and
	// never writes: the generation of a custom resource and the resource
	// version of a Secret. They roll the pods when a referenced object changes
	// behind an unchanged reference.
	//
	// An object that this controller writes must never go in. Its own write
	// bumps the generation, the hash changes, the pods roll, and the next
	// reconcile hashes the new generation. The referenced CamundaCluster is
	// such an object, because the exporter patch writes its spec. See exists.
	inputs []string
}

// preCheck resolves every reference of optimize, in the documented order: the
// attachment to the cluster, the referenced cluster, its secondary storage,
// the version gate, the Management Identity contract and its client Secret,
// the platform config of the cluster, the Elasticsearch credentials, and the
// exporter settings already on the cluster. A Secret outside the
// CamundaOptimize namespace is copied into the returned mirrors, and the input
// references the copy, so the renderer only ever names Secrets of that
// namespace.
//
// A failed check returns a *conditions.PreCheckFailure: ClusterAlreadyAttached
// when another CamundaOptimize holds the cluster, InvalidReference for a
// dangling reference, StorageTypeMismatch for a cluster that is not on
// Elasticsearch, VersionMismatch for a minor that differs from the cluster's,
// MissingSecret for a missing Secret or key, and ExporterConflict when an
// exporter setting collides with an entry the user owns. Any other error is a
// transient API failure.
func (r *Reconciler) preCheck(ctx context.Context, optimize *v1.CamundaOptimize) (resolved, error) {
	res := &resolver{
		reader:   r.APIReader,
		scheme:   r.Scheme,
		optimize: optimize,
		mirrors:  mirroredSecrets{},
	}
	out := resolved{
		Input:      components.Input{Optimize: optimize, ClusterName: optimize.Spec.ClusterRef.Name},
		ClusterKey: client.ObjectKey{Namespace: optimize.Namespace, Name: optimize.Spec.ClusterRef.Name},
	}

	holder, err := r.attachmentHolder(ctx, optimize)
	if err != nil {
		return out, err
	}
	if holder != nil && holder.Name != optimize.Name {
		return out, &conditions.PreCheckFailure{
			Reason: v1.ReasonClusterAlreadyAttached,
			Message: fmt.Sprintf(
				"CamundaOptimize %q is already attached to CamundaCluster %q; one cluster carries one Optimize instance",
				holder.Name,
				optimize.Spec.ClusterRef.Name,
			),
		}
	}

	var cluster v1.CamundaCluster
	if err := res.exists(ctx, out.ClusterKey, &cluster); err != nil {
		return out, err
	}

	binding, err := res.resolveStorage(ctx, &cluster)
	if err != nil {
		return out, err
	}

	effective, err := res.resolveEffective(ctx, &cluster)
	if err != nil {
		return out, err
	}
	out.Input.Partitions = effective.Partitions()

	if err := res.resolveAuth(ctx, &out); err != nil {
		return out, err
	}

	if err := res.resolvePlatform(ctx, &cluster, &out); err != nil {
		return out, err
	}

	if err := res.resolveElasticsearch(ctx, &cluster, binding, &out); err != nil {
		return out, err
	}

	if err := checkExporterConflicts(&cluster, out.ExporterStorage); err != nil {
		return out, err
	}

	out.Mirrors = res.mirrors
	slices.Sort(res.inputs)
	out.Input.HashInputs = res.inputs

	return out, nil
}

// resolveStorage reads the SecondaryStorageConfig that the cluster names and
// makes sure that it is an Elasticsearch one. Optimize reads the exported
// records out of Elasticsearch, so a cluster on relational storage has nothing
// for it to import.
func (res *resolver) resolveStorage(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (*v1.SecondaryStorageConfig, error) {
	var binding v1.SecondaryStorageConfig
	key := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Spec.StorageRef}
	if err := res.get(ctx, key, &binding); err != nil {
		return nil, err
	}

	if binding.Spec.Type != v1.SecondaryStorageTypeElasticsearch {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonStorageTypeMismatch,
			Message: fmt.Sprintf(
				"CamundaCluster %q runs on %s secondary storage; Optimize needs elasticsearch",
				cluster.Name, binding.Spec.Type,
			),
		}
	}
	if binding.Spec.Elasticsearch == nil {
		return nil, fmt.Errorf(
			"SecondaryStorageConfig %q has type elasticsearch and no elasticsearch block", key,
		)
	}

	return &binding, nil
}

// resolveEffective merges the preset of the cluster under its spec and checks
// the version gate. Camunda supports Optimize only on a matching minor, so the
// major and the minor of spec.version must equal those of the effective
// version of the cluster. The patch levels may differ: Optimize has its own
// patch line.
func (res *resolver) resolveEffective(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (clustercomponents.Effective, error) {
	var preset *v1.CamundaClusterPresetSpec
	if cluster.Spec.PresetRef != "" {
		var obj v1.CamundaClusterPreset
		if err := res.get(ctx, client.ObjectKey{Name: cluster.Spec.PresetRef}, &obj); err != nil {
			return clustercomponents.Effective{}, err
		}
		preset = &obj.Spec
	}

	merged := clustercomponents.MergePreset(cluster.Spec, preset)
	// The rules that admission cannot enforce, because a preset can supply the
	// values, are checked by the cluster controller and not by the API server.
	// A cluster below the version floor is therefore accepted by the API server
	// and never reconciled. An attachment to one deploys Optimize against a
	// cluster that never comes up.
	if err := clustercomponents.ValidateMerged(merged); err != nil {
		return clustercomponents.Effective{}, &conditions.PreCheckFailure{
			Reason: v1.ReasonInvalidReference,
			Message: fmt.Sprintf(
				"CamundaCluster %q has an invalid effective spec and is not reconciled: %s",
				cluster.Name, err,
			),
		}
	}

	effective := clustercomponents.NewEffective(merged)
	want := majorMinor(effective.Version)
	got := majorMinor(res.optimize.Spec.Version)
	if want != got {
		return effective, &conditions.PreCheckFailure{
			Reason: v1.ReasonVersionMismatch,
			Message: fmt.Sprintf(
				"spec.version %q is on minor %s; CamundaCluster %q runs %s, on minor %s",
				res.optimize.Spec.Version, got, cluster.Name, effective.Version, want,
			),
		}
	}

	return effective, nil
}

// majorMinor returns the "major.minor" prefix of a semantic version, or the
// version itself when it carries no patch level.
func majorMinor(version string) string {
	major, rest, found := strings.Cut(version, ".")
	if !found {
		return version
	}
	minor, _, _ := strings.Cut(rest, ".")

	return major + "." + minor
}

// resolveAuth reads the cluster-scoped ManagementAuthConfig that
// spec.managementAuthRef names and points its client secret reference at the
// local copy.
func (res *resolver) resolveAuth(ctx context.Context, out *resolved) error {
	var auth v1.ManagementAuthConfig
	key := client.ObjectKey{Name: res.optimize.Spec.ManagementAuthRef}
	if err := res.get(ctx, key, &auth); err != nil {
		return err
	}

	spec := auth.Spec.DeepCopy()
	if err := res.localize(ctx, &spec.ClientSecretRef, components.MirrorPurposeAuthClient); err != nil {
		return err
	}
	auth.Spec = *spec
	out.Input.Auth = &auth

	return nil
}

// resolvePlatform reads the CamundaPlatformConfig that the cluster names and
// points the license reference at its local copy. A cluster that names none
// leaves the platform spec empty, so Optimize runs from the public registry
// without a license.
func (res *resolver) resolvePlatform(ctx context.Context, cluster *v1.CamundaCluster, out *resolved) error {
	if cluster.Spec.PlatformConfigRef == "" {
		return nil
	}

	var cfg v1.CamundaPlatformConfig
	if err := res.get(ctx, client.ObjectKey{Name: cluster.Spec.PlatformConfigRef}, &cfg); err != nil {
		return err
	}
	out.Input.Platform = *cfg.Spec.DeepCopy()

	if out.Input.Platform.LicenseSecretRef != nil {
		return res.localize(ctx, out.Input.Platform.LicenseSecretRef, components.MirrorPurposeLicense)
	}

	return nil
}

// resolveElasticsearch fills both storage views: the one the Optimize pods
// read, with every Secret reference pointed at its local copy, and the one the
// exporter patch carries, with the credentials reference pointed at the copy
// that the cluster's own controller makes.
func (res *resolver) resolveElasticsearch(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	binding *v1.SecondaryStorageConfig,
	out *resolved,
) error {
	storage := binding.Spec.Elasticsearch.DeepCopy()
	creds := storage.CredentialsSecretRef

	if err := res.localizeCredentials(
		ctx,
		&storage.CredentialsSecretRef,
		components.MirrorPurposeESCredentials,
	); err != nil {
		return err
	}
	if storage.CASecretRef != nil {
		if err := res.localize(ctx, storage.CASecretRef, components.MirrorPurposeESCA); err != nil {
			return err
		}
	}
	out.Input.Storage = *storage

	// The broker reads the same credentials for its own secondary storage, so
	// the cluster's controller has already put a copy in the cluster namespace
	// under its own name when the contract names another namespace.
	exporter := *binding.Spec.Elasticsearch.DeepCopy()
	if creds.Namespace != cluster.Namespace {
		exporter.CredentialsSecretRef.Name = clustercomponents.MirroredSecretName(
			cluster, clustercomponents.MirrorPurposeESCredentials,
		)
		exporter.CredentialsSecretRef.Namespace = cluster.Namespace
	}
	out.ExporterStorage = exporter

	return nil
}

// checkExporterConflicts refuses to apply the exporter settings when the
// cluster already carries an entry of the same name that supplies its value
// the other way. Server-side apply would merge the two into one entry with
// both a value and a valueFrom, which a container rejects, so the rollout of
// the cluster would stall while the CamundaCluster still looked healthy.
func checkExporterConflicts(cluster *v1.CamundaCluster, storage v1.ElasticsearchStorage) error {
	if cluster.Spec.Zeebe == nil {
		return nil
	}

	conflicts := components.ExporterConflicts(components.ExporterEnv(storage), cluster.Spec.Zeebe.ExtraEnv)
	if len(conflicts) == 0 {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonExporterConflict,
		Message: fmt.Sprintf(
			"spec.zeebe.extraEnv of CamundaCluster %q carries %s with the other kind of value; "+
				"remove the entries or let the operator own them",
			cluster.Name, strings.Join(conflicts, ", "),
		),
	}
}

// get reads the referenced object through exists and records its generation as
// a hash input, so a change to that object rolls the pods.
//
// Use it only for an object that this controller never writes. The referenced
// CamundaCluster goes through exists instead.
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

// exists reads the referenced object without the cache and records no hash
// input. A missing object maps to InvalidReference, naming the kind and the
// reference (with its namespace for a namespaced kind). Any other error is a
// transient API failure.
//
// The referenced CamundaCluster is read through exists, not through get. This
// controller writes that object: the exporter patch sets spec.zeebe.extraEnv on
// it, which bumps its generation. A hash over that generation would roll the
// pods in answer to the controller's own write, and again on every unrelated
// edit of the cluster, such as a replica count or a label. Nothing is lost. The
// pods read no field of the cluster spec: the partition count and the version
// are already in the rendered environment and the image, which the hash covers
// through the rendered output, and the preset that can supply the partition
// count is hashed through get.
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

// localizeCredentials is localize for a username and password reference.
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

// secret checks that the Secret at key carries every one of keys. When the
// Secret lives outside the CamundaOptimize namespace, it copies the keys into
// the mirror of that purpose and returns the key of the copy in the
// CamundaOptimize namespace; otherwise it returns key unchanged. A missing
// Secret or key maps to MissingSecret.
func (res *resolver) secret(
	ctx context.Context,
	key client.ObjectKey,
	purpose string,
	keys ...string,
) (client.ObjectKey, error) {
	found, msg, err := secretref.Get(ctx, res.reader, key, keys...)
	if err != nil {
		return client.ObjectKey{}, fmt.Errorf("reading Secret %q: %w", key, err)
	}
	if msg != "" {
		return client.ObjectKey{}, &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: msg}
	}
	// The hash input is the Secret that was read, never the copy that this
	// controller applies. Hashing the copy would feed the controller's own
	// write back into the hash.
	res.inputs = append(res.inputs, "Secret/"+objectPath(key)+"="+found.ResourceVersion)

	if key.Namespace == res.optimize.Namespace {
		return key, nil
	}

	data := make(map[string][]byte, len(keys))
	for _, k := range keys {
		data[k] = found.Data[k]
	}
	res.mirrors[purpose] = data

	return client.ObjectKey{
		Namespace: res.optimize.Namespace,
		Name:      components.MirroredSecretName(res.optimize, purpose),
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
