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
)

// The component values of the camunda.io/component label. Each one is also
// the name suffix of the workload and Service of that process.
const (
	ComponentZeebe      = "zeebe"
	ComponentGateway    = "gateway"
	ComponentOperate    = "operate"
	ComponentTasklist   = "tasklist"
	ComponentIdentity   = "identity"
	ComponentConnectors = "connectors"
)

// The names that a user or another operator can observe on the rendered
// resources.
const (
	// ConfigHashAnnotation is the pod template annotation that carries the
	// hash of the rendered configuration. A change to it rolls the pods.
	ConfigHashAnnotation = "camunda.io/config-hash"
	// AdminUsername is the initial admin user of a basic-auth cluster.
	AdminUsername = "admin"
	// AdminUsernameKey and AdminPasswordKey are the keys of the admin Secret.
	AdminUsernameKey = "username"
	AdminPasswordKey = "password"
	// DataVolumeName is the volume claim template of the brokers.
	DataVolumeName = "data"
	// DataMountPath is where the brokers keep their data. It is the
	// camunda.data.primary-storage.directory default "data" under
	// CAMUNDA_HOME (PrimaryStorage.java:24, camunda.Dockerfile:122-131).
	DataMountPath = "/usr/local/camunda/data"
	// CAMountPath is where the Elasticsearch CA certificate is mounted.
	CAMountPath = "/etc/camunda/es-ca"
	// CamundaImage is the image of every unified process
	// (camunda.Dockerfile).
	CamundaImage = "camunda/camunda"
	// ConnectorsImage is the image of the connectors runtime (helm chart
	// 14.8.3 values.yaml:2289).
	ConnectorsImage = "camunda/connectors-bundle"
	// CamundaEntrypoint is the entrypoint of the unified image
	// (camunda.Dockerfile:156).
	CamundaEntrypoint = "/usr/local/camunda/bin/camunda"
)

// The ports of the unified binary (camunda.Dockerfile:129, defaults.yaml,
// application.properties:16,21) and of the connectors runtime (helm chart
// 14.8.3 values.yaml:2327).
const (
	PortGRPC       int32 = 26500
	PortHTTP       int32 = 8080
	PortManagement int32 = 9600
	PortCommand    int32 = 26501
	PortInternal   int32 = 26502
)

// The suffixes of the resources that are not named after a component.
const (
	adminSecretSuffix    = "-camunda-admin"
	serviceAccountSuffix = "-camunda"
)

// WorkloadName returns the name of the workload and Service of a component:
// the cluster name and the component, joined by a dash.
func WorkloadName(cluster *v1.CamundaCluster, component string) string {
	return cluster.Name + "-" + component
}

// AdminSecretName returns the name of the Secret that holds the admin
// credentials of a basic-auth cluster.
func AdminSecretName(cluster *v1.CamundaCluster) string {
	return cluster.Name + adminSecretSuffix
}

// ServiceAccountName returns the name of the ServiceAccount of every
// workload pod. The operator creates it only when spec.serviceAccount is set.
func ServiceAccountName(cluster *v1.CamundaCluster) string {
	return cluster.Name + serviceAccountSuffix
}
