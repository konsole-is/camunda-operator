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

package databaseserver

import (
	"context"
	"fmt"
	"slices"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
)

// contractTaken says why the DatabaseServerConfig the merged spec names is not
// this server's to publish, and returns the empty string when it is: the
// object does not exist, or this server controls it.
//
// A contract that exists and carries no controller is taken, the way a
// CloudNativePG cluster of the same shape is: it is the bring-your-own-server
// API, so a person wrote it for a PostgreSQL server the operator does not run.
// The guard of the contract component reads this message, because
// component.BlockOnForeignController blocks on a controller of somebody else
// and that contract carries none.
//
// The read also reaches the holder on a pass where the block never fires: the
// contract sits behind the superuser Secret, and a blocked resource stops
// every resource after it, so a server still waiting for that Secret would
// report the wait and never the holder.
func (r *DatabaseServerReconciler) contractTaken(
	ctx context.Context,
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
) (string, error) {
	if merged.DatabaseServerConfig == "" {
		return "", nil
	}

	key := types.NamespacedName{Namespace: server.Namespace, Name: merged.DatabaseServerConfig}

	var contract v1.DatabaseServerConfig
	if err := r.APIReader.Get(ctx, key, &contract); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}

		return "", fmt.Errorf("reading DatabaseServerConfig %s: %w", key, err)
	}

	// The controller reference alone, not ownedByServer. A contract that
	// carries our reference and lost its label is ours to repair, and holding
	// it against ourselves would name this server as its own holder.
	holder := metav1.GetControllerOf(&contract)
	if holder != nil && holder.UID == server.UID {
		return "", nil
	}

	return components.ContractTakenMessage(merged.DatabaseServerConfig, holder), nil
}

// archiveTaken says why the Barman Cloud ObjectStore of the name the server
// derives is not this server's to write, and returns the empty string when it
// is: the spec asks for no archive, no object of that name exists, nothing
// controls it, or this server controls it.
//
// An ObjectStore that nothing controls is adopted, where a contract and a
// CloudNativePG cluster of the same shape are refused. It carries the location
// of an archive and the way the plugin reaches it, both of which this server
// resolves from its own ObjectStorageConfig, so the apply takes no data of
// anybody. It is also what component.BlockOnForeignController does with one,
// so a message here names a holder that the apply then writes over on the
// same pass.
//
// The read is what keeps the cluster off the bucket of another owner. The
// archive plugin entry of the cluster names this ObjectStore, so the apply of
// the cluster runs before the block on the ObjectStore is ever reached, and
// CloudNativePG writes the write-ahead log of this server through whatever
// object holds the name.
func (r *DatabaseServerReconciler) archiveTaken(
	ctx context.Context,
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
) (string, error) {
	if !components.Archiving(merged) {
		return "", nil
	}

	name := components.ObjectStoreName(server)
	key := types.NamespacedName{Namespace: server.Namespace, Name: name}

	var store barmanobjectstore.ObjectStore
	if err := r.APIReader.Get(ctx, key, &store); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}

		return "", fmt.Errorf("reading the Barman Cloud ObjectStore %s: %w", key, err)
	}

	holder := metav1.GetControllerOf(&store)
	if holder == nil || holder.UID == server.UID {
		return "", nil
	}

	return components.ArchiveTakenMessage(name, *holder), nil
}

// serviceAccountTaken says why the ServiceAccount of the instance pods is not
// this server's, and returns the empty string when no object of that name
// exists, nothing controls it, or this server controls it.
func (r *DatabaseServerReconciler) serviceAccountTaken(
	ctx context.Context,
	server *v1.DatabaseServer,
) (string, error) {
	key := types.NamespacedName{Namespace: server.Namespace, Name: components.ServiceAccountName(server)}

	var account corev1.ServiceAccount
	if err := r.APIReader.Get(ctx, key, &account); err != nil {
		if apierrors.IsNotFound(err) {
			return "", nil
		}

		return "", fmt.Errorf("reading the ServiceAccount %s: %w", key, err)
	}

	holder := metav1.GetControllerOf(&account)
	if holder == nil || holder.UID == server.UID {
		return "", nil
	}

	return fmt.Sprintf("ServiceAccount %q is controlled by %s %q", key.Name, holder.Kind, holder.Name), nil
}

// removeSupersededContracts deletes every DatabaseServerConfig that the server
// owns and no longer publishes.
//
// spec.databaseServerConfig can be renamed. The contract of the name before it
// keeps its owner reference and its pitr.recovery: operator declaration, so a
// PointInTimeRestore that resolves through it writes a request on an object
// that this controller never reads again. That restore waits for an answer
// that never comes.
//
// It runs only when the contract the spec names now is published, so a
// reconcile that could not apply it never takes the one that is there.
//
// The contract that status.recovery names is never swept. It carries the
// request while the recovery runs, and it carries the answer afterwards:
// spec.pitr.lastRecovery on that object is the only place a PointInTimeRestore
// reads the result from. Sweeping it the moment the answer lands fails a
// rollback that succeeded. It goes when the next recovery answers on another
// contract, and status.recovery names that one instead.
//
// The list is cached: the contract is owned and watched.
func (r *DatabaseServerReconciler) removeSupersededContracts(
	ctx context.Context,
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
) error {
	answering := ""
	if running := server.Status.Recovery; running != nil {
		if running.CompletedAt == nil {
			return nil
		}
		answering = running.Contract
	}

	var contracts v1.DatabaseServerConfigList
	if err := r.List(
		ctx, &contracts,
		client.InNamespace(server.Namespace),
		client.MatchingLabels{labels.DatabaseServerKey: labels.OwnerName(server.Name)},
	); err != nil {
		return fmt.Errorf("listing the contracts of %q: %w", server.Name, err)
	}

	// The replacement goes in first. The contract component blocks while the
	// superuser Secret is missing and it publishes nothing then, so a sweep
	// on that look leaves the server with no contract at all.
	if !slices.ContainsFunc(contracts.Items, func(published v1.DatabaseServerConfig) bool {
		return published.Name == merged.DatabaseServerConfig && ownedByServer(server, &published)
	}) {
		return nil
	}

	for i := range contracts.Items {
		superseded := &contracts.Items[i]
		if superseded.Name == merged.DatabaseServerConfig || superseded.Name == answering ||
			!ownedByServer(server, superseded) {
			continue
		}
		if err := r.deleteOwned(ctx, superseded); err != nil {
			return fmt.Errorf("deleting the superseded contract %q: %w", superseded.Name, err)
		}
	}

	return nil
}

// stageTakenNames reports every derived name of the server that somebody else
// holds, on the condition of the component that writes that object.
func stageTakenNames(server *v1.DatabaseServer, resolved resolvedSpec) {
	taken := []struct {
		conditionType string
		reason        string
		message       string
	}{
		{v1.ConditionContractReady, v1.ReasonContractTaken, resolved.contractTaken},
		{v1.ConditionClusterReady, v1.ReasonClusterTaken, resolved.clusterTaken},
		{v1.ConditionArchiveReady, v1.ReasonArchiveTaken, resolved.archiveTaken},
	}

	for _, held := range taken {
		if held.message != "" {
			stageFailure(server, held.conditionType, held.reason, held.message)
		}
	}
}

// stageFailure puts a reason and a remedy on the condition of one component,
// over whatever ocf reported there. ocf answers a blocked apply with Blocked
// and the object it stopped at, which carries no remedy and reads the same as
// every other wait. The callers here name what happened and what to do about
// it, which is what the user acts on.
func stageFailure(server *v1.DatabaseServer, conditionType, reason, message string) {
	meta.SetStatusCondition(&server.Status.Conditions, metav1.Condition{
		Type:               conditionType,
		Status:             metav1.ConditionFalse,
		Reason:             reason,
		Message:            conditions.BoundMessage(message),
		ObservedGeneration: server.Generation,
	})
}
