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
	"slices"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
	"github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
)

// ServiceAccountName returns the ServiceAccount of the cluster of claim: the
// name that the operator derives for a cluster of the claim name.
func ServiceAccountName(claim string) string {
	return camundacluster.ServiceAccountName(clusterNamed(claim), camundacluster.Effective{})
}

// ConnectorsServiceAccountName returns the ServiceAccount of the connectors
// of the cluster of claim: the name that the operator derives for a cluster
// of the claim name.
func ConnectorsServiceAccountName(claim string) string {
	return camundacluster.ConnectorsServiceAccountName(clusterNamed(claim), camundacluster.Effective{})
}

func clusterNamed(name string) *v1.CamundaCluster {
	return &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

// OptimizeName returns the name of the CamundaOptimize of claim.
func OptimizeName(claim string) string {
	return claim + "-optimize"
}

// BackupScheduleName returns the name of the BackupSchedule of claim.
func BackupScheduleName(claim string) string {
	return claim + "-backups"
}

// AdditionalServiceAccounts returns the ServiceAccounts of the built-in
// storage that selections run for claim: the Elasticsearch account, then the
// PostgreSQL account. It returns nil when the claim selects neither.
func AdditionalServiceAccounts(
	claim string,
	selections []fleetv1alpha1.PluginSelection,
	storage BuiltinStorage,
) []string {
	var accounts []string
	if slices.ContainsFunc(selections, registered(storage.ElasticsearchRegistration)) {
		backing := &v1.ElasticsearchCluster{ObjectMeta: metav1.ObjectMeta{Name: claim + storage.ElasticsearchSuffix}}
		accounts = append(accounts, elasticsearchcluster.ServiceAccountName(backing, v1.ElasticsearchClusterSpec{}))
	}
	if slices.ContainsFunc(selections, registered(storage.DatabaseServerRegistration)) {
		backing := &v1.DatabaseServer{ObjectMeta: metav1.ObjectMeta{Name: claim + storage.DatabaseServerSuffix}}
		accounts = append(accounts, databaseserver.ServiceAccountName(backing))
	}

	return accounts
}

func registered(registration string) func(fleetv1alpha1.PluginSelection) bool {
	return func(s fleetv1alpha1.PluginSelection) bool { return s.Registration == registration }
}

// DeletionPolicy returns the deletion policy of the requests of claim: its
// own policy, else the default of its snapshot, else Delete.
func DeletionPolicy(claim *fleetv1alpha1.CamundaClusterClaim) pluginsv1alpha1.DeletionPolicy {
	return claimpolicy.DeletionPolicy(claim.Spec.DeletionPolicy, claim.Status.ResolvedPolicy)
}

// References returns the release and the platform config of the cluster of
// claim: the claim values, else the defaults of environment. A name is empty
// when neither sets it. environment may be nil.
func References(
	claim *fleetv1alpha1.CamundaClusterClaim,
	environment *fleetv1alpha1.Environment,
) (release, platformConfig string) {
	release, platformConfig = claim.Spec.ReleaseRef, claim.Spec.PlatformConfigRef
	if environment == nil {
		return release, platformConfig
	}

	if release == "" && environment.Spec.Releases != nil {
		release = environment.Spec.Releases.Default
	}
	if platformConfig == "" {
		platformConfig = environment.Spec.PlatformConfigRef
	}

	return release, platformConfig
}

// ChangedReferences returns spec.presetRef and spec.releaseRef of claim,
// with "" for each one that live already carries. A nil live, or one that
// the claim does not control, carries nothing.
func ChangedReferences(claim *fleetv1alpha1.CamundaClusterClaim, live *v1.CamundaCluster) (presetRef, releaseRef string) {
	presetRef, releaseRef = claim.Spec.PresetRef, claim.Spec.ReleaseRef
	if live == nil || !metav1.IsControlledBy(live, claim) {
		return presetRef, releaseRef
	}

	if presetRef == live.Spec.PresetRef {
		presetRef = ""
	}
	if releaseRef == live.Spec.ReleaseRef {
		releaseRef = ""
	}

	return presetRef, releaseRef
}
