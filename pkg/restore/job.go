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

package restore

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

const (
	// RestoreEntrypoint is the standalone restore application of the Camunda
	// distribution. It sits next to the broker entrypoint in the same image,
	// so a restore can never run another version than the brokers (Camunda 8.9
	// restore guides: command "/usr/local/camunda/bin/restore").
	RestoreEntrypoint = "/usr/local/camunda/bin/restore"
	// noRetries is the backoff limit of every restore Job. The restore
	// application refuses a non-empty data directory, so a second pod finds
	// what the first one wrote and fails for the wrong reason. A failed
	// restore is retried with a new restore resource, which creates the volume
	// again first.
	noRetries = int32(0)
)

// JobInput is everything the restore Job of one broker renders from. The
// controller resolves it. The builder only shapes it.
type JobInput struct {
	// Target holds the live broker StatefulSet that the Job mirrors.
	Target *Target
	// Owner is the restore resource. Its namespace gives the Job its
	// namespace. The name of the Job comes from OwnerLabel, and BuildJob
	// fails when the two name different resources.
	//
	// BuildJob sets no owner reference on the Job, because it renders without
	// a scheme. The caller sets the controller reference before it applies the
	// Job. Without that reference, deleting the restore resource leaves the
	// Jobs behind.
	Owner client.Object
	// OwnerLabel is the owner label of the restore kind, from pkg/labels, for
	// example labels.PointInTimeRestore.
	OwnerLabel labels.Owner
	// Ordinal is the broker this Job restores. It becomes the node id and
	// selects the data volume.
	Ordinal int32
	// Args are the arguments of the restore application: --backupId on the
	// Elasticsearch path, --to on the point-in-time path, and none on the
	// relational path, where the application reads the exporter position from
	// the restored database itself.
	Args []string
}

// isNil reports whether obj holds no object. An interface that carries a typed
// nil pointer is not equal to nil, and every read of its name panics the same
// way an untyped nil does.
func isNil(obj client.Object) bool {
	if obj == nil {
		return true
	}

	value := reflect.ValueOf(obj)

	return value.Kind() == reflect.Ptr && value.IsNil()
}

// jobKindInfixes name each restore kind inside a Job name. Two restores of
// different kinds and the same name can live in one namespace, so the kind has
// to reach the name. The values are the short names of the CRDs, which is what
// a user already types with kubectl.
var jobKindInfixes = map[string]string{
	labels.LogicalRestoreElasticsearchKey: "lres",
	labels.LogicalRestoreRDBMSKey:         "lrrdbms",
	labels.PointInTimeRestoreKey:          "pitr",
}

// JobName returns the name of the restore Job of one broker:
// <restore>-<kind>-<ordinal>, where kind is the short name of the restore
// CRD, for example pitr for a PointInTimeRestore. It derives from the owner
// and the ordinal alone, so a reconcile that re-enters after a crash finds the
// Job it already created.
//
// A restore name can be a full DNS subdomain, but a Job name is a DNS label.
// Build the owner through the constructor of its kind in pkg/labels, which
// bounds the name to what a label value admits. This bounds it a second time,
// to leave room for the suffix. Each step ends in a hash of the value that it
// cuts, so two long restore names still get two Job names.
//
// JobName returns the empty string for an owner of any other kind, for an
// owner without a name, and for a negative ordinal. BuildJob rejects all
// three.
func JobName(owner labels.Owner, ordinal int32) string {
	kind, known := jobKindInfixes[owner.Key]
	if !known || owner.Name == "" || ordinal < 0 {
		return ""
	}

	suffix := "-" + kind + "-" + strconv.FormatInt(int64(ordinal), 10)

	return labels.BoundedName(owner.Name, validation.DNS1123LabelMaxLength-len(suffix)) + suffix
}

// JobLabels returns every label that a restore Job and its pods carry: the
// owner of the restore, the restore component, the operator as manager, and
// the cluster the restore runs against.
//
// A consumer that selects these Jobs must build the labels here rather than
// by hand. Both owner label values are bounded to a DNS label, and a restore
// name and a cluster name can both be longer, so a hand-built selector misses
// every Job of a restore or a cluster whose name passes 63 characters.
func JobLabels(owner labels.Owner, cluster string) map[string]string {
	managed := labels.Managed(owner, ComponentRestore)
	managed[labels.ClusterKey] = labels.OwnerName(cluster)

	return managed
}

// JobSelector returns the labels that select every restore Job of one restore
// and every pod of those Jobs. It is what a caller passes to a List and to
// podstate.Stuck. It carries no cluster label, so it keeps selecting after a
// restore moved between clusters, and it carries no manager label, which a
// selector must not depend on.
//
// The pod informer of the manager is scoped by the manager label. That scope
// belongs to the cache, not to this selector: a caller that lists through the
// live API reads the same pods with these labels alone.
func JobSelector(owner labels.Owner) map[string]string {
	return labels.Discovery(owner, ComponentRestore)
}

// BuildJob renders the Job that runs the restore application for one broker.
//
// The pod starts as a copy of the broker pod of the live StatefulSet, so the
// restore application reads the same secondary storage with the same
// credentials, writes to the same backup store, and finds every file at the
// path the brokers use. BuildJob changes five things:
//
//   - The pod runs one container, and that container runs the restore
//     entrypoint under the restore Spring profile, with the node id as a
//     plain value. It carries no probe, because the restore application is a
//     one-shot process.
//   - The data volume becomes a claim on data-<cluster>-zeebe-<ordinal>,
//     because a Job has no claim template.
//   - The pod never restarts, and the Job never retries it.
//   - The topology spread constraints keep the shape of the broker's own, but
//     they select the pods of this restore. See spreadOverRestorePods.
//   - The operator labels of the restore go over the copied broker labels, so
//     the owner and the component name this restore.
//
// BuildJob sets no owner reference. See JobInput.Owner.
//
// The caller applies the Job once and then tracks it by name. A Job's
// spec.template is immutable after creation, and BuildJob renders it from the
// live broker StatefulSet, so a second apply after that StatefulSet changed
// is a validation error from the API server, not an adoption of the running
// Job. A controller therefore reads the Job by JobName first and applies only
// when it finds none.
func BuildJob(in JobInput) (*batchv1.Job, error) {
	if err := in.Target.complete(); err != nil {
		return nil, fmt.Errorf("building the restore Job of broker %d: %w", in.Ordinal, err)
	}
	if in.Ordinal < 0 || in.Ordinal >= in.Target.Brokers {
		return nil, fmt.Errorf(
			"building the restore Job of broker %d: the cluster runs %d brokers",
			in.Ordinal, in.Target.Brokers,
		)
	}

	name := JobName(in.OwnerLabel, in.Ordinal)
	if isNil(in.Owner) || name == "" {
		return nil, fmt.Errorf(
			"building the restore Job of broker %d: the input names no restore owner", in.Ordinal,
		)
	}
	// The Job takes its name from the label and its namespace from the object.
	// Two different resources there put the Job under one name in the other's
	// namespace, where neither controller looks for it.
	//
	// The comparison bounds the name of the object, because an Owner carries
	// the bounded name that a label value admits. A long name and its bound
	// are the same resource. Two different long names are not: the bound ends
	// in a hash of the whole name, so they stay apart.
	if labels.BoundedName(in.Owner.GetName(), validation.LabelValueMaxLength) != in.OwnerLabel.Name {
		return nil, fmt.Errorf(
			"building the restore Job of broker %d: the owner is %q but the owner label names %q",
			in.Ordinal, in.Owner.GetName(), in.OwnerLabel.Name,
		)
	}

	managed := JobLabels(in.OwnerLabel, in.Target.ClusterName)

	// The pods look like broker pods to a topology spread constraint, so the
	// recreated volumes land where the brokers can schedule afterwards. The
	// operator labels win over the copied ones.
	podLabels := labels.Merge(in.Target.StatefulSet.Spec.Template.Labels, managed)

	pod := *in.Target.StatefulSet.Spec.Template.Spec.DeepCopy()
	pod.RestartPolicy = corev1.RestartPolicyNever
	pod.Containers = []corev1.Container{restoreContainer(in)}
	// A broker init container prepares a broker, and the platform injects its
	// own again when this pod is created. The broker's copy therefore runs
	// twice if it stays. The trust store container is the exception: it fills
	// a volume that the restore container mounts, and the restore JVM runs
	// with the trust store options of the broker. Without it that JVM reads a
	// store that no container wrote, and the restore cannot reach the backup
	// store over TLS.
	pod.InitContainers = keepTrustStore(pod.InitContainers)
	pod.TopologySpreadConstraints = spreadOverRestorePods(pod.TopologySpreadConstraints, in.OwnerLabel)
	pod.Volumes = append(
		slices.Clone(pod.Volumes),
		corev1.Volume{
			Name: components.DataVolumeName,
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: in.Target.claimName(in.Ordinal),
				},
			},
		},
	)

	return &batchv1.Job{
		TypeMeta: metav1.TypeMeta{APIVersion: "batch/v1", Kind: "Job"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: in.Owner.GetNamespace(),
			Labels:    managed,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(noRetries),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: maps.Clone(in.Target.StatefulSet.Spec.Template.Annotations),
				},
				Spec: pod,
			},
		},
	}, nil
}

// spreadOverRestorePods retargets the spread constraints of the broker pods
// at the pods of this restore. The shape stays the broker's own: the same
// topology key, the same skew, and the same policies, so the restore pods
// spread exactly as the brokers do.
//
// The selector has to change. A restore pod carries camunda.io/component:
// restore, so a constraint that selects the broker component counts no pod at
// all and lets every restore pod land in one zone. With a
// WaitForFirstConsumer storage class the recreated volumes then bind in that
// one zone, and the brokers cannot spread over them afterwards.
func spreadOverRestorePods(
	constraints []corev1.TopologySpreadConstraint,
	owner labels.Owner,
) []corev1.TopologySpreadConstraint {
	if len(constraints) == 0 {
		return nil
	}

	retargeted := make([]corev1.TopologySpreadConstraint, 0, len(constraints))
	for _, constraint := range constraints {
		constraint.LabelSelector = &metav1.LabelSelector{MatchLabels: JobSelector(owner)}
		// MatchLabelKeys adds the value of a pod label to the selector above.
		// A Job stamps a per-Job identity onto its pods, so a key that keeps
		// the broker pods together keeps every restore pod apart.
		constraint.MatchLabelKeys = nil
		retargeted = append(retargeted, constraint)
	}

	return retargeted
}

// restoreContainer is the broker container running the restore application:
// the same image, environment, mounts, resources, and security context, with
// the broker's node-id shell wrapper replaced. A Job pod has a random host
// name, so the ordinal cannot come from the host name as it does for a
// StatefulSet pod. The restore application is a one-shot process, so it also
// carries no probe.
func restoreContainer(in JobInput) corev1.Container {
	// Target.Broker points into the StatefulSet that ReadTarget holds. One
	// deep copy keeps every Job of a target independent of the target and of
	// its siblings, down to the pointers inside a mount or an environment
	// source. A shallow clone of those slices copies the elements and leaves
	// what they point at shared.
	broker := in.Target.Broker.DeepCopy()

	env := overrideEnv(
		broker.Env,
		corev1.EnvVar{
			Name:  camundaconfig.KeyClusterNodeID.Env(),
			Value: strconv.FormatInt(int64(in.Ordinal), 10),
		},
		corev1.EnvVar{
			Name:  camundaconfig.EnvSpringProfilesActive,
			Value: camundaconfig.ProfileRestore,
		},
	)

	return corev1.Container{
		Name:            ComponentRestore,
		Image:           broker.Image,
		Command:         []string{RestoreEntrypoint},
		Args:            slices.Clone(in.Args),
		Env:             env,
		EnvFrom:         broker.EnvFrom,
		Resources:       broker.Resources,
		VolumeMounts:    broker.VolumeMounts,
		SecurityContext: broker.SecurityContext,
	}
}

// overrideEnv returns the broker environment with each override applied in
// place of the variable of the same name, and appended when the broker sets
// none. A name appears once, so the apply cannot fail on a duplicate list-map
// key and the restore cannot read the broker's value by accident.
func overrideEnv(broker []corev1.EnvVar, overrides ...corev1.EnvVar) []corev1.EnvVar {
	env := make([]corev1.EnvVar, 0, len(broker)+len(overrides))
	env = append(env, broker...)

	for _, override := range overrides {
		at := slices.IndexFunc(env, func(e corev1.EnvVar) bool { return e.Name == override.Name })
		if at < 0 {
			env = append(env, override)

			continue
		}
		env[at] = override
	}

	return env
}

// keepTrustStore returns the init containers of a broker pod that a restore
// pod must run: the trust store container alone.
func keepTrustStore(containers []corev1.Container) []corev1.Container {
	for _, c := range containers {
		if c.Name == components.InitContainerTrustStore {
			return []corev1.Container{*c.DeepCopy()}
		}
	}

	return nil
}
