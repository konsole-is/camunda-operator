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

package camundamanagementclusterclaim

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/claimdeletion"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementclusterclaim"
)

// finalize deletes what the claim generated, one step per pass: the
// CamundaManagementCluster, then the PluginRequests, as
// claimdeletion.Deleter.Finalize does.
func (r *Reconciler) finalize(
	ctx context.Context,
	claim *fleetv1alpha1.CamundaManagementClusterClaim,
) ([]string, error) {
	deleter := claimdeletion.Deleter{
		Client:        r.Client,
		APIReader:     r.APIReader,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
	}

	return deleter.Finalize(ctx, claimdeletion.Input{
		Claim:              claim,
		FieldManagerPrefix: fleetv1alpha1.FieldManagerPrefixPlaneClaim,
		DeletionPolicy:     components.DeletionPolicy(claim),
		Selections:         claim.Spec.Plugins,
		Stages: [][]client.Object{{
			&v1.CamundaManagementCluster{ObjectMeta: metav1.ObjectMeta{Name: claim.Name, Namespace: claim.Namespace}},
		}},
	})
}
