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

package camundaclusterclaim

import (
	"testing"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

func TestOwnRequests(t *testing.T) {
	request := func(name string, kind pluginsv1alpha1.ClaimKind, claim string) pluginsv1alpha1.PluginRequest {
		return pluginsv1alpha1.PluginRequest{
			ObjectMeta: metav1.ObjectMeta{Name: name},
			Spec:       pluginsv1alpha1.PluginRequestSpec{ClaimRef: pluginsv1alpha1.ClaimRef{Kind: kind, Name: claim}},
		}
	}
	own := request("orders-access", pluginsv1alpha1.ClaimKindCamundaClusterClaim, "orders")
	plane := request("orders-management-database", pluginsv1alpha1.ClaimKindCamundaManagementClusterClaim, "orders")
	other := request("billing-access", pluginsv1alpha1.ClaimKindCamundaClusterClaim, "billing")

	assert.Equal(t, []pluginsv1alpha1.PluginRequest{own}, ownRequests([]pluginsv1alpha1.PluginRequest{plane, own, other}, "orders"))
	assert.Empty(t, ownRequests(nil, "orders"))
}
