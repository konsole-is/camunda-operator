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
	"fmt"
	"time"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/secret"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/databaseserverconfig"
)

// ArchiveLag is how far the end of the archive that a server writes now is
// behind the present. CloudNativePG sets archive_timeout to five minutes, and
// the operator does not change it. Thus PostgreSQL closes a segment of the
// write-ahead log at least every five minutes while the server writes. The
// lag gives the upload of that segment one more minute.
const ArchiveLag = 6 * time.Minute

// maxRecoveryWindows is the maximum of spec.pitr.windows on a
// DatabaseServerConfig.
const maxRecoveryWindows = 256

// ContractComponent builds the contract component: the DatabaseServerConfig
// that the spec names, published in the namespace of the CR. It carries the
// address of the primary instance, a reference to the superuser Secret that
// CloudNativePG writes, and the point-in-time-recovery capability that the
// archive of the server gives it. The windows of that capability are
// status.archive.history of server as it stands when the component is built.
//
// A read-only registration guards the contract on the superuser Secret and
// blocks while that Secret is absent. A contract that named a Secret before
// CloudNativePG had written it would send every consumer to credentials that
// do not exist.
//
// The component leaves spec.recovery and spec.pitr.lastRecovery alone. A
// consumer writes the first and the server answers in the second, each under
// a field manager of its own, so neither is removed by this apply.
//
// The contract blocks on a foreign controller, so the first server to publish
// a name keeps it: a second server applies nothing while that owner holds the
// name. The caller reports v1.ReasonContractTaken over the block, because the
// framework names the owner and not the remedy.
//
// The contractTaken message is the ContractTakenMessage while a
// DatabaseServerConfig of that name exists that this server did not publish,
// and empty otherwise. A guard blocks the apply while it is set. It covers the
// contract that nothing controls, which the framework leaves to be adopted: a
// person writes that one for a PostgreSQL server the operator does not run,
// and the apply would rewrite its endpoint and its credentials.
//
// The withdrawn message is the ClusterTaken message when the contract must go
// because a CloudNativePG cluster of the name the server derives is not this
// server's, and empty otherwise. When it is set, the feature gate is off and
// the framework removes the published contract, because the contract names the
// endpoint and the superuser Secret of a database this server does not own. A
// consumer then reads InvalidReference instead of the endpoint of another
// database. A contract the server never published is not removed by that gate,
// because the object under the name is not the one the server published.
//
// The archiveTaken message is the ArchiveTaken message while the ObjectStore
// of the name the server derives belongs to somebody else, and empty
// otherwise. The published contract then declares no point-in-time-recovery
// capability, because the server archives nothing while that name is held.
func ContractComponent(
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	withdrawn string,
	contractTaken string,
	archiveTaken string,
) (*component.Component, error) {
	superuser, err := secret.NewBuilder(superuserSecretRef(server)).Build()
	if err != nil {
		return nil, err
	}

	contract, err := databaseserverconfig.NewBuilder(&v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      merged.DatabaseServerConfig,
			Namespace: server.Namespace,
			Labels:    managedLabels(server),
		},
		Spec: v1.DatabaseServerConfigSpec{
			Engine: v1.DatabaseEnginePostgres,
			Host:   ReadWriteHost(server),
			Port:   PostgresPort,
			AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
				Name:        SuperuserSecretName(server),
				UsernameKey: SuperuserUsernameKey,
				PasswordKey: SuperuserPasswordKey,
			},
			PITR: pitrCapability(server, merged, archiveTaken),
		},
	}).WithGuard(takenGuard[v1.DatabaseServerConfig](contractTaken)).Build()
	if err != nil {
		return nil, err
	}

	return component.NewComponentBuilder().
		WithName("contract").
		WithConditionType(v1.ConditionContractReady).
		WithFeatureGate(feature.NewBooleanGate(withdrawn == "" || contractTaken != "")).
		WithResource(superuser, component.ReadOnly(), component.BlockOnAbsence(), component.Auxiliary()).
		WithResource(contract, component.BlockOnForeignController(), component.DeleteOnlyIfOwned()).
		Build()
}

// pitrCapability renders the point-in-time-recovery capability the server
// publishes. A server with an archive declares the retention its bucket
// enforces, the windows of its archive history, and that the operator rolls it
// back on request. A server without one declares that no restore can reach it,
// and that nobody rolls it back.
//
// A server whose archive history is empty declares the same as a server
// without an archive. No base backup of its archive has completed, so no
// restore can reach a point yet.
//
// A server whose ObjectStore another owner holds declares the same as a server
// without an archive. It archives nothing while the name is held, and a
// rollback reads the archive of that ObjectStore, so a capability here sends a
// restore to the archive of another server.
func pitrCapability(server *v1.DatabaseServer, merged v1.DatabaseServerSpec, archiveTaken string) *v1.PITRCapability {
	var history []v1.ArchiveRecord
	if server.Status.Archive != nil {
		history = server.Status.Archive.History
	}

	if merged.Archive == nil || archiveTaken != "" || len(history) == 0 {
		return &v1.PITRCapability{Enabled: false, Recovery: v1.RecoveryModeExternal}
	}

	return &v1.PITRCapability{
		Enabled:             true,
		RetentionPeriodDays: new(merged.Archive.RetentionPeriodDays),
		Recovery:            v1.RecoveryModeOperator,
		Windows:             recoveryWindows(history),
	}
}

// recoveryWindows returns the windows that history gives a restore, oldest
// first. It keeps the newest maxRecoveryWindows records, the most that a
// contract holds.
func recoveryWindows(history []v1.ArchiveRecord) []v1.RecoveryWindow {
	if older := len(history) - maxRecoveryWindows; older > 0 {
		history = history[older:]
	}

	windows := make([]v1.RecoveryWindow, 0, len(history))
	for _, record := range history {
		window := v1.RecoveryWindow{
			Location: record.Location,
			From:     record.From,
			To:       record.To.DeepCopy(),
		}
		if record.UnverifiedFrom != nil {
			// The segment whose upload failed began before the record's time,
			// by at most archive_timeout while the server writes.
			cutoff := metav1.NewTime(record.UnverifiedFrom.Add(-ArchiveLag))
			window.UnverifiedFrom = &cutoff
		}
		if record.To == nil {
			window.Lag = &metav1.Duration{Duration: ArchiveLag}
		}
		windows = append(windows, window)
	}

	return windows
}

// ContractTakenMessage says that a DatabaseServerConfig of the name the merged
// spec asks for exists and is not this server's, and what to do about it.
// holder is the owner that controls it, and it is nil when nothing controls
// it. The guard of the contract and the ContractReady condition of the server
// both read it, so the reason a user acts on is written once.
//
// A contract with no controller is refused rather than adopted. It is the
// bring-your-own-server API: a person writes one for a PostgreSQL server the
// operator does not run, and the apply would rewrite the endpoint and the
// credentials that every consumer of it reads.
func ContractTakenMessage(name string, holder *metav1.OwnerReference) string {
	controller := "no owner controls it"
	if holder != nil {
		controller = fmt.Sprintf("%s %q controls it", holder.Kind, holder.Name)
	}

	return fmt.Sprintf(
		"DatabaseServerConfig %q already exists and %s. This server publishes nothing on it, so "+
			"the endpoint and the credentials its consumers read stay what they are. Give this "+
			"server a name of its own in spec.databaseServerConfig, or remove that contract.",
		name, controller,
	)
}
