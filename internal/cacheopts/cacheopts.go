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

// Package cacheopts holds the cache configuration of the manager. The
// operator and the envtest suites build their manager from the same
// configuration, so a suite reads through the informers that the operator
// runs with.
package cacheopts

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// Options returns the cache configuration of the manager.
//
// The kinds below are scoped by labels.ManagedSelector. The operator tracks
// only the Jobs and the pods that it applies itself, and a cache of every
// Job or every pod in the cluster wastes memory on foreign workloads.
//
// A scoped informer holds no object without the label. A controller that
// reads one of these kinds through the cache therefore reads a resource of
// the operator, or it reads nothing.
func Options() cache.Options {
	managed := labels.ManagedSelector()

	return cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&batchv1.Job{}: {Label: managed},
			// A Job does not report the waiting state of its pods, so the
			// pods of these Jobs are what tells a controller that a container
			// cannot start. See pkg/podstate.
			&corev1.Pod{}: {Label: managed},
		},
	}
}
