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
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
	camundacluster "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/logicalbackup"
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

// zeebeWorkload builds the Zeebe workload of cluster ns/cc with the given
// config hash and Elasticsearch endpoint on its pod template.
func zeebeWorkload(hash, endpoint string) *appsv1.StatefulSet {
	cluster := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cc"}}
	workload := &appsv1.StatefulSet{ObjectMeta: metav1.ObjectMeta{
		Namespace: "ns", Name: camundacluster.WorkloadName(cluster, camundacluster.ComponentZeebe),
	}}
	if hash != "" {
		workload.Spec.Template.Annotations = map[string]string{camundacluster.ConfigHashAnnotation: hash}
	}
	workload.Spec.Template.Spec.Containers = []corev1.Container{{
		Name: "zeebe",
		Env:  []corev1.EnvVar{camundaconfig.Var(camundaconfig.KeyElasticsearchURL, endpoint)},
	}}
	return workload
}

// workloadReader is a fake API reader that holds objects.
func workloadReader(t *testing.T, objects ...client.Object) client.Reader {
	t.Helper()
	s := runtime.NewScheme()
	require.NoError(t, appsv1.AddToScheme(s))
	return fake.NewClientBuilder().WithScheme(s).WithObjects(objects...).Build()
}

func TestZeebeRunsDestinationPinsTheHashOnlyWhenZeebeRunsTheDeclaredEndpoint(t *testing.T) {
	res := &logicalbackup.PreCheckResult{
		Cluster: &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "cc"}},
		Storage: &v1.SecondaryStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Namespace: "ns", Name: "storage"},
			Spec: v1.SecondaryStorageConfigSpec{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "https://es-new:9200/"},
			},
		},
	}
	tests := []struct {
		name     string
		workload *appsv1.StatefulSet
		hash     string
		wait     string
	}{
		{
			name:     "Zeebe runs the declared endpoint, with a trailing slash of difference",
			workload: zeebeWorkload("hash-2", "https://es-new:9200"),
			hash:     "hash-2",
		},
		{
			name:     "Zeebe still runs the old endpoint",
			workload: zeebeWorkload("hash-1", "https://es-old:9200"),
			wait:     "https://es-old:9200",
		},
		{
			name:     "the template carries no hash yet",
			workload: zeebeWorkload("", "https://es-new:9200"),
			wait:     "no config hash",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Reconciler{APIReader: workloadReader(t, tt.workload)}

			hash, failure, err := r.zeebeRunsDestination(t.Context(), res)
			require.NoError(t, err)
			if tt.wait == "" {
				assert.Nil(t, failure)
				assert.Equal(t, tt.hash, hash)
				return
			}
			require.NotNil(t, failure)
			assert.Equal(t, v1.ReasonProgressing, failure.Reason)
			assert.Contains(t, failure.Message, tt.wait)
			assert.Empty(t, hash, "nothing to pin while the backup waits")
		})
	}
}
