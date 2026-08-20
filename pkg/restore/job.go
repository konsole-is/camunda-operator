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
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"

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
	// jobNameInfix separates the restore name from the broker ordinal in a Job
	// name.
	jobNameInfix = "-restore-"
	// nameHashLength is the hex length of the hash that keeps a long name
	// unique after boundedName truncates it to a DNS label.
	nameHashLength = 10
	// noRetries is the backoff limit of every restore Job. The restore
	// application refuses a non-empty data directory, so a second pod would
	// find what the first one wrote and fail for the wrong reason. A failed
	// restore is retried with a new restore resource, which creates the volume
	// again first.
	noRetries = int32(0)
)

// JobInput is everything the restore Job of one broker renders from. The
// controller resolves it. The builder only shapes it.
type JobInput struct {
	// Target holds the live broker StatefulSet that the Job mirrors.
	Target *Target
	// Owner is the restore resource. Its name gives the Job its name, and its
	// namespace gives the Job its namespace.
	//
	// BuildJob sets no owner reference on the Job, because it renders without
	// a scheme. The caller sets the controller reference before it applies the
	// Job. Without that reference, deleting the restore resource leaves the
	// Jobs behind.
	Owner client.Object
	// OwnerLabel is the owner label of the restore kind:
	// labels.LogicalRestore or labels.PointInTimeRestore.
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

// JobName returns the name of the restore Job of one broker. It derives from
// the restore name and the ordinal alone, so a reconcile that re-enters after
// a crash adopts the Job it already created. A restore name can be a full DNS
// subdomain, but a Job name is a DNS label, so a long name truncates
// deterministically and stays unique through a hash of the whole name.
func JobName(owner client.Object, ordinal int32) string {
	suffix := jobNameInfix + strconv.FormatInt(int64(ordinal), 10)

	return boundedName(owner.GetName(), validation.DNS1123LabelMaxLength-len(suffix)) + suffix
}

// boundedName returns name when it fits limit, or its head followed by a hash
// of the whole name otherwise. The result is deterministic, so every render of
// one restore agrees, and two names that share the head differ in the hash.
func boundedName(name string, limit int) string {
	if len(name) <= limit {
		return name
	}
	sum := sha256.Sum256([]byte(name))
	hash := hex.EncodeToString(sum[:])[:nameHashLength]

	return strings.TrimRight(name[:limit-1-nameHashLength], "-.") + "-" + hash
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
func BuildJob(in JobInput) (*batchv1.Job, error) {
	if err := in.Target.complete(); err != nil {
		return nil, fmt.Errorf("building the restore Job of broker %d: %w", in.Ordinal, err)
	}
	if isNil(in.Owner) || in.OwnerLabel.Key == "" || in.OwnerLabel.Name == "" {
		return nil, fmt.Errorf(
			"building the restore Job of broker %d: the input names no owner", in.Ordinal,
		)
	}
	if in.Ordinal < 0 || in.Ordinal >= in.Target.Brokers {
		return nil, fmt.Errorf(
			"building the restore Job of broker %d: the cluster runs %d brokers",
			in.Ordinal, in.Target.Brokers,
		)
	}

	// Label values are bounded like DNS labels, and a restore name can be
	// longer.
	owner := in.OwnerLabel
	owner.Name = boundedName(owner.Name, validation.LabelValueMaxLength)

	managed := labels.Managed(owner, ComponentRestore)
	managed[labels.ClusterKey] = in.Target.ClusterName

	// The pods look like broker pods to a topology spread constraint, so the
	// recreated volumes land where the brokers can schedule afterwards. The
	// operator labels win over the copied ones.
	podLabels := labels.Merge(in.Target.StatefulSet.Spec.Template.Labels, managed)

	pod := *in.Target.StatefulSet.Spec.Template.Spec.DeepCopy()
	pod.RestartPolicy = corev1.RestartPolicyNever
	pod.Containers = []corev1.Container{restoreContainer(in)}
	pod.TopologySpreadConstraints = spreadOverRestorePods(pod.TopologySpreadConstraints, owner)
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
			Name:      JobName(in.Owner, in.Ordinal),
			Namespace: in.Owner.GetNamespace(),
			Labels:    managed,
		},
		Spec: batchv1.JobSpec{
			BackoffLimit: new(noRetries),
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels:      podLabels,
					Annotations: in.Target.StatefulSet.Spec.Template.Annotations,
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
// restore, so a constraint that selects the broker component would count no
// pod at all and let every restore pod land in one zone. With a
// WaitForFirstConsumer storage class the recreated volumes would then bind in
// that one zone, and the brokers could not spread over them afterwards.
func spreadOverRestorePods(
	constraints []corev1.TopologySpreadConstraint,
	owner labels.Owner,
) []corev1.TopologySpreadConstraint {
	if len(constraints) == 0 {
		return nil
	}

	retargeted := make([]corev1.TopologySpreadConstraint, 0, len(constraints))
	for _, constraint := range constraints {
		constraint.LabelSelector = &metav1.LabelSelector{
			MatchLabels: labels.Discovery(owner, ComponentRestore),
		}
		// MatchLabelKeys would add the value of a pod label to the selector
		// above. A Job stamps a per-Job identity onto its pods, so a key that
		// keeps the broker pods together would keep every restore pod apart.
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
	broker := in.Target.Broker

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
		Args:            in.Args,
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
