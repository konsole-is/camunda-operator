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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"

	"github.com/konsole-is/camunda-operator/test/utils"
)

// browserLoginRequest is one browser login against an application that
// authenticates through a Keycloak realm.
type browserLoginRequest struct {
	// Namespace is where the helper pod runs and where PasswordSecret lives.
	// Name distinguishes the pod from the other pods of the flow.
	Namespace string
	Name      string
	// StartURL is the page that a browser opens first. The application
	// answers it with the redirect to the authorization endpoint of the
	// realm.
	StartURL string
	// ProtectedURL is the endpoint that answers 200 to a caller with a
	// session and something else to an anonymous one.
	ProtectedURL string
	// Username is the user of the realm to sign in as. PasswordSecret is the
	// Secret in Namespace that holds the password of that user under
	// PasswordKey.
	Username       string
	PasswordSecret string
	PasswordKey    string
}

// browserLogin signs Username in to the application through the OpenID
// Connect authorization-code flow, from a pod of the cluster, and reads
// ProtectedURL with the session it gets. It returns what the pod wrote: one
// line for each hop of the flow, and the body of the protected endpoint after
// them.
//
// The error names the hop that broke and carries the status, the URL, and the
// body of it. Each hop fails for a reason of its own: a client that the realm
// does not serve on the browser, a redirect URI that the client does not
// carry, credentials that the realm refuses, and a session that the
// application does not accept.
//
// The helper reads the login page of Keycloak, which both flows of the suite
// run. It takes the form of the page by the login-actions/authenticate action
// that Keycloak renders on it.
func browserLogin(login browserLoginRequest) (string, error) {
	return utils.RunPod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "login-" + login.Name + "-" + utilrand.String(5),
			Namespace: login.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "curl",
				Image:   utils.CurlImage,
				Command: []string{"sh"},
				Args:    []string{"-ec", browserLoginScript},
				Env: []corev1.EnvVar{
					{Name: "LOGIN_START_URL", Value: login.StartURL},
					{Name: "LOGIN_PROTECTED_URL", Value: login.ProtectedURL},
					{Name: "LOGIN_USERNAME", Value: login.Username},
					utils.SecretEnv("LOGIN_PASSWORD", login.PasswordSecret, login.PasswordKey),
				},
			}},
		},
	}, podTimeout)
}

// browserLoginScript runs the hops of the flow with one cookie jar, the way a
// browser runs them.
//
// The user and the password go through --data-urlencode, so a name or a
// password with a reserved character in it reaches Keycloak whole, and neither
// of them ever reaches a URL.
//
// The third hop ends where the application takes the session over, so the
// check on it is the origin of the start URL. Keycloak answers a user it
// refuses, and a user whose profile it wants completed, with a page of its own
// under its own origin, and both stop the login there rather than three lines
// later on a 401 that says nothing about the cause.
//
// The last hop follows no redirect and asks for no content type of its own. A
// caller without a session gets the 401 of an API path or the redirect of a
// web application, and neither of those is a 200, so the status alone tells a
// session that the application accepts from one that it does not.
const browserLoginScript = `JAR=/tmp/cookies
BODY=/tmp/body
ORIGIN=$(echo "$LOGIN_START_URL" | cut -d/ -f1-3)
: > "$JAR"

request() {
  name=$1
  url=$2
  shift 2
  STATUS="no answer"
  URL=$url
  REDIRECT="[]"
  : > "$BODY"
  meta=$(curl -sS --cookie-jar "$JAR" --cookie "$JAR" -o "$BODY" ` +
	`-w '%{http_code} %{url_effective} [%{redirect_url}]' "$@" "$url") || fail "$name"
  STATUS=$(echo "$meta" | cut -d' ' -f1)
  URL=$(echo "$meta" | cut -d' ' -f2)
  REDIRECT=$(echo "$meta" | cut -d' ' -f3)
  echo "$name: $STATUS $URL $REDIRECT"
}

fail() {
  echo "the browser login of $LOGIN_USERNAME broke at $1: $STATUS $URL $REDIRECT" >&2
  cat "$BODY" >&2
  exit 1
}

request "login page" "$LOGIN_START_URL" -L -H 'Accept: text/html'
ACTION=$(grep -o 'action="[^"]*login-actions/authenticate[^"]*"' "$BODY" | ` +
	`head -n 1 | sed -e 's/^action="//' -e 's/"$//' -e 's/&amp;/\&/g')
if [ -z "$ACTION" ]; then fail "the login page of the realm"; fi

request "callback" "$ACTION" -L --data-urlencode "username=$LOGIN_USERNAME" ` +
	`--data-urlencode "password=$LOGIN_PASSWORD"
case "$URL" in "$ORIGIN"*) ;; *) fail "the return to the application" ;; esac

request "protected endpoint" "$LOGIN_PROTECTED_URL"
if [ "$STATUS" != "200" ]; then fail "the protected endpoint"; fi
cat "$BODY"`
