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

package camundaclusterclaim

import (
	"encoding/json"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/camundacluster"
)

// clusterComponentName is the name of the cluster component. The field
// manager of the CamundaCluster is CamundaClusterClaim/cluster/<claim UID>.
const clusterComponentName = "cluster"

// slotSetter sets the CamundaCluster field of a slot path in spec from the
// JSON value of the output. Without a value, it copies the field from live,
// and leaves it unset when live is nil.
type slotSetter func(spec, live *v1.CamundaClusterSpec, value *apiextensionsv1.JSON) error

// slotSetters holds the setter of each slot path of kind CamundaCluster.
var slotSetters = map[string]slotSetter{
	claimpolicy.PathExternalURL: setter(func(s *v1.CamundaClusterSpec) *string { return &s.ExternalURL }),
	claimpolicy.PathServiceAccountAnnotations: setter(func(s *v1.CamundaClusterSpec) *map[string]string {
		return &serviceAccount(&s.ServiceAccount).Annotations
	}),
	claimpolicy.PathConnectorsServiceAccountAnnotations: setter(func(s *v1.CamundaClusterSpec) *map[string]string {
		return &connectorsServiceAccount(s).Annotations
	}),
	claimpolicy.PathZeebeStorageClassName: setter(func(s *v1.CamundaClusterSpec) **string {
		if s.Zeebe == nil {
			s.Zeebe = &v1.ZeebeSpec{}
		}
		return &s.Zeebe.StorageClassName
	}),
	claimpolicy.PathBackupStorageRef: setter(func(s *v1.CamundaClusterSpec) *string { return &s.BackupStorageRef }),
	claimpolicy.PathStorageRef:       setter(func(s *v1.CamundaClusterSpec) *string { return &s.StorageRef }),
}

// clusterComponent builds the component of the CamundaCluster of the claim.
// It waits for RequestsReady before the first apply.
func clusterComponent(in Input) (*component.Component, error) {
	desired, waiting, err := cluster(in)
	if err != nil {
		return nil, err
	}

	resource, err := camundacluster.NewBuilder(desired).WithGuard(createWithOutputs(in.Live, waiting)).Build()
	if err != nil {
		return nil, fmt.Errorf("building the CamundaCluster %s: %w", desired.Name, err)
	}

	return component.NewComponentBuilder().
		WithName(clusterComponentName).
		WithConditionType(component.ConditionType(fleetv1alpha1.ConditionClusterReady)).
		WithPrerequisite(component.DependsOn(component.ConditionType(fleetv1alpha1.ConditionRequestsReady))).
		WithResource(resource, component.BlockOnForeignController()).
		Build()
}

// createWithOutputs returns the guard of the CamundaCluster. It holds back
// the first apply while a slot field has no output in waiting, because
// RequestsReady can read newer outputs than the ones that the cluster was
// rendered from, and some slot fields cannot change after creation.
func createWithOutputs(
	live *v1.CamundaCluster,
	waiting []string,
) func(v1.CamundaCluster) (concepts.GuardStatusWithReason, error) {
	status := concepts.GuardStatusWithReason{Status: concepts.GuardStatusUnblocked}
	if live == nil && len(waiting) > 0 {
		status = concepts.GuardStatusWithReason{
			Status: concepts.GuardStatusBlocked,
			Reason: "waiting for the outputs of " + strings.Join(waiting, ", "),
		}
	}

	return func(v1.CamundaCluster) (concepts.GuardStatusWithReason, error) {
		return status, nil
	}
}

// cluster returns the CamundaCluster of the claim: spec.cluster of the
// claim, the references and ServiceAccount names that the claim controller
// sets, and the slot fields from the outputs. waiting holds the sections,
// sorted and distinct, whose output of a CamundaCluster slot field is
// missing.
func cluster(in Input) (desired *v1.CamundaCluster, waiting []string, err error) {
	spec := v1.CamundaClusterSpec{}
	if in.Claim.Spec.Cluster != nil {
		spec = *in.Claim.Spec.Cluster.DeepCopy()
	}
	spec.PresetRef = in.Claim.Spec.PresetRef
	spec.ReleaseRef = in.ReleaseRef
	spec.PlatformConfigRef = in.PlatformConfigRef
	serviceAccount(&spec.ServiceAccount).Name = ServiceAccountName(in.Claim.Name)
	connectorsServiceAccount(&spec).Name = ConnectorsServiceAccountName(in.Claim.Name)

	fields, err := claimpolicy.SlotFields(
		pluginsv1alpha1.ClaimKindCamundaClusterClaim, in.Sections, in.Claim.Spec.Plugins, in.Requests,
	)
	if err != nil {
		return nil, nil, err
	}

	// The setters fill the intermediate structs of the copy, never of the
	// live object of the precheck.
	var live *v1.CamundaClusterSpec
	if in.Live != nil {
		live = in.Live.Spec.DeepCopy()
	}

	for _, f := range fields {
		// The CamundaOptimize field of the access slot belongs to the
		// attachments. TODO(#605): write it there.
		if f.Kind != claimpolicy.KindCamundaCluster {
			continue
		}

		set, ok := slotSetters[f.Path]
		if !ok {
			return nil, nil, fmt.Errorf("no CamundaCluster field for the slot path %s of section %q", f.Path, f.Section)
		}
		if err := set(&spec, live, f.Value); err != nil {
			return nil, nil, fmt.Errorf("setting %s from the outputs of section %q: %w", f.Path, f.Section, err)
		}
		if f.Value == nil {
			waiting = append(waiting, f.Section)
		}
	}
	slices.Sort(waiting)

	return &v1.CamundaCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaCluster"},
		ObjectMeta: metav1.ObjectMeta{
			Name:      in.Claim.Name,
			Namespace: in.Claim.Namespace,
			Labels:    map[string]string{fleetv1alpha1.LabelClaim: in.Claim.Name},
		},
		Spec: spec,
	}, slices.Compact(waiting), nil
}

// serviceAccount returns *account, and sets it to an empty ServiceAccountSpec
// first when it is nil.
func serviceAccount(account **v1.ServiceAccountSpec) *v1.ServiceAccountSpec {
	if *account == nil {
		*account = &v1.ServiceAccountSpec{}
	}

	return *account
}

// connectorsServiceAccount returns connectors.serviceAccount of spec, and
// sets the missing structs on the way.
func connectorsServiceAccount(spec *v1.CamundaClusterSpec) *v1.ServiceAccountSpec {
	if spec.Connectors == nil {
		spec.Connectors = &v1.ConnectorsSpec{}
	}

	return serviceAccount(&spec.Connectors.ServiceAccount)
}

// setter returns the slotSetter of the field that field returns. field may
// set the missing structs on the way to the field.
func setter[T any](field func(*v1.CamundaClusterSpec) *T) slotSetter {
	return func(spec, live *v1.CamundaClusterSpec, value *apiextensionsv1.JSON) error {
		if value == nil {
			if live == nil {
				return nil
			}
			if kept := *field(live); !reflect.ValueOf(&kept).Elem().IsZero() {
				*field(spec) = kept
			}
			return nil
		}

		var decoded T
		if err := json.Unmarshal(value.Raw, &decoded); err != nil {
			return err
		}
		*field(spec) = decoded

		return nil
	}
}
