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

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// clusterProcesses are the processes that run under the cluster account in
// the default topology with connectors enabled.
var clusterProcesses = []string{ComponentZeebe, ComponentGateway}

// accountHashes returns the ServiceAccount hash on the pod template of each
// process, keyed by process. A missing annotation maps to "".
func accountHashes(t *testing.T, in Input) map[string]string {
	t.Helper()

	hashes := map[string]string{}
	for _, process := range append([]string{ComponentConnectors}, clusterProcesses...) {
		template := previewedPodTemplate(t, renderedObjects(t, in, process))
		hashes[process] = template.Annotations[ServiceAccountHashAnnotation]
	}
	return hashes
}

// annotatedAccounts returns a cluster with connectors in which both accounts
// carry the given annotations.
func annotatedAccounts(t *testing.T, cluster, connectors map[string]string) Input {
	t.Helper()

	return connectorsInput(t, nil, func(spec *v1.CamundaClusterSpec) {
		spec.ServiceAccount = &v1.ServiceAccountSpec{Annotations: cluster}
		spec.Connectors.ServiceAccount = &v1.ServiceAccountSpec{Annotations: connectors}
	})
}

// Workload identity reads the annotations of the account when a pod starts,
// so a pod that started before a change keeps the old identity.
func TestClusterAccountAnnotationChangeRollsTheClusterPods(t *testing.T) {
	t.Parallel()

	connectors := map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/connectors"}
	old := map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/old"}
	updated := map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/new"}
	before := accountHashes(t, annotatedAccounts(t, old, connectors))
	after := accountHashes(t, annotatedAccounts(t, updated, connectors))

	for _, process := range clusterProcesses {
		require.NotEmpty(t, before[process], process)
		assert.NotEqual(t, before[process], after[process], process)
	}
	assert.Equal(t, before[ComponentConnectors], after[ComponentConnectors])
}

// A bucket that changes the identity it derives changes the cluster account
// as much as a spec edit does.
func TestDerivedAnnotationChangeRollsTheClusterPods(t *testing.T) {
	t.Parallel()

	in := connectorsInput(t, s3Bucket(), nil)
	before := accountHashes(t, in)

	in.ServiceAccountAnnotations = map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/moved"}
	after := accountHashes(t, in)

	for _, process := range clusterProcesses {
		require.NotEmpty(t, before[process], process)
		assert.NotEqual(t, before[process], after[process], process)
	}
}

func TestConnectorsAccountAnnotationChangeRollsTheConnectorsOnly(t *testing.T) {
	t.Parallel()

	cluster := map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/camunda"}
	old := map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/old"}
	updated := map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/new"}
	before := accountHashes(t, annotatedAccounts(t, cluster, old))
	after := accountHashes(t, annotatedAccounts(t, cluster, updated))

	require.NotEmpty(t, before[ComponentConnectors])
	assert.NotEqual(t, before[ComponentConnectors], after[ComponentConnectors])
	for _, process := range clusterProcesses {
		assert.Equal(t, before[process], after[process], process)
	}
}

// Go randomizes the iteration order of a map, so many keys make an order
// dependence show up across the builds.
func TestServiceAccountHashIsStable(t *testing.T) {
	t.Parallel()

	annotations := map[string]string{}
	for _, key := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i", "j"} {
		annotations["example.com/"+key] = key
	}
	in := annotatedAccounts(t, annotations, annotations)

	first := accountHashes(t, in)
	for range 20 {
		assert.Equal(t, first, accountHashes(t, in))
	}
}

// The operator rolls only for annotations that it renders itself. Without a
// rendered annotation the template carries no hash, so an upgrade of the
// operator does not roll these clusters.
func TestNoServiceAccountHashWithoutRenderedAnnotations(t *testing.T) {
	t.Parallel()

	foreign := map[string]string{v1.IRSARoleARNAnnotation: "arn:aws:iam::123456789012:role/foreign"}
	tests := []struct {
		name   string
		mutate func(*v1.CamundaClusterSpec)
	}{
		{name: "no named account", mutate: nil},
		{name: "accounts without annotations", mutate: func(spec *v1.CamundaClusterSpec) {
			spec.ServiceAccount = &v1.ServiceAccountSpec{}
			spec.Connectors.ServiceAccount = &v1.ServiceAccountSpec{}
		}},
		{name: "accounts with create false", mutate: func(spec *v1.CamundaClusterSpec) {
			spec.ServiceAccount = &v1.ServiceAccountSpec{
				Name: "platform", Create: new(false), Annotations: foreign,
			}
			spec.Connectors.ServiceAccount = &v1.ServiceAccountSpec{
				Name: "platform-connectors", Create: new(false), Annotations: foreign,
			}
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			for process, hash := range accountHashes(t, connectorsInput(t, nil, tt.mutate)) {
				assert.Empty(t, hash, process)
			}
		})
	}
}

// A user pod annotation must not pin the hash, or an annotation change would
// stop rolling the pods.
func TestServiceAccountHashWinsOverAUserPodAnnotation(t *testing.T) {
	t.Parallel()

	in := annotatedAccounts(t, map[string]string{"example.com/team": "platform"}, nil)
	in.Cluster.Spec.PodAnnotations = map[string]string{ServiceAccountHashAnnotation: "pinned"}
	in.Effective = NewEffective(MergeSpec(in.Cluster.Spec, nil, nil))

	hashes := accountHashes(t, in)
	for _, process := range clusterProcesses {
		assert.NotEqual(t, "pinned", hashes[process], process)
		assert.NotEmpty(t, hashes[process], process)
	}
}
