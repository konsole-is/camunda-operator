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

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// RealmTarget records the realm of provider: where it is, and which Secrets
// the operator signs in to it with. It is what status.callbackRealm holds
// after the login callbacks of Optimize are registered there.
//
// The oidc mode administers no realm, so its provider records nil.
func RealmTarget(provider IdentityProvider) *v1.KeycloakRealmTarget {
	if provider.Mode == ModeOIDC {
		return nil
	}

	target := &v1.KeycloakRealmTarget{
		URL:               provider.KeycloakURL,
		Realm:             provider.Realm,
		CABundleSecretRef: provider.CABundle.DeepCopy(),
	}
	if provider.AdminCredentials != nil {
		target.AdminCredentialsSecretRef = *provider.AdminCredentials.DeepCopy()
	}

	return target
}

// RealmIdentity returns the name of the realm that target holds, as the URL of
// its realm endpoint: the URL without its trailing slashes, then /realms/ and
// the realm. Two targets of one identity are one realm. The administrator and
// the certificate authority take no part in it. The result is deterministic,
// so a name derived from it (a hash, for example) identifies the realm across
// resources.
func RealmIdentity(target v1.KeycloakRealmTarget) string {
	return strings.TrimRight(target.URL, "/") + keycloakRealmPath + target.Realm
}

// SameRealm reports whether a and b name one realm.
func SameRealm(a, b v1.KeycloakRealmTarget) bool {
	return RealmIdentity(a) == RealmIdentity(b)
}

// RealmProvider builds the identity provider of a recorded realm target: the
// URL, the realm, the administrator, and the Optimize client, which is what
// the operator needs to administer the login callbacks there.
//
// It carries no mode and no issuer. A realm that the spec no longer names
// renders nothing, so only the calls against Keycloak read this provider.
func RealmProvider(target v1.KeycloakRealmTarget) IdentityProvider {
	return IdentityProvider{
		KeycloakURL:      strings.TrimRight(target.URL, "/"),
		Realm:            target.Realm,
		AdminCredentials: target.AdminCredentialsSecretRef.DeepCopy(),
		CABundle:         target.CABundleSecretRef.DeepCopy(),
		Clients:          ProviderClients{Optimize: Client{ID: keycloakClientOptimize}},
	}
}
