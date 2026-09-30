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
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/clusterclaim"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// Take claims the cluster for owner. cluster is the name of the target, in
// the namespace of the restore. Take reports Done when owner holds the claim.
// When another holder claims the cluster, Take reports a failure with
// v1.ReasonClusterClaimed and clusterclaim.WaitMessage. Nothing bounds that
// wait: a later call takes the claim over only once clusterclaim.Claim finds
// the holder inactive, and never from a Lease that the message says to delete.
//
// Call it when admission passes, before each phase that touches storage, so
// two restores of one cluster never both pass validation. reader must read
// the API server directly.
func Take(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	owner conditions.Owner,
	cluster string,
) (Outcome, error) {
	namespace := owner.GetNamespace()

	holder, err := clusterclaim.Claim(ctx, c, reader, namespace, cluster, claimantOf(owner))
	if err != nil {
		return Outcome{}, fmt.Errorf("claiming CamundaCluster %s/%s: %w", namespace, cluster, err)
	}
	if holder == "" {
		return Outcome{Done: true}, nil
	}

	return Outcome{Failure: &conditions.PreCheckFailure{
		Reason:  v1.ReasonClusterClaimed,
		Message: clusterclaim.WaitMessage(holder, namespace, cluster, "restore"),
	}}, nil
}

// Give releases the claim that owner holds on the cluster. A Lease that
// another claimant holds is left alone.
func Give(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	owner conditions.Owner,
	cluster string,
) error {
	namespace := owner.GetNamespace()

	if err := clusterclaim.Release(ctx, c, reader, namespace, cluster, claimantOf(owner)); err != nil {
		return fmt.Errorf("releasing CamundaCluster %s/%s: %w", namespace, cluster, err)
	}

	return nil
}

func claimantOf(owner conditions.Owner) clusterclaim.Claimant {
	return clusterclaim.Claimant{
		Kind: owner.GetKind(), Name: owner.GetName(), UID: owner.GetUID(),
	}
}
