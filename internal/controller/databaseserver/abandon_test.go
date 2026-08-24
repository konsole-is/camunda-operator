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
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// A recovery that has to be given up needs a cluster to give the server back
// to, and a record that names none is refused rather than answered.
//
// The reconciler carries no client here. Answering reaches one, so a run that
// gets past the refusal fails on the missing client rather than passing.
func TestAbandonRecoveryNeedsSomewhereToGoBack(t *testing.T) {
	t.Parallel()

	server := &v1.DatabaseServer{
		ObjectMeta: metav1.ObjectMeta{Name: "camunda", Namespace: "camunda-ns"},
		Status: v1.DatabaseServerStatus{
			Cluster: "camunda-r1",
			Recovery: &v1.DatabaseServerRecoveryStatus{
				RequestID:   "3f2b1c4d-5e6a-4b7c-8d9e-0f1a2b3c4d5e",
				Contract:    "camunda",
				RequestedBy: "camunda-ns/pitr-1",
				TargetTime:  "2026-08-20T14:30:00Z",
				Cluster:     "camunda-r1",
			},
		},
	}
	contract := &v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "camunda", Namespace: "camunda-ns"},
	}

	err := (&DatabaseServerReconciler{}).abandonRecovery(
		context.Background(),
		server,
		contract,
		recordedRequest(server.Status.Recovery),
		"camunda-ns/camunda-r1 was removed",
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "records no cluster to go back to")

	// The server stays where it was, so nothing after this reads the cluster
	// it abandons as the one to keep.
	assert.Equal(t, "camunda-r1", server.Status.Cluster)
	assert.Nil(t, server.Status.Recovery.CompletedAt)
}
