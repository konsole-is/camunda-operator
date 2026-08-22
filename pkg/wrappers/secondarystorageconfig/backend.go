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

package secondarystorageconfig

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
)

// Backend is the secondary storage backend that a contract resolves to: the
// server the workloads connect to, not the contract object that names it.
// Two contracts with one Identity name one backend, whatever their
// namespaces and credentials.
type Backend struct {
	// Type is the storage type of the contract.
	Type v1.SecondaryStorageType
	// Identity is the normalized endpoint of an Elasticsearch backend, or
	// host:port/database of a relational one.
	Identity string
}

// String names the backend for a condition message, for example
// `Elasticsearch "https://es.example.com:9200"` or `database "pg:5432/camunda"`.
func (b Backend) String() string {
	if b.Type == v1.SecondaryStorageTypeRDBMS {
		return fmt.Sprintf("database %q", b.Identity)
	}

	return fmt.Sprintf("Elasticsearch %q", b.Identity)
}

// ResolveBackend resolves contract to its Backend. An rdbms contract is
// followed through its DatabaseConfig, in the namespace of the contract, to
// the DatabaseServerConfig. A contract without the block of its type, a
// chain object that does not exist, or an endpoint that is not a URL comes
// back as a *conditions.PreCheckFailure with ReasonInvalidReference. An
// error is a transient read.
func ResolveBackend(
	ctx context.Context,
	reader client.Reader,
	contract *v1.SecondaryStorageConfig,
) (Backend, *conditions.PreCheckFailure, error) {
	key := client.ObjectKeyFromObject(contract)

	switch contract.Spec.Type {
	case v1.SecondaryStorageTypeElasticsearch:
		if contract.Spec.Elasticsearch == nil {
			return Backend{}, invalidReference("SecondaryStorageConfig %s has no elasticsearch block", key), nil
		}

		identity, err := ElasticsearchIdentity(contract.Spec.Elasticsearch.Endpoint)
		if err != nil {
			return Backend{}, invalidReference("SecondaryStorageConfig %s: %v", key, err), nil
		}

		return Backend{Type: v1.SecondaryStorageTypeElasticsearch, Identity: identity}, nil, nil

	case v1.SecondaryStorageTypeRDBMS:
		if contract.Spec.RDBMS == nil {
			return Backend{}, invalidReference("SecondaryStorageConfig %s has no rdbms block", key), nil
		}

		var dbConfig v1.DatabaseConfig
		dbKey := client.ObjectKey{Namespace: contract.Namespace, Name: contract.Spec.RDBMS.DatabaseConfigRef}
		if err := reader.Get(ctx, dbKey, &dbConfig); err != nil {
			if apierrors.IsNotFound(err) {
				return Backend{}, invalidReference("DatabaseConfig %s does not exist", dbKey), nil
			}

			return Backend{}, nil, fmt.Errorf("reading DatabaseConfig %s: %w", dbKey, err)
		}

		var server v1.DatabaseServerConfig
		serverKey := client.ObjectKey{Name: dbConfig.Spec.ServerRef}
		if err := reader.Get(ctx, serverKey, &server); err != nil {
			if apierrors.IsNotFound(err) {
				return Backend{}, invalidReference("DatabaseServerConfig %s does not exist", serverKey), nil
			}

			return Backend{}, nil, fmt.Errorf("reading DatabaseServerConfig %s: %w", serverKey, err)
		}

		return Backend{
			Type:     v1.SecondaryStorageTypeRDBMS,
			Identity: RDBMSIdentity(server.Spec.Host, server.Spec.Port, dbConfig.Spec.DatabaseName),
		}, nil, nil

	default:
		return Backend{}, invalidReference(
			"SecondaryStorageConfig %s has unsupported type %q",
			key,
			contract.Spec.Type,
		), nil
	}
}

func invalidReference(format string, args ...any) *conditions.PreCheckFailure {
	return &conditions.PreCheckFailure{Reason: v1.ReasonInvalidReference, Message: fmt.Sprintf(format, args...)}
}

// defaultPorts are the ports an endpoint URL implies when it names none.
var defaultPorts = map[string]string{"http": "80", "https": "443"}

// ElasticsearchIdentity normalizes an Elasticsearch endpoint URL so that two
// spellings of one server compare equal: the scheme and the host in lower
// case, the port explicit (the default port of the scheme when the URL names
// none), the path without its trailing slash, and no query or fragment. The
// path stays, because an Elasticsearch behind a path prefix is another
// server. An endpoint without a scheme, a host, or a resolvable port is an
// error.
func ElasticsearchIdentity(endpoint string) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", fmt.Errorf("endpoint %q is not a URL: %w", endpoint, err)
	}

	scheme := strings.ToLower(parsed.Scheme)
	host := strings.ToLower(parsed.Hostname())
	if scheme == "" || host == "" {
		return "", fmt.Errorf("endpoint %q has no scheme or no host", endpoint)
	}

	port := parsed.Port()
	if port == "" {
		port = defaultPorts[scheme]
	}
	if port == "" {
		return "", fmt.Errorf("endpoint %q names no port and scheme %q has no default port", endpoint, scheme)
	}

	return scheme + "://" + net.JoinHostPort(host, port) + strings.TrimRight(parsed.Path, "/"), nil
}

// RDBMSIdentity is the identity of a relational backend, host:port/database.
// The host is lower case. The database name keeps its case, because
// PostgreSQL distinguishes it.
func RDBMSIdentity(host string, port int32, database string) string {
	return net.JoinHostPort(strings.ToLower(host), strconv.Itoa(int(port))) + "/" + database
}
