package barmanobjectstore

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"
	"github.com/sourcehawk/operator-component-framework/pkg/mutation/editors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// testObject returns a valid namespaced ObjectStore fixture: an S3 bucket
// with static credentials and a retention of thirty days.
func testObject() *ObjectStore {
	return &ObjectStore{
		ObjectMeta: metav1.ObjectMeta{Name: "test-object", Namespace: "test-ns"},
		Spec: ObjectStoreSpec{
			Configuration: BarmanObjectStoreConfiguration{
				DestinationPath: "s3://test-bucket/databaseserver/test-ns/test-server-9b1c7f04/",
				EndpointURL:     "https://s3.eu-west-1.amazonaws.com",
				S3Credentials: &S3Credentials{
					AccessKeyID:     &SecretKeySelector{Name: "test-archive", Key: "accessKeyId"},
					SecretAccessKey: &SecretKeySelector{Name: "test-archive", Key: "secretAccessKey"},
				},
				Wal:  &WalBackupConfiguration{Compression: CompressionTypeGzip},
				Data: &DataBackupConfiguration{Compression: CompressionTypeGzip},
			},
			RetentionPolicy: "30d",
		},
	}
}

// TestBuilderPreviewsTheObjectStore proves that the rendered object keeps the
// bucket, the credentials, and the retention policy of the base state.
func TestBuilderPreviewsTheObjectStore(t *testing.T) {
	t.Parallel()

	res, err := NewBuilder(testObject()).Build()
	require.NoError(t, err)

	preview, err := res.Preview()
	require.NoError(t, err)

	store, ok := preview.(*ObjectStore)
	require.True(t, ok)
	assert.Equal(
		t,
		"s3://test-bucket/databaseserver/test-ns/test-server-9b1c7f04/",
		store.Spec.Configuration.DestinationPath,
	)
	assert.Equal(t, "30d", store.Spec.RetentionPolicy)
	require.NotNil(t, store.Spec.Configuration.S3Credentials)
	assert.Equal(t, "test-archive", store.Spec.Configuration.S3Credentials.AccessKeyID.Name)
}

func TestBuilderBuildValidation(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		obj         *ObjectStore
		expectedErr string
	}{
		{
			name:        "nil object",
			obj:         nil,
			expectedErr: "object cannot be nil",
		},
		{
			name: "empty name",
			obj: &ObjectStore{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test-ns"},
			},
			expectedErr: "object name cannot be empty",
		},
		{
			name: "empty namespace",
			obj: &ObjectStore{
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
			assert.Equal(t, "barmancloud.cnpg.io/v1/ObjectStore/test-ns/test-object", res.Identity())
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

	cell := concepts.NewData[string]("barmanobjectstore-name")
	builder := NewBuilder(testObject())
	ExtractInto(builder, cell, func(o ObjectStore) (string, error) {
		return o.Name, nil
	})

	res, err := builder.Build()
	require.NoError(t, err)

	produced := res.ProducedData()
	require.Len(t, produced, 1)
	assert.Equal(t, "barmanobjectstore-name", produced[0].Name())

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
