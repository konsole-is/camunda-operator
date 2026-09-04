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
	"fmt"
	"slices"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/secret"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// MirrorPurpose names one mirrored Secret of a CamundaOptimize. A pod can
// only reference a Secret in its own namespace, so the controller copies
// every referenced Secret that lives elsewhere into the CamundaOptimize
// namespace, one copy per purpose. The copies are the ones that
// MirrorPurposes lists; a purpose outside that set gets a name and no
// Secret, so every constant of this type must appear there.
type MirrorPurpose string

// The purposes of the mirrored Secrets. The Management Identity client secret
// is nearly always a mirrored one: ManagementAuthConfig is cluster-scoped, so
// one contract serves every namespace and its reference can name only one.
const (
	MirrorPurposeLicense    MirrorPurpose = "license"
	MirrorPurposeAuthClient MirrorPurpose = "auth-client"
)

// MirrorPurposes is the closed set of purposes that the component renders, in
// the order it renders their Secrets.
var MirrorPurposes = []MirrorPurpose{
	MirrorPurposeLicense,
	MirrorPurposeAuthClient,
}

// mirroredComponentName is the ocf name of the mirrored Secrets component.
const mirroredComponentName = "optimize-secrets"

// MirroredSecretComponent renders one Secret per purpose of MirrorPurposes in
// the CamundaOptimize namespace, in one component. The keys of mirrors are the
// purposes that are present, the values the copied data (only the keys that
// the reference names). The Secret of an absent purpose is gated off, so a
// reference that moved into the CamundaOptimize namespace or went away deletes
// its copy. The component is gated on any purpose being present: without one
// it reads Disabled and stays out of Ready. A purpose that is not in
// MirrorPurposes is an error, because nothing would render its copy.
func MirroredSecretComponent(
	o *v1.CamundaOptimize,
	mirrors map[MirrorPurpose]map[string][]byte,
) (*component.Component, error) {
	for purpose := range mirrors {
		if !purpose.Valid() {
			return nil, fmt.Errorf(
				"building %s component: purpose %q is not in MirrorPurposes", mirroredComponentName, purpose,
			)
		}
	}

	builder := component.NewComponentBuilder().
		WithName(mirroredComponentName).
		WithConditionType(component.ConditionType(v1.ConditionMirroredSecretsReady)).
		WithFeatureGate(feature.NewBooleanGate(len(mirrors) > 0))

	for _, purpose := range MirrorPurposes {
		data, present := mirrors[purpose]
		mirrored, err := secret.NewBuilder(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      MirroredSecretName(o, purpose),
				Namespace: o.Namespace,
				Labels:    labels.Managed(labels.Cluster(o.Spec.ClusterRef.Name), mirroredComponentName),
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}).Build()
		if err != nil {
			return nil, fmt.Errorf("building %s component: %w", mirroredComponentName, err)
		}
		builder = builder.WithResource(mirrored, component.GatedBy(feature.NewBooleanGate(present)))
	}

	return builder.Build()
}

// MirroredSecretName returns the name of the copy of a referenced Secret in
// the CamundaOptimize namespace: <name>-optimize-<purpose>. A long
// CamundaOptimize name truncates, the way WorkloadName truncates.
func MirroredSecretName(o *v1.CamundaOptimize, purpose MirrorPurpose) string {
	suffix := "-optimize-" + string(purpose)

	return labels.BoundedName(o.Name, validation.DNS1123LabelMaxLength-len(suffix)) + suffix
}

// Valid reports whether p is in MirrorPurposes, the set the component
// renders.
func (p MirrorPurpose) Valid() bool {
	return slices.Contains(MirrorPurposes, p)
}
