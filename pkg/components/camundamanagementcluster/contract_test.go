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

package camundamanagementcluster

import (
	"testing"

	"github.com/stretchr/testify/assert"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// In the oidc mode the contract carries the issuer and the endpoints of the
// platform config, the external URL of Management Identity, and the Optimize
// client with the reference to its secret as the platform config declared it.
func TestManagementAuthSpecInOIDCMode(t *testing.T) {
	t.Parallel()

	assert.Equal(
		t, v1.ManagementAuthConfigSpec{
			BaseURL:          fixtureExternal,
			IssuerURL:        fixtureIssuer,
			IssuerBackendURL: fixtureIssuer,
			AuthURL:          fixtureIssuer + "/oauth/authorize",
			TokenURL:         fixtureIssuer + "/oauth/token",
			JwksURL:          fixtureIssuer + "/.well-known/jwks.json",
			ClientID:         "optimize",
			Audience:         "optimize",
			ClientSecretRef: v1.SecretKeyRef{
				Name:      "oidc-credentials",
				Namespace: "platform",
				Key:       "optimize-client-secret",
			},
		}, ManagementAuthSpec(fixtureMinimal(t)),
	)
}

// The contract is cluster-scoped and its owner is namespaced, so the owner
// labels carry both the name and the namespace of the management cluster.
func TestContractLabelsCarryTheOwnerAndItsNamespace(t *testing.T) {
	t.Parallel()

	got := ContractLabels(newCluster(nil))

	assert.Equal(t, fixtureName, got[labels.ManagementClusterKey])
	assert.Equal(t, fixtureNamespace, got[labels.ManagementClusterNamespaceKey])
	assert.Equal(t, labels.ManagedBy, got[labels.ManagedByKey])
}

// The contract takes the name of the resource unless the spec names another
// one.
func TestContractName(t *testing.T) {
	t.Parallel()

	assert.Equal(t, fixtureName, ContractName(newCluster(nil)))
	assert.Equal(t, "shared-auth", ContractName(newCluster(func(mc *v1.CamundaManagementCluster) {
		mc.Spec.ManagementAuthConfigName = "shared-auth"
	})))
}
