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

// Package databaseconfig validates DatabaseConfig contracts and
// maintains their Ready condition. It never provisions anything.
package databaseconfig

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
const controllerName = "databaseconfig"

// SecretRefsField is the index field that lists DatabaseConfigs by the
// Secrets they reference, keyed with refindex.NamespacedKey. Other
// controllers look DatabaseConfigs up by it when a Secret changes.
const SecretRefsField = "databaseconfig.spec.secretRefs"

const databaseConfigServerRefField = "databaseconfig.spec.serverRef"

// DatabaseConfigReconciler validates DatabaseConfig contracts and maintains
// their Ready condition.
type DatabaseConfigReconciler struct {
	client.Client
	// APIReader reads directly from the API server and bypasses the cache.
	// Secret data needs it, because Secrets are watched metadata-only.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// Metrics records the condition gauge and the apply counters of the
	// framework. SetupWithManager sets it when it is nil.
	Metrics component.MetricsRecorder
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile validates the references of the contract and maintains its Ready
// condition. It never creates or mutates other resources.
func (r *DatabaseConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cfg v1.DatabaseConfig
	if err := r.Get(ctx, req.NamespacedName, &cfg); err != nil {
		if apierrors.IsNotFound(err) {
			observability.Forget(r.Metrics, new(v1.DatabaseConfig).GetKind(), req.NamespacedName)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
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

// validate runs the documented checks of the contract in order: the server
// reference first, then each credentials Secret. It returns the first failure
// as the Ready condition.
func (r *DatabaseConfigReconciler) validate(ctx context.Context, cfg *v1.DatabaseConfig) (metav1.Condition, error) {
	serverKey := types.NamespacedName{Namespace: cfg.Namespace, Name: cfg.Spec.ServerRef}

	var server v1.DatabaseServerConfig
	if err := r.Get(ctx, serverKey, &server); err != nil {
		if apierrors.IsNotFound(err) {
			msg := fmt.Sprintf("DatabaseServerConfig %s not found", serverKey)
			return conditions.Ready(metav1.ConditionFalse, v1.ReasonInvalidReference, msg, cfg.Generation), nil
		}
		return metav1.Condition{}, err
	}

	refs := []v1.LocalCredentialsSecretRef{cfg.Spec.CredentialsSecretRef}
	if cfg.Spec.BackupCredentialsSecretRef != nil {
		refs = append(refs, *cfg.Spec.BackupCredentialsSecretRef)
	}

	for _, ref := range refs {
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
	}

	return conditions.Ready(metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed", cfg.Generation), nil
}

// SetupWithManager registers the controller, an index of CRs by referenced
// Secret, an index by referenced DatabaseServerConfig, a metadata-only Secret
// watch, and a typed DatabaseServerConfig watch.
func (r *DatabaseConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Metrics == nil {
		r.Metrics = observability.Recorder(controllerName)
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.DatabaseConfig{},
		SecretRefsField, func(o client.Object) []string {
			cfg := o.(*v1.DatabaseConfig)
			keys := []string{
				refindex.NamespacedKey(cfg.Namespace, cfg.Spec.CredentialsSecretRef.Name),
			}
			if backup := cfg.Spec.BackupCredentialsSecretRef; backup != nil {
				keys = append(keys, refindex.NamespacedKey(cfg.Namespace, backup.Name))
			}
			return keys
		},
	); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.DatabaseConfig{},
		databaseConfigServerRefField, func(o client.Object) []string {
			cfg := o.(*v1.DatabaseConfig)
			return []string{refindex.NamespacedKey(cfg.Namespace, cfg.Spec.ServerRef)}
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.DatabaseConfig{}).
		Watches(
			&corev1.Secret{},
			refindex.Enqueue(
				mgr.GetClient(), &v1.DatabaseConfigList{},
				SecretRefsField, refindex.ObjectNamespacedName,
			),
			builder.OnlyMetadata,
		).
		Watches(
			&v1.DatabaseServerConfig{},
			refindex.Enqueue(
				mgr.GetClient(), &v1.DatabaseConfigList{},
				databaseConfigServerRefField, refindex.ObjectNamespacedName,
			),
		).
		Named(controllerName).
		Complete(r)
}
