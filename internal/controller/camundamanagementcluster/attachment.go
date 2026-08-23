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
	"slices"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8slabels "k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundamanagementcluster"
)

// ClaimValue returns the value of the claim annotation that mc puts on the
// orchestration clusters it serves: "<namespace>/<name>". One cluster answers
// to one management plane, so a cluster that carries another value is left
// alone.
func ClaimValue(mc *v1.CamundaManagementCluster) string {
	return mc.Namespace + "/" + mc.Name
}

// attachedClusters selects the orchestration clusters that mc serves, claims
// each of them, and reports what the management plane can do with each one.
//
// The selector follows the Kubernetes convention: an unset selector selects no
// cluster, and an empty one selects every cluster. A selected cluster that no
// other management plane holds gets the claim annotation. A cluster that
// another one holds is left untouched and reported ClaimedElsewhere. A cluster
// that publishes no gateway endpoints yet is reported NotReady. A cluster that
// the selector no longer matches has its claim withdrawn.
//
// The first return value carries the clusters that Console lists and Web
// Modeler deploys to, ordered by namespace and name. The second is
// status.clusters, with one row per selected cluster in the same order.
func (r *Reconciler) attachedClusters(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) ([]components.AttachedCluster, []v1.AttachedClusterStatus, error) {
	selector, err := metav1.LabelSelectorAsSelector(mc.Spec.ClusterSelector)
	if err != nil {
		return nil, nil, fmt.Errorf("reading spec.clusterSelector: %w", err)
	}

	clusters, err := r.listClusters(ctx)
	if err != nil {
		return nil, nil, err
	}

	var attached []components.AttachedCluster
	var rows []v1.AttachedClusterStatus
	for i := range clusters {
		cluster := &clusters[i]
		if !selector.Matches(k8slabels.Set(cluster.Labels)) {
			if err := r.withdrawClaim(ctx, mc, cluster); err != nil {
				return nil, nil, err
			}
			continue
		}

		row, err := r.attach(ctx, mc, cluster, &attached)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, row)
	}

	return attached, rows, nil
}

// listClusters reads every CamundaCluster of the Kubernetes cluster, ordered
// by namespace and name, so the claim decisions and status.clusters are stable
// across reconciles.
//
// The list is read live rather than from the cache. A claim decided from a
// stale cache can take a cluster that another management plane claimed
// moments ago.
func (r *Reconciler) listClusters(ctx context.Context) ([]v1.CamundaCluster, error) {
	var list v1.CamundaClusterList
	if err := r.APIReader.List(ctx, &list); err != nil {
		return nil, fmt.Errorf("listing the CamundaClusters: %w", err)
	}

	slices.SortFunc(list.Items, func(a, b v1.CamundaCluster) int {
		if a.Namespace != b.Namespace {
			return strings.Compare(a.Namespace, b.Namespace)
		}
		return strings.Compare(a.Name, b.Name)
	})

	return list.Items, nil
}

// attach claims one selected cluster and reports its row. A cluster that the
// management plane can serve is appended to attached.
func (r *Reconciler) attach(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	cluster *v1.CamundaCluster,
	attached *[]components.AttachedCluster,
) (v1.AttachedClusterStatus, error) {
	row := v1.AttachedClusterStatus{Name: cluster.Name, Namespace: cluster.Namespace}

	holder := cluster.Annotations[components.ClaimAnnotation]
	if holder != "" && holder != ClaimValue(mc) {
		row.Reason = v1.ReasonClaimedElsewhere
		row.Message = fmt.Sprintf("CamundaManagementCluster %q already serves this cluster", holder)

		return row, nil
	}
	if holder == "" {
		if err := r.claim(ctx, mc, cluster); err != nil {
			return row, err
		}
	}

	if cluster.Status.Gateway == nil {
		row.Reason = v1.ReasonNotReady
		row.Message = "The cluster publishes no gateway endpoints yet"

		return row, nil
	}

	method, failure, err := r.clusterAuthMethod(ctx, cluster)
	if err != nil {
		return row, err
	}
	if failure != "" {
		row.Reason = v1.ReasonInvalidReference
		row.Message = failure

		return row, nil
	}

	row.Attached = true
	*attached = append(*attached, components.AttachedCluster{
		Name:         cluster.Name,
		Namespace:    cluster.Namespace,
		UID:          cluster.UID,
		Version:      clusterVersion(cluster),
		ExternalURL:  cluster.Spec.ExternalURL,
		GRPCEndpoint: cluster.Status.Gateway.GRPCEndpoint,
		RESTEndpoint: cluster.Status.Gateway.RESTEndpoint,
		AuthMethod:   method,
	})

	return row, nil
}

// clusterAuthMethod reads how a cluster authenticates its users and clients,
// from the platform config that the cluster names. A dangling reference comes
// back as a message for the row of that cluster: the cluster's own controller
// reports the same reference, and one broken cluster must not stop the
// management plane.
func (r *Reconciler) clusterAuthMethod(
	ctx context.Context,
	cluster *v1.CamundaCluster,
) (v1.AuthenticationMethod, string, error) {
	var cfg v1.CamundaPlatformConfig
	key := client.ObjectKey{Name: cluster.Spec.PlatformConfigRef}
	if err := r.APIReader.Get(ctx, key, &cfg); err != nil {
		if apierrors.IsNotFound(err) {
			return "", fmt.Sprintf("CamundaPlatformConfig %q of this cluster not found", key.Name), nil
		}
		return "", "", fmt.Errorf("reading CamundaPlatformConfig %q: %w", key.Name, err)
	}

	return cfg.Spec.Method(), "", nil
}

// clusterVersion returns the Camunda version that a cluster runs: the one it
// publishes, and spec.version until it publishes one. A preset can supply
// spec.version, so the published version is the one that is always effective.
func clusterVersion(cluster *v1.CamundaCluster) string {
	if cluster.Status.Management != nil && cluster.Status.Management.Version != "" {
		return cluster.Status.Management.Version
	}

	return cluster.Spec.Version
}

// withdrawClaims removes the claim of mc from every cluster that carries it.
// The finalizer calls it, so a deleted management cluster leaves no cluster
// claimed by an owner that is gone.
func (r *Reconciler) withdrawClaims(ctx context.Context, mc *v1.CamundaManagementCluster) error {
	clusters, err := r.listClusters(ctx)
	if err != nil {
		return err
	}

	for i := range clusters {
		if err := r.withdrawClaim(ctx, mc, &clusters[i]); err != nil {
			return err
		}
	}

	return nil
}

// withdrawClaim removes the claim of mc from one cluster. A cluster that
// carries another claim, or none, is left alone.
func (r *Reconciler) withdrawClaim(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	cluster *v1.CamundaCluster,
) error {
	if cluster.Annotations[components.ClaimAnnotation] != ClaimValue(mc) {
		return nil
	}

	err := r.applyClaim(ctx, cluster, nil)
	// A cluster that went between the list and the apply needs no withdrawal.
	if apierrors.IsConflict(err) || apierrors.IsNotFound(err) {
		return nil
	}

	return err
}

// claim puts the claim of mc on one cluster.
func (r *Reconciler) claim(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	cluster *v1.CamundaCluster,
) error {
	return r.applyClaim(ctx, cluster, map[string]string{components.ClaimAnnotation: ClaimValue(mc)})
}

// applyClaim applies the minimal CamundaCluster object that carries the claim
// annotation under the attachment field manager. Nil annotations remove what
// that manager owns and leave every other annotation alone.
//
// The apply carries the UID of the cluster as a precondition. Server-side
// apply creates the object it does not find, so without it an apply against a
// cluster that went in the meantime would put an empty CamundaCluster in its
// place.
func (r *Reconciler) applyClaim(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	annotations map[string]string,
) error {
	patch := &v1.CamundaCluster{
		TypeMeta: metav1.TypeMeta{APIVersion: v1.GroupVersion.String(), Kind: "CamundaCluster"},
		ObjectMeta: metav1.ObjectMeta{
			Name:        cluster.Name,
			Namespace:   cluster.Namespace,
			UID:         cluster.UID,
			Annotations: annotations,
		},
	}

	//nolint:staticcheck // the repository applies through the deprecated client.Apply patch
	if err := r.Patch(
		ctx, patch, client.Apply,
		client.FieldOwner(components.AttachmentFieldManager), client.ForceOwnership,
	); err != nil {
		key := types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Name}

		return fmt.Errorf("applying the claim on CamundaCluster %q: %w", key, err)
	}

	return nil
}
