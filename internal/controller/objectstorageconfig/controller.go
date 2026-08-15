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

// Package objectstorageconfig validates ObjectStorageConfig contracts and
// maintains their Ready condition. It never provisions anything.
package objectstorageconfig

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// ObjectStorageConfigReconciler maintains the Ready condition of
// ObjectStorageConfig contracts.
type ObjectStorageConfigReconciler struct {
	client.Client
	// APIReader keeps the uniform reconciler shape of the five contract
	// validation controllers, which cmd/main.go constructs identically. It is
	// unused here because ObjectStorageConfig references no Secrets.
	APIReader client.Reader
	Scheme    *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=objectstorageconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=objectstorageconfigs/status,verbs=get;update;patch

// Reconcile maintains the Ready condition and status.observedGeneration of the
// contract. The CRD schema enforces every ObjectStorageConfig rule at
// admission, and the contract references no other objects. Reconcile never
// creates or mutates other resources.
func (r *ObjectStorageConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cfg v1.ObjectStorageConfig
	if err := r.Get(ctx, req.NamespacedName, &cfg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cond, err := r.validate(ctx, &cfg)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, conditions.PatchReady(ctx, r.Client, &cfg, cond)
}

// validate always reports Healthy. The CRD schema enforces every
// ObjectStorageConfig rule at admission, and the contract references no other
// objects.
//
//nolint:unparam // The error is always nil here. The signature is the uniform validate shape of the five contract validation controllers.
func (r *ObjectStorageConfigReconciler) validate(
	_ context.Context,
	cfg *v1.ObjectStorageConfig,
) (metav1.Condition, error) {
	return conditions.Ready(metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed", cfg.Generation), nil
}

// SetupWithManager registers the controller. The contract references no other
// objects, so it needs no other watches or indexes.
func (r *ObjectStorageConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ObjectStorageConfig{}).
		Named("objectstorageconfig").
		Complete(r)
}
