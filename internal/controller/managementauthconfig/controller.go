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

// Package managementauthconfig validates ManagementAuthConfig contracts and
// maintains their Ready condition. It never provisions anything.
package managementauthconfig

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
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

// SecretRefsField is the index field that lists contracts by the Secrets they
// reference, keyed by "<namespace>/<name>". A consumer of the contract watches
// those Secrets through it.
const SecretRefsField = "managementauthconfig.spec.secretRefs"

// ManagementAuthConfigReconciler validates ManagementAuthConfig contracts and
// maintains their Ready condition.
type ManagementAuthConfigReconciler struct {
	client.Client
	// APIReader reads directly from the API server and bypasses the cache.
	// Secret data needs it, because Secrets are watched metadata-only.
	APIReader client.Reader
	Scheme    *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=managementauthconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=managementauthconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile validates the references of the contract and maintains its Ready
// condition. It never creates or mutates other resources.
func (r *ManagementAuthConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cfg v1.ManagementAuthConfig
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
		component.ReconcileContext{Client: r.Client, APIReader: r.APIReader, Owner: &cfg},
		nil,
	)
}

// validate checks the machine-to-machine client secret reference. The CRD
// schema validates the endpoint URLs, and validate never probes them.
func (r *ManagementAuthConfigReconciler) validate(
	ctx context.Context,
	cfg *v1.ManagementAuthConfig,
) (metav1.Condition, error) {
	ref := cfg.Spec.ClientSecretRef
	msg, err := secretref.CheckKeys(
		ctx, r.APIReader,
		types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, ref.Key,
	)
	if err != nil {
		return metav1.Condition{}, err
	}
	if msg != "" {
		return conditions.Ready(metav1.ConditionFalse, v1.ReasonMissingSecret, msg, cfg.Generation), nil
	}

	return conditions.Ready(metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed", cfg.Generation), nil
}

// SetupWithManager registers the controller, an index of CRs by referenced
// Secret, and a metadata-only Secret watch.
func (r *ManagementAuthConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.ManagementAuthConfig{},
		SecretRefsField, func(o client.Object) []string {
			ref := o.(*v1.ManagementAuthConfig).Spec.ClientSecretRef
			return []string{refindex.NamespacedKey(ref.Namespace, ref.Name)}
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.ManagementAuthConfig{}).
		Watches(
			&corev1.Secret{},
			refindex.Enqueue(
				mgr.GetClient(), &v1.ManagementAuthConfigList{},
				SecretRefsField, refindex.ObjectNamespacedName,
			),
			builder.OnlyMetadata,
		).
		Named("managementauthconfig").
		Complete(r)
}
