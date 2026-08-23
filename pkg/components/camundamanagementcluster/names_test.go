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

package camundamanagementcluster

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/util/validation"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// Every resource of a management cluster takes its name from the resource and
// the component it belongs to.
func TestResourceNames(t *testing.T) {
	t.Parallel()

	mc := newCluster(nil)

	assert.Equal(t, "my-management-identity", IdentityName(mc))
	assert.Equal(t, "my-management-keycloak", KeycloakName(mc))
	assert.Equal(t, "my-management-console", ConsoleName(mc))
	assert.Equal(t, "my-management-web-modeler-restapi", WebModelerRestapiName(mc))
	assert.Equal(t, "my-management-web-modeler-websockets", WebModelerWebsocketsName(mc))
	assert.Equal(t, "my-management-identity-client", IdentityClientSecretName(mc))
	assert.Equal(t, "my-management-optimize-client", OptimizeClientSecretName(mc))
	assert.Equal(t, "my-management-identity-admin", IdentityAdminSecretName(mc))
	assert.Equal(t, "my-management-web-modeler-pusher", PusherSecretName(mc))
	assert.Equal(
		t,
		"my-management-web-modeler-cluster-1a2b3c4d",
		WebModelerClusterUserSecretName(mc, "1a2b3c4d-5e6f-7081-9203-a4b5c6d7e8f9"),
	)
}

// A name that a Service cannot carry is cut and keeps its identity in a hash,
// so two long names still render two Services.
func TestALongNameFitsADNSLabel(t *testing.T) {
	t.Parallel()

	long := newCluster(func(mc *v1.CamundaManagementCluster) {
		mc.Name = strings.Repeat("a", 200)
	})
	other := newCluster(func(mc *v1.CamundaManagementCluster) {
		mc.Name = strings.Repeat("a", 199) + "b"
	})

	assert.LessOrEqual(t, len(IdentityName(long)), validation.DNS1123LabelMaxLength)
	assert.LessOrEqual(t, len(MirroredSecretName(long, MirrorPurposeLicense)), validation.DNS1123LabelMaxLength)
	assert.NotEqual(t, IdentityName(long), IdentityName(other))
}

// A Secret of the management namespace is reachable where it is; one from
// another namespace is reachable through its copy.
func TestLocalSecretName(t *testing.T) {
	t.Parallel()

	mc := newCluster(nil)

	assert.Equal(
		t,
		"oidc-credentials",
		LocalSecretName(mc, fixtureNamespace, "oidc-credentials", MirrorPurposeIdentityClient),
	)
	assert.Equal(
		t,
		"my-management-management-identity-client",
		LocalSecretName(mc, "platform", "oidc-credentials", MirrorPurposeIdentityClient),
	)
}
