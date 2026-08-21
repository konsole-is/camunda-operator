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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// A short name is joined as written, so the names a user reads in the common
// case are the obvious ones.
func TestDerivedNamesOfAShortCluster(t *testing.T) {
	t.Parallel()

	cluster := &v1.CamundaCluster{}
	cluster.Name = "my-cluster"

	assert.Equal(t, "my-cluster-zeebe", WorkloadName(cluster, ComponentZeebe))
	assert.Equal(t, "my-cluster-camunda-admin", AdminSecretName(cluster))
	assert.Equal(t, "my-cluster-camunda", ServiceAccountName(cluster, NewEffective(v1.CamundaClusterSpec{})))
	assert.Equal(t, "my-cluster-camunda-license", MirroredSecretName(cluster, MirrorPurposeLicense))
}

// TestLongClusterNamesRenderValidObjects pins the bounds. A CamundaCluster
// name is a DNS subdomain of up to 253 characters, but the Service of each
// process is a DNS label of 63, and so is every label value. Every derived
// name truncates to fit.
func TestLongClusterNamesRenderValidObjects(t *testing.T) {
	t.Parallel()

	for _, length := range []int{63, 100, 253} {
		in := fixtureDefault(t)
		in.Cluster.Name = strings.Repeat("c", length)

		comps, err := Build(in)
		require.NoError(t, err)
		for _, pc := range comps {
			for _, obj := range previewObjects(t, pc.Component) {
				// The ServiceAccount is the one rendered object whose name is
				// a DNS subdomain rather than a label, so it keeps every
				// character that fits.
				if _, isAccount := obj.(*corev1.ServiceAccount); isAccount {
					assert.Empty(
						t, validation.IsDNS1123Subdomain(obj.GetName()),
						"%d: %s", length, obj.GetName(),
					)
				} else {
					assert.Empty(
						t, validation.IsDNS1123Label(obj.GetName()),
						"%d: %s", length, obj.GetName(),
					)
				}

				for key, value := range obj.GetLabels() {
					assert.Empty(
						t, validation.IsValidLabelValue(value),
						"%d: %s=%s", length, key, value,
					)
				}
			}
		}

		for _, component := range []string{
			ComponentZeebe, ComponentGateway, ComponentOperate,
			ComponentTasklist, ComponentAdmin, ComponentConnectors,
		} {
			name := WorkloadName(in.Cluster, component)
			assert.Empty(t, validation.IsDNS1123Label(name), "%d: %s", length, name)
		}

		for _, purpose := range MirrorPurposes {
			name := MirroredSecretName(in.Cluster, purpose)
			assert.Empty(t, validation.IsDNS1123Label(name), "%d: %s", length, name)
		}

		assert.Empty(t, validation.IsDNS1123Label(AdminSecretName(in.Cluster)), length)

		// A ServiceAccount name is a DNS subdomain, not a label: it is the
		// principal that a workload identity binds, so it keeps every
		// character that fits.
		account := ServiceAccountName(in.Cluster, in.Effective)
		assert.Empty(t, validation.IsDNS1123Subdomain(account), "%d: %s", length, account)
	}
}

// Two clusters whose names share the truncated head must not derive one name
// between them, or the second would adopt the workloads of the first.
func TestLongClusterNamesStayApart(t *testing.T) {
	t.Parallel()

	head := strings.Repeat("c", 80)
	first, second := &v1.CamundaCluster{}, &v1.CamundaCluster{}
	first.Name, second.Name = head+"-one", head+"-two"

	assert.NotEqual(t, WorkloadName(first, ComponentZeebe), WorkloadName(second, ComponentZeebe))
	assert.NotEqual(t, AdminSecretName(first), AdminSecretName(second))
	assert.NotEqual(
		t,
		MirroredSecretName(first, MirrorPurposeLicense),
		MirroredSecretName(second, MirrorPurposeLicense),
	)

	assert.Equal(
		t, WorkloadName(first, ComponentZeebe), WorkloadName(first, ComponentZeebe),
		"the name is deterministic",
	)
	assert.NotEqual(
		t, WorkloadName(first, ComponentZeebe), WorkloadName(first, ComponentGateway),
		"the processes of one cluster keep separate names",
	)
}

// A user-set ServiceAccount name is the user's own, so nothing truncates it.
func TestServiceAccountNameFromTheSpecIsUntouched(t *testing.T) {
	t.Parallel()

	cluster := &v1.CamundaCluster{}
	cluster.Name = strings.Repeat("c", 253)
	e := NewEffective(v1.CamundaClusterSpec{
		ServiceAccount: &v1.ServiceAccountSpec{Name: "my-account"},
	})

	assert.Equal(t, "my-account", ServiceAccountName(cluster, e))
}
