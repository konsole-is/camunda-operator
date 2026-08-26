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
	"slices"
	"strings"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// OptimizeCallbackPath is the path that Optimize completes an OIDC login at.
// Management Identity holds it in the redirect-uris of its optimize preset and
// appends it to every root URL of that preset
// (management-api/src/main/resources/application.yaml, the
// keycloak.init.optimize block, and
// management-api/src/main/java/io/camunda/identity/impl/keycloak/initializer/service/ClientInitializationService.java,
// generateRedirectUrls).
const OptimizeCallbackPath = "/api/authentication/callback"

// AttachedOptimizes returns one status row per CamundaOptimize of discovered
// that names a URL, ordered by namespace and name. A CamundaOptimize with no
// spec.externalUrl has no callback to register, so it gets no row.
//
// The caller passes the CamundaOptimizes that name the ManagementAuthConfig of
// this management cluster.
func AttachedOptimizes(discovered []v1.CamundaOptimize) []v1.AttachedOptimizeStatus {
	rows := make([]v1.AttachedOptimizeStatus, 0, len(discovered))
	for _, optimize := range discovered {
		if optimize.Spec.ExternalURL == "" {
			continue
		}
		rows = append(rows, v1.AttachedOptimizeStatus{
			Name:        optimize.Name,
			Namespace:   optimize.Namespace,
			ExternalURL: optimize.Spec.ExternalURL,
		})
	}

	slices.SortFunc(rows, func(a, b v1.AttachedOptimizeStatus) int {
		if a.Namespace != b.Namespace {
			return strings.Compare(a.Namespace, b.Namespace)
		}
		return strings.Compare(a.Name, b.Name)
	})

	return rows
}

// OptimizeURLs returns the URL of every Optimize that this management plane
// serves: spec.optimize first, then the discovered rows, without duplicates.
// The result is what Input.OptimizeURLs carries.
//
// spec.optimize comes first because it names an Optimize outside this
// operator, which no row can report.
func OptimizeURLs(
	mc *v1.CamundaManagementCluster,
	rows []v1.AttachedOptimizeStatus,
) []string {
	var urls []string
	if optimize := mc.Spec.Optimize; optimize != nil {
		urls = append(urls, optimize.ExternalURL)
	}
	for _, row := range rows {
		if !slices.Contains(urls, row.ExternalURL) {
			urls = append(urls, row.ExternalURL)
		}
	}

	return urls
}

// OptimizeCallbacks returns the redirect URI that each URL of in.OptimizeURLs
// completes its login at, in the same order. It is what the operator writes on
// the Optimize client of the realm, and what a freshly started Management
// Identity writes there from KEYCLOAK_INIT_OPTIMIZE_ROOT_URL.
func OptimizeCallbacks(in Input) []string {
	callbacks := make([]string, 0, len(in.OptimizeURLs))
	for _, url := range in.OptimizeURLs {
		// Plain concatenation, because Management Identity concatenates too
		// and never trims the root URL (ClientInitializationService.java,
		// generateRedirectUrls). Both writers therefore produce the same
		// entry. The API server refuses a URL that ends with a slash, so
		// neither of them can produce a doubled one.
		callbacks = append(callbacks, url+OptimizeCallbackPath)
	}

	return callbacks
}
