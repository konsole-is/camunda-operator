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

package restore

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// Shortly is how long a step waits after it staged a record that the next
// side effect depends on. The status flush of the reconcile persists the
// record, and the next look acts on a durable one.
const Shortly = time.Second

// Outcome is what a driver step reports back to a controller. The driver
// never writes status.phase, because each restore kind owns its own phase
// vocabulary. The controller maps an outcome onto its own phase.
type Outcome struct {
	// Wait is how long before the next look. Zero means that the watches
	// carry the wake-up, and the controller settles.
	Wait time.Duration
	// Done reports that the step finished and the controller advances.
	Done bool
	// Failure is why the step cannot go on, with the Ready reason and the
	// message that belong to it. What it means for the resource is the
	// controller's to decide: a failure of the primary-storage phase ends the
	// restore, and a cluster that another operation holds keeps it waiting.
	Failure *conditions.PreCheckFailure
}

// progressing stages that a step of the driver runs. Both steps a controller
// hands the driver report through it: the preparation of the cluster and the
// primary-storage phase.
func progressing(owner conditions.Owner, message string) {
	conditions.Stage(owner, conditions.Ready(
		metav1.ConditionFalse, v1.ReasonProgressing, message, owner.GetGeneration(),
	))
}

// failure ends the restore with message. Every failure that the driver raises
// itself reports the reason Failed. It runs behind every rule of a kind that
// carries a reason of its own, so no such rule reaches it.
func failure(message string) Outcome {
	return Outcome{Failure: &conditions.PreCheckFailure{
		Reason: v1.ReasonFailed, Message: message,
	}}
}
