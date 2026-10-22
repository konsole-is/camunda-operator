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
	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
	servercomponents "github.com/konsole-is/camunda-operator/pkg/components/databaseserver"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

func TestPostgreSQLGolden(t *testing.T) {
	archived := livePreset(&v1.DatabaseServerArchiveSpec{
		ObjectStorageRef: "fleet-backups", RetentionPeriodDays: 7, BaseBackupSchedule: "0 0 3 * * *",
	})
	tests := []struct {
		name, params string
		inputs       map[string]string
		live         liveObjects
	}{
		{"preset", `{"preset":"small","releaseRef":"camunda-8-9"}`, nil, nil},
		{
			"overrides",
			`{"preset":"small","releaseRef":"camunda-8-9","overrides":{"instances":3,` +
				`"resources":{"limits":{"memory":"2Gi"}},"storageSize":"30Gi",` +
				`"scheduling":{"tolerations":[{"key":"pool","value":"pg"}]},"podLabels":{"team":"a"}}}`,
			nil,
			nil,
		},
		{
			"inputs",
			`{"preset":"small","releaseRef":"camunda-8-9"}`,
			map[string]string{
				"storageClassName":          "encrypted",
				"objectStorageConfig":       "orders-backup",
				"serviceAccountAnnotations": `{"eks.amazonaws.com/role-arn":"arn:aws:iam::1:role/orders"}`,
			},
			archived,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s, err := postgresql{}.build(postgresRequest(tt.params, tt.inputs), tt.live)
			require.NoError(t, err)

			golden.AssertComponentYAML(
				t, "testdata/golden/postgresql-"+tt.name+".yaml", s.backing,
				golden.WithScheme(goldenScheme(t)), golden.Update(*updateGolden),
			)
		})
	}
}

func TestPostgreSQLWritesTheServerAndTheDatabaseOfTheClaim(t *testing.T) {
	request := postgresRequest(`{"preset":"small","releaseRef":"camunda-8-9"}`, nil)

	s, err := postgresql{}.build(request, nil)
	require.NoError(t, err)

	server, db := renderedPostgres(t, s)
	assert.Equal(t, "orders-pg", server.Name)
	assert.Equal(t, request.Namespace, server.Namespace)
	assert.Equal(t, "small", server.Spec.PresetRef)
	assert.Equal(t, "camunda-8-9", server.Spec.ReleaseRef)
	assert.Equal(t, "orders-pg", server.Spec.DatabaseServerConfig)
	assert.Equal(t, "orders", db.Name)
	assert.Equal(t, request.Namespace, db.Namespace)
	assert.Equal(
		t, v1.DatabaseSpec{
			ServerRef:              "orders-pg",
			DatabaseName:           "camunda",
			DatabaseConfig:         "orders",
			SecondaryStorageConfig: "orders-pg",
		}, db.Spec,
	)
	for _, obj := range []client.Object{server, db} {
		assert.Equal(
			t, map[string]string{
				pluginsv1alpha1.LabelRegistration: pluginstartup.RegistrationDatabaseServer,
				pluginsv1alpha1.LabelRequest:      request.Name,
				"app.kubernetes.io/managed-by":    "camunda-operator",
			}, obj.GetLabels(), obj.GetName(),
		)
	}
}

func TestPostgreSQLServerPassesTheValidationOfItsKind(t *testing.T) {
	s, err := postgresql{}.build(postgresRequest(`{"preset":"small","releaseRef":"camunda-8-9"}`, nil), nil)
	require.NoError(t, err)
	server, _ := renderedPostgres(t, s)
	preset := v1.DatabaseServerPresetSpec{Server: v1.DatabaseServerSpec{StorageSize: new(resource.MustParse("10Gi"))}}
	release := v1.CamundaReleaseSpec{Version: "8.9.0", DatabaseServer: &v1.ReleaseDatabaseServerSpec{Version: "17"}}

	assert.NoError(t, servercomponents.ValidateMerged(servercomponents.MergeSpec(server.Spec, &preset, &release)))
}

func TestPostgreSQLKeepsTheLiveStorageSize(t *testing.T) {
	tests := []struct {
		name, asked, live, volume, wantSize string
		wantItem                            bool
	}{
		{"smaller", "20Gi", "50Gi", "", "50Gi", true},
		{"larger", "80Gi", "50Gi", "", "80Gi", false},
		{"no live size", "20Gi", "", "", "20Gi", false},
		{"no override, live 50Gi", "", "50Gi", "", "", false},
		{"smaller than the volume of a preset size", "20Gi", "", "50Gi", "50Gi", true},
		{"larger than the data volume, smaller than the WAL volume", "30Gi", "20Gi", "20Gi", "30Gi", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			params := `{"preset":"small","releaseRef":"camunda-8-9"}`
			if tt.asked != "" {
				params = `{"preset":"small","releaseRef":"camunda-8-9","overrides":{"storageSize":"` + tt.asked + `"}}`
			}
			current := &v1.DatabaseServer{ObjectMeta: metav1.ObjectMeta{Name: "orders-pg"}}
			if tt.live != "" {
				current.Spec.StorageSize = new(resource.MustParse(tt.live))
			}
			if tt.volume != "" {
				current.Status.Volumes = []v1.VolumeStatus{
					{Name: "orders-pg-1", Capacity: resource.MustParse(tt.volume)},
				}
			}
			current.Status.Volumes = append(
				current.Status.Volumes,
				v1.VolumeStatus{Name: "orders-pg-1-wal", Capacity: resource.MustParse("100Gi")},
			)
			live := liveObjects{{kind: kindDatabaseServer, name: "orders-pg"}: current}

			s, err := postgresql{}.build(postgresRequest(params, nil), live)
			require.NoError(t, err)

			server, _ := renderedPostgres(t, s)
			if tt.wantSize == "" {
				assert.Nil(t, server.Spec.StorageSize)
			} else {
				require.NotNil(t, server.Spec.StorageSize)
				assert.Equal(t, tt.wantSize, server.Spec.StorageSize.String())
			}
			if tt.wantItem {
				assert.Contains(t, s.report().summary, pluginsv1alpha1.SummaryItem{
					Label:    "Storage size",
					Value:    "Storage size 20Gi is smaller than 50Gi, kept 50Gi",
					Severity: pluginsv1alpha1.SeverityWarning,
				})
			} else {
				for _, item := range s.report().summary {
					assert.NotEqual(t, "Storage size", item.Label)
				}
			}
		})
	}
}

func TestPostgreSQLSetsTheInputs(t *testing.T) {
	request := postgresRequest(`{"preset":"small","releaseRef":"camunda-8-9"}`, map[string]string{
		"storageClassName":          "encrypted",
		"objectStorageConfig":       "orders-backup",
		"serviceAccountAnnotations": `{"iam.gke.io/gcp-service-account":"orders@p.iam"}`,
	})
	live := livePreset(&v1.DatabaseServerArchiveSpec{
		ObjectStorageRef: "fleet-backups", RetentionPeriodDays: 7, BaseBackupSchedule: "0 0 3 * * *",
	})

	s, err := postgresql{}.build(request, live)
	require.NoError(t, err)

	server, _ := renderedPostgres(t, s)
	assert.Equal(t, new("encrypted"), server.Spec.StorageClassName)
	assert.Equal(
		t, &v1.DatabaseServerArchiveSpec{
			ObjectStorageRef: "orders-backup", RetentionPeriodDays: 7, BaseBackupSchedule: "0 0 3 * * *",
		}, server.Spec.Archive,
	)
	assert.Equal(
		t, &v1.DatabaseServerServiceAccountSpec{
			Annotations: map[string]string{"iam.gke.io/gcp-service-account": "orders@p.iam"},
		}, server.Spec.ServiceAccount,
	)
	for _, item := range s.report().summary {
		assert.NotEqual(t, "Backup storage", item.Label)
	}
}

func TestPostgreSQLLeavesTheArchiveToThePresetWithoutTheBucketInput(t *testing.T) {
	live := livePreset(&v1.DatabaseServerArchiveSpec{ObjectStorageRef: "fleet-backups", RetentionPeriodDays: 7})

	s, err := postgresql{}.build(postgresRequest(`{"preset":"small","releaseRef":"camunda-8-9"}`, nil), live)
	require.NoError(t, err)

	server, _ := renderedPostgres(t, s)
	assert.Nil(t, server.Spec.Archive)
}

func TestPostgreSQLSkipsTheBucketWithoutTheArchiveOfThePreset(t *testing.T) {
	tests := []struct {
		name string
		live liveObjects
		want string
	}{
		{"no preset", nil, "orders-backup is not used: the DatabaseServerPreset small does not exist"},
		{
			"preset without archive",
			livePreset(nil),
			"orders-backup is not used: the DatabaseServerPreset small sets no archive retention",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			request := postgresRequest(
				`{"preset":"small","releaseRef":"camunda-8-9"}`,
				map[string]string{"objectStorageConfig": "orders-backup"},
			)

			s, err := postgresql{}.build(request, tt.live)
			require.NoError(t, err)

			server, _ := renderedPostgres(t, s)
			assert.Nil(t, server.Spec.Archive)
			assert.Contains(t, s.report().summary, pluginsv1alpha1.SummaryItem{
				Label: "Backup storage", Value: tt.want, Severity: pluginsv1alpha1.SeverityWarning,
			})
		})
	}
}

func TestPostgreSQLTreatsAnEmptyInputAsAbsent(t *testing.T) {
	request := postgresRequest(`{"preset":"small","releaseRef":"camunda-8-9"}`, map[string]string{
		"storageClassName": "", "objectStorageConfig": "", "serviceAccountAnnotations": "",
	})

	s, err := postgresql{}.build(request, nil)
	require.NoError(t, err)

	server, _ := renderedPostgres(t, s)
	assert.Nil(t, server.Spec.StorageClassName)
	assert.Nil(t, server.Spec.Archive)
	assert.Nil(t, server.Spec.ServiceAccount)
}

func TestPostgreSQLRefusesParametersThatDoNotDecode(t *testing.T) {
	tests := []struct{ params, field string }{
		{`{"preset":"small","overrides":{"storageSize":"ten gigs"}}`, "storageSize"},
		{`{"preset":"small","overrides":{"resources":{"cpu":"1"}}}`, "cpu"},
		{`{"preset":"small","overrides":{"replicas":3}}`, "replicas"},
		{`{"preset":"small","version":"17"}`, "version"},
	}

	for _, tt := range tests {
		t.Run(tt.field, func(t *testing.T) {
			_, err := postgresql{}.build(postgresRequest(tt.params, nil), nil)

			var failure *conditions.PreCheckFailure
			require.ErrorAs(t, err, &failure)
			assert.Equal(t, pluginsv1alpha1.ReasonInvalidParameters, failure.Reason)
			assert.Contains(t, failure.Message, tt.field)
		})
	}
}

func TestPostgreSQLRefusesAnnotationsThatAreNotAnObject(t *testing.T) {
	for _, value := range []string{`["a"]`, `{"a":1}`, `role`, `null`, `{"a":null}`} {
		t.Run(value, func(t *testing.T) {
			request := postgresRequest(`{"preset":"small"}`, map[string]string{"serviceAccountAnnotations": value})

			_, err := postgresql{}.build(request, nil)

			var failure *conditions.PreCheckFailure
			require.ErrorAs(t, err, &failure)
			assert.Equal(t, pluginsv1alpha1.ReasonInvalidParameters, failure.Reason)
			assert.Contains(t, failure.Message, "spec.inputs.serviceAccountAnnotations")
		})
	}
}

func TestPostgreSQLReadsTheServerTheDatabaseAndThePreset(t *testing.T) {
	tests := []struct {
		name, params string
		want         []string
	}{
		{"parameters", `{"preset":"small"}`, []string{
			"DatabaseServer my-cluster-ns/orders-pg", "Database my-cluster-ns/orders", "DatabaseServerPreset /small",
		}},
		{"a preset name that is not an object name", `{"preset":"default/small"}`, []string{
			"DatabaseServer my-cluster-ns/orders-pg", "Database my-cluster-ns/orders",
		}},
		{"parameters that do not decode", `{"preset":"small","size":1}`, []string{
			"DatabaseServer my-cluster-ns/orders-pg", "Database my-cluster-ns/orders",
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := postgresql{}.backing(postgresRequest(tt.params, nil))
			got := make([]string, 0, len(refs))
			for _, ref := range refs {
				got = append(got, ref.kind+" "+ref.object.GetNamespace()+"/"+ref.object.GetName())
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestPostgreSQLPublishesTheSecondaryStorageConfigOfTheServer(t *testing.T) {
	request := postgresRequest(`{"preset":"small"}`, nil)
	want := contractRef{
		output: pluginstartup.OutputSecondaryStorageConfig,
		kind:   kindSecondaryStorageConfig,
		name:   "orders-pg",
	}

	assert.Equal(t, []contractRef{want}, postgresql{}.contracts(request))
	assert.Empty(t, postgresql{}.contracts(postgresRequest(`{"preset":"small","size":1}`, nil)))

	s, err := postgresql{}.build(request, nil)
	require.NoError(t, err)
	reconcileOutputs(t, s, request, fakeReader(t))
	assert.NotEqual(t, metav1.ConditionTrue, s.outputs.GetCondition(request).Status)

	ssc := &v1.SecondaryStorageConfig{ObjectMeta: metav1.ObjectMeta{Name: "orders-pg", Namespace: request.Namespace}}
	reconcileOutputs(t, s, request, fakeReader(t, ssc))
	assert.Equal(t, metav1.ConditionTrue, s.outputs.GetCondition(request).Status)
	assert.Equal(t, map[string]string{"secondaryStorageConfig": "orders-pg"}, s.report().outputs)
}

func TestPostgreSQLReportsTheResourcesAndThePods(t *testing.T) {
	s, err := postgresql{}.build(postgresRequest(`{"preset":"small"}`, nil), nil)
	require.NoError(t, err)

	rep := s.report()
	assert.Equal(
		t, []pluginsv1alpha1.ResourceRef{
			{Kind: "DatabaseServer", Name: "orders-pg", OnOrphan: pluginsv1alpha1.OnOrphanKept},
			{Kind: "Database", Name: "orders", OnOrphan: pluginsv1alpha1.OnOrphanKept},
		}, rep.resources,
	)
	assert.Equal(t, map[string]string{"camunda.io/database-server": "orders-pg"}, rep.podSelector.MatchLabels)
	assert.Equal(
		t, []pluginsv1alpha1.SummaryItem{
			{Label: "DatabaseServer", Value: "orders-pg, waiting", Severity: pluginsv1alpha1.SeverityWarning},
			{Label: "Database", Value: "orders, waiting", Severity: pluginsv1alpha1.SeverityWarning},
		}, rep.summary,
	)
}

func TestPostgreSQLDeleteRemovesTheDatabaseBeforeTheServer(t *testing.T) {
	c, err := postgresql{}.cleanup(
		postgresRequest(`{"preset":"small"}`, nil),
		pluginsv1alpha1.DeletionPolicyDelete,
		nil,
	)
	require.NoError(t, err)

	require.Len(t, c.stages, 2)
	assert.Equal(t, []string{"Database my-cluster-ns/orders"}, remainingOf(c.stages[0]))
	assert.Equal(t, []string{"DatabaseServer my-cluster-ns/orders-pg"}, remainingOf(c.stages[1]))
}

func TestPostgreSQLOrphanWaitsForBothOwnerReferencesToGo(t *testing.T) {
	c, err := postgresql{}.cleanup(
		postgresRequest(`{"preset":"small"}`, nil),
		pluginsv1alpha1.DeletionPolicyOrphan,
		nil,
	)
	require.NoError(t, err)

	require.Len(t, c.stages, 1)
	assert.Equal(
		t, []string{"Database my-cluster-ns/orders", "DatabaseServer my-cluster-ns/orders-pg"},
		remainingOf(c.stages[0]),
	)
}

func TestPostgreSQLDeleteKeepsTheServerWhileTheDatabaseRemains(t *testing.T) {
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

	waiting, err := r.finalize(t.Context(), rec, request, postgresql{})
	require.NoError(t, err)

	assert.True(t, waiting)
	var live v1.DatabaseServer
	require.NoError(t, c.Get(t.Context(), client.ObjectKeyFromObject(server), &live))
	assert.True(t, live.DeletionTimestamp.IsZero())
}

// postgresRequest returns a database-server request of the claim orders with
// params and inputs.
func postgresRequest(params string, inputs map[string]string) *pluginsv1alpha1.PluginRequest {
	request := requestFor(pluginstartup.RegistrationDatabaseServer, params)
	request.Spec.ClaimRef.Name = "orders"
	request.Spec.Inputs = inputs

	return request
}

// livePreset returns the live objects with the DatabaseServerPreset small
// that carries archive.
func livePreset(archive *v1.DatabaseServerArchiveSpec) liveObjects {
	preset := &v1.DatabaseServerPreset{ObjectMeta: metav1.ObjectMeta{Name: "small"}}
	preset.Spec.Server.Archive = archive

	return liveObjects{{kind: kindDatabaseServerPreset, name: "small"}: preset}
}

// renderedPostgres returns the DatabaseServer and the Database that the
// backing component of s applies.
func renderedPostgres(t *testing.T, s served) (*v1.DatabaseServer, *v1.Database) {
	objs, err := s.backing.Preview()
	require.NoError(t, err)
	require.Len(t, objs, 2)
	server, ok := objs[0].(*v1.DatabaseServer)
	require.True(t, ok)
	db, ok := objs[1].(*v1.Database)
	require.True(t, ok)

	return server, db
}

func remainingOf(stage cleanupStage) []string {
	names := make([]string, 0, len(stage.remaining))
	for _, obj := range stage.remaining {
		kind := "Database"
		if _, ok := obj.(*v1.DatabaseServer); ok {
			kind = "DatabaseServer"
		}
		names = append(names, kind+" "+obj.GetNamespace()+"/"+obj.GetName())
	}

	return names
}
