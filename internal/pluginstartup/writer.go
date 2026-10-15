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

package pluginstartup

import (
	"context"
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// +kubebuilder:rbac:groups=plugins.camunda.io,resources=pluginsections;pluginregistrations,verbs=get;create;patch

// maxRetryDelay bounds the wait between two tries of Start.
const maxRetryDelay = time.Minute

// Writer applies the core PluginSections and the built-in
// PluginRegistrations once, each time the manager starts. It is a manager
// Runnable.
type Writer struct {
	client          client.Client
	operatorVersion string
	serviceAccount  pluginsv1alpha1.ServiceAccountRef
	retryDelay      time.Duration
}

// New returns a Writer that applies through c. Each built-in registration
// names operatorVersion as the version of its controller, and
// serviceAccount as the ServiceAccount of the operator.
func New(c client.Client, operatorVersion string, serviceAccount pluginsv1alpha1.ServiceAccountRef) *Writer {
	return &Writer{
		client:          c,
		operatorVersion: operatorVersion,
		serviceAccount:  serviceAccount,
		retryDelay:      time.Second,
	}
}

// Start applies every core section and built-in registration with
// server-side apply under FieldManager. It sets the fields that it writes
// back to their values, and keeps the fields that another manager wrote. It
// never deletes an object, because a claim can still name a section that a
// newer operator no longer writes. When an apply fails, Start logs the error
// and tries again, after a delay of at most maxRetryDelay. It returns nil
// once every object is applied or when ctx ends, so a failed write never
// stops the manager.
func (w *Writer) Start(ctx context.Context) error {
	log := logf.FromContext(ctx).WithName("pluginstartup")
	delay := w.retryDelay

	for {
		err := w.apply(ctx)
		if err == nil {
			log.Info("Applied the core PluginSections and the built-in PluginRegistrations")
			return nil
		}
		log.Error(err, "Failed to apply the core PluginSections and the built-in PluginRegistrations", "retryAfter", delay)

		select {
		case <-ctx.Done():
			return nil
		case <-time.After(delay):
		}
		delay = min(2*delay, maxRetryDelay)
	}
}

// NeedLeaderElection reports true, so that only the leader replica writes
// the objects.
func (w *Writer) NeedLeaderElection() bool {
	return true
}

func (w *Writer) apply(ctx context.Context) error {
	var objs []client.Object
	for _, s := range Sections() {
		objs = append(objs, &s)
	}
	for _, r := range Registrations(w.operatorVersion, w.serviceAccount) {
		objs = append(objs, &r)
	}

	for _, obj := range objs {
		//nolint:staticcheck // the repository applies through the deprecated client.Apply patch
		err := w.client.Patch(ctx, obj, client.Apply, client.FieldOwner(FieldManager), client.ForceOwnership)
		if err != nil {
			return fmt.Errorf("applying %s %q: %w", obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), err)
		}
	}

	return nil
}
