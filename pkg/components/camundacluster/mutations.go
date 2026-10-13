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
	"github.com/konsole-is/camunda-operator/pkg/accounthash"
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
	// MutationServiceAccount names the ServiceAccount of the pods: the
	// account of the cluster, or the account of the connectors for the
	// connectors process.
	MutationServiceAccount = "ServiceAccount"
	// MutationServiceAccountHash puts ServiceAccountHashAnnotation on the pods
	// when the operator renders annotations on their ServiceAccount, so that
	// a change of those annotations restarts the pods.
	MutationServiceAccountHash = "ServiceAccountHash"
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

// workloadMutations returns the mutations of a process that a StatefulSet and
// a Deployment share: the override mutations, each gated on its field
// (Resources, SchedulingConstraints, PodMetadata, ServiceAccount),
// ServiceAccountHash, and TrustStore. Every builder registers them, so a
// process with no overrides, no annotations on a rendered ServiceAccount, and
// no private certificate authority renders the base workload only.
func workloadMutations(in Input, p Process) []workloadMutation {
	e := in.Effective
	workload := e.Workload(p.Component)

	scheduling := e.Scheduling
	if workload.Scheduling != nil {
		scheduling = workload.Scheduling
	}

	userLabels := labels.Merge(e.PodLabels, workload.PodLabels)
	userAnnotations := labels.Merge(e.PodAnnotations, workload.PodAnnotations)
	// A user value would keep an old hash on the template when the
	// ServiceAccountHash mutation is off, and the pods would not roll.
	delete(userAnnotations, ServiceAccountHashAnnotation)
	account := podServiceAccountName(in, p)
	accountHash := serviceAccountHash(in, p)

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
			Feature: feature.NewBooleanGate(account != ""),
			Mutate: func(m primitives.WorkloadMutator) error {
				m.EditPodSpec(func(spec *editors.PodSpecEditor) error {
					spec.SetServiceAccountName(account)
					return nil
				})
				return nil
			},
		},
		{
			Name:    MutationServiceAccountHash,
			Feature: feature.NewBooleanGate(accountHash != ""),
			Mutate: func(m primitives.WorkloadMutator) error {
				m.EditPodTemplateMetadata(func(meta *editors.ObjectMetaEditor) error {
					meta.EnsureAnnotation(ServiceAccountHashAnnotation, accountHash)
					return nil
				})
				return nil
			},
		},
		trustStoreMutation(in, p),
	}
}

// podServiceAccountName returns the ServiceAccount that the pods of p run
// under, or the empty string for the default account of the namespace.
func podServiceAccountName(in Input, p Process) string {
	if p.Component == ComponentConnectors {
		return PodConnectorsServiceAccountName(in)
	}

	return PodServiceAccountName(in)
}

// serviceAccountHash returns the hash of the annotations that the operator
// renders on the ServiceAccount of the pods of p, or the empty string when it
// renders none there. An account with create false is the user's, so a change
// to it is not the operator's to roll.
func serviceAccountHash(in Input, p Process) string {
	if p.Component == ComponentConnectors {
		if !rendersConnectorsServiceAccount(in) {
			return ""
		}
		return accounthash.Of(connectorsAccountAnnotations(in))
	}

	if !rendersServiceAccount(in) {
		return ""
	}
	return accounthash.Of(clusterAccountAnnotations(in))
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
