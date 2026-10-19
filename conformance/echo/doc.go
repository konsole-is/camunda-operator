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

// Package echo is a small plugin controller that serves one
// PluginRegistration. The conformance suite runs it to prove its own checks,
// and a test of the fleet manager can build a plugin image from it.
//
// For each request, the controller copies inputs to outputs and writes a
// ConfigMap and a StorageClass. When it serves the access section, it also
// writes an Ingress. It keeps the promise of both deletion policies. A
// Defect in the Options makes it break one rule of the contract.
package echo
