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
	ComponentAdmin      = "admin"
	ComponentConnectors = "connectors"
)

// The names that a user or another operator can observe on the rendered
// resources.
const (
	// ConfigHashAnnotation is the pod template annotation that carries the
	// hash of the rendered configuration. A change to it rolls the pods.
	ConfigHashAnnotation = "camunda.io/config-hash"
	// RequestedStorageSizeAnnotation is the annotation of the broker
	// StatefulSet that carries the effective storageSize. The claim template
	// keeps the size it was created with, so this is where the requested
	// size is visible, and the controller records an ignored shrink only when
	// it changes.
	RequestedStorageSizeAnnotation = "camunda.io/requested-storage-size"
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

// ServiceAccountName returns the name that the ServiceAccount of the cluster
// carries: the name the spec sets, or the name derived from the cluster. It
// never answers whether the cluster has one. A caller that needs the account
// a pod or a Job runs under asks PodServiceAccountName.
//
// It is the principal that a workload identity without an annotation binds,
// so it is part of the contract with the cloud provider.
func ServiceAccountName(cluster *v1.CamundaCluster, e Effective) string {
	if e.ServiceAccount != nil && e.ServiceAccount.Name != "" {
		return e.ServiceAccount.Name
	}

	return cluster.Name + serviceAccountSuffix
}

// PodServiceAccountName returns the ServiceAccount that the pods of the
// cluster run under, or the empty string when they run under the default
// account of their namespace.
//
// The pods need a named account only when something binds one: the spec asks
// for one, or a referenced bucket authenticates through workload identity.
// With static credentials nothing does. The credentials arrive in a Secret,
// so the pods carry no cloud identity and the operator renders no account.
//
// It takes the whole Input, so a bucket that a later spec adds reaches this
// rule through the field the controller already fills. A consumer outside
// this package never asks it: the controller publishes the answer on
// status.serviceAccountName, and the consumer reads that field.
//
// ServiceAccountName answers a different question: the name the account
// carries when the cluster has one. The site that names the ServiceAccount
// resource keeps using it. So does any site that is already gated on whether
// the cluster renders one.
func PodServiceAccountName(in Input) string {
	if in.Effective.ServiceAccount == nil &&
		!bucketUsesWorkloadIdentity(in.Backup) && !bucketUsesWorkloadIdentity(in.Documents) {
		return ""
	}

	return ServiceAccountName(in.Cluster, in.Effective)
}
