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
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// Finish is the terminal branch of every restore kind. Call it on every look
// of a restore in a terminal phase, in place of the next phase. It removes the
// Jobs of a completed restore, gives back what Resume gives back, and releases
// the claim on the cluster. label is the owner label of the restore. cluster
// is the name of the target, in the namespace of the restore.
//
// It reports Done once the claim is released, and Outcome.Wait until then. A
// failed restore keeps its Jobs, its hold and the suspension. After an error,
// call it again: the claim stays until the other steps succeed.
func Finish(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	owner conditions.Owner,
	label labels.Owner,
	p *v1.RestoreProgress,
	cluster string,
) (Outcome, error) {
	// A conflict on the terminal flush can restore a stale Ready from the
	// server. Staging it again on every look corrects that.
	StageTerminal(owner, p)

	// A completed Job keeps its pod, and the pvc-protection finalizer keeps a
	// broker volume that such a pod mounts.
	collected, err := CollectJobs(ctx, c, reader, owner, label, p)
	if err != nil || !collected.Done {
		return collected, err
	}

	// Resume can start the brokers again. A broker on another node cannot attach
	// a ReadWriteOnce volume that a Job pod still holds, so it waits for Done.
	if err := Resume(ctx, c, reader, owner, p, types.NamespacedName{
		Namespace: owner.GetNamespace(), Name: cluster,
	}); err != nil {
		return Outcome{}, err
	}

	// The claim goes last, because it tells the next operation that the
	// cluster is free.
	if err := Give(ctx, c, reader, owner, cluster); err != nil {
		return Outcome{}, err
	}

	return Outcome{Done: true}, nil
}
