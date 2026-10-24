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

package postgresql

import (
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta/testrestmapper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/controller/plugins/builtin/adapter"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

var testServiceAccount = pluginsv1alpha1.ServiceAccountRef{Namespace: "camunda-system", Name: "camunda-operator"}

// requestFor returns a request of the built-in registration with the given
// parameters. "" means no parameters.
func requestFor(registration, parameters string) *pluginsv1alpha1.PluginRequest {
	section := registrationSection(registration)
	request := &pluginsv1alpha1.PluginRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "orders-" + section,
			Namespace:  "my-cluster-ns",
			Generation: 2,
			Labels:     map[string]string{pluginsv1alpha1.LabelRegistration: registration},
		},
		Spec: pluginsv1alpha1.PluginRequestSpec{
			Section:        section,
			Registration:   registration,
			DeletionPolicy: pluginsv1alpha1.DeletionPolicyDelete,
		},
	}
	if parameters != "" {
		request.Spec.Parameters = &apiextensionsv1.JSON{Raw: []byte(parameters)}
	}

	return request
}

// registrationSection returns the first section of the built-in
// registration name.
func registrationSection(name string) string {
	for _, reg := range pluginstartup.Registrations("0.3.0", testServiceAccount) {
		if reg.Name == name {
			return reg.Spec.Sections[0]
		}
	}

	return ""
}

// fakeReader returns a client that holds objs.
func fakeReader(t *testing.T, objs ...client.Object) client.Client {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))
	require.NoError(t, pluginsv1alpha1.AddToScheme(scheme))

	return fake.NewClientBuilder().
		WithScheme(scheme).
		WithRESTMapper(testrestmapper.TestOnlyStaticRESTMapper(scheme)).
		WithObjects(objs...).
		WithStatusSubresource(
			&v1.ObjectStorageConfig{},
			&v1.SecondaryStorageConfig{},
			&v1.DatabaseConfig{},
			&v1.DatabaseServer{},
			&v1.Database{},
		).
		Build()
}

// reconcileOutputs reconciles the outputs component of s against c.
func reconcileOutputs(t *testing.T, s adapter.Served, request *pluginsv1alpha1.PluginRequest, c client.Client) {
	require.NoError(t, s.Outputs.Reconcile(t.Context(), component.ReconcileContext{
		Client: c, Scheme: c.Scheme(), Owner: request,
	}))
}
