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

// Package claimrequests renders the PluginRequests of a claim: one request
// for each selection of the claim, with its labels, its target, its resolved
// inputs, and its parameters. It also builds status.requests of the claim,
// the deletion policy write and the Ready message of a deleted claim, and
// the checks and filters that both claim controllers apply before a build.
// The cluster claim and the management cluster claim both use it, so the
// claim comes in as a client.Object and the kinds come in as values.
//
// The package is pure: spec in, resources out, no API calls.
package claimrequests
