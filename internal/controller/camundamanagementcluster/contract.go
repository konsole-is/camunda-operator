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

package camundamanagementcluster

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

// checkContractOwner refuses a contract name that another owner holds. The
// ManagementAuthConfig is cluster-scoped, so two management clusters in two
// namespaces can ask for the same name, and the first one there keeps it. The
// two owner labels identify the holder; an object without them belongs to
// whoever created it by hand.
func (r *Reconciler) checkContractOwner(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	name string,
) error {
	var existing v1.ManagementAuthConfig
	if err := r.APIReader.Get(ctx, client.ObjectKey{Name: name}, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("reading ManagementAuthConfig %q: %w", name, err)
	}
	if ownedBy(&existing, mc) {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonConflict,
		Message: fmt.Sprintf(
			"ManagementAuthConfig %q exists and belongs to %s; "+
				"set spec.managementAuthConfigName to a free name, or remove the object",
			name, contractHolder(&existing),
		),
	}
}

// applyContract writes the ManagementAuthConfig that Optimize reads. The apply
// carries the owner labels and the whole spec under this controller's field
// manager, so a hand edit of a field the operator owns converges back.
func (r *Reconciler) applyContract(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	spec v1.ManagementAuthConfigSpec,
) error {
	contract := &v1.ManagementAuthConfig{
		TypeMeta: metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "ManagementAuthConfig"},
		ObjectMeta: metav1.ObjectMeta{
			Name:   components.ContractName(mc),
			Labels: components.ContractLabels(mc),
		},
		Spec: spec,
	}

	//nolint:staticcheck // the repository applies through the deprecated client.Apply patch
	if err := r.Patch(
		ctx, contract, client.Apply,
		client.FieldOwner(components.FieldManager), client.ForceOwnership,
	); err != nil {
		return fmt.Errorf("applying ManagementAuthConfig %q: %w", contract.Name, err)
	}

	return nil
}

// withdrawContract deletes the ManagementAuthConfig of this management
// cluster. The contract is cluster-scoped and its owner is namespaced, so
// Kubernetes collects nothing: the finalizer is what removes it. A contract
// that another owner holds is left alone.
func (r *Reconciler) withdrawContract(ctx context.Context, mc *v1.CamundaManagementCluster) error {
	name := components.ContractName(mc)

	var existing v1.ManagementAuthConfig
	if err := r.APIReader.Get(ctx, client.ObjectKey{Name: name}, &existing); err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("reading ManagementAuthConfig %q: %w", name, err)
	}
	if !ownedBy(&existing, mc) {
		return nil
	}

	if err := r.Delete(ctx, &existing); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("deleting ManagementAuthConfig %q: %w", name, err)
	}

	return nil
}

// ownedBy reports whether the contract carries the owner labels of mc. The
// name label is bounded, so a long management cluster name is compared in the
// same bounded form.
func ownedBy(contract *v1.ManagementAuthConfig, mc *v1.CamundaManagementCluster) bool {
	got := contract.GetLabels()

	return got[labels.ManagementClusterKey] == labels.OwnerName(mc.Name) &&
		got[labels.ManagementClusterNamespaceKey] == mc.Namespace
}

// contractHolder names the owner of a contract for a condition message: the
// management cluster of its owner labels, or "another writer" when it carries
// none.
func contractHolder(contract *v1.ManagementAuthConfig) string {
	got := contract.GetLabels()
	name, namespace := got[labels.ManagementClusterKey], got[labels.ManagementClusterNamespaceKey]
	if name == "" || namespace == "" {
		return "another writer"
	}

	return fmt.Sprintf("CamundaManagementCluster %s/%s", namespace, name)
}
