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
// A write that loses a conflict to a spec or metadata write is tried once more
// on the live object. A write that loses a conflict to a status write of
// another writer adds no error: the reconcile requeues, or returns the error
// that it already had.
package ownerstatus

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ConflictRequeue is how long a reconcile waits before it reads the owner
// again after its status write lost a conflict.
const ConflictRequeue = time.Second

// Flush writes the status of rec.Owner with Rebase, and returns the result and
// the error for Reconcile to return, as Settle decides them. base is a copy of
// the owner as the reconcile read it, made before the reconcile staged any
// status. res and err are the values that the reconcile had before the write.
func Flush(
	ctx context.Context,
	rec component.ReconcileContext,
	base client.Object,
	res ctrl.Result,
	err error,
) (ctrl.Result, error) {
	return Settle(res, err, Rebase(ctx, rec, base))
}

// Settle returns the result and the error for Reconcile to return after a
// status write that returned flushErr. res and err are the values that the
// reconcile had before the write. A reconcile that must know whether the
// write landed calls Rebase itself and passes its error here.
//
// A conflict means that another writer changed the status of the owner after
// the reconcile read it, and that nothing was written. When err is nil, Settle
// returns a requeue after ConflictRequeue and no error. When err is set,
// Settle returns res and err, because err requeues the owner already. Any
// other failure of the write is joined to err.
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

// Rebase writes the status of rec.Owner with component.FlushStatus. base is a
// copy of the owner as the reconcile read it. When the write conflicts and the
// live owner still holds the status of base, Rebase writes the staged status
// once more, on the live resourceVersion. When the live status changed, Rebase
// returns the conflict and writes nothing, so it never rolls back another
// writer.
//
// It reads the live owner with rec.APIReader, or with rec.Client when
// rec.APIReader is nil. When the read fails, Rebase returns that failure, not a
// conflict. An owner that is gone returns the conflict.
func Rebase(ctx context.Context, rec component.ReconcileContext, base client.Object) error {
	err := component.FlushStatus(ctx, rec)
	if !apierrors.IsConflict(err) {
		return err
	}

	live, ok := base.DeepCopyObject().(client.Object)
	if !ok {
		return err
	}
	var reader client.Reader = rec.Client
	if rec.APIReader != nil {
		reader = rec.APIReader
	}
	key := client.ObjectKeyFromObject(base)
	if getErr := reader.Get(ctx, key, live); getErr != nil {
		if apierrors.IsNotFound(getErr) {
			return err
		}

		return fmt.Errorf("reading %s after a status conflict: %w", key, getErr)
	}

	same, cmpErr := sameStatus(base, live)
	if cmpErr != nil {
		return cmpErr
	}
	if !same {
		return err
	}

	rec.Owner.SetResourceVersion(live.GetResourceVersion())

	return component.FlushStatus(ctx, rec)
}

// sameStatus reports whether a and b hold the same status.
func sameStatus(a, b client.Object) (bool, error) {
	ua, err := runtime.DefaultUnstructuredConverter.ToUnstructured(a)
	if err != nil {
		return false, fmt.Errorf("converting %s: %w", client.ObjectKeyFromObject(a), err)
	}
	ub, err := runtime.DefaultUnstructuredConverter.ToUnstructured(b)
	if err != nil {
		return false, fmt.Errorf("converting %s: %w", client.ObjectKeyFromObject(b), err)
	}

	return equality.Semantic.DeepEqual(ua["status"], ub["status"]), nil
}
