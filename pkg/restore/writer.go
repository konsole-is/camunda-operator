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
	"time"

	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	clustercomponents "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/storagewriter"
)

// RegisterWriter registers owner, a restore into the cluster with UID target,
// as a writer of backend, or renews that registration. A cluster on backend
// waits while it is live, see storagewriter.Live. claimNamespace holds the
// storage claim Leases.
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
		time.Now(),
	)
}

// WriterWait bounds wait, the time until the next look of a restore that is
// registered as a writer, so that the look renews the registration before it
// expires. A zero wait asks for no look and is bounded too.
func WriterWait(wait time.Duration) time.Duration {
	if wait == 0 || wait > storagewriter.RenewInterval {
		return storagewriter.RenewInterval
	}

	return wait
}

// WriterRateLimiter is the rate limiter of a restore controller. It retries a
// failed look within storagewriter.RenewInterval.
func WriterRateLimiter() workqueue.TypedRateLimiter[reconcile.Request] {
	return workqueue.NewTypedItemExponentialFailureRateLimiter[reconcile.Request](
		5*time.Millisecond,
		storagewriter.RenewInterval,
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

// ReleaseWriter ends the registration that RegisterWriter made.
func ReleaseWriter(
	ctx context.Context,
	c client.Client,
	claimNamespace, backend string,
	owner conditions.Owner,
	target types.UID,
) error {
	return storagewriter.Release(ctx, c, claimNamespace, backend, writerOf(owner, target))
}
