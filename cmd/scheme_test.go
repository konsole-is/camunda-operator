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

package main

import (
	"testing"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/keycloak"
)

// The manager reads, watches, and applies every kind below through typed
// objects. A kind the scheme does not recognize fails at runtime only, on a
// Kubernetes cluster that serves it, and no envtest suite catches it: the
// suites build a scheme of their own.
func TestSchemeRecognizesEveryKindTheManagerReconciles(t *testing.T) {
	t.Parallel()

	for _, gvk := range []schema.GroupVersionKind{
		corev1.SchemeGroupVersion.WithKind("Secret"),
		v1.GroupVersion.WithKind("CamundaManagementCluster"),
		esv1.GroupVersion.WithKind("Elasticsearch"),
		monitoringv1.SchemeGroupVersion.WithKind(monitoringv1.PodMonitorsKind),
		keycloak.GroupVersion.WithKind("Keycloak"),
	} {
		assert.True(t, scheme.Recognizes(gvk), "scheme does not recognize %s", gvk)
	}
}
