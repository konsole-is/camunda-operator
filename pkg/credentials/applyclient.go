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

package credentials

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// PreconditionAnnotation names the UID that an apply of a credential Secret
// must find on the server. Password.PreconditionAnnotations writes it, and
// NewApplyClient removes it from the patch and sets metadata.uid from it. The
// annotation is a protocol between the two and never reaches the cluster.
const PreconditionAnnotation = "credentials.camunda.io/expected-uid"

// NewApplyClient wraps c so that a Server-Side Apply of a Secret that carries
// PreconditionAnnotation is applied with metadata.uid set to the annotated
// value. Every other call passes through unchanged.
//
// The API server accepts such a patch only when the named object is still on
// the server. If it was deleted, the apply is rejected with a conflict and
// nothing is created.
//
// A controller that publishes a credential Secret must place this wrapper in
// the Client of its ocf ReconcileContext.
func NewApplyClient(c client.Client) client.Client {
	return &applyClient{Client: c}
}

type applyClient struct {
	client.Client
}

// Patch turns PreconditionAnnotation on a Server-Side Apply of a Secret into
// the metadata.uid precondition of the patch. All other patches pass through.
func (c *applyClient) Patch(
	ctx context.Context,
	obj client.Object,
	patch client.Patch,
	opts ...client.PatchOption,
) error {
	secret, ok := obj.(*corev1.Secret)
	if !ok || patch != client.Apply { //nolint:staticcheck // ocf applies through the deprecated client.Apply patch
		return c.Client.Patch(ctx, obj, patch, opts...)
	}

	uid, ok := secret.Annotations[PreconditionAnnotation]
	if !ok {
		return c.Client.Patch(ctx, obj, patch, opts...)
	}

	secret.Annotations = withoutPrecondition(secret.Annotations)
	secret.UID = types.UID(uid)

	return c.Client.Patch(ctx, obj, patch, opts...)
}

// withoutPrecondition returns a copy of annotations without
// PreconditionAnnotation, or nil when nothing else is left. It copies rather
// than deletes in place, because the caller of the apply keeps the map it
// built.
func withoutPrecondition(annotations map[string]string) map[string]string {
	kept := make(map[string]string, len(annotations)-1)
	for key, value := range annotations {
		if key != PreconditionAnnotation {
			kept[key] = value
		}
	}

	if len(kept) == 0 {
		return nil
	}

	return kept
}
