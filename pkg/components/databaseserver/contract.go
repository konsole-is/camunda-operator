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
	"github.com/sourcehawk/operator-component-framework/pkg/component"
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
func ContractComponent(
	server *v1.DatabaseServer,
	merged v1.DatabaseServerSpec,
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
	}).Build()
	if err != nil {
		return nil, err
	}

	return component.NewComponentBuilder().
		WithName("contract").
		WithConditionType(ConditionContract).
		WithResource(superuser, component.ReadOnly(), component.BlockOnAbsence(), component.Auxiliary()).
		WithResource(contract).
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
