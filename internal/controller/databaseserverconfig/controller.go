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

// Package databaseserverconfig validates DatabaseServerConfig contracts and
// maintains their Ready condition. It never provisions anything.
package databaseserverconfig

import (
	"context"
	"fmt"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/pgbootstrap"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

const databaseServerConfigSecretRefsField = "databaseserverconfig.spec.secretRefs"

const (
	// connectionRetryInterval is the wait before the controller retries a
	// server it could not reach. It cannot watch the external server, so a
	// timed requeue is the only trigger.
	connectionRetryInterval = 30 * time.Second
	// probeInterval is how often a reachable server is probed again, so a
	// major upgrade behind the same endpoint reaches status without a spec
	// change.
	probeInterval = 10 * time.Minute
)

// DatabaseServerConfigReconciler validates DatabaseServerConfig contracts. It
// checks the admin credentials Secret reference, reaches the server with those
// credentials, and publishes the major version the server reports. It never
// provisions anything.
type DatabaseServerConfigReconciler struct {
	client.Client
	// APIReader reads directly from the API server and bypasses the cache.
	// Secret data needs it, because Secrets are watched metadata-only.
	APIReader client.Reader
	Scheme    *runtime.Scheme

	// probe reaches the server and reads its major version. Nil means
	// probeServer; the tests count and stub it.
	probe func(ctx context.Context, cfg *v1.DatabaseServerConfig, user, password string) (string, error)
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile validates the contract against the live server and maintains its
// Ready condition and the probed status fields. It never creates or mutates
// other resources. A reconcile that finds the last probe fresh writes nothing
// new, so the status update is a no-op on the server and wakes no watch: the
// probe cadence is the requeue, never a status-write loop.
func (r *DatabaseServerConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var cfg v1.DatabaseServerConfig
	if err := r.Get(ctx, req.NamespacedName, &cfg); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	cond, next, err := r.validate(ctx, &cfg)
	if err != nil {
		return ctrl.Result{}, err
	}

	conditions.Stage(&cfg, cond)

	// The contract owns no components, so its Ready condition follows the
	// server on a conflict and is staged again on the next reconcile.
	if err := component.FlushStatus(
		ctx,
		component.ReconcileContext{Client: r.Client, APIReader: r.APIReader, Owner: &cfg},
		nil,
	); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{RequeueAfter: next}, nil
}

// validate runs the documented checks of the contract and returns the Ready
// condition and when to look again: MissingSecret when the admin credentials
// do not resolve, ConnectionFailed when the server does not answer them
// (retried after connectionRetryInterval), Healthy once the server reported
// its version — which validate records on cfg. A probe that is still fresh
// for the current spec and Secret is not repeated: the recorded Ready stands
// and the requeue is the remaining part of the interval. It returns an error
// only for transient API failures.
func (r *DatabaseServerConfigReconciler) validate(
	ctx context.Context,
	cfg *v1.DatabaseServerConfig,
) (metav1.Condition, time.Duration, error) {
	ref := cfg.Spec.AdminCredentialsSecretRef

	secret, msg, err := secretref.Get(
		ctx, r.APIReader,
		types.NamespacedName{
			Namespace: ref.Namespace,
			Name:      ref.Name,
		},
		ref.UsernameKey, ref.PasswordKey,
	)
	if err != nil {
		return metav1.Condition{}, 0, err
	}
	if msg != "" {
		return conditions.Ready(metav1.ConditionFalse, v1.ReasonMissingSecret, msg, cfg.Generation), 0, nil
	}

	if fresh, remaining := probeIsFresh(cfg, secret.ResourceVersion, time.Now()); fresh {
		return *meta.FindStatusCondition(cfg.Status.Conditions, v1.ConditionReady), remaining, nil
	}

	major, err := r.probeServer(
		ctx, cfg, string(secret.Data[ref.UsernameKey]), string(secret.Data[ref.PasswordKey]),
	)
	if err != nil {
		return conditions.Ready(
			metav1.ConditionFalse,
			v1.ReasonConnectionFailed,
			fmt.Sprintf("Connecting to %s:%d: %v", cfg.Spec.Host, cfg.Spec.Port, err),
			cfg.Generation,
		), connectionRetryInterval, nil
	}

	now := metav1.Now()
	cfg.Status.ServerVersion = major
	cfg.Status.ProbedAt = &now
	cfg.Status.ProbedSecretVersion = secret.ResourceVersion

	return conditions.Ready(
		metav1.ConditionTrue,
		v1.ReasonHealthy,
		"Reached the server; it runs major version "+major,
		cfg.Generation,
	), probeInterval, nil
}

// probeIsFresh reports whether the recorded probe still stands for cfg as it
// is now — a successful probe within the interval, taken for the current
// spec generation with the current Secret — and how long it stands. Anything
// else means the server is probed again: no probe yet, a failed one (which
// records no ProbedAt), a stale one, a spec change since the last reconcile
// (ObservedGeneration lags), or a changed Secret.
func probeIsFresh(cfg *v1.DatabaseServerConfig, secretVersion string, now time.Time) (bool, time.Duration) {
	ready := meta.FindStatusCondition(cfg.Status.Conditions, v1.ConditionReady)
	if cfg.Status.ProbedAt == nil || ready == nil || ready.Status != metav1.ConditionTrue {
		return false, 0
	}
	if cfg.Status.ObservedGeneration != cfg.Generation || ready.ObservedGeneration != cfg.Generation {
		return false, 0
	}
	if cfg.Status.ProbedSecretVersion != secretVersion {
		return false, 0
	}
	age := now.Sub(cfg.Status.ProbedAt.Time)
	if age >= probeInterval {
		return false, 0
	}

	return true, probeInterval - age
}

// probeServer reaches the server through the injected probe, or the default.
func (r *DatabaseServerConfigReconciler) probeServer(
	ctx context.Context, cfg *v1.DatabaseServerConfig, user, password string,
) (string, error) {
	if r.probe != nil {
		return r.probe(ctx, cfg, user, password)
	}

	return probe(ctx, cfg, user, password)
}

// probe opens the admin connection to the server that cfg describes and reads
// the major version it reports. Any failure means the server, as declared,
// is not usable with these credentials.
func probe(ctx context.Context, cfg *v1.DatabaseServerConfig, user, password string) (string, error) {
	admin, err := pgbootstrap.Connect(ctx, pgbootstrap.Connection{
		Host:          cfg.Spec.Host,
		Port:          cfg.Spec.Port,
		AdminUser:     user,
		AdminPassword: password,
	})
	if err != nil {
		return "", err
	}
	defer admin.Close()

	return admin.ServerVersion(ctx)
}

// SetupWithManager registers the controller, an index of CRs by referenced
// Secret, and a metadata-only Secret watch.
func (r *DatabaseServerConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.DatabaseServerConfig{},
		databaseServerConfigSecretRefsField, func(o client.Object) []string {
			ref := o.(*v1.DatabaseServerConfig).Spec.AdminCredentialsSecretRef
			return []string{refindex.NamespacedKey(ref.Namespace, ref.Name)}
		},
	); err != nil {
		return err
	}

	// Status-only updates of the contract — its own flushes above all — wake
	// nothing: the controller reads the spec and the Secret, and its probe
	// cadence is the requeue.
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.DatabaseServerConfig{}, builder.WithPredicates(predicate.GenerationChangedPredicate{})).
		Watches(
			&corev1.Secret{},
			refindex.Enqueue(
				mgr.GetClient(), &v1.DatabaseServerConfigList{},
				databaseServerConfigSecretRefsField, refindex.ObjectNamespacedName,
			),
			builder.OnlyMetadata,
		).
		Named("databaseserverconfig").
		Complete(r)
}
