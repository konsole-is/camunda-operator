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
	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// Overrides are the fields of a claim that a selected slot can fill. A
// cluster claim sets Cluster and Optimize, and a plane claim sets
// Management.
type Overrides struct {
	Cluster    *v1.CamundaClusterSpec             `json:"cluster,omitempty"`
	Optimize   *fleetv1alpha1.OptimizeAttachment  `json:"optimize,omitempty"`
	Management *fleetv1alpha1.ManagementOverrides `json:"management,omitempty"`
}

// slotConflict is a field of a claim that a slot fills.
type slotConflict struct {
	slot pluginsv1alpha1.Slot
	path string
	// isSet reports whether the claim sets the field: a string that is not
	// empty, or a map with a key.
	isSet func(Overrides) bool
}

// slotConflicts are the fields of a claim that a slot fills. storageRef and
// backupStorageRef have no row, because the API server refuses them in
// spec.cluster.
var slotConflicts = []slotConflict{
	{pluginsv1alpha1.SlotExternalAccess, "spec.cluster.externalUrl", func(o Overrides) bool {
		return o.Cluster != nil && o.Cluster.ExternalURL != ""
	}},
	{pluginsv1alpha1.SlotExternalAccess, "spec.attachments.optimize.externalUrl", func(o Overrides) bool {
		return o.Optimize != nil && o.Optimize.ExternalURL != ""
	}},
	{pluginsv1alpha1.SlotExternalAccess, "spec.management.identityProvider.keycloak.externalUrl", func(o Overrides) bool {
		return o.Management != nil && o.Management.IdentityProvider.Keycloak != nil &&
			o.Management.IdentityProvider.Keycloak.ExternalURL != ""
	}},
	{pluginsv1alpha1.SlotExternalAccess, "spec.management.identity.externalUrl", func(o Overrides) bool {
		return o.Management != nil && o.Management.Identity.ExternalURL != ""
	}},
	{pluginsv1alpha1.SlotExternalAccess, "spec.management.console.externalUrl", func(o Overrides) bool {
		return o.Management != nil && o.Management.Console != nil && o.Management.Console.ExternalURL != ""
	}},
	{pluginsv1alpha1.SlotExternalAccess, "spec.management.webModeler.externalUrl", func(o Overrides) bool {
		return o.Management != nil && o.Management.WebModeler != nil && o.Management.WebModeler.ExternalURL != ""
	}},
	{pluginsv1alpha1.SlotExternalAccess, "spec.management.webModeler.websocketsExternalUrl", func(o Overrides) bool {
		return o.Management != nil && o.Management.WebModeler != nil &&
			o.Management.WebModeler.WebsocketsExternalURL != ""
	}},
	{pluginsv1alpha1.SlotWorkloadIdentity, "spec.cluster.serviceAccount.annotations", func(o Overrides) bool {
		return o.Cluster != nil && o.Cluster.ServiceAccount != nil && len(o.Cluster.ServiceAccount.Annotations) > 0
	}},
	{pluginsv1alpha1.SlotConnectorsIdentity, "spec.cluster.connectors.serviceAccount.annotations", func(o Overrides) bool {
		return o.Cluster != nil && o.Cluster.Connectors != nil && o.Cluster.Connectors.ServiceAccount != nil &&
			len(o.Cluster.Connectors.ServiceAccount.Annotations) > 0
	}},
	{pluginsv1alpha1.SlotVolumeEncryption, "spec.cluster.zeebe.storageClassName", func(o Overrides) bool {
		return o.Cluster != nil && o.Cluster.Zeebe != nil && o.Cluster.Zeebe.StorageClassName != nil &&
			*o.Cluster.Zeebe.StorageClassName != ""
	}},
}

// CheckSlotConflicts refuses with SlotConflict the first field of overrides
// that the slot of a selected section fills, in claim order. A selection in
// a section that does not exist is refused with InvalidReference.
func CheckSlotConflicts(
	sections []pluginsv1alpha1.PluginSection,
	selections []fleetv1alpha1.PluginSelection,
	overrides Overrides,
) error {
	for _, s := range selections {
		section, err := sectionOf(s.Section, sections)
		if err != nil {
			return err
		}

		for _, c := range slotConflicts {
			if c.slot == section.Spec.Slot && c.isSet(overrides) {
				return refuse(
					fleetv1alpha1.ReasonSlotConflict,
					"%s is filled by section %q, so the claim cannot set it",
					c.path,
					s.Section,
				)
			}
		}
	}

	return nil
}
