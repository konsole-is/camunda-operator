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
	secondaryStorageConfigSecretRefsField        = "secondarystorageconfig.spec.secretRefs"
	secondaryStorageConfigDatabaseConfigRefField = "secondarystorageconfig.spec.rdbms.databaseConfigRef"
)

// SecondaryStorageConfigReconciler validates SecondaryStorageConfig contracts
// and maintains their Ready condition.
type SecondaryStorageConfigReconciler struct {
	client.Client
	// APIReader reads directly from the API server, bypassing the cache; used for
	// Secret data because Secrets are watched metadata-only.
	APIReader client.Reader
	Scheme    *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=secondarystorageconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=secondarystorageconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile validates the contract's references and maintains its Ready
// condition; it never creates or mutates other resources.
func (r *SecondaryStorageConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cfg v1.SecondaryStorageConfig
	if err := r.Get(ctx, req.NamespacedName, &cfg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cond, err := r.validate(ctx, &cfg)
	if err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, conditions.PatchReady(ctx, r.Client, &cfg, cond)
}

// validate branches on the contract's storage type: elasticsearch contracts
// check their credentials Secret, rdbms contracts check the referenced
// DatabaseConfig exists in the contract's own namespace. The CRD schema
// enforces that exactly the matching block is set; an object that slipped past
// admission with a missing block or unknown type yields an error rather than a
// condition.
func (r *SecondaryStorageConfigReconciler) validate(ctx context.Context, cfg *v1.SecondaryStorageConfig) (metav1.Condition, error) {
	switch {
	case cfg.Spec.Type == v1.SecondaryStorageTypeElasticsearch && cfg.Spec.Elasticsearch != nil:
		ref := cfg.Spec.Elasticsearch.CredentialsSecretRef
		msg, err := secretref.CheckKeys(ctx, r.APIReader,
			types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name},
			ref.UsernameKey, ref.PasswordKey)
		if err != nil {
			return metav1.Condition{}, err
		}
		if msg != "" {
			return conditions.Ready(metav1.ConditionFalse, conditions.ReasonMissingSecret, msg, cfg.Generation), nil
		}
	case cfg.Spec.Type == v1.SecondaryStorageTypeRDBMS && cfg.Spec.RDBMS != nil:
		name := cfg.Spec.RDBMS.DatabaseConfigRef
		var db v1.DatabaseConfig
		if err := r.Get(ctx, types.NamespacedName{Namespace: cfg.Namespace, Name: name}, &db); err != nil {
			if apierrors.IsNotFound(err) {
				msg := fmt.Sprintf("DatabaseConfig %q not found", name)
				return conditions.Ready(metav1.ConditionFalse, conditions.ReasonInvalidReference, msg, cfg.Generation), nil
			}
			return metav1.Condition{}, err
		}
	default:
		return metav1.Condition{}, fmt.Errorf("spec.type %q has no matching configuration block", cfg.Spec.Type)
	}
	return conditions.Ready(metav1.ConditionTrue, conditions.ReasonHealthy, "All checks passed", cfg.Generation), nil
}

// SetupWithManager registers the controller, indexes of contracts by
// referenced Secret and same-namespace DatabaseConfig, a metadata-only Secret
// watch, and a typed DatabaseConfig watch.
func (r *SecondaryStorageConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1.SecondaryStorageConfig{},
		secondaryStorageConfigSecretRefsField, func(o client.Object) []string {
			es := o.(*v1.SecondaryStorageConfig).Spec.Elasticsearch
			if es == nil {
				return nil
			}
			ref := es.CredentialsSecretRef
			return []string{refindex.NamespacedKey(ref.Namespace, ref.Name)}
		}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1.SecondaryStorageConfig{},
		secondaryStorageConfigDatabaseConfigRefField, func(o client.Object) []string {
			rdbms := o.(*v1.SecondaryStorageConfig).Spec.RDBMS
			if rdbms == nil {
				return nil
			}
			return []string{refindex.NamespacedKey(o.GetNamespace(), rdbms.DatabaseConfigRef)}
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.SecondaryStorageConfig{}).
		Watches(&corev1.Secret{},
			refindex.Enqueue(mgr.GetClient(), &v1.SecondaryStorageConfigList{},
				secondaryStorageConfigSecretRefsField, refindex.ObjectNamespacedName),
			builder.OnlyMetadata).
		Watches(&v1.DatabaseConfig{},
			refindex.Enqueue(mgr.GetClient(), &v1.SecondaryStorageConfigList{},
				secondaryStorageConfigDatabaseConfigRefField, refindex.ObjectNamespacedName)).
		Named("secondarystorageconfig").
		Complete(r)
}
