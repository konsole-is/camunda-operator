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

package claimpolicy

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

func TestResolveFixtures(t *testing.T) {
	t.Parallel()

	runFixtures(t, "resolve", func(in ResolveInput) (any, error) { return Resolve(in) })
}

func TestResolveGivesTheSameRequestsFromTheRecordedSnapshot(t *testing.T) {
	t.Parallel()

	in := ResolveInput{
		Registrations: []pluginsv1alpha1.PluginRegistration{{
			ObjectMeta: metav1.ObjectMeta{Name: "s3"},
			Spec: pluginsv1alpha1.PluginRegistrationSpec{
				Sections:        []string{"backup-storage"},
				Mode:            pluginsv1alpha1.ModeDefault,
				ParameterValues: &apiextensionsv1.JSON{Raw: []byte(`{"region":null,"tier":"standard"}`)},
			},
		}},
		Selections: []fleetv1alpha1.PluginSelection{{Section: "backup-storage", Registration: "s3"}},
	}
	first, err := Resolve(in)
	require.NoError(t, err)

	// The claim controller writes the snapshot to status.resolvedPolicy and
	// reads it back on the next reconcile.
	raw, err := json.Marshal(first.Policy)
	require.NoError(t, err)
	in.Recorded = &fleetv1alpha1.ResolvedPolicy{}
	require.NoError(t, json.Unmarshal(raw, in.Recorded))

	second, err := Resolve(in)
	require.NoError(t, err)
	assert.Equal(t, first.Requests, second.Requests)
}
