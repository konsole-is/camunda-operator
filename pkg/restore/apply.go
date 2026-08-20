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

package restore

import (
	"context"

	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FieldManagerPointInTimeRestore owns every Job and every recreated volume
// that a PointInTimeRestore applies, so the fields a restore owns are visible
// on the object. Each restore kind has a field manager of its own.
const FieldManagerPointInTimeRestore client.FieldOwner = "camunda-operator/pointintimerestore"

// Apply server-side applies obj under manager, forcing ownership of every
// field the operator sets. Every resource a restore manages goes through it,
// so the field manager of a restore is one string in one place.
func Apply(ctx context.Context, c client.Client, obj client.Object, manager client.FieldOwner) error {
	//nolint:staticcheck // ocf applies through the deprecated client.Apply patch
	return c.Patch(ctx, obj, client.Apply, manager, client.ForceOwnership)
}
