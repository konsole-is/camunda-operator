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

package claimrequests

import (
	"context"
	"encoding/json"
	"maps"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/konsole-is/camunda-operator/pkg/wrappers/pluginrequest"
)

// The apply of a request sends only the fields that the claim and the
// outputs set, so that another writer can own the rest.
func TestRequestApplySendsOnlyTheSetFields(t *testing.T) {
	var sent []byte
	recorder := fake.NewClientBuilder().WithInterceptorFuncs(interceptor.Funcs{
		Patch: func(
			_ context.Context, _ client.WithWatch, obj client.Object,
			patch client.Patch, _ ...client.PatchOption,
		) error {
			data, err := patch.Data(obj)
			sent = data

			return err
		},
	}).Build()
	volume := previewRequests(t, volumeInput(t))[1]

	//nolint:staticcheck // ocf applies through the deprecated client.Apply patch
	require.NoError(t, pluginrequest.NewApplyClient(recorder).Patch(t.Context(), volume, client.Apply))

	var applied struct {
		Spec map[string]json.RawMessage `json:"spec"`
	}
	require.NoError(t, json.Unmarshal(sent, &applied))
	keys := slices.Sorted(maps.Keys(applied.Spec))
	assert.Equal(t, []string{"claimRef", "deletionPolicy", "inputs", "registration", "section", "target"}, keys)

	var target map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(applied.Spec["target"], &target))
	assert.NotContains(t, target, "additionalServiceAccounts")
}
