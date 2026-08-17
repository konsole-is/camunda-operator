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

package elasticsearchcluster

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/sourcehawk/operator-component-framework/pkg/primitives/secret"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/elasticsearchcluster"
)

// healthyComponent returns a component whose condition is staged True on
// cluster, so conditions.Aggregate over it reports a Ready that would be True.
func healthyComponent(t *testing.T, cluster *v1.ElasticsearchCluster) *component.Component {
	t.Helper()

	resource, err := secret.NewBuilder(&corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}).Build()
	require.NoError(t, err)

	comp, err := component.NewComponentBuilder().
		WithName("test").
		WithConditionType("TestReady").
		WithResource(resource).
		Build()
	require.NoError(t, err)

	meta.SetStatusCondition(&cluster.Status.Conditions, metav1.Condition{
		Type: "TestReady", Status: metav1.ConditionTrue, Reason: "Healthy", Message: "test",
	})

	return comp
}

// repositoryFailure is a failing SnapshotRepositoryReady condition, as the
// registration path builds one.
func repositoryFailure(cluster *v1.ElasticsearchCluster) metav1.Condition {
	return metav1.Condition{
		Type:               components.ConditionSnapshotRepository,
		Status:             metav1.ConditionFalse,
		Reason:             v1.ReasonConnectionFailed,
		Message:            "registering snapshot repository: connection refused",
		ObservedGeneration: cluster.Generation,
	}
}

// A cluster whose components are all healthy but whose repository cannot be
// registered is not ready: its backups have nowhere to go. conditions.Aggregate
// cannot see the repository, because it is not a component, so the override is
// the only place this rule can live.
func TestReadyWithRepositoryOverridesATrueReady(t *testing.T) {
	t.Parallel()

	cluster := &v1.ElasticsearchCluster{}
	comp := healthyComponent(t, cluster)
	failure := repositoryFailure(cluster)

	ready := readyWithRepository(cluster, failure, []*component.Component{comp})

	require.Equal(t, metav1.ConditionFalse, ready.Status)
	assert.Equal(t, v1.ReasonConnectionFailed, ready.Reason)
	assert.Equal(t, failure.Message, ready.Message)

	staged := meta.FindStatusCondition(cluster.Status.Conditions, components.ConditionSnapshotRepository)
	require.NotNil(t, staged, "the repository condition must be staged on the cluster")
	assert.Equal(t, metav1.ConditionFalse, staged.Status)
}

// A healthy repository leaves a True Ready untouched.
func TestReadyWithRepositoryKeepsATrueReadyWhenHealthy(t *testing.T) {
	t.Parallel()

	cluster := &v1.ElasticsearchCluster{}
	comp := healthyComponent(t, cluster)
	healthy := metav1.Condition{
		Type:   components.ConditionSnapshotRepository,
		Status: metav1.ConditionTrue,
		Reason: v1.ReasonHealthy,
	}

	ready := readyWithRepository(cluster, healthy, []*component.Component{comp})

	assert.Equal(t, metav1.ConditionTrue, ready.Status)
}

// The override only fires against a Ready that would otherwise be True: a
// cluster that is already failing keeps the component failure as its reason,
// because that is the failure to fix first.
func TestReadyWithRepositoryKeepsAFalseReadysReason(t *testing.T) {
	t.Parallel()

	cluster := &v1.ElasticsearchCluster{}
	ready := readyWithRepository(cluster, repositoryFailure(cluster), nil)

	assert.NotEqual(t, v1.ReasonConnectionFailed, ready.Reason)
}

// A zero repository condition means the cluster takes no part in backups, so
// a condition left over from an earlier bucket reference is removed rather
// than left asserting stale state.
func TestReadyWithRepositoryDropsAStaleCondition(t *testing.T) {
	t.Parallel()

	cluster := &v1.ElasticsearchCluster{}
	meta.SetStatusCondition(&cluster.Status.Conditions, repositoryFailure(cluster))

	readyWithRepository(cluster, metav1.Condition{}, nil)

	assert.Nil(t, meta.FindStatusCondition(
		cluster.Status.Conditions, components.ConditionSnapshotRepository,
	))
}
