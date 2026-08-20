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

// Package restore is the shared machinery of the two restore kinds,
// LogicalRestore and PointInTimeRestore: the facts they read off the live
// broker StatefulSet, the broker data volumes they delete and create again,
// and the restore-application Job they run once per broker.
//
// The restore Jobs never re-render the broker configuration. They copy it
// from the StatefulSet that the CamundaCluster controller applied, which
// still exists while the cluster is suspended. The restore application then
// always runs with the configuration the brokers run with, and the two
// cannot drift.
//
// The package holds no knowledge of either restore CR's spec. It renders and
// applies. The controllers decide.
package restore
