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
	"context"
	"fmt"
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// ComponentRestore is the camunda.io/component of every resource that a
// restore renders.
const ComponentRestore = "restore"

// Target is everything a restore reads off the live broker StatefulSet of its
// cluster. The restore never re-renders the broker configuration. It copies
// it, so the restore application and the brokers cannot disagree.
type Target struct {
	// ClusterName is the CamundaCluster the StatefulSet belongs to. Every
	// resource a restore renders carries it, so an extension finds the
	// restore of a cluster by the same label it finds the cluster's workloads
	// by.
	ClusterName string
	// StatefulSet is the live broker StatefulSet, <cluster>-zeebe.
	StatefulSet *appsv1.StatefulSet
	// Broker is the container named camunda in the pod template. The Job
	// copies its image, environment, mounts, resources, and security context.
	Broker *corev1.Container
	// Brokers is the broker count, read from CAMUNDA_CLUSTER_SIZE on the
	// broker container. A suspended StatefulSet runs at zero replicas, so
	// spec.replicas cannot answer it.
	Brokers int32
	// Partitions is the partition count, read from
	// CAMUNDA_CLUSTER_PARTITIONCOUNT on the broker container. A restore of a
	// backup must match it.
	Partitions int32
	// Version is the Camunda version, read from the tag of the broker image.
	Version string
	// ClaimTemplate is the data claim template of the StatefulSet. It carries
	// the storage class, the access modes, the labels, and the size that a
	// recreated volume keeps.
	ClaimTemplate *corev1.PersistentVolumeClaim
}

// ReadTarget reads the broker StatefulSet of cluster and extracts the facts a
// restore needs from it. Every fact that the StatefulSet cannot answer is a
// *conditions.PreCheckFailure with v1.ReasonInvalidReference, whose message
// names what is missing. A transport error is a plain wrapped error, and the
// caller retries it.
//
// A cluster whose broker StatefulSet was deleted cannot restore until its own
// controller applies it again. Suspending a cluster keeps the StatefulSet in
// place, so this is an edge, not a flow.
func ReadTarget(
	ctx context.Context,
	reader client.Reader,
	cluster *v1.CamundaCluster,
) (*Target, error) {
	name := components.WorkloadName(cluster, components.ComponentZeebe)

	var sts appsv1.StatefulSet
	key := types.NamespacedName{Namespace: cluster.Namespace, Name: name}
	if err := reader.Get(ctx, key, &sts); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, invalidTarget(
				name, "the broker StatefulSet %s/%s does not exist", cluster.Namespace, name,
			)
		}

		return nil, fmt.Errorf("reading the broker StatefulSet %s/%s: %w", cluster.Namespace, name, err)
	}

	broker := containerNamed(sts.Spec.Template.Spec.Containers, components.ContainerCamunda)
	if broker == nil {
		return nil, invalidTarget(name, "it has no container named %q", components.ContainerCamunda)
	}

	brokers, err := envInt32(name, broker, camundaconfig.KeyClusterSize)
	if err != nil {
		return nil, err
	}

	partitions, err := envInt32(name, broker, camundaconfig.KeyClusterPartitionCount)
	if err != nil {
		return nil, err
	}

	version := imageTag(broker.Image)
	if version == "" {
		return nil, invalidTarget(
			name, "the broker image %q carries no version tag", broker.Image,
		)
	}

	claim := claimTemplateNamed(sts.Spec.VolumeClaimTemplates, components.DataVolumeName)
	if claim == nil {
		return nil, invalidTarget(
			name, "it has no volume claim template named %q", components.DataVolumeName,
		)
	}

	return &Target{
		ClusterName:   cluster.Name,
		StatefulSet:   &sts,
		Broker:        broker,
		Brokers:       brokers,
		Partitions:    partitions,
		Version:       version,
		ClaimTemplate: claim,
	}, nil
}

// invalidTarget builds the one failure shape that every unreadable fact of
// the broker StatefulSet reports.
func invalidTarget(name, format string, args ...any) *conditions.PreCheckFailure {
	return &conditions.PreCheckFailure{
		Reason: v1.ReasonInvalidReference,
		Message: fmt.Sprintf(
			"the restore cannot read the broker StatefulSet %s: %s", name, fmt.Sprintf(format, args...),
		),
	}
}

func containerNamed(containers []corev1.Container, name string) *corev1.Container {
	for i := range containers {
		if containers[i].Name == name {
			return &containers[i]
		}
	}

	return nil
}

func claimTemplateNamed(claims []corev1.PersistentVolumeClaim, name string) *corev1.PersistentVolumeClaim {
	for i := range claims {
		if claims[i].Name == name {
			return &claims[i]
		}
	}

	return nil
}

// envInt32 reads one plain numeric variable off the broker container. A
// variable that carries a ValueFrom is not readable here: its value lives in
// a Secret, a ConfigMap, or a field of the pod, and only the kubelet resolves
// it.
func envInt32(sts string, broker *corev1.Container, key camundaconfig.Key) (int32, error) {
	name := key.Env()
	for _, env := range broker.Env {
		if env.Name != name {
			continue
		}
		if env.ValueFrom != nil {
			return 0, invalidTarget(sts, "%s comes from a reference and is not readable here", name)
		}

		value, err := strconv.ParseInt(env.Value, 10, 32)
		if err != nil {
			return 0, invalidTarget(sts, "%s is %q, which is not a number", name, env.Value)
		}

		return int32(value), nil
	}

	return 0, invalidTarget(sts, "its container carries no %s", name)
}

// imageTag returns the tag of an image reference, or the empty string when it
// carries none. A registry host with a port also holds a colon, so only a
// colon after the last slash starts a tag.
func imageTag(image string) string {
	colon := strings.LastIndex(image, ":")
	if colon < 0 || colon < strings.LastIndex(image, "/") {
		return ""
	}

	return image[colon+1:]
}
