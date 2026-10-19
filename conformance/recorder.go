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
	"slices"
	"sync"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/resourceversion"
	"k8s.io/apimachinery/pkg/watch"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// recorder keeps, for each key, the earliest object state that its watches
// saw. Earliest is by resource version, so the order holds across watches
// whose events arrive in another order.
type recorder struct {
	mu       sync.Mutex
	seq      int
	marks    map[string]mark
	seen     map[types.UID]bool
	latest   map[string]string
	failures []error
	stop     context.CancelFunc
}

// mark is one object state that a recorder saw.
type mark struct {
	resourceVersion string
	seq             int
	ref             string
}

// watch starts a watch of the lists that newList returns, with opts, at the
// resource version from. Each object of an event that keyOf gives a key is
// observed under that key. A watch that the API server closes starts again.
// A watch that fails after it started is a failure of the recorder. The
// watch runs until ctx ends or it fails.
func (r *recorder) watch(
	ctx context.Context,
	c client.WithWatch,
	newList func() client.ObjectList,
	keyOf func(client.Object) string,
	from string,
	opts ...client.ListOption,
) error {
	after := func(rv string) []client.ListOption {
		return append(slices.Clone(opts), &client.ListOptions{Raw: &metav1.ListOptions{ResourceVersion: rv}})
	}

	w, err := c.Watch(ctx, newList(), after(from)...)
	if err != nil {
		return err
	}

	go func() {
		last := from
		for {
			var err error
			last, err = r.drain(ctx, w, keyOf, last)
			if ctx.Err() != nil {
				return
			}
			if err != nil {
				r.fail(err)
				return
			}

			if w, err = c.Watch(ctx, newList(), after(last)...); err != nil {
				r.fail(fmt.Errorf("starting a watch again: %w", err))
				return
			}
		}
	}()

	return nil
}

// drain observes the events of w until w or ctx ends, and returns the last
// resource version that w gave, or last when it gave none.
func (r *recorder) drain(
	ctx context.Context,
	w watch.Interface,
	keyOf func(client.Object) string,
	last string,
) (string, error) {
	defer w.Stop()

	for {
		select {
		case <-ctx.Done():
			return last, nil
		case ev, ok := <-w.ResultChan():
			if !ok {
				return last, nil
			}

			if ev.Type == watch.Error {
				return last, fmt.Errorf("a watch failed: %w", apierrors.FromObject(ev.Object))
			}

			obj, isObject := ev.Object.(client.Object)
			if !isObject {
				continue
			}
			last = obj.GetResourceVersion()

			if key := keyOf(obj); key != "" && ev.Type != watch.Bookmark {
				r.observe(obj, key)
			}
		}
	}
}

// advance records rv as the last resource version that the watch stream
// gave.
func (r *recorder) advance(stream, rv string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.latest == nil {
		r.latest = map[string]string{}
	}
	r.latest[stream] = rv
}

// caughtUp says whether the watch stream gave the resource version rv or a
// later one.
func (r *recorder) caughtUp(stream, rv string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	latest, ok := r.latest[stream]
	if !ok {
		return false
	}

	cmp, err := resourceversion.CompareResourceVersion(latest, rv)
	return err == nil && cmp >= 0
}

func (r *recorder) fail(err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.failures = append(r.failures, err)
}

// err returns the failures of the watches. The marks of a recorder that
// failed can miss an event.
func (r *recorder) err() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return errors.Join(r.failures...)
}

// observe keeps obj as the mark of key when it is earlier than the mark that
// key has.
func (r *recorder) observe(obj client.Object, key string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if r.seen == nil {
		r.seen = map[types.UID]bool{}
	}
	r.seen[obj.GetUID()] = true

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

// saw says whether the recorder observed the object uid under a key.
func (r *recorder) saw(uid types.UID) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.seen[uid]
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
	if err := r.err(); err != nil {
		return err
	}

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
