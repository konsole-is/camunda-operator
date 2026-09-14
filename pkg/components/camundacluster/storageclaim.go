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
	"context"
	"fmt"
	"net"
	"net/url"
	"slices"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/leaseclaim"
)

// StorageClaimComponent is the component label value of a storage claim
// Lease.
const StorageClaimComponent = "storage-claim"

// podPhaseField is the field of a pod that the API server serves as a field
// selector, so a list can leave the pods that ended to the API server.
const podPhaseField = "status.phase"

// storageClaimLeasePrefix starts the name of every storage claim Lease.
const storageClaimLeasePrefix = "camunda-storage-"

// The annotations of a storage claim Lease. The first three name the
// CamundaCluster that holds the backend, the fourth the backend itself. A
// Lease without all three holder annotations is not one of ours.
const (
	StorageClaimHolderNamespaceAnnotation = "camunda.io/storage-claim-holder-namespace"
	StorageClaimHolderNameAnnotation      = "camunda.io/storage-claim-holder-name"
	StorageClaimHolderUIDAnnotation       = "camunda.io/storage-claim-holder-uid"
	StorageClaimKeyAnnotation             = "camunda.io/storage-claim-key"
)

// StorageClaimSchema is the shape of the storage claim Leases. One
// CamundaCluster writes one backend, so the claim key is the backend and
// every cluster that resolves it meets on one Lease, whatever contract it
// names.
func StorageClaimSchema() leaseclaim.Schema[*v1.CamundaCluster] {
	return leaseclaim.Schema[*v1.CamundaCluster]{
		Prefix:                    storageClaimLeasePrefix,
		Noun:                      "storage claim",
		HolderNamespaceAnnotation: StorageClaimHolderNamespaceAnnotation,
		HolderNameAnnotation:      StorageClaimHolderNameAnnotation,
		HolderUIDAnnotation:       StorageClaimHolderUIDAnnotation,
		KeyAnnotation:             StorageClaimKeyAnnotation,
		Labels:                    StorageClaimLeaseLabels,
	}
}

// StorageClaimLeaseLabels returns the labels of the storage claim Leases of
// the CamundaClusters named name. They carry the name alone, so two clusters
// of two namespaces that share a name share these labels. A caller reads
// HolderOf of StorageClaimSchema on a listed Lease to learn which cluster
// holds it.
func StorageClaimLeaseLabels(name string) map[string]string {
	return labels.Managed(labels.Cluster(name), StorageClaimComponent)
}

// OtherPodsOnClaim returns the pods that carry the storage claim named claim
// and a cluster UID other than self, as sorted "namespace/name" paths. A
// handover waits for exactly these: every one of them writes the backend of
// that claim. The reader must read the API server directly, because a decision
// from a stale cache starts a second writer.
//
// The list covers every namespace, because two clusters of two namespaces can
// resolve one backend. It leaves out the pods that reached Failed or
// Succeeded: an evicted pod of a previous holder keeps its object under a
// ReplicaSet that nobody deleted, and it writes nothing. It keeps a pod with a
// deletion timestamp, because one on a lost node still writes until the node
// comes back or the pod is forced away.
//
// The pods of a CamundaOptimize attached to another cluster carry the same
// labels, see pkg/components/camundaoptimize, and its importer writes the
// backend like a pod of that cluster.
func OtherPodsOnClaim(
	ctx context.Context,
	reader client.Reader,
	claim string,
	self types.UID,
) ([]string, error) {
	pods := podMetadataList()
	err := reader.List(
		ctx,
		pods,
		client.MatchingLabels(map[string]string{labels.StorageClaimKey: labels.OwnerName(claim)}),
		client.MatchingFieldsSelector{Selector: endedPodsExcluded()},
	)
	if err != nil {
		return nil, fmt.Errorf("listing the pods on storage claim %q: %w", claim, err)
	}

	var names []string
	for i := range pods.Items {
		if pods.Items[i].Labels[labels.ClusterUIDKey] == string(self) {
			continue
		}
		names = append(names, pods.Items[i].Namespace+"/"+pods.Items[i].Name)
	}
	slices.Sort(names)

	return names, nil
}

// ClaimsWrittenByPods returns the claims of names that a pod of the cluster
// still carries, sorted. A cluster gives a backend back only when none of its
// own pods writes it any more: a release under running pods lets the next
// claimant start beside them, because that claimant reads the pods once.
//
// The pods of the cluster live in its namespace, so the list stays there. It
// leaves out the pods that ended, like OtherPodsOnClaim.
func ClaimsWrittenByPods(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	self types.UID,
	names []string,
) ([]string, error) {
	if len(names) == 0 {
		return nil, nil
	}

	pods := podMetadataList()
	err := reader.List(
		ctx,
		pods,
		client.InNamespace(namespace),
		client.MatchingLabels(map[string]string{labels.ClusterUIDKey: string(self)}),
		client.MatchingFieldsSelector{Selector: endedPodsExcluded()},
	)
	if err != nil {
		return nil, fmt.Errorf("listing the pods of the cluster in namespace %q: %w", namespace, err)
	}

	written := make(map[string]bool, len(pods.Items))
	for i := range pods.Items {
		written[pods.Items[i].Labels[labels.StorageClaimKey]] = true
	}

	var carried []string
	for _, name := range names {
		if written[labels.OwnerName(name)] {
			carried = append(carried, name)
		}
	}
	slices.Sort(carried)

	return carried, nil
}

// endedPodsExcluded leaves the pods that reached Failed or Succeeded to the API
// server. An evicted pod of a previous holder keeps its object under a
// ReplicaSet that nobody deleted, and it writes nothing. A pod with a deletion
// timestamp is not excluded: one on a lost node still writes until the node
// comes back or the pod is forced away.
func endedPodsExcluded() fields.Selector {
	return fields.AndSelectors(
		fields.OneTermNotEqualSelector(podPhaseField, string(corev1.PodFailed)),
		fields.OneTermNotEqualSelector(podPhaseField, string(corev1.PodSucceeded)),
	)
}

// podMetadataList returns the list that a pod gate fills. The name, the
// namespace and the labels are all it reads, so the API server sends metadata
// and no pod spec.
func podMetadataList() *metav1.PartialObjectMetadataList {
	list := &metav1.PartialObjectMetadataList{}
	list.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("PodList"))

	return list
}

// StorageClaimKey returns the backend that storage addresses, as the key of
// its storage claim. Two contracts that name one address give one key, and a
// contract that is edited to another address gives another key.
//
// An Elasticsearch key is the type, then the scheme and the host of the
// endpoint, and its port as a number (80 for http, 443 for https when the URL
// names none). The path of the endpoint is left out: Optimize connects to the
// host and the port, so two endpoints that differ in the path reach one
// Elasticsearch for at least one writer. An rdbms key is the type, then the
// host, the port, and the database name. Every host goes through
// normalizeHost. A chain that names no address, or an endpoint that is no URL,
// is an error.
func StorageClaimKey(storage Storage) (string, error) {
	switch storage.Type {
	case v1.SecondaryStorageTypeElasticsearch:
		if storage.Elasticsearch == nil {
			return "", fmt.Errorf("storage of type %s has no elasticsearch block", storage.Type)
		}
		endpoint, err := normalizeEndpoint(storage.Elasticsearch.Endpoint)
		if err != nil {
			return "", err
		}

		return string(storage.Type) + "|" + endpoint, nil
	case v1.SecondaryStorageTypeRDBMS:
		if storage.RDBMS == nil {
			return "", fmt.Errorf("storage of type %s has no rdbms block", storage.Type)
		}

		return fmt.Sprintf(
			"%s|%s:%d/%s",
			storage.Type,
			normalizeHost(storage.RDBMS.Host),
			storage.RDBMS.Port,
			storage.RDBMS.Database,
		), nil
	default:
		return "", fmt.Errorf("unknown secondary storage type %q", storage.Type)
	}
}

// StorageClaimKeyFailure is the Ready failure that a contract earns when
// StorageClaimKey cannot name its backend. Both the cluster and its Optimize
// report it, so a chain that resolves to no address reads the same on either.
func StorageClaimKeyFailure(contract client.ObjectKey, err error) *conditions.PreCheckFailure {
	return &conditions.PreCheckFailure{
		Reason:  v1.ReasonInvalidReference,
		Message: fmt.Sprintf("SecondaryStorageConfig %q: %s", contract.Namespace+"/"+contract.Name, err),
	}
}

// normalizeEndpoint renders the address of an Elasticsearch endpoint the way
// StorageClaimKey documents it: the scheme, the host, and the port, and
// nothing of the path.
//
// Optimize connects to the host and the port of the endpoint, so two endpoints
// that differ only in the path reach one Elasticsearch for its importer.
func normalizeEndpoint(endpoint string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parsing the Elasticsearch endpoint %q: %w", endpoint, err)
	}
	if parsed.Scheme == "" || parsed.Hostname() == "" {
		return "", fmt.Errorf("the Elasticsearch endpoint %q names no scheme or no host", endpoint)
	}

	scheme := strings.ToLower(parsed.Scheme)
	port := 80
	if scheme == "https" {
		port = 443
	}
	// The number renders one spelling for ":09200" and ":9200", so one address
	// gives one key.
	if written := parsed.Port(); written != "" {
		port, err = strconv.Atoi(written)
		if err != nil {
			return "", fmt.Errorf("the Elasticsearch endpoint %q has no numeric port: %w", endpoint, err)
		}
	}

	// Hostname strips the brackets of an IPv6 literal, and JoinHostPort puts
	// them back. Without them the address reads as another host and another
	// port.
	host := net.JoinHostPort(normalizeHost(parsed.Hostname()), strconv.Itoa(port))

	return scheme + "://" + host, nil
}

// normalizeHost lowercases a host name and drops the trailing dot of the DNS
// root. Both spellings resolve to the same host, so both must give one key.
func normalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(host), ".")
}
