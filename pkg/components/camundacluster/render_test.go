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
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/camundaconfig"
)

// newInput returns the doc's minimal cluster (my-cluster in my-cluster-ns,
// version 8.9.9) with an Elasticsearch binding, basic auth, and no license or
// registry, then applies mutate.
func newInput(t *testing.T, mutate func(*Input)) Input {
	t.Helper()

	cluster := &v1.CamundaCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-cluster",
			Namespace: "my-cluster-ns",
			UID:       "my-cluster-uid",
		},
		Spec: v1.CamundaClusterSpec{
			PlatformConfigRef: "my-platform-config",
			Version:           "8.9.9",
			StorageRef:        "my-storage-config",
		},
	}
	in := Input{
		Cluster:   cluster,
		Effective: NewEffective(MergeSpec(cluster.Spec, nil, nil)),
		Storage: Storage{
			Type: v1.SecondaryStorageTypeElasticsearch,
			Elasticsearch: &v1.ElasticsearchStorage{
				Endpoint: "https://es-http.my-cluster-ns.svc:9200",
				CredentialsSecretRef: v1.LocalCredentialsSecretRef{
					Name:        "es-user",
					UsernameKey: "username",
					PasswordKey: "password",
				},
			},
		},
	}
	if mutate != nil {
		mutate(&in)
	}

	// The controller sets the claim after it took it, so every rendered pod
	// carries a real Lease name. A fixture that replaces Storage is covered
	// because this runs after mutate.
	key, err := StorageClaimKey(in.Storage)
	require.NoError(t, err)
	in.Storage.Claim = StorageClaimSchema().LeaseName(key)

	return in
}

// oidcPlatform is a platform config with OIDC and every discovery override.
func oidcPlatform() v1.CamundaPlatformConfigSpec {
	return v1.CamundaPlatformConfigSpec{
		Auth: &v1.PlatformAuthSpec{
			Method: v1.AuthenticationMethodOIDC,
			OIDC: &v1.OIDCSpec{
				IssuerURL: "https://idp.example.com/realms/camunda",
				JWKSURL:   "https://idp.internal/realms/camunda/protocol/openid-connect/certs",
				TokenURL:  "https://idp.internal/realms/camunda/protocol/openid-connect/token",
				AuthURL:   "https://idp.example.com/realms/camunda/protocol/openid-connect/auth",
				ClientID:  "platform-client",
				ClientSecretRef: v1.SecretKeyRef{
					Name: "platform-oidc", Namespace: "camunda-system", Key: "client-secret",
				},
			},
		},
	}
}

func envByName(env []corev1.EnvVar, name string) (corev1.EnvVar, bool) {
	for _, e := range env {
		if e.Name == name {
			return e, true
		}
	}
	return corev1.EnvVar{}, false
}

func assertEnv(t *testing.T, env []corev1.EnvVar, name, value string) {
	t.Helper()

	got, ok := envByName(env, name)
	require.True(t, ok, "env %s missing", name)
	assert.Equal(t, value, got.Value, name)
	assert.Nil(t, got.ValueFrom, "%s must be a literal", name)
}

func assertSecretEnv(t *testing.T, env []corev1.EnvVar, name, secret, key string) {
	t.Helper()

	got, ok := envByName(env, name)
	require.True(t, ok, "env %s missing", name)
	require.NotNil(t, got.ValueFrom, "%s must come from a source", name)
	require.NotNil(t, got.ValueFrom.SecretKeyRef, "%s must come from a Secret", name)
	assert.Equal(t, secret, got.ValueFrom.SecretKeyRef.Name, name)
	assert.Equal(t, key, got.ValueFrom.SecretKeyRef.Key, name)
}

func assertNoEnv(t *testing.T, env []corev1.EnvVar, name string) {
	t.Helper()

	_, ok := envByName(env, name)
	assert.False(t, ok, "env %s must be absent", name)
}

// process returns the process of the named component from Resolve.
func process(t *testing.T, in Input, component string) Process {
	t.Helper()

	for _, p := range Resolve(in.Effective) {
		if p.Component == component {
			return p
		}
	}
	require.Failf(t, "process missing", "no process for component %s", component)
	return Process{}
}

func TestRenderZeebeIdentity(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Effective = NewEffective(v1.CamundaClusterSpec{Zeebe: &v1.ZeebeSpec{
			WorkloadSpec:      v1.WorkloadSpec{Replicas: new(int32(3))},
			Partitions:        new(int32(3)),
			ReplicationFactor: new(int32(3)),
		}})
	})
	r := render(in, process(t, in, ComponentZeebe))

	assertEnv(t, r.env, "CAMUNDA_CLUSTER_NAME", "my-cluster")
	assertEnv(t, r.env, "CAMUNDA_CLUSTER_SIZE", "3")
	assertEnv(t, r.env, "CAMUNDA_CLUSTER_PARTITIONCOUNT", "3")
	assertEnv(t, r.env, "CAMUNDA_CLUSTER_REPLICATIONFACTOR", "3")
	assertEnv(
		t, r.env, "CAMUNDA_CLUSTER_INITIALCONTACTPOINTS",
		"my-cluster-zeebe-0.my-cluster-zeebe.my-cluster-ns.svc:26502,"+
			"my-cluster-zeebe-1.my-cluster-zeebe.my-cluster-ns.svc:26502,"+
			"my-cluster-zeebe-2.my-cluster-zeebe.my-cluster-ns.svc:26502",
	)
	assertEnv(t, r.env, "CAMUNDA_API_GRPC_ADDRESS", "0.0.0.0")
	assertEnv(t, r.env, "SPRING_PROFILES_ACTIVE", "broker,consolidated-auth")
	assertEnv(t, r.env, "ZEEBE_BROKER_GATEWAY_ENABLE", "false")
	assert.Equal(
		t,
		[]string{"bash", "-c", "export CAMUNDA_CLUSTER_NODEID=${HOSTNAME##*-}; exec /usr/local/camunda/bin/camunda"},
		r.command,
	)
	assertNoEnv(t, r.env, "CAMUNDA_CLUSTER_NODEID")
	assertNoEnv(t, r.env, "CAMUNDA_CLUSTER_GATEWAYID")
	assertNoEnv(t, r.env, "CAMUNDA_MODE")
}

func TestRenderEmbeddedGateway(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Effective = NewEffective(v1.CamundaClusterSpec{Gateway: &v1.GatewaySpec{Mode: v1.ComponentModeEmbedded}})
	})
	r := render(in, process(t, in, ComponentZeebe))

	assertEnv(t, r.env, "SPRING_PROFILES_ACTIVE", "admin,broker,consolidated-auth,operate,tasklist")
	assertEnv(t, r.env, "ZEEBE_BROKER_GATEWAY_ENABLE", "true")
}

func TestRenderGatewayIdentity(t *testing.T) {
	t.Parallel()

	in := newInput(t, nil)
	r := render(in, process(t, in, ComponentGateway))

	gatewayID, ok := envByName(r.env, "CAMUNDA_CLUSTER_GATEWAYID")
	require.True(t, ok)
	require.NotNil(t, gatewayID.ValueFrom)
	require.NotNil(t, gatewayID.ValueFrom.FieldRef)
	assert.Equal(t, "metadata.name", gatewayID.ValueFrom.FieldRef.FieldPath)

	assertEnv(t, r.env, "CAMUNDA_CLUSTER_NAME", "my-cluster")
	assertEnv(
		t,
		r.env,
		"CAMUNDA_CLUSTER_INITIALCONTACTPOINTS",
		"my-cluster-zeebe-0.my-cluster-zeebe.my-cluster-ns.svc:26502",
	)
	assertEnv(t, r.env, "SPRING_PROFILES_ACTIVE", "admin,consolidated-auth,gateway,operate,tasklist")
	assertNoEnv(t, r.env, "CAMUNDA_CLUSTER_NODEID")
	assertNoEnv(t, r.env, "ZEEBE_BROKER_GATEWAY_ENABLE")
	assert.Nil(t, r.command)
}

func TestRenderElasticsearch(t *testing.T) {
	t.Parallel()

	in := newInput(t, nil)
	r := render(in, process(t, in, ComponentZeebe))

	assertEnv(t, r.env, "CAMUNDA_DATA_SECONDARYSTORAGE_TYPE", "elasticsearch")
	assertEnv(t, r.env, "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_URL", "https://es-http.my-cluster-ns.svc:9200")
	assertSecretEnv(t, r.env, "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_USERNAME", "es-user", "username")
	assertSecretEnv(t, r.env, "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_PASSWORD", "es-user", "password")
	assertNoEnv(t, r.env, "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_SECURITY_ENABLED")
	assert.Empty(t, r.volumes)
	assert.Empty(t, r.mounts)
}

func TestRenderElasticsearchWithCA(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Storage.Elasticsearch.CASecretRef = &v1.LocalSecretKeyRef{
			Name: "es-http-certs-public", Key: "ca.crt",
		}
	})
	r := render(in, process(t, in, ComponentGateway))

	assertEnv(t, r.env, "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_SECURITY_ENABLED", "true")
	assertEnv(
		t,
		r.env,
		"CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_SECURITY_CERTIFICATEPATH",
		"/etc/camunda/es-ca/ca.crt",
	)
	assertEnv(t, r.env, "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_SECURITY_VERIFYHOSTNAME", "true")
	assertEnv(t, r.env, "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_SECURITY_SELFSIGNED", "false")

	require.Len(t, r.volumes, 1)
	assert.Equal(t, "es-ca", r.volumes[0].Name)
	require.NotNil(t, r.volumes[0].Secret)
	assert.Equal(t, "es-http-certs-public", r.volumes[0].Secret.SecretName)
	assert.Equal(t, []corev1.KeyToPath{{Key: "ca.crt", Path: "ca.crt"}}, r.volumes[0].Secret.Items)

	require.Len(t, r.mounts, 1)
	assert.Equal(t, corev1.VolumeMount{Name: "es-ca", MountPath: "/etc/camunda/es-ca", ReadOnly: true}, r.mounts[0])
}

func TestRenderElasticsearchIndexReplicas(t *testing.T) {
	t.Parallel()

	const key = "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_NUMBEROFREPLICAS"
	cases := map[string]struct {
		nodeCount *int32
		requested *int32
		want      string
	}{
		"one node gets no replica":             {nodeCount: new(int32(1)), want: "0"},
		"three nodes get one replica":          {nodeCount: new(int32(3)), want: "1"},
		"the cluster setting wins":             {nodeCount: new(int32(1)), requested: new(int32(2)), want: "2"},
		"the cluster setting without nodes":    {requested: new(int32(0)), want: "0"},
		"no node count and no setting renders": {},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			in := newInput(t, func(in *Input) {
				in.Storage.Elasticsearch.NodeCount = tc.nodeCount
				in.Effective.IndexReplicas = tc.requested
			})
			r := render(in, process(t, in, ComponentZeebe))

			if tc.want == "" {
				assertNoEnv(t, r.env, key)
				return
			}
			assertEnv(t, r.env, key, tc.want)
		})
	}
}

func TestRenderElasticsearchIndexReplicasBesideLegacyKey(t *testing.T) {
	t.Parallel()

	const key = "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_NUMBEROFREPLICAS"
	legacyOnOperate := func(in *Input) {
		in.Effective.Operate = &v1.WebAppSpec{Mode: v1.ComponentModeStandalone}
		in.Effective.Operate.ExtraEnv = []corev1.EnvVar{
			{Name: "ZEEBE_BROKER_EXPORTERS_CAMUNDAEXPORTER_ARGS_INDEX_NUMBEROFREPLICAS", Value: "1"},
		}
	}
	cases := map[string]struct {
		mutate    func(in *Input)
		component string
		want      string
	}{
		"a legacy key in the global extraEnv keeps the default out": {
			mutate: func(in *Input) {
				in.Effective.ExtraEnv = []corev1.EnvVar{{Name: "CAMUNDA_DATABASE_INDEX_NUMBEROFREPLICAS", Value: "1"}}
			},
			component: ComponentZeebe,
		},
		"a legacy key on standalone Operate keeps the default out of Operate": {
			mutate:    legacyOnOperate,
			component: ComponentOperate,
		},
		"a legacy key on standalone Operate leaves the default on Zeebe": {
			mutate:    legacyOnOperate,
			component: ComponentZeebe,
			want:      "0",
		},
		"an explicit count renders beside a legacy key": {
			mutate: func(in *Input) {
				in.Effective.IndexReplicas = new(int32(0))
				in.Effective.ExtraEnv = []corev1.EnvVar{{Name: "CAMUNDA_DATABASE_INDEX_NUMBEROFREPLICAS", Value: "0"}}
			},
			component: ComponentZeebe,
			want:      "0",
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			in := newInput(t, func(in *Input) {
				in.Storage.Elasticsearch.NodeCount = new(int32(1))
				tc.mutate(in)
			})
			r := render(in, process(t, in, tc.component))

			if tc.want == "" {
				assertNoEnv(t, r.env, key)
				return
			}
			assertEnv(t, r.env, key, tc.want)
		})
	}
}

func TestRenderRDBMSIgnoresIndexReplicas(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Storage = Storage{
			Type: v1.SecondaryStorageTypeRDBMS,
			RDBMS: &RDBMSStorage{
				Host:        "pg.ns.svc",
				Port:        5432,
				Database:    "camunda",
				Credentials: v1.LocalCredentialsSecretRef{Name: "camunda-db", UsernameKey: "user", PasswordKey: "pass"},
			},
		}
		in.Effective.IndexReplicas = new(int32(1))
	})
	r := render(in, process(t, in, ComponentZeebe))

	assertNoEnv(t, r.env, "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_NUMBEROFREPLICAS")
}

func TestRenderRDBMS(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Storage = Storage{
			Type: v1.SecondaryStorageTypeRDBMS,
			RDBMS: &RDBMSStorage{
				Host:     "pg.ns.svc",
				Port:     5432,
				Database: "camunda",
				Credentials: v1.LocalCredentialsSecretRef{
					Name: "camunda-db", UsernameKey: "user", PasswordKey: "pass",
				},
			},
		}
	})
	r := render(in, process(t, in, ComponentZeebe))

	assertEnv(t, r.env, "CAMUNDA_DATA_SECONDARYSTORAGE_TYPE", "rdbms")
	assertEnv(t, r.env, "CAMUNDA_DATA_SECONDARYSTORAGE_RDBMS_URL", "jdbc:postgresql://pg.ns.svc:5432/camunda")
	assertSecretEnv(t, r.env, "CAMUNDA_DATA_SECONDARYSTORAGE_RDBMS_USERNAME", "camunda-db", "user")
	assertSecretEnv(t, r.env, "CAMUNDA_DATA_SECONDARYSTORAGE_RDBMS_PASSWORD", "camunda-db", "pass")
	assertEnv(t, r.env, "CAMUNDA_DATA_SECONDARYSTORAGE_RDBMS_DATABASEVENDORID", "postgresql")
	assertNoEnv(t, r.env, "CAMUNDA_DATA_SECONDARYSTORAGE_ELASTICSEARCH_URL")
}

func TestRenderBasicAuthSeedsAdmin(t *testing.T) {
	t.Parallel()

	in := newInput(t, nil)
	r := render(in, process(t, in, ComponentGateway))

	assertEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_METHOD", "basic")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_USERS_0_USERNAME", "admin")
	assertSecretEnv(
		t,
		r.env,
		"CAMUNDA_SECURITY_INITIALIZATION_USERS_0_PASSWORD",
		"my-cluster-camunda-admin",
		"password",
	)
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_USERS_0_NAME", "admin")
	// The address comes from the Secret, like the password: a changed
	// address then never touches a pod template.
	assertSecretEnv(
		t,
		r.env,
		"CAMUNDA_SECURITY_INITIALIZATION_USERS_0_EMAIL",
		"my-cluster-camunda-admin",
		"email",
	)
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_ADMIN_USERS_0", "admin")
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_ISSUERURI")
	assertNoEnv(t, r.env, "CAMUNDA_LICENSE_KEY")
}

func TestRenderOIDC(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Platform = oidcPlatform()
		in.Cluster.Spec.ExternalURL = "https://my-cluster.example.com"
		in.Effective = NewEffective(in.Cluster.Spec)
	})
	r := render(in, process(t, in, ComponentGateway))

	assertEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_METHOD", "oidc")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_ISSUERURI", "https://idp.example.com/realms/camunda")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_CLIENTID", "platform-client")
	assertSecretEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_CLIENTSECRET", "platform-oidc", "client-secret")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_AUDIENCES", "platform-client")
	assertEnv(
		t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_JWKSETURI",
		"https://idp.internal/realms/camunda/protocol/openid-connect/certs",
	)
	assertEnv(
		t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_TOKENURI",
		"https://idp.internal/realms/camunda/protocol/openid-connect/token",
	)
	assertEnv(
		t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_AUTHORIZATIONURI",
		"https://idp.example.com/realms/camunda/protocol/openid-connect/auth",
	)
	assertEnv(
		t,
		r.env,
		"CAMUNDA_SECURITY_AUTHENTICATION_OIDC_REDIRECTURI",
		"https://my-cluster.example.com/sso-callback",
	)
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_USERS_0_USERNAME")
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_ADMIN_USERS_0")
}

// Without discovery overrides and without externalUrl the endpoint keys and
// the redirect stay unset, so the binary derives them.
func TestRenderOIDCLeavesDiscoveryToTheBinary(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Platform = oidcPlatform()
		in.Platform.Auth.OIDC.JWKSURL = ""
		in.Platform.Auth.OIDC.TokenURL = ""
		in.Platform.Auth.OIDC.AuthURL = ""
		in.Platform.Auth.OIDC.Audience = "camunda-api"
	})
	r := render(in, process(t, in, ComponentGateway))

	assertEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_AUDIENCES", "camunda-api")
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_JWKSETURI")
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_TOKENURI")
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_AUTHORIZATIONURI")
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_REDIRECTURI")
}

func TestRenderOIDCClusterAuthWins(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Platform = oidcPlatform()
		in.Cluster.Spec.PresetRef = "medium"
		preset := &v1.CamundaClusterPresetSpec{
			Cluster: v1.CamundaClusterSpec{Auth: &v1.ClusterAuthSpec{
				ClientID: "preset-client",
				Audience: "preset-audience",
				ClientSecretRef: &v1.LocalSecretKeyRef{
					Name: "preset-oidc", Key: "secret",
				},
			}},
		}
		in.Effective = NewEffective(MergeSpec(in.Cluster.Spec, preset, nil))
	})
	r := render(in, process(t, in, ComponentGateway))

	assertEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_CLIENTID", "preset-client")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_AUDIENCES", "preset-audience")
	assertSecretEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_CLIENTSECRET", "preset-oidc", "secret")

	in.Cluster.Spec.Auth = &v1.ClusterAuthSpec{
		ClientID:        "cluster-client",
		ClientSecretRef: &v1.LocalSecretKeyRef{Name: "cluster-oidc", Key: "secret"},
	}
	preset := &v1.CamundaClusterPresetSpec{
		Cluster: v1.CamundaClusterSpec{Auth: &v1.ClusterAuthSpec{
			ClientID:        "preset-client",
			Audience:        "preset-audience",
			ClientSecretRef: &v1.LocalSecretKeyRef{Name: "preset-oidc", Key: "secret"},
		}},
	}
	in.Effective = NewEffective(MergeSpec(in.Cluster.Spec, preset, nil))
	r = render(in, process(t, in, ComponentGateway))

	assertEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_CLIENTID", "cluster-client")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_AUDIENCES", "cluster-client")
	assertSecretEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_CLIENTSECRET", "cluster-oidc", "secret")
}

// A client id owns its audience and its secret. The layer that sets the
// client id (platform config, preset, or cluster) gives the audience and the
// secret, and a lower layer never adds the values of its own client.
func TestResolveAuth(t *testing.T) {
	t.Parallel()

	basic := ResolveAuth(newInput(t, nil))
	assert.Equal(t, v1.AuthenticationMethodBasic, basic.Method)
	assert.Nil(t, basic.OIDC)

	presetSecret := &v1.LocalSecretKeyRef{Name: "preset-oidc", Key: "secret"}
	clusterSecret := &v1.LocalSecretKeyRef{Name: "cluster-oidc", Key: "secret"}
	platformRef := v1.SecretKeyRef{Name: "platform-oidc", Namespace: "camunda-system", Key: "client-secret"}
	presetRef := v1.SecretKeyRef{Name: "preset-oidc", Namespace: "my-cluster-ns", Key: "secret"}
	clusterRef := v1.SecretKeyRef{Name: "cluster-oidc", Namespace: "my-cluster-ns", Key: "secret"}

	tests := []struct {
		name         string
		preset       *v1.ClusterAuthSpec
		cluster      *v1.ClusterAuthSpec
		wantClientID string
		wantAudience string
		wantSecret   v1.SecretKeyRef
	}{
		{
			name:         "no preset or cluster auth uses the platform client",
			wantClientID: "platform-client",
			wantAudience: "platform-audience",
			wantSecret:   platformRef,
		},
		{
			name:         "a cluster client id replaces the platform client",
			cluster:      &v1.ClusterAuthSpec{ClientID: "C", ClientSecretRef: clusterSecret},
			wantClientID: "C",
			wantAudience: "C",
			wantSecret:   clusterRef,
		},
		{
			name:         "a cluster client id replaces the preset client id",
			preset:       &v1.ClusterAuthSpec{ClientID: "P", ClientSecretRef: presetSecret},
			cluster:      &v1.ClusterAuthSpec{ClientID: "C", ClientSecretRef: clusterSecret},
			wantClientID: "C",
			wantAudience: "C",
			wantSecret:   clusterRef,
		},
		{
			name:         "a cluster client id drops the audience of the preset client",
			preset:       &v1.ClusterAuthSpec{ClientID: "P", Audience: "PA", ClientSecretRef: presetSecret},
			cluster:      &v1.ClusterAuthSpec{ClientID: "C", ClientSecretRef: clusterSecret},
			wantClientID: "C",
			wantAudience: "C",
			wantSecret:   clusterRef,
		},
		{
			name:         "a cluster client id drops a preset audience that names no client",
			preset:       &v1.ClusterAuthSpec{Audience: "PA"},
			cluster:      &v1.ClusterAuthSpec{ClientID: "C", ClientSecretRef: clusterSecret},
			wantClientID: "C",
			wantAudience: "C",
			wantSecret:   clusterRef,
		},
		{
			name:         "a cluster client id keeps the audience of the cluster",
			preset:       &v1.ClusterAuthSpec{ClientID: "P", Audience: "PA", ClientSecretRef: presetSecret},
			cluster:      &v1.ClusterAuthSpec{ClientID: "C", Audience: "CA", ClientSecretRef: clusterSecret},
			wantClientID: "C",
			wantAudience: "CA",
			wantSecret:   clusterRef,
		},
		{
			name:         "a cluster client id without a secret never gets the secret of another client",
			preset:       &v1.ClusterAuthSpec{ClientID: "P", ClientSecretRef: presetSecret},
			cluster:      &v1.ClusterAuthSpec{ClientID: "C"},
			wantClientID: "C",
			wantAudience: "C",
		},
		{
			name:         "a preset client applies to a cluster without auth",
			preset:       &v1.ClusterAuthSpec{ClientID: "P", Audience: "PA", ClientSecretRef: presetSecret},
			wantClientID: "P",
			wantAudience: "PA",
			wantSecret:   presetRef,
		},
		{
			name:         "a cluster audience overrides the audience of the preset client",
			preset:       &v1.ClusterAuthSpec{ClientID: "P", ClientSecretRef: presetSecret},
			cluster:      &v1.ClusterAuthSpec{Audience: "CA"},
			wantClientID: "P",
			wantAudience: "CA",
			wantSecret:   presetRef,
		},
		{
			name:         "a cluster secret overrides the secret of the preset client",
			preset:       &v1.ClusterAuthSpec{ClientID: "P", ClientSecretRef: presetSecret},
			cluster:      &v1.ClusterAuthSpec{ClientSecretRef: clusterSecret},
			wantClientID: "P",
			wantAudience: "P",
			wantSecret:   clusterRef,
		},
		{
			name:         "a preset client id drops the audience and the secret of the platform client",
			preset:       &v1.ClusterAuthSpec{ClientID: "P"},
			wantClientID: "P",
			wantAudience: "P",
		},
		{
			name:         "without a client id the preset overrides the platform client field by field",
			preset:       &v1.ClusterAuthSpec{Audience: "PA", ClientSecretRef: presetSecret},
			wantClientID: "platform-client",
			wantAudience: "PA",
			wantSecret:   presetRef,
		},
		{
			name:         "without a client id the cluster overrides the platform client field by field",
			cluster:      &v1.ClusterAuthSpec{ClientSecretRef: clusterSecret},
			wantClientID: "platform-client",
			wantAudience: "platform-audience",
			wantSecret:   clusterRef,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			in := newInput(t, func(in *Input) {
				in.Platform = oidcPlatform()
				in.Platform.Auth.OIDC.Audience = "platform-audience"
				in.Cluster.Spec.Auth = tt.cluster.DeepCopy()
				preset := &v1.CamundaClusterPresetSpec{
					Cluster: v1.CamundaClusterSpec{Auth: tt.preset.DeepCopy()},
				}
				in.Effective = NewEffective(MergeSpec(in.Cluster.Spec, preset, nil))
			})
			got := ResolveAuth(in)

			assert.Equal(t, v1.AuthenticationMethodOIDC, got.Method)
			require.NotNil(t, got.OIDC)
			assert.Equal(t, tt.wantClientID, got.OIDC.ClientID)
			assert.Equal(t, tt.wantAudience, got.OIDC.Audience)
			assert.Equal(t, tt.wantSecret, got.OIDC.ClientSecretRef)
			assert.Equal(t, "https://idp.example.com/realms/camunda", got.OIDC.IssuerURL)
			assert.Equal(t, "platform-client", in.Platform.Auth.OIDC.ClientID, "the platform spec is not mutated")
		})
	}
}

func TestRenderLicenseAndRegistry(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Platform.LicenseSecretRef = &v1.SecretKeyRef{
			Name:      "camunda-license",
			Namespace: "camunda-system",
			Key:       "key",
		}
		in.Platform.Images = &v1.ImagesSpec{
			Camunda:    "registry.example.com/camunda/camunda/",
			Connectors: "registry.example.com/camunda/connectors-bundle",
		}
		in.Cluster.Spec.Connectors = &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"}
		in.Effective = NewEffective(in.Cluster.Spec)
	})
	r := render(in, process(t, in, ComponentZeebe))

	assertSecretEnv(t, r.env, "CAMUNDA_LICENSE_KEY", "camunda-license", "key")
	assert.Equal(t, "registry.example.com/camunda/camunda:8.9.9", Image(in, process(t, in, ComponentZeebe)))
	assert.Equal(
		t,
		"registry.example.com/camunda/connectors-bundle:8.9.7",
		Image(in, process(t, in, ComponentConnectors)),
	)

	in.Platform.Images = nil
	assert.Equal(t, "camunda/camunda:8.9.9", Image(in, process(t, in, ComponentGateway)))

	in.Images = &v1.ReleaseImagesSpec{
		Camunda:    "registry.example.com/camunda/camunda@sha256:" + strings.Repeat("a", 64),
		Connectors: "registry.example.com/camunda/connectors-bundle:8.9.7-patched",
	}
	assert.Equal(
		t,
		"registry.example.com/camunda/camunda@sha256:"+strings.Repeat("a", 64),
		Image(in, process(t, in, ComponentZeebe)),
		"a pinned image is pulled as it is",
	)
	assert.Equal(
		t,
		"registry.example.com/camunda/connectors-bundle:8.9.7-patched",
		Image(in, process(t, in, ComponentConnectors)),
	)
}

func TestRenderExtraEnvWinsByName(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Cluster.Spec.ExtraEnv = []corev1.EnvVar{
			{Name: "FOO", Value: "global"},
			{Name: "CAMUNDA_CLUSTER_NAME", Value: "x"},
		}
		in.Cluster.Spec.ExtraEnvFrom = []corev1.EnvFromSource{{Prefix: "GLOBAL_"}}
		in.Cluster.Spec.Zeebe = &v1.ZeebeSpec{WorkloadSpec: v1.WorkloadSpec{
			ExtraEnv:     []corev1.EnvVar{{Name: "FOO", Value: "zeebe"}},
			ExtraEnvFrom: []corev1.EnvFromSource{{Prefix: "ZEEBE_"}},
		}}
		in.Effective = NewEffective(in.Cluster.Spec)
	})
	r := render(in, process(t, in, ComponentZeebe))

	assertEnv(t, r.env, "FOO", "zeebe")
	assertEnv(t, r.env, "CAMUNDA_CLUSTER_NAME", "x")

	count := 0
	for _, e := range r.env {
		if e.Name == "FOO" || e.Name == "CAMUNDA_CLUSTER_NAME" {
			count++
		}
	}
	assert.Equal(t, 2, count, "one entry per name")
	assert.Equal(t, []corev1.EnvFromSource{{Prefix: "GLOBAL_"}, {Prefix: "ZEEBE_"}}, r.envFrom)

	gateway := render(in, process(t, in, ComponentGateway))
	assertEnv(t, gateway.env, "FOO", "global")
	assert.Equal(t, []corev1.EnvFromSource{{Prefix: "GLOBAL_"}}, gateway.envFrom)
}

// An embedded web application's extraEnv applies to its host process; the
// host's own entry wins over it.
func TestRenderEmbeddedAppEnvAppliesToHost(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Cluster.Spec.Operate = &v1.WebAppSpec{WorkloadSpec: v1.WorkloadSpec{
			ExtraEnv: []corev1.EnvVar{{Name: "FROM_OPERATE", Value: "yes"}, {Name: "SHARED", Value: "operate"}},
		}}
		in.Cluster.Spec.Gateway = &v1.GatewaySpec{WorkloadSpec: v1.WorkloadSpec{
			ExtraEnv: []corev1.EnvVar{{Name: "SHARED", Value: "gateway"}},
		}}
		in.Effective = NewEffective(in.Cluster.Spec)
	})

	gateway := render(in, process(t, in, ComponentGateway))
	assertEnv(t, gateway.env, "FROM_OPERATE", "yes")
	assertEnv(t, gateway.env, "SHARED", "gateway")

	zeebe := render(in, process(t, in, ComponentZeebe))
	assertNoEnv(t, zeebe.env, "FROM_OPERATE")
}

// With the gateway embedded, the gateway's extraEnv and extraEnvFrom apply
// to the brokers; the brokers' own entry wins over it.
func TestRenderEmbeddedGatewayEnvAppliesToZeebe(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Cluster.Spec.Gateway = &v1.GatewaySpec{
			Mode: v1.ComponentModeEmbedded,
			WorkloadSpec: v1.WorkloadSpec{
				ExtraEnv:     []corev1.EnvVar{{Name: "FROM_GATEWAY", Value: "yes"}, {Name: "SHARED", Value: "gateway"}},
				ExtraEnvFrom: []corev1.EnvFromSource{{Prefix: "GATEWAY_"}},
			},
		}
		in.Cluster.Spec.Zeebe = &v1.ZeebeSpec{WorkloadSpec: v1.WorkloadSpec{
			ExtraEnv: []corev1.EnvVar{{Name: "SHARED", Value: "zeebe"}},
		}}
		in.Effective = NewEffective(in.Cluster.Spec)
	})

	zeebe := render(in, process(t, in, ComponentZeebe))
	assertEnv(t, zeebe.env, "FROM_GATEWAY", "yes")
	assertEnv(t, zeebe.env, "SHARED", "zeebe")
	assert.Equal(t, []corev1.EnvFromSource{{Prefix: "GATEWAY_"}}, zeebe.envFrom)

	standalone := newInput(t, func(in *Input) {
		in.Cluster.Spec.Gateway = &v1.GatewaySpec{WorkloadSpec: v1.WorkloadSpec{
			ExtraEnv: []corev1.EnvVar{{Name: "FROM_GATEWAY", Value: "yes"}},
		}}
		in.Effective = NewEffective(in.Cluster.Spec)
	})
	assertNoEnv(t, render(standalone, process(t, standalone, ComponentZeebe)).env, "FROM_GATEWAY")
}

func TestRenderJavaToolOptions(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Cluster.Spec.Connectors = &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"}
		in.Effective = NewEffective(in.Cluster.Spec)
	})

	for _, component := range []string{ComponentZeebe, ComponentGateway} {
		r := render(in, process(t, in, component))
		assertEnv(t, r.env, "JAVA_TOOL_OPTIONS", "-XX:+ExitOnOutOfMemoryError")
	}
	connectors := render(in, process(t, in, ComponentConnectors))
	assertNoEnv(t, connectors.env, "JAVA_TOOL_OPTIONS")
}

func TestRenderConnectors(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Platform.LicenseSecretRef = &v1.SecretKeyRef{
			Name:      "camunda-license",
			Namespace: "camunda-system",
			Key:       "key",
		}
		in.Cluster.Spec.Connectors = &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"}
		in.Effective = NewEffective(in.Cluster.Spec)
	})
	r := render(in, process(t, in, ComponentConnectors))

	assertEnv(t, r.env, "CAMUNDA_CLIENT_MODE", "selfManaged")
	assertEnv(t, r.env, "CAMUNDA_CLIENT_GRPCADDRESS", "http://my-cluster-gateway.my-cluster-ns.svc:26500")
	assertEnv(t, r.env, "CAMUNDA_CLIENT_RESTADDRESS", "http://my-cluster-gateway.my-cluster-ns.svc:8080")
	assertEnv(t, r.env, "CAMUNDA_CLIENT_AUTH_METHOD", "basic")
	assertEnv(t, r.env, "CAMUNDA_CLIENT_AUTH_USERNAME", "admin")
	assertSecretEnv(t, r.env, "CAMUNDA_CLIENT_AUTH_PASSWORD", "my-cluster-camunda-admin", "password")
	assertSecretEnv(t, r.env, "CAMUNDA_LICENSE_KEY", "camunda-license", "key")
	assertNoEnv(t, r.env, "SPRING_PROFILES_ACTIVE")
	assertNoEnv(t, r.env, "CAMUNDA_CLUSTER_NAME")
	assertNoEnv(t, r.env, "CAMUNDA_DATA_SECONDARYSTORAGE_TYPE")
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_METHOD")
	assert.Nil(t, r.command)
	assert.Empty(t, r.volumes)
}

func TestRenderConnectorsOIDCAndEmbeddedGateway(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Platform = oidcPlatform()
		in.Cluster.Spec.Connectors = &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"}
		in.Cluster.Spec.Gateway = &v1.GatewaySpec{Mode: v1.ComponentModeEmbedded}
		in.Effective = NewEffective(in.Cluster.Spec)
	})
	r := render(in, process(t, in, ComponentConnectors))

	assertEnv(t, r.env, "CAMUNDA_CLIENT_GRPCADDRESS", "http://my-cluster-zeebe.my-cluster-ns.svc:26500")
	assertEnv(t, r.env, "CAMUNDA_CLIENT_RESTADDRESS", "http://my-cluster-zeebe.my-cluster-ns.svc:8080")
	assertEnv(t, r.env, "CAMUNDA_CLIENT_AUTH_METHOD", "oidc")
	assertEnv(t, r.env, "CAMUNDA_CLIENT_AUTH_CLIENTID", "platform-client")
	assertSecretEnv(t, r.env, "CAMUNDA_CLIENT_AUTH_CLIENTSECRET", "platform-oidc", "client-secret")
	assertEnv(t, r.env, "CAMUNDA_CLIENT_AUTH_ISSUERURL", "https://idp.example.com/realms/camunda")
	assertEnv(t, r.env, "CAMUNDA_CLIENT_AUTH_AUDIENCE", "platform-client")
	assertNoEnv(t, r.env, "CAMUNDA_CLIENT_AUTH_USERNAME")
	assertNoEnv(t, r.env, "CAMUNDA_LICENSE_KEY")
}

// Every CAMUNDA_, ZEEBE_, and SPRING_ variable of every process of every
// golden fixture is a declared key. This is the gate against configuration
// drift: a key that 8.9 does not read cannot leave the renderer.
func TestRenderOnlyDeclaredKeys(t *testing.T) {
	t.Parallel()

	for name, in := range goldenFixtures(t) {
		for _, p := range Resolve(in.Effective) {
			for _, e := range render(in, p).env {
				if !strings.HasPrefix(e.Name, "CAMUNDA_") &&
					!strings.HasPrefix(e.Name, "ZEEBE_") &&
					!strings.HasPrefix(e.Name, "SPRING_") {
					continue
				}
				assert.True(t, camundaconfig.IsDeclared(e.Name), "%s/%s: %s is not declared", name, p.Component, e.Name)
			}
		}
	}
}

func TestResolveAuthAdmin(t *testing.T) {
	t.Parallel()

	admin := &v1.ClusterAdminSpec{Clients: []string{"my-cluster-client"}}

	basic := ResolveAuth(newInput(t, func(in *Input) {
		in.Cluster.Spec.Auth = &v1.ClusterAuthSpec{Admin: admin}
		in.Effective = NewEffective(in.Cluster.Spec)
	}))
	assert.Nil(t, basic.Admin, "basic authentication seeds its own administrator")

	oidc := ResolveAuth(newInput(t, func(in *Input) {
		in.Platform = oidcPlatform()
		in.Cluster.Spec.Auth = &v1.ClusterAuthSpec{Admin: admin}
		in.Effective = NewEffective(in.Cluster.Spec)
	}))
	require.NotNil(t, oidc.Admin)
	assert.Equal(t, []string{"my-cluster-client"}, oidc.Admin.Clients)
}

func TestRenderOIDCClaims(t *testing.T) {
	t.Parallel()

	bare := newInput(t, func(in *Input) { in.Platform = oidcPlatform() })
	r := render(bare, process(t, bare, ComponentGateway))
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_USERNAMECLAIM")
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_CLIENTIDCLAIM")

	set := newInput(t, func(in *Input) {
		in.Platform = oidcPlatform()
		in.Platform.Auth.OIDC.UsernameClaim = "preferred_username"
		in.Platform.Auth.OIDC.ClientIDClaim = "client_id"
	})
	r = render(set, process(t, set, ComponentGateway))
	assertEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_USERNAMECLAIM", "preferred_username")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_AUTHENTICATION_OIDC_CLIENTIDCLAIM", "client_id")
}

func TestRenderOIDCAdminBootstrap(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Platform = oidcPlatform()
		in.Cluster.Spec.Auth = &v1.ClusterAuthSpec{Admin: &v1.ClusterAdminSpec{
			Users:   []string{"ada@example.com", "grace@example.com"},
			Clients: []string{"my-cluster-client"},
			MappingRules: []v1.AdminMappingRule{
				{ID: "platform-admins", ClaimName: "groups", ClaimValue: "camunda-admins"},
			},
		}}
		in.Effective = NewEffective(in.Cluster.Spec)
	})
	r := render(in, process(t, in, ComponentGateway))

	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_ADMIN_USERS_0", "ada@example.com")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_ADMIN_USERS_1", "grace@example.com")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_ADMIN_CLIENTS_0", "my-cluster-client")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_ADMIN_MAPPINGRULES_0", "platform-admins")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_MAPPINGRULES_0_MAPPINGRULEID", "platform-admins")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_MAPPINGRULES_0_CLAIMNAME", "groups")
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_MAPPINGRULES_0_CLAIMVALUE", "camunda-admins")
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_USERS_0_USERNAME")
}

// Basic authentication seeds its own administrator, so the block renders
// nothing and the seeded admin user stays untouched.
func TestRenderAdminBootstrapIgnoredUnderBasicAuth(t *testing.T) {
	t.Parallel()

	in := newInput(t, func(in *Input) {
		in.Cluster.Spec.Auth = &v1.ClusterAuthSpec{Admin: &v1.ClusterAdminSpec{
			Clients: []string{"my-cluster-client"},
		}}
		in.Effective = NewEffective(in.Cluster.Spec)
	})
	r := render(in, process(t, in, ComponentGateway))

	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_ADMIN_USERS_0", "admin")
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_ADMIN_CLIENTS_0")
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_MAPPINGRULES_0_MAPPINGRULEID")
}

func TestRenderConnectorsRoleGrant(t *testing.T) {
	t.Parallel()

	input := func(claim string, connectors bool) Input {
		return newInput(t, func(in *Input) {
			in.Platform = oidcPlatform()
			in.Platform.Auth.OIDC.ClientIDClaim = claim
			if connectors {
				in.Cluster.Spec.Connectors = &v1.ConnectorsSpec{Enabled: new(true), Version: "8.9.7"}
			}
			in.Effective = NewEffective(in.Cluster.Spec)
		})
	}

	in := input("client_id", true)
	r := render(in, process(t, in, ComponentGateway))
	assertEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_CONNECTORS_CLIENTS_0", "platform-client")

	in = input("", true)
	r = render(in, process(t, in, ComponentGateway))
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_CONNECTORS_CLIENTS_0")

	in = input("client_id", false)
	r = render(in, process(t, in, ComponentGateway))
	assertNoEnv(t, r.env, "CAMUNDA_SECURITY_INITIALIZATION_DEFAULTROLES_CONNECTORS_CLIENTS_0")
}

func TestResolveAuthBasic(t *testing.T) {
	t.Parallel()

	assert.Nil(t, ResolveAuth(newInput(t, nil)).Basic)

	block := &v1.BasicAuthSpec{PasswordRotation: "2026-08"}

	basic := ResolveAuth(newInput(t, func(in *Input) {
		in.Cluster.Spec.Auth = &v1.ClusterAuthSpec{Basic: block}
		in.Effective = NewEffective(in.Cluster.Spec)
	}))
	require.NotNil(t, basic.Basic)
	assert.Equal(t, "2026-08", basic.Basic.PasswordRotation)

	oidc := ResolveAuth(newInput(t, func(in *Input) {
		in.Platform = oidcPlatform()
		in.Cluster.Spec.Auth = &v1.ClusterAuthSpec{Basic: block}
		in.Effective = NewEffective(in.Cluster.Spec)
	}))
	assert.Nil(t, oidc.Basic, "the basic block has no effect under OIDC")
}
