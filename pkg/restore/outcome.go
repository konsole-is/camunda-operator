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

// Shortly is the wait after a step staged a record that its next side effect
// depends on. The status flush persists the record before the next look.
const Shortly = time.Second

// Outcome is what a driver step reports to a controller. The driver never
// writes status.phase: the controller maps an outcome onto its own phase.
type Outcome struct {
	// Wait is how long before the next look. Zero means that the watches
	// carry the wake-up, and the controller settles.
	Wait time.Duration
	// Done reports that the step finished and the controller advances.
	Done bool
	// Failure is why the step cannot go on, with its Ready reason and message.
	// The controller decides what it means for the resource.
	Failure *conditions.PreCheckFailure
}

func progressing(owner conditions.Owner, message string) {
	conditions.Stage(owner, conditions.Ready(
		metav1.ConditionFalse, v1.ReasonProgressing, message, owner.GetGeneration(),
	))
}

func failure(message string) Outcome {
	return Outcome{Failure: &conditions.PreCheckFailure{
		Reason: v1.ReasonFailed, Message: message,
	}}
}
