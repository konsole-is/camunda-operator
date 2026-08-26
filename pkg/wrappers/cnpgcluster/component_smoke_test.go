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

package cnpgcluster_test

import (
	"flag"
	"testing"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/cnpgcluster"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/cnpgscheduledbackup"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/podmonitor"
)

// updateGolden refreshes the golden manifests with the rendered output:
// go test ./pkg/wrappers/cnpgcluster/ -run Golden -update-golden
//
// The Cluster snapshots carry the empty status sub-structs that the typed
// object always serializes. They are not desired state, and a bump of
// github.com/cloudnative-pg/api can churn them.
var updateGolden = flag.Bool("update-golden", false, "update golden files")

// TestWrappersAssembleIntoComponent proves the four generated wrappers
// register with an ocf component and render through Preview, the way a
// database server controller consumes them.
func TestWrappersAssembleIntoComponent(t *testing.T) {
	t.Parallel()

	comp, err := component.NewComponentBuilder().
		WithName("server").
		WithConditionType("ClusterReady").
		WithResource(clusterResource(t)).
		WithResource(objectStoreResource(t)).
		WithResource(scheduledBackupResource(t)).
		WithResource(podMonitorResource(t)).
		Build()
	require.NoError(t, err)

	objects, err := comp.Preview()
	require.NoError(t, err)
	require.Len(t, objects, 4)

	assert.Equal(t, "my-server", objects[0].GetName())
	assert.Equal(t, "my-archive", objects[1].GetName())
	assert.Equal(t, "my-base-backup", objects[2].GetName())
	assert.Equal(t, "my-metrics", objects[3].GetName())
}

// TestClusterGolden pins the rendered Cluster of a server that archives
// through the Barman Cloud plugin.
func TestClusterGolden(t *testing.T) {
	t.Parallel()

	golden.AssertYAML(
		t, "testdata/golden/cluster.yaml", clusterResource(t),
		golden.WithScheme(goldenScheme(t)), golden.Update(*updateGolden),
	)
}

// TestSuspendedClusterGolden pins the rendered Cluster of a suspended server.
// The diff against cluster.yaml is the hibernation annotation alone, so a
// suspension that reached any other field shows up here.
func TestSuspendedClusterGolden(t *testing.T) {
	t.Parallel()

	res, err := cnpgcluster.NewBuilder(clusterBase()).
		WithMutation(cnpgcluster.Mutation{
			Name:   "suspend",
			Mutate: cnpgcluster.DefaultSuspendMutationHandler,
		}).
		Build()
	require.NoError(t, err)

	golden.AssertYAML(
		t, "testdata/golden/cluster-suspended.yaml", res,
		golden.WithScheme(goldenScheme(t)), golden.Update(*updateGolden),
	)
}

func goldenScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, cnpgv1.AddToScheme(scheme))
	require.NoError(t, barmanobjectstore.AddToScheme(scheme))
	require.NoError(t, monitoringv1.AddToScheme(scheme))

	return scheme
}

func clusterBase() *cnpgv1.Cluster {
	return &cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-server", Namespace: "my-server-ns"},
		Spec: cnpgv1.ClusterSpec{
			Instances:             1,
			ImageName:             "ghcr.io/cloudnative-pg/postgresql:17",
			StorageConfiguration:  cnpgv1.StorageConfiguration{Size: "20Gi"},
			EnableSuperuserAccess: new(true),
			Plugins: []cnpgv1.PluginConfiguration{{
				Name:          "barman-cloud.cloudnative-pg.io",
				IsWALArchiver: new(true),
				Parameters: map[string]string{
					"barmanObjectName": "my-archive",
					"serverName":       "my-server",
				},
			}},
		},
	}
}

func clusterResource(t *testing.T) *cnpgcluster.Resource {
	t.Helper()

	res, err := cnpgcluster.NewBuilder(clusterBase()).Build()
	require.NoError(t, err)

	return res
}

func objectStoreResource(t *testing.T) *barmanobjectstore.Resource {
	t.Helper()

	res, err := barmanobjectstore.NewBuilder(&barmanobjectstore.ObjectStore{
		ObjectMeta: metav1.ObjectMeta{Name: "my-archive", Namespace: "my-server-ns"},
		Spec: barmanobjectstore.ObjectStoreSpec{
			Configuration: barmanobjectstore.BarmanObjectStoreConfiguration{
				DestinationPath: "s3://my-bucket/databaseserver/my-server-ns/my-server-3d5e7a12/",
				EndpointURL:     "https://s3.eu-west-1.amazonaws.com",
				S3Credentials: &barmanobjectstore.S3Credentials{
					AccessKeyID:     &barmanobjectstore.SecretKeySelector{Name: "my-archive", Key: "accessKeyId"},
					SecretAccessKey: &barmanobjectstore.SecretKeySelector{Name: "my-archive", Key: "secretAccessKey"},
				},
				Wal:  &barmanobjectstore.WalBackupConfiguration{Compression: barmanobjectstore.CompressionTypeGzip},
				Data: &barmanobjectstore.DataBackupConfiguration{Compression: barmanobjectstore.CompressionTypeGzip},
			},
			RetentionPolicy: "30d",
		},
	}).Build()
	require.NoError(t, err)

	return res
}

func scheduledBackupResource(t *testing.T) *cnpgscheduledbackup.Resource {
	t.Helper()

	res, err := cnpgscheduledbackup.NewBuilder(&cnpgv1.ScheduledBackup{
		ObjectMeta: metav1.ObjectMeta{Name: "my-base-backup", Namespace: "my-server-ns"},
		Spec: cnpgv1.ScheduledBackupSpec{
			Schedule:            "0 0 2 * * *",
			Immediate:           new(true),
			Method:              cnpgv1.BackupMethodPlugin,
			PluginConfiguration: &cnpgv1.BackupPluginConfiguration{Name: "barman-cloud.cloudnative-pg.io"},
			Cluster:             cnpgv1.LocalObjectReference{Name: "my-server"},
		},
	}).Build()
	require.NoError(t, err)

	return res
}

func podMonitorResource(t *testing.T) *podmonitor.Resource {
	t.Helper()

	res, err := podmonitor.NewBuilder(&monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{Name: "my-metrics", Namespace: "my-server-ns"},
		Spec: monitoringv1.PodMonitorSpec{
			Selector: metav1.LabelSelector{
				MatchLabels: map[string]string{"cnpg.io/cluster": "my-server"},
			},
			PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{Port: new("metrics")}},
		},
	}).Build()
	require.NoError(t, err)

	return res
}
