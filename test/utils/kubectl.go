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
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Kubectl runs kubectl with args and returns its combined output.
func Kubectl(args ...string) (string, error) {
	return Run(exec.Command("kubectl", args...))
}

// KubectlWithStdin runs kubectl with args and stdin as its standard input.
func KubectlWithStdin(stdin string, args ...string) (string, error) {
	cmd := exec.Command("kubectl", args...)
	cmd.Stdin = strings.NewReader(stdin)
	return Run(cmd)
}

// namespaceArgs returns the -n flag for namespace, or nothing for a
// cluster-scoped call.
func namespaceArgs(namespace string) []string {
	if namespace == "" {
		return nil
	}
	return []string{"-n", namespace}
}

// Get reads resource name in namespace as JSON into obj. An empty namespace
// reads a cluster-scoped resource.
func Get(resource, name, namespace string, obj any) error {
	args := append([]string{"get", resource, name, "-o", "json"}, namespaceArgs(namespace)...)
	out, err := Kubectl(args...)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(out), obj); err != nil {
		return fmt.Errorf("decoding %s %q: %w", resource, name, err)
	}

	return nil
}

// List reads the resources in namespace that match the label selector as JSON
// into list. An empty selector matches everything.
func List(resource, namespace, selector string, list any) error {
	args := append([]string{"get", resource, "-o", "json"}, namespaceArgs(namespace)...)
	if selector != "" {
		args = append(args, "-l", selector)
	}
	out, err := Kubectl(args...)
	if err != nil {
		return err
	}

	if err := json.Unmarshal([]byte(out), list); err != nil {
		return fmt.Errorf("decoding %s list: %w", resource, err)
	}

	return nil
}

// Exists reports whether resource name exists in namespace. An empty
// namespace names a cluster-scoped resource.
func Exists(resource, name, namespace string) (bool, error) {
	args := append([]string{"get", resource, name, "--ignore-not-found", "-o", "name"}, namespaceArgs(namespace)...)
	out, err := Kubectl(args...)
	if err != nil {
		return false, err
	}

	return strings.TrimSpace(out) != "", nil
}

// Condition returns the status condition of type condType on resource name,
// or nil when the resource carries no such condition.
func Condition(resource, name, namespace, condType string) (*metav1.Condition, error) {
	var obj struct {
		Status struct {
			Conditions []metav1.Condition `json:"conditions"`
		} `json:"status"`
	}
	if err := Get(resource, name, namespace, &obj); err != nil {
		return nil, err
	}

	return meta.FindStatusCondition(obj.Status.Conditions, condType), nil
}

// SecretValue returns the decoded value of key in Secret name.
func SecretValue(namespace, name, key string) (string, error) {
	var secret corev1.Secret
	if err := Get("secret", name, namespace, &secret); err != nil {
		return "", err
	}

	value, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("secret %s/%s has no key %q", namespace, name, key)
	}

	return string(value), nil
}

// RunPod creates pod, waits until it completes, and returns its logs. It
// deletes the pod afterwards. A pod that fails, or that does not complete
// within timeout, is an error that carries the logs.
func RunPod(pod *corev1.Pod, timeout time.Duration) (string, error) {
	pod.APIVersion = "v1"
	pod.Kind = "Pod"
	pod.Spec.RestartPolicy = corev1.RestartPolicyNever

	manifest, err := json.Marshal(pod)
	if err != nil {
		return "", fmt.Errorf("encoding pod %q: %w", pod.Name, err)
	}

	if _, err := KubectlWithStdin(string(manifest), "create", "-f", "-"); err != nil {
		return "", err
	}
	defer func() {
		_, _ = Kubectl("delete", "pod", pod.Name, "-n", pod.Namespace, "--ignore-not-found")
	}()

	phase, err := waitForPodPhase(pod, timeout)
	logs, _ := Kubectl("logs", pod.Name, "-n", pod.Namespace, "--all-containers")
	if err != nil {
		return logs, err
	}
	if phase != corev1.PodSucceeded {
		return logs, fmt.Errorf("pod %s/%s ended in phase %s:\n%s", pod.Namespace, pod.Name, phase, logs)
	}

	return logs, nil
}

// waitForPodPhase polls the pod until it reaches a terminal phase or timeout
// passes.
func waitForPodPhase(pod *corev1.Pod, timeout time.Duration) (corev1.PodPhase, error) {
	deadline := time.Now().Add(timeout)
	for {
		out, err := Kubectl("get", "pod", pod.Name, "-n", pod.Namespace, "-o", "jsonpath={.status.phase}")
		if err != nil {
			return "", err
		}

		phase := corev1.PodPhase(strings.TrimSpace(out))
		if phase == corev1.PodSucceeded || phase == corev1.PodFailed {
			return phase, nil
		}

		if time.Now().After(deadline) {
			describe, _ := Kubectl("describe", "pod", pod.Name, "-n", pod.Namespace)
			return phase, fmt.Errorf(
				"pod %s/%s did not complete within %s (phase %q):\n%s",
				pod.Namespace, pod.Name, timeout, phase, describe,
			)
		}

		time.Sleep(2 * time.Second)
	}
}
