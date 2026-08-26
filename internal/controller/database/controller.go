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
	"strings"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

// databaseServerRefField indexes Database CRs by the DatabaseServerConfig
// they reference, keyed as "<namespace>/<serverRef>". Events for a
// DatabaseServerConfig or its admin Secret map back to the Databases that
// reference it.
const databaseServerRefField = "database.spec.serverRef"

// connectionRetryInterval is the wait before the controller retries a server
// whose connection pre-check failed. The controller cannot watch the external
// server, so a timed requeue is the only trigger.
const connectionRetryInterval = 30 * time.Second

// DatabaseReconciler bootstraps a logical database and its users on an
// existing PostgreSQL server over plain SQL. It publishes the credential
// Secrets, the DatabaseConfig, and the optional SecondaryStorageConfig in the
// namespace of the CR.
type DatabaseReconciler struct {
	client.Client
	// APIReader reads without the cache. The admin Secret and the generated
	// credential Secrets must be read live.
	APIReader client.Reader
	Scheme    *runtime.Scheme
	// EventRecorder publishes the resource events of the component framework.
	// SetupWithManager sets it to the recorder of the manager when it is nil.
	EventRecorder events.EventRecorder
	// ClaimNamespace holds the claim Leases of every logical database. One
	// claim crosses namespaces, so every claimant meets on a Lease of this
	// one namespace. It is the namespace of the operator, and
	// SetupWithManager refuses an empty one.
	ClaimNamespace string

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
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete

// Reconcile converges a Database. The pre-checks resolve the server, its
// identity, its admin credentials, the claim, and the connection. A failed
// pre-check stops the reconcile and reports its documented Ready reason. The
// SQL bootstrap then converges the logical database, the roles, and the
// grants.
// This step always runs before the bindings component publishes the
// credential Secrets. A published Secret then never names a password that the
// server does not know.
//
// Status is written once per reconcile. The component and conditions.Stage
// stage conditions on the in-memory Database, and the deferred FlushStatus persists
// them together.
func (r *DatabaseReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, err error) {
	// Cached, not live. No step here is keyed on the last status write of
	// this controller: the collision key is derived again from the identity
	// the contract reports, and the bootstrap converges what the server
	// holds. A status write this reconcile loses is written again on the next
	// look, and the controller records no event that a second look repeats.
	var database v1.Database
	if err := r.Get(ctx, req.NamespacedName, &database); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// A deleted Database publishes no status. Its bindings go with the owner
	// references, and the finalizer gives the claim back.
	if !database.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.finalize(ctx, &database)
	}

	// The finalizer must exist before the first claim. A deletion between the
	// claim and the next write would leave a logical database claimed by an
	// owner that is gone.
	if controllerutil.AddFinalizer(&database, ClaimFinalizer) {
		if err := r.Update(ctx, &database); err != nil {
			// A deletion that races this write is fine. The deletion path owns
			// the object from here.
			if apierrors.IsNotFound(err) {
				return ctrl.Result{}, nil
			}

			return ctrl.Result{}, fmt.Errorf("adding the claim finalizer: %w", err)
		}
	}

	rec := component.ReconcileContext{
		Client:        r.componentClient,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
		APIReader:     r.APIReader,
		Owner:         &database,
	}
	// Declared before the deferred flush, so the closure sees the component
	// that the reconcile builds below and FlushStatus owns its condition.
	var comps []*component.Component
	defer func() {
		if flushErr := component.FlushStatus(ctx, rec, comps); flushErr != nil {
			err = errors.Join(err, flushErr)
		}
	}()

	bootstrapper, err := r.preCheck(ctx, &database)
	var failure *conditions.PreCheckFailure
	if errors.As(err, &failure) {
		if errors.Is(err, errClaimLost) {
			kept, withdrawErr := r.withdrawBindings(ctx, rec, &database, &comps)
			if len(kept) > 0 {
				failure.Message += fmt.Sprintf(
					". These bindings belong to another Database and stay in place: %s",
					strings.Join(kept, ", "),
				)
			}
			conditions.Stage(&database, conditions.Failed(&database, failure))
			if withdrawErr != nil {
				return ctrl.Result{}, withdrawErr
			}

			// The withdrawal leaves nothing that names the logical database
			// this Database recorded before, so it gives that claim back. A
			// Database that kept it would hold a name it no longer uses.
			if dropErr := r.dropClaim(
				ctx, database.Status.CollisionKey, selfHolder(&database),
			); dropErr != nil {
				return ctrl.Result{}, dropErr
			}

			return ctrl.Result{RequeueAfter: claimRetryInterval}, nil
		}

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
	comps = []*component.Component{comp}

	reconcileErr := comp.Reconcile(ctx, rec)
	conditions.Stage(&database, conditions.Aggregate(&database, comp))

	return ctrl.Result{}, reconcileErr
}

// withdrawBindings deletes the bindings that database owns and sets comps to
// the component that did it, so the one status flush of this reconcile owns
// BindingsReady and reports it as Disabled. A Database that lost its claim
// keeps neither the contracts nor the credential Secrets it published.
//
// It returns the bindings it left in place. Two Databases can name one binding
// explicitly, and only the one that owns it may delete it, so a loser that
// names the winner's object withdraws everything else and reports that name.
func (r *DatabaseReconciler) withdrawBindings(
	ctx context.Context,
	rec component.ReconcileContext,
	database *v1.Database,
	comps *[]*component.Component,
) ([]string, error) {
	owned, kept, err := r.ownedBindings(ctx, database)
	if err != nil {
		return nil, err
	}

	comp, err := components.WithdrawnBindingsComponent(database, owned)
	if err != nil {
		return kept, err
	}
	*comps = []*component.Component{comp}

	return kept, comp.Reconcile(ctx, rec)
}

// ownedBindings reports which published bindings of database carry a
// controller owner reference to it, and names the ones that do not.
func (r *DatabaseReconciler) ownedBindings(
	ctx context.Context, database *v1.Database,
) (components.OwnedBindings, []string, error) {
	rb := components.ResolveBindings(database)

	// One entry per binding the component can register. The order decides
	// only the order of the names in the message.
	type binding struct {
		key   types.NamespacedName
		obj   client.Object
		owned *bool
	}

	var owned components.OwnedBindings
	bindings := []binding{
		{rb.AppSecret, &corev1.Secret{}, &owned.AppSecret},
		{rb.BackupSecret, &corev1.Secret{}, &owned.BackupSecret},
		{
			types.NamespacedName{Namespace: database.Namespace, Name: rb.DatabaseConfigName},
			&v1.DatabaseConfig{},
			&owned.DatabaseConfig,
		},
	}
	if database.Spec.SecondaryStorageConfig != "" {
		bindings = append(bindings, binding{
			types.NamespacedName{
				Namespace: database.Namespace, Name: database.Spec.SecondaryStorageConfig,
			},
			&v1.SecondaryStorageConfig{},
			&owned.SecondaryStorage,
		})
	}

	var kept []string
	for _, b := range bindings {
		mine, err := r.ownsBinding(ctx, database, b.key, b.obj)
		if err != nil {
			return components.OwnedBindings{}, nil, err
		}
		*b.owned = mine
		if !mine {
			kept = append(kept, b.key.String())
		}
	}

	return owned, kept, nil
}

// ownsBinding reports whether the live object at key carries a controller
// owner reference to database. An object that does not exist is owned: there
// is nothing of anybody else's to delete.
func (r *DatabaseReconciler) ownsBinding(
	ctx context.Context, database *v1.Database, key types.NamespacedName, obj client.Object,
) (bool, error) {
	if err := r.APIReader.Get(ctx, key, obj); err != nil {
		if apierrors.IsNotFound(err) {
			return true, nil
		}

		return false, fmt.Errorf("reading the published binding %q: %w", key, err)
	}

	owner := metav1.GetControllerOf(obj)

	return owner != nil && owner.UID == database.UID, nil
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

	if err := b.EnsureUser(ctx, rb.AppUser, rb.AppPassword.Value); err != nil {
		return fmt.Errorf("ensuring application role %q: %w", rb.AppUser, err)
	}
	if err := b.GrantApplication(ctx, rb.AppUser, name); err != nil {
		return fmt.Errorf("granting application role %q on %q: %w", rb.AppUser, name, err)
	}

	if !rb.BackupEnabled {
		return nil
	}

	if err := b.EnsureBackupUser(ctx, rb.BackupUser, rb.BackupPassword.Value, name); err != nil {
		return fmt.Errorf("ensuring backup role %q on %q: %w", rb.BackupUser, name, err)
	}

	return nil
}

// preCheck runs the documented pre-checks in order: server reference, server
// identity, admin credentials Secret, claim, connection. It records the claim
// of the Database in status.collisionKey once the Database holds it and the
// server answers, and gives up a claim it recorded under another key at the
// same point. It returns the connected Bootstrapper, which the caller
// closes.
//
// A failed check returns an error carrying a *conditions.PreCheckFailure with
// its Ready reason, and a lost claim wraps errClaimLost beside it. Any other
// error is a transient API failure.
func (r *DatabaseReconciler) preCheck(ctx context.Context, database *v1.Database) (pgbootstrap.Bootstrapper, error) {
	server, err := r.resolveServer(ctx, database)
	if err != nil {
		return nil, err
	}

	if server.Status.SystemIdentifier == "" {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonServerIdentityUnknown,
			Message: fmt.Sprintf(
				"DatabaseServerConfig %s has not published its system identifier yet. Wait until "+
					"it reports Ready", client.ObjectKeyFromObject(server),
			),
		}
	}

	// The identity of an endpoint the contract no longer names is the identity
	// of another server. The claim would key on that one while the connection
	// below reaches the endpoint of the spec, so the Database could take a
	// logical database another Database already holds on the endpoint it
	// connects to.
	if !server.ProbedForCurrentSpec() {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonServerIdentityUnknown,
			Message: fmt.Sprintf(
				"DatabaseServerConfig %s has not reached the server its spec names now, so its "+
					"system identifier belongs to the server before that change: the record was "+
					"probed at %s with Secret %q and keys %s, and the spec names %s with Secret "+
					"%q and keys %s. Wait until the contract is probed again for the endpoint and "+
					"the credentials it names now",
				client.ObjectKeyFromObject(server),
				server.Status.ProbedEndpoint, server.Status.ProbedSecretName,
				server.Status.ProbedSecretKeys,
				fmt.Sprintf("%s:%d", server.Spec.Host, server.Spec.Port),
				server.Spec.AdminCredentialsSecretRef.Name,
				server.Spec.AdminCredentialsSecretRef.UsernameKey+"/"+server.Spec.AdminCredentialsSecretRef.PasswordKey,
			),
		}
	}

	key := components.CollisionKey(server.Status.SystemIdentifier, database.Spec.DatabaseName)

	user, password, err := r.adminCredentials(ctx, server)
	if err != nil {
		return nil, err
	}

	if err := r.claim(ctx, database, key); err != nil {
		return nil, err
	}

	bootstrapper, err := connect(ctx, server, user, password)
	if err != nil {
		return nil, err
	}

	// The Database holds the logical database it names now, and the server
	// answers for it. The one it named before is free to go. A release before
	// this point leaves the bindings of this Database on a logical database
	// that another Database can take and rotate the roles of.
	if err := r.releaseStaleClaim(ctx, database, key); err != nil {
		bootstrapper.Close()

		return nil, err
	}
	database.Status.CollisionKey = key

	return bootstrapper, nil
}

// resolveServer fetches the DatabaseServerConfig that spec.serverRef names in
// the namespace of the Database. A dangling reference maps to
// InvalidReference.
func (r *DatabaseReconciler) resolveServer(
	ctx context.Context, database *v1.Database,
) (*v1.DatabaseServerConfig, error) {
	key := types.NamespacedName{Namespace: database.Namespace, Name: database.Spec.ServerRef}

	var server v1.DatabaseServerConfig
	if err := r.Get(ctx, key, &server); err != nil {
		if apierrors.IsNotFound(err) {
			return nil, &conditions.PreCheckFailure{
				Reason:  v1.ReasonInvalidReference,
				Message: fmt.Sprintf("DatabaseServerConfig %s not found", key),
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
	key := types.NamespacedName{Namespace: server.Namespace, Name: ref.Name}

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

// connect opens the admin connection to server and pings it. Any failure maps
// to ConnectionFailed. The caller closes the returned Bootstrapper.
func connect(
	ctx context.Context, server *v1.DatabaseServerConfig, user, password string,
) (pgbootstrap.Bootstrapper, error) {
	bootstrapper, err := pgbootstrap.Connect(ctx, pgbootstrap.Connection{
		Host:     server.Spec.Host,
		Port:     server.Spec.Port,
		User:     user,
		Password: password,
	})
	if err == nil {
		if err = bootstrapper.Ping(ctx); err != nil {
			bootstrapper.Close()
		}
	}
	if err != nil {
		return nil, &conditions.PreCheckFailure{
			Reason: v1.ReasonConnectionFailed,
			Message: fmt.Sprintf(
				"Connecting to DatabaseServerConfig %s: %v", client.ObjectKeyFromObject(server), err,
			),
		}
	}

	return bootstrapper, nil
}

// enqueueForAdminSecret maps a Secret event to every Database whose server
// names that Secret as the admin credentials Secret. A contract names a
// Secret of its own namespace, so the scan covers the contracts of the
// Secret's namespace only. The affected Databases resolve through the
// serverRef index.
func (r *DatabaseReconciler) enqueueForAdminSecret() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		var servers v1.DatabaseServerConfigList
		if err := r.List(ctx, &servers, client.InNamespace(o.GetNamespace())); err != nil {
			logf.FromContext(ctx).Error(err, "listing database server configs for admin Secret enqueue")
			return nil
		}

		var reqs []reconcile.Request
		for _, server := range servers.Items {
			if server.Spec.AdminCredentialsSecretRef.Name != o.GetName() {
				continue
			}

			var databases v1.DatabaseList
			if err := r.List(ctx, &databases, client.MatchingFields{
				databaseServerRefField: refindex.NamespacedKey(server.Namespace, server.Name),
			}); err != nil {
				logf.FromContext(ctx).Error(err, "listing databases for admin Secret enqueue", "server", server.Name)
				continue
			}
			for i := range databases.Items {
				reqs = append(reqs, reconcile.Request{
					NamespacedName: client.ObjectKeyFromObject(&databases.Items[i]),
				})
			}
		}
		return reqs
	})
}

// SetupWithManager registers the controller, the serverRef and collision field
// indexes, and the watches that trigger a reconcile. It refuses a reconciler
// without a namespace for the claim Leases. The watches cover the owned
// bindings, the referenced DatabaseServerConfig, its admin credentials Secret
// (metadata-only), and sibling Databases that contest the same claim. It also
// sets EventRecorder to the recorder of the manager and builds the uncached
// client for the bindings component.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.ClaimNamespace == "" {
		return errors.New("the namespace of the claim Leases is required")
	}

	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder("database")
	}

	if r.componentClient == nil {
		componentClient, err := client.New(mgr.GetConfig(), client.Options{
			Scheme: mgr.GetScheme(),
			Mapper: mgr.GetRESTMapper(),
		})
		if err != nil {
			return fmt.Errorf("building the component client: %w", err)
		}
		// The apply wrapper enforces the precondition of a reused role
		// password, so a delete of a credential Secret rotates it.
		r.componentClient = credentials.NewApplyClient(componentClient)
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.Database{},
		databaseServerRefField, func(o client.Object) []string {
			db := o.(*v1.Database)
			return []string{refindex.NamespacedKey(db.Namespace, db.Spec.ServerRef)}
		},
	); err != nil {
		return err
	}

	if err := mgr.GetFieldIndexer().IndexField(
		context.Background(), &v1.Database{},
		databaseCollisionField, func(o client.Object) []string {
			key := o.(*v1.Database).Status.CollisionKey
			if key == "" {
				return nil
			}
			return []string{key}
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
				databaseServerRefField, refindex.ObjectNamespacedName,
			),
		).
		Watches(&corev1.Secret{}, r.enqueueForAdminSecret(), builder.OnlyMetadata).
		Watches(
			&v1.Database{},
			refindex.Enqueue(
				mgr.GetClient(), &v1.DatabaseList{},
				databaseCollisionField, func(o client.Object) string {
					return o.(*v1.Database).Status.CollisionKey
				},
			),
		).
		Named("database").
		Complete(r)
}
