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

package conformance

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"k8s.io/apimachinery/pkg/util/resourceversion"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// recorder keeps, for each key, the earliest object state that its watches
// saw. Earliest is by resource version, so the order holds across watches
// whose events arrive in another order.
type recorder struct {
	mu    sync.Mutex
	seq   int
	marks map[string]mark
	stop  context.CancelFunc
}

// mark is one object state that a recorder saw.
type mark struct {
	resourceVersion string
	seq             int
	ref             string
}

// watch starts a watch of list with opts. Each object of an event that keyOf
// gives a key is observed under that key. The watch runs until ctx ends.
func (r *recorder) watch(
	ctx context.Context,
	c client.WithWatch,
	list client.ObjectList,
	keyOf func(client.Object) string,
	opts ...client.ListOption,
) error {
	w, err := c.Watch(ctx, list, opts...)
	if err != nil {
		return err
	}

	go func() {
		defer w.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case ev, ok := <-w.ResultChan():
				if !ok {
					return
				}

				obj, isObject := ev.Object.(client.Object)
				if !isObject || ev.Type == watch.Error || ev.Type == watch.Bookmark {
					continue
				}

				if key := keyOf(obj); key != "" {
					r.observe(obj, key)
				}
			}
		}
	}()

	return nil
}

// observe keeps obj as the mark of key when it is earlier than the mark that
// key has.
func (r *recorder) observe(obj client.Object, key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.seq++
	m := mark{resourceVersion: obj.GetResourceVersion(), seq: r.seq, ref: objectRef(obj)}

	if old, ok := r.marks[key]; ok && !m.before(old) {
		return
	}

	if r.marks == nil {
		r.marks = map[string]mark{}
	}
	r.marks[key] = m
}

// first returns the earliest mark of key.
func (r *recorder) first(key string) (mark, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	m, ok := r.marks[key]
	return m, ok
}

// before compares the resource versions, and the order of arrival when a
// resource version cannot be compared.
func (m mark) before(other mark) bool {
	cmp, err := resourceversion.CompareResourceVersion(m.resourceVersion, other.resourceVersion)
	if err != nil {
		return m.seq < other.seq
	}

	return cmp < 0
}

func finalizerKey(request string) string {
	return "finalizer/" + request
}

func objectKey(request string) string {
	return "object/" + request
}

// finalizerProblem says whether the request got the cleanup finalizer before
// the first object that the plugin wrote for it.
func finalizerProblem(r *recorder, request string) error {
	f, ok := r.first(finalizerKey(request))
	if !ok {
		return errors.New("the request never got the finalizer " + pluginsv1alpha1.FinalizerCleanup)
	}

	if o, ok := r.first(objectKey(request)); ok && o.before(f) {
		return fmt.Errorf("%s appeared before the finalizer %s", o.ref, pluginsv1alpha1.FinalizerCleanup)
	}

	return nil
}

// objectRef names obj as <Kind> <namespace>/<name>, or <Kind> <name> when it
// is cluster-scoped.
func objectRef(obj client.Object) string {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	if obj.GetNamespace() == "" {
		return kind + " " + obj.GetName()
	}

	return kind + " " + obj.GetNamespace() + "/" + obj.GetName()
}
