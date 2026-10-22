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

package claimrequests

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	pluginsv1alpha1 "github.com/konsole-is/camunda-operator/api/plugins/v1alpha1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// DeletionPolicyFieldManager returns the field manager under which a claim
// writes the deletion policy on its requests. prefix is the field manager
// prefix of the claim kind, such as
// fleetv1alpha1.FieldManagerPrefixClusterClaim.
func DeletionPolicyFieldManager(prefix string, uid types.UID) string {
	return prefix + "deletionpolicy/" + string(uid)
}

// DeletionPolicyApply returns the server-side apply object that sets
// spec.deletionPolicy of request to policy. It holds no other field, so the
// apply owns only that one. It carries the UID of request, so the API server
// refuses it on an object that replaced request under its name.
func DeletionPolicyApply(
	request *pluginsv1alpha1.PluginRequest,
	policy pluginsv1alpha1.DeletionPolicy,
) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": pluginsv1alpha1.GroupVersion.String(),
		"kind":       "PluginRequest",
		"metadata": map[string]any{
			"name":      request.Name,
			"namespace": request.Namespace,
			"uid":       string(request.UID),
		},
		"spec": map[string]any{"deletionPolicy": string(policy)},
	}}
}

// WaitingMessage returns the Ready message of a deleted claim that waits for
// the objects remaining, each given as "<Kind> <name>". The message fits
// conditions.MaxMessageLength.
func WaitingMessage(remaining []string) string {
	return conditions.BoundMessage(
		"The claim is deleted after these objects are gone: " + strings.Join(remaining, ", "),
	)
}
