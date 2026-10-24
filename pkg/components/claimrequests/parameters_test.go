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
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

func TestCheckParameterObjectsRefusesParametersThatAreNotAnObject(t *testing.T) {
	selections := []fleetv1alpha1.PluginSelection{
		{Section: "access", Registration: "test-access", Parameters: &apiextensionsv1.JSON{Raw: []byte(`{"a":1}`)}},
		{Section: "management-database", Registration: "db", Parameters: &apiextensionsv1.JSON{Raw: []byte(`[1]`)}},
	}

	err := CheckParameterObjects(selections)

	var failure *conditions.PreCheckFailure
	require.True(t, errors.As(err, &failure))
	assert.Equal(t, fleetv1alpha1.ReasonNotAllowed, failure.Reason)
	assert.Contains(t, failure.Message, `"management-database"`)
}

func TestCheckParameterObjectsPassesObjectsAndAbsentParameters(t *testing.T) {
	selections := []fleetv1alpha1.PluginSelection{
		{Section: "access", Registration: "test-access"},
		{Section: "management-database", Registration: "db", Parameters: &apiextensionsv1.JSON{Raw: []byte(`{}`)}},
	}

	assert.NoError(t, CheckParameterObjects(selections))
}
