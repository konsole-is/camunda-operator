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

package logicalbackup

import (
	"context"

	"k8s.io/apimachinery/pkg/types"
)

// SiblingInProgress reports the name of a backup of the other backup kind
// for the cluster that has started — allocated its identity — and is not
// terminal yet, or the empty string when none has. Pending siblings do not
// block: the in-kind tie-break decides which pending backup starts first, so
// two pending backups of different kinds cannot deadlock each other waiting
// for one another — this mirrors the in-kind rule, where only a started
// backup blocks unconditionally. Each backup controller checks its own kind
// itself; the manager wires this seam between the two kinds. Both
// controllers use this one signature, so the wiring needs no adapters.
type SiblingInProgress func(ctx context.Context, cluster types.NamespacedName) (string, error)
