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
	"errors"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

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

// Registration is a restore that holds a writer registration on Backend, for
// the cluster with UID Target.
type Registration struct {
	Owner   conditions.Owner
	Backend string
	Target  types.UID
}

// Renewable reports whether the Renewer renews the registration of a restore
// with the given backend.
func Renewable(backend string, terminal bool, deleted *metav1.Time) bool {
	return backend != "" && !terminal && deleted.IsZero()
}

// Renewer renews the registrations that List returns, every
// storagewriter.RenewInterval, while this operator leads.
type Renewer struct {
	Client         client.Client
	Reader         client.Reader
	ClaimNamespace string
	List func(ctx context.Context) ([]Registration, error)
}

// NeedLeaderElection makes the manager run Start only on the leader.
func (r *Renewer) NeedLeaderElection() bool { return true }

// Start renews until ctx ends. The manager runs it once this operator leads.
func (r *Renewer) Start(ctx context.Context) error {
	ticker := time.NewTicker(storagewriter.RenewInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := r.renew(ctx, time.Now()); err != nil {
				log.FromContext(ctx).Error(err, "Could not renew the writer registrations of the restores")
			}
		}
	}
}

// renew renews every registration that List returns once, and keeps going
// past a registration that fails.
func (r *Renewer) renew(ctx context.Context, now time.Time) error {
	registrations, err := r.List(ctx)
	if err != nil {
		return err
	}

	var errs []error
	for _, reg := range registrations {
		w := writerOf(reg.Owner, reg.Target)
		if err := storagewriter.Renew(ctx, r.Client, r.Reader, r.ClaimNamespace, reg.Backend, w, now); err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
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
