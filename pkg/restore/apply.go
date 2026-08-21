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

// Each restore kind owns every Job and every recreated volume it applies
// under a field manager of its own, so the fields a restore owns are visible
// on the object.
const (
	// FieldManagerLogicalRestoreElasticsearch owns what a
	// LogicalRestoreElasticsearch applies.
	FieldManagerLogicalRestoreElasticsearch client.FieldOwner = "camunda-operator/logicalrestoreelasticsearch"
	// FieldManagerLogicalRestoreRDBMS owns what a LogicalRestoreRDBMS applies.
	FieldManagerLogicalRestoreRDBMS client.FieldOwner = "camunda-operator/logicalrestorerdbms"
	// FieldManagerPointInTimeRestore owns what a PointInTimeRestore applies.
	FieldManagerPointInTimeRestore client.FieldOwner = "camunda-operator/pointintimerestore"
)

// Apply server-side applies obj under manager, forcing ownership of every
// field the operator sets. The recreated broker volumes are what goes through
// it. A restore Job does not: its pod template is immutable after creation,
// so every Job is created rather than applied. A Create carries the same
// field manager, which keeps one string per restore kind on both.
func Apply(ctx context.Context, c client.Client, obj client.Object, manager client.FieldOwner) error {
	//nolint:staticcheck // ocf applies through the deprecated client.Apply patch
	return c.Patch(ctx, obj, client.Apply, manager, client.ForceOwnership)
}
