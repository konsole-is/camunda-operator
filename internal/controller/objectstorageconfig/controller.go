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

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/observability"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

// controllerName is the name the controller registers with controller-runtime.
// It labels its events and every metrics series it records.
const controllerName = "objectstorageconfig"

// ObjectStorageConfigReconciler validates ObjectStorageConfig contracts and
// maintains their Ready condition.
type ObjectStorageConfigReconciler struct {
	client.Client
	// APIReader reads directly from the API server and bypasses the cache.
	// Secret data needs it, because Secrets are watched metadata-only.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// Metrics records the condition gauge and the apply counters of the
	// framework. SetupWithManager sets it when it is nil.
	Metrics component.MetricsRecorder
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=objectstorageconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=objectstorageconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile validates the credentials Secret of the contract, when it has
// one, and maintains its Ready condition. It never creates or mutates other
// resources.
func (r *ObjectStorageConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cfg v1.ObjectStorageConfig
	if err := r.Get(ctx, req.NamespacedName, &cfg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cond, err := r.validate(ctx, &cfg)
	if err != nil {
		return ctrl.Result{}, err
	}

	conditions.Stage(&cfg, cond)

	// The contract owns no components, so its Ready condition follows the
	// server on a conflict and is staged again on the next reconcile.
	return ctrl.Result{}, component.FlushStatus(
		ctx,
		component.ReconcileContext{Client: r.Client, APIReader: r.APIReader, Metrics: r.Metrics, Owner: &cfg},
		nil,
	)
}

// validate checks the static credentials Secret of the contract. The CRD
// schema enforces every other rule at admission, and a workload-identity
// contract references no Secrets.
func (r *ObjectStorageConfigReconciler) validate(
	ctx context.Context,
	cfg *v1.ObjectStorageConfig,
) (metav1.Condition, error) {
	creds := cfg.CredentialsSecret()
	if creds == nil {
		return conditions.Ready(metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed", cfg.Generation), nil
	}

	msg, err := secretref.CheckKeys(
		ctx, r.APIReader,
		types.NamespacedName{Namespace: creds.Namespace, Name: creds.Name}, creds.Keys...,
	)
	if err != nil {
		return metav1.Condition{}, err
	}
	if msg != "" {
		return conditions.Ready(metav1.ConditionFalse, v1.ReasonMissingSecret, msg, cfg.Generation), nil
	}

	return conditions.Ready(metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed", cfg.Generation), nil
}

// SetupWithManager registers the controller, an index of contracts by their
// credentials Secret, and a metadata-only Secret watch.
func (r *ObjectStorageConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Metrics == nil {
		r.Metrics = observability.Recorder(controllerName)
	}

	if err := refindex.EnsureObjectStorageConfigSecretIndex(mgr); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ObjectStorageConfig{}).
		Watches(
			&corev1.Secret{},
			refindex.Enqueue(
				mgr.GetClient(), &v1.ObjectStorageConfigList{},
				refindex.ObjectStorageConfigSecretField, refindex.ObjectNamespacedName,
			),
			builder.OnlyMetadata,
		).
		Named(controllerName).
		Complete(r)
}
