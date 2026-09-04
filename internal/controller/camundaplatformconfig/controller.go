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
const controllerName = "camundaplatformconfig"

// SecretRefsField is the index field that lists platform configs by the
// Secrets they reference, keyed with refindex.NamespacedKey. Other
// controllers look platform configs up by it when a Secret changes.
const SecretRefsField = "camundaplatformconfig.spec.secretRefs"

// fieldRef is one Secret reference of a platform config together with the spec
// path that holds it. The message of a MissingSecret condition names that
// path, so the user knows which field to correct.
type fieldRef struct {
	Path string
	v1.SecretKeyRef
}

// CamundaPlatformConfigReconciler validates CamundaPlatformConfig resources
// and maintains their Ready condition.
type CamundaPlatformConfigReconciler struct {
	client.Client
	// APIReader reads directly from the API server and bypasses the cache.
	// Secret data needs it, because Secrets are watched metadata-only.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// Metrics records the condition gauge and the apply counters of the
	// framework. SetupWithManager sets it when it is nil.
	Metrics component.MetricsRecorder
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaplatformconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=camundaplatformconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile validates the Secret references of the platform config and
// maintains its Ready condition. It never creates or mutates other resources.
func (r *CamundaPlatformConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cfg v1.CamundaPlatformConfig
	if err := r.Get(ctx, req.NamespacedName, &cfg); err != nil {
		if apierrors.IsNotFound(err) {
			observability.Forget(r.Metrics, new(v1.CamundaPlatformConfig).GetKind(), req.NamespacedName)
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

// validate checks every Secret reference that secretRefs returns. The first
// missing Secret or key becomes the condition message. The CRD schema ties
// the oidc block to method oidc. An object that passed admission with method
// oidc and no block yields an error, not a condition.
func (r *CamundaPlatformConfigReconciler) validate(
	ctx context.Context,
	cfg *v1.CamundaPlatformConfig,
) (metav1.Condition, error) {
	if cfg.Spec.Method() == v1.AuthenticationMethodOIDC && cfg.Spec.Auth.OIDC == nil {
		return metav1.Condition{}, fmt.Errorf("spec.auth.method %q has no oidc block", cfg.Spec.Method())
	}

	for _, ref := range secretRefs(cfg) {
		msg, err := secretref.CheckKeys(
			ctx, r.APIReader,
			types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}, ref.Key,
		)
		if err != nil {
			return metav1.Condition{}, err
		}
		if msg != "" {
			return conditions.Ready(
				metav1.ConditionFalse, v1.ReasonMissingSecret, ref.Path+": "+msg, cfg.Generation,
			), nil
		}
	}

	return conditions.Ready(metav1.ConditionTrue, v1.ReasonHealthy, "All checks passed", cfg.Generation), nil
}

// SetupWithManager registers the controller, an index of platform configs by
// referenced Secret, and a metadata-only Secret watch.
func (r *CamundaPlatformConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Metrics == nil {
		r.Metrics = observability.Recorder(controllerName)
	}

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
		Named(controllerName).
		Complete(r)
}

// secretRefs is shared by validate and the SecretRefsField indexer so both see the same references.
func secretRefs(cfg *v1.CamundaPlatformConfig) []fieldRef {
	var refs []fieldRef
	if cfg.Spec.Method() == v1.AuthenticationMethodOIDC && cfg.Spec.Auth.OIDC != nil {
		oidc := cfg.Spec.Auth.OIDC
		refs = append(refs, fieldRef{"spec.auth.oidc.clientSecretRef", oidc.ClientSecretRef})
		refs = append(refs, managementClientRefs(oidc.Management)...)
	}
	if cfg.Spec.LicenseSecretRef != nil {
		refs = append(refs, fieldRef{"spec.licenseSecretRef", *cfg.Spec.LicenseSecretRef})
	}
	return refs
}

// managementClientRefs returns the client secrets of the management plane
// clients. The public clients of Console and Web Modeler have none.
func managementClientRefs(mgmt *v1.ManagementOIDCClientsSpec) []fieldRef {
	if mgmt == nil {
		return nil
	}

	const prefix = "spec.auth.oidc.management.clients."

	var refs []fieldRef
	if c := mgmt.Clients.Identity; c != nil {
		refs = append(refs, fieldRef{prefix + "identity.clientSecretRef", c.ClientSecretRef})
	}
	if c := mgmt.Clients.Optimize; c != nil {
		refs = append(refs, fieldRef{prefix + "optimize.clientSecretRef", c.ClientSecretRef})
	}
	if c := mgmt.Clients.WebModelerAPI; c != nil {
		refs = append(refs, fieldRef{prefix + "webModelerApi.clientSecretRef", c.ClientSecretRef})
	}
	return refs
}
