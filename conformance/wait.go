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

package conformance

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/util/wait"
)

// pollInterval is the wait between two calls of the check of waitFor.
const pollInterval = 500 * time.Millisecond

// waitFor calls check until it returns nil, for at most timeout. On a timeout
// the error names what the wait was for and holds the last error of check. A
// canceled ctx ends the wait at once with the error of ctx.
func waitFor(ctx context.Context, timeout time.Duration, what string, check func(ctx context.Context) error) error {
	var last error
	err := wait.PollUntilContextTimeout(ctx, pollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		last = check(ctx)
		return last == nil, nil
	})

	switch {
	case err == nil:
		return nil
	case ctx.Err() != nil:
		return ctx.Err()
	default:
		return fmt.Errorf("%s within %s: %w", what, timeout, last)
	}
}
