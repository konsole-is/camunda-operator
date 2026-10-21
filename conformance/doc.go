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

// Package conformance checks that a plugin controller keeps the contract of
// plugins.camunda.io. A plugin repository calls Run from its own tests,
// against a cluster where the plugin chart, the CRDs of the group, the grant
// admission policy, and the operator manager are installed. Each check has a
// stable ID, and the report, a skip, and the docs use the same ID. A plugin
// in another language runs the binary plugin-conformance instead.
package conformance
