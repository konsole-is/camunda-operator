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

// Package database reconciles the Database CR. The controller bootstraps a
// logical database and its roles on an existing PostgreSQL server. It then
// publishes the credential Secrets and the contracts that bind workloads to
// that database.
package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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
	components "github.com/konsole-is/camunda-operator/pkg/components/database"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
	"github.com/konsole-is/camunda-operator/pkg/pgbootstrap"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

// databaseServerRefField indexes Database CRs by spec.serverRef. Events for a
// DatabaseServerConfig or its admin Secret map back to the Databases that
// reference it.
const databaseServerRefField = "database.spec.serverRef"

// databaseCollisionField indexes Database CRs by the server-scoped logical
// database that they claim. The collision rule can then list all claimants
// with one field-indexed query.
const databaseCollisionField = "database.spec.serverDatabase"

// connectionRetryInterval is the wait before the controller retries a server
// whose connection pre-check failed. The controller cannot watch the external
// server, so a timed requeue is the only trigger.
const connectionRetryInterval = 30 * time.Second

// DatabaseReconciler bootstraps a logical database and its users on an
// existing PostgreSQL server over plain SQL. It publishes the credential
// Secrets, the DatabaseConfig, and the optional SecondaryStorageConfig in the
// target namespace of the CR.
type DatabaseReconciler struct {
	client.Client
	// APIReader reads without the cache. The admin Secret and the generated
	// credential Secrets must be read live.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// Recorder publishes the resource events of the component framework.
	// SetupWithManager sets it to the recorder of the manager when it is nil.
	Recorder record.EventRecorder

	// componentClient is the uncached client that the bindings component
	// reconciles with. It keeps the published credential Secrets out of the
	// informer cache, which watches Secrets metadata-only. SetupWithManager
	// sets it when it is nil.
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

// Reconcile converges a Database. The pre-checks resolve the server, its admin
// credentials, the collision rule, and the connection. A failed pre-check
// stops the reconcile and reports its documented Ready reason. The SQL
// bootstrap then converges the logical database, the roles, and the grants.
// This step always runs before the bindings component publishes the
// credential Secrets. A published Secret then never names a password that the
// server does not know.
//
// Status is written once per reconcile. The component and conditions.Stage
// stage conditions on the in-memory Database, and the deferred FlushStatus persists
// them together.
func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	var database v1.Database
	if err := r.Get(ctx, req.NamespacedName, &database); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	rec := component.ReconcileContext{
		Client:   r.componentClient,
		Scheme:   r.Scheme,
		Recorder: r.Recorder,
		Owner:    &database,
	}
	defer func() {
		if flushErr := component.FlushStatus(ctx, rec); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	bootstrapper, err := r.preCheck(ctx, &database)
	var failure *conditions.PreCheckFailure
	if errors.As(err, &failure) {
		conditions.Stage(&database, conditions.Failed(&database, failure))
		if failure.Reason == v1.ReasonConnectionFailed {
			return ctrl.Result{RequeueAfter: connectionRetryInterval}, nil
		}
		return ctrl.Result{}, nil
	}
	if err != nil {
		return ctrl.Result{}, err
	}
	defer bootstrapper.Close()

	rb := components.ResolveBindings(&database)
	if err := r.resolvePasswords(ctx, &rb); err != nil {
		return ctrl.Result{}, err
	}

	if err := bootstrapSQL(ctx, bootstrapper, database.Spec.DatabaseName, rb); err != nil {
		return ctrl.Result{}, err
	}

	comp, err := components.BindingsComponent(&database, rb)
	if err != nil {
		return ctrl.Result{}, err
	}

	reconcileErr := comp.Reconcile(ctx, rec)
	conditions.Stage(&database, conditions.Aggregate(&database, comp))

	return ctrl.Result{}, reconcileErr
}

// resolvePasswords fills the role passwords of rb. It reuses the password of
// an existing published Secret, so credentials stay stable after creation. A
// missing Secret or key yields a new password. To rotate a credential, delete
// its Secret.
func (r *DatabaseReconciler) resolvePasswords(ctx context.Context, rb *components.Bindings) error {
	var err error
	if rb.AppPassword, err = credentials.LookupOrNew(
		ctx,
		r.APIReader,
		rb.AppSecret,
		components.CredentialPasswordKey,
	); err != nil {
		return err
	}

	if !rb.BackupEnabled {
		return nil
	}

	rb.BackupPassword, err = credentials.LookupOrNew(
		ctx,
		r.APIReader,
		rb.BackupSecret,
		components.CredentialPasswordKey,
	)
	return err
}

// bootstrapSQL runs the idempotent bootstrap sequence: the logical database,
// the application role, and, unless disabled, the backup role.
func bootstrapSQL(ctx context.Context, b pgbootstrap.Bootstrapper, name string, rb components.Bindings) error {
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

// preCheck runs the documented pre-checks in order: server reference, admin
// credentials Secret, collision rule, connection. It returns the connected
// Bootstrapper, which the caller closes. A failed check returns a
// *conditions.PreCheckFailure that carries its Ready reason. Any other error
// is a transient API failure.
func (r *DatabaseReconciler) preCheck(ctx context.Context, database *v1.Database) (pgbootstrap.Bootstrapper, error) {
	server, err := r.resolveServer(ctx, database)
	if err != nil {
		return nil, err
	}

	user, password, err := r.adminCredentials(ctx, server)
	if err != nil {
		return nil, err
	}

	if err := r.checkCollision(ctx, database); err != nil {
		return nil, err
	}

	return connect(ctx, server, user, password)
}

// resolveServer fetches the DatabaseServerConfig that spec.serverRef names. A
// dangling reference maps to InvalidReference.
func (r *DatabaseReconciler) resolveServer(
	ctx context.Context, database *v1.Database,
) (*v1.DatabaseServerConfig, error) {
	var server v1.DatabaseServerConfig
	if err := r.Get(ctx, types.NamespacedName{Name: database.Spec.ServerRef}, &server); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &conditions.PreCheckFailure{
				Reason:  v1.ReasonInvalidReference,
				Message: fmt.Sprintf("DatabaseServerConfig %q not found", database.Spec.ServerRef),
			}
		}
		return nil, err
	}

	return &server, nil
}

// adminCredentials checks that the admin credentials Secret of the server has
// the configured keys, then returns the admin username and password. A missing
// Secret or key maps to MissingSecret.
func (r *DatabaseReconciler) adminCredentials(
	ctx context.Context, server *v1.DatabaseServerConfig,
) (user, password string, err error) {
	ref := server.Spec.AdminCredentialsSecretRef
	key := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}

	msg, err := secretref.CheckKeys(ctx, r.APIReader, key, ref.UsernameKey, ref.PasswordKey)
	if err != nil {
		return "", "", err
	}
	if msg != "" {
		return "", "", &conditions.PreCheckFailure{Reason: v1.ReasonMissingSecret, Message: msg}
	}

	var secret corev1.Secret
	if err := r.APIReader.Get(ctx, key, &secret); err != nil {
		return "", "", fmt.Errorf("reading admin credentials Secret %q: %w", key, err)
	}

	return string(secret.Data[ref.UsernameKey]), string(secret.Data[ref.PasswordKey]), nil
}

// checkCollision applies the first-creation-wins rule to the logical database
// that this CR claims. If an older Database claims the same serverRef and
// databaseName, the failure names that winner.
func (r *DatabaseReconciler) checkCollision(ctx context.Context, database *v1.Database) error {
	var list v1.DatabaseList
	if err := r.List(
		ctx,
		&list,
		client.MatchingFields{databaseCollisionField: components.CollisionKey(database)},
	); err != nil {
		return fmt.Errorf("listing databases claiming %q: %w", components.CollisionKey(database), err)
	}

	winner := components.CollisionWinner(list.Items)
	if winner == nil || winner.Name == database.Name {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonInvalidReference,
		Message: fmt.Sprintf(
			"Database %q already claims database %q on server %q",
			winner.Name, database.Spec.DatabaseName, database.Spec.ServerRef,
		),
	}
}

// connect opens the admin connection to server and pings it. Any failure maps
// to ConnectionFailed. The caller closes the returned Bootstrapper.
func connect(
	ctx context.Context, server *v1.DatabaseServerConfig, user, password string,
) (pgbootstrap.Bootstrapper, error) {
	bootstrapper, err := pgbootstrap.Connect(ctx, pgbootstrap.Connection{
		Host:          server.Spec.Host,
		Port:          server.Spec.Port,
		AdminUser:     user,
		AdminPassword: password,
	})
	if err == nil {
		if err = bootstrapper.Ping(ctx); err != nil {
			bootstrapper.Close()
		}
	}
	if err != nil {
		return nil, &conditions.PreCheckFailure{
			Reason:  v1.ReasonConnectionFailed,
			Message: fmt.Sprintf("Connecting to DatabaseServerConfig %q: %v", server.Name, err),
		}
	}

	return bootstrapper, nil
}

// enqueueForAdminSecret maps a Secret event to every Database whose server
// names that Secret as the admin credentials Secret. Server configs are few,
// so the handler scans them directly. The affected Databases resolve through
// the serverRef index.
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

// SetupWithManager registers the controller, the serverRef and collision field
// indexes, and the watches that trigger a reconcile. The watches cover the
// owned bindings, the referenced DatabaseServerConfig, its admin credentials
// Secret (metadata-only), and sibling Databases that contest the same claim.
// It also sets Recorder to the recorder of the manager and builds the uncached
// client for the bindings component.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		// The ReconcileContext of the component framework takes the legacy
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

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.Database{},
		databaseServerRefField, func(o client.Object) []string {
			return []string{o.(*v1.Database).Spec.ServerRef}
		},
	); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.Database{},
		databaseCollisionField, func(o client.Object) []string {
			return []string{components.CollisionKey(o.(*v1.Database))}
		},
	); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Database{}).
		Owns(&corev1.Secret{}, builder.OnlyMetadata).
		Owns(&v1.DatabaseConfig{}).
		Owns(&v1.SecondaryStorageConfig{}).
		Watches(
			&v1.DatabaseServerConfig{},
			refindex.Enqueue(
				mgr.GetClient(), &v1.DatabaseList{},
				databaseServerRefField, refindex.ObjectName,
			),
		).
		Watches(&corev1.Secret{}, r.enqueueForAdminSecret(), builder.OnlyMetadata).
		Watches(
			&v1.Database{},
			refindex.Enqueue(
				mgr.GetClient(), &v1.DatabaseList{},
				databaseCollisionField, func(o client.Object) string {
					return components.CollisionKey(o.(*v1.Database))
				},
			),
		).
		Named("database").
		Complete(r)
}
