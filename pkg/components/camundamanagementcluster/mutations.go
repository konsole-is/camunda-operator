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
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/editors"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/selectors"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/deployment"
	corev1 "k8s.io/api/core/v1"

	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// The names of the mutations that layer the override surfaces of a
// WorkloadSpec onto the base Deployment of a component. They match the
// mutation names of the CamundaCluster and CamundaOptimize components.
const (
	// MutationResources sets the container resources (resources of the
	// component block).
	MutationResources = "Resources"
	// MutationSchedulingConstraints sets the affinity and the tolerations of
	// the pods (scheduling of the component block).
	MutationSchedulingConstraints = "SchedulingConstraints"
	// MutationPodMetadata adds the user pod labels and annotations of the
	// component block; a discovery label always wins.
	MutationPodMetadata = "PodMetadata"
	// MutationExtraEnv adds the extra environment variables of the component
	// block. An entry with the name of a rendered variable replaces it.
	MutationExtraEnv = "ExtraEnv"
	// MutationExtraEnvFrom adds the extra environment sources (ConfigMaps,
	// Secrets) of the component block.
	MutationExtraEnvFrom = "ExtraEnvFrom"
)

// workloadMutations returns the override mutations of a component, each gated
// on its field. Every Deployment registers all of them, so a component with no
// overrides renders the base workload only. container is the container that
// the resource and environment mutations edit.
func workloadMutations(in Input, component, container string) []deployment.Mutation {
	spec := in.workload(component)

	return []deployment.Mutation{
		{
			Name:    MutationResources,
			Feature: feature.NewBooleanGate(spec.Resources != nil),
			Mutate: func(m *deployment.Mutator) error {
				m.EditContainers(selectors.ContainerNamed(container), func(c *editors.ContainerEditor) error {
					c.SetResources(*spec.Resources)
					return nil
				})
				return nil
			},
		},
		{
			Name:    MutationSchedulingConstraints,
			Feature: feature.NewBooleanGate(spec.Scheduling != nil),
			Mutate: func(m *deployment.Mutator) error {
				m.EditPodSpec(func(pod *editors.PodSpecEditor) error {
					if spec.Scheduling.NodeAffinity != nil || spec.Scheduling.PodAffinity != nil {
						pod.Raw().Affinity = &corev1.Affinity{
							NodeAffinity: spec.Scheduling.NodeAffinity,
							PodAffinity:  spec.Scheduling.PodAffinity,
						}
					}
					pod.Raw().Tolerations = spec.Scheduling.Tolerations
					return nil
				})
				return nil
			},
		},
		{
			Name:    MutationPodMetadata,
			Feature: feature.NewBooleanGate(len(spec.PodLabels) > 0 || len(spec.PodAnnotations) > 0),
			Mutate: func(m *deployment.Mutator) error {
				m.EditPodTemplateMetadata(func(meta *editors.ObjectMetaEditor) error {
					// The discovery labels of the base template win over a
					// user entry with the same key.
					meta.Raw().Labels = labels.Merge(spec.PodLabels, meta.Raw().Labels)
					meta.Raw().Annotations = labels.Merge(spec.PodAnnotations, meta.Raw().Annotations)
					return nil
				})
				return nil
			},
		},
		{
			Name:    MutationExtraEnv,
			Feature: feature.NewBooleanGate(len(spec.ExtraEnv) > 0),
			Mutate: func(m *deployment.Mutator) error {
				m.EditContainers(selectors.ContainerNamed(container), func(c *editors.ContainerEditor) error {
					c.EnsureEnvVars(spec.ExtraEnv)
					return nil
				})
				return nil
			},
		},
		{
			Name:    MutationExtraEnvFrom,
			Feature: feature.NewBooleanGate(len(spec.ExtraEnvFrom) > 0),
			Mutate: func(m *deployment.Mutator) error {
				m.EditContainers(selectors.ContainerNamed(container), func(c *editors.ContainerEditor) error {
					c.Raw().EnvFrom = append(c.Raw().EnvFrom, spec.ExtraEnvFrom...)
					return nil
				})
				return nil
			},
		},
	}
}
