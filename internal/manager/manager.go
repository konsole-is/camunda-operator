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

// Package manager holds the scheme and the cache configuration of the
// manager. The operator and the envtest suites build their manager from this
// package, so a suite knows the same kinds and reads through the same
// informers as the operator.
package manager

import (
	cnpgv1 "github.com/cloudnative-pg/api/pkg/api/v1"
	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	batchv1 "k8s.io/api/batch/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/barmanobjectstore"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/keycloak"
)

// AddToScheme registers in s every kind that the manager reads, watches, or
// applies: the built-in Kubernetes kinds, the kinds of this operator, and the
// kinds of ECK, the Prometheus Operator, the Keycloak Operator,
// CloudNativePG, and the Barman Cloud plugin.
func AddToScheme(s *runtime.Scheme) error {
	builder := runtime.NewSchemeBuilder(
		clientgoscheme.AddToScheme,
		v1.AddToScheme,
		esv1.AddToScheme,
		monitoringv1.AddToScheme,
		keycloak.AddToScheme,
		cnpgv1.AddToScheme,
		barmanobjectstore.AddToScheme,
	)

	return builder.AddToScheme(s)
}

// CacheOptions returns the cache configuration of the manager.
//
// The kinds below are scoped by labels.ManagedSelector. The operator tracks
// only the Jobs and the pods that it applies itself, and a cache of every
// Job or every pod in the cluster wastes memory on foreign workloads.
//
// A scoped informer holds no object without the label. A controller that
// reads one of these kinds through the cache therefore reads a resource of
// the operator, or it reads nothing.
func CacheOptions() cache.Options {
	managed := labels.ManagedSelector()

	return cache.Options{
		ByObject: map[client.Object]cache.ByObject{
			&batchv1.Job{}: {Label: managed},
			// A Job does not report the waiting state of its pods, so the
			// pods of these Jobs are what tells a controller that a container
			// cannot start. See pkg/podstate.
			&corev1.Pod{}: {Label: managed},
			// The claim Leases of the operator carry the label. The leader
			// election Leases of every operator in the cluster do not, and a
			// cache of those is memory spent on nothing.
			&coordinationv1.Lease{}: {Label: managed},
		},
	}
}
