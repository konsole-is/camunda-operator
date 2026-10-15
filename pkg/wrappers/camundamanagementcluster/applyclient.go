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
	"sigs.k8s.io/controller-runtime/pkg/client"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/internal/applyprune"
)

// NewApplyClient wraps c so that a server-side apply of a CamundaManagementCluster
// sends only the fields that hold a value: no status, no empty string, and
// no struct without a set field. A set pointer to an empty struct, such as
// identityProvider.oidc, stays in the apply. A field that the component did not
// set then never joins its field manager, and another writer can own it. A
// controller that reconciles the Resource through an ocf component places
// this client in the Client of the ReconcileContext.
func NewApplyClient(c client.Client) client.Client {
	return applyprune.NewClient[*apiv1.CamundaManagementCluster](c)
}
