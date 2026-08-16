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

// Package camundacluster renders the resources that a CamundaCluster CR
// publishes. It merges the preset into the spec, validates the merged spec,
// resolves the documented defaults, and assembles the ocf components of the
// processes. Everything here is pure: spec in, resources out, no API calls.
// The controller in internal/controller/camundacluster drives it.
package camundacluster
