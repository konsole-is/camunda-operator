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
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// The documented defaults of the fields that the schema leaves open, so a
// preset can set them. They apply after the preset merge.
const (
	defaultReplicas          int32 = 1
	defaultPartitions        int32 = 1
	defaultReplicationFactor int32 = 1
	defaultStorageSize             = "10Gi"
)

// Effective is the merged spec with the documented defaults applied. Every
// reader of the topology uses it, so a default lives in exactly one place.
type Effective struct {
	v1.CamundaClusterSpec
}

// NewEffective wraps a preset-merged spec. It copies nothing: the accessors
// read the spec on every call.
func NewEffective(merged v1.CamundaClusterSpec) Effective {
	return Effective{CamundaClusterSpec: merged}
}

// ZeebeReplicas returns the number of brokers. Defaults to 1.
func (e Effective) ZeebeReplicas() int32 { return e.Replicas(ComponentZeebe) }

// Partitions returns the partition count. Defaults to 1.
func (e Effective) Partitions() int32 {
	if e.Zeebe == nil || e.Zeebe.Partitions == nil {
		return defaultPartitions
	}
	return *e.Zeebe.Partitions
}

// ReplicationFactor returns the replication factor. Defaults to 1.
func (e Effective) ReplicationFactor() int32 {
	if e.Zeebe == nil || e.Zeebe.ReplicationFactor == nil {
		return defaultReplicationFactor
	}
	return *e.Zeebe.ReplicationFactor
}

// StorageSize returns the size of the data volume of each broker. Defaults
// to 10Gi.
func (e Effective) StorageSize() resource.Quantity {
	if e.Zeebe == nil || e.Zeebe.StorageSize == nil {
		return resource.MustParse(defaultStorageSize)
	}
	return *e.Zeebe.StorageSize
}

// VolumeRetention returns what happens to the broker volumes when the
// cluster is deleted. Defaults to Delete.
func (e Effective) VolumeRetention() v1.PersistentVolumeClaimRetentionPolicyType {
	if e.Zeebe == nil || e.Zeebe.PersistentVolumeClaimRetentionPolicy == nil ||
		e.Zeebe.PersistentVolumeClaimRetentionPolicy.WhenDeleted == "" {
		return v1.DeletePersistentVolumeClaimRetentionPolicyType
	}
	return e.Zeebe.PersistentVolumeClaimRetentionPolicy.WhenDeleted
}

// GatewayMode returns where the gateway runs. Defaults to Standalone.
func (e Effective) GatewayMode() v1.ComponentMode {
	if e.Gateway == nil || e.Gateway.Mode == "" {
		return v1.ComponentModeStandalone
	}
	return e.Gateway.Mode
}

// OperateMode returns where Operate runs. Defaults to Embedded.
func (e Effective) OperateMode() v1.ComponentMode { return webAppMode(e.Operate) }

// TasklistMode returns where Tasklist runs. Defaults to Embedded.
func (e Effective) TasklistMode() v1.ComponentMode { return webAppMode(e.Tasklist) }

// IdentityMode returns where Identity runs. Defaults to Embedded.
func (e Effective) IdentityMode() v1.ComponentMode { return webAppMode(e.Identity) }

func webAppMode(app *v1.WebAppSpec) v1.ComponentMode {
	if app == nil || app.Mode == "" {
		return v1.ComponentModeEmbedded
	}
	return app.Mode
}

// ConnectorsEnabled reports whether the connectors runtime runs. Defaults to
// false.
func (e Effective) ConnectorsEnabled() bool {
	return e.Connectors != nil && e.Connectors.Enabled != nil && *e.Connectors.Enabled
}

// Replicas returns the number of pods of the named component, one of the
// Component constants. Defaults to 1 when the component block or its replicas
// field is unset. An unknown component name returns 0.
func (e Effective) Replicas(component string) int32 {
	workload, known := e.workload(component)
	if !known {
		return 0
	}

	if workload.Replicas == nil {
		return defaultReplicas
	}
	return *workload.Replicas
}

// Workload returns the per-component block of the named component, one of
// the Component constants. It returns the zero value when the block is unset
// or the name is unknown.
func (e Effective) Workload(component string) v1.WorkloadSpec {
	workload, _ := e.workload(component)
	return workload
}

// workload reports false for a name that is not a component of the cluster,
// so Replicas can tell an unknown name from an unset block.
func (e Effective) workload(component string) (v1.WorkloadSpec, bool) {
	switch component {
	case ComponentZeebe:
		if e.Zeebe != nil {
			return e.Zeebe.WorkloadSpec, true
		}
	case ComponentGateway:
		if e.Gateway != nil {
			return e.Gateway.WorkloadSpec, true
		}
	case ComponentOperate:
		if e.Operate != nil {
			return e.Operate.WorkloadSpec, true
		}
	case ComponentTasklist:
		if e.Tasklist != nil {
			return e.Tasklist.WorkloadSpec, true
		}
	case ComponentIdentity:
		if e.Identity != nil {
			return e.Identity.WorkloadSpec, true
		}
	case ComponentConnectors:
		if e.Connectors != nil {
			return e.Connectors.WorkloadSpec, true
		}
	default:
		return v1.WorkloadSpec{}, false
	}

	return v1.WorkloadSpec{}, true
}
