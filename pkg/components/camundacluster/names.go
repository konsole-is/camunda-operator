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
	"k8s.io/apimachinery/pkg/util/validation"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
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
	// AllowVersionDowngradeAnnotation is the annotation of a CamundaCluster
	// that sanctions one move of its effective version below the version its
	// brokers run. The value is the exact target version, x.y.z. The
	// controller removes the annotation once the brokers carry that version,
	// and as soon as it names a version other than the effective one. Set it
	// in the same edit as the version, or after the refusal.
	AllowVersionDowngradeAnnotation = "camunda.io/allow-version-downgrade"
	// BrokerVersionAnnotation is the annotation that carries the Camunda
	// version of the brokers. The rendered broker StatefulSet carries the
	// effective version, which the image tag cannot tell once a release pins
	// the image. The controller stamps it on every bound broker claim with
	// the highest version an applied broker StatefulSet has asked the
	// claim's data to run, which can lead the pods during a roll. The stamp
	// never goes down on a live claim. It outlives the cluster when the
	// claims are retained, so the version rule holds for a cluster recreated
	// on them.
	BrokerVersionAnnotation = "camunda.io/broker-version"
	// AdminUsername is the initial admin user of a basic-auth cluster.
	AdminUsername = "admin"
	// DefaultAdminEmail is the email of the seeded admin user when
	// spec.auth.basic.adminEmail names none. The domain is the one that RFC
	// 2606 reserves for documentation, so an unset value never claims an
	// address that somebody owns. The user API validates the address on
	// every update and refuses a domain without a dot, such as
	// admin@localhost, with 400 INVALID_ARGUMENT.
	DefaultAdminEmail = "admin@example.com"
	// AdminUsernameKey, AdminEmailKey, and AdminPasswordKey are the keys of
	// the admin Secret. The email lives here, and not in the rendered pod
	// template, so that a changed address never restarts a workload: the
	// processes read it from this Secret, and the orchestration cluster
	// reads it once, when it seeds the user. It always carries an address,
	// because a process that read none would seed an incomplete user.
	AdminUsernameKey = "username"
	AdminEmailKey    = "email"
	AdminPasswordKey = "password"
	// AdminAppliedEmailKey holds the address that the orchestration cluster
	// has accepted, which is not always the one under AdminEmailKey: a
	// changed address is published for the processes at once and recorded
	// here only after the user API takes it. The operator compares the two
	// to decide whether the cluster still has to be told. The workloads
	// never read it.
	AdminAppliedEmailKey = "email-applied"
	// AdminPendingPasswordKey holds the requested password while a rotation
	// is in flight, next to the active one under AdminPasswordKey. The
	// workloads never read it.
	AdminPendingPasswordKey = "password-pending"
	// AdminPendingRotationKey holds the rotation value that staged the
	// password under AdminPendingPasswordKey. The two travel together, so a
	// promote records the request that produced the password it promotes,
	// even when the spec changed while the rotation was in flight. The
	// workloads never read it.
	AdminPendingRotationKey = "password-pending-rotation"
	// AdminRotationKey holds the spec.auth.basic.passwordRotation value that
	// produced the password under AdminPasswordKey. It travels with the
	// password, in the same apply, so the operator always knows which
	// request the published password answers. status.adminPassword.rotation
	// projects it. The workloads never read it.
	AdminRotationKey = "password-rotation"
	// DataVolumeName is the volume claim template of the brokers.
	DataVolumeName = "data"
	// DataMountPath is where the brokers keep their data. It is the
	// camunda.data.primary-storage.directory default "data" under
	// CAMUNDA_HOME (PrimaryStorage.java:24, camunda.Dockerfile:122-131).
	DataMountPath = "/usr/local/camunda/data"
	// CAMountPath is where the Elasticsearch CA certificate is mounted.
	CAMountPath = "/etc/camunda/es-ca"
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
// the cluster name and the component, joined by a dash. The Service is the
// tightest bound of the three, a DNS label of 63 characters, so a long
// cluster name truncates and keeps its identity in a hash.
func WorkloadName(cluster *v1.CamundaCluster, component string) string {
	suffix := "-" + component

	return labels.BoundedName(cluster.Name, validation.DNS1123LabelMaxLength-len(suffix)) + suffix
}

// AdminSecretName returns the name of the Secret that holds the admin
// credentials of a basic-auth cluster.
func AdminSecretName(cluster *v1.CamundaCluster) string {
	limit := validation.DNS1123LabelMaxLength - len(adminSecretSuffix)

	return labels.BoundedName(cluster.Name, limit) + adminSecretSuffix
}

// PodServiceAccountName returns the ServiceAccount that the pods of the
// cluster run under, or the empty string when they run under the default
// account of their namespace.
//
// The pods need a named account only when something binds one: the spec asks
// for one, or a referenced bucket authenticates through workload identity.
//
// A consumer outside this package never asks it: the controller publishes the
// answer on status.serviceAccountName, and the consumer reads that field.
//
// ServiceAccountName answers a different question: the name the account
// carries when the cluster has one.
func PodServiceAccountName(in Input) string {
	if in.Effective.ServiceAccount == nil &&
		!bucketUsesWorkloadIdentity(in.Backup) && !bucketUsesWorkloadIdentity(in.Documents) {
		return ""
	}

	return ServiceAccountName(in.Cluster, in.Effective)
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

	limit := validation.DNS1123SubdomainMaxLength - len(serviceAccountSuffix)

	return labels.BoundedName(cluster.Name, limit) + serviceAccountSuffix
}
