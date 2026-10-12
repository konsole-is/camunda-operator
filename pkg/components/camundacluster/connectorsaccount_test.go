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

package camundacluster

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// connectorsInput returns the minimal cluster with connectors enabled and,
// when bucket is set, that bucket wired in as the backup bucket, after mutate
// edits the cluster spec.
func connectorsInput(t *testing.T, bucket *v1.ObjectStorageConfig, mutate func(*v1.CamundaClusterSpec)) Input {
	t.Helper()

	return newInput(t, func(in *Input) {
		in.Cluster.Spec.Connectors = &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"}
		if mutate != nil {
			mutate(&in.Cluster.Spec)
		}

		if bucket != nil {
			in.Cluster.Spec.BackupStorageRef = bucket.Name
			in.Backup = bucket
			in.Storage.Elasticsearch.SnapshotRepository = fixtureSnapshotRepository
			annotations, err := DerivedServiceAccountAnnotations(bucket)
			require.NoError(t, err)
			in.ServiceAccountAnnotations = annotations
		}
		in.Effective = NewEffective(MergeSpec(in.Cluster.Spec, nil, nil))
	})
}

// azureBucket returns an Azure Blob contract with workload identity. An
// empty clientID names no identity, as for a binding on the cloud side.
func azureBucket(clientID string) *v1.ObjectStorageConfig {
	return &v1.ObjectStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "my-backup-config"},
		Spec: v1.ObjectStorageConfigSpec{
			Type: v1.ObjectStorageTypeAzureBlob,
			AzureBlob: &v1.AzureBlobStorage{
				AccountName: "camundabackups",
				Container:   "backups",
				Auth: v1.AzureBlobStorageAuth{
					Type:             v1.ObjectStorageAuthTypeWorkloadIdentity,
					WorkloadIdentity: &v1.AzureBlobWorkloadIdentity{ClientID: clientID},
				},
			},
		},
	}
}

// renderedObjects returns the previewed objects of the named process.
func renderedObjects(t *testing.T, in Input, process string) []client.Object {
	t.Helper()

	comps, err := Build(in)
	require.NoError(t, err)
	for _, pc := range comps {
		if pc.Process.Component == process {
			return previewObjects(t, pc.Component)
		}
	}
	require.Failf(t, "process missing", "no component for %s", process)
	return nil
}

// renderedAccount returns the ServiceAccount among objects, or nil.
func renderedAccount(objects []client.Object) *corev1.ServiceAccount {
	for _, obj := range objects {
		if account, ok := obj.(*corev1.ServiceAccount); ok {
			return account
		}
	}
	return nil
}

// The connectors run the calls that process authors model, so the identity
// of the cluster, which reads and deletes backups, must never reach them.
func TestConnectorsRunUnderTheDefaultAccountBesideAClusterIdentity(t *testing.T) {
	t.Parallel()

	in := connectorsInput(t, s3Bucket(), func(spec *v1.CamundaClusterSpec) {
		spec.ServiceAccount = &v1.ServiceAccountSpec{}
	})

	connectors := renderedObjects(t, in, ComponentConnectors)
	assert.Equal(t, defaultServiceAccountName, previewedPodTemplate(t, connectors).Spec.ServiceAccountName)
	assert.Nil(t, renderedAccount(connectors))

	for _, process := range []string{ComponentZeebe, ComponentGateway} {
		template := previewedPodTemplate(t, renderedObjects(t, in, process))
		assert.Equal(t, "my-cluster-camunda", template.Spec.ServiceAccountName, process)
	}
}

func TestConnectorsRunUnderTheirOwnAccount(t *testing.T) {
	t.Parallel()

	in := connectorsInput(t, s3Bucket(), func(spec *v1.CamundaClusterSpec) {
		spec.ServiceAccount = &v1.ServiceAccountSpec{
			Annotations: map[string]string{"example.com/cluster": "only-the-cluster"},
		}
		spec.Connectors.ServiceAccount = &v1.ServiceAccountSpec{
			Annotations: map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/connectors"},
		}
	})

	connectors := renderedObjects(t, in, ComponentConnectors)
	assert.Equal(t, "my-cluster-connectors", previewedPodTemplate(t, connectors).Spec.ServiceAccountName)

	account := renderedAccount(connectors)
	require.NotNil(t, account)
	assert.Equal(t, "my-cluster-connectors", account.Name)
	assert.Equal(t, "my-cluster-ns", account.Namespace)
	assert.Equal(
		t,
		map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/connectors"},
		account.Annotations,
		"the bucket identity and the cluster annotations stay off the connectors account",
	)

	cluster := renderedAccount(renderedObjects(t, in, ComponentZeebe))
	require.NotNil(t, cluster)
	assert.Equal(t, "arn:aws:iam::123456789012:role/camunda", cluster.Annotations[v1.IRSARoleARNAnnotation])
}

// create: false names an account that the user owns: the pods use it, and
// the operator renders nothing that it would delete with the cluster.
func TestForeignConnectorsAccountIsReferencedNotRendered(t *testing.T) {
	t.Parallel()

	in := connectorsInput(t, nil, func(spec *v1.CamundaClusterSpec) {
		spec.Connectors.ServiceAccount = &v1.ServiceAccountSpec{Name: "platform-connectors", Create: new(false)}
	})

	connectors := renderedObjects(t, in, ComponentConnectors)
	assert.Equal(t, "platform-connectors", previewedPodTemplate(t, connectors).Spec.ServiceAccountName)
	assert.Nil(t, renderedAccount(connectors))
}

// An Azure bucket without a client ID still labels the pods that use it.
// That label belongs to the cluster identity, so the connectors never get it.
func TestConnectorsPodsCarryNoBucketPodLabels(t *testing.T) {
	t.Parallel()

	in := connectorsInput(t, azureBucket(""), nil)

	connectors := previewedPodTemplate(t, renderedObjects(t, in, ComponentConnectors))
	assert.NotContains(t, connectors.Labels, v1.AzureWorkloadIdentityUseLabel)

	zeebe := previewedPodTemplate(t, renderedObjects(t, in, ComponentZeebe))
	assert.Equal(t, "true", zeebe.Labels[v1.AzureWorkloadIdentityUseLabel])
}

func TestAzureBucketIdentityStaysOffTheConnectors(t *testing.T) {
	t.Parallel()

	in := connectorsInput(t, azureBucket("11111111-2222-3333-4444-555555555555"), func(spec *v1.CamundaClusterSpec) {
		spec.Connectors.ServiceAccount = &v1.ServiceAccountSpec{}
	})

	connectors := renderedObjects(t, in, ComponentConnectors)
	assert.NotContains(t, previewedPodTemplate(t, connectors).Labels, v1.AzureWorkloadIdentityUseLabel)
	require.NotNil(t, renderedAccount(connectors))
	assert.NotContains(t, renderedAccount(connectors).Annotations, v1.AzureClientIDAnnotation)

	zeebe := renderedObjects(t, in, ComponentZeebe)
	assert.Equal(t, "true", previewedPodTemplate(t, zeebe).Labels[v1.AzureWorkloadIdentityUseLabel])
	require.NotNil(t, renderedAccount(zeebe))
	assert.Equal(
		t,
		"11111111-2222-3333-4444-555555555555",
		renderedAccount(zeebe).Annotations[v1.AzureClientIDAnnotation],
	)
}

// An identity slot writes annotations only. The Azure client ID on the
// connectors account is enough for the webhook to bind the connectors pods.
func TestAzureClientIDOnTheConnectorsAccountLabelsTheConnectorsOnly(t *testing.T) {
	t.Parallel()

	in := connectorsInput(t, nil, func(spec *v1.CamundaClusterSpec) {
		spec.Connectors.ServiceAccount = &v1.ServiceAccountSpec{
			Annotations: map[string]string{v1.AzureClientIDAnnotation: "connectors-client"},
		}
	})

	connectors := previewedPodTemplate(t, renderedObjects(t, in, ComponentConnectors))
	assert.Equal(t, "true", connectors.Labels[v1.AzureWorkloadIdentityUseLabel])

	for _, process := range []string{ComponentZeebe, ComponentGateway} {
		template := previewedPodTemplate(t, renderedObjects(t, in, process))
		assert.NotContains(t, template.Labels, v1.AzureWorkloadIdentityUseLabel, process)
	}
}

func TestAzureClientIDOnTheClusterAccountLabelsTheOtherProcessesOnly(t *testing.T) {
	t.Parallel()

	in := connectorsInput(t, nil, func(spec *v1.CamundaClusterSpec) {
		spec.ServiceAccount = &v1.ServiceAccountSpec{
			Annotations: map[string]string{v1.AzureClientIDAnnotation: "cluster-client"},
		}
	})

	for _, process := range []string{ComponentZeebe, ComponentGateway} {
		template := previewedPodTemplate(t, renderedObjects(t, in, process))
		assert.Equal(t, "true", template.Labels[v1.AzureWorkloadIdentityUseLabel], process)
	}

	connectors := previewedPodTemplate(t, renderedObjects(t, in, ComponentConnectors))
	assert.NotContains(t, connectors.Labels, v1.AzureWorkloadIdentityUseLabel)
}

// The backup Jobs run under the cluster account, so they need the label of
// its Azure identity as well.
func TestClusterAccountPodLabels(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		account *v1.ServiceAccountSpec
		want    map[string]string
	}{
		{name: "no account", want: nil},
		{name: "an account without an Azure client ID", account: &v1.ServiceAccountSpec{
			Annotations: map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/camunda"},
		}, want: nil},
		{name: "an account with an Azure client ID", account: &v1.ServiceAccountSpec{
			Annotations: map[string]string{v1.AzureClientIDAnnotation: "cluster-client"},
		}, want: map[string]string{v1.AzureWorkloadIdentityUseLabel: "true"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cluster := &v1.CamundaCluster{Spec: v1.CamundaClusterSpec{ServiceAccount: tt.account}}

			assert.Equal(t, tt.want, ClusterAccountPodLabels(cluster))
		})
	}
}

// connectorsComponent returns the component of the connectors process.
func connectorsComponent(t *testing.T, in Input) ProcessComponent {
	t.Helper()

	comps, err := Build(in)
	require.NoError(t, err)
	for _, pc := range comps {
		if pc.Process.Component == ComponentConnectors {
			return pc
		}
	}
	require.Fail(t, "no connectors component")
	return ProcessComponent{}
}

// Disabled connectors skip the check that keeps the two accounts apart. A
// connectors block that names the cluster account must then not release the
// account that the cluster component owns.
func TestDisabledConnectorsReleaseOnlyAnAccountOfTheirOwn(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		account    string
		registered bool
	}{
		{name: "the name of the cluster account", account: "my-cluster-camunda", registered: false},
		{name: "a name of their own", account: "my-cluster-connectors", registered: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			in := connectorsInput(t, nil, func(spec *v1.CamundaClusterSpec) {
				spec.ServiceAccount = &v1.ServiceAccountSpec{}
				spec.Connectors.Enabled = new(false)
				spec.Connectors.ServiceAccount = &v1.ServiceAccountSpec{Name: tt.account, Create: new(false)}
			})

			_, ok := connectorsComponent(t, in).Component.Resource("v1/ServiceAccount/my-cluster-ns/" + tt.account)
			assert.Equal(t, tt.registered, ok)
		})
	}
}
