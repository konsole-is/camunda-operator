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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

const (
	databaseConfigSecretRefsField = "databaseconfig.spec.secretRefs"
	databaseConfigServerRefField  = "databaseconfig.spec.serverRef"
)

// DatabaseConfigReconciler validates DatabaseConfig contracts and maintains
// their Ready condition.
type DatabaseConfigReconciler struct {
	client.Client
	// APIReader reads directly from the API server, bypassing the cache; used for
	// Secret data because Secrets are watched metadata-only.
	APIReader client.Reader
	Scheme    *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile validates the contract's references and maintains its Ready
// condition; it never creates or mutates other resources.
func (r *DatabaseConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cfg v1.DatabaseConfig
	if err := r.Get(ctx, req.NamespacedName, &cfg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cond, err := r.validate(ctx, &cfg)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, conditions.PatchReady(ctx, r.Client, &cfg, cond)
}

// validate runs the contract's documented checks in order — server reference
// first, then each credentials Secret — and returns the first failure as the
// Ready condition.
func (r *DatabaseConfigReconciler) validate(ctx context.Context, cfg *v1.DatabaseConfig) (metav1.Condition, error) {
	var server v1.DatabaseServerConfig
	if err := r.Get(ctx, types.NamespacedName{Name: cfg.Spec.ServerRef}, &server); err != nil {
		if apierrors.IsNotFound(err) {
			msg := fmt.Sprintf("DatabaseServerConfig %q not found", cfg.Spec.ServerRef)
			return conditions.Ready(metav1.ConditionFalse, conditions.ReasonInvalidReference, msg, cfg.Generation), nil
		}
		return metav1.Condition{}, err
	}

	refs := []v1.CredentialsSecretRef{cfg.Spec.CredentialsSecretRef}
	if cfg.Spec.BackupCredentialsSecretRef != nil {
		refs = append(refs, *cfg.Spec.BackupCredentialsSecretRef)
	}

	for _, ref := range refs {
		msg, err := secretref.CheckKeys(ctx, r.APIReader,
			types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name},
			ref.UsernameKey, ref.PasswordKey)
		if err != nil {
			return metav1.Condition{}, err
		}
		if msg != "" {
			return conditions.Ready(metav1.ConditionFalse, conditions.ReasonMissingSecret, msg, cfg.Generation), nil
		}
	}

	return conditions.Ready(metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed", cfg.Generation), nil
}

// SetupWithManager registers the controller, indexes of CRs by referenced
// Secret and DatabaseServerConfig, a metadata-only Secret watch, and a typed
// DatabaseServerConfig watch.
func (r *DatabaseConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1.DatabaseConfig{},
		databaseConfigSecretRefsField, func(o client.Object) []string {
			spec := o.(*v1.DatabaseConfig).Spec
			keys := []string{refindex.NamespacedKey(spec.CredentialsSecretRef.Namespace, spec.CredentialsSecretRef.Name)}
			if spec.BackupCredentialsSecretRef != nil {
				keys = append(keys, refindex.NamespacedKey(spec.BackupCredentialsSecretRef.Namespace, spec.BackupCredentialsSecretRef.Name))
			}
			return keys
		}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1.DatabaseConfig{},
		databaseConfigServerRefField, func(o client.Object) []string {
			return []string{o.(*v1.DatabaseConfig).Spec.ServerRef}
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.DatabaseConfig{}).
		Watches(&corev1.Secret{},
			refindex.Enqueue(mgr.GetClient(), &v1.DatabaseConfigList{},
				databaseConfigSecretRefsField, refindex.ObjectNamespacedName),
			builder.OnlyMetadata).
		Watches(&v1.DatabaseServerConfig{},
			refindex.Enqueue(mgr.GetClient(), &v1.DatabaseConfigList{},
				databaseConfigServerRefField, refindex.ObjectName)).
		Named("databaseconfig").
		Complete(r)
}
