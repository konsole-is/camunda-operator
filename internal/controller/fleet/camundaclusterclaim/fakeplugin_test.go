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

package camundaclusterclaim

import (
	"context"
	"sync"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// heldRetry is how often the fake plugin looks again at a request of a held
// registration.
const heldRetry = 250 * time.Millisecond

// fakePlugin serves the PluginRequests of the registrations it knows: it
// writes their outputs, OutputsReady=True, and Ready=True for the current
// generation. It writes nothing for a held registration until the spec
// releases it. The specs never write the status of a request.
//
// It holds every request with the plugin finalizer. On a deleted request it
// records spec.deletionPolicy once, then removes the finalizer unless the
// spec holds the cleanup of that request.
type fakePlugin struct {
	reader client.Reader
	writer client.Client

	mu               sync.Mutex
	outputs          map[string]map[string]string
	held             map[string]bool
	cleanupHeld      map[types.NamespacedName]bool
	policyAtDeletion map[types.NamespacedName]pluginsv1alpha1.DeletionPolicy
}

func newFakePlugin() *fakePlugin {
	return &fakePlugin{
		outputs:          map[string]map[string]string{},
		held:             map[string]bool{},
		cleanupHeld:      map[types.NamespacedName]bool{},
		policyAtDeletion: map[types.NamespacedName]pluginsv1alpha1.DeletionPolicy{},
	}
}

// serve makes the plugin write outputs for the requests of registration.
func (p *fakePlugin) serve(registration string, outputs map[string]string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.outputs[registration] = outputs
}

// hold stops the plugin from writing the outputs of registration.
func (p *fakePlugin) hold(registration string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.held[registration] = true
}

// release lets the plugin write the outputs of registration again.
func (p *fakePlugin) release(registration string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.held, registration)
}

// holdCleanup keeps the plugin finalizer on the request key after its
// deletion.
func (p *fakePlugin) holdCleanup(key types.NamespacedName) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cleanupHeld[key] = true
}

// releaseCleanup lets the plugin remove its finalizer from the request key.
func (p *fakePlugin) releaseCleanup(key types.NamespacedName) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.cleanupHeld, key)
}

// deletionPolicyOf returns spec.deletionPolicy of the request key when the
// plugin first saw its deletion, and whether it saw it.
func (p *fakePlugin) deletionPolicyOf(key types.NamespacedName) (pluginsv1alpha1.DeletionPolicy, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	policy, ok := p.policyAtDeletion[key]
	return policy, ok
}

// cleanup records the policy of the deleted request on its first sight and
// reports whether the plugin holds its cleanup.
func (p *fakePlugin) cleanup(request *pluginsv1alpha1.PluginRequest) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	key := client.ObjectKeyFromObject(request)
	if _, seen := p.policyAtDeletion[key]; !seen {
		p.policyAtDeletion[key] = request.Spec.DeletionPolicy
	}
	return p.cleanupHeld[key]
}

// state returns the outputs of registration, whether the plugin serves it,
// and whether it is held.
func (p *fakePlugin) state(registration string) (map[string]string, bool, bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	outputs, ok := p.outputs[registration]
	return outputs, ok, p.held[registration]
}

func (p *fakePlugin) SetupWithManager(mgr ctrl.Manager) error {
	p.reader, p.writer = mgr.GetAPIReader(), mgr.GetClient()

	return ctrl.NewControllerManagedBy(mgr).
		For(&pluginsv1alpha1.PluginRequest{}).
		Named("fakeplugin").
		Complete(p)
}

func (p *fakePlugin) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var request pluginsv1alpha1.PluginRequest
	if err := p.reader.Get(ctx, req.NamespacedName, &request); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	if !request.DeletionTimestamp.IsZero() {
		if p.cleanup(&request) {
			return ctrl.Result{RequeueAfter: heldRetry}, nil
		}
		return ctrl.Result{}, p.setFinalizer(ctx, &request, controllerutil.RemoveFinalizer)
	}

	if err := p.setFinalizer(ctx, &request, controllerutil.AddFinalizer); err != nil {
		return ctrl.Result{}, err
	}

	outputs, served, held := p.state(request.Spec.Registration)
	if !served {
		return ctrl.Result{}, nil
	}
	if held {
		return ctrl.Result{RequeueAfter: heldRetry}, nil
	}

	ready := meta.FindStatusCondition(request.Status.Conditions, pluginsv1alpha1.ConditionOutputsReady)
	if ready != nil && ready.Status == metav1.ConditionTrue && ready.ObservedGeneration == request.Generation {
		return ctrl.Result{}, nil
	}

	request.Status.Outputs = outputs
	for _, conditionType := range []string{pluginsv1alpha1.ConditionOutputsReady, v1.ConditionReady} {
		meta.SetStatusCondition(&request.Status.Conditions, metav1.Condition{
			Type:               conditionType,
			Status:             metav1.ConditionTrue,
			Reason:             "Served",
			Message:            "The fake plugin wrote the outputs",
			ObservedGeneration: request.Generation,
		})
	}
	if err := p.writer.Status().Update(ctx, &request); err != nil {
		if apierrors.IsConflict(err) {
			return ctrl.Result{RequeueAfter: heldRetry}, nil
		}
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

// setFinalizer applies change to the plugin finalizer of request and patches
// the request when that changed it.
func (p *fakePlugin) setFinalizer(
	ctx context.Context,
	request *pluginsv1alpha1.PluginRequest,
	change func(client.Object, string) bool,
) error {
	before := request.DeepCopy()
	if !change(request, pluginsv1alpha1.FinalizerCleanup) {
		return nil
	}

	return client.IgnoreNotFound(p.writer.Patch(ctx, request, client.MergeFrom(before)))
}
