//go:build e2e
// +build e2e

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

package e2e

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	optimizecomponents "github.com/konsole-is/camunda-operator/pkg/components/camundaoptimize"
	"github.com/konsole-is/camunda-operator/test/utils"
)

// The fixtures of the two realm journeys of the keycloak flow: a management
// plane that moves its login callbacks from one realm to another, and a second
// plane that waits for the realm the first one holds. Both run in the
// externalKeycloak mode against the Keycloak that the flow already runs, on
// realms of their own. Neither realm is mcRealm, which the management plane of
// the flow holds.
const (
	// mcRealmA is the realm the retargeted plane starts in, and mcRealmB the
	// one it moves to and the second plane waits for.
	mcRealmA = "e2e-retarget-a"
	mcRealmB = "e2e-retarget-b"

	// mcRetargetName is the plane that moves from realm A to realm B.
	// mcClaimantName is the plane that names realm B while the first one
	// still holds it.
	mcRetargetName = "camunda-management-retarget"
	mcClaimantName = "camunda-management-claimant"

	// The Management Identity database of each plane. Identity owns every
	// table of its database, so no two planes share one.
	mcRetargetIdentityDB = "management-retarget-identity-db"
	mcClaimantIdentityDB = "management-claimant-identity-db"

	// The first administrator of each plane. The two names differ, because
	// both planes end up on realm B and Management Identity creates its first
	// administrator as a user of the realm it administers.
	mcRetargetAdmin = "retarget-admin"
	mcClaimantAdmin = "claimant-admin"

	// mcRetargetOptimizeName is the CamundaOptimize that the retargeted plane
	// serves. Its cluster is still to come, so it renders no workload, and
	// the management plane finds it all the same: discovery reads
	// spec.managementAuthRef and spec.externalUrl and nothing of the cluster.
	mcRetargetOptimizeName = "camunda-management-retarget-optimize"
	// mcRetargetOptimizeCluster is the CamundaCluster that
	// mcRetargetOptimizeName waits for.
	mcRetargetOptimizeCluster = "camunda-management-retarget-cluster-to-come"

	// mcOptimizeClientID is the client that Management Identity creates for
	// Optimize in the realm it administers. The operator writes the login
	// callbacks of every Optimize it serves on that client.
	mcOptimizeClientID = "optimize"
	// mcOptimizeClientJSON is what the answer of realmClient carries for a
	// realm that holds that client. A read of a realm Keycloak does not serve
	// is empty and a read of a realm without the client is an empty list, so
	// an assertion that a callback is gone holds in both. This one beside it
	// says the realm and the client are there and the callback is not.
	mcOptimizeClientJSON = `"clientId":"` + mcOptimizeClientID + `"`

	// handoffWatchSeconds bounds the watch of the two realms inside the pod,
	// and handoffPodTimeout bounds the pod itself. Both cover a whole move:
	// the withdrawal from the old realm, the stop of every Management Identity
	// that points at it, and the start of the one of the new realm, which
	// registers the login callbacks while it starts.
	handoffWatchSeconds = 480
	handoffPodTimeout   = 10 * time.Minute
)

// externalKeycloakPlane returns a management plane in the externalKeycloak
// mode: Management Identity alone on realm, against the Keycloak that keycloak
// runs, signed in with the administrator that the Keycloak Operator published
// beside it.
//
// It deploys no Console and no Web Modeler, and it selects no orchestration
// cluster, so one such plane costs the node one Management Identity.
func externalKeycloakPlane(
	name, realm, databaseRef, admin string,
	keycloak *v1.CamundaManagementCluster,
) *v1.CamundaManagementCluster {
	mc := &v1.CamundaManagementCluster{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaManagementCluster"},
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: mcKeycloakNamespace},
	}
	mc.Spec = v1.CamundaManagementClusterSpec{
		PlatformConfigRef: mcKeycloakPlatform,
		IdentityProvider: v1.IdentityProviderSpec{
			ExternalKeycloak: &v1.ExternalKeycloakSpec{
				URL:   keycloakServiceURL(keycloak),
				Realm: realm,
				AdminCredentialsSecretRef: v1.LocalCredentialsSecretRef{
					Name:        components.KeycloakInitialAdminSecretName(keycloak),
					UsernameKey: components.KeycloakAdminUsernameKey,
					PasswordKey: components.KeycloakAdminPasswordKey,
				},
			},
		},
		Identity: v1.IdentitySpec{
			Version:           os.Getenv(envIdentityVersion),
			ExternalURL:       components.IdentityServiceURL(mc),
			DatabaseConfigRef: databaseRef,
			Admin:             v1.IdentityAdminSpec{Username: admin, Email: admin + "@example.com"},
			WorkloadSpec:      v1.WorkloadSpec{Resources: capped("150m", "512Mi", "1280Mi")},
		},
	}

	return mc
}

// retargetOptimize returns the CamundaOptimize that the retargeted plane
// serves. A plane records the realm of its login callbacks only while it
// serves an Optimize, so the journey needs one, and this one renders no
// workload.
func retargetOptimize() *v1.CamundaOptimize {
	return &v1.CamundaOptimize{
		TypeMeta:   metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaOptimize"},
		ObjectMeta: metav1.ObjectMeta{Name: mcRetargetOptimizeName, Namespace: mcKeycloakNamespace},
		Spec: v1.CamundaOptimizeSpec{
			Version:           os.Getenv(envOptimizeVersion),
			ManagementAuthRef: mcRetargetName,
			ExternalURL:       retargetOptimizeURL(),
			ClusterRef:        v1.ClusterRef{Name: mcRetargetOptimizeCluster},
		},
	}
}

// retargetOptimizeURL is the address of the Optimize webapp of the journeys,
// in the shape that spec.externalUrl carries. Nothing dials it. The management
// plane registers the login callback under it, and the realm is where the
// journeys read that registration.
func retargetOptimizeURL() string {
	optimize := &v1.CamundaOptimize{ObjectMeta: metav1.ObjectMeta{Name: mcRetargetOptimizeName}}

	return fmt.Sprintf(
		"http://%s.%s.svc:%d",
		optimizecomponents.WorkloadName(optimize, optimizecomponents.ComponentWebapp),
		mcKeycloakNamespace, optimizecomponents.PortHTTP,
	)
}

// retargetCallbackURL is the login callback that the management plane
// registers for the Optimize of the journeys.
func retargetCallbackURL() string {
	return retargetOptimizeURL() + components.OptimizeCallbackPath
}

// managementPlane returns the CamundaManagementCluster called name, from the
// namespace of the keycloak flow. One read carries the record and every
// condition, so a caller that reads both reads one state rather than two.
func managementPlane(g Gomega, name string) *v1.CamundaManagementCluster {
	var mc v1.CamundaManagementCluster
	g.Expect(utils.Get(mcResource, name, mcKeycloakNamespace, &mc)).To(Succeed())

	return &mc
}

// expectCallbackRealm asserts the realm that mc records. A plane that records
// nothing fails, rather than reading as a plane that records another realm.
func expectCallbackRealm(g Gomega, mc *v1.CamundaManagementCluster, realm string) {
	g.Expect(mc.Status.CallbackRealm).NotTo(
		BeNil(), "CamundaManagementCluster %q records no callback realm", mc.Name,
	)
	g.Expect(mc.Status.CallbackRealm.Realm).To(Equal(realm))
}

// realmClient returns the JSON representation of the client clientID in realm,
// on the Keycloak that keycloak runs, read through the admin API as the
// administrator that the Keycloak Operator published beside it.
//
// A realm that Keycloak does not serve gives an empty answer, and a realm that
// holds no such client gives an empty JSON list, so a caller tells a realm
// that nothing created yet from one that holds no such client.
func realmClient(keycloak *v1.CamundaManagementCluster, realm, clientID string) (string, error) {
	adminSecret := components.KeycloakInitialAdminSecretName(keycloak)

	return utils.RunPod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keycloak-realm-client-" + utilrand.String(5),
			Namespace: keycloak.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "curl",
				Image:   utils.CurlImage,
				Command: []string{"sh"},
				Args:    []string{"-ec", readRealmClientScript},
				Env: []corev1.EnvVar{
					{Name: "KC_URL", Value: keycloakServiceURL(keycloak)},
					{Name: "KC_REALM", Value: realm},
					{Name: "KC_CLIENT_ID", Value: clientID},
					utils.SecretEnv("KC_USER", adminSecret, components.KeycloakAdminUsernameKey),
					utils.SecretEnv("KC_PASSWORD", adminSecret, components.KeycloakAdminPasswordKey),
				},
			}},
		},
	}, podTimeout)
}

// readRealmClientScript prints the clients of KC_REALM whose client id is
// KC_CLIENT_ID. A realm that Keycloak does not serve answers 404 and prints
// nothing, which is a state of its own for the caller and no failure.
//
// The spaces go, the way the role reads of this suite drop them, so a caller
// matches a field of the answer whatever Keycloak puts between the colon and
// the value. No value that a caller matches carries a space.
const readRealmClientScript = keycloakTokenScript + `KC_CODE=$(curl -sS -o /tmp/response ` +
	`-w '%{http_code}' -H "Authorization: Bearer $KC_TOKEN" ` +
	`"$KC_URL/admin/realms/$KC_REALM/clients?clientId=$KC_CLIENT_ID")
case "$KC_CODE" in
  200) tr -d ' ' < /tmp/response ;;
  404) ;;
  *) echo "reading the clients of realm $KC_REALM: $KC_CODE" >&2; cat /tmp/response >&2; exit 1 ;;
esac`

// realmCallbackHandoff watches the client clientID of both realms while the
// management plane moves its login callbacks, and returns one line for each
// sample it took:
//
//	t=<seconds> <realmFrom>=<0|1> <realmTo>=<0|1>
//
// A 1 says the client of that realm carries callback. The watch ends at the
// first sample that finds it in realmTo, and it fails when no sample does
// before its own deadline. A realm that Keycloak does not serve reads as a 0,
// so a realm that Management Identity has not created yet is no failure.
//
// The result outlives the move, so a caller reads the order of the two realms
// from it whatever the plane did between two samples.
func realmCallbackHandoff(
	keycloak *v1.CamundaManagementCluster,
	realmFrom, realmTo, clientID, callback string,
) (string, error) {
	adminSecret := components.KeycloakInitialAdminSecretName(keycloak)

	return utils.RunPod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "keycloak-realm-handoff-" + utilrand.String(5),
			Namespace: keycloak.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "curl",
				Image:   utils.CurlImage,
				Command: []string{"sh"},
				Args:    []string{"-ec", watchRealmCallbacksScript},
				Env: []corev1.EnvVar{
					{Name: "KC_URL", Value: keycloakServiceURL(keycloak)},
					{Name: "KC_REALM_FROM", Value: realmFrom},
					{Name: "KC_REALM_TO", Value: realmTo},
					{Name: "KC_CLIENT_ID", Value: clientID},
					{Name: "KC_CALLBACK", Value: callback},
					{Name: "KC_WATCH_SECONDS", Value: strconv.Itoa(handoffWatchSeconds)},
					utils.SecretEnv("KC_USER", adminSecret, components.KeycloakAdminUsernameKey),
					utils.SecretEnv("KC_PASSWORD", adminSecret, components.KeycloakAdminPasswordKey),
				},
			}},
		},
	}, handoffPodTimeout)
}

// watchRealmCallbacksScript samples the client of both realms every three
// seconds and prints one line for each sample. It reads its deadline before
// each sample, so it starts none after it. The first sample takes the token of
// the prelude, and each sleep is followed by a token of its own, because the
// watch outlives the lifespan of one.
//
// A probe exits the script through its command substitution: an unexpected
// status from Keycloak ends the watch rather than reading as an absent
// callback. A refused connection is retried instead, because the watch makes
// hundreds of requests and one of them meets a Keycloak that is busy. Every
// branch of the loop is an if, because a bare test at the end of an iteration
// would end the script under set -e.
const watchRealmCallbacksScript = keycloakTokenScript + `probe() {
  code=$(curl -sS --retry 3 --retry-connrefused --retry-delay 1 ` +
	`-o /tmp/probe -w '%{http_code}' -H "Authorization: Bearer $KC_TOKEN" ` +
	`"$KC_URL/admin/realms/$1/clients?clientId=$KC_CLIENT_ID")
  case "$code" in
    200) if grep -qF -- "$KC_CALLBACK" /tmp/probe; then echo 1; else echo 0; fi ;;
    404) echo 0 ;;
    *) echo "reading the clients of realm $1: $code" >&2; exit 1 ;;
  esac
}

start=$(date +%s)
while :; do
  if [ "$(( $(date +%s) - start ))" -ge "$KC_WATCH_SECONDS" ]; then
    echo "realm $KC_REALM_TO never carried the login callback" >&2
    exit 1
  fi
  from=$(probe "$KC_REALM_FROM")
  to=$(probe "$KC_REALM_TO")
  echo "t=$(( $(date +%s) - start )) $KC_REALM_FROM=$from $KC_REALM_TO=$to"
  if [ "$to" = 1 ]; then exit 0; fi
  sleep 3
  KC_TOKEN=$(keycloak_token)
  if [ -z "$KC_TOKEN" ]; then echo "no access_token from $KC_URL" >&2; exit 1; fi
done`

// expectCallbackHandoff asserts the order of the two realms from the samples
// that realmCallbackHandoff took: no sample carries the login callback in both
// realms, one carries it in neither, and the last one carries it in realmTo.
func expectCallbackHandoff(samples, realmFrom, realmTo string) {
	GinkgoHelper()

	lines := strings.Split(strings.TrimSpace(samples), "\n")
	taken := make([][]string, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		Expect(fields).To(HaveLen(3), "the watch of the two realms wrote %q", line)
		taken = append(taken, fields)
	}

	// A watch that started after the callbacks reached realmTo saw no order at
	// all, and it passes every assertion below without proving one.
	Expect(taken[0][2]).To(
		Equal(realmTo+"=0"), "the watch of the two realms started too late:\n%s", samples,
	)

	var both, neither []string
	for _, fields := range taken {
		if both == nil && fields[1] == realmFrom+"=1" && fields[2] == realmTo+"=1" {
			both = fields
		}
		if neither == nil && fields[1] == realmFrom+"=0" && fields[2] == realmTo+"=0" {
			neither = fields
		}
	}
	Expect(both).To(
		BeEmpty(),
		"realm %q carried the login callback while realm %q still carried it:\n%s",
		realmTo, realmFrom, samples,
	)
	// The plane empties the old realm, stops every Management Identity that
	// points at it, and starts the one of the new realm, which registers the
	// callbacks while it starts. The two realms are therefore empty for a
	// Management Identity start, and a watch that never read that state read
	// something other than a handoff.
	Expect(neither).NotTo(
		BeEmpty(),
		"no sample found the login callback gone from realm %q and not yet in realm %q:\n%s",
		realmFrom, realmTo, samples,
	)

	Expect(taken[len(taken)-1][2]).To(
		Equal(realmTo+"=1"),
		"the watch ended without the login callback in realm %q:\n%s", realmTo, samples,
	)
}

// identityUID returns the UID of the Management Identity Deployment of mc. A
// withdrawal from a realm deletes that Deployment, and the component builds it
// again, so a new UID is one restart of Management Identity.
func identityUID(mc *v1.CamundaManagementCluster) (types.UID, error) {
	var identity appsv1.Deployment
	if err := utils.Get("deployment", components.IdentityName(mc), mc.Namespace, &identity); err != nil {
		return "", err
	}

	return identity.UID, nil
}

// realmClaimLease returns the Lease that claims realm on the Keycloak that
// keycloak runs. The claims live in the namespace of the operator, and their
// annotations name the management plane that holds each one.
func realmClaimLease(keycloak *v1.CamundaManagementCluster, realm string) (*coordinationv1.Lease, error) {
	identity := components.RealmIdentity(v1.KeycloakRealmTarget{
		URL:   keycloakServiceURL(keycloak),
		Realm: realm,
	})

	var lease coordinationv1.Lease
	if err := utils.Get("lease", components.RealmClaimLeaseName(identity), namespace, &lease); err != nil {
		return nil, err
	}

	return &lease, nil
}
