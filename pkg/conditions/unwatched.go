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

package conditions

// UnwatchedPreCheckFailure marks a pre-check failure that no watch resolves:
// nothing enqueues the owner when the missing object appears, so the
// reconcile must come back on its own timer. A controller matches it with
// errors.As and returns a RequeueAfter instead of relying on an event.
type UnwatchedPreCheckFailure struct {
	Failure *PreCheckFailure
}

// NewUnwatchedFailure wraps reason and message as a pre-check failure that no
// watch resolves.
func NewUnwatchedFailure(reason, message string) *UnwatchedPreCheckFailure {
	return &UnwatchedPreCheckFailure{
		Failure: &PreCheckFailure{Reason: reason, Message: message},
	}
}

// Error returns the message of the wrapped failure.
func (u *UnwatchedPreCheckFailure) Error() string { return u.Failure.Error() }

// Unwrap exposes the wrapped failure, so errors.As finds both types.
func (u *UnwatchedPreCheckFailure) Unwrap() error { return u.Failure }
