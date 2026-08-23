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

package utils

import (
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
)

// CurlImage is the pinned curl image of the in-cluster helper pods.
// renovate: datasource=docker depName=curlimages/curl
const CurlImage = "curlimages/curl:8.21.0"

// fileNamePattern is what a file of CamundaRequest.Files can be called: a
// plain file name that is safe inside the shell script of the helper pod.
var fileNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// CamundaAuth is how one call authenticates. BasicAuth and ClientCredentials
// implement it. Each contributes the variables the helper pod reads its
// credentials from, and the shell that defines the function every call goes
// through.
//
// The methods are unexported on purpose, so this package holds every
// authentication mode. A caller outside it selects a mode, and cannot add
// one.
type CamundaAuth interface {
	// env returns the variables of the helper pod.
	env() []corev1.EnvVar
	// script returns shell that defines the function camunda_curl, which
	// takes curl arguments and adds the authentication of this mode.
	script() string
}

// BasicAuth sends the user of a Secret with curl -u. The pod reads the Secret
// through secretKeyRef, so the password never appears in the pod spec.
type BasicAuth struct {
	// Username is the user to sign in as. Set it for a Secret that holds the
	// password alone; it wins over UsernameKey.
	Username string
	// Secret is the Secret in the namespace of the helper pod that holds the
	// user under UsernameKey and PasswordKey.
	Secret      string
	UsernameKey string
	PasswordKey string
}

func (a BasicAuth) env() []corev1.EnvVar {
	user := SecretEnv("CAMUNDA_USER", a.Secret, a.UsernameKey)
	if a.Username != "" {
		user = corev1.EnvVar{Name: "CAMUNDA_USER", Value: a.Username}
	}

	return []corev1.EnvVar{user, SecretEnv("CAMUNDA_PASSWORD", a.Secret, a.PasswordKey)}
}

func (a BasicAuth) script() string {
	return `camunda_curl() { curl -sS -L -u "$CAMUNDA_USER:$CAMUNDA_PASSWORD" "$@"; }`
}

// ClientCredentials reads an access token with the OAuth client-credentials
// grant, then sends it as a bearer token. Both requests happen inside the one
// helper pod, so the token never reaches the output of the suite.
type ClientCredentials struct {
	// TokenURL is the token endpoint of the identity provider.
	TokenURL string
	// ClientID and Audience are sent as form fields.
	ClientID string
	Audience string
	// Secret is the Secret in the namespace of the helper pod that holds the
	// client secret under SecretKey.
	Secret    string
	SecretKey string
}

func (a ClientCredentials) env() []corev1.EnvVar {
	return []corev1.EnvVar{
		{Name: "CAMUNDA_TOKEN_URL", Value: a.TokenURL},
		{Name: "CAMUNDA_CLIENT_ID", Value: a.ClientID},
		{Name: "CAMUNDA_AUDIENCE", Value: a.Audience},
		SecretEnv("CAMUNDA_CLIENT_SECRET", a.Secret, a.SecretKey),
	}
}

// The token endpoint answers JSON, and the curl image carries no jq, so the
// token is cut out with sed. An empty result fails the pod, because a call
// without a token would otherwise look like an authorization problem of the
// cluster under test.
func (a ClientCredentials) script() string {
	return `CAMUNDA_TOKEN=$(curl -sS -X POST ` +
		`-d grant_type=client_credentials ` +
		`--data-urlencode "client_id=$CAMUNDA_CLIENT_ID" ` +
		`--data-urlencode "client_secret=$CAMUNDA_CLIENT_SECRET" ` +
		`--data-urlencode "audience=$CAMUNDA_AUDIENCE" ` +
		`"$CAMUNDA_TOKEN_URL" | ` +
		`sed -n 's/.*"access_token"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
if [ -z "$CAMUNDA_TOKEN" ]; then echo "no access_token from $CAMUNDA_TOKEN_URL" >&2; exit 1; fi
camunda_curl() { curl -sS -L -H "Authorization: Bearer $CAMUNDA_TOKEN" "$@"; }`
}

// anonymousScript is the mode of a nil CamundaAuth: no credentials at all,
// which is how the suite proves that an endpoint refuses an anonymous call.
const anonymousScript = `camunda_curl() { curl -sS -L "$@"; }`

// CamundaRequest is one call from a helper pod against the Orchestration
// Cluster REST API of a CamundaCluster.
type CamundaRequest struct {
	// Namespace is where the helper pod runs. Name distinguishes the pod;
	// a random suffix makes every pod unique.
	Namespace string
	Name      string
	// Method and URL are the HTTP method and the full URL of the call.
	Method string
	URL    string
	// Auth is how the call authenticates. A nil Auth sends no credentials.
	Auth CamundaAuth
	// Files are written into /tmp of the pod before the call, by file name,
	// so a form part can upload them (-F resources=@/tmp/<name>). A name is
	// letters, digits, dots, dashes, and underscores only.
	Files map[string]string
	// Args are extra curl arguments: headers, a JSON body, form parts.
	Args []string
	// Timeout bounds the helper pod.
	Timeout time.Duration
}

// CamundaResponse is the result of one call, after curl followed every
// redirect.
type CamundaResponse struct {
	// Status is the status code of the final response.
	Status int
	// URL is the URL the call ended at. It differs from the URL of the
	// request when the server redirected, which is what proves an OIDC login
	// redirect.
	URL string
	// Body is the body of the final response.
	Body string
}

// CamundaREST performs req with curl, follows redirects, and returns the
// final status code, the final URL, and the body. A curl failure (for example
// a connection refused) is an error, and so is a token that the identity
// provider does not give out.
func CamundaREST(req CamundaRequest) (CamundaResponse, error) {
	var env []corev1.EnvVar
	auth := anonymousScript
	if req.Auth != nil {
		env = append(env, req.Auth.env()...)
		auth = req.Auth.script()
	}

	var script strings.Builder
	for i, name := range slices.Sorted(maps.Keys(req.Files)) {
		if !fileNamePattern.MatchString(name) {
			return CamundaResponse{}, fmt.Errorf("file name %q is not a plain file name", name)
		}
		envName := "CAMUNDA_FILE_" + strconv.Itoa(i)
		env = append(env, corev1.EnvVar{Name: envName, Value: req.Files[name]})
		fmt.Fprintf(&script, "printf '%%s' \"$%s\" > '/tmp/%s'\n", envName, name)
	}

	script.WriteString(auth)
	// $0 is "curl", the remaining arguments are the curl arguments. The
	// status code comes first, the final URL second, the body after them.
	script.WriteString(
		"\ncamunda_curl -o /tmp/response -w '%{http_code}\\n%{url_effective}\\n' \"$@\"\n" +
			"cat /tmp/response",
	)

	args := append([]string{"-ec", script.String(), "curl", "-X", req.Method}, req.Args...)
	args = append(args, req.URL)

	out, err := RunPod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "curl-" + req.Name + "-" + utilrand.String(5),
			Namespace: req.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:    "curl",
				Image:   CurlImage,
				Command: []string{"sh"},
				Args:    args,
				Env:     env,
			}},
		},
	}, req.Timeout)
	if err != nil {
		return CamundaResponse{Body: out}, err
	}

	code, rest, _ := strings.Cut(out, "\n")
	url, body, _ := strings.Cut(rest, "\n")
	status, err := strconv.Atoi(strings.TrimSpace(code))
	if err != nil {
		return CamundaResponse{Body: out}, fmt.Errorf(
			"parsing the status code of %s %s: %w", req.Method, req.URL, err,
		)
	}

	return CamundaResponse{Status: status, URL: strings.TrimSpace(url), Body: body}, nil
}

// SecretEnv returns an environment variable that reads key of the Secret
// name in the namespace of the pod.
func SecretEnv(envName, name, key string) corev1.EnvVar {
	return corev1.EnvVar{
		Name: envName,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: name},
				Key:                  key,
			},
		},
	}
}
