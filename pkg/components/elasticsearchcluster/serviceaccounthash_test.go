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

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/accounthash"
)

// nodePodTemplate returns the pod template of the node set that the
// elasticsearch component renders for cluster and storage.
func nodePodTemplate(
	t *testing.T,
	cluster *v1.ElasticsearchCluster,
	storage *SnapshotStorage,
) corev1.PodTemplateSpec {
	t.Helper()

	_, preset, release := goldenMinimalElasticsearchCluster()
	merged := MergeSpec(cluster.Spec, preset, release)
	comp, err := ElasticsearchComponent(cluster, merged, RequestedStorage{Size: merged.StorageSize}, storage, 0)
	require.NoError(t, err)

	objects, err := comp.Preview()
	require.NoError(t, err)
	for _, obj := range objects {
		if es, ok := obj.(*esv1.Elasticsearch); ok {
			return es.Spec.NodeSets[0].PodTemplate
		}
	}
	require.FailNow(t, "the component renders no Elasticsearch")
	return corev1.PodTemplateSpec{}
}

// annotatedCluster returns the minimal cluster with an account that carries
// annotations.
func annotatedCluster(annotations map[string]string) *v1.ElasticsearchCluster {
	cluster, _, _ := goldenMinimalElasticsearchCluster()
	cluster.Spec.ServiceAccount = &v1.ServiceAccountSpec{Annotations: annotations}
	return cluster
}

// identityBucket returns a snapshot bucket whose workload identity is role.
func identityBucket(role string) *SnapshotStorage {
	return &SnapshotStorage{Config: snapshotBucket(v1.S3StorageAuth{
		Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
		WorkloadIdentity: &v1.S3WorkloadIdentity{RoleARN: role},
	})}
}

// Workload identity reads the annotations of the account when a pod starts,
// so a pod that started before a change keeps the old identity.
func TestAccountAnnotationChangeRollsTheNodes(t *testing.T) {
	t.Parallel()

	old := annotatedCluster(map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/old"})
	updated := annotatedCluster(map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/new"})
	before := nodePodTemplate(t, old, nil)
	after := nodePodTemplate(t, updated, nil)

	require.NotEmpty(t, before.Annotations[accounthash.Annotation])
	assert.NotEqual(t, before, after)
	assert.NotEqual(t, before.Annotations[accounthash.Annotation], after.Annotations[accounthash.Annotation])
}

// A bucket that changes the identity it derives changes the account as much
// as a spec edit does.
func TestBucketIdentityChangeRollsTheNodes(t *testing.T) {
	t.Parallel()

	cluster, _, _ := goldenMinimalElasticsearchCluster()
	cluster.Spec.SnapshotStorageRef = goldenBucketName
	before := nodePodTemplate(t, cluster, identityBucket("arn:aws:iam::123456789012:role/old"))
	after := nodePodTemplate(t, cluster, identityBucket("arn:aws:iam::123456789012:role/moved"))

	require.NotEmpty(t, before.Annotations[accounthash.Annotation])
	assert.NotEqual(t, before.Annotations[accounthash.Annotation], after.Annotations[accounthash.Annotation])
}

// Only a change of the account annotations may move the hash. An edit
// outside the pod template keeps the whole template, and an edit of the pod
// metadata keeps the hash.
func TestUnrelatedEditKeepsTheServiceAccountHash(t *testing.T) {
	t.Parallel()

	annotations := map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/es"}
	before := nodePodTemplate(t, annotatedCluster(annotations), nil)

	outside := annotatedCluster(annotations)
	outside.Spec.Replicas = new(int32(5))
	outside.Spec.StorageSize = new(resource.MustParse("256Gi"))
	assert.Equal(t, before, nodePodTemplate(t, outside, nil))

	metadata := annotatedCluster(annotations)
	metadata.Spec.PodLabels = map[string]string{"team": "platform"}
	metadata.Spec.PodAnnotations = map[string]string{"example.com/note": "unrelated"}
	after := nodePodTemplate(t, metadata, nil)
	assert.Equal(t, before.Annotations[accounthash.Annotation], after.Annotations[accounthash.Annotation])
}

// The operator rolls only for annotations that it renders itself. Without a
// rendered annotation the template carries no hash, so an upgrade of the
// operator does not roll these clusters.
func TestNoServiceAccountHashWithoutRenderedAnnotations(t *testing.T) {
	t.Parallel()

	foreign := map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/foreign"}
	tests := []struct {
		name    string
		account *v1.ServiceAccountSpec
		storage *SnapshotStorage
	}{
		{name: "no account"},
		{name: "an account without annotations", account: &v1.ServiceAccountSpec{}},
		{
			name:    "an annotation-less workload identity",
			storage: identityBucket(""),
		},
		{
			name:    "an account with create false",
			account: &v1.ServiceAccountSpec{Name: "platform-es", Create: new(false), Annotations: foreign},
			storage: identityBucket("arn:aws:iam::123456789012:role/es"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cluster, _, _ := goldenMinimalElasticsearchCluster()
			cluster.Spec.ServiceAccount = tt.account
			template := nodePodTemplate(t, cluster, tt.storage)

			assert.NotContains(t, template.Annotations, accounthash.Annotation)
		})
	}
}

// A user pod annotation must not pin the hash, or removing the last
// annotation of the account would not roll the pods.
func TestUserPodAnnotationCannotSetTheServiceAccountHash(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		annotations map[string]string
	}{
		{name: "an account with annotations", annotations: map[string]string{"example.com/team": "platform"}},
		{name: "an account without annotations", annotations: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cluster := annotatedCluster(tt.annotations)
			cluster.Spec.PodAnnotations = map[string]string{accounthash.Annotation: "pinned"}
			template := nodePodTemplate(t, cluster, nil)

			assert.NotEqual(t, "pinned", template.Annotations[accounthash.Annotation])
		})
	}
}
