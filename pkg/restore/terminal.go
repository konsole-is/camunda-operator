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
	"context"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// Finish is the terminal branch of every restore kind. The controller runs it
// on every look of a restore that reached a terminal phase, in place of the
// phase it would otherwise advance. It gives back what the restore held: the
// Jobs of a completed restore, the suspension that the restore applied, and
// the claim on the cluster. cluster is the name of the target, which lives in
// the namespace of the restore.
//
// It stages the recorded terminal condition first. A conflict on the terminal
// flush can restore a stale Ready from the server, and staging it again on
// every look heals that.
//
// It reports Done only when all three are given back, and it converges over
// more than one look. Collecting the Jobs deletes them with foreground
// propagation, which outlives the call: a Job that still exists means its pods
// can still exist. Outcome.Wait then paces the look that finds them gone, and
// the two steps behind the Jobs do not run until that look.
//
// The order of the three releases is the correctness of this branch, and a
// tidy-up must not change it:
//
//   - The Jobs go before the unsuspend. A completed Job keeps its pod, and a
//     pod that mounts a broker data volume counts as a user of it. The
//     unsuspend starts the brokers again. A broker that the scheduler places
//     on another node cannot attach a ReadWriteOnce volume that a completed
//     pod still holds. An unsuspend that ran first would stall the cluster on
//     the very volumes this branch is freeing.
//   - Both go before the claim. The claim is what tells the next operation
//     that the cluster is free, so the volumes are asked for first.
//
// Every step is safe on every look, and an error keeps the claim for one more
// look. The branch runs again on the next one, and the step that failed heals
// there.
func Finish(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	owner conditions.Owner,
	p *v1.RestoreProgress,
	cluster string,
) (Outcome, error) {
	StageTerminal(owner, p)

	collected, err := CollectJobs(ctx, c, reader, owner, p)
	if err != nil || !collected.Done {
		return collected, err
	}

	if err := Resume(ctx, c, reader, p, types.NamespacedName{
		Namespace: owner.GetNamespace(), Name: cluster,
	}); err != nil {
		return Outcome{}, err
	}

	if err := Give(ctx, c, reader, owner, cluster); err != nil {
		return Outcome{}, err
	}

	return Outcome{Done: true}, nil
}
