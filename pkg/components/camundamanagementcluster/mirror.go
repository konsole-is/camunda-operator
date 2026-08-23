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

// MirrorPurpose names one mirrored Secret of a CamundaManagementCluster. A pod
// can only reference a Secret in its own namespace, so the controller copies
// every referenced Secret that lives elsewhere into the management namespace,
// one copy per purpose. The copies are the ones that MirrorPurposes lists; a
// purpose outside that set gets a name and no Secret, so every constant of
// this type must appear there.
type MirrorPurpose string

// The purposes of the mirrored Secrets. The license and the identity provider
// clients are nearly always mirrored ones: CamundaPlatformConfig is
// cluster-scoped, so one platform config serves every namespace and its
// references can name only one.
const (
	// MirrorPurposeLicense names the copy of the Camunda license Secret.
	MirrorPurposeLicense MirrorPurpose = "license"
	// MirrorPurposeIdentityClient names the copy of the Management Identity OIDC client secret.
	MirrorPurposeIdentityClient MirrorPurpose = "identity-client"
	// MirrorPurposeIdentityDB names the copy of the Management Identity database credentials Secret.
	MirrorPurposeIdentityDB MirrorPurpose = "identity-db"
	// MirrorPurposeIdentityAdmin names the copy of the Secret with the password of the first administrator.
	MirrorPurposeIdentityAdmin MirrorPurpose = "identity-admin"
	// MirrorPurposeKeycloakAdmin names the copy of the administrator credentials of an external Keycloak.
	MirrorPurposeKeycloakAdmin MirrorPurpose = "keycloak-admin"
	// MirrorPurposeKeycloakDB names the copy of the Keycloak database credentials Secret.
	MirrorPurposeKeycloakDB MirrorPurpose = "keycloak-db"
	// MirrorPurposeWebModelerDB names the copy of the Web Modeler database credentials Secret.
	MirrorPurposeWebModelerDB MirrorPurpose = "web-modeler-db"
	// MirrorPurposeWebModelerMail names the copy of the Web Modeler SMTP credentials Secret.
	MirrorPurposeWebModelerMail MirrorPurpose = "web-modeler-mail"
)

// MirrorPurposes is the closed set of purposes that the component renders, in
// the order it renders their Secrets.
var MirrorPurposes = []MirrorPurpose{
	MirrorPurposeLicense,
	MirrorPurposeIdentityClient,
	MirrorPurposeIdentityDB,
	MirrorPurposeIdentityAdmin,
	MirrorPurposeKeycloakAdmin,
	MirrorPurposeKeycloakDB,
	MirrorPurposeWebModelerDB,
	MirrorPurposeWebModelerMail,
}

// MirroredSecretName returns the name of the copy of a referenced Secret in
// the management namespace: <name>-management-<purpose>.
func MirroredSecretName(mc *v1.CamundaManagementCluster, purpose MirrorPurpose) string {
	suffix := "-management-" + string(purpose)

	return labels.BoundedName(mc.Name, validation.DNS1123LabelMaxLength-len(suffix)) + suffix
}

// LocalSecretName returns the name under which a Secret at namespace/name is
// reachable from a pod of the management plane. A Secret of the management
// namespace keeps its own name; one from any other namespace resolves to its
// purpose-named copy.
func LocalSecretName(mc *v1.CamundaManagementCluster, namespace, name string, purpose MirrorPurpose) string {
	if namespace == mc.Namespace {
		return name
	}

	return MirroredSecretName(mc, purpose)
}

// Valid reports whether p is in MirrorPurposes, the set the component
// renders.
func (p MirrorPurpose) Valid() bool {
	return slices.Contains(MirrorPurposes, p)
}

// mirroredSecretComponent renders one Secret per purpose of MirrorPurposes in
// the management namespace, in one component. The keys of in.Mirrors are the
// purposes that are present, the values the copied data. The Secret of an
// absent purpose is gated off, so a reference that moved into the management
// namespace or went away deletes its copy. The component is gated on any
// purpose being present: without one it reads Disabled and stays out of Ready.
// A purpose that is not in MirrorPurposes is an error, because nothing would
// render its copy.
func mirroredSecretComponent(in Input) (Built, error) {
	for purpose := range in.Mirrors {
		if !purpose.Valid() {
			return Built{}, fmt.Errorf(
				"building %s component: purpose %q is not in MirrorPurposes", ComponentMirroredSecrets, purpose,
			)
		}
	}

	builder := component.NewComponentBuilder().
		WithName(ComponentMirroredSecrets).
		WithConditionType(component.ConditionType(v1.ConditionMirroredSecretsReady)).
		WithFeatureGate(feature.NewBooleanGate(len(in.Mirrors) > 0))

	for _, purpose := range MirrorPurposes {
		data, present := in.Mirrors[purpose]
		mirrored, err := secret.NewBuilder(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      MirroredSecretName(in.Cluster, purpose),
				Namespace: in.Cluster.Namespace,
				Labels:    managedLabels(in, ComponentMirroredSecrets),
			},
			Type: corev1.SecretTypeOpaque,
			Data: data,
		}).Build()
		if err != nil {
			return Built{}, fmt.Errorf("building %s component: %w", ComponentMirroredSecrets, err)
		}
		builder = builder.WithResource(mirrored, component.GatedBy(feature.NewBooleanGate(present)))
	}

	comp, err := builder.Build()
	if err != nil {
		return Built{}, fmt.Errorf("building %s component: %w", ComponentMirroredSecrets, err)
	}

	built := Built{Components: []*component.Component{comp}}
	if len(in.Mirrors) > 0 {
		built.Ready = built.Components
	}

	return built, nil
}
