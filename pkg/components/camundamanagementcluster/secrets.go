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

import "github.com/sourcehawk/operator-component-framework/pkg/component"

// secretsComponents renders the Secrets that the operator generates. The oidc
// mode generates none: the platform config names every client secret, and the
// first administrator is a token claim rather than a user with a password.
func secretsComponents(_ Input) ([]*component.Component, error) {
	// Sub-issue #188 renders the generated Secrets of the Keycloak modes
	// here.
	return nil, nil
}
