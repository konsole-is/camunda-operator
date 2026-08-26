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

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/feature"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/secret"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/databaseserverconfig"
)

// ContractComponent builds the contract component: the DatabaseServerConfig
// that the spec names, published in the namespace of the CR. It carries the
// address of the primary instance, a reference to the superuser Secret that
// CloudNativePG writes, and the point-in-time-recovery capability that the
// archive of the server gives it.
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
// The clusterTaken message is the ClusterTaken message while a CloudNativePG
// cluster of the name the server derives is not this server's, and empty
// otherwise. When it is set, the feature gate is off and the framework removes
// the published contract, because the contract names the endpoint and the
// superuser Secret of a database this server does not own. A consumer then
// reads InvalidReference instead of the endpoint of another database. A
// contract the server never published is not removed by that gate, because
// the object under the name is not the one the server published.
func ContractComponent(
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
	clusterTaken string,
	contractTaken string,
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
			PITR: pitrCapability(merged),
		},
	}).WithGuard(takenGuard[v1.DatabaseServerConfig](contractTaken)).Build()
	if err != nil {
		return nil, err
	}

	return component.NewComponentBuilder().
		WithName("contract").
		WithConditionType(v1.ConditionContractReady).
		WithFeatureGate(feature.NewBooleanGate(clusterTaken == "" || contractTaken != "")).
		WithResource(superuser, component.ReadOnly(), component.BlockOnAbsence(), component.Auxiliary()).
		WithResource(contract, component.BlockOnForeignController()).
		Build()
}

// pitrCapability renders the point-in-time-recovery capability the server
// publishes. A server with an archive declares the retention its bucket
// enforces, and that the operator rolls it back on request. A server without
// one declares that no restore can reach it, and that nobody rolls it back.
func pitrCapability(merged v1.DatabaseServerSpec) *v1.PITRCapability {
	if merged.Archive == nil {
		return &v1.PITRCapability{Enabled: false, Recovery: v1.RecoveryModeExternal}
	}

	return &v1.PITRCapability{
		Enabled:             true,
		RetentionPeriodDays: new(merged.Archive.RetentionPeriodDays),
		Recovery:            v1.RecoveryModeOperator,
	}
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
