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

// Package camundaclusterclaim renders what a CamundaClusterClaim writes: the
// PluginRequests of its selections, through pkg/components/claimrequests,
// the CamundaCluster of the claim, with the fields that the slot outputs
// fill, and the attachments: the CamundaOptimize and the BackupSchedule of
// spec.attachments.
//
// The package is pure: spec in, resources out, no API calls. The controller in
// internal/controller/fleet/camundaclusterclaim reads the cluster state into
// Input and applies what this package renders.
package camundaclusterclaim
