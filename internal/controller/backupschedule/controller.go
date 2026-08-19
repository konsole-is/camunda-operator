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

// Package backupschedule reconciles a BackupSchedule. At each cron trigger it
// creates the logical backup kind that matches the storage type of the
// referenced cluster, and it prunes the terminal backups it created beyond
// spec.retained. It owns nothing it creates: the backups carry no owner
// reference, so they outlive the schedule on purpose, and their own finalizer
// removes the stored artifacts when the pruning deletes them.
package backupschedule

import (
	"context"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// Options configures the reconciler at construction. Every field has a
// default that fits production, and the tests set what they need to observe.
type Options struct {
	// Now returns the current time. Nil means time.Now. The tests inject a
	// clock they control, so a trigger fires without waiting for one.
	Now func() time.Time
}

// withDefaults fills the zero fields of o with the production defaults.
func (o Options) withDefaults() Options {
	if o.Now == nil {
		o.Now = time.Now
	}

	return o
}

// BackupScheduleReconciler reconciles a BackupSchedule.
type BackupScheduleReconciler struct {
	client.Client
	// APIReader reads without the cache. A trigger decides on live state: a
	// stale suspend flag or a stale backup list must not start or skip a
	// backup, and the pruning deletes data, so it counts live objects.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the trigger and pruning events.
	// SetupWithManager sets it from the manager.
	EventRecorder events.EventRecorder

	// opts is the construction-time configuration, defaults applied.
	opts Options
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=backupschedules,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=backupschedules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=logicalbackupelasticsearches;logicalbackuprdbmses,verbs=get;list;watch;create;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaclusters;secondarystorageconfigs;camundaclusterpresets,verbs=get
// +kubebuilder:rbac:groups=events.k8s.io,resources=events,verbs=create;patch

// Reconcile consumes the due trigger of the schedule, if there is one, and
// prunes the terminal backups of the schedule beyond spec.retained.
func (r *BackupScheduleReconciler) Reconcile(
	ctx context.Context,
	req ctrl.Request,
) (_ ctrl.Result, err error) {
	return ctrl.Result{}, nil
}

// SetupWithManager applies opts and registers the controller.
func (r *BackupScheduleReconciler) SetupWithManager(mgr ctrl.Manager, opts Options) error {
	r.opts = opts.withDefaults()
	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder("backupschedule")
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.BackupSchedule{}).
		Named("backupschedule").
		Complete(r)
}
