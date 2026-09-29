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

package restore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/storagewriter"
)

const writerNamespace = "operator"

func writerClient(t *testing.T) client.Client {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	return fake.NewClientBuilder().WithScheme(scheme).Build()
}

func writingRestore(name string) *v1.LogicalRestoreRDBMS {
	return &v1.LogicalRestoreRDBMS{
		TypeMeta:   metav1.TypeMeta{Kind: "LogicalRestoreRDBMS"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "apps", Name: name, UID: types.UID("uid-" + name)},
	}
}

func writerLeases(t *testing.T, c client.Client) []coordinationv1.Lease {
	t.Helper()
	var leases coordinationv1.LeaseList
	require.NoError(t, c.List(context.Background(), &leases, client.InNamespace(writerNamespace)))

	return leases.Items
}

func TestOtherWritersListsEveryRegistrationButTheOwnOne(t *testing.T) {
	ctx := context.Background()
	c := writerClient(t)
	backend := "rdbms|db.apps.svc:5432/camunda"
	earlier, next := writingRestore("earlier"), writingRestore("next")
	require.NoError(t, RegisterWriter(ctx, c, c, writerNamespace, backend, earlier, "uid-target"))
	require.NoError(t, RegisterWriter(ctx, c, c, writerNamespace, backend, next, "uid-target"))

	writers, err := OtherWriters(ctx, c, writerNamespace, backend, "", next)

	require.NoError(t, err)
	assert.Equal(t, []string{"LogicalRestoreRDBMS apps/earlier"}, writers)
}

func TestReleaseWritersEndsTheRegistrationsOfTheOwnerOnEveryBackend(t *testing.T) {
	ctx := context.Background()
	c := writerClient(t)
	owner, other := writingRestore("owner"), writingRestore("other")
	require.NoError(t, RegisterWriter(ctx, c, c, writerNamespace, "rdbms|old.apps.svc:5432/camunda", owner, "t"))
	require.NoError(t, RegisterWriter(ctx, c, c, writerNamespace, "rdbms|new.apps.svc:5432/camunda", owner, "t"))
	require.NoError(t, RegisterWriter(ctx, c, c, writerNamespace, "rdbms|old.apps.svc:5432/camunda", other, "t"))

	require.NoError(t, ReleaseWriters(ctx, c, c, writerNamespace, owner))

	leases := writerLeases(t, c)
	require.Len(t, leases, 1)
	assert.Equal(t, "LogicalRestoreRDBMS apps/other", leases[0].Annotations[storagewriter.WriterAnnotation])
}

func TestADatabaseWriterHoldsItsContractAtAMovedAddress(t *testing.T) {
	ctx := context.Background()
	c := writerClient(t)
	contract := DatabaseContract(types.NamespacedName{Namespace: "apps", Name: "pg"}, "camunda")
	earlier, next := writingRestore("earlier"), writingRestore("next")
	require.NoError(t, RegisterDatabaseWriter(
		ctx, c, c, writerNamespace, "rdbms|old.apps.svc:5432/camunda", contract, earlier, "uid-target",
	))

	writers, err := OtherWriters(ctx, c, writerNamespace, "rdbms|new.apps.svc:5432/camunda", contract, next)

	require.NoError(t, err)
	assert.Equal(t, []string{"LogicalRestoreRDBMS apps/earlier"}, writers)
}
