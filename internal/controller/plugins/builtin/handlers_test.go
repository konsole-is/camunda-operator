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

package builtin

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/adapter"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/elasticsearch"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/postgresql"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

const requestUID = "request-uid"

func TestExistingRegistrationsReadTheirContracts(t *testing.T) {
	tests := []struct {
		registration, params string
		want                 []adapter.ContractRef
	}{
		{
			pluginstartup.RegistrationExistingObjectStorage, `{"name":"my-bucket"}`,
			[]adapter.ContractRef{{
				Output: pluginstartup.OutputObjectStorageConfig, Kind: adapter.KindObjectStorageConfig, Name: "my-bucket",
			}},
		},
		{
			pluginstartup.RegistrationExistingSecondaryStorage, `{"name":"my-es"}`,
			[]adapter.ContractRef{{
				Output: pluginstartup.OutputSecondaryStorageConfig, Kind: adapter.KindSecondaryStorageConfig, Name: "my-es",
			}},
		},
		{
			pluginstartup.RegistrationExistingDatabaseConfigs, `{"webModeler":"wm","keycloak":"kc"}`,
			[]adapter.ContractRef{
				{Output: "keycloakDatabaseConfig", Kind: adapter.KindDatabaseConfig, Name: "kc"},
				{Output: "webModelerDatabaseConfig", Kind: adapter.KindDatabaseConfig, Name: "wm"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.registration, func(t *testing.T) {
			h, ok := handlers()[tt.registration]
			require.True(t, ok)
			assert.Equal(t, tt.want, h.Contracts(requestFor(tt.registration, tt.params)))
		})
	}
}

func TestContractsOfUnparsableParametersAreEmpty(t *testing.T) {
	for name, h := range handlers() {
		t.Run(name, func(t *testing.T) {
			assert.Empty(t, h.Contracts(requestFor(name, `{"name":3,"keycloak":3}`)))
		})
	}
}

func TestElasticsearchRefusesParameterNamesInAnotherCase(t *testing.T) {
	schema, err := newParameterSchema(schemaOf(t, pluginstartup.RegistrationElasticsearch))
	require.NoError(t, err)
	r := &Reconciler{
		APIReader: fakeReader(t),
		schemas:   map[string]*parameterSchema{pluginstartup.RegistrationElasticsearch: schema},
	}
	tests := []struct{ params, field string }{
		{`{"preset":"small","releaseRef":"approved","releaseref":"other"}`, "releaseref"},
		{`{"preset":"small","releaseRef":"approved","Overrides":{"replicas":0}}`, "Overrides"},
		{`{"preset":"small","releaseRef":"approved","overrides":{"Replicas":0}}`, "Replicas"},
		{
			`{"preset":"small","releaseRef":"approved","overrides":{"resources":` +
				`{"limits":{"memory":"1Gi"},"Limits":{"memory":"4Gi"}}}}`,
			"Limits",
		},
		{
			`{"preset":"small","releaseRef":"approved","overrides":{"scheduling":` +
				`{"tolerations":[{"Key":"pool","value":"es"}]}}}`,
			"Key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			_, err := r.build(t.Context(), elasticsearchRequest(tt.params, nil), elasticsearch.Handler{})

			var failure *conditions.PreCheckFailure
			require.ErrorAs(t, err, &failure)
			assert.Equal(t, pluginsv1alpha1.ReasonInvalidParameters, failure.Reason)
			assert.Contains(t, failure.Message, tt.field)
		})
	}
}

func TestElasticsearchOrphanReadsTheClusterWithoutTheCache(t *testing.T) {
	stored := elasticsearchRequest(`{"preset":"small"}`, nil)
	stored.UID = "request-uid"
	stored.Spec.DeletionPolicy = pluginsv1alpha1.DeletionPolicyOrphan
	stored.Finalizers = []string{pluginsv1alpha1.FinalizerCleanup}
	cluster := &v1.ElasticsearchCluster{ObjectMeta: metav1.ObjectMeta{
		Name: "orders-es", Namespace: stored.Namespace,
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: pluginsv1alpha1.GroupVersion.String(), Kind: "PluginRequest",
			Name: stored.Name, UID: stored.UID, Controller: new(true),
		}},
	}}
	apiServer := fakeReader(t, stored, cluster)
	// The cache has not seen the cluster yet. Writes reach the API server.
	cached := uncachedReads{Client: apiServer, reader: fakeReader(t)}
	request := &pluginsv1alpha1.PluginRequest{}
	require.NoError(t, apiServer.Get(t.Context(), client.ObjectKeyFromObject(stored), request))
	r := &Reconciler{Client: cached, APIReader: apiServer}
	rec := component.ReconcileContext{
		Client: cached, Scheme: apiServer.Scheme(), EventRecorder: events.NewFakeRecorder(10),
		APIReader: apiServer, Owner: request,
	}

	_, err := r.finalize(t.Context(), rec, request, elasticsearch.Handler{})
	require.NoError(t, err)

	var got v1.ElasticsearchCluster
	require.NoError(t, apiServer.Get(t.Context(), client.ObjectKeyFromObject(cluster), &got))
	assert.Empty(t, got.OwnerReferences)
}

func TestPostgreSQLDeleteKeepsTheFinalizerWhileTheDatabaseRemains(t *testing.T) {
	stored := postgresRequest(`{"preset":"small"}`, nil)
	stored.UID = requestUID
	stored.Finalizers = []string{pluginsv1alpha1.FinalizerCleanup}
	owner := []metav1.OwnerReference{{
		APIVersion: pluginsv1alpha1.GroupVersion.String(), Kind: "PluginRequest",
		Name: stored.Name, UID: stored.UID, Controller: new(true),
	}}
	db := &v1.Database{ObjectMeta: metav1.ObjectMeta{
		Name: "orders", Namespace: stored.Namespace, OwnerReferences: owner, Finalizers: []string{testHold},
	}}
	server := &v1.DatabaseServer{ObjectMeta: metav1.ObjectMeta{
		Name: "orders-pg", Namespace: stored.Namespace, OwnerReferences: owner,
	}}
	c := fakeReader(t, stored, db, server)
	request := &pluginsv1alpha1.PluginRequest{}
	require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(stored), request))
	r := &Reconciler{Client: c, APIReader: c}
	rec := component.ReconcileContext{
		Client: c, Scheme: c.Scheme(), EventRecorder: events.NewFakeRecorder(10), APIReader: c, Owner: request,
	}

	waiting, err := r.finalize(t.Context(), rec, request, postgresql.Handler{})
	require.NoError(t, err)

	assert.True(t, waiting)
	assert.True(
		t, apierrors.IsNotFound(c.Get(t.Context(), client.ObjectKeyFromObject(server), &v1.DatabaseServer{})),
		"the server does not wait for the Database",
	)
	var live v1.Database
	require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(db), &live))
	assert.False(t, live.DeletionTimestamp.IsZero())
}

// elasticsearchRequest returns an elasticsearch request of the claim orders
// with params and inputs.
func elasticsearchRequest(params string, inputs map[string]string) *pluginsv1alpha1.PluginRequest {
	request := requestFor(pluginstartup.RegistrationElasticsearch, params)
	request.Spec.ClaimRef.Name = "orders"
	request.Spec.Inputs = inputs

	return request
}

// postgresRequest returns a database-server request of the claim orders with
// params and inputs.
func postgresRequest(params string, inputs map[string]string) *pluginsv1alpha1.PluginRequest {
	request := requestFor(pluginstartup.RegistrationDatabaseServer, params)
	request.Spec.ClaimRef.Name = "orders"
	request.Spec.Inputs = inputs

	return request
}
