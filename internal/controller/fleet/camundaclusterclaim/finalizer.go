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

package camundaclusterclaim

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	fleetv1alpha1 "github.com/konsole-is/camunda-operator/api/fleet/v1alpha1"
	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/claimdeletion"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundaclusterclaim"
)

// finalize deletes what the claim generated, one step per pass: the
// attachments, then the CamundaCluster, then the PluginRequests, as
// claimdeletion.Deleter.Finalize does.
func (r *Reconciler) finalize(ctx context.Context, claim *fleetv1alpha1.CamundaClusterClaim) ([]string, error) {
	deleter := claimdeletion.Deleter{
		Client:        r.Client,
		APIReader:     r.APIReader,
		Scheme:        r.Scheme,
		EventRecorder: r.EventRecorder,
	}

	return deleter.Finalize(ctx, claimdeletion.Input{
		Claim:              claim,
		FieldManagerPrefix: fleetv1alpha1.FieldManagerPrefixClusterClaim,
		DeletionPolicy:     components.DeletionPolicy(claim),
		Selections:         claim.Spec.Plugins,
		Stages: [][]client.Object{
			{
				&v1.CamundaOptimize{ObjectMeta: objectKey(claim, components.OptimizeName(claim.Name))},
				&v1.BackupSchedule{ObjectMeta: objectKey(claim, components.BackupScheduleName(claim.Name))},
			},
			{&v1.CamundaCluster{ObjectMeta: objectKey(claim, claim.Name)}},
		},
	})
}

// objectKey returns the metadata of the object name in the namespace of
// claim.
func objectKey(claim *fleetv1alpha1.CamundaClusterClaim, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{Name: name, Namespace: claim.Namespace}
}
