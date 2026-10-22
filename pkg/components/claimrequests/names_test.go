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
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/validation"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

func TestName(t *testing.T) {
	s := fleetv1alpha1.PluginSelection{Section: "alerts", Registration: "mail"}

	assert.Equal(t, "orders-alerts", Name("orders", s, pluginsv1alpha1.SelectionOne))
	assert.Equal(t, "orders-alerts", Name("orders", s, ""))
	assert.Equal(t, "orders-alerts-mail", Name("orders", s, pluginsv1alpha1.SelectionMany))
}

func TestNameCutsALongNameAndKeepsItApart(t *testing.T) {
	claim := strings.Repeat("c", 40)
	s := fleetv1alpha1.PluginSelection{Section: "alerts", Registration: strings.Repeat("r", 30)}
	other := fleetv1alpha1.PluginSelection{Section: "alerts", Registration: strings.Repeat("r", 29) + "x"}
	full := claim + "-alerts-" + s.Registration
	sum := sha256.Sum256([]byte(full))

	name := Name(claim, s, pluginsv1alpha1.SelectionMany)

	assert.Equal(t, full[:54]+"-"+hex.EncodeToString(sum[:])[:8], name)
	assert.Len(t, name, 63)
	assert.NotEqual(t, name, Name(claim, other, pluginsv1alpha1.SelectionMany))
}

func TestNameKeepsANameOfSixtyThreeCharacters(t *testing.T) {
	claim := strings.Repeat("c", 63-len("-alerts"))

	assert.Equal(t, claim+"-alerts", Name(claim, fleetv1alpha1.PluginSelection{Section: "alerts"}, ""))
}

func TestNameCutsBeforeADotAtTheBoundary(t *testing.T) {
	claim := strings.Repeat("c", 40)
	s := fleetv1alpha1.PluginSelection{Section: strings.Repeat("s", 12) + "." + strings.Repeat("s", 20)}
	full := claim + "-" + s.Section
	sum := sha256.Sum256([]byte(full))

	name := Name(claim, s, pluginsv1alpha1.SelectionOne)

	assert.Equal(t, full[:53]+"-"+hex.EncodeToString(sum[:])[:8], name)
	assert.Empty(t, validation.IsDNS1123Subdomain(name))
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
