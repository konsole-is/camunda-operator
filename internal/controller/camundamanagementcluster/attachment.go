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
	"errors"
	"fmt"
	"net/url"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
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
// provider is the identity provider that the pre-checks resolved.
//
// The selector follows the Kubernetes convention: an unset selector selects no
// cluster, and an empty one selects every cluster. A selected cluster that no
// other management plane holds gets the claim annotation. A cluster that
// another one holds is left untouched and reported ClaimedElsewhere. A cluster
// that publishes no gateway endpoints yet, and one whose claim the API server
// refused because the cluster changed, is reported NotReady. A cluster whose
// platform config cannot be read is reported InvalidReference, and so is an
// oidc cluster that trusts another issuer. A cluster that the selector no longer
// matches keeps its claim until releaseClaims runs, after the Web Modeler user
// and the Console ping are withdrawn: a claim that went first would let
// another management plane adopt the web-modeler user that this one is about
// to remove.
//
// Only an API failure that concerns every cluster stops the reconcile. What
// one cluster answers is a row of that cluster, so a single broken cluster
// never holds back the management plane.
//
// The first return value carries the clusters that Console lists and Web
// Modeler deploys to, ordered by namespace and name. The second is
// status.clusters, with one row per selected cluster in the same order.
func (r *Reconciler) attachedClusters(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	clusters []v1.CamundaCluster,
	namespaces map[string]bool,
	provider components.IdentityProvider,
) ([]components.AttachedCluster, []v1.AttachedClusterStatus, error) {
	selector, err := metav1.LabelSelectorAsSelector(mc.Spec.ClusterSelector)
	if err != nil {
		return nil, nil, fmt.Errorf("reading spec.clusterSelector: %w", err)
	}

	var attached []components.AttachedCluster
	var rows []v1.AttachedClusterStatus
	for i := range clusters {
		cluster := &clusters[i]
		if !inNamespaces(cluster, namespaces) || !selector.Matches(k8slabels.Set(cluster.Labels)) {
			continue
		}

		row, err := r.attach(ctx, mc, cluster, provider, &attached)
		if err != nil {
			return nil, nil, err
		}
		rows = append(rows, row)
	}

	return attached, rows, nil
}

// releaseClaims withdraws the claim of mc from every cluster that the selector
// no longer matches. It runs after the Web Modeler user and the Console ping
// of those clusters are withdrawn, and only when both succeeded, so that no
// other management plane takes a cluster whose user this one still has to
// remove.
func (r *Reconciler) releaseClaims(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	clusters []v1.CamundaCluster,
	namespaces map[string]bool,
) error {
	selector, err := metav1.LabelSelectorAsSelector(mc.Spec.ClusterSelector)
	if err != nil {
		return fmt.Errorf("reading spec.clusterSelector: %w", err)
	}

	var errs []error
	for i := range clusters {
		cluster := &clusters[i]
		if inNamespaces(cluster, namespaces) && selector.Matches(k8slabels.Set(cluster.Labels)) {
			continue
		}
		errs = append(errs, r.withdrawClaim(ctx, mc, cluster))
	}

	return errors.Join(errs...)
}

// selectedNamespaces returns the names of the namespaces that
// spec.namespaceSelector matches. Nil puts no bound on the namespace: the
// selector is unset or empty. An empty set is a bound that admits none.
func (r *Reconciler) selectedNamespaces(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
) (map[string]bool, error) {
	spec := mc.Spec.NamespaceSelector
	if spec == nil || (len(spec.MatchLabels) == 0 && len(spec.MatchExpressions) == 0) {
		return nil, nil
	}

	selector, err := metav1.LabelSelectorAsSelector(spec)
	if err != nil {
		return nil, fmt.Errorf("reading spec.namespaceSelector: %w", err)
	}

	// Only the names and the labels are needed, so the list stays metadata.
	list := &metav1.PartialObjectMetadataList{}
	list.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("NamespaceList"))
	if err := r.APIReader.List(ctx, list, client.MatchingLabelsSelector{Selector: selector}); err != nil {
		return nil, fmt.Errorf("listing the namespaces of spec.namespaceSelector: %w", err)
	}

	selected := make(map[string]bool, len(list.Items))
	for i := range list.Items {
		selected[list.Items[i].Name] = true
	}

	return selected, nil
}

// inNamespaces reports whether the cluster sits inside the namespace bound.
// A nil bound admits every namespace.
func inNamespaces(cluster *v1.CamundaCluster, namespaces map[string]bool) bool {
	return namespaces == nil || namespaces[cluster.Namespace]
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
	provider components.IdentityProvider,
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
		err := r.claim(ctx, mc, cluster)
		// The cluster changed or went between the list and the apply. Its own
		// event brings the next reconcile, and the other clusters converge
		// meanwhile.
		if apierrors.IsConflict(err) || apierrors.IsNotFound(err) {
			row.Reason = v1.ReasonNotReady
			row.Message = "The cluster changed while the management plane claimed it"

			return row, nil
		}
		if err != nil {
			return row, err
		}
	}

	if cluster.Status.Gateway == nil {
		row.Reason = v1.ReasonNotReady
		row.Message = "The cluster publishes no gateway endpoints yet"

		return row, nil
	}

	method, failure, err := r.clusterAuth(ctx, cluster, provider)
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
		// The name follows from the cluster UID, so it is known before the
		// Secret exists. The Web Modeler user hook publishes it later in the
		// same reconcile.
		BasicUserSecret: basicUserSecret(mc, cluster, method),
	})

	return row, nil
}

// basicUserSecret returns the name of the Secret that holds the password of
// the Web Modeler user on a basic-auth cluster. An oidc cluster needs no user
// of its own, so it gets no name.
func basicUserSecret(
	mc *v1.CamundaManagementCluster,
	cluster *v1.CamundaCluster,
	method v1.AuthenticationMethod,
) string {
	if method != v1.AuthenticationMethodBasic || mc.Spec.WebModeler == nil {
		return ""
	}

	return components.WebModelerClusterUserSecretName(mc, cluster.UID)
}

// clusterAuth reads how a cluster authenticates its users and clients, from
// the platform config that the cluster names, and refuses an oidc cluster that
// validates the tokens of another issuer than provider. It returns the
// authentication method, or a message for the row of that cluster when the
// management plane cannot serve it.
//
// A dangling reference is a message rather than an error: the cluster's own
// controller reports the same reference, and one broken cluster must not stop
// the management plane.
//
// A basic-auth cluster is never refused for its issuer. Web Modeler signs in
// to it with a user that the operator publishes.
func (r *Reconciler) clusterAuth(
	ctx context.Context,
	cluster *v1.CamundaCluster,
	provider components.IdentityProvider,
) (v1.AuthenticationMethod, string, error) {
	var cfg v1.CamundaPlatformConfig
	key := client.ObjectKey{Name: cluster.Spec.PlatformConfigRef}
	if err := r.APIReader.Get(ctx, key, &cfg); err != nil {
		if apierrors.IsNotFound(err) {
			return "", fmt.Sprintf("CamundaPlatformConfig %q of this cluster not found", key.Name), nil
		}
		return "", "", fmt.Errorf("reading CamundaPlatformConfig %q: %w", key.Name, err)
	}

	method := cfg.Spec.Method()
	// The CRD admits no oidc method without the oidc block. The nil check
	// keeps an object that reached the API server before that rule from
	// stopping the manager.
	if method != v1.AuthenticationMethodOIDC || cfg.Spec.Auth.OIDC == nil {
		return method, "", nil
	}

	issuer := cfg.Spec.Auth.OIDC.IssuerURL
	if issuer == "" {
		return method, fmt.Sprintf(
			"CamundaPlatformConfig %q of this cluster sets no spec.auth.oidc.issuerUrl", key.Name,
		), nil
	}
	if !trustsIssuer(issuer, provider) {
		return method, fmt.Sprintf(
			"Web Modeler deploys with tokens of the issuer %q, and this cluster validates tokens of %q instead",
			provider.IssuerURL, issuer,
		), nil
	}

	return method, "", nil
}

// trustsIssuer reports whether a cluster that names issuer validates the
// tokens that the management plane hands out.
//
// The management plane carries two forms of its issuer: the one a browser
// reaches, which every token names, and the one a container reaches inside the
// Kubernetes cluster. They are the same address for a generic OIDC provider
// and different ones for a Keycloak that the operator runs, and a cluster may
// name either.
func trustsIssuer(issuer string, provider components.IdentityProvider) bool {
	if issuer == "" {
		return false
	}
	normalized := normalizeIssuer(issuer)

	return normalized == normalizeIssuer(provider.IssuerURL) ||
		normalized == normalizeIssuer(provider.IssuerBackendURL)
}

// normalizeIssuer returns the form of an issuer URL that trustsIssuer
// compares: the scheme and the host lowercased, and no trailing slash. A port,
// a path, and the case of a path select another issuer, so they are kept.
//
// A URL that does not parse comes back with the trailing slash removed and
// nothing else, so two such strings still compare exactly.
func normalizeIssuer(issuer string) string {
	parsed, err := url.Parse(issuer)
	if err != nil {
		return strings.TrimRight(issuer, "/")
	}

	parsed.Scheme = strings.ToLower(parsed.Scheme)
	parsed.Host = strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimRight(parsed.Path, "/")

	return parsed.String()
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
func (r *Reconciler) withdrawClaims(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
	clusters []v1.CamundaCluster,
) error {
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

	err := r.applyClaim(ctx, mc, cluster, nil)
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
	return r.applyClaim(ctx, mc, cluster, map[string]string{components.ClaimAnnotation: ClaimValue(mc)})
}

// applyClaim applies the minimal CamundaCluster object that carries the claim
// annotation under the attachment field manager of mc. Nil annotations remove
// what that manager owns and leave every other annotation alone.
//
// The apply forces no ownership: the annotation that another management
// cluster wrote belongs to that one's manager, and the API server answers a
// second claimant with a conflict. That is what makes two management clusters
// that read an unclaimed cluster at the same time end with one holder.
//
// The apply carries the UID of the cluster as a precondition. Server-side
// apply creates the object it does not find, so without it an apply against a
// cluster that went in the meantime would put an empty CamundaCluster in its
// place.
func (r *Reconciler) applyClaim(
	ctx context.Context,
	mc *v1.CamundaManagementCluster,
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
		ctx, patch, client.Apply, client.FieldOwner(components.AttachmentFieldManager(mc)),
	); err != nil {
		key := types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Name}

		return fmt.Errorf("applying the claim on CamundaCluster %q: %w", key, err)
	}

	return nil
}
