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
	"testing"

	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/cnpgcluster"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/cnpgscheduledbackup"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/podmonitor"
)

// TestWrappersAssembleIntoComponent proves the four generated wrappers
// register with an ocf component and render through Preview, the way a
// database server controller consumes them.
func TestWrappersAssembleIntoComponent(t *testing.T) {
	t.Parallel()

	cluster, err := cnpgcluster.NewBuilder(&cnpgv1.Cluster{
		ObjectMeta: metav1.ObjectMeta{Name: "my-server", Namespace: "my-server-ns"},
		Spec:       cnpgv1.ClusterSpec{Instances: 1},
	}).Build()
	require.NoError(t, err)

	store, err := barmanobjectstore.NewBuilder(&barmanobjectstore.ObjectStore{
		ObjectMeta: metav1.ObjectMeta{Name: "my-archive", Namespace: "my-server-ns"},
		Spec: barmanobjectstore.ObjectStoreSpec{
			Configuration: barmanobjectstore.BarmanObjectStoreConfiguration{
				DestinationPath: "s3://my-bucket/databaseserver/my-server-ns/my-server/",
			},
		},
	}).Build()
	require.NoError(t, err)

	backup, err := cnpgscheduledbackup.NewBuilder(&cnpgv1.ScheduledBackup{
		ObjectMeta: metav1.ObjectMeta{Name: "my-base-backup", Namespace: "my-server-ns"},
		Spec:       cnpgv1.ScheduledBackupSpec{Schedule: "0 0 2 * * *"},
	}).Build()
	require.NoError(t, err)

	monitor, err := podmonitor.NewBuilder(&monitoringv1.PodMonitor{
		ObjectMeta: metav1.ObjectMeta{Name: "my-metrics", Namespace: "my-server-ns"},
	}).Build()
	require.NoError(t, err)

	comp, err := component.NewComponentBuilder().
		WithName("server").
		WithConditionType("ClusterReady").
		WithResource(cluster).
		WithResource(store).
		WithResource(backup).
		WithResource(monitor).
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
