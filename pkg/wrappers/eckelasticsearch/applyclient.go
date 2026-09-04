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

package eckelasticsearch

import (
	"context"
	"fmt"

	esv1 "github.com/elastic/cloud-on-k8s/v3/pkg/apis/elasticsearch/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type applyClient struct {
	client.Client
}

// NewApplyClient wraps c so that Server-Side Apply patches of typed
// Elasticsearch objects are serialized without the fields that the ECK CRD
// does not accept. The wrapper converts such a patch to sanitized
// unstructured content, applies it, and decodes the server response back into
// the typed object. Every other call passes through unchanged.
//
// A controller that reconciles the Elasticsearch Resource through an ocf
// component must place this wrapper in the Client of the ReconcileContext.
func NewApplyClient(c client.Client) client.Client {
	return &applyClient{Client: c}
}

// Patch converts Server-Side Apply patches of *esv1.Elasticsearch to
// sanitized unstructured content and decodes the server response back into
// obj. All other patches pass through.
func (c *applyClient) Patch(
	ctx context.Context,
	obj client.Object,
	patch client.Patch,
	opts ...client.PatchOption,
) error {
	es, ok := obj.(*esv1.Elasticsearch)
	if !ok || patch != client.Apply { //nolint:staticcheck // ocf applies through the deprecated client.Apply patch
		return c.Client.Patch(ctx, obj, patch, opts...)
	}

	u, err := sanitizeForApply(es)
	if err != nil {
		return err
	}

	if err := c.Client.Patch(ctx, u, patch, opts...); err != nil {
		return err
	}

	var applied esv1.Elasticsearch
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &applied); err != nil {
		return fmt.Errorf("decoding applied Elasticsearch %q: %w", es.Name, err)
	}
	*es = applied

	return nil
}

// sanitizeForApply converts es to unstructured content that carries only the
// fields that the ECK CRD schema declares. It removes the top-level status and
// the status and creationTimestamp of every volumeClaimTemplate, which the
// typed structs always serialize as zero values.
func sanitizeForApply(es *esv1.Elasticsearch) (*unstructured.Unstructured, error) {
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(es)
	if err != nil {
		return nil, fmt.Errorf("converting Elasticsearch %q: %w", es.Name, err)
	}

	unstructured.RemoveNestedField(raw, "status")
	unstructured.RemoveNestedField(raw, "metadata", "creationTimestamp")

	nodeSets, found, err := unstructured.NestedSlice(raw, "spec", "nodeSets")
	if err != nil || !found {
		return &unstructured.Unstructured{Object: raw}, err
	}

	for _, nodeSet := range nodeSets {
		nodeSetMap, ok := nodeSet.(map[string]any)
		if !ok {
			continue
		}
		claims, ok := nodeSetMap["volumeClaimTemplates"].([]any)
		if !ok {
			continue
		}
		for _, claim := range claims {
			claimMap, ok := claim.(map[string]any)
			if !ok {
				continue
			}
			delete(claimMap, "status")
			if metadata, ok := claimMap["metadata"].(map[string]any); ok {
				delete(metadata, "creationTimestamp")
			}
		}
	}

	if err := unstructured.SetNestedSlice(raw, nodeSets, "spec", "nodeSets"); err != nil {
		return nil, fmt.Errorf("sanitizing Elasticsearch %q node sets: %w", es.Name, err)
	}

	return &unstructured.Unstructured{Object: raw}, nil
}
