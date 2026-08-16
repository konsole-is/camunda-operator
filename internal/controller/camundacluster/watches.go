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

package camundacluster

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

// SetupWithManager registers the controller and the ownership watches on the
// workloads, Services, ServiceAccounts, and Secrets (metadata only). It also
// sets Recorder to the recorder of the manager and builds the uncached
// component client when they are nil.
func (r *CamundaClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Recorder == nil {
		// The ReconcileContext of the component framework takes the legacy
		// record.EventRecorder, so the deprecated accessor is required here.
		r.Recorder = mgr.GetEventRecorderFor("camundacluster") //nolint:staticcheck
	}
	r.restMapper = mgr.GetRESTMapper()

	if r.componentClient == nil {
		componentClient, err := client.New(mgr.GetConfig(), client.Options{
			Scheme: mgr.GetScheme(),
			Mapper: mgr.GetRESTMapper(),
		})
		if err != nil {
			return fmt.Errorf("building the component client: %w", err)
		}
		r.componentClient = componentClient
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.CamundaCluster{}).
		Owns(&appsv1.StatefulSet{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&corev1.Secret{}, builder.OnlyMetadata).
		Named("camundacluster").
		Complete(r)
}
