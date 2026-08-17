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
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
)

// managementBinding returns the binding that the cluster publishes in
// status.management, or nil while the cluster is suspended: every workload is
// then scaled to zero, so the endpoint would answer nothing and a consumer
// that trusted it would report a connection failure instead of a suspended
// cluster.
//
// Camunda 8.9 serves the actuator endpoints of the management port without
// authentication, so the method is none. Protecting the port is a network
// policy, not a credential.
func managementBinding(cluster *v1.CamundaCluster, in components.Input) *v1.ManagementBinding {
	if in.Effective.Suspend {
		return nil
	}

	binding := &v1.ManagementBinding{
		Endpoint:   components.ManagementEndpoint(cluster, in.Effective),
		Auth:       v1.ManagementAuth{Method: v1.ManagementAuthMethodNone},
		Version:    in.Effective.Version,
		Partitions: in.Effective.Partitions(),
	}

	if in.Storage.Type == v1.SecondaryStorageTypeElasticsearch && in.Storage.Elasticsearch != nil {
		binding.BackupRepository = in.Storage.Elasticsearch.SnapshotRepository
	}

	return binding
}
