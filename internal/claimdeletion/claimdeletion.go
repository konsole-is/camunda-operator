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

// Package claimdeletion deletes what a deleted claim generated, for both
// claim kinds: it writes the deletion policy on the requests, then deletes
// the generated objects in stages, then the requests, and removes the
// finalizer of the claim when nothing is left.
package claimdeletion

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/claimpolicy"
	"github.com/konsole-is/camunda-operator/pkg/components/claimrequests"
)

// The event of a delete call that the deletion of a claim sends, in the
// vocabulary of the ocf component deletion.
const (
	eventReasonResourceDeleted = "ResourceDeleted"
	eventActionDelete          = "Delete"
)

// Deleter sends the reads, the writes, and the deletes of a deleted claim.
type Deleter struct {
	// Client writes the requests and the claim, deletes, and lists the
	// PluginRegistrations.
	Client client.Client
	// APIReader reads the generated objects and the requests without the
	// cache.
	APIReader     client.Reader
	Scheme        *runtime.Scheme
	EventRecorder events.EventRecorder
}

// Input is a deleted claim and what it generated.
type Input struct {
	Claim client.Object
	// FieldManagerPrefix is the field manager prefix of the claim kind, such
	// as fleetv1alpha1.FieldManagerPrefixClusterClaim.
	FieldManagerPrefix string
	// DeletionPolicy is the effective policy of the requests, as
	// claimpolicy.DeletionPolicy returns it.
	DeletionPolicy pluginsv1alpha1.DeletionPolicy
	Selections     []fleetv1alpha1.PluginSelection
	// Stages hold the objects that the claim generated apart from its
	// requests, in deletion order. Each object names only its name and
	// namespace. A stage is deleted after every stage before it is gone, and
	// the requests after every stage.
	Stages [][]client.Object
}

// Finalize deletes what in.Claim generated, one step per pass. Before any
// step it writes in.DeletionPolicy on the requests that the claim controls.
// It returns the objects that the claim still waits for, as "<Kind> <name>",
// and removes the finalizer of the claim when nothing is left. An object
// that the claim does not control counts as gone. A claim without the
// finalizer fleet.camunda.io/cleanup returns nil and writes nothing. It
// returns a *conditions.PreCheckFailure, and writes and deletes nothing,
// when a selected registration does not support the policy.
func (d Deleter) Finalize(ctx context.Context, in Input) ([]string, error) {
	if !controllerutil.ContainsFinalizer(in.Claim, fleetv1alpha1.FinalizerCleanup) {
		return nil, nil
	}

	requests, err := d.ownedRequests(ctx, in.Claim)
	if err != nil {
		return nil, err
	}

	// A plugin reads the policy when its request gets a deletion timestamp,
	// so the policy goes on before any delete of this pass.
	if err := d.writeDeletionPolicy(ctx, in, requests); err != nil {
		return nil, err
	}

	stages := make([][]client.Object, 0, len(in.Stages)+1)
	for _, stage := range in.Stages {
		owned, err := d.ownedObjects(ctx, in.Claim, stage)
		if err != nil {
			return nil, err
		}
		stages = append(stages, owned)
	}
	stages = append(stages, requests)

	i := slices.IndexFunc(stages, func(stage []client.Object) bool { return len(stage) > 0 })
	if i < 0 {
		controllerutil.RemoveFinalizer(in.Claim, fleetv1alpha1.FinalizerCleanup)
		if err := d.Client.Update(ctx, in.Claim); err != nil && !apierrors.IsNotFound(err) {
			return nil, fmt.Errorf("removing the finalizer: %w", err)
		}
		return nil, nil
	}

	if err := d.deleteAll(ctx, in.Claim, stages[i]); err != nil {
		return nil, err
	}

	return d.describe(slices.Concat(stages...))
}

// ownedRequests reads the PluginRequests in the namespace of claim that
// claim controls, without the cache.
func (d Deleter) ownedRequests(ctx context.Context, claim client.Object) ([]client.Object, error) {
	// Not by the claim label: a user can remove a label, and the claim must
	// still find the request.
	var list pluginsv1alpha1.PluginRequestList
	if err := d.APIReader.List(ctx, &list, client.InNamespace(claim.GetNamespace())); err != nil {
		return nil, fmt.Errorf("listing the PluginRequests of the claim: %w", err)
	}

	var owned []client.Object
	for i := range list.Items {
		if metav1.IsControlledBy(&list.Items[i], claim) {
			owned = append(owned, &list.Items[i])
		}
	}

	return owned, nil
}

// writeDeletionPolicy applies in.DeletionPolicy to each of requests that has
// no deletion timestamp and another policy. It writes nothing and returns
// the refusal of claimpolicy.CheckDeletionPolicy when a selected
// registration does not support the policy.
func (d Deleter) writeDeletionPolicy(ctx context.Context, in Input, requests []client.Object) error {
	var registrations pluginsv1alpha1.PluginRegistrationList
	if err := d.Client.List(ctx, &registrations); err != nil {
		return fmt.Errorf("listing the PluginRegistrations: %w", err)
	}
	if err := claimpolicy.CheckDeletionPolicy(in.DeletionPolicy, in.Selections, registrations.Items); err != nil {
		return err
	}

	owner := client.FieldOwner(claimrequests.DeletionPolicyFieldManager(in.FieldManagerPrefix, in.Claim.GetUID()))
	for _, obj := range requests {
		request, ok := obj.(*pluginsv1alpha1.PluginRequest)
		if !ok || !request.DeletionTimestamp.IsZero() || request.Spec.DeletionPolicy == in.DeletionPolicy {
			continue
		}

		apply := claimrequests.DeletionPolicyApply(request, in.DeletionPolicy)
		//nolint:staticcheck // the repository applies through the deprecated client.Apply patch
		if err := d.Client.Patch(ctx, apply, client.Apply, owner, client.ForceOwnership); err != nil {
			return fmt.Errorf("writing the deletion policy on PluginRequest %s: %w", request.Name, err)
		}
	}

	return nil
}

// ownedObjects reads each of objs by its name without the cache and returns
// the ones that exist and that claim controls.
func (d Deleter) ownedObjects(ctx context.Context, claim client.Object, objs []client.Object) ([]client.Object, error) {
	var owned []client.Object
	for _, obj := range objs {
		err := d.APIReader.Get(ctx, client.ObjectKeyFromObject(obj), obj)
		switch {
		case apierrors.IsNotFound(err):
			continue
		case err != nil:
			return nil, fmt.Errorf("reading %T %s: %w", obj, obj.GetName(), err)
		}

		if metav1.IsControlledBy(obj, claim) {
			owned = append(owned, obj)
		}
	}

	return owned, nil
}

// deleteAll deletes each of objs that has no deletion timestamp yet, and
// records an event for each delete that it sends.
func (d Deleter) deleteAll(ctx context.Context, claim client.Object, objs []client.Object) error {
	for _, obj := range objs {
		if !obj.GetDeletionTimestamp().IsZero() {
			continue
		}

		kind, err := d.kindOf(obj)
		if err != nil {
			return err
		}

		// The UID keeps the delete off an object that replaced this one
		// under its name since the read.
		err = d.Client.Delete(ctx, obj, client.Preconditions{UID: new(obj.GetUID())})
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return fmt.Errorf("deleting %s %s: %w", kind, obj.GetName(), err)
		}

		d.EventRecorder.Eventf(
			claim,
			obj,
			corev1.EventTypeNormal,
			eventReasonResourceDeleted,
			eventActionDelete,
			"Deleted %s %s",
			kind,
			obj.GetName(),
		)
	}

	return nil
}

// describe returns "<Kind> <name>" for each of objs.
func (d Deleter) describe(objs []client.Object) ([]string, error) {
	names := make([]string, 0, len(objs))
	for _, obj := range objs {
		kind, err := d.kindOf(obj)
		if err != nil {
			return nil, err
		}
		names = append(names, kind+" "+obj.GetName())
	}

	return names, nil
}

func (d Deleter) kindOf(obj client.Object) (string, error) {
	gvk, err := apiutil.GVKForObject(obj, d.Scheme)
	if err != nil {
		return "", fmt.Errorf("finding the kind of %T: %w", obj, err)
	}

	return gvk.Kind, nil
}
