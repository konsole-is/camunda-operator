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
	"net/url"
	"strings"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/leaseclaim"
)

// StorageClaimComponent is the component label value of a storage claim
// Lease.
const StorageClaimComponent = "storage-claim"

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

// StorageClaimKey returns the backend that storage addresses, as the key of
// its storage claim. Two contracts that name one address give one key, and a
// contract that is edited to another address gives another key.
//
// An Elasticsearch key is the type, then the endpoint with the scheme and
// the host in lower case, the port written out (80 for http, 443 for https
// when the URL names none), and no trailing slash on the path. An rdbms key
// is the type, then the host in lower case, the port, and the database name.
// A chain that names no address, or an endpoint that is no URL, is an error.
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
			strings.ToLower(storage.RDBMS.Host),
			storage.RDBMS.Port,
			storage.RDBMS.Database,
		), nil
	default:
		return "", fmt.Errorf("unknown secondary storage type %q", storage.Type)
	}
}

// normalizeEndpoint renders an Elasticsearch endpoint the way StorageClaimKey
// documents it.
func normalizeEndpoint(endpoint string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("parsing the Elasticsearch endpoint %q: %w", endpoint, err)
	}
	if parsed.Scheme == "" || parsed.Hostname() == "" {
		return "", fmt.Errorf("the Elasticsearch endpoint %q names no scheme or no host", endpoint)
	}

	scheme := strings.ToLower(parsed.Scheme)
	port := parsed.Port()
	if port == "" {
		port = "80"
		if scheme == "https" {
			port = "443"
		}
	}

	return scheme + "://" + strings.ToLower(parsed.Hostname()) + ":" + port + strings.TrimSuffix(parsed.Path, "/"), nil
}
