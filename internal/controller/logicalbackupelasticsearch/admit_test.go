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

package logicalbackupelasticsearch

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestStorageMissingSeparatesAGoneContractFromATransientRead(t *testing.T) {
	resource := schema.GroupResource{Group: "core.camunda.io", Resource: "secondarystorageconfigs"}
	tests := []struct {
		name    string
		err     error
		missing bool
	}{
		{
			name:    "the cluster names no contract",
			err:     fmt.Errorf("%w: CamundaCluster ns/cc no longer names a storage contract", errNoStorage),
			missing: true,
		},
		{
			name: "the named contract does not exist",
			err: fmt.Errorf(
				"reading SecondaryStorageConfig ns/storage: %w",
				apierrors.NewNotFound(resource, "storage"),
			),
			missing: true,
		},
		{
			name:    "the API server timed out",
			err:     fmt.Errorf("reading SecondaryStorageConfig ns/storage: %w", apierrors.NewTimeoutError("etcd", 1)),
			missing: false,
		},
		{
			name:    "the read failed for another reason",
			err:     errors.New("connection refused"),
			missing: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.missing, storageMissing(tt.err))
		})
	}
}

func TestClusterReplacedComparesThePinnedUID(t *testing.T) {
	t.Parallel()

	backup := &v1.LogicalBackupElasticsearch{}
	cluster := &v1.CamundaCluster{}
	cluster.UID = "uid-1"

	assert.False(t, clusterReplaced(backup, cluster), "no pin yet: nothing to compare against")
	backup.Status.ClusterUID = "uid-1"
	assert.False(t, clusterReplaced(backup, cluster))
	cluster.UID = "uid-2"
	assert.True(t, clusterReplaced(backup, cluster), "a same-named cluster with another UID is a replacement")
}
