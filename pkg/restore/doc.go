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

// Package restore holds the code that every restore kind shares: the facts
// read off the live broker StatefulSet, the claim on the cluster, the
// preparation of the cluster, the recreated broker volumes, the restore Jobs,
// and the terminal branch.
//
// The package reads no restore spec. It reads and writes [v1.RestoreProgress],
// which every restore status embeds. It never writes status.phase: a step
// reports an [Outcome], and the controller maps it onto its own phase.
package restore
