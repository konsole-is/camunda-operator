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

package logicalbackuprdbms

import (
	"context"
	"fmt"
	"os"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// namespaceFile is where the ServiceAccount admission controller mounts the
// namespace of the pod.
const namespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// managerContainer is the container of the manager Deployment whose image is
// the operator's own.
const managerContainer = "manager"

// operatorImage returns the image the upload container runs: the explicit
// --operator-image when set, or the image of the operator's own Pod. The pod
// spec is immutable, so one successful resolution is cached for the process
// lifetime; a failed one is retried, which sync.Once could not do. The mutex
// makes the cache safe under concurrent reconciles. Reading the running pod
// means a chart or kustomize overlay cannot drift from the deployed image,
// because nothing templates the image a second time.
func (r *LogicalBackupRDBMSReconciler) operatorImage(ctx context.Context) (string, error) {
	r.imageMu.Lock()
	defer r.imageMu.Unlock()
	if r.OperatorImage != "" {
		return r.OperatorImage, nil
	}

	image, err := ResolveOperatorImage(ctx, r.APIReader, os.Hostname, os.ReadFile)
	if err != nil {
		return "", err
	}
	r.OperatorImage = image

	return image, nil
}

// ResolveOperatorImage reads the image of the manager container from the
// operator's own Pod, located by the hostname (the pod name) and the mounted
// ServiceAccount namespace.
func ResolveOperatorImage(
	ctx context.Context,
	reader client.Reader,
	hostname func() (string, error),
	readFile func(string) ([]byte, error),
) (string, error) {
	name, err := hostname()
	if err != nil {
		return "", fmt.Errorf("reading the pod name: %w", err)
	}

	namespace, err := readFile(namespaceFile)
	if err != nil {
		return "", fmt.Errorf("reading the pod namespace: %w", err)
	}

	var pod corev1.Pod
	key := types.NamespacedName{Namespace: strings.TrimSpace(string(namespace)), Name: name}
	if err := reader.Get(ctx, key, &pod); err != nil {
		return "", fmt.Errorf("reading the own Pod %s: %w", key, err)
	}

	for _, container := range pod.Spec.Containers {
		if container.Name == managerContainer {
			return container.Image, nil
		}
	}

	return "", fmt.Errorf("no container %q in Pod %s", managerContainer, key)
}
