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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

func TestCheckServedPassesARegistrationThatServesItsSection(t *testing.T) {
	selections := []fleetv1alpha1.PluginSelection{{Section: "access", Registration: "ingress"}}
	registrations := []pluginsv1alpha1.PluginRegistration{registration("ingress", "access")}

	assert.NoError(t, CheckServed(selections, registrations))
}

func TestCheckServedPassesAMissingRegistration(t *testing.T) {
	selections := []fleetv1alpha1.PluginSelection{{Section: "access", Registration: "ingress"}}

	assert.NoError(t, CheckServed(selections, nil))
}

func TestCheckServedRefusesARegistrationOfAnotherSection(t *testing.T) {
	selections := []fleetv1alpha1.PluginSelection{
		{Section: "access", Registration: "ingress"},
		{Section: "secondary-storage", Registration: "existing-object-storage"},
	}
	registrations := []pluginsv1alpha1.PluginRegistration{
		registration("ingress", "access"),
		registration("existing-object-storage", "object-storage"),
	}

	failure := refusal(t, CheckServed(selections, registrations))

	assert.Equal(t, fleetv1alpha1.ReasonNotAllowed, failure.Reason)
	assert.Equal(
		t,
		`PluginRegistration "existing-object-storage" does not serve section "secondary-storage"`,
		failure.Message,
	)
}

func TestCheckNamesPassesDistinctNames(t *testing.T) {
	selections := []fleetv1alpha1.PluginSelection{
		{Section: "alerts", Registration: "mail"},
		{Section: "alerts", Registration: "chat"},
		{Section: "access", Registration: "ingress"},
	}

	assert.NoError(t, CheckNames("orders", selections, []pluginsv1alpha1.PluginSection{manySection("alerts")}))
}

func TestCheckNamesRefusesTwoOptionsOfAOneSection(t *testing.T) {
	selections := []fleetv1alpha1.PluginSelection{
		{Section: "secondary-storage", Registration: "elasticsearch"},
		{Section: "secondary-storage", Registration: "database-server"},
	}

	failure := refusal(t, CheckNames("orders", selections, nil))

	assert.Equal(t, fleetv1alpha1.ReasonNotAllowed, failure.Reason)
	assert.Equal(
		t,
		`section "secondary-storage" takes one option, and the claim selects "elasticsearch" and "database-server"`,
		failure.Message,
	)
}

func TestCheckNamesRefusesNamesThatMeetAcrossSections(t *testing.T) {
	selections := []fleetv1alpha1.PluginSelection{
		{Section: "alerts", Registration: "mail-pager"},
		{Section: "alerts-mail", Registration: "pager"},
	}
	sections := []pluginsv1alpha1.PluginSection{manySection("alerts"), manySection("alerts-mail")}

	failure := refusal(t, CheckNames("orders", selections, sections))

	assert.Equal(t, fleetv1alpha1.ReasonNotAllowed, failure.Reason)
	assert.Equal(
		t,
		`the selections alerts/mail-pager and alerts-mail/pager get the same request name "orders-alerts-mail-pager"`,
		failure.Message,
	)
}

func manySection(name string) pluginsv1alpha1.PluginSection {
	return pluginsv1alpha1.PluginSection{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec:       pluginsv1alpha1.PluginSectionSpec{Selection: pluginsv1alpha1.SelectionMany},
	}
}

func refusal(t *testing.T, err error) *conditions.PreCheckFailure {
	t.Helper()

	var failure *conditions.PreCheckFailure
	require.True(t, errors.As(err, &failure), "want a *conditions.PreCheckFailure, got %v", err)

	return failure
}
