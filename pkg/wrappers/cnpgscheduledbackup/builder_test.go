package cnpgscheduledbackup

import (
	"testing"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/editors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// barmanPluginName is the CNPG-I plugin that takes a base backup to an
// ObjectStore.
const barmanPluginName = "barman-cloud.cloudnative-pg.io"

// testObject returns a valid namespaced ScheduledBackup fixture: a nightly
// base backup of one Cluster through the Barman Cloud plugin.
func testObject() *cnpgv1.ScheduledBackup {
	return &cnpgv1.ScheduledBackup{
		ObjectMeta: metav1.ObjectMeta{Name: "test-object", Namespace: "test-ns"},
		Spec: cnpgv1.ScheduledBackupSpec{
			Schedule:            "0 0 2 * * *",
			Immediate:           new(true),
			Method:              cnpgv1.BackupMethodPlugin,
			PluginConfiguration: &cnpgv1.BackupPluginConfiguration{Name: barmanPluginName},
			Cluster:             cnpgv1.LocalObjectReference{Name: "test-server"},
		},
	}
}

// TestBuilderPreviewsTheScheduledBackup proves that the rendered object keeps
// the plugin method the archive needs.
func TestBuilderPreviewsTheScheduledBackup(t *testing.T) {
	t.Parallel()

	res, err := NewBuilder(testObject()).Build()
	require.NoError(t, err)

	preview, err := res.Preview()
	require.NoError(t, err)

	backup, ok := preview.(*cnpgv1.ScheduledBackup)
	require.True(t, ok)
	assert.Equal(t, "0 0 2 * * *", backup.Spec.Schedule)
	assert.Equal(t, cnpgv1.BackupMethodPlugin, backup.Spec.Method)
	require.NotNil(t, backup.Spec.PluginConfiguration)
	assert.Equal(t, barmanPluginName, backup.Spec.PluginConfiguration.Name)
	assert.Equal(t, "test-server", backup.Spec.Cluster.Name)
}

func TestBuilderBuildValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		obj         *cnpgv1.ScheduledBackup
		expectedErr string
	}{
		{
			name:        "nil object",
			obj:         nil,
			expectedErr: "object cannot be nil",
		},
		{
			name: "empty name",
			obj: &cnpgv1.ScheduledBackup{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test-ns"},
			},
			expectedErr: "object name cannot be empty",
		},
		{
			name: "empty namespace",
			obj: &cnpgv1.ScheduledBackup{
				ObjectMeta: metav1.ObjectMeta{Name: "test-object"},
			},
			expectedErr: "object namespace cannot be empty",
		},
		{
			name: "valid object",
			obj:  testObject(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			res, err := NewBuilder(tt.obj).Build()
			if tt.expectedErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.expectedErr)
				assert.Nil(t, res)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, res)
			assert.Equal(t, "postgresql.cnpg.io/v1/ScheduledBackup/test-ns/test-object", res.Identity())
		})
	}
}

func TestMutationAppliesThroughMutator(t *testing.T) {
	t.Parallel()

	res, err := NewBuilder(testObject()).
		WithMutation(Mutation{
			Name: "scaffolded-label",
			Mutate: func(m *Mutator) error {
				m.EditObjectMetadata(func(e *editors.ObjectMetaEditor) error {
					e.EnsureLabel("scaffolded-by", "ocf")
					return nil
				})
				return nil
			},
		}).
		Build()
	require.NoError(t, err)
	assert.Equal(t, []string{"scaffolded-label"}, res.RegisteredMutations())

	current := testObject()
	require.NoError(t, res.Mutate(current))
	assert.Equal(t, "ocf", current.Labels["scaffolded-by"])
}

func TestExtractIntoDeclaredExtraction(t *testing.T) {
	t.Parallel()

	cell := concepts.NewData[string]("cnpgscheduledbackup-name")
	builder := NewBuilder(testObject())
	ExtractInto(builder, cell, func(o cnpgv1.ScheduledBackup) (string, error) {
		return o.Name, nil
	})

	res, err := builder.Build()
	require.NoError(t, err)

	produced := res.ProducedData()
	require.Len(t, produced, 1)
	assert.Equal(t, "cnpgscheduledbackup-name", produced[0].Name())

	require.NoError(t, res.ExtractData())
	value, ok := cell.Get()
	assert.True(t, ok)
	assert.Equal(t, "test-object", value)
}

func TestWithDataGuardAndOptionalDataDeclarations(t *testing.T) {
	t.Parallel()

	guarded := concepts.NewData[string]("db-host")
	optional := concepts.NewData[string]("db-port")

	res, err := NewBuilder(testObject()).
		WithDataGuard(guarded).
		WithOptionalData(optional).
		Build()
	require.NoError(t, err)

	consumed := res.ConsumedData()
	require.Len(t, consumed, 2)
	assert.Equal(t, "db-host", consumed[0].Cell.Name())
	assert.False(t, consumed[0].Optional)
	assert.Equal(t, "db-port", consumed[1].Cell.Name())
	assert.True(t, consumed[1].Optional)

	status, err := res.GuardStatus()
	require.NoError(t, err)
	assert.Equal(t, concepts.GuardStatusBlocked, status.Status)
	assert.Equal(t, `waiting for data "db-host"`, status.Reason)

	guarded.Set("postgres.default.svc")
	status, err = res.GuardStatus()
	require.NoError(t, err)
	assert.Equal(t, concepts.GuardStatusUnblocked, status.Status)
}
