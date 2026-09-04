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

package database

import (
	"context"
	"fmt"

	coordinationv1 "k8s.io/api/coordination/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/leaseclaim"
)

// ClaimComponent is the component label value of a claim Lease.
const ClaimComponent = "database-claim"

// claimLeasePrefix starts the name of every claim Lease.
const claimLeasePrefix = "camunda-database-"

// The annotations of a claim Lease name the Database that holds it, and the
// claim it holds. Every decision about ownership reads them. A Lease without
// all three holder annotations is not one of ours.
const (
	ClaimHolderNamespaceAnnotation = "camunda.io/database-claim-holder-namespace"
	ClaimHolderNameAnnotation      = "camunda.io/database-claim-holder-name"
	ClaimHolderUIDAnnotation       = "camunda.io/database-claim-holder-uid"
	ClaimKeyAnnotation             = "camunda.io/database-claim-key"
)

// ClaimHolder is the Database that a claim Lease records. The UID tells it
// apart from a later Database of the same name.
type ClaimHolder = leaseclaim.Holder

// Claim is one logical database that a Database holds. Key is the claim key,
// so CollisionIdentity gives the PostgreSQL instance behind it.
type Claim struct {
	Holder ClaimHolder
	Key    string
}

// ClaimSchema is the shape of the claim Leases of a logical database. The
// Database controller runs the claim protocol of pkg/leaseclaim over it.
func ClaimSchema() leaseclaim.Schema[*v1.Database] {
	return leaseclaim.Schema[*v1.Database]{
		Prefix:                    claimLeasePrefix,
		Noun:                      "claim",
		HolderNamespaceAnnotation: ClaimHolderNamespaceAnnotation,
		HolderNameAnnotation:      ClaimHolderNameAnnotation,
		HolderUIDAnnotation:       ClaimHolderUIDAnnotation,
		KeyAnnotation:             ClaimKeyAnnotation,
		Labels:                    ClaimLeaseLabels,
	}
}

// ClaimLeaseLabels returns the labels of the claim Leases of the Databases
// named name. They carry the name alone, so two Databases of two namespaces
// that share a name share these labels. A caller reads ClaimHolderOf on a
// listed Lease to learn which Database holds it.
func ClaimLeaseLabels(name string) map[string]string {
	return labels.Managed(labels.Database(name), ClaimComponent)
}

// ListClaims returns every claim that the Leases of namespace record. The
// namespace is the one the operator runs in, where every claimant of one
// logical database meets. A Lease that names no Database, and one that names
// no claim, is not a claim of this operator and is left out.
//
// The claim is what a Database occupies, not what it asks for. A caller that
// decides whether a PostgreSQL instance is free reads this, and never the
// server that a spec names.
func ListClaims(ctx context.Context, reader client.Reader, namespace string) ([]Claim, error) {
	var leases coordinationv1.LeaseList
	err := reader.List(
		ctx, &leases,
		client.InNamespace(namespace),
		client.MatchingLabels(claimLeaseSelector()),
	)
	if err != nil {
		return nil, fmt.Errorf("listing the claim Leases of namespace %q: %w", namespace, err)
	}

	claims := make([]Claim, 0, len(leases.Items))
	for i := range leases.Items {
		lease := &leases.Items[i]
		holder, ours := ClaimHolderOf(lease)
		if key := lease.Annotations[ClaimKeyAnnotation]; ours && key != "" {
			claims = append(claims, Claim{Holder: holder, Key: key})
		}
	}

	return claims, nil
}

// claimLeaseSelector matches the claim Lease of every Database.
func claimLeaseSelector() map[string]string {
	return map[string]string{
		labels.ComponentKey: ClaimComponent,
		labels.ManagedByKey: labels.ManagedBy,
	}
}

// ClaimHolderOf returns the Database that the annotations of the Lease name,
// and whether all three of them are there. Only the annotations carry
// ownership. The holderIdentity of the Lease is a display form for a reader.
func ClaimHolderOf(lease *coordinationv1.Lease) (ClaimHolder, bool) {
	return ClaimSchema().HolderOf(lease)
}
