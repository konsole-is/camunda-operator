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
	"errors"
	"fmt"
	"strconv"

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

// ComponentRestore is the camunda.io/component of the restore Jobs and their
// pods. A recreated broker volume keeps the labels of the claim template.
const ComponentRestore = "restore"

// Target is everything a restore reads off the live broker StatefulSet of its
// cluster. A restore copies the broker configuration from it and never renders
// its own, so the restore application and the brokers cannot disagree.
//
// Get a Target from ResolveTarget. The methods of a Target that a caller built
// by hand can panic. Prepare and Primary report a failure Outcome for an
// incomplete Target, and BuildJob and RecreateClaims return an error.
type Target struct {
	// ClusterName is the CamundaCluster the StatefulSet belongs to. Every
	// resource that a restore renders carries it in the cluster label.
	ClusterName string
	// StatefulSet is the live broker StatefulSet, <cluster>-zeebe.
	StatefulSet *appsv1.StatefulSet
	// Broker is the container named camunda in the pod template.
	Broker *corev1.Container
	// Brokers is the broker count, read from CAMUNDA_CLUSTER_SIZE on the
	// broker container, and always one or more. A suspended StatefulSet runs
	// at zero replicas, so spec.replicas cannot answer it.
	Brokers int32
	// Partitions is the partition count, read from
	// CAMUNDA_CLUSTER_PARTITIONCOUNT on the broker container. A restore of a
	// backup must match it.
	Partitions int32
	// Version is the Camunda version, read from the camunda.io/broker-version
	// annotation of the StatefulSet.
	Version string
	// ClaimTemplate is the data claim template of the StatefulSet.
	ClaimTemplate *corev1.PersistentVolumeClaim
}

// readTarget returns a *conditions.PreCheckFailure with
// v1.ReasonInvalidReference for a fact that the StatefulSet cannot answer, and
// a wrapped error for a failed read.
func readTarget(
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

	brokers, err := envCount(name, broker, camundaconfig.KeyClusterSize)
	if err != nil {
		return nil, err
	}

	partitions, err := envCount(name, broker, camundaconfig.KeyClusterPartitionCount)
	if err != nil {
		return nil, err
	}

	version := sts.Annotations[components.BrokerVersionAnnotation]
	if version == "" {
		return nil, invalidTarget(
			name,
			"it carries no %s annotation. The cluster controller stamps it on every apply, "+
				"so let one reconcile of the cluster finish and retry",
			components.BrokerVersionAnnotation,
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

// complete returns an error when the target lacks a pointer or a count that a
// render reads, also for a nil target. The callers run inside a reconcile,
// where a nil dereference takes the whole manager down.
func (t *Target) complete() error {
	switch {
	case t == nil:
		return errors.New("the target is empty")
	case t.StatefulSet == nil:
		return errors.New("the target carries no broker StatefulSet")
	case t.Broker == nil:
		return errors.New("the target carries no broker container")
	case t.ClaimTemplate == nil:
		return errors.New("the target carries no data claim template")
	case t.Brokers < 1:
		return errors.New("the target carries no broker count of one or more")
	case t.Partitions < 1:
		return errors.New("the target carries no partition count of one or more")
	}

	return nil
}

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

// envCount cannot read a variable that carries a ValueFrom: only the kubelet
// resolves it. A count below one is a failure, because zero brokers makes a
// restore recreate no volume, run no Job, and report that it finished.
func envCount(sts string, broker *corev1.Container, key camundaconfig.Key) (int32, error) {
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
		if value < 1 {
			return 0, invalidTarget(sts, "%s is %q, which is not a count of one or more", name, env.Value)
		}

		return int32(value), nil
	}

	return 0, invalidTarget(sts, "its container carries no %s", name)
}

func claimTemplateNamed(claims []corev1.PersistentVolumeClaim, name string) *corev1.PersistentVolumeClaim {
	for i := range claims {
		if claims[i].Name == name {
			return &claims[i]
		}
	}

	return nil
}
