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

package v1

// The condition vocabulary that both restore kinds report. A reason that only
// one restore kind reports is declared next to that kind, in its types file.
const (
	// ReasonClusterNotSuspended means that the target cluster still runs. A
	// restore rewrites primary storage, so it waits in Pending until the
	// owner of the cluster suspends it. The restore controllers never write
	// spec.suspend.
	ReasonClusterNotSuspended = "ClusterNotSuspended"
)
