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
	"crypto/sha256"
	"encoding/hex"

	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// realmClaimLeasePrefix starts the name of every realm claim Lease.
const realmClaimLeasePrefix = "camunda-realm-"

// maxHolderIdentityLength bounds the holderIdentity field of a realm claim
// Lease. The field is a display form for a reader, and the exact holder lives
// in the annotations, which every decision about ownership reads.
const maxHolderIdentityLength = 128

// The annotations of a realm claim Lease name the CamundaManagementCluster
// that holds it, and the realm it holds. Every decision about ownership reads
// them. A Lease without all three holder annotations is not one of ours.
const (
	RealmClaimHolderNamespaceAnnotation = "camunda.io/realm-claim-holder-namespace"
	RealmClaimHolderNameAnnotation      = "camunda.io/realm-claim-holder-name"
	RealmClaimHolderUIDAnnotation       = "camunda.io/realm-claim-holder-uid"
	RealmClaimRealmAnnotation           = "camunda.io/realm-claim-realm"
)

// RealmClaimHolder is the CamundaManagementCluster that a realm claim Lease
// records. The UID tells it apart from a later management cluster of the same
// name.
type RealmClaimHolder struct {
	types.NamespacedName
	UID types.UID
}

// NewRealmClaimLease builds the Lease that claims the realm of target for mc.
// The API server serializes the create, so exactly one management cluster
// holds the realm that target names. The realm annotation carries the
// identity that RealmIdentity returns, for a reader of the Lease.
func NewRealmClaimLease(
	namespace string,
	target v1.KeycloakRealmTarget,
	mc *v1.CamundaManagementCluster,
) *coordinationv1.Lease {
	holder := labels.BoundedName(mc.Namespace+"/"+mc.Name, maxHolderIdentityLength)
	identity := RealmIdentity(target)
	now := metav1.NowMicro()

	return &coordinationv1.Lease{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      RealmClaimLeaseName(identity),
			Labels:    RealmClaimLeaseLabels(mc.Name),
			Annotations: map[string]string{
				RealmClaimHolderNamespaceAnnotation: mc.Namespace,
				RealmClaimHolderNameAnnotation:      mc.Name,
				RealmClaimHolderUIDAnnotation:       string(mc.UID),
				RealmClaimRealmAnnotation:           identity,
			},
		},
		Spec: coordinationv1.LeaseSpec{HolderIdentity: &holder, AcquireTime: &now},
	}
}

// RealmClaimLeaseName returns the name of the Lease that claims the realm
// whose identity RealmIdentity returned: "camunda-realm-<hash of the
// identity>". A realm identity is a URL, which is no DNS subdomain, so the
// name is built from a hash of it. Every claimant of one realm therefore
// meets on one Lease, and the realm annotation says which realm that is.
func RealmClaimLeaseName(identity string) string {
	sum := sha256.Sum256([]byte(identity))

	return realmClaimLeasePrefix + hex.EncodeToString(sum[:])[:40]
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
	annotations := lease.GetAnnotations()
	holder := RealmClaimHolder{
		NamespacedName: types.NamespacedName{
			Namespace: annotations[RealmClaimHolderNamespaceAnnotation],
			Name:      annotations[RealmClaimHolderNameAnnotation],
		},
		UID: types.UID(annotations[RealmClaimHolderUIDAnnotation]),
	}
	if holder.Namespace == "" || holder.Name == "" || holder.UID == "" {
		return RealmClaimHolder{}, false
	}

	return holder, true
}
