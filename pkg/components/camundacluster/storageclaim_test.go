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
	"path/filepath"
	"strings"
	"testing"

	"github.com/sourcehawk/operator-component-framework/pkg/testing/golden"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
)

func TestStorageClaimSchemaIsValid(t *testing.T) {
	require.NoError(t, StorageClaimSchema().Validate())
	assert.Equal(t, "camunda-storage-", StorageClaimSchema().Prefix)
}

func TestStorageClaimKey(t *testing.T) {
	cases := map[string]struct {
		storage Storage
		key     string
		wantErr bool
	}{
		"elasticsearch with an explicit port": {
			storage: Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "https://es.data.svc:9200"},
			},
			key: "elasticsearch|https://es.data.svc:9200",
		},
		"elasticsearch fills the default port, lowercases, and drops the trailing slash": {
			storage: Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "HTTPS://ES.Example.COM/"},
			},
			key: "elasticsearch|https://es.example.com:443",
		},
		"elasticsearch renders the port as a number": {
			storage: Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "https://es:09200"},
			},
			key: "elasticsearch|https://es:9200",
		},
		"elasticsearch drops every trailing slash": {
			storage: Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "https://es.example.com///"},
			},
			key: "elasticsearch|https://es.example.com:443",
		},
		"elasticsearch keeps the brackets of an IPv6 host": {
			storage: Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "https://[::1]:9200"},
			},
			key: "elasticsearch|https://[::1]:9200",
		},
		// Optimize connects to the host and the port of the endpoint and drops
		// the path, so two paths on one host and port are one Elasticsearch.
		"elasticsearch drops a path prefix": {
			storage: Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "http://proxy:8080/es/"},
			},
			key: "elasticsearch|http://proxy:8080",
		},
		"elasticsearch drops another path prefix on the same address": {
			storage: Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "http://proxy:8080/analytics"},
			},
			key: "elasticsearch|http://proxy:8080",
		},
		"rdbms": {
			storage: Storage{
				Type:  v1.SecondaryStorageTypeRDBMS,
				RDBMS: &RDBMSStorage{Host: "PG.data.svc", Port: 5432, Database: "camunda"},
			},
			key: "rdbms|pg.data.svc:5432/camunda",
		},
		"an endpoint that is no URL": {
			storage: Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "://bad"},
			},
			wantErr: true,
		},
		"an endpoint whose port is not a number": {
			storage: Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "https://es.data.svc:http"},
			},
			wantErr: true,
		},
		"a type without its block": {
			storage: Storage{Type: v1.SecondaryStorageTypeRDBMS},
			wantErr: true,
		},
		"an unknown type": {
			storage: Storage{Type: v1.SecondaryStorageType("mongodb")},
			wantErr: true,
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			key, err := StorageClaimKey(tc.storage)
			if tc.wantErr {
				require.Error(t, err)

				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.key, key)
		})
	}
}

func TestStorageClaimLeaseLabelsSelectOneCluster(t *testing.T) {
	set := StorageClaimLeaseLabels("orders")

	assert.Equal(t, labels.OwnerName("orders"), set[labels.ClusterKey])
	assert.Equal(t, StorageClaimComponent, set[labels.ComponentKey])
	assert.Equal(t, labels.ManagedBy, set[labels.ManagedByKey])
}

// The name, the labels and the annotations of a storage claim Lease are the
// protocol between two clusters that resolve one backend. A change of any of
// them lets a second cluster start beside the holder, which is the data loss
// the claim exists to prevent.
func TestNewStorageClaimLeaseMatchesTheGolden(t *testing.T) {
	cases := map[string]struct {
		file    string
		key     string
		cluster *v1.CamundaCluster
	}{
		"storageclaimlease": {
			file: "storageclaimlease.yaml",
			key:  "elasticsearch|https://es.data.svc:9200",
			cluster: &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{
				Namespace: "apps", Name: "orders", UID: "uid-1",
			}},
		},
		"a name that the bounds cut": {
			file: "storageclaimlease-longname.yaml",
			key:  "rdbms|pg.data.svc:5432/camunda",
			cluster: &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{
				Namespace: strings.Repeat("s", 60),
				Name:      strings.Repeat("n", 200),
				UID:       "uid-2",
			}},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			lease := StorageClaimSchema().NewLease("camunda-system", tc.key, tc.cluster)

			require.NotNil(t, lease.Spec.AcquireTime)
			golden.AssertYAML(
				t,
				filepath.Join("testdata", "golden", tc.file),
				leasePreview{lease: lease},
				golden.WithScheme(leaseGoldenScheme(t)),
				golden.Update(*updateGolden),
			)
		})
	}
}

// leasePreview renders a storage claim Lease for the golden comparison. The
// acquire time is the wall clock of the render, so it is zeroed.
type leasePreview struct {
	lease *coordinationv1.Lease
}

func (p leasePreview) Preview() (client.Object, error) {
	lease := p.lease.DeepCopy()
	lease.Spec.AcquireTime = nil

	return lease, nil
}

// leaseGoldenScheme serves the Lease that the golden serializer renders.
func leaseGoldenScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	return scheme
}
