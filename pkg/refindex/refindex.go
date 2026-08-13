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

// Package refindex maps watch events on referenced objects back to the
// contract CRs naming them, via controller-runtime field indexes.
package refindex

import (
	"context"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// NamespacedKey returns the index key under which a CR's reference to a
// namespaced object is stored: "<namespace>/<name>".
func NamespacedKey(namespace, name string) string { return namespace + "/" + name }

// ObjectNamespacedName keys an event object by "<namespace>/<name>". Use it as
// keyOf for namespaced referents such as Secrets.
func ObjectNamespacedName(o client.Object) string {
	return NamespacedKey(o.GetNamespace(), o.GetName())
}

// ObjectName keys an event object by name. Use it as keyOf for cluster-scoped
// referents.
func ObjectName(o client.Object) string { return o.GetName() }

// Enqueue returns an event handler that enqueues a reconcile request for every
// object in list whose index field matches the event object's key computed by
// keyOf. List failures drop the event and are logged; the periodic informer
// resync re-delivers missed transitions only on its own schedule (default
// SyncPeriod is ~10 hours), so the log line is the operational signal.
func Enqueue(
	c client.Client,
	list client.ObjectList,
	field string,
	keyOf func(client.Object) string,
) handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		key := keyOf(o)

		l := list.DeepCopyObject().(client.ObjectList)
		if err := c.List(ctx, l, client.MatchingFields{field: key}); err != nil {
			logf.FromContext(ctx).Error(err, "listing referrers for enqueue", "field", field, "key", key)
			return nil
		}

		items, err := meta.ExtractList(l)
		if err != nil {
			logf.FromContext(ctx).Error(err, "extracting referrer list for enqueue", "field", field, "key", key)
			return nil
		}

		reqs := make([]reconcile.Request, 0, len(items))
		for _, item := range items {
			om, err := meta.Accessor(item)
			if err != nil {
				continue
			}
			reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{
				Name: om.GetName(), Namespace: om.GetNamespace(),
			}})
		}
		return reqs
	})
}
