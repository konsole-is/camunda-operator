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

// Package adapter holds the contract between the built-in controller and the
// handler of each built-in registration, and the helpers that more than one
// handler uses.
package adapter

import (
	"slices"

	"github.com/sourcehawk/operator-component-framework/pkg/component"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// The kinds of the contracts that the handlers read.
const (
	KindObjectStorageConfig    = "ObjectStorageConfig"
	KindSecondaryStorageConfig = "SecondaryStorageConfig"
	KindDatabaseConfig         = "DatabaseConfig"
)

// The kinds of the backing resources of database-server and
// management-database-server.
const (
	KindDatabaseServer = "DatabaseServer"
	KindDatabase       = "Database"
)

// KindElasticsearchCluster is the kind of the backing object of elasticsearch.
const KindElasticsearchCluster = "ElasticsearchCluster"

// KindDatabaseServerPreset is the kind of the preset that a database-server
// request reads.
const KindDatabaseServerPreset = "DatabaseServerPreset"

// Handler serves the requests of one built-in registration. It reads no
// cluster state: the controller gives it the live backing objects of the
// request, and reconciles what it returns.
type Handler interface {
	// Build returns the components that serve request. Parameters that
	// break a rule of the registration that its schema cannot state return
	// a *conditions.PreCheckFailure with reason InvalidParameters.
	Build(request *pluginsv1alpha1.PluginRequest, live LiveObjects) (Served, error)
	// Cleanup returns what applies policy to the resources that Build
	// writes, for a request that is being deleted.
	Cleanup(
		request *pluginsv1alpha1.PluginRequest,
		policy pluginsv1alpha1.DeletionPolicy,
		live LiveObjects,
	) (Cleanup, error)
	// Contracts returns the contracts that request reads, in its own
	// namespace. Parameters that do not parse read none.
	Contracts(request *pluginsv1alpha1.PluginRequest) []ContractRef
	// Backing returns the objects that Build reads or writes for request.
	// The controller reads them before it calls Build and Cleanup.
	// Parameters that do not parse return the objects whose names do not
	// depend on them.
	Backing(request *pluginsv1alpha1.PluginRequest) []BackingRef
}

// BackingRef is one backing object of a request.
type BackingRef struct {
	Kind string // a Kind constant of the handler
	// Object is an empty typed object with its name, and its namespace
	// unless the kind is cluster-scoped.
	Object client.Object
}

// LiveObjects are the backing objects of one request as the API server
// holds them before the pass, by kind and name. An object that does not
// exist is absent.
type LiveObjects map[LiveKey]client.Object

// LiveAs returns the live object of kind and name as T. It reports false
// when the object is absent or is not a T.
func LiveAs[T client.Object](live LiveObjects, kind, name string) (T, bool) {
	obj, ok := live[LiveKey{Kind: kind, Name: name}].(T)
	return obj, ok
}

// LiveKey identifies one object of LiveObjects.
type LiveKey struct{ Kind, Name string }

// Served is what serves one request.
type Served struct {
	// Outputs has condition type OutputsReady. It reads the contracts that
	// the request publishes.
	Outputs *component.Component
	// Backing has condition type Ready and writes the backing resources.
	// It is nil when the handler writes nothing, and Ready then follows
	// OutputsReady.
	Backing *component.Component
	// Report returns the status fields of the request after the components
	// reconciled.
	Report func() Report
}

// Readiness returns the component that Ready follows.
func (s Served) Readiness() *component.Component {
	if s.Backing != nil {
		return s.Backing
	}

	return s.Outputs
}

// Report holds the status fields that a handler fills.
type Report struct {
	Outputs     map[string]string
	Summary     []pluginsv1alpha1.SummaryItem
	Resources   []pluginsv1alpha1.ResourceRef
	PodSelector *metav1.LabelSelector
}

// Cleanup applies a deletion policy. The finalizer stays until every
// Remaining object that the request owns is gone. A zero Cleanup removes the
// finalizer at once.
type Cleanup struct {
	// Backing applies the policy to the resources that Build writes. It is
	// nil when the handler writes nothing.
	Backing *component.Component
	// Remaining are the objects that the finalizer waits for.
	Remaining []client.Object
}

// ContractRef is one contract that a request reads and publishes.
type ContractRef struct {
	Output string // the output key, a pluginstartup.Output* constant
	Kind   string // KindObjectStorageConfig, KindSecondaryStorageConfig, or KindDatabaseConfig
	Name   string
}

// BackingLabels returns the labels of each object that a built-in handler
// writes for request.
func BackingLabels(request *pluginsv1alpha1.PluginRequest) map[string]string {
	return map[string]string{
		pluginsv1alpha1.LabelRegistration: request.Spec.Registration,
		pluginsv1alpha1.LabelRequest:      request.Name,
		labels.ManagedByKey:               labels.ManagedBy,
	}
}

// OwnedBy reports whether obj has an owner reference to uid.
func OwnedBy(obj client.Object, uid types.UID) bool {
	return slices.ContainsFunc(obj.GetOwnerReferences(), func(ref metav1.OwnerReference) bool {
		return ref.UID == uid
	})
}
