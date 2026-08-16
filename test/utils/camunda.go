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
	"slices"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CurlImage is the pinned curl image of the in-cluster helper pods.
const CurlImage = "curlimages/curl:8.17.0"

// CamundaRequest is one call from a helper pod against the Orchestration
// Cluster REST API of a CamundaCluster, authenticated with basic auth.
type CamundaRequest struct {
	// Namespace is where the helper pod runs. Name distinguishes the pod.
	Namespace string
	Name      string
	// Method and URL are the HTTP method and the full URL of the call.
	Method string
	URL    string
	// User and Password are the basic-auth credentials.
	User     string
	Password string
	// Files are written into /tmp of the pod before the call, by file name,
	// so a form part can upload them (-F resources=@/tmp/<name>).
	Files map[string]string
	// Args are extra curl arguments: headers, a JSON body, form parts.
	Args []string
	// Timeout bounds the helper pod.
	Timeout time.Duration
}

// CamundaREST performs req with curl, follows redirects, and returns the
// final HTTP status code and the response body. A curl failure (for example
// a connection refused) is an error.
func CamundaREST(req CamundaRequest) (int, string, error) {
	env := make([]corev1.EnvVar, 0, 2+len(req.Files))
	env = append(
		env,
		corev1.EnvVar{Name: "CAMUNDA_USER", Value: req.User},
		corev1.EnvVar{Name: "CAMUNDA_PASSWORD", Value: req.Password},
	)

	var script strings.Builder
	for i, name := range slices.Sorted(maps.Keys(req.Files)) {
		envName := "CAMUNDA_FILE_" + strconv.Itoa(i)
		env = append(env, corev1.EnvVar{Name: envName, Value: req.Files[name]})
		fmt.Fprintf(&script, "printf '%%s' \"$%s\" > /tmp/%s && ", envName, name)
	}
	// $0 is "curl", the remaining arguments are the curl arguments. The
	// status code goes first, the body after a newline.
	script.WriteString(
		`curl -sS -L -u "$CAMUNDA_USER:$CAMUNDA_PASSWORD" -o /tmp/response -w '%{http_code}' "$@" && ` +
			`echo && cat /tmp/response`,
	)

	args := append([]string{"-ec", script.String(), "curl", "-X", req.Method}, req.Args...)
	args = append(args, req.URL)

	out, err := RunPod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "curl-" + req.Name, Namespace: req.Namespace},
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
		return 0, out, err
	}

	code, body, _ := strings.Cut(out, "\n")
	status, err := strconv.Atoi(strings.TrimSpace(code))
	if err != nil {
		return 0, out, fmt.Errorf("parsing the status code of %s %s: %w", req.Method, req.URL, err)
	}

	return status, body, nil
}
