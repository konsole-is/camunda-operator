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
	// distribution (Camunda 8.9 restore guides). It ships in the broker image,
	// so a restore runs the version of the brokers.
	RestoreEntrypoint = "/usr/local/camunda/bin/restore"
	// noRetries is the backoff limit of the per-broker Jobs. The restore
	// application refuses a non-empty data directory, so a second pod finds
	// what the first one wrote and fails for the wrong reason.
	noRetries = int32(0)
)

// JobInput is everything the restore Job of one broker renders from.
type JobInput struct {
	// Target holds the live broker StatefulSet that the Job mirrors.
	Target *Target
	// Owner is the restore resource. Its namespace gives the Job its
	// namespace. BuildJob sets no owner reference: set the controller
	// reference before you create the Job, or deleting the restore leaves the
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
	// relational path.
	Args []string
}

// BuildJob renders the Job that runs the restore application for one broker.
// The pod is a copy of the broker pod of the live StatefulSet, so the restore
// application reads the configuration, the credentials, and the files that the
// brokers use. The Job never retries its pod.
//
// BuildJob returns an error for an incomplete target, an ordinal outside the
// broker count, a nil owner, an OwnerLabel of no restore kind or with no
// name, and an owner that OwnerLabel does not name.
//
// Create the Job once, then read it by JobName. The pod template of a Job is
// immutable, so a second apply after the StatefulSet changed fails validation.
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
	// Two different resources here put the Job under the name of one restore
	// in the namespace of the other, where no controller looks for it. The
	// label carries the bounded name, so the object name is bounded too.
	if labels.BoundedName(in.Owner.GetName(), validation.LabelValueMaxLength) != in.OwnerLabel.Name {
		return nil, fmt.Errorf(
			"building the restore Job of broker %d: the owner is %q but the owner label names %q",
			in.Ordinal, in.Owner.GetName(), in.OwnerLabel.Name,
		)
	}

	managed := JobLabels(in.OwnerLabel, in.Target.ClusterName)

	// The pods look like broker pods to a topology spread constraint, so the
	// recreated volumes land where the brokers can schedule afterwards.
	podLabels := labels.Merge(in.Target.StatefulSet.Spec.Template.Labels, managed)

	pod := *in.Target.StatefulSet.Spec.Template.Spec.DeepCopy()
	pod.RestartPolicy = corev1.RestartPolicyNever
	pod.Containers = []corev1.Container{restoreContainer(in)}
	// The trust store container stays: it fills a volume that the restore
	// container mounts, and without it the restore cannot reach the backup
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

// jobKindInfixes are the CRD short names. Two restores of different kinds and
// the same name can live in one namespace, so the kind is part of a Job name.
var jobKindInfixes = map[string]string{
	labels.LogicalRestoreElasticsearchKey: "lres",
	labels.LogicalRestoreRDBMSKey:         "lrrdbms",
	labels.PointInTimeRestoreKey:          "pitr",
}

// JobName returns the name of the restore Job of one broker:
// <restore>-<kind>-<ordinal>, where kind is the short name of the restore
// CRD, for example pitr. The name depends on the owner and the ordinal alone.
//
// Build the owner with the constructor of its kind in pkg/labels. A long
// restore name is bounded to a DNS label, and two long names still get two
// Job names.
//
// JobName returns the empty string for an owner of another kind, an owner
// without a name, and a negative ordinal.
func JobName(owner labels.Owner, ordinal int32) string {
	kind, known := jobKindInfixes[owner.Key]
	if !known || owner.Name == "" || ordinal < 0 {
		return ""
	}

	suffix := "-" + kind + "-" + strconv.FormatInt(int64(ordinal), 10)

	return labels.BoundedName(owner.Name, validation.DNS1123LabelMaxLength-len(suffix)) + suffix
}

// isNil also catches a typed nil pointer, which is not equal to nil inside an
// interface.
func isNil(obj client.Object) bool {
	if obj == nil {
		return true
	}

	value := reflect.ValueOf(obj)

	return value.Kind() == reflect.Ptr && value.IsNil()
}

// JobLabels returns the operator labels of a restore Job and its pods: the
// owner, the restore component, the operator as manager, and the cluster.
//
// Do not build these labels by hand. A restore name or a cluster name longer
// than 63 characters is bounded in the label value.
func JobLabels(owner labels.Owner, cluster string) map[string]string {
	managed := labels.Managed(owner, ComponentRestore)
	managed[labels.ClusterKey] = labels.OwnerName(cluster)

	return managed
}

// restoreContainer drops the node-id shell wrapper of the broker: a Job pod
// has a random host name, so the ordinal cannot come from the host name. The
// restore application is a one-shot process, so it carries no probe.
func restoreContainer(in JobInput) corev1.Container {
	// Target.Broker points into the StatefulSet of the target. A shallow
	// clone shares the pointers inside a mount or an environment source.
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

// overrideEnv replaces a broker variable of the same name, so the restore
// cannot read the broker value by accident.
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

func keepTrustStore(containers []corev1.Container) []corev1.Container {
	for _, c := range containers {
		if c.Name == components.InitContainerTrustStore {
			return []corev1.Container{*c.DeepCopy()}
		}
	}

	return nil
}

// spreadOverRestorePods keeps the broker spread constraints but selects the
// restore pods. A selector of the broker component counts no restore pod, so
// every restore pod can land in one zone. With a WaitForFirstConsumer storage
// class the recreated volumes then bind in that zone, and the brokers cannot
// spread over them afterwards.
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
		// A Job stamps a per-Job identity label onto its pods, so a match
		// label key that keeps the broker pods together keeps restore pods apart.
		constraint.MatchLabelKeys = nil
		retargeted = append(retargeted, constraint)
	}

	return retargeted
}

// JobSelector returns the labels that select the Jobs that BuildJob renders for
// one restore and their pods, for a List and for podstate.Stuck.
func JobSelector(owner labels.Owner) map[string]string {
	return labels.Discovery(owner, ComponentRestore)
}
