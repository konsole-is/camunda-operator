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
	"k8s.io/apimachinery/pkg/api/meta"
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
	"github.com/konsole-is/camunda-operator/internal/observability"
	components "github.com/konsole-is/camunda-operator/pkg/components/database"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/credentials"
	"github.com/konsole-is/camunda-operator/pkg/leaseclaim"
	"github.com/konsole-is/camunda-operator/pkg/pgbootstrap"
	"github.com/konsole-is/camunda-operator/pkg/refindex"
	"github.com/konsole-is/camunda-operator/pkg/secretref"
)

// controllerName is the name the controller registers with controller-runtime.
// It labels its events and every metrics series it records.
const controllerName = "database"

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
	// Metrics records the condition gauge and the apply counters of the
	// framework. SetupWithManager sets it when it is nil.
	Metrics component.MetricsRecorder

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
		if apierrors.IsNotFound(err) {
			observability.Forget(r.Metrics, new(v1.Database).GetKind(), req.NamespacedName)
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, err
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
		Metrics:       r.Metrics,
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
		// Nothing holds the logical database yet, so this Database has lost
		// nothing. It keeps its bindings and every claim it holds, and looks
		// again when the claimant that goes first has taken or left the name.
		if errors.Is(err, errClaimNotFirst) {
			conditions.Stage(&database, conditions.Failed(&database, failure))

			return ctrl.Result{RequeueAfter: claimRetryInterval}, nil
		}

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

			// The withdrawal leaves nothing that names a logical database of
			// this Database, so it gives every claim it holds back. A
			// Database that kept one would hold a name it no longer uses.
			if dropErr := r.releaseHeldClaims(
				ctx, leaseclaim.HolderOf(&database), "",
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

// preCheck runs the documented pre-checks in order: server reference, server
// identity, admin credentials Secret, claim, connection. It records the
// logical database that the Database names in status.collisionKey as soon as
// the identity is known, and gives back every other claim it holds once it
// holds this one and the server answers. It returns the connected
// Bootstrapper, which the caller closes.
//
// A failed check returns an error carrying a *conditions.PreCheckFailure with
// its Ready reason. A lost claim wraps errClaimLost beside it, and a claim
// that another Database goes first for wraps errClaimNotFirst. Any other
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
	// The field says which logical database this Database names. Every
	// claimant records it, the one that loses included, so the index that
	// checkCollision reads lists a Database under the name it asks for now
	// and under no name it gave up.
	database.Status.CollisionKey = key

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
	// answers for it. Everything else it holds is free to go. A release
	// before this point leaves the bindings of this Database on a logical
	// database that another Database can take and rotate the roles of.
	//
	// The sweep runs on every pre-check that reaches here. status.collisionKey
	// is written by the flush of this reconcile whether the sweep ran or not,
	// so it is no record that the release happened, and a sweep that skipped
	// on it would never run again after one failure.
	if err := r.releaseHeldClaims(ctx, leaseclaim.HolderOf(database), key); err != nil {
		bootstrapper.Close()

		return nil, err
	}

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

// withdrawBindings deletes the bindings that database published and still
// controls, and sets comps to the component that did it, so the one status
// flush of this reconcile owns BindingsReady and reports it as Disabled. A
// Database that lost its claim keeps neither the contracts nor the credential
// Secrets it published.
//
// It returns the bindings it left in place. Two Databases can name one binding
// explicitly, and only the one that controls it may delete it, so a loser that
// names the winner's object withdraws everything else and reports that name.
func (r *DatabaseReconciler) withdrawBindings(
	ctx context.Context,
	rec component.ReconcileContext,
	database *v1.Database,
	comps *[]*component.Component,
) ([]string, error) {
	published, kept, err := r.publishedBindings(ctx, database)
	if err != nil {
		return nil, err
	}

	comp, err := components.WithdrawnBindingsComponent(database, published)
	if err != nil {
		return kept, err
	}
	*comps = []*component.Component{comp}

	return kept, comp.Reconcile(ctx, rec)
}

// publishedBindings reports the objects that database published and still
// controls, and names the ones another Database controls. It reads the live
// objects, not the spec of database.
//
// One edit can rename a binding and move the Database onto the logical database
// of another at the same time. The names of the spec then reach nothing, and
// the objects stay under the names from before that edit.
func (r *DatabaseReconciler) publishedBindings(
	ctx context.Context, database *v1.Database,
) (components.PublishedBindings, []string, error) {
	rb := components.ResolveBindings(database)
	var published components.PublishedBindings

	// One entry per binding kind. The order decides only the order of the names
	// in the message.
	kinds := []struct {
		list  client.ObjectList
		empty func() client.Object
		named []string
		into  *[]string
	}{
		{
			&corev1.SecretList{},
			func() client.Object { return &corev1.Secret{} },
			[]string{rb.AppSecret.Name, rb.BackupSecret.Name},
			&published.Secrets,
		},
		{
			&v1.DatabaseConfigList{},
			func() client.Object { return &v1.DatabaseConfig{} },
			[]string{rb.DatabaseConfigName},
			&published.DatabaseConfigs,
		},
		{
			&v1.SecondaryStorageConfigList{},
			func() client.Object { return &v1.SecondaryStorageConfig{} },
			[]string{database.Spec.SecondaryStorageConfig},
			&published.SecondaryStorageConfigs,
		},
	}

	var kept []string
	for _, kind := range kinds {
		withdraw, left, err := r.bindingsOfKind(ctx, database, kind.list, kind.empty, kind.named)
		if err != nil {
			return components.PublishedBindings{}, nil, err
		}

		*kind.into = withdraw
		kept = append(kept, left...)
	}

	return published, kept, nil
}

// bindingsOfKind decides one kind of binding. It returns the names that
// database may withdraw, and the names it must leave in place. empty returns an
// object of that kind for a live read.
//
// It looks at two sets of objects. The labels find every object that database
// published, under the name it carries now. The names of the spec find an
// object that another Database published under a name this one asks for: such
// an object carries the labels of its own Database, so nothing but the spec
// points at it.
func (r *DatabaseReconciler) bindingsOfKind(
	ctx context.Context,
	database *v1.Database,
	list client.ObjectList,
	empty func() client.Object,
	named []string,
) (withdraw, kept []string, err error) {
	decide := func(obj client.Object) {
		if controls(database, obj) {
			withdraw = append(withdraw, obj.GetName())

			return
		}

		kept = append(kept, client.ObjectKeyFromObject(obj).String())
	}

	key := client.ObjectKeyFromObject(database)
	if err := r.APIReader.List(
		ctx, list,
		client.InNamespace(database.Namespace),
		client.MatchingLabels(components.BindingSelector(database)),
	); err != nil {
		return nil, nil, fmt.Errorf("listing the published bindings of %q: %w", key, err)
	}

	items, err := meta.ExtractList(list)
	if err != nil {
		return nil, nil, fmt.Errorf("reading the published bindings of %q: %w", key, err)
	}

	labelled := make(map[string]bool, len(items))
	for _, item := range items {
		obj, ok := item.(client.Object)
		if !ok {
			return nil, nil, fmt.Errorf("published binding of %q is no object: %T", key, item)
		}

		labelled[obj.GetName()] = true
		decide(obj)
	}

	for _, name := range named {
		if name == "" || labelled[name] {
			continue
		}

		bindingKey := types.NamespacedName{Namespace: database.Namespace, Name: name}
		obj := empty()
		if err := r.APIReader.Get(ctx, bindingKey, obj); err != nil {
			// Nothing under the name, so there is nothing to withdraw and
			// nothing of anybody else's to report.
			if apierrors.IsNotFound(err) {
				continue
			}

			return nil, nil, fmt.Errorf("reading the published binding %q: %w", bindingKey, err)
		}

		decide(obj)
	}

	return withdraw, kept, nil
}

// controls reports whether obj carries the controller owner reference of
// database. Only the Database that controls an object may delete it.
func controls(database *v1.Database, obj client.Object) bool {
	owner := metav1.GetControllerOf(obj)

	return owner != nil && owner.UID == database.UID
}

// SetupWithManager registers the controller, the serverRef and collision field
// indexes, and the watches that trigger a reconcile. It refuses a reconciler
// without a namespace for the claim Leases. The watches cover the owned
// bindings, the referenced DatabaseServerConfig, its admin credentials Secret
// (metadata-only), and sibling Databases that contest the same claim. It also
// sets EventRecorder and Metrics when they are nil, and builds the uncached
// client for the bindings component.
func (r *DatabaseReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.ClaimNamespace == "" {
		return errors.New("the namespace of the claim Leases is required")
	}

	if err := components.ClaimSchema().Validate(); err != nil {
		return fmt.Errorf("the claim Schema of Database: %w", err)
	}

	if r.EventRecorder == nil {
		r.EventRecorder = mgr.GetEventRecorder(controllerName)
	}
	if r.Metrics == nil {
		r.Metrics = observability.Recorder(controllerName)
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
		Named(controllerName).
		Complete(r)
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
