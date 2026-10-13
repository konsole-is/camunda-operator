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

package main

import (
	"context"
	"fmt"
	"slices"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// collectGarbage does over objs what the garbage collector of
// kube-controller-manager does, which envtest does not run: it deletes the
// dependents of an owner that is gone, and finishes foreground and orphan
// deletions. It returns the number of writes it made.
func (s *simulator) collectGarbage(ctx context.Context, objs []unstructured.Unstructured) (int, error) {
	byUID := map[types.UID]*unstructured.Unstructured{}
	dependents := map[types.UID][]*unstructured.Unstructured{}
	for i := range objs {
		byUID[objs[i].GetUID()] = &objs[i]
		for _, ref := range objs[i].GetOwnerReferences() {
			dependents[ref.UID] = append(dependents[ref.UID], &objs[i])
		}
	}

	writes := 0
	for i := range objs {
		obj := &objs[i]
		n, err := s.collectObject(ctx, obj, byUID, dependents[obj.GetUID()])
		writes += n
		if err != nil {
			return writes, fmt.Errorf("collecting the garbage of %s: %w", describe(obj), err)
		}
	}
	return writes, nil
}

// collectObject applies collectGarbage to obj, whose dependents in objs are
// deps.
func (s *simulator) collectObject(
	ctx context.Context,
	obj *unstructured.Unstructured,
	byUID map[types.UID]*unstructured.Unstructured,
	deps []*unstructured.Unstructured,
) (int, error) {
	if obj.GetDeletionTimestamp() != nil {
		switch {
		case slices.Contains(obj.GetFinalizers(), metav1.FinalizerOrphanDependents):
			return s.orphan(ctx, obj, deps)
		case slices.Contains(obj.GetFinalizers(), metav1.FinalizerDeleteDependents):
			if len(deps) > 0 {
				return s.deleteAll(ctx, deps)
			}
			return 1, s.removeFinalizer(ctx, obj, metav1.FinalizerDeleteDependents)
		}
	}

	refs := obj.GetOwnerReferences()
	if len(refs) == 0 || obj.GetDeletionTimestamp() != nil {
		return 0, nil
	}
	for _, ref := range refs {
		gone, err := s.ownerGone(ctx, obj, ref, byUID)
		if err != nil || !gone {
			return 0, err
		}
	}
	return s.deleteAll(ctx, []*unstructured.Unstructured{obj})
}

// ownerGone reports whether the owner that ref names no longer exists. An
// owner that objs does not hold is read from the API server, because it can
// be outside the scope of the scenario.
func (s *simulator) ownerGone(
	ctx context.Context,
	obj *unstructured.Unstructured,
	ref metav1.OwnerReference,
	byUID map[types.UID]*unstructured.Unstructured,
) (bool, error) {
	if _, ok := byUID[ref.UID]; ok {
		return false, nil
	}

	gv, err := schema.ParseGroupVersion(ref.APIVersion)
	if err != nil {
		return false, err
	}
	mapping, err := s.client.RESTMapper().RESTMapping(schema.GroupKind{Group: gv.Group, Kind: ref.Kind}, gv.Version)
	if meta.IsNoMatchError(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	owner := &unstructured.Unstructured{}
	owner.SetGroupVersionKind(gv.WithKind(ref.Kind))
	key := client.ObjectKey{Name: ref.Name}
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		key.Namespace = obj.GetNamespace()
	}
	err = s.client.Get(ctx, key, owner)
	if apierrors.IsNotFound(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return owner.GetUID() != ref.UID, nil
}

// orphan removes owner from the owner references of each of deps, then
// removes the orphan finalizer of owner.
func (s *simulator) orphan(
	ctx context.Context,
	owner *unstructured.Unstructured,
	deps []*unstructured.Unstructured,
) (int, error) {
	writes := 0
	for _, dep := range deps {
		refs := slices.DeleteFunc(dep.GetOwnerReferences(), func(ref metav1.OwnerReference) bool {
			return ref.UID == owner.GetUID()
		})
		dep.SetOwnerReferences(refs)
		err := s.client.Update(ctx, dep, client.FieldOwner(kubeControllerManager))
		if client.IgnoreNotFound(err) != nil {
			return writes, err
		}
		writes++
	}
	return writes + 1, s.removeFinalizer(ctx, owner, metav1.FinalizerOrphanDependents)
}

// deleteAll deletes each of objs in the background, and ignores one that is
// already gone.
func (s *simulator) deleteAll(ctx context.Context, objs []*unstructured.Unstructured) (int, error) {
	writes := 0
	for _, obj := range objs {
		if obj.GetDeletionTimestamp() != nil {
			continue
		}
		err := s.client.Delete(ctx, obj, client.PropagationPolicy(metav1.DeletePropagationBackground))
		if client.IgnoreNotFound(err) != nil {
			return writes, err
		}
		writes++
	}
	return writes, nil
}

func (s *simulator) removeFinalizer(ctx context.Context, obj *unstructured.Unstructured, finalizer string) error {
	obj.SetFinalizers(slices.DeleteFunc(obj.GetFinalizers(), func(f string) bool { return f == finalizer }))
	err := s.client.Update(ctx, obj, client.FieldOwner(kubeControllerManager))
	return client.IgnoreNotFound(err)
}
