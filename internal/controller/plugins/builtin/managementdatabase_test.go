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
	"reflect"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

func TestManagementDatabasesParseDeduplicatesAndOrders(t *testing.T) {
	request := planeRequest(`["webModeler","identity","identity"]`)

	assert.Equal(
		t, []contractRef{
			{output: pluginstartup.OutputIdentityDatabaseConfig, kind: kindDatabaseConfig, name: "ops-identity"},
			{output: pluginstartup.OutputWebModelerDatabaseConfig, kind: kindDatabaseConfig, name: "ops-web-modeler"},
		}, managementDatabases{}.contracts(request),
	)
}

func TestManagementDatabasesReadTheListedDatabaseConfigs(t *testing.T) {
	s := buildFor(t, planeRequest(`["keycloak","identity"]`))

	require.NotNil(t, s.backing)
	for name, want := range map[string]bool{"ops-keycloak": true, "ops-identity": true, "ops-web-modeler": false} {
		_, ok := s.outputs.Resource("core.camunda.io/v1/DatabaseConfig/my-cluster-ns/" + name)
		assert.Equal(t, want, ok, name)
	}
}

func TestManagementDatabasesPublishOnlyTheListedOutputs(t *testing.T) {
	request := planeRequest(`["keycloak","identity"]`)
	s := buildFor(t, request)

	reconcileOutputs(t, s, request, fakeReader(t, databaseConfig("ops-keycloak"), databaseConfig("ops-identity")))

	assert.Equal(t, metav1.ConditionTrue, s.outputs.GetCondition(request).Status)
	assert.Equal(
		t, map[string]string{
			pluginstartup.OutputKeycloakDatabaseConfig: "ops-keycloak",
			pluginstartup.OutputIdentityDatabaseConfig: "ops-identity",
		}, s.report().outputs,
	)
}

func TestManagementDatabasesWaitForAMissingDatabaseConfig(t *testing.T) {
	request := planeRequest(`["keycloak","identity"]`)
	s := buildFor(t, request)

	reconcileOutputs(t, s, request, fakeReader(t, databaseConfig("ops-keycloak")))

	cond := s.outputs.GetCondition(request)
	assert.Equal(t, metav1.ConditionFalse, cond.Status)
	assert.Equal(t, string(component.GuardBlocked), cond.Reason)
}

func TestManagementDatabasesWithoutDatabasesWritesTheServerOnly(t *testing.T) {
	for name, parameters := range map[string]string{
		"empty list":   `{"preset":"small","databases":[]}`,
		"no databases": `{"preset":"small"}`,
	} {
		t.Run(name, func(t *testing.T) {
			request := planeRequest(`[]`)
			request.Spec.Parameters.Raw = []byte(parameters)
			s := buildFor(t, request)

			reconcileOutputs(t, s, request, fakeReader(t))

			assert.Empty(t, managementDatabases{}.contracts(request))
			assert.Equal(t, metav1.ConditionTrue, s.outputs.GetCondition(request).Status)
			assert.Empty(t, s.report().outputs)
			assert.Equal(
				t, []pluginsv1alpha1.ResourceRef{
					{Kind: kindDatabaseServer, Name: "ops-db", OnOrphan: pluginsv1alpha1.OnOrphanKept},
				}, s.report().resources,
			)
		})
	}
}

func TestManagementDatabasesReportTheServerAndTheDatabases(t *testing.T) {
	s := buildFor(t, planeRequest(`["identity"]`))

	rep := s.report()

	assert.Equal(
		t, []pluginsv1alpha1.ResourceRef{
			{Kind: kindDatabaseServer, Name: "ops-db", OnOrphan: pluginsv1alpha1.OnOrphanKept},
			{Kind: kindDatabase, Name: "ops-identity", OnOrphan: pluginsv1alpha1.OnOrphanKept},
		}, rep.resources,
	)
	require.NotNil(t, rep.podSelector)
	assert.Equal(t, map[string]string{"camunda.io/database-server": "ops-db"}, rep.podSelector.MatchLabels)
	assert.Equal(
		t, []pluginsv1alpha1.SummaryItem{
			{Label: kindDatabaseServer, Value: "ops-db, waiting", Severity: pluginsv1alpha1.SeverityWarning},
			{Label: kindDatabase, Value: "ops-identity, waiting", Severity: pluginsv1alpha1.SeverityWarning},
		}, rep.summary,
	)
}

func TestManagementDatabasesReportTheVersionOfAReadyServer(t *testing.T) {
	request := planeRequest(`[]`)
	request.UID = requestUID
	s := buildFor(t, request)
	server := &v1.DatabaseServer{ObjectMeta: metav1.ObjectMeta{Name: "ops-db", Namespace: "my-cluster-ns"}}
	server.Status.Version = "17"
	server.Status.Conditions = []metav1.Condition{
		{Type: v1.ConditionReady, Status: metav1.ConditionTrue, Reason: v1.ReasonHealthy},
	}
	c := fakeReader(t, request, server)

	require.NoError(t, s.backing.Reconcile(t.Context(), component.ReconcileContext{
		Client: c, APIReader: c, Scheme: c.Scheme(), Owner: request, EventRecorder: events.NewFakeRecorder(100),
	}))

	assert.Equal(
		t, pluginsv1alpha1.SummaryItem{
			Label: kindDatabaseServer, Value: "ops-db, PostgreSQL 17, Ready", Severity: pluginsv1alpha1.SeverityInfo,
		}, s.report().summary[0],
	)
}

func TestManagementDatabasesReportTheServer(t *testing.T) {
	ready := func(generation int64) []metav1.Condition {
		return []metav1.Condition{{
			Type: v1.ConditionReady, Status: metav1.ConditionTrue, Reason: v1.ReasonHealthy,
			ObservedGeneration: generation,
		}}
	}
	tests := map[string]struct {
		status v1.DatabaseServerStatus
		want   pluginsv1alpha1.SummaryItem
	}{
		"Ready at the current generation": {
			v1.DatabaseServerStatus{ObservedGeneration: 2, Version: "17", Conditions: ready(2)},
			pluginsv1alpha1.SummaryItem{
				Label: kindDatabaseServer, Value: "ops-db, PostgreSQL 17, Ready", Severity: pluginsv1alpha1.SeverityInfo,
			},
		},
		"Ready from an older generation": {
			v1.DatabaseServerStatus{ObservedGeneration: 2, Version: "17", Conditions: ready(1)},
			pluginsv1alpha1.SummaryItem{
				Label: kindDatabaseServer, Value: "ops-db, updating", Severity: pluginsv1alpha1.SeverityWarning,
			},
		},
		"status from an older generation": {
			v1.DatabaseServerStatus{ObservedGeneration: 1, Version: "17", Conditions: ready(2)},
			pluginsv1alpha1.SummaryItem{
				Label: kindDatabaseServer, Value: "ops-db, updating", Severity: pluginsv1alpha1.SeverityWarning,
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			cell := concepts.NewData[v1.DatabaseServer]("server")
			cell.Set(v1.DatabaseServer{
				ObjectMeta: metav1.ObjectMeta{Name: "ops-db", Generation: 2},
				Status:     tt.status,
			})

			assert.Equal(t, tt.want, serverItem("ops-db", cell))
		})
	}
}

func TestManagementDatabasesWarnAboutTheOwnedDatabaseOfARemovedComponent(t *testing.T) {
	request := planeRequest(`["identity"]`)
	request.UID = requestUID
	owned := liveDatabases(liveDatabase("ops-web-modeler", request.UID))

	s, err := managementDatabases{}.build(request, owned)
	require.NoError(t, err)

	assert.Contains(t, s.report().summary, removedItem, "the status of the request does not matter")
}

func TestManagementDatabasesDoNotWarnWithoutAnOwnedDatabase(t *testing.T) {
	tests := map[string]struct {
		databases string
		live      liveObjects
	}{
		"the Database is gone": {`["identity"]`, liveDatabases()},
		"the Database has another owner": {
			`["identity"]`, liveDatabases(liveDatabase("ops-web-modeler", "other-uid")),
		},
		"the component is listed": {
			`["identity","webModeler"]`, liveDatabases(liveDatabase("ops-web-modeler", requestUID)),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			request := planeRequest(tt.databases)
			request.UID = requestUID
			request.Status.Summary = []pluginsv1alpha1.SummaryItem{removedItem}

			s, err := managementDatabases{}.build(request, tt.live)
			require.NoError(t, err)

			assert.NotContains(t, s.report().summary, removedItem)
		})
	}
}

func TestManagementDatabasesTakeTheStorageClassInput(t *testing.T) {
	request := planeRequest(`["identity"]`)
	request.Spec.Inputs = map[string]string{pluginstartup.OutputStorageClassName: "encrypted"}

	server := previewServer(t, buildFor(t, request))

	require.NotNil(t, server.Spec.StorageClassName)
	assert.Equal(t, "encrypted", *server.Spec.StorageClassName)
}

func TestManagementDatabasesSetThePostgreSQLVersion(t *testing.T) {
	request := planeRequest(`["identity"]`)
	assert.Equal(
		t, pluginstartup.ManagementDatabaseServerVersion, previewServer(t, buildFor(t, request)).Spec.Version,
		"an absent version takes the default of the schema",
	)

	request.Spec.Parameters.Raw = []byte(`{"preset":"small","version":"16"}`)
	assert.Equal(t, "16", previewServer(t, buildFor(t, request)).Spec.Version)
}

func TestManagementDatabasesDeleteTheDatabasesBeforeTheServer(t *testing.T) {
	c, err := managementDatabases{}.cleanup(planeRequest(`["identity"]`), pluginsv1alpha1.DeletionPolicyDelete, nil)
	require.NoError(t, err)

	require.Len(t, c.stages, 2)
	assert.Equal(
		t, []string{"Database/ops-keycloak", "Database/ops-identity", "Database/ops-web-modeler"},
		objectNames(c.stages[0].remaining), "every Database, also one that is not listed now",
	)
	assert.Equal(t, []string{"DatabaseServer/ops-db"}, objectNames(c.stages[1].remaining))
}

func TestManagementDatabasesOrphanWaitsUntilNoObjectReferencesTheRequest(t *testing.T) {
	c, err := managementDatabases{}.cleanup(planeRequest(`["identity"]`), pluginsv1alpha1.DeletionPolicyOrphan, nil)
	require.NoError(t, err)

	require.Len(t, c.stages, 1)
	assert.Len(t, c.stages[0].components, 1)
	assert.ElementsMatch(
		t, []string{
			"DatabaseServer/ops-db", "Database/ops-keycloak", "Database/ops-identity", "Database/ops-web-modeler",
		}, objectNames(c.stages[0].remaining), "garbage collection follows an owner reference that stays",
	)
}

func TestManagementDatabasesRefuseAnUnknownParameter(t *testing.T) {
	request := planeRequest(`["identity"]`)
	request.Spec.Parameters.Raw = []byte(`{"preset":"small","databases":["identity"],"Databases":[]}`)

	_, err := managementDatabases{}.build(request, nil)

	var failure *conditions.PreCheckFailure
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, pluginsv1alpha1.ReasonInvalidParameters, failure.Reason)
	assert.Contains(t, failure.Message, "spec.parameters.Databases")
	assert.Nil(t, managementDatabases{}.contracts(request))
}

func TestManagementDatabasesLabelEveryBackingObject(t *testing.T) {
	request := planeRequest(`["keycloak","identity","webModeler"]`)
	objs, err := buildFor(t, request).backing.Preview()
	require.NoError(t, err)

	require.Len(t, objs, 4)
	for _, obj := range objs {
		assert.Equal(
			t, map[string]string{
				pluginsv1alpha1.LabelRegistration: pluginstartup.RegistrationManagementDatabaseServer,
				pluginsv1alpha1.LabelRequest:      request.Name,
				"app.kubernetes.io/managed-by":    "camunda-operator",
			}, obj.GetLabels(), obj.GetName(),
		)
	}
}

func TestManagementDatabasesNameEveryBackingObject(t *testing.T) {
	refs := managementDatabases{}.backing(planeRequest(`[]`))
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		assert.Equal(t, "my-cluster-ns", ref.object.GetNamespace())
		names = append(names, ref.kind+"/"+ref.object.GetName())
	}

	assert.Equal(
		t, []string{
			"DatabaseServer/ops-db", "Database/ops-keycloak", "Database/ops-identity", "Database/ops-web-modeler",
		}, names, "the names do not depend on the parameters",
	)
}

const requestUID = "request-uid"

var removedItem = pluginsv1alpha1.SummaryItem{
	Label:    kindDatabase,
	Value:    "ops-web-modeler removed, the logical database web_modeler stays on ops-db",
	Severity: pluginsv1alpha1.SeverityWarning,
}

// planeRequest returns a request of management-database-server for the
// plane ops with the given databases.
func planeRequest(databases string) *pluginsv1alpha1.PluginRequest {
	request := requestFor(
		pluginstartup.RegistrationManagementDatabaseServer,
		`{"preset":"small","databases":`+databases+`}`,
	)
	request.Spec.ClaimRef = pluginsv1alpha1.ClaimRef{
		Kind: pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim,
		Name: "ops",
	}

	return request
}

// buildFor builds the components of the management databases handler for
// request.
func buildFor(t *testing.T, request *pluginsv1alpha1.PluginRequest) served {
	s, err := managementDatabases{}.build(request, nil)
	require.NoError(t, err)
	return s
}

// liveDatabase returns the Database name in the namespace of planeRequest,
// with an owner reference to uid.
func liveDatabase(name string, uid types.UID) *v1.Database {
	return &v1.Database{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "my-cluster-ns",
		OwnerReferences: []metav1.OwnerReference{{
			APIVersion: pluginsv1alpha1.GroupVersion.String(), Kind: "PluginRequest", Name: "ops", UID: uid,
		}},
	}}
}

// liveDatabases returns databases as the live objects of a request.
func liveDatabases(databases ...*v1.Database) liveObjects {
	live := liveObjects{}
	for _, db := range databases {
		live[liveKey{kind: kindDatabase, name: db.Name}] = db
	}
	return live
}

// previewServer returns the DatabaseServer ops-db that s applies.
func previewServer(t *testing.T, s served) *v1.DatabaseServer {
	res, ok := s.backing.Resource("core.camunda.io/v1/DatabaseServer/my-cluster-ns/ops-db")
	require.True(t, ok)
	previewer, ok := res.(interface{ Preview() (client.Object, error) })
	require.True(t, ok)
	obj, err := previewer.Preview()
	require.NoError(t, err)
	server, ok := obj.(*v1.DatabaseServer)
	require.True(t, ok)
	return server
}

func databaseConfig(name string) *v1.DatabaseConfig {
	return &v1.DatabaseConfig{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "my-cluster-ns"}}
}

func objectNames(objs []client.Object) []string {
	names := make([]string, 0, len(objs))
	for _, o := range objs {
		names = append(names, reflect.TypeOf(o).Elem().Name()+"/"+o.GetName())
	}
	return names
}
