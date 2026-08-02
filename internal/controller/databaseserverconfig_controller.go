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

package controller

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

const databaseServerConfigSecretRefsField = "databaseserverconfig.spec.secretRefs"

// DatabaseServerConfigReconciler validates DatabaseServerConfig contracts: it
// checks the admin credentials Secret reference and maintains the Ready
// condition, never provisioning anything.
type DatabaseServerConfigReconciler struct {
	client.Client
	// APIReader reads directly from the API server, bypassing the cache; used for
	// Secret data because Secrets are watched metadata-only.
	APIReader client.Reader
	Scheme    *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile validates the contract's references and maintains its Ready
// condition; it never creates or mutates other resources.
func (r *DatabaseServerConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cfg v1.DatabaseServerConfig
	if err := r.Get(ctx, req.NamespacedName, &cfg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cond, err := r.validate(ctx, &cfg)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, conditions.PatchReady(ctx, r.Client, &cfg, cond)
}

// validate runs the contract's documented checks and returns the resulting
// Ready condition, or an error for transient API failures.
func (r *DatabaseServerConfigReconciler) validate(ctx context.Context, cfg *v1.DatabaseServerConfig) (metav1.Condition, error) {
	ref := cfg.Spec.AdminCredentialsSecretRef

	msg, err := secretref.CheckKeys(ctx, r.APIReader,
		types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name},
		ref.UsernameKey, ref.PasswordKey)
	if err != nil {
		return metav1.Condition{}, err
	}
	if msg != "" {
		return conditions.Ready(metav1.ConditionFalse, conditions.ReasonMissingSecret, msg, cfg.Generation), nil
	}

	return conditions.Ready(metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed", cfg.Generation), nil
}

// SetupWithManager registers the controller, an index of CRs by referenced
// Secret, and a metadata-only Secret watch.
func (r *DatabaseServerConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1.DatabaseServerConfig{},
		databaseServerConfigSecretRefsField, func(o client.Object) []string {
			ref := o.(*v1.DatabaseServerConfig).Spec.AdminCredentialsSecretRef
			return []string{refindex.SecretKey(ref.Namespace, ref.Name)}
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.DatabaseServerConfig{}).
		Watches(&corev1.Secret{},
			refindex.Enqueue(mgr.GetClient(), &v1.DatabaseServerConfigList{},
				databaseServerConfigSecretRefsField, refindex.ObjectNamespacedName),
			builder.OnlyMetadata).
		Named("databaseserverconfig").
		Complete(r)
}
