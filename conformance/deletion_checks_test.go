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

package conformance

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

func route(t *testing.T, apiVersion, kind, spec string) *unstructured.Unstructured {
	t.Helper()

	obj := &unstructured.Unstructured{}
	require.NoError(t, obj.UnmarshalJSON([]byte(
		`{"apiVersion":"`+apiVersion+`","kind":"`+kind+`","metadata":{"name":"r","namespace":"ns"},"spec":`+spec+`}`,
	)))
	return obj
}

func TestRouteBackends(t *testing.T) {
	tests := []struct {
		name  string
		route *unstructured.Unstructured
		want  []types.NamespacedName
	}{
		{
			name: "an Ingress with a default backend and two paths to one Service",
			route: route(t, "networking.k8s.io/v1", "Ingress", `{
				"defaultBackend":{"service":{"name":"web","port":{"number":80}}},
				"rules":[{"http":{"paths":[
					{"path":"/a","backend":{"service":{"name":"web"}}},
					{"path":"/b","backend":{"service":{"name":"web"}}}]}}]}`),
			want: []types.NamespacedName{{Namespace: "ns", Name: "web"}},
		},
		{
			name: "an HTTPRoute with a backend in another namespace",
			route: route(t, "gateway.networking.k8s.io/v1", "HTTPRoute", `{"rules":[{"backendRefs":[
				{"name":"api","namespace":"other","port":80},
				{"name":"local","group":"","kind":"Service"}]}]}`),
			want: []types.NamespacedName{{Namespace: "other", Name: "api"}, {Namespace: "ns", Name: "local"}},
		},
		{
			name: "a backend of another kind",
			route: route(t, "gateway.networking.k8s.io/v1", "GRPCRoute", `{"rules":[{"backendRefs":[
				{"name":"imported","group":"multicluster.x-k8s.io","kind":"ServiceImport"}]}]}`),
		},
		{
			name:  "a route without backends",
			route: route(t, "gateway.networking.k8s.io/v1alpha2", "TCPRoute", `{"rules":[{}]}`),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, routeBackends(tt.route))
		})
	}
}

func TestIsRouteKnowsTheIngressAndTheGatewayRoutes(t *testing.T) {
	for _, kind := range []string{"HTTPRoute", "GRPCRoute", "TLSRoute", "TCPRoute"} {
		assert.True(t, isRoute(route(t, "gateway.networking.k8s.io/v1", kind, `{}`)), kind)
	}
	assert.True(t, isRoute(route(t, "networking.k8s.io/v1", "Ingress", `{}`)))
	assert.False(t, isRoute(route(t, "v1", "Service", `{}`)))
	assert.False(t, isRoute(route(t, "gateway.networking.k8s.io/v1", "Gateway", `{}`)))
}

func TestOrderVerdict(t *testing.T) {
	registration := &pluginsv1alpha1.PluginRegistration{}
	registration.Kind = "PluginRegistration"
	registration.Name = "echo-1"
	request := testRequest()
	request.Kind = kindPluginRequest

	at := func(obj interface{ SetResourceVersion(string) }, rv string) {
		obj.SetResourceVersion(rv)
	}

	tests := []struct {
		name    string
		events  func(r *recorder)
		message string
	}{
		{
			name: "the registration is Ready before the request is served",
			events: func(r *recorder) {
				at(registration, "10")
				r.observe(registration, registrationReadyKey)
				at(request, "11")
				r.observe(request, requestServedKey)
			},
		},
		{
			name: "the request is served before the registration is Ready",
			events: func(r *recorder) {
				at(registration, "12")
				r.observe(registration, registrationReadyKey)
				at(request, "11")
				r.observe(request, requestServedKey)
			},
			message: "the plugin served the request before it wrote the registration with Ready=True",
		},
		{
			name: "the request is never served",
			events: func(r *recorder) {
				at(registration, "10")
				r.observe(registration, registrationReadyKey)
			},
			message: "the watch did not see the request get the finalizer",
		},
		{
			name: "a failed watch",
			events: func(r *recorder) {
				r.fail(errors.New("a watch failed: gone"))
			},
			message: "a watch failed: gone",
		},
		{
			name: "the registration is never Ready",
			events: func(r *recorder) {
				at(request, "11")
				r.observe(request, requestServedKey)
			},
			message: "the plugin did not write the registration echo-1 again with Ready=True",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &recorder{}
			tt.events(r)

			err := orderVerdict(r, "echo-1")

			if tt.message == "" {
				assert.NoError(t, err)
				return
			}
			assert.ErrorContains(t, err, tt.message)
		})
	}
}

func TestOrphanProblems(t *testing.T) {
	req := testRequest()
	kept := pluginsv1alpha1.ResourceRef{Kind: "ConfigMap", Name: "x", OnOrphan: pluginsv1alpha1.OnOrphanKept}
	deleted := pluginsv1alpha1.ResourceRef{Kind: "Ingress", Name: "x", OnOrphan: pluginsv1alpha1.OnOrphanDeleted}
	keptClusterScoped := pluginsv1alpha1.ResourceRef{
		Kind:     "StorageClass",
		Name:     "x",
		OnOrphan: pluginsv1alpha1.OnOrphanKept,
	}

	tests := []struct {
		name    string
		row     pluginsv1alpha1.ResourceRef
		current *unstructured.Unstructured
		message string
	}{
		{name: "a kept object without the owner reference", row: kept,
			current: written("ConfigMap", "ns", "x", writerLabels())},
		{name: "a kept cluster-scoped object is checked by its labels only", row: keptClusterScoped,
			current: written("StorageClass", "", "x", writerLabels(), ownerOf(req))},
		{name: "a kept cluster-scoped object without the registration label", row: keptClusterScoped,
			current: written("StorageClass", "", "x", map[string]string{pluginsv1alpha1.LabelRequest: "basic-delete"}),
			message: "StorageClass x lost the label plugins.camunda.io/registration"},
		{name: "a deleted object that is gone", row: deleted},
		{name: "a kept object that is gone", row: kept, message: "ConfigMap ns/x has onOrphan Kept and is gone"},
		{name: "a kept object with the owner reference", row: kept,
			current: written("ConfigMap", "ns", "x", writerLabels(), ownerOf(req)),
			message: "ConfigMap ns/x still has an owner reference to the request"},
		{name: "a kept object without the request label", row: kept,
			current: written("ConfigMap", "ns", "x", map[string]string{pluginsv1alpha1.LabelRegistration: "echo-1"}),
			message: "ConfigMap ns/x lost the label plugins.camunda.io/request"},
		{name: "a kept object that was written again", row: kept,
			current: withUID(written("ConfigMap", "ns", "x", writerLabels()), "new-uid"),
			message: "ConfigMap ns/x was deleted and written again"},
		{name: "a kept object that is being deleted", row: kept,
			current: deleting(written("ConfigMap", "ns", "x", writerLabels())),
			message: "ConfigMap ns/x is being deleted"},
		{name: "a deleted object that stays", row: deleted,
			current: written("Ingress", "ns", "x", writerLabels()),
			message: "Ingress ns/x has onOrphan Deleted and still exists"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := "ns"
			if tt.row.Kind == "StorageClass" {
				namespace = ""
			}
			before := written(tt.row.Kind, namespace, "x", writerLabels())

			problems := orphanProblems(tt.row, before, tt.current, req)

			if tt.message == "" {
				assert.Empty(t, problems)
				return
			}
			require.Len(t, problems, 1)
			assert.Contains(t, problems[0], tt.message)
		})
	}
}

func withUID(obj *unstructured.Unstructured, uid types.UID) *unstructured.Unstructured {
	obj.SetUID(uid)
	return obj
}

func deleting(obj *unstructured.Unstructured) *unstructured.Unstructured {
	now := metav1.Now()
	obj.SetDeletionTimestamp(&now)
	return obj
}

func TestDeleteRequestKeepsARequestThatReplacedTheOneOfTheRun(t *testing.T) {
	replacement := &pluginsv1alpha1.PluginRequest{}
	replacement.Namespace = "conformance-echo"
	replacement.Name = "basic-delete"
	replacement.UID = "replacement-uid"

	// The fake client ignores a UID precondition, so the interceptor
	// answers as the API server does.
	funcs := interceptor.Funcs{
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			var del client.DeleteOptions
			del.ApplyOptions(opts)

			var current pluginsv1alpha1.PluginRequest
			if err := c.Get(ctx, client.ObjectKeyFromObject(obj), &current); err != nil {
				return err
			}
			if del.Preconditions != nil && *del.Preconditions.UID != current.UID {
				return apierrors.NewConflict(schema.GroupResource{}, obj.GetName(), errors.New("UID differs"))
			}

			return c.Delete(ctx, obj, opts...)
		},
	}
	s := newTestSuite(t, validConfig(), funcs, replacement)

	err := deleteRequest(t.Context(), s, "basic-delete", "run-uid")

	assert.ErrorContains(t, err, "is not the request that the run created")
	var kept pluginsv1alpha1.PluginRequest
	assert.NoError(t, s.client.Get(t.Context(), client.ObjectKeyFromObject(replacement), &kept))
}

func TestIsKindOfThePluginTellsAnObjectFromACloudResource(t *testing.T) {
	kinds := []kindInfo{{gvk: schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, namespaced: true}}

	assert.True(t, isKindOfThePlugin("ConfigMap", kinds))
	assert.False(t, isKindOfThePlugin("S3Bucket", kinds))
}
