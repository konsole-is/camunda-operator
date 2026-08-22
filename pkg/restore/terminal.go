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
// It reports Done only when all three are given back. Until then it reports
// Outcome.Wait, which is a wait and not a failure, and the controller looks
// again after it.
//
// Every step is safe on every look, so a call that failed is safe to repeat.
// An error keeps the claim, and the step that failed heals on the next look.
func Finish(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	owner conditions.Owner,
	p *v1.RestoreProgress,
	cluster string,
) (Outcome, error) {
	// A conflict on the terminal flush can restore a stale Ready from the
	// server. Staging it again on every look heals that.
	StageTerminal(owner, p)

	// The Jobs go first, and the two steps below wait for them. A completed
	// Job keeps its pod, and a pod that mounts a broker data volume counts as
	// a user of it under the pvc-protection finalizer. Done says that no such
	// pod is left.
	collected, err := CollectJobs(ctx, c, reader, owner, p)
	if err != nil || !collected.Done {
		return collected, err
	}

	// The unsuspend sits behind that gate, and a tidy-up must not lift it out.
	// It starts the brokers again, and a broker that the scheduler places on
	// another node cannot attach a ReadWriteOnce volume that a completed pod
	// still holds, so the cluster would stall on the very volumes this branch
	// is freeing.
	if err := Resume(ctx, c, reader, p, types.NamespacedName{
		Namespace: owner.GetNamespace(), Name: cluster,
	}); err != nil {
		return Outcome{}, err
	}

	// The claim goes last. It is what tells the next operation that the
	// cluster is free, so the volumes are asked for before it.
	if err := Give(ctx, c, reader, owner, cluster); err != nil {
		return Outcome{}, err
	}

	return Outcome{Done: true}, nil
}
