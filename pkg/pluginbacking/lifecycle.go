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

// Package pluginbacking holds the lifecycle rule of the backing objects that
// a built-in plugin handler writes for a PluginRequest.
package pluginbacking

import (
	"github.com/sourcehawk/operator-component-framework/pkg/component"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
)

// Lifecycle is the phase of a request that a backing component serves. The
// zero value serves a request that is not being deleted.
type Lifecycle struct {
	// Deleted is true for a request that is being deleted.
	Deleted bool
	// Policy is the deletion policy of a deleted request.
	Policy pluginsv1alpha1.DeletionPolicy
}

// Deleting returns the lifecycle of a request that is being deleted with
// policy.
func Deleting(policy pluginsv1alpha1.DeletionPolicy) Lifecycle {
	return Lifecycle{Deleted: true, Policy: policy}
}

// Options returns the resource options of one backing object. While the
// request lives, the object is applied, or deleted when drop is true. While
// the request is deleted, the Orphan policy keeps the object and removes the
// owner reference, and any other policy deletes it. A delete never touches an
// object that the request does not own.
func (l Lifecycle) Options(drop bool) []component.ResourceOption {
	switch {
	case !l.Deleted:
		return []component.ResourceOption{component.DeleteWhen(drop), component.DeleteOnlyIfOwned()}
	case l.Policy == pluginsv1alpha1.DeletionPolicyOrphan:
		// OrphanWhen and DeleteWhen cannot be on one resource, even when false.
		return []component.ResourceOption{component.OrphanWhen(true)}
	default:
		return []component.ResourceOption{component.Delete(), component.DeleteOnlyIfOwned()}
	}
}
