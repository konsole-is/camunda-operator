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
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/editors"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/selectors"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/deployment"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/statefulset"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// The names of the mutations that layer the override surfaces of the spec
// onto the base workload of a process. They match the mutation names of the
// ElasticsearchCluster component.
const (
	// MutationResources sets the container resources of the process
	// (resources of its component block).
	MutationResources = "Resources"
	// MutationSchedulingConstraints sets the affinity and the tolerations of
	// the pods (scheduling of the component block, else of the cluster).
	MutationSchedulingConstraints = "SchedulingConstraints"
	// MutationPodMetadata adds the user pod labels and annotations of the
	// cluster and the component block; a discovery label always wins.
	MutationPodMetadata = "PodMetadata"
	// MutationServiceAccount names the ServiceAccount of the cluster on the
	// pods when spec.serviceAccount is set.
	MutationServiceAccount = "ServiceAccount"
	// MutationVolumeRetention keeps the broker volumes on deletion when
	// spec.zeebe.persistentVolumeClaimRetentionPolicy.whenDeleted is Retain.
	MutationVolumeRetention = "VolumeRetention"
	// MutationTrustStore builds the JVM trust store of the process when the
	// secondary storage names a certificate authority, see
	// trustStoreMutation.
	MutationTrustStore = "TrustStore"
)

// workloadMutation is a mutation that applies to a StatefulSet and to a
// Deployment alike; the builders lift it into their own mutation type.
type workloadMutation = feature.Mutation[primitives.WorkloadMutator]

// workloadMutations returns the mutations of a process that a StatefulSet and
// a Deployment share: the override mutations, each gated on its field
// (Resources, SchedulingConstraints, PodMetadata, ServiceAccount), and
// TrustStore. Every builder registers them, so a process with no overrides
// and no private certificate authority renders the base workload only.
func workloadMutations(in Input, p Process) []workloadMutation {
	e := in.Effective
	workload := e.Workload(p.Component)

	scheduling := e.Scheduling
	if workload.Scheduling != nil {
		scheduling = workload.Scheduling
	}

	userLabels := labels.Merge(e.PodLabels, workload.PodLabels)
	userAnnotations := labels.Merge(e.PodAnnotations, workload.PodAnnotations)

	return []workloadMutation{
		{
			Name:    MutationResources,
			Feature: feature.NewBooleanGate(workload.Resources != nil),
			Mutate: func(m primitives.WorkloadMutator) error {
				m.EditContainers(selectors.ContainerNamed(containerName(p)), func(c *editors.ContainerEditor) error {
					c.SetResources(*workload.Resources)
					return nil
				})
				return nil
			},
		},
		{
			Name:    MutationSchedulingConstraints,
			Feature: feature.NewBooleanGate(scheduling != nil),
			Mutate: func(m primitives.WorkloadMutator) error {
				m.EditPodSpec(func(spec *editors.PodSpecEditor) error {
					if scheduling.NodeAffinity != nil || scheduling.PodAffinity != nil {
						spec.Raw().Affinity = &corev1.Affinity{
							NodeAffinity: scheduling.NodeAffinity,
							PodAffinity:  scheduling.PodAffinity,
						}
					}
					spec.Raw().Tolerations = scheduling.Tolerations
					return nil
				})
				return nil
			},
		},
		{
			Name:    MutationPodMetadata,
			Feature: feature.NewBooleanGate(len(userLabels) > 0 || len(userAnnotations) > 0),
			Mutate: func(m primitives.WorkloadMutator) error {
				m.EditPodTemplateMetadata(func(meta *editors.ObjectMetaEditor) error {
					// The discovery labels and the config hash of the base
					// template win over a user entry with the same key.
					meta.Raw().Labels = labels.Merge(userLabels, meta.Raw().Labels)
					meta.Raw().Annotations = labels.Merge(userAnnotations, meta.Raw().Annotations)
					return nil
				})
				return nil
			},
		},
		{
			Name:    MutationServiceAccount,
			Feature: feature.NewBooleanGate(usesServiceAccount(in)),
			Mutate: func(m primitives.WorkloadMutator) error {
				m.EditPodSpec(func(spec *editors.PodSpecEditor) error {
					spec.SetServiceAccountName(ServiceAccountName(in.Cluster, in.Effective))
					return nil
				})
				return nil
			},
		},
		trustStoreMutation(in, p),
	}
}

// statefulSetMutations returns the mutations of the broker StatefulSet: the
// workload mutations plus VolumeRetention.
func statefulSetMutations(in Input, p Process) []statefulset.Mutation {
	shared := workloadMutations(in, p)
	mutations := make([]statefulset.Mutation, 0, len(shared)+1)
	for _, m := range shared {
		mutations = append(mutations, statefulset.LiftMutation(m))
	}

	retain := in.Effective.VolumeRetention() == v1.RetainPersistentVolumeClaimRetentionPolicyType
	return append(mutations, statefulset.Mutation{
		Name:    MutationVolumeRetention,
		Feature: feature.NewBooleanGate(retain),
		Mutate: func(m *statefulset.Mutator) error {
			m.EditStatefulSetSpec(func(spec *editors.StatefulSetSpecEditor) error {
				spec.SetPersistentVolumeClaimRetentionPolicy(&appsv1.StatefulSetPersistentVolumeClaimRetentionPolicy{
					WhenDeleted: appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
					WhenScaled:  appsv1.RetainPersistentVolumeClaimRetentionPolicyType,
				})
				return nil
			})
			return nil
		},
	})
}

// deploymentMutations returns the workload mutations of a Deployment-backed
// process.
func deploymentMutations(in Input, p Process) []deployment.Mutation {
	shared := workloadMutations(in, p)
	mutations := make([]deployment.Mutation, 0, len(shared))
	for _, m := range shared {
		mutations = append(mutations, deployment.LiftMutation(m))
	}
	return mutations
}
