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

// Package camundamanagementclusterclaim renders what a
// CamundaManagementClusterClaim writes: the PluginRequests of its
// selections, through pkg/components/claimrequests, and the
// CamundaManagementCluster of the claim, with the fields that the slot
// outputs fill.
//
// The package is pure: spec in, resources out, no API calls. The controller
// in internal/controller/fleet/camundamanagementclusterclaim reads the
// cluster state into Input and applies what this package renders.
package camundamanagementclusterclaim
