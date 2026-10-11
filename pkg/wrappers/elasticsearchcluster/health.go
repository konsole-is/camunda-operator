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

package elasticsearchcluster

import (
	"github.com/sourcehawk/operator-component-framework/pkg/component/concepts"

	apiv1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/internal/readycondition"
)

// The handlers in this file and in suspension.go are the kind-specific status
// logic NewBuilder registers by default; they replace the scaffolded defaults
// the ocf generator writes into builder.go. After regenerating the wrapper
// with --force, delete the scaffolded Default* handlers from builder.go again
// so these implementations take their place.

// DefaultConvergingStatusHandler maps the Ready condition of the
// ElasticsearchCluster to the component convergence state. Ready=True is
// Healthy, unless its reason is Suspended. Ready=False with the reason
// Failing, Down, Degraded, or Error is Failing. Every other state, also a
// Ready of an older generation, is Creating on the first apply and Updating
// afterwards.
func DefaultConvergingStatusHandler(
	op concepts.ConvergingOperation, esc *apiv1.ElasticsearchCluster,
) (concepts.AliveStatusWithReason, error) {
	return readycondition.Converging(op, observed(esc)), nil
}

// observed is the part of the ElasticsearchCluster that the handlers read.
func observed(esc *apiv1.ElasticsearchCluster) readycondition.Object {
	return readycondition.Object{
		Kind:               "ElasticsearchCluster",
		Generation:         esc.Generation,
		ObservedGeneration: esc.Status.ObservedGeneration,
		Conditions:         esc.Status.Conditions,
	}
}

// DefaultGraceStatusHandler grades a ElasticsearchCluster that is still not
// converged after the grace period: Ready=True is Healthy, Ready=False with
// the reason Degraded is Degraded, and anything else is Down.
func DefaultGraceStatusHandler(esc *apiv1.ElasticsearchCluster) (concepts.GraceStatusWithReason, error) {
	return readycondition.Grace(observed(esc)), nil
}
