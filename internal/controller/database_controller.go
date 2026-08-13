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
	"errors"
	"fmt"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
	"github.com/konsole-is/camunda-operator/pkg/pgbootstrap"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

// databaseServerRefField indexes Database CRs by their spec.serverRef, so
// DatabaseServerConfig and admin-Secret events map back to the Databases they
// affect.
const databaseServerRefField = "database.spec.serverRef"

// connectionRetryInterval is how long the controller waits before retrying a
// server whose connection pre-check failed; the external server cannot be
// watched, so a timed requeue is the only re-trigger.
const connectionRetryInterval = 30 * time.Second

// DatabaseReconciler bootstraps a logical database and its users on an
// existing PostgreSQL server over plain SQL and publishes the credential
// Secrets, DatabaseConfig, and optional SecondaryStorageConfig bindings in
// the CR's target namespace.
type DatabaseReconciler struct {
	client.Client
	// APIReader reads uncached, for admin and generated credential Secrets
	// whose data must be read live.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// Recorder publishes the component framework's resource events; it
	// defaults to the manager's recorder in SetupWithManager.
	Recorder record.EventRecorder

	// componentClient is the uncached client the bindings component
	// reconciles with; it keeps the published credential Secrets out of the
	// informer cache, which watches Secrets metadata-only. Defaulted in
	// SetupWithManager.
	componentClient client.Client
}

// +kubebuilder:rbac:groups=core.camunda.io,resources=databases,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=databases/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databases/finalizers,verbs=update
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseserverconfigs,verbs=get;list;watch
// +kubebuilder:rbac:groups=core.camunda.io,resources=databaseconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core.camunda.io,resources=secondarystorageconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=events,verbs=create;patch

// Reconcile converges a Database: pre-checks resolve the server, its admin
// credentials, the collision rule, and connectivity; a failed pre-check
// short-circuits into the documented Ready reason. The SQL bootstrap then
// converges the logical database, roles, and grants — always before the
// bindings component publishes the credential Secrets, so a published Secret
// never names a password the server does not know.
func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	var database v1.Database
	if err := r.Get(ctx, req.NamespacedName, &database); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	bootstrapper, failure, err := r.preCheck(ctx, &database)
	if err != nil {
		return ctrl.Result{}, err
	}
	if failure != nil {
		cond := conditions.Ready(metav1.ConditionFalse, failure.Reason, failure.Message, database.Generation)
		if err := conditions.PatchReady(ctx, r.Client, &database, cond); err != nil {
			return ctrl.Result{}, err
		}
		if failure.Reason == conditions.ReasonConnectionFailed {
			return ctrl.Result{RequeueAfter: connectionRetryInterval}, nil
		}
		return ctrl.Result{}, nil
	}
	defer bootstrapper.Close()

	rb := resolveBindings(&database)
	if err := r.resolvePasswords(ctx, &rb); err != nil {
		return ctrl.Result{}, err
	}

	if err := bootstrapSQL(ctx, bootstrapper, &database, rb); err != nil {
		return ctrl.Result{}, err
	}

	comp, err := databaseBindingsComponent(&database, rb)
	if err != nil {
		return ctrl.Result{}, err
	}

	rec := component.ReconcileContext{
		Client:   r.componentClient,
		Scheme:   r.Scheme,
		Recorder: r.Recorder,
		Owner:    &database,
	}

	// The component stages its condition on the in-memory owner; FlushStatus
	// persists the staged conditions, and only then is the CR-level Ready
	// derived and SSA-patched — the reverse order would let the flush's full
	// status update clobber the freshly patched Ready.
	reconcileErr := comp.Reconcile(ctx, rec)
	flushErr := component.FlushStatus(ctx, rec)
	patchErr := conditions.PatchReady(ctx, r.Client, &database, r.deriveReady(&database))

	return ctrl.Result{}, errors.Join(reconcileErr, flushErr, patchErr)
}

// resolvePasswords fills rb's role passwords: an existing published Secret's
// password is reused so credentials stay stable once created, and a missing
// Secret or key yields a newly generated password — deleting a credential
// Secret is the rotation mechanism.
func (r *DatabaseReconciler) resolvePasswords(ctx context.Context, rb *resolvedBindings) error {
	appPassword, found, err := credentials.Lookup(ctx, r.APIReader, rb.AppSecret, credentialPasswordKey)
	if err != nil {
		return err
	}
	if !found {
		if appPassword, err = credentials.NewPassword(); err != nil {
			return err
		}
	}
	rb.AppPassword = appPassword

	if !rb.BackupEnabled {
		return nil
	}

	backupPassword, found, err := credentials.Lookup(ctx, r.APIReader, rb.BackupSecret, credentialPasswordKey)
	if err != nil {
		return err
	}
	if !found {
		if backupPassword, err = credentials.NewPassword(); err != nil {
			return err
		}
	}
	rb.BackupPassword = backupPassword

	return nil
}

// bootstrapSQL runs the idempotent bootstrap sequence for the logical
// database, the application role, and — unless disabled — the backup role.
func bootstrapSQL(
	ctx context.Context, b pgbootstrap.Bootstrapper, database *v1.Database, rb resolvedBindings,
) error {
	name := database.Spec.DatabaseName

	if err := b.EnsureDatabase(ctx, name); err != nil {
		return fmt.Errorf("ensuring database %q: %w", name, err)
	}

	if err := b.EnsureUser(ctx, rb.AppUser, rb.AppPassword); err != nil {
		return fmt.Errorf("ensuring application role %q: %w", rb.AppUser, err)
	}
	if err := b.GrantApplication(ctx, rb.AppUser, name); err != nil {
		return fmt.Errorf("granting application role %q on %q: %w", rb.AppUser, name, err)
	}

	if !rb.BackupEnabled {
		return nil
	}

	if err := b.EnsureBackupUser(ctx, rb.BackupUser, rb.BackupPassword, name); err != nil {
		return fmt.Errorf("ensuring backup role %q on %q: %w", rb.BackupUser, name, err)
	}

	return nil
}

// deriveReady maps the staged component conditions to the CR-level Ready
// condition at the current generation.
func (r *DatabaseReconciler) deriveReady(database *v1.Database) metav1.Condition {
	componentConds := make([]metav1.Condition, 0, len(database.Status.Conditions))
	for _, cond := range database.Status.Conditions {
		if cond.Type != conditions.TypeReady {
			componentConds = append(componentConds, cond)
		}
	}
	if len(componentConds) == 0 {
		componentConds = []metav1.Condition{{
			Type:    string(databaseBindingsConditionType),
			Status:  metav1.ConditionUnknown,
			Message: "not yet reported",
		}}
	}

	reason, message := conditions.DeriveReady(nil, componentConds, false)
	status := metav1.ConditionFalse
	if reason == conditions.ReasonHealthy {
		status = metav1.ConditionTrue
	}

	return conditions.Ready(status, reason, message, database.Generation)
}

// preCheck runs the documented pre-checks in order — server reference, admin
// credentials Secret, collision rule, connectivity — and returns either the
// connected Bootstrapper (the caller closes it) or the first failure mapped
// to its Ready reason. An error is returned only for transient API failures.
func (r *DatabaseReconciler) preCheck(
	ctx context.Context, database *v1.Database,
) (pgbootstrap.Bootstrapper, *conditions.PreCheckFailure, error) {
	server, failure, err := r.resolveServer(ctx, database)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	user, password, failure, err := r.adminCredentials(ctx, server)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	failure, err = r.checkCollision(ctx, database)
	if err != nil || failure != nil {
		return nil, failure, err
	}

	bootstrapper, err := pgbootstrap.Connect(ctx, pgbootstrap.Connection{
		Host:          server.Spec.Host,
		Port:          server.Spec.Port,
		AdminUser:     user,
		AdminPassword: password,
	})
	if err == nil {
		err = bootstrapper.Ping(ctx)
		if err != nil {
			bootstrapper.Close()
		}
	}
	if err != nil {
		return nil, &conditions.PreCheckFailure{
			Reason:  conditions.ReasonConnectionFailed,
			Message: fmt.Sprintf("Connecting to DatabaseServerConfig %q: %v", server.Name, err),
		}, nil
	}

	return bootstrapper, nil, nil
}

// resolveServer fetches the DatabaseServerConfig named by spec.serverRef,
// mapping a dangling reference to InvalidReference.
func (r *DatabaseReconciler) resolveServer(
	ctx context.Context, database *v1.Database,
) (*v1.DatabaseServerConfig, *conditions.PreCheckFailure, error) {
	var server v1.DatabaseServerConfig
	if err := r.Get(ctx, types.NamespacedName{Name: database.Spec.ServerRef}, &server); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &conditions.PreCheckFailure{
				Reason:  conditions.ReasonInvalidReference,
				Message: fmt.Sprintf("DatabaseServerConfig %q not found", database.Spec.ServerRef),
			}, nil
		}
		return nil, nil, err
	}

	return &server, nil, nil
}

// adminCredentials checks the server's admin credentials Secret for the
// configured keys — mapping a missing Secret or key to MissingSecret — and
// returns the admin username and password.
func (r *DatabaseReconciler) adminCredentials(
	ctx context.Context, server *v1.DatabaseServerConfig,
) (string, string, *conditions.PreCheckFailure, error) {
	ref := server.Spec.AdminCredentialsSecretRef
	key := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}

	msg, err := secretref.CheckKeys(ctx, r.APIReader, key, ref.UsernameKey, ref.PasswordKey)
	if err != nil {
		return "", "", nil, err
	}
	if msg != "" {
		return "", "", &conditions.PreCheckFailure{Reason: conditions.ReasonMissingSecret, Message: msg}, nil
	}

	var secret corev1.Secret
	if err := r.APIReader.Get(ctx, key, &secret); err != nil {
		return "", "", nil, fmt.Errorf("reading admin credentials Secret %q: %w", key, err)
	}

	return string(secret.Data[ref.UsernameKey]), string(secret.Data[ref.PasswordKey]), nil, nil
}

// checkCollision enforces the first-creation-wins rule for the logical
// database this CR claims: when another, older Database claims the same
// serverRef and databaseName, the failure names the winner.
func (r *DatabaseReconciler) checkCollision(
	ctx context.Context, database *v1.Database,
) (*conditions.PreCheckFailure, error) {
	var list v1.DatabaseList
	if err := r.List(ctx, &list, client.MatchingFields{databaseCollisionField: collisionKey(database)}); err != nil {
		return nil, fmt.Errorf("listing databases claiming %q: %w", collisionKey(database), err)
	}

	winner := collisionWinner(list.Items)
	if winner == nil || winner.Name == database.Name {
		return nil, nil
	}

	return &conditions.PreCheckFailure{
		Reason: conditions.ReasonInvalidReference,
		Message: fmt.Sprintf(
			"Database %q already claims database %q on server %q",
			winner.Name, database.Spec.DatabaseName, database.Spec.ServerRef,
		),
	}, nil
}

// enqueueForAdminSecret maps a Secret event to every Database whose
// referenced server names it as the admin credentials Secret. Server configs
// are few, so they are scanned directly; the affected Databases resolve
// through the serverRef index.
func (r *DatabaseReconciler) enqueueForAdminSecret() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		var servers v1.DatabaseServerConfigList
		if err := r.List(ctx, &servers); err != nil {
			logf.FromContext(ctx).Error(err, "listing database server configs for admin Secret enqueue")
			return nil
		}

		var reqs []reconcile.Request
		for _, server := range servers.Items {
			ref := server.Spec.AdminCredentialsSecretRef
			if ref.Namespace != o.GetNamespace() || ref.Name != o.GetName() {
				continue
			}

			var databases v1.DatabaseList
			if err := r.List(ctx, &databases, client.MatchingFields{databaseServerRefField: server.Name}); err != nil {
				logf.FromContext(ctx).Error(err, "listing databases for admin Secret enqueue", "server", server.Name)
				continue
			}
			for _, database := range databases.Items {
				reqs = append(reqs, reconcile.Request{NamespacedName: types.NamespacedName{Name: database.Name}})
			}
		}
		return reqs
	})
}

// SetupWithManager registers the controller, the serverRef and collision
// field indexes, and the watches that re-trigger reconciliation: the owned
// bindings, the referenced DatabaseServerConfig, its admin credentials Secret
// (metadata-only), and sibling Databases contesting the same claim. It also
// defaults Recorder to the manager's recorder and builds the uncached client
// the bindings component reconciles with.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		// The component framework's ReconcileContext takes the legacy
		// record.EventRecorder, so the deprecated accessor is required here.
		r.Recorder = mgr.GetEventRecorderFor("database-controller") //nolint:staticcheck
	}

	if r.componentClient == nil {
		componentClient, err := client.New(mgr.GetConfig(), client.Options{
			Scheme: mgr.GetScheme(),
			Mapper: mgr.GetRESTMapper(),
		})
		if err != nil {
			return fmt.Errorf("building the component client: %w", err)
		}
		r.componentClient = componentClient
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1.Database{},
		databaseServerRefField, func(o client.Object) []string {
			return []string{o.(*v1.Database).Spec.ServerRef}
		}); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &v1.Database{},
		databaseCollisionField, func(o client.Object) []string {
			return []string{collisionKey(o.(*v1.Database))}
		}); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Database{}).
		Owns(&corev1.Secret{}, builder.OnlyMetadata).
		Owns(&v1.DatabaseConfig{}).
		Owns(&v1.SecondaryStorageConfig{}).
		Watches(&v1.DatabaseServerConfig{},
			refindex.Enqueue(mgr.GetClient(), &v1.DatabaseList{},
				databaseServerRefField, refindex.ObjectName)).
		Watches(&corev1.Secret{}, r.enqueueForAdminSecret(), builder.OnlyMetadata).
		Watches(&v1.Database{},
			refindex.Enqueue(mgr.GetClient(), &v1.DatabaseList{},
				databaseCollisionField, func(o client.Object) string {
					return collisionKey(o.(*v1.Database))
				})).
		Named("database").
		Complete(r)
}
