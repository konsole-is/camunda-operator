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

package claimpolicy

import (
	"cmp"
	"encoding/json"
	"fmt"
	"slices"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/planedatabases"
)

// The kinds of the resources whose fields a slot fills.
const (
	KindCamundaCluster           = "CamundaCluster"
	KindCamundaOptimize          = "CamundaOptimize"
	KindCamundaManagementCluster = "CamundaManagementCluster"
)

// The fields that a slot fills, as paths from the root of the object in the
// field path grammar of the fleet manager. The slot managementDatabases
// fills the field paths of planedatabases.All.
const (
	// PathExternalURL is a field of CamundaCluster and of CamundaOptimize.
	PathExternalURL                         = "spec.externalUrl"
	PathServiceAccountAnnotations           = "spec.serviceAccount.annotations"
	PathConnectorsServiceAccountAnnotations = "spec.connectors.serviceAccount.annotations"
	PathZeebeStorageClassName               = "spec.zeebe.storageClassName"
	PathBackupStorageRef                    = "spec.backupStorageRef"
	PathStorageRef                          = "spec.storageRef"
	PathKeycloakExternalURL                 = "spec.identityProvider.keycloak.externalUrl"
	PathIdentityExternalURL                 = "spec.identity.externalUrl"
	PathConsoleExternalURL                  = "spec.console.externalUrl"
	PathWebModelerExternalURL               = "spec.webModeler.externalUrl"
	PathWebModelerWebsocketsExternalURL     = "spec.webModeler.websocketsExternalUrl"
)

// FieldValue is one field that a slot fills.
type FieldValue struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
	// Section is the section whose selection fills the field.
	Section string `json:"section"`
	// Value is nil while the output is not ready. The claim controller then
	// keeps the live value of the field.
	Value *apiextensionsv1.JSON `json:"value,omitempty"`
}

// The output keys that the slots read. They are the keys of the core
// sections in internal/pluginstartup, which this package does not import.
const (
	outputServiceAccountAnnotations = "serviceAccountAnnotations"
	outputStorageClassName          = "storageClassName"
	outputObjectStorageConfig       = "objectStorageConfig"
	outputSecondaryStorageConfig    = "secondaryStorageConfig"
	outputExternalURL               = "externalUrl"
	outputOptimizeURL               = "optimizeUrl"
	outputManagementHost            = "managementHost"
)

// slotField is one field that a slot fills for one claim kind.
type slotField struct {
	claimKind pluginsv1alpha1.ClaimKind
	slot      pluginsv1alpha1.Slot
	kind      string
	path      string
	output    string
	// prefix and suffix turn a host output into a URL.
	prefix, suffix string
	// object marks an output that holds a JSON object.
	object bool
}

func (f slotField) value(output string) (*apiextensionsv1.JSON, error) {
	if f.object {
		var object map[string]json.RawMessage
		if json.Unmarshal([]byte(output), &object) != nil || object == nil {
			return nil, errNotAnObject
		}
		raw, err := json.Marshal(object)
		if err != nil {
			return nil, err
		}
		return &apiextensionsv1.JSON{Raw: raw}, nil
	}

	raw, err := json.Marshal(f.prefix + output + f.suffix)
	if err != nil {
		return nil, err
	}

	return &apiextensionsv1.JSON{Raw: raw}, nil
}

var (
	clusterClaim = pluginsv1alpha1.ClaimKindCamundaClusterClaim
	planeClaim   = pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim
)

// slotTable is the slot table of the contract: the fields that each slot
// fills, for each claim kind.
var slotTable = append([]slotField{
	{
		claimKind: clusterClaim, slot: pluginsv1alpha1.SlotWorkloadIdentity,
		kind: KindCamundaCluster, path: PathServiceAccountAnnotations,
		output: outputServiceAccountAnnotations, object: true,
	},
	{
		claimKind: clusterClaim, slot: pluginsv1alpha1.SlotConnectorsIdentity,
		kind: KindCamundaCluster, path: PathConnectorsServiceAccountAnnotations,
		output: outputServiceAccountAnnotations, object: true,
	},
	{
		claimKind: clusterClaim, slot: pluginsv1alpha1.SlotVolumeEncryption,
		kind: KindCamundaCluster, path: PathZeebeStorageClassName, output: outputStorageClassName,
	},
	{
		claimKind: clusterClaim, slot: pluginsv1alpha1.SlotBackupStorage,
		kind: KindCamundaCluster, path: PathBackupStorageRef, output: outputObjectStorageConfig,
	},
	{
		claimKind: clusterClaim, slot: pluginsv1alpha1.SlotSecondaryStorage,
		kind: KindCamundaCluster, path: PathStorageRef, output: outputSecondaryStorageConfig,
	},
	{
		claimKind: clusterClaim, slot: pluginsv1alpha1.SlotExternalAccess,
		kind: KindCamundaCluster, path: PathExternalURL, output: outputExternalURL,
	},
	{
		claimKind: clusterClaim, slot: pluginsv1alpha1.SlotExternalAccess,
		kind: KindCamundaOptimize, path: PathExternalURL, output: outputOptimizeURL,
	},
	{
		claimKind: planeClaim, slot: pluginsv1alpha1.SlotExternalAccess,
		kind: KindCamundaManagementCluster, path: PathKeycloakExternalURL,
		output: outputManagementHost, prefix: "https://", suffix: "/auth",
	},
	{
		claimKind: planeClaim, slot: pluginsv1alpha1.SlotExternalAccess,
		kind: KindCamundaManagementCluster, path: PathIdentityExternalURL,
		output: outputManagementHost, prefix: "https://", suffix: "/identity",
	},
	{
		claimKind: planeClaim, slot: pluginsv1alpha1.SlotExternalAccess,
		kind: KindCamundaManagementCluster, path: PathConsoleExternalURL,
		output: outputManagementHost, prefix: "https://", suffix: "/console",
	},
	{
		claimKind: planeClaim, slot: pluginsv1alpha1.SlotExternalAccess,
		kind: KindCamundaManagementCluster, path: PathWebModelerExternalURL,
		output: outputManagementHost, prefix: "https://", suffix: "/modeler",
	},
	{
		claimKind: planeClaim, slot: pluginsv1alpha1.SlotExternalAccess,
		kind: KindCamundaManagementCluster, path: PathWebModelerWebsocketsExternalURL,
		output: outputManagementHost, prefix: "https://", suffix: "/modeler-ws",
	},
}, managementDatabaseFields()...)

func managementDatabaseFields() []slotField {
	fields := make([]slotField, 0, len(planedatabases.All))
	for _, d := range planedatabases.All {
		fields = append(fields, slotField{
			claimKind: planeClaim, slot: pluginsv1alpha1.SlotManagementDatabases,
			kind: KindCamundaManagementCluster, path: d.FieldPath, output: d.Output,
		})
	}

	return fields
}

// SlotFields returns the fields of the generated resources of a claim of
// kind that its selections fill through the slots of their sections, sorted
// by Kind and then by Path. A section without a slot fills nothing. A field
// has a Value only while the request of its selection reports
// OutputsReady=True for its current generation and has the output. The
// externalAccess slot of a plane fills five URLs, https://<managementHost>
// with a fixed path each.
//
// A selection in a section that does not exist is refused with
// InvalidReference. An output serviceAccountAnnotations that is not a JSON
// object is an error, also when it is empty.
func SlotFields(
	kind pluginsv1alpha1.ClaimKind,
	sections []pluginsv1alpha1.PluginSection,
	selections []fleetv1alpha1.PluginSelection,
	requests []pluginsv1alpha1.PluginRequest,
) ([]FieldValue, error) {
	fields := []FieldValue{}
	for _, s := range selections {
		section, err := sectionOf(s.Section, sections)
		if err != nil {
			return nil, err
		}

		outputs := readyOutputs(requests, s.Section, s.Registration)
		for _, row := range slotTable {
			if row.claimKind != kind || row.slot != section.Spec.Slot {
				continue
			}

			field := FieldValue{Kind: row.kind, Path: row.path, Section: s.Section}
			if output, ok := outputs[row.output]; ok {
				if field.Value, err = row.value(output); err != nil {
					return nil, fmt.Errorf("output %s of section %q: %w", row.output, s.Section, err)
				}
			}
			fields = append(fields, field)
		}
	}

	slices.SortFunc(fields, func(a, b FieldValue) int {
		return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Path, b.Path))
	})

	return fields, nil
}

func sectionOf(name string, sections []pluginsv1alpha1.PluginSection) (*pluginsv1alpha1.PluginSection, error) {
	for i := range sections {
		if sections[i].Name == name {
			return &sections[i], nil
		}
	}

	return nil, refuse(v1.ReasonInvalidReference, "PluginSection %q does not exist", name)
}

// TargetServiceAccount returns the ServiceAccount that a request of claim of
// kind in section targets. A request of a CamundaManagementClusterClaim
// targets <claim>-management, the account of the plane. A request of a
// CamundaClusterClaim targets <claim>-connectors for a section with the slot
// connectorsIdentity, and <claim>-camunda for every other section, also one
// without a slot.
func TargetServiceAccount(kind pluginsv1alpha1.ClaimKind, claim string, section *pluginsv1alpha1.PluginSection) string {
	if kind == pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim {
		return claim + "-management"
	}
	if section != nil && section.Spec.Slot == pluginsv1alpha1.SlotConnectorsIdentity {
		return claim + "-connectors"
	}

	return claim + "-camunda"
}
