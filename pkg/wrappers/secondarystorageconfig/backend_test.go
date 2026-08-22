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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
)

func TestElasticsearchIdentity(t *testing.T) {
	cases := map[string]struct {
		endpoint string
		want     string
	}{
		"lowercases the scheme and the host": {
			endpoint: "HTTPS://ES.Example.com:9200", want: "https://es.example.com:9200",
		},
		"adds the default https port": {
			endpoint: "https://es.example.com", want: "https://es.example.com:443",
		},
		"adds the default http port": {
			endpoint: "http://es.example.com", want: "http://es.example.com:80",
		},
		"drops a trailing slash": {
			endpoint: "https://es.example.com:9200/", want: "https://es.example.com:9200",
		},
		"keeps a path prefix without its trailing slash": {
			endpoint: "https://es.example.com:9200/search/", want: "https://es.example.com:9200/search",
		},
		"drops the query and the fragment": {
			endpoint: "https://es.example.com:9200/?pretty#top", want: "https://es.example.com:9200",
		},
		"keeps an IPv6 host bracketed": {
			endpoint: "https://[::1]:9200", want: "https://[::1]:9200",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := ElasticsearchIdentity(tc.endpoint)
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}

	for name, endpoint := range map[string]string{
		"no scheme":    "es.example.com:9200",
		"no host":      "https://",
		"empty":        "",
		"unknown port": "ldap://es.example.com",
		"not a URL":    "https://es.example.com:92 00",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := ElasticsearchIdentity(endpoint)
			assert.Error(t, err)
		})
	}
}

func TestRDBMSIdentity(t *testing.T) {
	assert.Equal(t, "pg.example.com:5432/Camunda", RDBMSIdentity("PG.Example.com", 5432, "Camunda"))
	assert.Equal(t, "[::1]:5432/camunda", RDBMSIdentity("::1", 5432, "camunda"))
}

func TestBackendString(t *testing.T) {
	assert.Equal(
		t,
		`Elasticsearch "https://es.example.com:9200"`,
		Backend{Type: v1.SecondaryStorageTypeElasticsearch, Identity: "https://es.example.com:9200"}.String(),
	)
	assert.Equal(
		t,
		`database "pg:5432/camunda"`,
		Backend{Type: v1.SecondaryStorageTypeRDBMS, Identity: "pg:5432/camunda"}.String(),
	)
}

func TestResolveBackend(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	server := &v1.DatabaseServerConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "pg"},
		Spec:       v1.DatabaseServerConfigSpec{Engine: v1.DatabaseEnginePostgres, Host: "PG.example.com", Port: 5432},
	}
	dbConfig := &v1.DatabaseConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "camunda-db", Namespace: "team-a"},
		Spec:       v1.DatabaseConfigSpec{ServerRef: "pg", DatabaseName: "camunda"},
	}
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(server, dbConfig).Build()

	contract := func(spec v1.SecondaryStorageConfigSpec) *v1.SecondaryStorageConfig {
		return &v1.SecondaryStorageConfig{
			ObjectMeta: metav1.ObjectMeta{Name: "storage", Namespace: "team-a"},
			Spec:       spec,
		}
	}

	t.Run("elasticsearch resolves to its normalized endpoint", func(t *testing.T) {
		backend, failure, err := ResolveBackend(context.Background(), reader, contract(v1.SecondaryStorageConfigSpec{
			Type:          v1.SecondaryStorageTypeElasticsearch,
			Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "https://ES.example.com/"},
		}))
		require.NoError(t, err)
		require.Nil(t, failure)
		assert.Equal(
			t,
			Backend{Type: v1.SecondaryStorageTypeElasticsearch, Identity: "https://es.example.com:443"},
			backend,
		)
	})

	t.Run("rdbms follows the chain to host, port, and database", func(t *testing.T) {
		backend, failure, err := ResolveBackend(context.Background(), reader, contract(v1.SecondaryStorageConfigSpec{
			Type:  v1.SecondaryStorageTypeRDBMS,
			RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: "camunda-db"},
		}))
		require.NoError(t, err)
		require.Nil(t, failure)
		assert.Equal(t, Backend{Type: v1.SecondaryStorageTypeRDBMS, Identity: "pg.example.com:5432/camunda"}, backend)
	})

	t.Run("a missing DatabaseConfig is an invalid reference", func(t *testing.T) {
		_, failure, err := ResolveBackend(context.Background(), reader, contract(v1.SecondaryStorageConfigSpec{
			Type:  v1.SecondaryStorageTypeRDBMS,
			RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: "missing"},
		}))
		require.NoError(t, err)
		require.NotNil(t, failure)
		assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
		assert.Contains(t, failure.Message, "team-a/missing")
	})

	t.Run("a contract without its block is an invalid reference", func(t *testing.T) {
		_, failure, err := ResolveBackend(context.Background(), reader, contract(v1.SecondaryStorageConfigSpec{
			Type: v1.SecondaryStorageTypeElasticsearch,
		}))
		require.NoError(t, err)
		require.NotNil(t, failure)
		assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
	})

	t.Run("an endpoint that is not a URL is an invalid reference", func(t *testing.T) {
		_, failure, err := ResolveBackend(context.Background(), reader, contract(v1.SecondaryStorageConfigSpec{
			Type:          v1.SecondaryStorageTypeElasticsearch,
			Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "es.example.com:9200"},
		}))
		require.NoError(t, err)
		require.NotNil(t, failure)
		assert.Equal(t, v1.ReasonInvalidReference, failure.Reason)
	})
}
