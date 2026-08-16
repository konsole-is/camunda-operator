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

// Package camundaplatformconfig validates CamundaPlatformConfig resources and
// maintains their Ready condition. It never provisions anything.
package camundaplatformconfig

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

// SecretRefsField is the index field that lists platform configs by the
// Secrets they reference, keyed with refindex.NamespacedKey. Other
// controllers look platform configs up by it when a Secret changes.
const SecretRefsField = "camundaplatformconfig.spec.secretRefs"

// CamundaPlatformConfigReconciler validates CamundaPlatformConfig resources
// and maintains their Ready condition.
type CamundaPlatformConfigReconciler struct {
	client.Client
	// APIReader reads directly from the API server and bypasses the cache.
	// Secret data needs it, because Secrets are watched metadata-only.
	APIReader client.Reader
	Scheme    *runtime.Scheme
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaplatformconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaplatformconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile validates the Secret references of the platform config and
// maintains its Ready condition. It never creates or mutates other resources.
func (r *CamundaPlatformConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cfg v1.CamundaPlatformConfig
	if err := r.Get(ctx, req.NamespacedName, &cfg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cond, err := r.validate(ctx, &cfg)
	if err != nil {
		return ctrl.Result{}, err
	}

	conditions.Stage(&cfg, cond)

	return ctrl.Result{}, component.FlushStatus(ctx, component.ReconcileContext{Client: r.Client, Owner: &cfg})
}

// validate checks every Secret reference of the platform config: the OIDC
// client secret when the method is oidc, and the license when it is set. The
// first missing Secret or key becomes the condition message.
func (r *CamundaPlatformConfigReconciler) validate(
	ctx context.Context,
	cfg *v1.CamundaPlatformConfig,
) (metav1.Condition, error) {
	for _, ref := range secretRefs(cfg) {
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
	}

	return conditions.Ready(metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed", cfg.Generation), nil
}

// secretRefs returns the Secret references that the platform config makes:
// the OIDC client secret when the method is oidc, then the license when set.
func secretRefs(cfg *v1.CamundaPlatformConfig) []v1.SecretKeyRef {
	var refs []v1.SecretKeyRef
	if cfg.Spec.Method() == v1.AuthenticationMethodOIDC && cfg.Spec.Auth.OIDC != nil {
		refs = append(refs, cfg.Spec.Auth.OIDC.ClientSecretRef)
	}
	if cfg.Spec.LicenseSecretRef != nil {
		refs = append(refs, *cfg.Spec.LicenseSecretRef)
	}
	return refs
}

// SetupWithManager registers the controller, an index of platform configs by
// referenced Secret, and a metadata-only Secret watch.
func (r *CamundaPlatformConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.CamundaPlatformConfig{},
		SecretRefsField, func(o client.Object) []string {
			refs := secretRefs(o.(*v1.CamundaPlatformConfig))
			keys := make([]string, 0, len(refs))
			for _, ref := range refs {
				keys = append(keys, refindex.NamespacedKey(ref.Namespace, ref.Name))
			}
			return keys
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.CamundaPlatformConfig{}).
		Watches(
			&corev1.Secret{},
			refindex.Enqueue(
				mgr.GetClient(), &v1.CamundaPlatformConfigList{},
				SecretRefsField, refindex.ObjectNamespacedName,
			),
			builder.OnlyMetadata,
		).
		Named("camundaplatformconfig").
		Complete(r)
}
