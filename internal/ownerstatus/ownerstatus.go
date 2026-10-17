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

// Package ownerstatus writes the status of the object that a reconcile owns.
// A status write that loses a conflict adds no error: the reconcile requeues,
// or returns the error that it already had.
package ownerstatus

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConflictRequeue is how long a reconcile waits before it reads the owner
// again after its status write lost a conflict.
const ConflictRequeue = time.Second

// Flush writes the status of rec.Owner with component.FlushStatus, and returns
// the result and the error for Reconcile to return, as Settle decides them.
// res and err are the values that the reconcile had before the write.
func Flush(ctx context.Context, rec component.ReconcileContext, res ctrl.Result, err error) (ctrl.Result, error) {
	return Settle(res, err, component.FlushStatus(ctx, rec))
}

// Settle returns the result and the error for Reconcile to return after a
// status write that returned flushErr. res and err are the values that the
// reconcile had before the write. A reconcile that must know whether the
// write landed calls component.FlushStatus itself and passes its error here.
//
// A conflict means that another writer changed the owner after the reconcile
// read it, and that nothing was written. When err is nil, Settle returns a
// requeue after ConflictRequeue and no error. When err is set, Settle returns
// res and err, because err requeues the owner already. Any other failure of
// the write is joined to err.
func Settle(res ctrl.Result, err, flushErr error) (ctrl.Result, error) {
	switch {
	case flushErr == nil:
		return res, err
	case apierrors.IsConflict(flushErr):
		if err != nil {
			return res, err
		}

		return ctrl.Result{RequeueAfter: ConflictRequeue}, nil
	default:
		return ctrl.Result{}, errors.Join(err, flushErr)
	}
}

// Record writes, at once, the status fields that owner changed since before,
// a copy of owner that the caller made before it staged them. Call it right
// after a side effect whose result a later pass cannot rebuild, such as the
// id of a backup that the cluster generated, so that a lost status write at
// the end of the pass does not lose that result.
//
// The write is a merge patch that carries no resourceVersion, so a write of
// another field since the read does not make it conflict. The changed fields
// must be status fields that no other writer sets, because the patch replaces
// them whatever the server holds. owner is not changed. The patch moves the
// resourceVersion, so the status write at the end of the same pass
// conflicts, and the next pass reads the recorded fields back.
func Record(ctx context.Context, c client.Client, before, owner client.Object) error {
	receipt, ok := owner.DeepCopyObject().(client.Object)
	if !ok {
		return fmt.Errorf("copying %s: unexpected type %T", client.ObjectKeyFromObject(owner), owner)
	}
	if err := c.Status().Patch(ctx, receipt, client.MergeFrom(before)); err != nil {
		return fmt.Errorf("recording the status of %s: %w", client.ObjectKeyFromObject(owner), err)
	}

	return nil
}
