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

// Package restore is the shared machinery of every restore kind, in the role
// that pkg/logicalbackup has for the backup pair. It holds the facts a
// restore reads off the live broker StatefulSet, the broker data volumes it
// deletes and creates again, the restore-application Job it runs once per
// broker, the claim it takes on the cluster, and the mid-run grace and the
// terminal transitions that every kind shares.
//
// The restore Jobs never re-render the broker configuration. They copy it
// from the StatefulSet that the CamundaCluster controller applied, which
// still exists while the cluster is suspended. The restore application then
// always runs with the configuration the brokers run with, and the two
// cannot drift.
//
// The package reads no restore CR's spec. It reads and writes
// [v1.RestoreProgress] in place, which every restore status embeds, and it
// never writes status.phase: each kind owns its own phase vocabulary. A
// driver step reports an [Outcome], and the controller maps that outcome onto
// its own phase.
package restore
