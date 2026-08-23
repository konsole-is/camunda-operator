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
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// ContractLabels returns the labels of the ManagementAuthConfig that a
// management cluster writes: the two owner labels and its own component value.
// The contract is cluster-scoped and its owner is namespaced, so a namespaced
// owner cannot hold it through an owner reference: the owner labels are what
// tells a reader, and the controller itself, which management cluster wrote
// it.
func ContractLabels(mc *v1.CamundaManagementCluster) map[string]string {
	return labels.Merge(
		map[string]string{labels.ManagementClusterNamespaceKey: mc.Namespace},
		labels.Managed(labels.ManagementCluster(mc.Name), ComponentManagementAuth),
	)
}

// ManagementAuthSpec derives the ManagementAuthConfig that Optimize reads: the
// endpoints of the identity provider, the base URL of Management Identity, and
// the Optimize client. Every field comes from in.Provider, so the derivation
// is the same in all three identity provider modes.
//
// The client secret reference keeps the namespace it was declared in. The
// contract is cluster-scoped, so a consumer in another namespace copies the
// Secret for itself; a rewritten reference would name a copy that only the
// management namespace can read.
func ManagementAuthSpec(in Input) v1.ManagementAuthConfigSpec {
	optimize := in.Provider.Clients.Optimize

	spec := v1.ManagementAuthConfigSpec{
		BaseURL:          in.Cluster.Spec.Identity.ExternalURL,
		IssuerURL:        in.Provider.IssuerURL,
		IssuerBackendURL: in.Provider.IssuerBackendURL,
		AuthURL:          in.Provider.AuthURL,
		TokenURL:         in.Provider.TokenURL,
		JwksURL:          in.Provider.JwksURL,
		ClientID:         optimize.ID,
		Audience:         optimize.Audience,
	}
	if optimize.SecretRef != nil {
		spec.ClientSecretRef = *optimize.SecretRef
	}

	return spec
}
