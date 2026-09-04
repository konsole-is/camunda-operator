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

package keycloak

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type applyClient struct {
	client.Client
}

// NewApplyClient wraps c so that Server-Side Apply patches of typed Keycloak
// objects are serialized without the zero values that the Keycloak CRD schema
// refuses. The wrapper converts such a patch to sanitized unstructured
// content, applies it, and decodes the server response back into the typed
// object. Every other call passes through unchanged.
//
// A controller that reconciles the Keycloak Resource through an ocf component
// must place this wrapper in the Client of the ReconcileContext.
func NewApplyClient(c client.Client) client.Client {
	return &applyClient{Client: c}
}

// Patch converts Server-Side Apply patches of *Keycloak to sanitized
// unstructured content and decodes the server response back into obj. All
// other patches pass through.
func (c *applyClient) Patch(
	ctx context.Context,
	obj client.Object,
	patch client.Patch,
	opts ...client.PatchOption,
) error {
	kc, ok := obj.(*Keycloak)
	if !ok || patch != client.Apply { //nolint:staticcheck // ocf applies through the deprecated client.Apply patch
		return c.Client.Patch(ctx, obj, patch, opts...)
	}

	u, err := sanitizeForApply(kc)
	if err != nil {
		return err
	}

	if err := c.Client.Patch(ctx, u, patch, opts...); err != nil {
		return err
	}

	var applied Keycloak
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &applied); err != nil {
		return fmt.Errorf("decoding applied Keycloak %q: %w", kc.Name, err)
	}
	*kc = applied

	return nil
}

// sanitizeForApply converts kc to unstructured content that the Keycloak CRD
// schema accepts. It removes the status, which is a subresource, every
// creationTimestamp that the typed structs serialize as a zero value, and the
// null container list of an empty pod template.
func sanitizeForApply(kc *Keycloak) (*unstructured.Unstructured, error) {
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(kc)
	if err != nil {
		return nil, fmt.Errorf("converting Keycloak %q: %w", kc.Name, err)
	}

	unstructured.RemoveNestedField(raw, "status")
	unstructured.RemoveNestedField(raw, "metadata", "creationTimestamp")
	unstructured.RemoveNestedField(
		raw, "spec", "unsupported", "podTemplate", "metadata", "creationTimestamp",
	)
	removeNil(raw, "spec", "unsupported", "podTemplate", "spec", "containers")

	return &unstructured.Unstructured{Object: raw}, nil
}

// removeNil deletes the field at path when it holds no value. A pod template
// that names no container still serializes "containers: null", and the schema
// declares an array there, so the null has to go; a template that does name a
// container keeps its list.
func removeNil(content map[string]any, path ...string) {
	value, found, err := unstructured.NestedFieldNoCopy(content, path...)
	if err != nil || !found || value != nil {
		return
	}
	unstructured.RemoveNestedField(content, path...)
}
