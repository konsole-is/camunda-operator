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

package camundaoptimize

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/util/validation"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// A short name is joined as written, so the names a user reads in the common
// case are the obvious ones.
func TestDerivedNamesOfAShortResource(t *testing.T) {
	t.Parallel()

	o := &v1.CamundaOptimize{}
	o.Name = "my-optimize"

	assert.Equal(t, "my-optimize-webapp", WorkloadName(o, ComponentWebapp))
	assert.Equal(t, "my-optimize-importer", WorkloadName(o, ComponentImporter))
	assert.Equal(t, "my-optimize-optimize-license", MirroredSecretName(o, MirrorPurposeLicense))
}

// TestLongResourceNamesRenderValidObjects pins the bounds. A CamundaOptimize
// name is a DNS subdomain of up to 253 characters, but the Service of each
// component is a DNS label of 63, and so is every label value. Every derived
// name truncates to fit, and two names that agree on the truncated head stay
// apart.
func TestLongResourceNamesRenderValidObjects(t *testing.T) {
	t.Parallel()

	for _, length := range []int{63, 100, 253} {
		in := newInput(t, func(in *Input) {
			in.Optimize.Name = strings.Repeat("o", length)
			in.ClusterName = strings.Repeat("c", length)
		})

		comps, err := Build(in)
		require.NoError(t, err)
		for _, comp := range comps {
			for _, obj := range previewObjects(t, comp) {
				assert.Empty(
					t, validation.IsDNS1123Label(obj.GetName()),
					"%d: %s", length, obj.GetName(),
				)
				for key, value := range obj.GetLabels() {
					assert.Empty(
						t, validation.IsValidLabelValue(value),
						"%d: %s=%s", length, key, value,
					)
				}
			}
		}

		for _, purpose := range MirrorPurposes {
			name := MirroredSecretName(in.Optimize, purpose)
			assert.Empty(t, validation.IsDNS1123Label(name), "%d: %s", length, name)
		}
	}
}

// Two CamundaOptimize resources whose names share the truncated head must not
// derive one name between them, or the second would adopt the workloads of
// the first.
func TestLongResourceNamesStayApart(t *testing.T) {
	t.Parallel()

	head := strings.Repeat("o", 80)
	first, second := &v1.CamundaOptimize{}, &v1.CamundaOptimize{}
	first.Name, second.Name = head+"-one", head+"-two"

	assert.NotEqual(t, WorkloadName(first, ComponentWebapp), WorkloadName(second, ComponentWebapp))
	assert.NotEqual(
		t,
		MirroredSecretName(first, MirrorPurposeLicense),
		MirroredSecretName(second, MirrorPurposeLicense),
	)

	assert.Equal(
		t, WorkloadName(first, ComponentWebapp), WorkloadName(first, ComponentWebapp),
		"the name is deterministic",
	)
	assert.NotEqual(
		t, WorkloadName(first, ComponentWebapp), WorkloadName(first, ComponentImporter),
		"the two components of one resource keep separate names",
	)
}
