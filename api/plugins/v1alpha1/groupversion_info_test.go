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

package v1alpha1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// probe stands in for a kind of the group.
type probe struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

func (p *probe) DeepCopyObject() runtime.Object {
	out := *p
	p.DeepCopyInto(&out.ObjectMeta)

	return &out
}

func TestAddToSchemeRegistersUnderPluginsGroup(t *testing.T) {
	v1alpha1.SchemeBuilder.Register(&probe{})

	s := runtime.NewScheme()
	require.NoError(t, v1alpha1.AddToScheme(s))

	kinds, _, err := s.ObjectKinds(&probe{})
	require.NoError(t, err)
	assert.Equal(
		t,
		[]schema.GroupVersionKind{{Group: "plugins.camunda.io", Version: "v1alpha1", Kind: "probe"}},
		kinds,
	)
}
