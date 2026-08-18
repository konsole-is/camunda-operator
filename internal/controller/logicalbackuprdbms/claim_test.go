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

package logicalbackuprdbms

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// TestClaimGateBreaksTheTieBreakDeadlock replays the interleaving that the
// sibling controller hit. B is the older backup by the tie-break, but the
// reconcile of A ran first. A claimed the Lease and its id flush failed, so A
// still shows no id. Now B passes the tie-break (A is pending and younger),
// but the Lease blocks it. A re-enters, and the tie-break says that B goes
// first. Without the Holds pre-filter, both wait forever. With it, exactly
// one, the holder, proceeds to allocate.
func TestClaimGateBreaksTheTieBreakDeadlock(t *testing.T) {
	t.Parallel()

	scheme := dumpScheme(t)
	now := metav1.Now()
	older := metav1.NewTime(now.Add(-time.Minute))
	newBackup := func(name, uid string, created metav1.Time) *v1.LogicalBackupRDBMS {
		b := &v1.LogicalBackupRDBMS{
			ObjectMeta: metav1.ObjectMeta{
				Name: name, Namespace: "camunda", CreationTimestamp: created,
			},
			Spec: v1.LogicalBackupRDBMSSpec{ClusterRef: v1.ClusterRef{Name: "my-cluster"}},
		}
		b.UID = types.UID("uid-" + uid)

		return b
	}
	a := newBackup("a", "a", now)
	b := newBackup("b", "b", older)

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(a, b).Build()
	r := &LogicalBackupRDBMSReconciler{Client: c, APIReader: c}
	ctx := context.Background()

	// The reconcile of A ran first, and it claimed the cluster. Its id flush
	// failed, so the store still shows A without an id.
	holder, err := r.claimCluster(ctx, a)
	require.NoError(t, err)
	assert.Empty(t, holder, "A holds the claim")

	// B passes the tie-break (A is pending and younger), but the Lease blocks it.
	blocking, err := r.inProgress(b)(ctx)
	require.NoError(t, err)
	assert.Empty(t, blocking, "the tie-break lets B through")
	holder, err = r.claimCluster(ctx, b)
	require.NoError(t, err)
	assert.Equal(t, "LogicalBackupRDBMS/a", holder, "the Lease names the active holder; no takeover")

	// A re-enters. The tie-break alone sends it behind B forever. The holder
	// goes first.
	blocking, err = r.inProgress(a)(ctx)
	require.NoError(t, err)
	assert.Empty(t, blocking, "the holder passes the pre-filter whatever the tie-break says")
	holder, err = r.claimCluster(ctx, a)
	require.NoError(t, err)
	assert.Empty(t, holder, "the claim is idempotent for its holder")

	// For contrast, the tie-break alone: without the claim, B blocks A.
	assert.True(t, blocks(b, a))
	assert.False(t, blocks(a, b))
}
