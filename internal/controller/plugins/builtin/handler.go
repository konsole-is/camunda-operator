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

package builtin

import (
	"maps"
	"slices"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/internal/pluginstartup"
)

// handler serves the requests of one built-in registration. It reads no
// cluster state: the controller gives it the live backing objects of the
// request, and reconciles what it returns.
type handler interface {
	// build returns the components that serve request. Parameters that
	// break a rule of the registration that its schema cannot state return
	// a *conditions.PreCheckFailure with reason InvalidParameters.
	build(request *pluginsv1alpha1.PluginRequest, live liveObjects) (served, error)
	// cleanup returns what applies policy to the resources that build
	// writes, for a request that is being deleted.
	cleanup(
		request *pluginsv1alpha1.PluginRequest,
		policy pluginsv1alpha1.DeletionPolicy,
		live liveObjects,
	) (cleanup, error)
	// contracts returns the contracts that request reads, in its own
	// namespace. Parameters that do not parse read none.
	contracts(request *pluginsv1alpha1.PluginRequest) []contractRef
	// backing returns the objects that build reads or writes for request.
	// The controller reads them before it calls build and cleanup.
	// Parameters that do not parse return the objects whose names do not
	// depend on them.
	backing(request *pluginsv1alpha1.PluginRequest) []backingRef
}

// backingRef is one backing object of a request.
type backingRef struct {
	kind string // a kind* constant of the handler
	// object is an empty typed object with its name, and its namespace
	// unless the kind is cluster-scoped.
	object client.Object
}

// liveObjects are the backing objects of one request as the API server
// holds them before the pass, by kind and name. An object that does not
// exist is absent.
type liveObjects map[liveKey]client.Object

// liveAs returns the live object of kind and name as T. It reports false
// when the object is absent or is not a T.
func liveAs[T client.Object](live liveObjects, kind, name string) (T, bool) {
	obj, ok := live[liveKey{kind: kind, name: name}].(T)
	return obj, ok
}

type liveKey struct{ kind, name string }

// served is what serves one request.
type served struct {
	// outputs has condition type OutputsReady. It reads the contracts that
	// the request publishes.
	outputs *component.Component
	// backing has condition type Ready and writes the backing resources.
	// It is nil when the handler writes nothing, and Ready then follows
	// OutputsReady.
	backing *component.Component
	// report returns the status fields of the request after the components
	// reconciled.
	report func() report
}

// readiness returns the component that Ready follows.
func (s served) readiness() *component.Component {
	if s.backing != nil {
		return s.backing
	}

	return s.outputs
}

// report holds the status fields that a handler fills.
type report struct {
	outputs     map[string]string
	summary     []pluginsv1alpha1.SummaryItem
	resources   []pluginsv1alpha1.ResourceRef
	podSelector *metav1.LabelSelector
}

// cleanup applies a deletion policy.
type cleanup struct {
	// components apply the policy to the resources of the request.
	components []*component.Component
	// remaining are the objects that must be gone before the finalizer goes.
	remaining []client.Object
}

// contractRef is one contract that a request reads and publishes.
type contractRef struct {
	output string // the output key, a pluginstartup.Output* constant
	kind   string // kindObjectStorageConfig, kindSecondaryStorageConfig, or kindDatabaseConfig
	name   string
}

// Served returns the sorted names of the built-in registrations whose
// requests the controller serves.
func Served() []string {
	return slices.Sorted(maps.Keys(handlers()))
}

// handlers returns the handler of each built-in registration that the
// controller serves, by registration name.
func handlers() map[string]handler {
	return map[string]handler{
		pluginstartup.RegistrationExistingObjectStorage: existing{
			parse: oneContract(pluginstartup.OutputObjectStorageConfig, kindObjectStorageConfig),
		},
		pluginstartup.RegistrationExistingSecondaryStorage: existing{
			parse: oneContract(pluginstartup.OutputSecondaryStorageConfig, kindSecondaryStorageConfig),
		},
		pluginstartup.RegistrationExistingDatabaseConfigs: existing{parse: databaseConfigs},
	}
}
