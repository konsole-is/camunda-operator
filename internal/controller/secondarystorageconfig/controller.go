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

// Package secondarystorageconfig validates SecondaryStorageConfig contracts
// and maintains their Ready condition. It never provisions anything.
package secondarystorageconfig

import (
	"context"
	"fmt"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
const controllerName = "secondarystorageconfig"

// SecretRefsField is the index field that lists bindings by the Secrets they
// reference, keyed with refindex.NamespacedKey. Other controllers look
// bindings up by it when a Secret changes.
const SecretRefsField = "secondarystorageconfig.spec.secretRefs"

const secondaryStorageConfigDatabaseConfigRefField = "secondarystorageconfig.spec.rdbms.databaseConfigRef"

// SecondaryStorageConfigReconciler validates SecondaryStorageConfig contracts
// and maintains their Ready condition.
type SecondaryStorageConfigReconciler struct {
	client.Client
	// APIReader reads directly from the API server and bypasses the cache.
	// Secret data needs it, because Secrets are watched metadata-only.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// Metrics records the condition gauge and the apply counters of the
	// framework. SetupWithManager sets it when it is nil.
	Metrics component.MetricsRecorder
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=secondarystorageconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=secondarystorageconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile validates the references of the contract and maintains its Ready
// condition. It never creates or mutates other resources.
func (r *SecondaryStorageConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cfg v1.SecondaryStorageConfig
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

// validate branches on the storage type of the contract. An elasticsearch
// contract checks its credentials Secret and, when set, its CA Secret. An
// rdbms contract checks that the referenced DatabaseConfig exists in the
// namespace of the contract. The CRD schema makes sure that exactly the
// matching block is set. An object that passed admission with a missing block
// or an unknown type yields an error, not a condition.
func (r *SecondaryStorageConfigReconciler) validate(
	ctx context.Context,
	cfg *v1.SecondaryStorageConfig,
) (metav1.Condition, error) {
	switch {
	case cfg.Spec.Type == v1.SecondaryStorageTypeElasticsearch && cfg.Spec.Elasticsearch != nil:
		ref := cfg.Spec.Elasticsearch.CredentialsSecretRef
		msg, err := secretref.CheckKeys(
			ctx, r.APIReader,
			types.NamespacedName{Namespace: cfg.Namespace, Name: ref.Name},
			ref.UsernameKey, ref.PasswordKey,
		)
		if err != nil {
			return metav1.Condition{}, err
		}
		if msg != "" {
			return conditions.Ready(metav1.ConditionFalse, v1.ReasonMissingSecret, msg, cfg.Generation), nil
		}

		if ca := cfg.Spec.Elasticsearch.CASecretRef; ca != nil {
			msg, err := secretref.CheckKeys(
				ctx, r.APIReader,
				types.NamespacedName{Namespace: cfg.Namespace, Name: ca.Name}, ca.Key,
			)
			if err != nil {
				return metav1.Condition{}, err
			}
			if msg != "" {
				return conditions.Ready(metav1.ConditionFalse, v1.ReasonMissingSecret, msg, cfg.Generation), nil
			}
		}
	case cfg.Spec.Type == v1.SecondaryStorageTypeRDBMS && cfg.Spec.RDBMS != nil:
		name := cfg.Spec.RDBMS.DatabaseConfigRef
		var db v1.DatabaseConfig
		if err := r.Get(ctx, types.NamespacedName{Namespace: cfg.Namespace, Name: name}, &db); err != nil {
			if apierrors.IsNotFound(err) {
				msg := fmt.Sprintf("DatabaseConfig %q not found", name)
				return conditions.Ready(
					metav1.ConditionFalse,
					v1.ReasonInvalidReference,
					msg,
					cfg.Generation,
				), nil
			}
			return metav1.Condition{}, err
		}
	default:
		return metav1.Condition{}, fmt.Errorf("spec.type %q has no matching configuration block", cfg.Spec.Type)
	}
	return conditions.Ready(metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed", cfg.Generation), nil
}

// SetupWithManager registers the controller, an index of contracts by
// referenced Secret, an index by same-namespace DatabaseConfig, a
// metadata-only Secret watch, and a typed DatabaseConfig watch.
func (r *SecondaryStorageConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Metrics == nil {
		r.Metrics = observability.Recorder(controllerName)
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.SecondaryStorageConfig{},
		SecretRefsField, func(o client.Object) []string {
			cfg := o.(*v1.SecondaryStorageConfig)
			es := cfg.Spec.Elasticsearch
			if es == nil {
				return nil
			}
			keys := []string{refindex.NamespacedKey(cfg.Namespace, es.CredentialsSecretRef.Name)}
			if es.CASecretRef != nil {
				keys = append(keys, refindex.NamespacedKey(cfg.Namespace, es.CASecretRef.Name))
			}
			return keys
		},
	); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.SecondaryStorageConfig{},
		secondaryStorageConfigDatabaseConfigRefField, func(o client.Object) []string {
			rdbms := o.(*v1.SecondaryStorageConfig).Spec.RDBMS
			if rdbms == nil {
				return nil
			}
			return []string{refindex.NamespacedKey(o.GetNamespace(), rdbms.DatabaseConfigRef)}
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.SecondaryStorageConfig{}).
		Watches(
			&corev1.Secret{},
			refindex.Enqueue(
				mgr.GetClient(), &v1.SecondaryStorageConfigList{},
				SecretRefsField, refindex.ObjectNamespacedName,
			),
			builder.OnlyMetadata,
		).
		Watches(
			&v1.DatabaseConfig{},
			refindex.Enqueue(
				mgr.GetClient(), &v1.SecondaryStorageConfigList{},
				secondaryStorageConfigDatabaseConfigRefField, refindex.ObjectNamespacedName,
			),
		).
		Named(controllerName).
		Complete(r)
}
