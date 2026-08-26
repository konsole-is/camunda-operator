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
	// server that it did not reach. The controller cannot watch the external
	// server, so a timed requeue is the only trigger.
	connectionRetryInterval = 30 * time.Second
	// probeInterval is how often the controller probes a reachable server
	// again. A major upgrade behind the same endpoint then reaches status
	// without a spec change.
	probeInterval = 10 * time.Minute
	// probeTimeout bounds one probe, the connection and the query together.
	// The reconcile context carries no deadline, and connect_timeout bounds
	// only the handshake. A server that accepts the connection and then
	// stalls must not hold the worker, because one stalled server would
	// then block every other DatabaseServerConfig.
	probeTimeout = 30 * time.Second
)

// DatabaseServerConfigReconciler validates DatabaseServerConfig contracts. It
// checks the admin credentials Secret reference, reaches the server with those
// credentials, and publishes the major version and the system identifier that
// the server reports. It never provisions anything.
type DatabaseServerConfigReconciler struct {
	client.Client
	// APIReader reads directly from the API server and bypasses the cache.
	// Secret data needs it, because Secrets are watched metadata-only.
	APIReader client.Reader
	Scheme    *runtime.Scheme

	// probe reaches the server and reads its major version and its system
	// identifier. Nil means probeServer. The tests count and stub it.
	probe func(
		ctx context.Context, cfg *v1.DatabaseServerConfig, user, password string,
	) (version, systemIdentifier string, err error)
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

// Reconcile validates the contract against the live server and maintains its
// Ready condition and the probed status fields. It never creates or mutates
// other resources. If the last probe is fresh, the reconcile writes nothing
// new. The status update is then a no-op on the server and wakes no watch.
// The requeue sets the probe cadence, not a status-write loop.
func (r *DatabaseServerConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Cached, not live. The freshness of the last probe is the one thing this
	// controller reads back from its own status write, and a stale copy only
	// costs a probe it could have skipped. Nothing terminal hangs on it, a
	// lost status write is taken again on the probe timer, and the controller
	// records no event that a second look repeats.
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
// condition and the time until the next check. If the admin credentials do
// not resolve, the reason is MissingSecret. If the server does not answer
// them, the reason is ConnectionFailed and the next check is after
// connectionRetryInterval. When the server answers, the reason is Healthy and
// validate records the version and the system identifier on cfg. If the probe is still
// fresh for the current spec and Secret, validate does not repeat it. The
// recorded Ready stands and the requeue is the remaining part of the
// interval. It returns an error only for transient API failures.
func (r *DatabaseServerConfigReconciler) validate(
	ctx context.Context,
	cfg *v1.DatabaseServerConfig,
) (metav1.Condition, time.Duration, error) {
	// A spec that names another server describes another server until the next
	// probe says otherwise. The recorded version and identity belong to the
	// server the old spec named, and a consumer that keyed on the identity
	// would place the database on a server nothing reached. The whole record
	// of the probe goes, so status reads as no probe for this spec rather than
	// a probe of a server this contract no longer describes.
	//
	// The test is the endpoint and the admin credentials, not the generation.
	// Every field of this spec is writable, and the ones that carry a recovery
	// request and its answer are written on a live contract while a rollback
	// runs. Clearing the identity for those takes the contract, and every
	// Database on it, out of Ready for a write that cannot move the server.
	if probedAnotherServer(cfg) {
		cfg.Status.ServerVersion = ""
		cfg.Status.SystemIdentifier = ""
		cfg.Status.ProbedAt = nil
		cfg.Status.ProbedEndpoint = ""
		cfg.Status.ProbedSecretName = ""
		cfg.Status.ProbedSecretKeys = ""
		cfg.Status.ProbedSecretVersion = ""
	}

	ref := cfg.Spec.AdminCredentialsSecretRef

	secret, msg, err := secretref.Get(
		ctx, r.APIReader,
		types.NamespacedName{
			Namespace: cfg.Namespace,
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
		// The recorded answer stands for this spec too, so it is re-stated for
		// the generation of this spec. A consumer that waits on Ready reads
		// the generation to tell an answer about the spec it holds from an
		// answer about the one before it.
		stands := *meta.FindStatusCondition(cfg.Status.Conditions, v1.ConditionReady)
		stands.ObservedGeneration = cfg.Generation

		return stands, remaining, nil
	}

	major, systemIdentifier, err := r.probeServer(
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
	cfg.Status.SystemIdentifier = systemIdentifier
	cfg.Status.ProbedAt = &now
	cfg.Status.ProbedEndpoint = probedEndpoint(cfg)
	cfg.Status.ProbedSecretName = ref.Name
	cfg.Status.ProbedSecretKeys = probedSecretKeys(cfg)
	cfg.Status.ProbedSecretVersion = secret.ResourceVersion

	return conditions.Ready(
		metav1.ConditionTrue,
		v1.ReasonHealthy,
		"Reached the server; it runs major version "+major,
		cfg.Generation,
	), probeInterval, nil
}

// probedAnotherServer reports whether the record of the last probe describes a
// server, or a user on it, that this spec no longer names. A contract that has
// never been probed has nothing to clear.
func probedAnotherServer(cfg *v1.DatabaseServerConfig) bool {
	return cfg.Status.ProbedAt != nil && !cfg.ProbedForCurrentSpec()
}

// probedEndpoint renders the endpoint of the spec as status records it.
func probedEndpoint(cfg *v1.DatabaseServerConfig) string {
	return fmt.Sprintf("%s:%d", cfg.Spec.Host, cfg.Spec.Port)
}

// probedSecretKeys renders the credential keys of the spec as status records
// them. One Secret can hold the credentials of more than one user, so a spec
// that reads other keys of one Secret reads another user.
func probedSecretKeys(cfg *v1.DatabaseServerConfig) string {
	ref := cfg.Spec.AdminCredentialsSecretRef

	return ref.UsernameKey + "/" + ref.PasswordKey
}

// probeIsFresh reports whether the recorded probe still stands for cfg as it
// is now, and for how long. A probe stands when it succeeded within the
// interval, against the endpoint and the Secret that the spec names now. In
// every other case the controller probes the server again. That is the case
// when there is no probe yet, or the last probe failed (a failed probe records
// no ProbedAt). It is also the case when the probe is stale, the spec names
// another server or another user, or the Secret changed.
func probeIsFresh(cfg *v1.DatabaseServerConfig, secretVersion string, now time.Time) (bool, time.Duration) {
	ready := meta.FindStatusCondition(cfg.Status.Conditions, v1.ConditionReady)
	if cfg.Status.ProbedAt == nil || ready == nil || ready.Status != metav1.ConditionTrue {
		return false, 0
	}
	if probedAnotherServer(cfg) {
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
) (version, systemIdentifier string, err error) {
	if r.probe != nil {
		return r.probe(ctx, cfg, user, password)
	}

	return probe(ctx, cfg, user, password)
}

// probe opens the admin connection to the server that cfg describes and reads
// the major version and the system identifier that it reports. Any failure
// means that the server, as declared, is not usable with these credentials.
// The whole probe ends within probeTimeout.
func probe(
	ctx context.Context, cfg *v1.DatabaseServerConfig, user, password string,
) (version, systemIdentifier string, err error) {
	return probeWithin(ctx, probeTimeout, cfg, user, password)
}

// probeWithin is probe with an explicit deadline for the connection and the
// queries together.
func probeWithin(
	ctx context.Context,
	timeout time.Duration,
	cfg *v1.DatabaseServerConfig,
	user, password string,
) (version, systemIdentifier string, err error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	admin, err := pgbootstrap.Connect(ctx, pgbootstrap.Connection{
		Host:     cfg.Spec.Host,
		Port:     cfg.Spec.Port,
		User:     user,
		Password: password,
	})
	if err != nil {
		return "", "", err
	}
	defer admin.Close()

	version, err = admin.ServerVersion(ctx)
	if err != nil {
		return "", "", err
	}

	systemIdentifier, err = admin.SystemIdentifier(ctx)
	if err != nil {
		return "", "", err
	}

	return version, systemIdentifier, nil
}

// SetupWithManager registers the controller, an index of CRs by referenced
// Secret, and a metadata-only Secret watch.
func (r *DatabaseServerConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.DatabaseServerConfig{},
		databaseServerConfigSecretRefsField, func(o client.Object) []string {
			cfg := o.(*v1.DatabaseServerConfig)
			return []string{
				refindex.NamespacedKey(cfg.Namespace, cfg.Spec.AdminCredentialsSecretRef.Name),
			}
		},
	); err != nil {
		return err
	}

	// Status-only updates of the contract wake nothing, its own flushes
	// included. The controller reads the spec and the Secret, and the requeue
	// sets its probe cadence.
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
