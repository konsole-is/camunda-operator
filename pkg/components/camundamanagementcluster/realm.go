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
	"net"
	"net/url"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// RealmTarget records the realm of provider: where it is, and which Secrets
// the operator signs in to it with. In the externalKeycloak mode it is what
// status.callbackRealm holds after the login callbacks of Optimize are
// registered there. The keycloak mode returns a target too, for comparisons
// against a record, but it is never persisted: the operator deletes that
// Keycloak together with the mode, so there would be nothing to withdraw
// from.
//
// The oidc mode administers no realm, and a provider that names no
// administrator gives the operator nothing to sign in with, so both record
// nil.
func RealmTarget(provider IdentityProvider) *v1.KeycloakRealmTarget {
	if provider.Mode == ModeOIDC || provider.AdminCredentials == nil {
		return nil
	}

	return &v1.KeycloakRealmTarget{
		URL:                       provider.KeycloakURL,
		Realm:                     provider.Realm,
		AdminCredentialsSecretRef: *provider.AdminCredentials.DeepCopy(),
		CABundleSecretRef:         provider.CABundle.DeepCopy(),
	}
}

// RealmIdentity returns the name of the realm that target holds, as the URL
// of its realm endpoint: the URL, then /realms/ and the realm. The scheme and
// the host of the URL are folded to lower case, and a default port (443 on
// https, 80 on http), a user in the URL, and every trailing slash are
// dropped. The path of the URL and the realm stay as they are, because both
// are case-sensitive. Two targets of one identity are one realm. The
// administrator and the certificate authority take no part in it. The result
// is deterministic, so a name derived from it (a hash, for example)
// identifies the realm across resources, and it is safe to read, so a claim
// can name the realm it holds in the clear.
func RealmIdentity(target v1.KeycloakRealmTarget) string {
	return normalizeKeycloakURL(target.URL) + keycloakRealmPath + target.Realm
}

// normalizeKeycloakURL trims the trailing slashes of raw and folds what
// names the same server either way: the case of the scheme and of the host,
// a port that only spells out the default of the scheme, and a user in the
// URL. A raw value that does not parse as a URL comes back with only the
// slashes trimmed.
func normalizeKeycloakURL(raw string) string {
	trimmed := strings.TrimRight(raw, "/")
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return trimmed
	}

	// A user in the URL reaches the same realm as the URL without it, and it
	// carries a password. Keeping it would let two spellings of one realm
	// take a claim each, and would write the password into the annotations of
	// the claim.
	parsed.User = nil
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Hostname())
	port := parsed.Port()
	if (parsed.Scheme == "https" && port == "443") || (parsed.Scheme == "http" && port == "80") {
		port = ""
	}
	switch {
	case port != "":
		parsed.Host = net.JoinHostPort(host, port)
	case strings.Contains(host, ":"):
		// An IPv6 host keeps the brackets that Hostname stripped.
		parsed.Host = "[" + host + "]"
	default:
		parsed.Host = host
	}

	return parsed.String()
}

// NormalizeRealmIdentity folds a hand-written realm identity the way
// RealmIdentity folds a URL: the case of the scheme and of the host, a
// spelled-out default port, a user in the URL, and trailing slashes make no
// difference. The path, and with it the realm at its end, stays as it is. It
// lets an annotation written from status.callbackRealm match the recorded
// identity, whose URL was folded the same way.
func NormalizeRealmIdentity(value string) string {
	return normalizeKeycloakURL(value)
}

// RealmURL is the URL of target as RealmIdentity reads it: a user in the URL,
// a default port, the case of the scheme and of the host, and every trailing
// slash are folded out. A message that names the Keycloak of a realm uses it,
// so a password in the URL never reaches a condition or an event.
func RealmURL(target v1.KeycloakRealmTarget) string {
	return normalizeKeycloakURL(target.URL)
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

// IdentityWritesRealm reports whether one of pods is a Management Identity
// pod that points at the realm of target and is not ready. Such a pod is
// starting, and Management Identity writes the whole Optimize client of its
// realm while it starts, so a withdrawal from that realm would be put back
// by it. A ready pod started long ago and writes nothing more, and a pod
// that points at another realm writes that one. A pod that is going away
// counts: its container can still be inside the start until the kubelet
// stops it.
func IdentityWritesRealm(pods []corev1.Pod, target v1.KeycloakRealmTarget) bool {
	for i := range pods {
		pod := &pods[i]
		if podDone(pod) || podReady(pod) {
			continue
		}
		if IdentityTemplatePointsAtRealm(&pod.Spec, target) {
			return true
		}
	}

	return false
}

// IdentityPointsAtRealm reports whether one of pods is a Management Identity
// pod that points at the realm of target and can still run, ready or not.
// Such a pod can restart and write the Optimize client of that realm from
// its environment, so the record of that realm must outlive the pod: it is
// what finds the realm again and empties it.
func IdentityPointsAtRealm(pods []corev1.Pod, target v1.KeycloakRealmTarget) bool {
	for i := range pods {
		pod := &pods[i]
		if podDone(pod) {
			continue
		}
		if IdentityTemplatePointsAtRealm(&pod.Spec, target) {
			return true
		}
	}

	return false
}

// IdentityTemplatePointsAtRealm reports whether the Management Identity
// container of spec points at the realm of target. It reads a pod, or the
// pod template of a Deployment or a ReplicaSet: a workload whose template
// points at the realm can start a pod against it at any moment, even while
// it has none, so it counts as a writer of that realm the same as a pod.
//
// A container that takes the Keycloak URL or the realm from a reference
// answers yes for every target. spec.identity.extraEnv can replace either
// variable, and the value behind a reference is not in the workload, so the
// realm such a container writes is unknown and the record of any realm must
// outlive it.
func IdentityTemplatePointsAtRealm(spec *corev1.PodSpec, target v1.KeycloakRealmTarget) bool {
	realm, state := identityRealmEnv(spec)
	switch state {
	case realmUnknown:
		return true
	case realmNamed:
		return SameRealm(realm, target)
	default:
		return false
	}
}

// IdentityRealms returns the realm of every Management Identity pod of pods
// whose containers can still run, and reports whether one of them writes a
// realm that the pod does not name. A ready pod counts because a restart runs
// its start against that realm again, and a terminating pod counts until it
// is gone, because one inside its initializer still writes. Pods of the oidc
// mode name no Keycloak and contribute nothing.
func IdentityRealms(pods []corev1.Pod) ([]v1.KeycloakRealmTarget, bool) {
	var realms []v1.KeycloakRealmTarget
	var unknown bool
	for i := range pods {
		pod := &pods[i]
		if podDone(pod) {
			continue
		}
		switch realm, state := identityRealmEnv(&pod.Spec); state {
		case realmUnknown:
			unknown = true
		case realmNamed:
			realms = append(realms, realm)
		case realmNone:
		}
	}

	return realms, unknown
}

// IdentityTemplateRealms returns the realm that the pod template of the
// Management Identity Deployment names, and reports whether that template
// writes a realm it does not name. A Deployment names at most one realm, and
// none at all when it starts no pod against a Keycloak: one scaled to zero,
// and one of the oidc mode.
//
// The template outlives every pod of it. A Deployment that still names a
// realm starts a pod against that realm at any moment, so a caller that reads
// the pods alone gives the realm back in the gap between a pod that went and
// its replacement.
func IdentityTemplateRealms(deployment *appsv1.Deployment) ([]v1.KeycloakRealmTarget, bool) {
	return templateRealms(deployment.Spec.Replicas, &deployment.Spec.Template.Spec)
}

// IdentityReplicaSetRealms returns the realm of every Management Identity
// ReplicaSet of sets that can still create a pod, and reports whether one of
// them writes a realm that its template does not name.
//
// A ReplicaSet keeps the template it was made from. The old ReplicaSet of a
// rollout therefore starts a pod against the realm the Deployment has already
// left, for as long as it is not scaled to zero.
func IdentityReplicaSetRealms(sets []appsv1.ReplicaSet) ([]v1.KeycloakRealmTarget, bool) {
	var realms []v1.KeycloakRealmTarget
	var unknown bool
	for i := range sets {
		set := &sets[i]
		setRealms, setUnknown := templateRealms(set.Spec.Replicas, &set.Spec.Template.Spec)
		realms = append(realms, setRealms...)
		unknown = unknown || setUnknown
	}

	return realms, unknown
}

// templateRealms returns the realm that a pod template names, and reports
// whether it writes a realm it does not name. A template names none when it
// starts no pod against a Keycloak: a workload scaled to zero, and the oidc
// mode. An unset replica count is one replica, as Kubernetes reads it.
func templateRealms(replicas *int32, spec *corev1.PodSpec) ([]v1.KeycloakRealmTarget, bool) {
	if replicas != nil && *replicas == 0 {
		return nil, false
	}

	switch realm, state := identityRealmEnv(spec); state {
	case realmUnknown:
		return nil, true
	case realmNamed:
		return []v1.KeycloakRealmTarget{realm}, false
	default:
		return nil, false
	}
}

// podReady reports the Ready condition of pod.
func podReady(pod *corev1.Pod) bool {
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			return condition.Status == corev1.ConditionTrue
		}
	}

	return false
}

// podDone reports a pod whose containers ran and will not run again, which a
// Deployment can leave behind as an object until it is collected.
func podDone(pod *corev1.Pod) bool {
	return pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed
}

// realmState is what the environment of a Management Identity container says
// about the realm that its Optimize client lives in.
type realmState int

const (
	// realmNone is a container that names no Keycloak, which is the oidc mode.
	realmNone realmState = iota
	// realmNamed is a container whose environment holds the realm.
	realmNamed
	// realmUnknown is a container that takes the Keycloak URL or the realm
	// from a reference, so the realm is not in the workload.
	realmUnknown
)

// identityRealmEnv reads the Keycloak URL and the realm that the Management
// Identity container of spec points at, and reports what the environment
// says about them. The realm is meaningful for realmNamed alone.
func identityRealmEnv(spec *corev1.PodSpec) (v1.KeycloakRealmTarget, realmState) {
	var realm v1.KeycloakRealmTarget
	var unknown bool
	for _, container := range spec.Containers {
		if container.Name != identityContainer {
			continue
		}
		for _, env := range container.Env {
			switch env.Name {
			case keycloakEnvURL:
				realm.URL = env.Value
			case keycloakEnvRealm:
				realm.Realm = env.Value
			default:
				continue
			}
			if env.ValueFrom != nil {
				unknown = true
			}
		}
	}

	switch {
	case unknown:
		return realm, realmUnknown
	case realm.URL != "":
		return realm, realmNamed
	default:
		return realm, realmNone
	}
}
