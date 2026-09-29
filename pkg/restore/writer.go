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

package restore

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	clustercomponents "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/storagewriter"
)

// RegisterWriter registers owner, a restore into the cluster with UID target,
// as a writer of backend. Another cluster on backend waits while the
// registration exists. claimNamespace holds the storage claim Leases. A
// registration that exists already is fine.
func RegisterWriter(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	claimNamespace, backend string,
	owner conditions.Owner,
	target types.UID,
) error {
	return storagewriter.Register(
		ctx,
		c,
		reader,
		claimNamespace,
		backend,
		clustercomponents.StorageClaimSchema().LeaseName(backend),
		writerOf(owner, target),
	)
}

func writerOf(owner conditions.Owner, target types.UID) storagewriter.Writer {
	return storagewriter.Writer{
		Kind:       owner.GetKind(),
		Namespace:  owner.GetNamespace(),
		Name:       owner.GetName(),
		UID:        owner.GetUID(),
		ClusterUID: target,
	}
}

// ReleaseWriter ends the registration that RegisterWriter made on backend.
func ReleaseWriter(
	ctx context.Context,
	c client.Client,
	claimNamespace, backend string,
	owner conditions.Owner,
	target types.UID,
) error {
	return storagewriter.Release(ctx, c, claimNamespace, backend, writerOf(owner, target))
}

// ReleaseWriters ends every registration of owner on every backend, also one
// that status never recorded. reader must read the API server directly.
func ReleaseWriters(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	claimNamespace string,
	owner client.Object,
) error {
	return storagewriter.ReleaseAll(ctx, c, reader, claimNamespace, owner.GetUID())
}

// OtherWriters returns the writers of backend other than owner, the writers
// for its own target included. reader must read the API server directly.
func OtherWriters(
	ctx context.Context,
	reader client.Reader,
	claimNamespace, backend string,
	owner conditions.Owner,
) ([]string, error) {
	return storagewriter.LiveExcept(
		ctx,
		reader,
		claimNamespace,
		backend,
		clustercomponents.StorageClaimSchema().LeaseName(backend),
		writerOf(owner, ""),
	)
}
