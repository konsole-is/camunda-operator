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

// Take claims the cluster for self. A cluster that no live holder claims is
// claimed, and Take reports Done. A cluster that another live holder claims
// is not, and Take reports a failure with v1.ReasonClusterClaimed that names
// the holder.
//
// Nothing bounds the hold. The restore starts on its own when the holder
// reaches a terminal phase, because the next look takes the claim over. The
// holder can be a backup or another restore, so the reason names no kind.
//
// A restore takes the claim when its admission passes, which is before every
// phase that touches storage. Two restores of one cluster therefore never
// both pass validation.
func Take(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	namespace, cluster string,
	self clusterclaim.Claimant,
) (Outcome, error) {
	holder, err := clusterclaim.Claim(ctx, c, reader, namespace, cluster, self)
	if err != nil {
		return Outcome{}, fmt.Errorf("claiming CamundaCluster %s/%s: %w", namespace, cluster, err)
	}
	if holder == "" {
		return Outcome{Done: true}, nil
	}

	// The Lease records the exact identity, which carries a UID that says
	// nothing to a reader of the condition.
	display := holder
	if parsed, parseErr := clusterclaim.ParseClaimant(holder); parseErr == nil {
		display = parsed.Display()
	}

	return Outcome{Failure: &conditions.PreCheckFailure{
		Reason: v1.ReasonClusterClaimed,
		Message: fmt.Sprintf(
			"%s holds CamundaCluster %s/%s. Only one backup or restore of a cluster runs at a time, "+
				"so this restore starts when that operation reaches a terminal phase",
			display, namespace, cluster,
		),
	}}, nil
}

// Give returns the claim on the cluster. It is safe on every look of a
// terminal restore and inside a finalizer: a Lease that another claimant
// holds is left alone.
func Give(
	ctx context.Context,
	c client.Client,
	reader client.Reader,
	namespace, cluster string,
	self clusterclaim.Claimant,
) error {
	if err := clusterclaim.Release(ctx, c, reader, namespace, cluster, self); err != nil {
		return fmt.Errorf("releasing CamundaCluster %s/%s: %w", namespace, cluster, err)
	}

	return nil
}
