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

package v1_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/runtime"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestZeroBuilderRefusesToAddToScheme(t *testing.T) {
	s := runtime.NewScheme()

	err := (&v1.Builder{}).Register(&v1.CamundaCluster{}).AddToScheme(s)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "group version")
	assert.False(t, s.Recognizes(v1.GroupVersion.WithKind("CamundaCluster")))
}
