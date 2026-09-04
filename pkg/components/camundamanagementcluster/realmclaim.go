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
	coordinationv1 "k8s.io/api/coordination/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/leaseclaim"
)

// realmClaimLeasePrefix starts the name of every realm claim Lease.
const realmClaimLeasePrefix = "camunda-realm-"

// The annotations of a realm claim Lease name the CamundaManagementCluster
// that holds it, and the realm it holds. Every decision about ownership reads
// them. A Lease without all three holder annotations is not one of ours.
const (
	// RealmClaimHolderNamespaceAnnotation names the namespace of the holder.
	RealmClaimHolderNamespaceAnnotation = "camunda.io/realm-claim-holder-namespace"
	// RealmClaimHolderNameAnnotation names the holder.
	RealmClaimHolderNameAnnotation = "camunda.io/realm-claim-holder-name"
	// RealmClaimHolderUIDAnnotation carries the UID of the holder, which
	// tells it apart from a later management cluster of the same name.
	RealmClaimHolderUIDAnnotation = "camunda.io/realm-claim-holder-uid"
	// RealmClaimRealmAnnotation carries the identity of the claimed realm,
	// as RealmIdentity returns it, for a reader of the Lease.
	RealmClaimRealmAnnotation = "camunda.io/realm-claim-realm"
)

// RealmClaimHolder is the CamundaManagementCluster that a realm claim Lease
// records. The UID tells it apart from a later management cluster of the same
// name.
type RealmClaimHolder = leaseclaim.Holder

// NewRealmClaimLease builds the Lease that claims the realm of target for mc.
// The API server serializes the create, so exactly one management cluster
// holds the realm that target names. The realm annotation carries the
// identity that RealmIdentity returns, for a reader of the Lease.
func NewRealmClaimLease(
	namespace string,
	target v1.KeycloakRealmTarget,
	mc *v1.CamundaManagementCluster,
) *coordinationv1.Lease {
	return RealmClaimSchema().NewLease(namespace, RealmIdentity(target), mc)
}

// RealmClaimSchema is the shape of the claim Leases of a Keycloak realm. The
// CamundaManagementCluster controller runs the claim protocol of
// pkg/leaseclaim over it.
func RealmClaimSchema() leaseclaim.Schema {
	return leaseclaim.Schema{
		Prefix:                    realmClaimLeasePrefix,
		Noun:                      "realm claim",
		HolderNamespaceAnnotation: RealmClaimHolderNamespaceAnnotation,
		HolderNameAnnotation:      RealmClaimHolderNameAnnotation,
		HolderUIDAnnotation:       RealmClaimHolderUIDAnnotation,
		KeyAnnotation:             RealmClaimRealmAnnotation,
		Labels:                    RealmClaimLeaseLabels,
	}
}

// RealmClaimLeaseName returns the name of the Lease that claims the realm
// whose identity RealmIdentity returned: "camunda-realm-<hash of the
// identity>". A realm identity is a URL, which is no DNS subdomain, so the
// name is built from a hash of it. Every claimant of one realm therefore
// meets on one Lease, and the realm annotation says which realm that is.
func RealmClaimLeaseName(identity string) string {
	return RealmClaimSchema().LeaseName(identity)
}

// RealmClaimLeaseLabels returns the labels of the realm claim Leases of the
// management clusters named name. They carry the name alone, so two
// management clusters of two namespaces that share a name share these labels.
// A caller reads RealmClaimHolderOf on a listed Lease to learn which one holds
// it.
func RealmClaimLeaseLabels(name string) map[string]string {
	return labels.Managed(labels.ManagementCluster(name), ComponentRealmClaim)
}

// RealmClaimHolderOf returns the management cluster that the annotations of
// the Lease name, and whether all three of them are there. Only the
// annotations carry ownership. The holderIdentity of the Lease is a display
// form for a reader.
func RealmClaimHolderOf(lease *coordinationv1.Lease) (RealmClaimHolder, bool) {
	return RealmClaimSchema().HolderOf(lease)
}
