# One CamundaCluster per Secondary Storage Backend Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make one `CamundaCluster` the only user of a secondary storage backend (Elasticsearch endpoint or relational database), park every later cluster on that backend as suspended with `Ready=False/StorageAlreadyAttached`, and resume it when the holder releases the backend (#166).

**Architecture:** A pure backend identity in `pkg/wrappers/secondarystorageconfig` (normalized endpoint, or `host:port/database`), computed from a contract through one reader-backed `ResolveBackend`. A new pre-check step in the CamundaCluster controller lists the clusters live, resolves each sibling's backend, and lets the oldest cluster hold the backend through a sibling helper that the Azure guard also moves onto. The cluster that yields records the holder on `Input.Storage.Holder`; the reconcile then forces `Effective.Suspend` on and writes `Ready=False/StorageAlreadyAttached` instead of the aggregate. A self-watch on `CamundaCluster` and the contract, DatabaseConfig watches enqueue every parked cluster, so handover needs no timer.

**Tech Stack:** Go 1.26, kubebuilder/controller-runtime, ocf v0.19.1, Ginkgo/Gomega (envtest suite in `internal/controller/camundacluster`), testify (unit tests), controller-runtime fake client.

**Spec:** `docs/superpowers/specs/2026-08-22-one-cluster-per-secondary-storage-design.md` — read it before the first task. Issue #166 carries the acceptance criteria.

**Tracking:** Single PR against `main` from branch `fix/one-cluster-per-secondary-storage`, `Fixes #166`. Orchestration state: `docs/superpowers/states/2026-08-22-one-cluster-per-secondary-storage-state.md`.

## Global Constraints

- Branch `fix/one-cluster-per-secondary-storage`, worktree `.claude/worktrees/one-cluster-per-secondary-storage`. Never commit to `main`.
- Load `how-we-write-go` before Go, `simple-english:simple-english` before any prose (GoDoc, docs, condition messages). Reason names follow the Optimize precedent: `StorageAlreadyAttached` mirrors `ClusterAlreadyAttached`.
- Commit subjects carry the issue: `fix(camundacluster): ... (#166)`.
- Every gate before the PR: `make setup-envtest`, `go test ./...`, `go -C api test ./...`, `make lint`, `make manifests generate` then `git status --porcelain config api` prints nothing, `go vet -tags=e2e ./test/e2e/`, `mkdocs build --strict`.
- Tests encode intent. Never change an existing test to make it pass; the existing Azure and suspend specs must keep passing unchanged.
- No new condition type. One new reason on `Ready`.
- The identity ignores credentials. Two users on one endpoint share one backend.
- A sibling whose contract does not resolve is skipped, never counted as a holder.
- The holder is the oldest cluster: `creationTimestamp`, then name. The holder never re-checks.

## Contracts

None. The work is one PR, executed sequentially.

## Conventions

- **Layout.** Pure identity code in `pkg/wrappers/secondarystorageconfig/backend.go` beside `esadmin.go`. Controller guard in `internal/controller/camundacluster/storage.go` beside `backup.go`. The shared sibling helper in `precheck.go`, with the other `resolver` helpers. Watch handlers in `watches.go`. Envtest specs in `internal/controller/camundacluster/storage_test.go`.
- **Naming.** `Backend`, `ResolveBackend`, `ElasticsearchIdentity`, `RDBMSIdentity` (exported, `pkg/`). `resolveStorageHolder`, `usesBackend`, `olderSibling`, `olderThan`, `storageHeld`, `enqueueParked`, `enqueueForBinding`, `addParked` (controller). `components.StorageHolder`, `Storage.Holder`. The reason is `v1.ReasonStorageAlreadyAttached`.
- **Vocabulary in prose.** "backend" is the thing the workloads connect to; "contract" is the `SecondaryStorageConfig`; "holder" is the cluster that uses the backend; a cluster that yields is "parked". "Suspended" is the ocf state of its workloads.
- **Messages.** The parked cluster's message: `CamundaCluster "<ns>/<name>" already uses <backend>; one CamundaCluster per secondary storage backend, so this cluster stays suspended until that one releases it`, where `<backend>` is `Backend.String()`: `Elasticsearch "https://…"` or `database "host:5432/camunda"`.
- **Reads.** The guard lists and resolves through `res.reader` (the `APIReader`). Watch handlers read the cache (`r.Client`).
- **Tests.** Envtest pairs are named `cc-a-…` (holder) and `cc-b-…` (parked) and created in that order, so the oldest-wins rule is deterministic whether or not the two share a creation second.

---

### Task 1: The reason and the field GoDoc

**Files:**
- Modify: `api/v1/camundacluster_types.go:52-70` (reason constants) and `:437-442` (`StorageRef` GoDoc)
- Regenerate: `config/crd/bases/core.camunda.io_camundaclusters.yaml` through `make manifests`

**Interfaces:**
- Produces: `v1.ReasonStorageAlreadyAttached = "StorageAlreadyAttached"`.

- [ ] **Step 1: Add the reason constant**

In `api/v1/camundacluster_types.go`, after the `ReasonRejected` block (line ~68) add:

```go
// ReasonStorageAlreadyAttached on Ready means that another CamundaCluster,
// created earlier, already uses the secondary storage backend that
// spec.storageRef resolves to: the same Elasticsearch endpoint, or the same
// database on the same server. One CamundaCluster uses one backend, because
// the index names and the tables are fixed, so two clusters on one backend
// write each other's data. The operator keeps this cluster suspended, with
// its volumes, until that cluster releases the backend, and then resumes it
// on its own. The message names the holder and the backend.
const ReasonStorageAlreadyAttached = "StorageAlreadyAttached"
```

- [ ] **Step 2: Extend the StorageRef GoDoc**

Replace the `StorageRef` comment at `api/v1/camundacluster_types.go:437-441` with:

```go
	// StorageRef names the SecondaryStorageConfig, in the namespace of this
	// cluster, that describes the secondary storage backend. Required on a
	// CamundaCluster, forbidden in a preset. One CamundaCluster uses one
	// backend: a cluster whose backend an older cluster already uses is
	// suspended with Ready reason StorageAlreadyAttached until that cluster
	// releases it.
```

- [ ] **Step 3: Regenerate and verify**

Run: `make manifests generate && go -C api build ./... && git status --porcelain config api`
Expected: the CRD yaml under `config/crd/bases/` changes (the `storageRef` description), nothing else is dirty after the regeneration.

- [ ] **Step 4: Commit**

```bash
git add api/v1/camundacluster_types.go config/crd/bases/
git commit -m "feat(api): add the StorageAlreadyAttached reason to CamundaCluster (#166)"
```

---

### Task 2: The backend identity

**Files:**
- Create: `pkg/wrappers/secondarystorageconfig/backend.go`
- Test: `pkg/wrappers/secondarystorageconfig/backend_test.go`

**Interfaces:**
- Produces:
  - `type Backend struct { Type v1.SecondaryStorageType; Identity string }` with `func (b Backend) String() string`
  - `func ElasticsearchIdentity(endpoint string) (string, error)`
  - `func RDBMSIdentity(host string, port int32, database string) string`
  - `func ResolveBackend(ctx context.Context, reader client.Reader, contract *v1.SecondaryStorageConfig) (Backend, *conditions.PreCheckFailure, error)`
- Consumes: `conditions.PreCheckFailure`, `v1.ReasonInvalidReference`.

- [ ] **Step 1: Write the failing unit tests**

Create `pkg/wrappers/secondarystorageconfig/backend_test.go` (copy the license header from `esadmin_test.go`):

```go
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
		"no scheme":     "es.example.com:9200",
		"no host":       "https://",
		"empty":         "",
		"unknown port":  "ldap://es.example.com",
		"not a URL":     "https://es.example.com:92 00",
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
		assert.Equal(t, Backend{Type: v1.SecondaryStorageTypeElasticsearch, Identity: "https://es.example.com:443"}, backend)
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
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./pkg/wrappers/secondarystorageconfig/ -run 'TestElasticsearchIdentity|TestRDBMSIdentity|TestBackendString|TestResolveBackend'`
Expected: FAIL, `undefined: ElasticsearchIdentity` (and the others).

- [ ] **Step 3: Write backend.go**

Create `pkg/wrappers/secondarystorageconfig/backend.go` (license header as in `esadmin.go`):

```go
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

// defaultPorts are the ports an endpoint URL implies when it names none.
var defaultPorts = map[string]string{"http": "80", "https": "443"}

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
		return Backend{}, invalidReference("SecondaryStorageConfig %s has unsupported type %q", key, contract.Spec.Type), nil
	}
}

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

func invalidReference(format string, args ...any) *conditions.PreCheckFailure {
	return &conditions.PreCheckFailure{Reason: v1.ReasonInvalidReference, Message: fmt.Sprintf(format, args...)}
}
```

Note: `url.Parse("es.example.com:9200")` yields scheme `es.example.com` and an empty host, which the scheme/host check rejects. `url.Parse("https://es.example.com:92 00")` errors on the space.

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./pkg/wrappers/secondarystorageconfig/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add pkg/wrappers/secondarystorageconfig/backend.go pkg/wrappers/secondarystorageconfig/backend_test.go
git commit -m "feat(secondarystorageconfig): resolve a contract to its backend identity (#166)"
```

---

### Task 3: The guard, the parked reconcile, and the envtest specs

**Files:**
- Modify: `pkg/components/camundacluster/input.go:27-38` (`Storage`)
- Modify: `internal/controller/camundacluster/precheck.go` (resolver field, steps, `olderSibling`/`olderThan`)
- Modify: `internal/controller/camundacluster/backup.go:105-151` (Azure guard onto `olderSibling`)
- Create: `internal/controller/camundacluster/storage.go`
- Modify: `internal/controller/camundacluster/controller.go:177-231` (forced suspend, Ready)
- Test: `internal/controller/camundacluster/storage_test.go`

**Interfaces:**
- Consumes: `secondarystorageconfig.ResolveBackend`, `Backend.String()`, `v1.ReasonStorageAlreadyAttached`.
- Produces: `components.StorageHolder{Cluster types.NamespacedName; Backend string}`, `components.Storage.Holder *StorageHolder`; resolver method `olderSibling(ctx, matches) (*v1.CamundaCluster, error)`; `storageHeld(cluster, holder) metav1.Condition`. Task 4 reads `Ready.Reason == v1.ReasonStorageAlreadyAttached` from status only.

- [ ] **Step 1: Write the failing envtest specs**

Create `internal/controller/camundacluster/storage_test.go` (license header as in `backup_test.go`; the imports you need are the ones `backup_test.go` uses plus `fixtures`):

```go
package camundacluster

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/internal/fixtures"
)

// newNamedCluster is newCluster with a name prefix, so a pair created in one
// second still has a deterministic holder: the oldest cluster holds the
// backend, with the name breaking a tie, and "cc-a-" sorts before "cc-b-".
func newNamedCluster(
	prefix, namespace string,
	cfg *v1.CamundaPlatformConfig,
	binding *v1.SecondaryStorageConfig,
) *v1.CamundaCluster {
	cluster := newCluster(namespace, cfg, binding)
	cluster.Name = prefix + utilrand.String(8)
	return cluster
}

// createRDBMSBinding creates a DatabaseConfig on server in namespace, with its
// credentials Secret, and an rdbms binding that names it.
func createRDBMSBinding(namespace string, server *v1.DatabaseServerConfig) *v1.SecondaryStorageConfig {
	GinkgoHelper()
	dbConfig := fixtures.DatabaseConfig()
	dbConfig.Namespace = namespace
	dbConfig.Spec.ServerRef = server.Name
	dbConfig.Spec.CredentialsSecretRef.Namespace = namespace
	Expect(k8sClient.Create(ctx, dbConfig)).To(Succeed())
	createSecret(namespace, dbConfig.Spec.CredentialsSecretRef.Name, map[string]string{
		"username": "camunda", "password": "db-password",
	})

	binding := &v1.SecondaryStorageConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "rdbms-" + utilrand.String(8), Namespace: namespace},
		Spec: v1.SecondaryStorageConfigSpec{
			Type:  v1.SecondaryStorageTypeRDBMS,
			RDBMS: &v1.RDBMSStorage{DatabaseConfigRef: dbConfig.Name},
		},
	}
	Expect(k8sClient.Create(ctx, binding)).To(Succeed())
	return binding
}

// expectParked polls until cluster reports StorageAlreadyAttached naming
// holder, and its broker StatefulSet exists with zero replicas.
func expectParked(cluster, holder *v1.CamundaCluster) {
	GinkgoHelper()
	expectReady(
		cluster,
		metav1.ConditionFalse,
		Equal(v1.ReasonStorageAlreadyAttached),
		ContainSubstring(holder.Namespace+"/"+holder.Name),
	)
	zeebeKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"}
	Eventually(func(g Gomega) {
		g.Expect(*fetchStatefulSet(zeebeKey).Spec.Replicas).To(BeZero())
	}, timeout, interval).Should(Succeed())
}

// expectHolds polls until cluster reports a Ready reason other than
// StorageAlreadyAttached, and its broker StatefulSet asks for one replica.
func expectHolds(cluster *v1.CamundaCluster) {
	GinkgoHelper()
	Eventually(func(g Gomega) {
		var latest v1.CamundaCluster
		g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
		ready := meta.FindStatusCondition(latest.Status.Conditions, v1.ConditionReady)
		g.Expect(ready).NotTo(BeNil())
		g.Expect(ready.Reason).NotTo(Equal(v1.ReasonStorageAlreadyAttached))
	}, timeout, interval).Should(Succeed())
	zeebeKey := client.ObjectKey{Namespace: cluster.Namespace, Name: cluster.Name + "-zeebe"}
	Eventually(func(g Gomega) {
		g.Expect(*fetchStatefulSet(zeebeKey).Spec.Replicas).To(Equal(int32(1)))
	}, timeout, interval).Should(Succeed())
}

var _ = Describe("CamundaCluster secondary storage backend", func() {
	// One CamundaCluster uses one backend: the index names and the tables are
	// fixed, so two clusters on one backend write each other's data. The
	// oldest cluster holds the backend; the other is suspended, with its
	// volumes, until the holder releases it.
	It("suspends the newer of two clusters on one Elasticsearch contract and names the holder", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)

		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		createCluster(holder)
		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)

		expectHolds(holder)
		expectParked(parked, holder)
		expectReady(
			parked,
			metav1.ConditionFalse,
			Equal(v1.ReasonStorageAlreadyAttached),
			ContainSubstring(binding.Spec.Elasticsearch.Endpoint),
		)
	})

	// The backend is the endpoint, not the contract: two contracts in two
	// namespaces that name one endpoint name one backend.
	It("suspends a cluster in another namespace whose contract names the same endpoint", func() {
		holderNS := newNamespace()
		holderBinding := createBinding(holderNS, true)
		holder := newNamedCluster("cc-a-", holderNS, createPlatformConfig(), holderBinding)
		createCluster(holder)

		parkedNS := newNamespace()
		parkedBinding := fixtures.SecondaryStorageConfigElasticsearch(parkedNS)
		parkedBinding.Spec.Elasticsearch.Endpoint = holderBinding.Spec.Elasticsearch.Endpoint
		Expect(k8sClient.Create(ctx, parkedBinding)).To(Succeed())
		createSecret(parkedNS, parkedBinding.Spec.Elasticsearch.CredentialsSecretRef.Name, map[string]string{
			"username": "camunda", "password": "es-password",
		})
		parked := newNamedCluster("cc-b-", parkedNS, createPlatformConfig(), parkedBinding)
		createCluster(parked)

		expectHolds(holder)
		expectParked(parked, holder)
	})

	It("suspends the newer of two clusters on one database", func() {
		server := fixtures.DatabaseServerConfig()
		Expect(k8sClient.Create(ctx, server)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, server) })

		holderNS := newNamespace()
		holder := newNamedCluster("cc-a-", holderNS, createPlatformConfig(), createRDBMSBinding(holderNS, server))
		createCluster(holder)
		parkedNS := newNamespace()
		parked := newNamedCluster("cc-b-", parkedNS, createPlatformConfig(), createRDBMSBinding(parkedNS, server))
		createCluster(parked)

		expectHolds(holder)
		expectParked(parked, holder)
		expectReady(
			parked,
			metav1.ConditionFalse,
			Equal(v1.ReasonStorageAlreadyAttached),
			ContainSubstring(server.Spec.Host+":5432/camunda"),
		)
	})

	It("lets two clusters on two endpoints both run", func() {
		ns := newNamespace()
		first := newNamedCluster("cc-a-", ns, createPlatformConfig(), createBinding(ns, true))
		createCluster(first)
		second := newNamedCluster("cc-b-", ns, createPlatformConfig(), createBinding(ns, true))
		createCluster(second)

		expectHolds(first)
		expectHolds(second)
	})
})
```

Note: `fixtures.SecondaryStorageConfigElasticsearch` gives every binding an endpoint of its own name, so "two endpoints" needs nothing beyond two bindings. The handover spec belongs to Task 4; it needs the watch.

- [ ] **Step 2: Run the new specs to verify they fail**

Run: `make setup-envtest` once, then `KUBEBUILDER_ASSETS="$(bin/setup-envtest use -p path)" go test ./internal/controller/camundacluster/ -ginkgo.focus "secondary storage backend" -count=1`
(`make test` prints the exact `KUBEBUILDER_ASSETS` form for this repo at `Makefile:96`; reuse it.)
Expected: FAIL. `expectParked` times out because the second cluster reports another reason.

- [ ] **Step 3: Add StorageHolder to the render input**

In `pkg/components/camundacluster/input.go`, add `"k8s.io/apimachinery/pkg/types"` to the imports and change `Storage`:

```go
// Storage is the resolved secondary storage binding, filled by the
// controller from the SecondaryStorageConfig that spec.storageRef names.
type Storage struct {
	// Type selects which of the two blocks is set.
	Type v1.SecondaryStorageType
	// Elasticsearch is set when Type is elasticsearch.
	Elasticsearch *v1.ElasticsearchStorage
	// RDBMS is set when Type is rdbms.
	RDBMS *RDBMSStorage
	// Holder is set when another CamundaCluster, created earlier, uses the
	// backend of this binding. One CamundaCluster uses one backend, so the
	// controller renders a cluster with a Holder suspended and reports the
	// holder on Ready. Nil when this cluster holds its backend.
	Holder *StorageHolder
}

// StorageHolder is the CamundaCluster that holds a secondary storage backend
// and the backend it holds, as Backend.String of the contract wrapper names
// it.
type StorageHolder struct {
	Cluster types.NamespacedName
	Backend string
}
```

- [ ] **Step 4: Move the sibling helper into precheck.go and add the contract to the resolver**

In `internal/controller/camundacluster/precheck.go`:

(a) Add a field to `resolver` (after `mirrors`):

```go
	// storage is the SecondaryStorageConfig that spec.storageRef names, set
	// by resolveStorage for the steps after it.
	storage *v1.SecondaryStorageConfig
```

(b) In `resolveStorage`, after `in.Storage.Type = binding.Spec.Type` add `res.storage = &binding`.

(c) In `preCheck`, insert `res.resolveStorageHolder,` into `steps` directly after `res.resolveStorage,`. Extend the `preCheck` GoDoc's order sentence: "... the storage binding and its chain, the holder of its backend, the object storage references."

(d) Add the shared helper below `objectKind` (at the end of the resolver helpers), and move `olderThan` from `backup.go` to sit under it:

```go
// olderSibling returns the oldest CamundaCluster other than res.cluster that
// was created before it (ties break by name) and that matches, or nil. The
// two guards that let the oldest cluster keep a resource only one cluster
// can use call it. The list is read live: a cached list can miss a sibling
// created a moment ago, and the rule exists to protect that sibling.
func (res *resolver) olderSibling(
	ctx context.Context,
	matches func(context.Context, *v1.CamundaCluster) (bool, error),
) (*v1.CamundaCluster, error) {
	var clusters v1.CamundaClusterList
	if err := res.reader.List(ctx, &clusters); err != nil {
		return nil, fmt.Errorf("listing the clusters: %w", err)
	}

	var oldest *v1.CamundaCluster
	for i := range clusters.Items {
		other := &clusters.Items[i]
		if other.UID == res.cluster.UID || !olderThan(other, res.cluster) {
			continue
		}
		if oldest != nil && !olderThan(other, oldest) {
			continue
		}

		ok, err := matches(ctx, other)
		if err != nil {
			return nil, err
		}
		if ok {
			oldest = other
		}
	}

	return oldest, nil
}

// olderThan reports whether a was created before b, with the name as the
// tie-break, so exactly one of two clusters ever yields.
func olderThan(a, b *v1.CamundaCluster) bool {
	if !a.CreationTimestamp.Equal(&b.CreationTimestamp) {
		return a.CreationTimestamp.Before(&b.CreationTimestamp)
	}
	return a.Name < b.Name
}
```

- [ ] **Step 5: Move the Azure guard onto the helper**

Replace `rejectSharedAzureContainer` in `internal/controller/camundacluster/backup.go:105-141` with the version below and delete `olderThan` from that file (it now lives in `precheck.go`):

```go
// rejectSharedAzureContainer rejects an Azure backup bucket that an older
// cluster already backs up to. The azure store has no container field: its
// base-path IS the container, so the per-cluster prefix that isolates two
// clusters on S3 and GCS does not exist there, and two clusters on one
// contract would write into one container. The oldest cluster keeps the
// contract (ties break by name), so creating a second cluster never breaks
// the first.
func (res *resolver) rejectSharedAzureContainer(ctx context.Context, bucket *v1.ObjectStorageConfig) error {
	if bucket.Spec.AzureBlob == nil {
		return nil
	}

	other, err := res.olderSibling(ctx, func(_ context.Context, other *v1.CamundaCluster) (bool, error) {
		return other.Spec.BackupStorageRef == bucket.Name, nil
	})
	if err != nil {
		return err
	}
	if other == nil {
		return nil
	}

	return &conditions.PreCheckFailure{
		Reason: v1.ReasonInvalidReference,
		Message: fmt.Sprintf(
			"ObjectStorageConfig %q is an Azure container that CamundaCluster %q already backs up "+
				"to; the azure store writes into one container per contract, so every cluster needs "+
				"a contract with its own container",
			bucket.Name, objectPath(client.ObjectKeyFromObject(other)),
		),
	}
}
```

- [ ] **Step 6: Write storage.go**

Create `internal/controller/camundacluster/storage.go` (license header as in `backup.go`):

```go
package camundacluster

import (
	"context"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	components "github.com/konsole-is/camunda-operator/pkg/components/camundacluster"
	"github.com/konsole-is/camunda-operator/pkg/conditions"
	"github.com/konsole-is/camunda-operator/pkg/wrappers/secondarystorageconfig"
)

// resolveStorageHolder finds the CamundaCluster that holds the backend of
// the storage binding when it is not this cluster, and records it on
// in.Storage.Holder. One CamundaCluster uses one backend: the index names of
// Elasticsearch and the tables of a database are fixed, so two clusters on
// one backend write each other's data. The oldest cluster holds the backend
// (ties break by name), so creating a second cluster never breaks the first.
// It needs res.storage from resolveStorage. A contract whose backend does
// not resolve fails the pre-check with InvalidReference.
func (res *resolver) resolveStorageHolder(ctx context.Context, in *components.Input) error {
	backend, failure, err := secondarystorageconfig.ResolveBackend(ctx, res.reader, res.storage)
	if err != nil {
		return fmt.Errorf(
			"resolving the backend of SecondaryStorageConfig %s: %w",
			client.ObjectKeyFromObject(res.storage), err,
		)
	}
	if failure != nil {
		return failure
	}

	holder, err := res.olderSibling(ctx, func(ctx context.Context, other *v1.CamundaCluster) (bool, error) {
		return res.usesBackend(ctx, other, backend)
	})
	if err != nil {
		return err
	}
	if holder == nil {
		return nil
	}

	in.Storage.Holder = &components.StorageHolder{
		Cluster: client.ObjectKeyFromObject(holder),
		Backend: backend.String(),
	}

	return nil
}

// usesBackend reports whether other resolves to backend. A cluster whose
// contract or chain does not resolve uses nothing: it cannot run either, and
// it checks this cluster again when its contract resolves.
func (res *resolver) usesBackend(
	ctx context.Context,
	other *v1.CamundaCluster,
	backend secondarystorageconfig.Backend,
) (bool, error) {
	var binding v1.SecondaryStorageConfig
	key := client.ObjectKey{Namespace: other.Namespace, Name: other.Spec.StorageRef}
	if err := res.reader.Get(ctx, key, &binding); err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}

		return false, fmt.Errorf("reading SecondaryStorageConfig %s: %w", key, err)
	}

	theirs, failure, err := secondarystorageconfig.ResolveBackend(ctx, res.reader, &binding)
	if err != nil {
		return false, err
	}

	return failure == nil && theirs == backend, nil
}

// storageHeld builds the Ready condition of a cluster whose backend another
// cluster holds.
func storageHeld(cluster *v1.CamundaCluster, holder *components.StorageHolder) metav1.Condition {
	return conditions.Ready(
		metav1.ConditionFalse,
		v1.ReasonStorageAlreadyAttached,
		fmt.Sprintf(
			"CamundaCluster %q already uses %s; one CamundaCluster per secondary storage backend, "+
				"so this cluster stays suspended until that one releases it",
			objectPath(holder.Cluster), holder.Backend,
		),
		cluster.Generation,
	)
}
```

`spec.storageRef` is required by CEL (`api/v1/camundacluster_types.go:527`), so `usesBackend` never does a `Get` with an empty name.

- [ ] **Step 7: Force the suspension and the Ready condition in the controller**

In `internal/controller/camundacluster/controller.go`, after the pre-check error handling (after the `if err != nil { return ctrl.Result{}, err }` that follows the failure branch, line ~192) insert:

```go
	// A cluster whose backend another cluster holds renders suspended: every
	// workload at zero and the volumes kept, until the holder releases it.
	// The suspension also idles the admin rotation and clears the management
	// binding, as a user suspension does.
	if in.Storage.Holder != nil {
		in.Effective.Suspend = true
	}
```

Replace the line `conditions.Stage(&cluster, conditions.Aggregate(&cluster, built.ready...))` (line ~228) with:

```go
	if in.Storage.Holder != nil {
		conditions.Stage(&cluster, storageHeld(&cluster, in.Storage.Holder))
	} else {
		conditions.Stage(&cluster, conditions.Aggregate(&cluster, built.ready...))
	}
```

Extend the `Reconcile` GoDoc (line ~118) with one sentence after "a failed pre-check reports its Ready reason and stops.": "A cluster whose backend an older cluster holds renders suspended and reports StorageAlreadyAttached instead of the aggregate."

- [ ] **Step 8: Run the package tests**

Run: `go build ./... && KUBEBUILDER_ASSETS="$(bin/setup-envtest use -p path)" go test ./internal/controller/camundacluster/ -count=1`
Expected: PASS, including the four new specs and the unchanged Azure spec "rejects a second cluster on one Azure contract, oldest wins" and the suspend spec.

If the second spec ("another namespace") flakes on the holder's `expectHolds`: the holder's first reconcile lists no sibling and proceeds; this is the expected path, not a race. If `expectParked` fails on the StatefulSet, read `ocf` `docs/primitives/statefulset.md:250`: a suspended component applies the StatefulSet with zero replicas, so the object exists.

- [ ] **Step 9: Commit**

```bash
git add pkg/components/camundacluster/input.go internal/controller/camundacluster/precheck.go \
  internal/controller/camundacluster/backup.go internal/controller/camundacluster/storage.go \
  internal/controller/camundacluster/controller.go internal/controller/camundacluster/storage_test.go
git commit -m "fix(camundacluster): let one cluster hold a secondary storage backend (#166)"
```

---

### Task 4: Handover watches

**Files:**
- Modify: `internal/controller/camundacluster/watches.go` (`enqueueInNamespace`, new `enqueueParked`, `enqueueForBinding`, `requestSet.addParked`, `SetupWithManager`)
- Test: `internal/controller/camundacluster/storage_test.go` (one more spec)

**Interfaces:**
- Consumes: `v1.ReasonStorageAlreadyAttached`, `StorageRefField`, `refindex.ObjectNamespacedName`.

- [ ] **Step 1: Write the failing handover spec**

Append inside the `Describe("CamundaCluster secondary storage backend", ...)` block of `storage_test.go`:

```go
	// Nothing but a watch on the clusters tells a parked cluster that its
	// holder went: its own watch reports events on itself only.
	It("resumes the parked cluster when the holder is deleted", func() {
		ns := newNamespace()
		binding := createBinding(ns, true)
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), binding)
		Expect(k8sClient.Create(ctx, holder)).To(Succeed())
		parked := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(parked)
		expectParked(parked, holder)

		Expect(k8sClient.Delete(ctx, holder)).To(Succeed())
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, client.ObjectKeyFromObject(holder), &v1.CamundaCluster{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue())
		}, timeout, interval).Should(Succeed())

		expectHolds(parked)
	})
```

Add `apierrors "k8s.io/apimachinery/pkg/api/errors"` to the test imports. The holder is created with `k8sClient.Create` directly, not `createCluster`, because the spec deletes it itself.

- [ ] **Step 2: Run it to verify it fails**

Run: `KUBEBUILDER_ASSETS="$(bin/setup-envtest use -p path)" go test ./internal/controller/camundacluster/ -ginkgo.focus "resumes the parked cluster" -count=1`
Expected: FAIL on `expectHolds(parked)`: without a watch nothing re-enqueues the parked cluster after the delete.

- [ ] **Step 3: Add the handlers**

In `internal/controller/camundacluster/watches.go`, add `"k8s.io/apimachinery/pkg/api/meta"` to the imports.

(a) Add to `requestSet`, under `addList`:

```go
// addParked adds every cluster that waits for the holder of its backend to
// release it: the clusters whose Ready reason is StorageAlreadyAttached. A
// list failure is logged and drops those requests.
func (s requestSet) addParked(ctx context.Context, c client.Client) {
	var clusters v1.CamundaClusterList
	if err := c.List(ctx, &clusters); err != nil {
		logf.FromContext(ctx).Error(err, "listing parked clusters for enqueue")
		return
	}
	for i := range clusters.Items {
		ready := meta.FindStatusCondition(clusters.Items[i].Status.Conditions, v1.ConditionReady)
		if ready != nil && ready.Reason == v1.ReasonStorageAlreadyAttached {
			s[client.ObjectKeyFromObject(&clusters.Items[i])] = struct{}{}
		}
	}
}
```

(b) Replace `enqueueInNamespace` with:

```go
// enqueueInNamespace maps a DatabaseConfig event to every cluster of its
// namespace, and to every parked cluster: a DatabaseConfig that names another
// server or database can release the backend a parked cluster waits for.
func (r *CamundaClusterReconciler) enqueueInNamespace() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		set := requestSet{}
		set.addList(ctx, r.Client, client.InNamespace(o.GetNamespace()))
		set.addParked(ctx, r.Client)
		return set.requests()
	})
}
```

(c) Add, directly under `enqueueInNamespace`:

```go
// enqueueForBinding maps a SecondaryStorageConfig event to every cluster
// bound to it, and to every parked cluster: a contract that names another
// endpoint can release the backend a parked cluster waits for.
func (r *CamundaClusterReconciler) enqueueForBinding() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		set := requestSet{}
		set.addList(ctx, r.Client, client.MatchingFields{StorageRefField: refindex.ObjectNamespacedName(o)})
		set.addParked(ctx, r.Client)
		return set.requests()
	})
}

// enqueueParked maps an event on one cluster to every parked cluster. A
// parked cluster resumes when its holder goes or names another backend, and
// its own watch reports events on itself only.
func (r *CamundaClusterReconciler) enqueueParked() handler.EventHandler {
	return handler.EnqueueRequestsFromMapFunc(func(ctx context.Context, o client.Object) []reconcile.Request {
		set := requestSet{}
		set.addParked(ctx, r.Client)
		delete(set, client.ObjectKeyFromObject(o))
		return set.requests()
	})
}
```

(d) In `SetupWithManager`, replace the `SecondaryStorageConfig` watch

```go
		Watches(
			&v1.SecondaryStorageConfig{},
			refindex.Enqueue(cached, clusters, StorageRefField, refindex.ObjectNamespacedName),
		).
```

with

```go
		Watches(&v1.SecondaryStorageConfig{}, r.enqueueForBinding()).
```

and add, directly before `.Watches(&corev1.PersistentVolumeClaim{}, ...)`:

```go
		Watches(&v1.CamundaCluster{}, r.enqueueParked()).
```

Extend the `SetupWithManager` GoDoc: after "DatabaseServerConfigs for every cluster," insert "the clusters themselves for the parked clusters that wait for a holder (enqueueParked, which enqueueForBinding and enqueueInNamespace also serve),".

- [ ] **Step 4: Run the package tests**

Run: `KUBEBUILDER_ASSETS="$(bin/setup-envtest use -p path)" go test ./internal/controller/camundacluster/ -count=1`
Expected: PASS, all specs including the handover one.

- [ ] **Step 5: Commit**

```bash
git add internal/controller/camundacluster/watches.go internal/controller/camundacluster/storage_test.go
git commit -m "fix(camundacluster): resume a parked cluster when its holder releases the backend (#166)"
```

---

### Task 5: The words

**Files:**
- Modify: `pkg/components/elasticsearchcluster/snapshotstorage.go:308-311`
- Modify: `docs/crds/camundacluster.md` (new section before `## Secondary storage over TLS`, status table row, `## Suspend and pause`)
- Modify: `docs/crds/secondarystorageconfig.md:12` (Consumers row)
- Modify: `docs/crds/logicalrestoreelasticsearch.md:135`
- Modify: `docs/guides/secondary-storage.md` (new section after `## Choose a backend`, and the bring-your-own section)
- Modify: `docs/guides/backup.md:34`

Load `simple-english:simple-english` first. Classify all of this as descriptive text: 25 words per sentence, no "should", no semicolons in prose.

- [ ] **Step 1: Correct the repository comment**

In `pkg/components/elasticsearchcluster/snapshotstorage.go:308-311`, replace

```go
// RepositoryConfig returns the settings of the snapshot repository that the
// operator registers for the cluster in Elasticsearch. Every cluster writes
// under its own prefix of the shared bucket, so two clusters that reference
// the same contract never share a repository. The credentials are not part of
// it: they reach the nodes through the keystore.
```

with

```go
// RepositoryConfig returns the settings of the snapshot repository that the
// operator registers for the cluster in Elasticsearch. Every
// ElasticsearchCluster writes under its own prefix of the shared bucket, so
// two ElasticsearchClusters that reference the same bucket contract never
// share a repository. One CamundaCluster uses one Elasticsearch, so the
// repository is that cluster's alone. The credentials are not part of it:
// they reach the nodes through the keystore.
```

- [ ] **Step 2: docs/crds/camundacluster.md**

(a) Insert before `## Secondary storage over TLS` (line 63):

```markdown
## Secondary storage

`spec.storageRef` names the `SecondaryStorageConfig` in the namespace of the cluster. The contract tells the cluster where its backend is. The backend is the Elasticsearch endpoint, or the database on the server, that the workloads connect to.

One `CamundaCluster` uses one backend. Camunda fixes the index names and the tables, so two clusters on one backend write each other's data, and a restore of one deletes the data of the other. The API server accepts a second cluster whose contract resolves to a backend that an older cluster uses. The operator keeps the oldest cluster on the backend, with the name breaking a tie. Every other cluster is suspended: every workload at zero, the volumes kept, `Ready` `False` with reason `StorageAlreadyAttached`, and a message that names the holder and the backend. When the holder is deleted, changes its `storageRef`, or its contract names another endpoint, the suspended cluster resumes on its own.

Two contracts count as one backend when they resolve to the same endpoint, in one namespace or in two, whoever wrote them. The comparison ignores the credentials.
```

(b) In the status table (line ~137), add after the `Ready` / `Suspended` row:

```markdown
| `Ready` | `StorageAlreadyAttached` | Another `CamundaCluster`, created earlier, uses the backend that `storageRef` resolves to. This cluster is suspended. | Give this cluster a backend of its own, or delete the holder. The message names both. |
```

(c) In `## Suspend and pause`, after the first paragraph add:

```markdown
The operator also suspends a cluster whose backend another cluster holds, see [Secondary storage](#secondary-storage). `spec.suspend` stays yours. That suspension shows in the `Ready` reason `StorageAlreadyAttached` and ends when the holder releases the backend.
```

- [ ] **Step 3: docs/crds/secondarystorageconfig.md**

Replace the Consumers row (line 12) with:

```markdown
| Consumers | [CamundaCluster](camundacluster.md) (through `storageRef`, one cluster per backend, see [Secondary storage](camundacluster.md#secondary-storage)), [LogicalBackupElasticsearch](logicalbackupelasticsearch.md) and [LogicalBackupRDBMS](logicalbackuprdbms.md) (through the `storageRef` of the cluster they back up) |
```

- [ ] **Step 4: docs/crds/logicalrestoreelasticsearch.md**

Replace the first sentence of the `## Secondary storage` section (line 135) with:

```markdown
The operator deletes the Camunda indices of the target first, then asks Elasticsearch to restore every snapshot of the backup. One `CamundaCluster` uses one Elasticsearch, see [Secondary storage](camundacluster.md#secondary-storage), so every Camunda index on that Elasticsearch belongs to the target.
```

Keep the rest of the paragraph ("It names the Optimize indices only when ...") as it is.

- [ ] **Step 5: docs/guides/secondary-storage.md**

(a) Insert after the last paragraph of `## Choose a backend` (after "If your organization already runs PostgreSQL ..."):

```markdown
## One cluster per backend

A backend belongs to one `CamundaCluster`. Camunda fixes the index names in Elasticsearch and the tables in a database, so two clusters on one backend write each other's data. Give every cluster its own `ElasticsearchCluster` or its own `Database`. Two clusters can share one PostgreSQL server, each with its own database.

If a second cluster resolves to a backend that an older cluster uses, the operator suspends the second cluster. Its `Ready` condition reads `False` with reason `StorageAlreadyAttached` and names the holder. It resumes on its own when the holder releases the backend. The [CamundaCluster reference](../crds/camundacluster.md#secondary-storage) has the rule in full.
```

(b) In `## Bring your own backend`, after "The `CamundaCluster` does not know who created them." add:

```markdown
The rule of [one cluster per backend](#one-cluster-per-backend) holds for a hand-written contract as well. Two contracts that name one endpoint, in one namespace or in two, name one backend.
```

- [ ] **Step 6: docs/guides/backup.md**

Replace line 34 with:

```markdown
`basePath` is a key prefix inside the bucket, without leading or trailing slashes. Every backup of a cluster lands under `<basePath>/<namespace>/<cluster>/`. Two clusters can share one bucket and never share one prefix. Azure Blob is the exception: the Zeebe backup store writes into the whole container. On Azure, create one container and one `ObjectStorageConfig` per cluster. The bucket is the only storage that two clusters share. The secondary storage backend belongs to one cluster, see [one cluster per backend](./secondary-storage.md#one-cluster-per-backend).
```

- [ ] **Step 7: Build the docs and run the Go gates touched**

Run: `mkdocs build --strict && go build ./... && make lint`
Expected: `mkdocs` exits 0 with no warning; lint reports 0 issues.

- [ ] **Step 8: Commit**

```bash
git add pkg/components/elasticsearchcluster/snapshotstorage.go docs/crds/camundacluster.md \
  docs/crds/secondarystorageconfig.md docs/crds/logicalrestoreelasticsearch.md \
  docs/guides/secondary-storage.md docs/guides/backup.md
git commit -m "docs: state that one CamundaCluster uses one secondary storage backend (#166)"
```

---

### Task 6: Gates and the pull request

**Files:** none new.

- [ ] **Step 1: Run every gate**

```bash
make setup-envtest
go test ./...
go -C api test ./...
make lint
make manifests generate && git status --porcelain config api
go vet -tags=e2e ./test/e2e/
mkdocs build --strict
```

Expected: every command exits 0; `git status --porcelain config api` prints nothing.

- [ ] **Step 2: Self-review the diff against `how-we-write-go` and `simple-english`**

Run: `git diff origin/main --stat && git diff origin/main -- '*.go' | grep -n '^+.*//' | head -80`
Read every added comment: no narration, no semicolon, no "should". Read every added godoc: a contract, not an algorithm.

- [ ] **Step 3: Open the PR**

Load `feature-dev-workflow:opening-a-pull-request` and follow it. Title: `fix(camundacluster): one cluster per secondary storage backend`. Body says `Fixes #166`, names the statements the change falsified and corrected (the `RepositoryConfig` comment, `backup.md:34`, `logicalrestoreelasticsearch.md`), and the out-of-scope item (same-named `ElasticsearchCluster` in two namespaces, to be filed).

---

## Self-review against the spec

- The rule (identity table): Task 2 (`ElasticsearchIdentity`, `RDBMSIdentity`, `ResolveBackend`).
- The guard (live list, skip unresolvable sibling, oldest wins, shared loop with Azure): Task 3 steps 4-6.
- The cluster that yields (forced suspension, truthful component conditions, `Ready` override, message): Task 3 step 7 and `storageHeld`.
- Handover (self-watch, contract watch, DatabaseConfig watch): Task 4.
- What the rule closes without code (comments and docs): Task 5.
- Out of scope items: named in the PR body (Task 6).
- Testing list of the spec: Task 2 (unit), Task 3 (four envtest specs), Task 4 (handover spec).
