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
	"strconv"
	"strings"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// RealmTarget records the realm of provider: where it is, and which Secrets
// the operator signs in to it with. In the externalKeycloak mode it is what
// status.callbackRealm holds after the login callbacks of Optimize are
// registered there. The keycloak mode returns a target too, which is the realm
// that mode claims and the one a record is compared against, but that target
// is never persisted: the operator deletes that Keycloak together with the
// mode, so there would be nothing to withdraw from.
//
// The URL is recorded as RealmIdentity folds it, so two spellings of one realm
// record one value and a user in the URL reaches no status.
//
// The oidc mode administers no realm, and a provider that names no
// administrator gives the operator nothing to sign in with, so both record
// nil.
func RealmTarget(provider IdentityProvider) *v1.KeycloakRealmTarget {
	if provider.Mode == ModeOIDC || provider.AdminCredentials == nil {
		return nil
	}

	return &v1.KeycloakRealmTarget{
		// The CEL rules on url admit a user with a password in it. The record
		// is written to the API server, so it holds the folded URL, which the
		// operator reaches the same realm with and signs in to with the
		// administrator Secret the record names.
		URL:                       normalizeKeycloakURL(provider.KeycloakURL),
		Realm:                     provider.Realm,
		AdminCredentialsSecretRef: *provider.AdminCredentials.DeepCopy(),
		CABundleSecretRef:         provider.CABundle.DeepCopy(),
	}
}

// RealmIdentity returns the name of the realm that target holds, as the URL
// of its realm endpoint: the URL, then /realms/ and the realm. Spellings of
// one Keycloak fold to one identity: the case of the scheme and of the host,
// a user in the URL, a default port (443 on https, 80 on http), a terminal
// dot of the host, the spelling of an IP literal or of a port, a percent
// escape of the path, and every trailing slash make no difference. The case
// of the path and of the realm does make one, because Keycloak treats both as
// case-sensitive. The administrator and the certificate authority take no
// part in it. The result is deterministic, so a name derived from it (a hash,
// for example) identifies the realm across resources, and it is safe to read,
// so a claim can name the realm it holds in the clear.
func RealmIdentity(target v1.KeycloakRealmTarget) string {
	return normalizeKeycloakURL(target.URL) + keycloakRealmPath + target.Realm
}

// normalizeKeycloakURL trims the trailing slashes of raw and folds every
// spelling of one server into one: the case of the scheme and of the host, a
// user in the URL, a default port of the scheme, a terminal dot of the host,
// the spelling of an IP literal or of a port, and a percent escape of the
// path. A raw value that does not parse as a URL comes back with only the
// slashes trimmed.
func normalizeKeycloakURL(raw string) string {
	folded, _ := foldKeycloakURL(raw)

	return folded
}

// foldKeycloakURL is normalizeKeycloakURL with the answer to whether the value
// is a realm identity and nothing else. A false says that the value held
// something no identity has, so a caller that shows the result to a user must
// not show it: what it held is still in there, or was dropped from a value
// that carries who knows what else.
func foldKeycloakURL(raw string) (string, bool) {
	trimmed := strings.TrimRight(raw, "/")
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return trimmed, false
	}

	// A realm is a URL with a path and nothing after it, which the CEL rules
	// on url hold to. A query and a fragment name no realm and can carry
	// anything, so they go, and the value counts as no identity.
	identity := parsed.RawQuery == "" && parsed.Fragment == ""
	parsed.RawQuery = ""
	parsed.Fragment = ""

	// A user in the URL reaches the same realm as the URL without it, and it
	// carries a password. Keeping it would let two spellings of one realm
	// take a claim each, and would write the password into the annotations of
	// the claim.
	parsed.User = nil
	parsed.Scheme = strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Hostname())
	// A terminal dot spells the fully qualified form of the same name, and one
	// IP address has many spellings, an IPv6 literal most of all. Each
	// spelling would otherwise take a claim of its own on one realm.
	if fqdn := strings.TrimSuffix(host, "."); fqdn != "" {
		host = fqdn
	}
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	}
	// A port is a number, and url.Parse admits the leading zeroes that spell
	// the same one. They go before the default port is read, so that 0443
	// reaches https as its default too.
	port := parsed.Port()
	if number, err := strconv.Atoi(port); err == nil {
		port = strconv.Itoa(number)
	}
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
	// url.Parse keeps the spelling of the path beside the decoded one, so
	// /%61uth and /auth would take a claim each on one realm.
	if escaped := foldUnreservedEscapes(parsed.EscapedPath()); escaped != "" {
		if decoded, err := url.PathUnescape(escaped); err == nil {
			parsed.Path, parsed.RawPath = decoded, escaped
		}
	}

	return parsed.String(), identity
}

// foldUnreservedEscapes decodes the percent escapes of an already escaped path
// that stand for an unreserved character, and writes every other escape in
// upper case. RFC 3986 makes those two spellings of one path, so /%61uth and
// /auth are one realm.
//
// An escape of a reserved character stays as it is. %2F and / are not one
// path: a server reads the first inside a segment and the second as the
// separator between two, and a proxy can route them apart.
func foldUnreservedEscapes(escaped string) string {
	var folded strings.Builder
	folded.Grow(len(escaped))
	for i := 0; i < len(escaped); i++ {
		if escaped[i] != '%' || i+2 >= len(escaped) {
			folded.WriteByte(escaped[i])

			continue
		}
		char, err := strconv.ParseUint(escaped[i+1:i+3], 16, 8)
		switch {
		case err != nil:
			folded.WriteByte(escaped[i])

			continue
		case unreserved(byte(char)):
			folded.WriteByte(byte(char))
		default:
			folded.WriteString(strings.ToUpper(escaped[i : i+3]))
		}
		i += 2
	}

	return folded.String()
}

// unreserved reports whether char is one of the unreserved characters of RFC
// 3986, which a percent escape and the character itself spell alike.
func unreserved(char byte) bool {
	switch {
	case char >= 'A' && char <= 'Z', char >= 'a' && char <= 'z', char >= '0' && char <= '9':
		return true
	default:
		return char == '-' || char == '.' || char == '_' || char == '~'
	}
}

// NormalizeRealmIdentity folds a hand-written realm identity the way
// RealmIdentity folds a URL, so an annotation written from
// status.callbackRealm matches the recorded identity however it is spelled.
//
// The second result is false for a value that is not a realm identity: one
// that does not parse as a URL comes back with a user and a password still in
// it, and one that carries a query or a fragment comes back without them. A
// message that a user reads must carry neither.
func NormalizeRealmIdentity(value string) (string, bool) {
	return foldKeycloakURL(value)
}

// RealmURL is the URL of target as RealmIdentity reads it, with every
// spelling folded into one and a user in the URL dropped. A message that
// names the Keycloak of a realm uses it, so a password in the URL never
// reaches a condition or an event.
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
// that points at another realm writes that one. A pod that is going away and
// was never ready counts, because its container can still be inside the start
// until the kubelet stops it. One that was ready before it went away does not,
// for the reason any ready pod does not.
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
// none at all when it starts no pod against a Keycloak: one of the oidc mode,
// and one that is fully scaled down, which scaledDown defines.
//
// The template outlives every pod of it. A Deployment that still names a
// realm starts a pod against that realm at any moment, so a caller that reads
// the pods alone gives the realm back in the gap between a pod that went and
// its replacement.
func IdentityTemplateRealms(deployment *appsv1.Deployment) ([]v1.KeycloakRealmTarget, bool) {
	if scaledDown(
		deployment.Spec.Replicas,
		deployment.Generation,
		deployment.Status.ObservedGeneration,
		deployment.Status.Replicas,
	) {
		return nil, false
	}

	return templateRealms(&deployment.Spec.Template.Spec)
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
		if scaledDown(
			set.Spec.Replicas, set.Generation, set.Status.ObservedGeneration, set.Status.Replicas,
		) {
			continue
		}
		setRealms, setUnknown := templateRealms(&set.Spec.Template.Spec)
		realms = append(realms, setRealms...)
		unknown = unknown || setUnknown
	}

	return realms, unknown
}

// scaledDown reports a Management Identity workload that starts no pod any
// more: it asks for none, its controller has read that, and it holds none.
//
// The three go together. A workload that asks for none and whose controller
// has not caught up can have the pod creation of the generation before it in
// flight, and that pod starts against the realm of the template.
//
// An unset replica count is one replica, as Kubernetes reads it.
//
// The withdrawal wait of stopOldIdentityWriters asks the same question of a
// ReplicaSet and answers it on the replica count alone. It must, because that
// wait holds status.callbackRealm: a rule that waits for a controller to catch
// up would keep a plane in the realm it is leaving whenever the count never
// gets read. A claim that is held too long only makes the next claimant wait
// for its retry, so this rule takes the safer side of the same question.
func scaledDown(replicas *int32, generation, observed int64, held int32) bool {
	return replicas != nil && *replicas == 0 && observed >= generation && held == 0
}

// templateRealms returns the realm that a pod template names, and reports
// whether it writes a realm it does not name. A template of the oidc mode
// names no Keycloak and returns none.
func templateRealms(spec *corev1.PodSpec) ([]v1.KeycloakRealmTarget, bool) {
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
	case realm.URL == "":
		// The operator renders both variables in every mode, and the oidc mode
		// renders an empty URL beside the default realm. No URL is no Keycloak.
		return realm, realmNone
	case realm.Realm == "":
		// A container that names a Keycloak and no realm takes the realm from
		// its image, so the realm it writes cannot be read from here.
		return realm, realmUnknown
	default:
		return realm, realmNamed
	}
}
