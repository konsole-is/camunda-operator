# Suspend Only For Shared Backends Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The operator stops a running workload only when it would write a backend another live cluster writes. The storage claim becomes a Lease keyed by the backend, and the scale-to-zero on a failed pre-check goes.

**Architecture:** `pkg/components/camundacluster` declares a `leaseclaim.Schema` for the storage claim and the function that turns a resolved chain into a backend key. The cluster controller takes the Lease after it resolves the chain, releases every other storage Lease it holds, and gates its render on pods of other clusters that carry the Lease name. The pre-check failure branch of the cluster and Optimize controllers stages `Ready` and returns, and `CamundaCluster.Suspended()` reports the two claim states only.

**Tech Stack:** Go 1.26, controller-runtime, `pkg/leaseclaim` (in-repo, #364), Ginkgo/Gomega envtest, testify, `pkg/testing/golden`.

**Spec:** `docs/superpowers/specs/2026-09-13-suspend-only-for-shared-backends-design.md`

## Global Constraints

- Load `how-we-write-go` before any Go edit, `writing-operator-docs` before any docs edit, `simple-english:simple-english` before any GoDoc or message edit. The repository rules in `CLAUDE.md` bind every task.
- Two Go modules: run `go test ./...` and `go -C api test ./...`. `make lint` must report 0 issues in both.
- Every test uses testify `assert` and `require`, never `t.Fatal`. Ginkgo only in the envtest suites.
- Wrap every `go test`, `make lint`, and envtest run in `flock /tmp/claude-1000/gates/lock` (see memory: serialize heavy gates).
- Commit subjects carry the sub-issue: `(#369)` for PR 1 tasks, `(#370)` for PR 2 tasks. End every commit message with the attribution lines the session reminder gives.
- The Lease name prefix is `camunda-storage-`. The annotation keys are `camunda.io/storage-claim-holder-namespace`, `camunda.io/storage-claim-holder-name`, `camunda.io/storage-claim-holder-uid`, `camunda.io/storage-claim-key`. The pod label is `camunda.io/storage-claim`. The pod UID label is `camunda.io/cluster-uid` (exists in `pkg/labels`).
- The claim key is `elasticsearch|<scheme>://<host>:<port><path>` or `rdbms|<host>:<port>/<database>`, exactly as `StorageClaimKey` in Task 1 renders it.
- No finalizer on `CamundaCluster`. No change to the Optimize attachment claim, the Database claim, or the realm claim.

## Contracts

The two PRs run in parallel off the feature branch. They share no code contract: PR 2 removes code that PR 1 does not touch, and PR 1 adds code that PR 2 does not read. Two test files carry hunks from both (`internal/controller/camundacluster/secondarystorage_test.go` and `internal/controller/camundaoptimize/controller_test.go`), so whichever sub-PR lands second merges the feature branch forward and resolves them. The assertion that a cluster keeps its Lease when its contract is deleted needs both PRs, so it is written in that merge-forward step, in the contract-deleted spec of PR 2.

| Name | Producer (issue) | Consumer (issue) | Shape | Realization |
| --- | --- | --- | --- | --- |
| `suspended-ready-reasons` | #370 | none in parallel | `api/v1.suspendedReadyReasons = [StorageAlreadyAttached, WaitingForHandover]` | data-only |

## Conventions

- Files keep the order `how-we-write-go` prescribes: a function is reached after the code that calls it.
- The new claim code of the cluster components lives in `pkg/components/camundacluster/storageclaim.go`, mirroring `pkg/components/database/claim.go`. The controller side stays in `internal/controller/camundacluster/secondarystorage.go`.
- The Lease name is called the "storage claim" in code, GoDoc, and docs. The key is called the "backend". The pod label carries the storage claim.
- Messages name the holder as `namespace/name` through `objectPath`, and the backend as the key string.
- Tests that need a Lease golden copy `leaseGoldenScheme` and `leasePreview` into their own `_test.go` file (test helpers stay in `_test.go`, see memory).
- Docs describe outcomes for the user, never reconcile steps. Show a status block where a reason changes.
- Deliberate, recorded at the 2026-09-14 checkpoint: the cluster does not use the `held` predicate of `workloadsuspend.KeepAtZero` because it lists its workloads by label, while Optimize reads one Deployment per component; one predicate over two list shapes would hide the difference. Optimize keeps its suspension vocabulary (`clustersuspension.go`) apart from the act of following it (`followsuspension.go`) because it carries two waits and three events; the cluster has one of each and keeps both in `explicitsuspend.go`.

---

## PR 1: the storage claim on a Lease (#369)

Branch: `feat/storage-claim-on-a-lease` off `fix/suspend-only-for-shared-backends`. Worktree: `.claude/worktrees/suspend-only-for-shared-backends/.claude/worktrees/storage-claim-on-a-lease`. PR targets `fix/suspend-only-for-shared-backends` with body line `Towards #369`.

### Task 1: The storage claim Schema and the backend key

**Files:**
- Create: `pkg/components/camundacluster/storageclaim.go`
- Create: `pkg/components/camundacluster/storageclaim_test.go`
- Create: `pkg/components/camundacluster/testdata/golden/storageclaimlease.yaml` (through `-update-golden`)
- Modify: `pkg/labels/labels.go:56-60` (rename the label constant)

**Interfaces:**
- Consumes: `leaseclaim.Schema[T]`, `labels.Managed`, `labels.Cluster`.
- Produces:
  - `const StorageClaimComponent = "storage-claim"`
  - `func StorageClaimSchema() leaseclaim.Schema[*v1.CamundaCluster]`
  - `func StorageClaimLeaseLabels(name string) map[string]string`
  - `func StorageClaimKey(storage Storage) (string, error)`
  - `labels.StorageClaimKey = "camunda.io/storage-claim"` (replaces `labels.StorageContractKey`)

- [ ] **Step 1: Rename the label constant**

In `pkg/labels/labels.go` replace the `StorageContractKey` block with:

```go
	// StorageClaimKey names the storage claim Lease of the backend that a pod
	// of a CamundaCluster, or of the CamundaOptimize attached to it, writes.
	// It is on the pod, never on the selector, so a change of backend rolls
	// the pods and the new ones carry the new value.
	StorageClaimKey = "camunda.io/storage-claim"
```

Run `grep -rn StorageContractKey --include=*.go .` and leave the two callers (`pkg/components/camundacluster/components.go`, `internal/controller/camundacluster/secondarystorage.go`) red for Task 2 and Task 3. Do not rename the label in the docs yet; Task 5 does that.

- [ ] **Step 2: Write the failing tests**

`pkg/components/camundacluster/storageclaim_test.go`:

```go
package camundacluster

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/testing/golden"
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
		"elasticsearch keeps a path prefix": {
			storage: Storage{
				Type:          v1.SecondaryStorageTypeElasticsearch,
				Elasticsearch: &v1.ElasticsearchStorage{Endpoint: "http://proxy:8080/es/"},
			},
			key: "elasticsearch|http://proxy:8080/es",
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
		"a type without its block": {
			storage: Storage{Type: v1.SecondaryStorageTypeRDBMS},
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

func leaseGoldenScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	require.NoError(t, coordinationv1.AddToScheme(scheme))

	return scheme
}
```

Check the name of the golden update flag in this package first: `grep -n "flag.Bool" pkg/components/camundacluster/*_test.go`. Use that variable in place of `updateGolden` if it differs. Copy `leaseGoldenScheme` from `pkg/components/database/claim_test.go` if its body differs from the one above.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `flock /tmp/claude-1000/gates/lock go test ./pkg/components/camundacluster/ -run 'TestStorageClaim|TestNewStorageClaimLease' 2>&1 | head -20`
Expected: compile errors naming `StorageClaimSchema`, `StorageClaimKey`, `StorageClaimLeaseLabels`.

- [ ] **Step 4: Write the implementation**

`pkg/components/camundacluster/storageclaim.go`:

```go
package camundacluster

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	v1 "github.com/konsole-is/camunda-operator/api/v1"
	"github.com/konsole-is/camunda-operator/pkg/labels"
	"github.com/konsole-is/camunda-operator/pkg/leaseclaim"
)

// StorageClaimComponent is the component label of the storage claim Leases.
const StorageClaimComponent = "storage-claim"

// storageClaimLeasePrefix starts the name of every storage claim Lease.
const storageClaimLeasePrefix = "camunda-storage-"

// The annotations of a storage claim Lease. The first three record the
// CamundaCluster that holds the backend, the fourth the backend key.
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
// the CamundaCluster named name.
func StorageClaimLeaseLabels(name string) map[string]string {
	return labels.Managed(labels.Cluster(name), StorageClaimComponent)
}

// StorageClaimKey returns the backend that storage addresses, as the key of
// its storage claim. Two contracts that name one address give one key.
//
// An Elasticsearch key is the type, then the endpoint with the scheme and
// the host in lower case, the port written out (80 for http, 443 for https
// when the URL names none), and no trailing slash on the path. An rdbms key
// is the type, then host, port, and database name, with the host in lower
// case.
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
			storage.Type, strings.ToLower(storage.RDBMS.Host), storage.RDBMS.Port, storage.RDBMS.Database,
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
		switch scheme {
		case "https":
			port = "443"
		default:
			port = "80"
		}
	}
	if _, err := strconv.Atoi(port); err != nil {
		return "", fmt.Errorf("the Elasticsearch endpoint %q has no numeric port: %w", endpoint, err)
	}

	return scheme + "://" + strings.ToLower(parsed.Hostname()) + ":" + port + strings.TrimSuffix(parsed.Path, "/"), nil
}
```

- [ ] **Step 5: Run the tests to verify they pass, and write the goldens**

Run: `flock /tmp/claude-1000/gates/lock go test ./pkg/components/camundacluster/ -run 'TestNewStorageClaimLease' -update-golden` (use the flag name found in Step 2).
Then: `flock /tmp/claude-1000/gates/lock go test ./pkg/components/camundacluster/ -run 'TestStorageClaim|TestNewStorageClaimLease' -v 2>&1 | tail -20`
Expected: PASS. Open `testdata/golden/storageclaimlease.yaml` and check that the name starts with `camunda-storage-`, the four annotations are there, and the labels carry `camunda.io/cluster: orders` and `camunda.io/component: storage-claim`.

- [ ] **Step 6: Commit**

```bash
git add pkg/components/camundacluster/storageclaim.go pkg/components/camundacluster/storageclaim_test.go pkg/components/camundacluster/testdata/golden/storageclaimlease*.yaml pkg/labels/labels.go
git commit -m "feat(camundacluster): declare the storage claim Schema and the backend key (#369)"
```

### Task 2: The pods carry the storage claim and the cluster UID

**Files:**
- Modify: `pkg/components/camundacluster/input.go:29-51` (`Storage` gains `Claim`, `StorageHolder` changes shape)
- Modify: `pkg/components/camundacluster/components.go:288-317` (`podTemplate`, `StoragePodLabels`)
- Modify: `pkg/components/camundacluster/mutations_test.go:65`
- Modify: `pkg/components/camundacluster/fixtures_test.go:86` and the other fixture builders (set `Storage.Claim`)
- Modify: `pkg/components/camundaoptimize/input.go:52-56` (`StorageContract` becomes `StorageClaim`, add `ClusterUID`)
- Modify: `pkg/components/camundaoptimize/components.go:180-190` (`podLabels`)
- Modify: golden manifests of both packages through `-update-golden`

**Interfaces:**
- Consumes: `labels.StorageClaimKey`, `labels.ClusterUIDKey` from Task 1.
- Produces:
  - `Storage.Claim string`: the name of the storage claim Lease. Set by the controller after `Take`.
  - `StorageHolder{Cluster types.NamespacedName; Backend string}`.
  - `func StoragePodLabels(cluster string, uid types.UID, claim string) map[string]string`
  - Optimize `Input.StorageClaim string`, `Input.ClusterUID types.UID`.

- [ ] **Step 1: Change the input types**

In `pkg/components/camundacluster/input.go` change the `Storage` and `StorageHolder` types to:

```go
// Storage is the resolved secondary storage of the cluster.
type Storage struct {
	// Type selects which of the two blocks is set.
	Type v1.SecondaryStorageType
	// Elasticsearch is set when Type is elasticsearch.
	Elasticsearch *v1.ElasticsearchStorage
	// RDBMS is set when Type is rdbms.
	RDBMS *RDBMSStorage
	// Claim is the name of the storage claim Lease of the backend. Every
	// pod carries it, so a cluster that takes the backend over finds the
	// pods that still write it. It is set once the controller took or was
	// refused the claim.
	Claim string
	// Holder is set when another CamundaCluster holds the storage claim of
	// the backend this cluster resolves. One CamundaCluster writes one
	// backend, so the controller renders a cluster with a Holder suspended
	// and reports the holder on Ready. Nil when this cluster holds the
	// claim.
	Holder *StorageHolder
}

// StorageHolder is the CamundaCluster that holds the storage claim of the
// backend this cluster resolves.
type StorageHolder struct {
	// Cluster is the namespace and name of the holder.
	Cluster types.NamespacedName
	// Backend is the claim key of the backend, see StorageClaimKey.
	Backend string
}
```

- [ ] **Step 2: Change the pod labels**

In `pkg/components/camundacluster/components.go` replace `StoragePodLabels` and its use in `podTemplate`:

```go
			Labels: labels.Merge(
				DerivedPodLabels(in.Backup, in.Documents),
				StoragePodLabels(in.Cluster.Name, in.Cluster.UID, in.Storage.Claim),
				discoveryLabels(in.Cluster, p.Component),
			),
```

```go
// StoragePodLabels returns the labels that find every pod that writes the
// backend of the storage claim named claim: the cluster name, the cluster
// UID, and the claim. A cluster that takes the backend over lists the pods
// of every other cluster by the claim, and tells its own pods apart by the
// UID. The name and the claim are bounded the way a label value demands.
func StoragePodLabels(cluster string, uid types.UID, claim string) map[string]string {
	return map[string]string{
		labels.ClusterKey:      labels.OwnerName(cluster),
		labels.ClusterUIDKey:   string(uid),
		labels.StorageClaimKey: labels.OwnerName(claim),
	}
}
```

Add `"k8s.io/apimachinery/pkg/types"` to the imports. Update `mutations_test.go:65` to the new call.

- [ ] **Step 3: Set the claim in the fixtures**

In `pkg/components/camundacluster/fixtures_test.go`, where each fixture builds `Storage`, set `Claim` from the key, so the golden manifests carry a real Lease name:

```go
	key, err := StorageClaimKey(in.Storage)
	require.NoError(t, err)
	in.Storage.Claim = StorageClaimSchema().LeaseName(key)
```

Put this in the one helper every fixture passes through (find it with `grep -n "Storage:" fixtures_test.go`). If the fixtures set `Storage` inline in several builders, add a small `withStorageClaim(t, in Input) Input` helper in `fixtures_test.go` and call it in each.

- [ ] **Step 4: Change the Optimize input and labels**

In `pkg/components/camundaoptimize/input.go` replace `StorageContract` with:

```go
	// StorageClaim is the name of the storage claim Lease of the backend
	// that the referenced cluster writes. It is the camunda.io/storage-claim
	// label value of the pods, so a cluster that takes the backend over
	// waits for the importer of the previous holder as it waits for the pods
	// of that cluster. It is always set.
	StorageClaim string
	// ClusterUID is the UID of the referenced cluster. The pods carry it
	// beside the storage claim, so the cluster tells its own pods from
	// those of another cluster on the same backend.
	ClusterUID types.UID
```

In `pkg/components/camundaoptimize/components.go` change `podLabels`:

```go
// podLabels returns the labels of the pods of a component: the discovery
// labels and the storage claim of the backend they write. The importer
// writes the analytics indices of that backend, so a cluster that takes the
// backend over finds these pods with the same selector as the pods of the
// previous holder. The label is on the pods, never on the selector, so a
// change of backend rolls them and the new ones carry the new value.
func podLabels(in Input, comp string) map[string]string {
	return labels.Merge(
		discoveryLabels(in, comp),
		clustercomponents.StoragePodLabels(in.ClusterName, in.ClusterUID, in.StorageClaim),
	)
}
```

In the Optimize fixtures (`grep -n "StorageContract" pkg/components/camundaoptimize/*_test.go`), set `StorageClaim` to `clustercomponents.StorageClaimSchema().LeaseName("elasticsearch|" + <the fixture endpoint normalized>)` and `ClusterUID` to a fixed UID such as `"cluster-uid"`.

- [ ] **Step 5: Regenerate the goldens and check the diff**

Run: `flock /tmp/claude-1000/gates/lock go test ./pkg/components/camundacluster/ ./pkg/components/camundaoptimize/ -update-golden` (flag name per package, check `grep -rn "flag.Bool" pkg/components/camundaoptimize/*_test.go`).
Then: `git diff --stat pkg/components/*/testdata/golden/ && git diff pkg/components/*/testdata/golden/ | grep '^[-+] ' | grep -v 'storage-contract\|storage-claim\|cluster-uid' | head`
Expected: the second command prints nothing. The only changed lines are the label rename and the added UID label.

- [ ] **Step 6: Run both packages**

Run: `flock /tmp/claude-1000/gates/lock go test ./pkg/components/camundacluster/ ./pkg/components/camundaoptimize/ 2>&1 | tail -5`
Expected: `ok` for both.

- [ ] **Step 7: Commit**

```bash
git add pkg/components/camundacluster pkg/components/camundaoptimize
git commit -m "feat(components): label the pods with the storage claim and the cluster UID (#369)"
```

### Task 3: The controller takes the claim on the Lease

**Files:**
- Modify: `internal/controller/camundacluster/controller.go:61-95` (add `ClaimNamespace`), the RBAC markers block, and `storageHeld` uses
- Modify: `internal/controller/camundacluster/watches.go` (`SetupWithManager` validates the namespace and the Schema)
- Modify: `internal/controller/camundacluster/secondarystorage.go` (rewrite `claimStorage`, `waitForHandover`, `storageHeld`; delete `staleHolder`, `holderPods`)
- Modify: `internal/controller/camundacluster/secondarystorage_test.go:51-265` (delete `TestStaleHolder`, `TestHolderPods`; add `TestOtherPodsOnClaim`; update `TestStorageHeld`)
- Modify: `internal/controller/camundacluster/precheck.go` (the `resolver` struct gains `claims`)
- Modify: `cmd/main.go:286-290`
- Modify: `internal/controller/camundacluster/suite_test.go:106-112`, `internal/controller/camundaoptimize/suite_test.go:65-71`
- Delete: `pkg/wrappers/secondarystorageconfig/claim.go`, `pkg/wrappers/secondarystorageconfig/claim_test.go`
- Modify: `config/rbac/role.yaml` only through `make manifests`

**Interfaces:**
- Consumes: `StorageClaimSchema`, `StorageClaimKey`, `StoragePodLabels` from Tasks 1 and 2; `leaseclaim.Claim.Take`, `Held`, `Release`, `LeaseName`.
- Produces: `CamundaClusterReconciler.ClaimNamespace string`, `resolver.claims *leaseclaim.Claim[*v1.CamundaCluster]`, `func (res *resolver) otherPodsOnClaim(ctx, claim string) ([]string, error)`.

- [ ] **Step 1: Write the failing unit test for the handover gate**

Replace `TestStaleHolder` and `TestHolderPods` in `secondarystorage_test.go` with:

```go
func TestOtherPodsOnClaim(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, clientgoscheme.AddToScheme(scheme))
	require.NoError(t, v1.AddToScheme(scheme))

	const claim = "camunda-storage-0123456789abcdef0123456789abcdef01234567"
	self := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Namespace: "team-a", Name: "holder", UID: "uid-1"}}
	pod := func(namespace, name string, podLabels map[string]string) *corev1.Pod {
		return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Labels: podLabels}}
	}

	cases := map[string]struct {
		objects []client.Object
		pods    []string
	}{
		"no pods": {},
		"pods of this cluster": {
			objects: []client.Object{
				pod("team-a", "holder-zeebe-0", components.StoragePodLabels("holder", "uid-1", claim)),
			},
		},
		"pods of a previous holder on the claim, sorted": {
			objects: []client.Object{
				pod("team-a", "old-zeebe-1", components.StoragePodLabels("old", "uid-old", claim)),
				pod("team-a", "old-zeebe-0", components.StoragePodLabels("old", "uid-old", claim)),
			},
			pods: []string{"old-zeebe-0", "old-zeebe-1"},
		},
		"the Optimize importer pod of a previous holder": {
			objects: []client.Object{
				pod("team-a", "old-optimize-importer-0", labels.Merge(
					components.StoragePodLabels("old", "uid-old", claim),
					map[string]string{labels.ComponentKey: optimizecomponents.ComponentImporter},
				)),
			},
			pods: []string{"old-optimize-importer-0"},
		},
		"pods of a same-named earlier cluster": {
			objects: []client.Object{
				pod("team-a", "holder-zeebe-0", components.StoragePodLabels("holder", "uid-0", claim)),
			},
			pods: []string{"holder-zeebe-0"},
		},
		"pods on another claim": {
			objects: []client.Object{
				pod("team-a", "old-zeebe-0", components.StoragePodLabels("old", "uid-old", "camunda-storage-other")),
			},
		},
		"pods in another namespace": {
			objects: []client.Object{
				pod("team-b", "old-zeebe-0", components.StoragePodLabels("old", "uid-old", claim)),
			},
		},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			res := &resolver{
				reader:  fake.NewClientBuilder().WithScheme(scheme).WithObjects(tc.objects...).Build(),
				cluster: self,
			}
			pods, err := res.otherPodsOnClaim(context.Background(), claim)
			require.NoError(t, err)
			assert.Equal(t, tc.pods, pods)
		})
	}
}
```

Update `TestStorageHeld` so the holder is `&components.StorageHolder{Cluster: ..., Backend: "elasticsearch|https://es:9200"}` and the message assertion contains the backend string instead of the contract path. Remove the `secondarystorageconfig` import from the test file.

- [ ] **Step 2: Run the test to verify it fails**

Run: `flock /tmp/claude-1000/gates/lock go test ./internal/controller/camundacluster/ -run 'TestOtherPodsOnClaim|TestStorageHeld' 2>&1 | head`
Expected: compile error, `otherPodsOnClaim` undefined and `StorageHolder` has no field `Contract`.

- [ ] **Step 3: Add `ClaimNamespace`, the RBAC marker, and the setup checks**

In `controller.go`, after `Metrics`, add:

```go
	// ClaimNamespace holds the storage claim Leases of every cluster. One
	// namespace for all of them, so two clusters in two namespaces that
	// resolve one backend meet on one Lease. SetupWithManager refuses an
	// empty value.
	ClaimNamespace string
```

Add to the RBAC markers of the controller:

```go
// +kubebuilder:rbac:groups=coordination.k8s.io,resources=leases,verbs=get;list;watch;create;update;patch;delete
```

In `watches.go` at the top of `SetupWithManager`, before the indexers:

```go
	if r.ClaimNamespace == "" {
		return errors.New("the namespace of the storage claim Leases is required")
	}
	if err := components.StorageClaimSchema().Validate(); err != nil {
		return fmt.Errorf("the storage claim Schema of CamundaCluster: %w", err)
	}
```

In `cmd/main.go` the cluster reconciler gains `ClaimNamespace: operatorNamespace,` (the same variable the Database reconciler uses at line 322). In both suite files the cluster reconciler gains `ClaimNamespace: "default",` with a one-line comment that envtest has that namespace from the start, as `internal/controller/database/suite_test.go:41-43` explains.

- [ ] **Step 4: Rewrite the claim step**

In `precheck.go`, the `resolver` struct gains `claims *leaseclaim.Claim[*v1.CamundaCluster]`, and `preCheck` sets it: `claims: components.StorageClaimSchema().NewClaim(r.Client, r.APIReader, r.ClaimNamespace),`.

In `secondarystorage.go`, replace `claimStorage`, `staleHolder`, `waitForHandover`, `holderPods` with:

```go
// claimStorage takes the storage claim of the backend that resolveStorage
// resolved, or records the cluster that holds it. The claim key is the
// backend, so two contracts that name one address meet on one Lease. The
// first CamundaCluster that takes the claim holds it while it exists; a
// holder that is gone is taken over. A live holder lands on
// in.Storage.Holder and the controller renders this cluster suspended. A
// cluster that holds the claim releases every other storage claim it
// holds, so a repoint frees the old backend, and then waits for the pods of
// other clusters that still write this one. It needs in.Storage from
// resolveStorage.
func (res *resolver) claimStorage(ctx context.Context, in *components.Input) error {
	key, err := components.StorageClaimKey(in.Storage)
	if err != nil {
		return &conditions.PreCheckFailure{
			Reason:  v1.ReasonInvalidReference,
			Message: fmt.Sprintf("SecondaryStorageConfig %q: %s", objectPath(client.ObjectKeyFromObject(res.storage)), err),
		}
	}
	in.Storage.Claim = res.claims.LeaseName(key)

	blocker, err := res.claims.Take(ctx, res.cluster, key)
	if err != nil {
		return err
	}
	if blocker != nil {
		if blocker.Foreign() {
			return &conditions.PreCheckFailure{
				Reason: v1.ReasonInvalidReference,
				Message: fmt.Sprintf(
					"Lease %s claims the backend %q and names no CamundaCluster. Delete it if nothing uses it",
					blocker.Lease, key,
				),
			}
		}
		in.Storage.Holder = &components.StorageHolder{
			Cluster: blocker.Holder.NamespacedName,
			Backend: key,
		}

		return nil
	}

	if err := res.releaseOtherClaims(ctx, in.Storage.Claim); err != nil {
		return err
	}

	return res.waitForHandover(ctx, in.Storage.Claim, key)
}

// releaseOtherClaims gives back every storage claim of the cluster except
// keep. A cluster that moved to another backend holds two claims until here,
// and the old one must go so the next cluster can take that backend.
func (res *resolver) releaseOtherClaims(ctx context.Context, keep string) error {
	leases, err := res.claims.Held(ctx, res.cluster)
	if err != nil {
		return err
	}
	for i := range leases {
		if leases[i].Name == keep {
			continue
		}
		if err := res.claims.Release(ctx, &leases[i]); err != nil {
			return err
		}
	}

	return nil
}

// waitForHandover fails with WaitingForHandover while a pod of another
// cluster still carries the storage claim: a deleted holder leaves its pods
// to the garbage collector, and a repointed one replaces them through a
// rollout. Nothing watches those pods for this cluster, so the failure is
// unwatched and the controller looks again on its timer.
//
// The list is read once, before the render. A previous holder that is
// pointed back at the backend in that moment can start pods next to this
// cluster, and it stops again as soon as it meets the claim.
func (res *resolver) waitForHandover(ctx context.Context, claim, key string) error {
	pods, err := res.otherPodsOnClaim(ctx, claim)
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		return nil
	}

	return conditions.NewUnwatchedFailure(
		v1.ReasonWaitingForHandover,
		fmt.Sprintf(
			"Pods of another CamundaCluster still write the backend %q: %s. This cluster starts when they are gone",
			key, strings.Join(pods, ", "),
		),
	)
}

// otherPodsOnClaim returns the sorted names of the pods of the namespace of
// the cluster that carry the storage claim and another cluster UID. The pods
// of a CamundaOptimize attached to another cluster carry the same labels,
// see pkg/components/camundaoptimize, and its importer writes the backend
// like a pod of that cluster.
func (res *resolver) otherPodsOnClaim(ctx context.Context, claim string) ([]string, error) {
	var pods corev1.PodList
	if err := res.reader.List(
		ctx,
		&pods,
		client.InNamespace(res.cluster.Namespace),
		client.MatchingLabels(map[string]string{labels.StorageClaimKey: labels.OwnerName(claim)}),
	); err != nil {
		return nil, fmt.Errorf("listing the pods on storage claim %q: %w", claim, err)
	}

	var names []string
	for i := range pods.Items {
		if pods.Items[i].Labels[labels.ClusterUIDKey] == string(res.cluster.UID) {
			continue
		}
		names = append(names, pods.Items[i].Name)
	}
	slices.Sort(names)

	return names, nil
}
```

Note that a foreign Lease is a `PreCheckFailure` with `InvalidReference`, as the Database claim reports it: the cluster renders nothing new and `Ready` names the Lease to delete. It is not `StorageAlreadyAttached`, because no cluster holds the backend and `Suspended()` must not report a stop that the render did not make. Keep `eventReasonStorageClaimed` and record the event after a successful `Take` only when `Take` created the Lease. `Take` does not say which, so record it when `res.claims.Holds` was false before the call: read `held, err := res.claims.Holds(ctx, key, res.cluster)` before `Take` and record the event when `!held` and the call succeeded. The message becomes `Claimed the backend %q through SecondaryStorageConfig %q`.

Rewrite `storageHeld`:

```go
// storageHeld builds the Ready condition of a cluster whose backend another
// cluster holds. When applyErr is set, the message carries it as the error
// of the last apply.
func storageHeld(cluster *v1.CamundaCluster, holder *components.StorageHolder, applyErr error) metav1.Condition {
	message := fmt.Sprintf(
		"CamundaCluster %q already writes the backend %q. One CamundaCluster writes one backend, "+
			"so this cluster stays suspended until that one releases it",
		objectPath(holder.Cluster), holder.Backend,
	)
	if applyErr != nil {
		message += fmt.Sprintf(". The last apply of the suspended workloads failed: %s", applyErr)
	}

	return conditions.Ready(metav1.ConditionFalse, v1.ReasonStorageAlreadyAttached, message, cluster.Generation)
}
```

Delete `pkg/wrappers/secondarystorageconfig/claim.go` and `claim_test.go`. Run `grep -rn "secondarystorageconfig\.\(Claim\|HolderOf\|Holder\b\|ClaimHolder\)" --include=*.go .` and fix every hit (the Optimize envtest at `internal/controller/camundaoptimize/controller_test.go:734-760` is one; Task 4 rewrites it, so leave that one until then).

- [ ] **Step 5: Run the unit tests and the manifests**

Run: `flock /tmp/claude-1000/gates/lock go test ./internal/controller/camundacluster/ -run 'TestOtherPodsOnClaim|TestStorageHeld' -v 2>&1 | tail -20`
Expected: PASS.
Run: `make manifests generate && git status --porcelain config api`
Expected: no output. If `config/rbac/role.yaml` changed, the Database marker did not already grant the verbs; keep the change and say so in the PR body.

- [ ] **Step 6: Commit**

```bash
git add -A internal/controller/camundacluster pkg/wrappers/secondarystorageconfig cmd/main.go internal/controller/camundaoptimize/suite_test.go config
git commit -m "feat(camundacluster): take the storage claim on a Lease keyed by the backend (#369)"
```

### Task 4: The envtest specs read the Lease

**Files:**
- Modify: `internal/controller/camundacluster/secondarystorage_test.go:278-360` (helpers) and `:358-640` (specs)
- Modify: `internal/controller/camundaoptimize/controller_test.go:734-786` (the handover spec builds a ghost Lease)
- Modify: `internal/controller/camundaoptimize/precheck.go:158` (`StorageClaim` and `ClusterUID` on the input)

**Interfaces:**
- Consumes: `components.StorageClaimSchema`, `components.StorageClaimKey`, `components.StoragePodLabels`.
- Produces: helpers `expectClaimedBy(cluster, binding)`, `createStoragePod(cluster, binding)`, `createStorageLeaseFor(namespace, key string, holder leaseclaim.Holder)`.

- [ ] **Step 1: Set the Optimize input**

In `internal/controller/camundaoptimize/precheck.go` replace `out.Input.StorageContract = binding.Name` with:

```go
	key, err := clustercomponents.StorageClaimKey(clustercomponents.Storage{
		Type:          v1.SecondaryStorageTypeElasticsearch,
		Elasticsearch: binding.Spec.Elasticsearch,
	})
	if err != nil {
		return out, &conditions.PreCheckFailure{
			Reason:  v1.ReasonInvalidReference,
			Message: fmt.Sprintf("SecondaryStorageConfig %q: %s", objectPath(client.ObjectKeyFromObject(binding)), err),
		}
	}
	out.Input.StorageClaim = clustercomponents.StorageClaimSchema().LeaseName(key)
	out.Input.ClusterUID = cluster.UID
```

Check that `objectPath` exists in the Optimize package (`grep -n "func objectPath" internal/controller/camundaoptimize/*.go`); use `client.ObjectKeyFromObject(binding).String()` if it does not. `resolveStorage` already rejects a non-Elasticsearch binding before this point, so `binding.Spec.Elasticsearch` is set.

- [ ] **Step 2: Rewrite the cluster envtest helpers**

`expectClaimedBy` reads the Lease:

```go
// expectClaimedBy polls until the storage claim of the backend that binding
// names records cluster.
func expectClaimedBy(binding *v1.SecondaryStorageConfig, cluster *v1.CamundaCluster) {
	GinkgoHelper()
	key := storageKeyOf(binding)
	Eventually(func(g Gomega) {
		var lease coordinationv1.Lease
		name := client.ObjectKey{Namespace: testClaimNamespace, Name: components.StorageClaimSchema().LeaseName(key)}
		g.Expect(k8sClient.Get(ctx, name, &lease)).To(Succeed())
		holder, ours := components.StorageClaimSchema().HolderOf(&lease)
		g.Expect(ours).To(BeTrue())
		g.Expect(holder.NamespacedName).To(Equal(client.ObjectKeyFromObject(cluster)))
		g.Expect(holder.UID).To(Equal(cluster.UID))
	}, timeout, interval).Should(Succeed())
}

// storageKeyOf returns the claim key of the backend that binding names. The
// test bindings are Elasticsearch ones.
func storageKeyOf(binding *v1.SecondaryStorageConfig) string {
	GinkgoHelper()
	key, err := components.StorageClaimKey(components.Storage{
		Type:          binding.Spec.Type,
		Elasticsearch: binding.Spec.Elasticsearch,
	})
	Expect(err).NotTo(HaveOccurred())
	return key
}
```

Add `const testClaimNamespace = "default"` to `suite_test.go` and use it in the reconciler wiring from Task 3.

`createStoragePod` carries the new labels:

```go
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cluster.Name + "-zeebe-0",
			Namespace: cluster.Namespace,
			Labels: components.StoragePodLabels(
				cluster.Name, cluster.UID, components.StorageClaimSchema().LeaseName(storageKeyOf(binding)),
			),
		},
```

`expectWaitingForHandover` asserts the message names the backend and the pod: replace `ContainSubstring(previous.Namespace+"/"+previous.Name)` with `ContainSubstring(storageKeyOf(binding))` and add a `binding` parameter. Update its callers.

`expectParked` keeps its shape; the message names the holder as before.

- [ ] **Step 3: Adjust the specs**

- `lets the first cluster hold a contract and suspends the second`, `resumes the parked cluster when the holder is deleted`, `waits for the pods of a deleted holder before it resumes`, `waits for the pods of a repointed holder before it resumes`, `resumes the parked cluster when the holder repoints its storageRef`, `parks a repointed cluster whose version is also refused`: pass with the helper changes. Where the repoint spec asserts `expectClaimedBy(other, holder)`, `other` is a second binding; give it a distinct endpoint in `createBinding` (add an `endpoint string` argument or a variant `createBindingAt(ns, endpoint)`) so its key differs.
- `lets two clusters on two contracts both run`: give the two bindings two endpoints. Add a sibling spec:

```go
	// Two contracts that name one address are one backend. The second
	// cluster parks on the claim of the first.
	It("parks the second cluster when two contracts name one address", func() {
		ns := newNamespace()
		first := newNamedCluster("cc-a-", ns, createPlatformConfig(), createBindingAt(ns, "https://shared.es:9200"))
		createCluster(first)
		expectHolds(first)

		binding := createBindingAt(ns, "https://SHARED.es:9200/")
		second := newNamedCluster("cc-b-", ns, createPlatformConfig(), binding)
		createCluster(second)
		expectParked(second, first)
		expectReady(second, metav1.ConditionFalse, Equal(v1.ReasonStorageAlreadyAttached),
			ContainSubstring("elasticsearch|https://shared.es:9200"))
	})
```

- `scales a running cluster to zero when its contract goes away, while the other keeps running`: change nothing but what the helper rename forces. PR 2 rewrites this spec, and the merge-forward step of whichever PR lands second adds that a third cluster on a contract with the same endpoint parks with `StorageAlreadyAttached` naming `second`.
- `takes over a claim whose holder does not exist`: create the ghost Lease instead of annotating the binding:

```go
// createStorageLeaseFor writes a storage claim Lease that records holder,
// the way a cluster that is gone leaves one behind.
func createStorageLeaseFor(key string, holder *v1.CamundaCluster) *coordinationv1.Lease {
	GinkgoHelper()
	lease := components.StorageClaimSchema().NewLease(testClaimNamespace, key, holder)
	Expect(k8sClient.Create(ctx, lease)).To(Succeed())
	return lease
}
```

with `holder := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "ghost", UID: "ghost-uid"}}`.
- `keeps a claim through an apply of the contract by its producer`: delete it. The claim no longer lives on the contract.
- Add a repoint spec that proves the release:

```go
	It("releases the old backend when the holder repoints and the new one resolves", func() {
		ns := newNamespace()
		old := createBindingAt(ns, "https://old.es:9200")
		holder := newNamedCluster("cc-a-", ns, createPlatformConfig(), old)
		createCluster(holder)
		expectClaimedBy(old, holder)

		other := createBindingAt(ns, "https://new.es:9200")
		updateCluster(holder, func(c *v1.CamundaCluster) { c.Spec.StorageRef = other.Name })
		expectClaimedBy(other, holder)
		Eventually(func(g Gomega) {
			var lease coordinationv1.Lease
			name := client.ObjectKey{Namespace: testClaimNamespace, Name: components.StorageClaimSchema().LeaseName(storageKeyOf(old))}
			g.Expect(apierrors.IsNotFound(k8sClient.Get(ctx, name, &lease))).To(BeTrue())
		}, timeout, interval).Should(Succeed(), "the claim of the old backend is released")
	})
```

- [ ] **Step 4: Rewrite the Optimize handover spec**

In `internal/controller/camundaoptimize/controller_test.go:734-760`, replace the annotation write and the pod with:

```go
			By("claiming the backend for a holder that is gone, with a pod it left behind")
			key, err := clustercomponents.StorageClaimKey(clustercomponents.Storage{
				Type: v1.SecondaryStorageTypeElasticsearch, Elasticsearch: binding.Spec.Elasticsearch,
			})
			Expect(err).NotTo(HaveOccurred())
			ghost := &v1.CamundaCluster{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: "ghost", UID: "ghost-uid"}}
			Expect(k8sClient.Create(ctx, clustercomponents.StorageClaimSchema().NewLease("default", key, ghost))).To(Succeed())
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "ghost-zeebe-0",
					Namespace: ns,
					Labels: clustercomponents.StoragePodLabels(
						"ghost", "ghost-uid", clustercomponents.StorageClaimSchema().LeaseName(key),
					),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "camunda", Image: "camunda/camunda:8.9.9"}},
				},
			}
```

Remove the `secondarystorageconfig` import if nothing else in the file uses it. The Optimize suite reconciler already got `ClaimNamespace: "default"` in Task 3.

- [ ] **Step 5: Run both envtest suites**

Run: `make setup-envtest` once, then `flock /tmp/claude-1000/gates/lock go test ./internal/controller/camundacluster/ ./internal/controller/camundaoptimize/ 2>&1 | tail -30`
Expected: `ok` for both. Read every failure before touching a test: a failing spec after this change is either a helper that still reads the annotations, or a real defect in Task 3.

- [ ] **Step 6: Commit**

```bash
git add internal/controller/camundacluster internal/controller/camundaoptimize
git commit -m "test(camundacluster): prove the storage claim on the Lease and across contracts (#369)"
```

### Task 5: Docs for the Lease claim

**Files:**
- Modify: `docs/crds/camundacluster.md:124-181` (Secondary storage section)
- Modify: `docs/crds/secondarystorageconfig.md:16-24` (The claim section)
- Modify: `docs/guides/secondary-storage.md:21-25` (One cluster per contract)
- Modify: `docs/crds/camundaoptimize.md:52` (the label paragraph)
- Modify: `docs/crds/camundacluster.md` reason table row `StorageAlreadyAttached` (line 321)

- [ ] **Step 1: Rewrite the Secondary storage section of the cluster reference**

Load `writing-operator-docs`. Replace the paragraphs from "One `CamundaCluster` uses one contract." through "Two hand-written contracts that name one Elasticsearch are not caught." with prose that says, in this order:

1. One `CamundaCluster` writes one backend. Camunda fixes the index names and the tables, so two clusters on one backend write each other's data, and a restore of one deletes the data of the other.
2. The operator claims the backend, not the contract. The claim is a Lease named `camunda-storage-<hash>` in the namespace of the operator, and the hash is of the address the contract resolves to: the Elasticsearch endpoint, or the host, port, and database of the PostgreSQL chain. Two contracts that name one address are one backend. The first cluster to take the claim holds it while it exists, so a cluster whose contract is deleted keeps its backend and keeps running.
3. A second cluster on a held backend is suspended: every workload at zero and the volumes kept. `Ready` is `False` with reason `StorageAlreadyAttached`, and the message names the holder and the backend. Keep the status block, with the new message text from `storageHeld`.
4. The suspended cluster looks again every 30 seconds. When the holder is deleted, the suspended cluster takes the claim. When the holder repoints to another backend, it releases this one once the new backend resolves. A paused holder keeps its claim.
5. The handover paragraph and its `WaitingForHandover` status block, with the new message text from `waitForHandover`.
6. The label paragraph: every pod carries `camunda.io/storage-claim` with the name of the Lease, and `camunda.io/cluster-uid`. Optimize pods carry both. Update the two `kubectl` commands to the new label; the Lease name is 56 characters, so the selector form always works, and the sentence about a cut name goes.
7. Keep the caution about pointing `storageRef` back during a handover, reworded for the backend.
8. End with: the operator compares addresses, not servers. Two contracts that reach one Elasticsearch through two hostnames are not caught. Give one backend one address.

Delete the "A recreated contract is a new claim" paragraph.

- [ ] **Step 2: Rewrite the claim section of the contract reference**

Replace the four paragraphs under `## The claim` in `docs/crds/secondarystorageconfig.md` with two: the claim belongs to the backend the contract resolves to, held on a Lease in the namespace of the operator, so two contracts on one address are one claim and a deleted contract does not free the backend; and a pointer to the cluster reference for the rule. Delete the sentence "The contract is the unit of the claim, not the endpoint" and its paragraph.

- [ ] **Step 3: Update the guide and the Optimize reference**

In `docs/guides/secondary-storage.md` rename the section to `## One cluster per backend`, keep the first paragraph, and rewrite the second: the operator holds one claim per backend address, a second cluster on a held backend reports `StorageAlreadyAttached`, and it resumes when the holder releases the backend. Drop "The operator compares contracts, not endpoints".

In `docs/crds/camundaoptimize.md:52` replace the label sentence: the pods carry `camunda.io/storage-claim` with the storage claim of the backend the cluster writes, and `camunda.io/cluster-uid`.

In the cluster reason table, the `StorageAlreadyAttached` row reads: "Another `CamundaCluster` holds the storage claim of the backend that `storageRef` resolves to. This cluster is suspended." and the action: "Give this cluster a backend of its own, or delete the holder. The message names both."

- [ ] **Step 4: Check the build and the wording**

Run: `mkdocs build --strict 2>&1 | tail -3` and `grep -rn "claim-holder\|storage-contract\|compares contracts" docs README.md`
Expected: a clean build, and the grep prints nothing.

- [ ] **Step 5: Commit**

```bash
git add docs
git commit -m "docs(camundacluster): describe the storage claim on the backend (#369)"
```

### Task 6: Gates and the PR for #369

- [ ] **Step 1: Run every gate**

```bash
flock /tmp/claude-1000/gates/lock go test ./... 2>&1 | grep -v '^ok' | head
go -C api test ./... 2>&1 | tail -3
flock /tmp/claude-1000/gates/lock make lint 2>&1 | tail -3
make manifests generate && git status --porcelain config api
go vet -tags=e2e ./test/e2e/
mkdocs build --strict 2>&1 | tail -2
```

Expected: no failing package, 0 lint issues, no porcelain output, no vet output, a clean build.

- [ ] **Step 2: Open the PR**

Load `feature-dev-workflow:opening-a-pull-request`. Push the branch and open the PR against `fix/suspend-only-for-shared-backends` with `Towards #369` in the body. The body names the Schema, the key, the label rename, the deleted wrapper claim, and whether `config/rbac/role.yaml` changed. Run the Copilot review loop to clean (`feature-dev-workflow:copilot-review-loop`, manual request per round since the base is not `main`). Self-merge into the feature branch, then `gh issue close 369`.

---

## PR 2: no suspension on a failed pre-check (#370)

Branch: `fix/keep-workloads-on-precheck-failure` off `fix/suspend-only-for-shared-backends`, in parallel with PR 1. Worktree: `.claude/worktrees/suspend-only-for-shared-backends/.claude/worktrees/keep-workloads-on-precheck-failure`. PR targets `fix/suspend-only-for-shared-backends` with `Towards #370`.

### Task 7: The cluster keeps its workloads on a failed pre-check

**Files:**
- Delete: `internal/controller/camundacluster/suspend.go`, `internal/controller/camundacluster/suspend_test.go`
- Modify: `internal/controller/camundacluster/controller.go:221-253` (the failure branch)
- Modify: `internal/controller/camundacluster/controller_test.go:562` (the credentials spec)
- Modify: `internal/controller/camundacluster/secondarystorage_test.go` (the contract-gone spec)

- [ ] **Step 1: Rewrite the failing envtest specs first**

In `controller_test.go`, rename `scales a running cluster to zero when its credentials Secret goes away, and resumes on its return` to `keeps a running cluster on its workloads when its credentials Secret goes away` and make it assert:

```go
		expectReady(cluster, metav1.ConditionFalse, Equal(v1.ReasonMissingSecret), ContainSubstring(secret.Name))
		Consistently(func(g Gomega) {
			g.Expect(*fetchStatefulSet(zeebeKey).Spec.Replicas).To(Equal(int32(1)))
		}, "3s", interval).Should(Succeed(), "the broker StatefulSet keeps its replicas")
		Eventually(func(g Gomega) {
			var latest v1.CamundaCluster
			g.Expect(k8sClient.Get(ctx, client.ObjectKeyFromObject(cluster), &latest)).To(Succeed())
			g.Expect(latest.Status.Gateway).NotTo(BeNil(), "a running cluster keeps publishing its endpoints")
			g.Expect(latest.Status.Management).NotTo(BeNil())
		}, timeout, interval).Should(Succeed())
```

and, after the Secret is recreated, that `Ready` leaves `MissingSecret` and the replicas are still 1. Drop every assertion on the `WorkloadsSuspended` event and on a `Suspended` per-process reason.

In `secondarystorage_test.go`, rename `scales a running cluster to zero when its contract goes away, while the other keeps running` to `keeps a running cluster on its workloads when its contract goes away` and replace the zero-replica and `Suspended` assertions with `Consistently` replicas at 1. Leave the claim assertions of the spec as they are on this branch. When this PR merges the feature branch forward after PR 1, add `expectClaimedBy(binding, second)` after the delete, reading the Lease that survives it (the Lease is in `testClaimNamespace`, not in `ns`, so it is not garbage collected with the contract), and the third-cluster-parks assertion from Task 4.

- [ ] **Step 2: Run the two specs to verify they fail**

Run: `flock /tmp/claude-1000/gates/lock go test ./internal/controller/camundacluster/ -ginkgo.focus 'keeps a running cluster' 2>&1 | tail -20`
Expected: FAIL on the replicas assertion (they go to zero).

- [ ] **Step 3: Rewrite the failure branch**

In `controller.go` replace the pre-check failure branch with:

```go
	in, mirrors, err := r.preCheck(ctx, &cluster)
	var failure *conditions.PreCheckFailure
	if errors.As(err, &failure) {
		// A running cluster keeps its workloads while its pre-check fails:
		// they run on the configuration of the last pass, and the storage
		// claim keeps its backend for it. Ready reports the failure.
		conditions.Stage(&cluster, conditions.Failed(&cluster, failure))

		// Only an unwatched failure needs a timer; everything else the
		// pre-checks resolve re-enqueues through a watch.
		var unwatched *conditions.UnwatchedPreCheckFailure
		if errors.As(err, &unwatched) {
			return ctrl.Result{RequeueAfter: r.retryInterval()}, nil
		}
		return ctrl.Result{}, nil
	}
```

Delete `suspend.go` and `suspend_test.go`. Run `grep -rn "eventReasonWorkloadsSuspended\|suspendWorkloads\|stageSuspension\|processConditions" internal/controller/camundacluster` and remove every leftover.

- [ ] **Step 4: Run the package**

Run: `flock /tmp/claude-1000/gates/lock go test ./internal/controller/camundacluster/ 2>&1 | tail -20`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add -A internal/controller/camundacluster
git commit -m "fix(camundacluster): keep the workloads running when a pre-check fails (#370)"
```

### Task 8: Optimize keeps its workloads on a failed pre-check

**Files:**
- Delete: `internal/controller/camundaoptimize/suspend.go` lines that belong to the pre-check suspension only; `internal/controller/camundaoptimize/suspend_test.go` tests of `suspendWorkloads` and `stageSuspension`
- Modify: `internal/controller/camundaoptimize/controller.go:171-200`
- Modify: `internal/controller/camundaoptimize/controller_test.go:788` and the `VersionMismatch` spec

**Interfaces:**
- Keeps: `recordSuspensionChange`, `wasSuspending`, the event constants they use (`eventActionSuspend`, `eventReasonClusterSuspended`, `eventReasonClusterResumed`, `noteSuspended`, `noteResumed`). Move them into `controller.go` or a file named for what they do (`clustersuspension.go`) so `suspend.go` can go.

- [ ] **Step 1: Rewrite the failing spec first**

In `controller_test.go`, rename `scales to zero when the storage contract of its cluster is deleted` to `keeps its workloads when the storage contract of its cluster is deleted` and assert `expectReplicas(1, webappKey, importerKey)` with a `Consistently` of 3 seconds after the delete, plus `Ready=False/InvalidReference` naming the contract. Find the `VersionMismatch` spec (`grep -n VersionMismatch controller_test.go`) and make it assert the replicas stay at 1 while the versions disagree.

- [ ] **Step 2: Run them to verify they fail**

Run: `flock /tmp/claude-1000/gates/lock go test ./internal/controller/camundaoptimize/ -ginkgo.focus 'keeps its workloads|VersionMismatch' 2>&1 | tail -20`
Expected: FAIL on the replicas.

- [ ] **Step 3: Rewrite the failure branch**

In `controller.go` the branch becomes:

```go
	res, err := r.preCheck(ctx, &optimize)
	var failure *conditions.PreCheckFailure
	if errors.As(err, &failure) {
		conditions.Stage(&optimize, conditions.Failed(&optimize, failure))
		// A CamundaOptimize that lost the attachment must not keep the
		// workloads it built while it held it, see releaseWorkloads. Every
		// other failed check keeps the workloads on the configuration of
		// the last pass and reports on Ready.
		if failure.Reason == v1.ReasonClusterAlreadyAttached {
			// The component conditions describe workloads that the next call
			// deletes, and comps stays nil on this path, so the flush does not
			// own those types and would write the stale values back. A parked
			// CamundaOptimize renders nothing, so it reports nothing about
			// what it used to render.
			removeComponentConditions(&optimize)
			return ctrl.Result{}, r.releaseWorkloads(ctx, &optimize)
		}

		return ctrl.Result{}, nil
	}
```

Move `recordSuspensionChange`, `wasSuspending`, and their constants out of `suspend.go` into `clustersuspension.go` with a package comment line that says they report the suspension that follows the referenced cluster. Delete the rest of `suspend.go` and the matching tests in `suspend_test.go` (keep the tests of the two moved functions, in `clustersuspension_test.go`).

- [ ] **Step 4: Run the package**

Run: `flock /tmp/claude-1000/gates/lock go test ./internal/controller/camundaoptimize/ 2>&1 | tail -20`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add -A internal/controller/camundaoptimize
git commit -m "fix(camundaoptimize): keep the workloads running when a pre-check fails (#370)"
```

### Task 9: `Suspended()` reports the two claim states only

**Files:**
- Modify: `api/v1/camundacluster_types.go:620-631`
- Modify: the `Suspended()` table test in `api/v1` (`grep -n "func TestSuspended\|Suspended()" api/v1/*_test.go`)
- Modify: `pkg/logicalbackup/precheck_test.go` (the `ClusterSuspended` cases)

- [ ] **Step 1: Write the failing tests**

In the `api/v1` table test, change the `InvalidReference` and `MissingSecret` rows to expect `false`, keep `StorageAlreadyAttached` and `WaitingForHandover` at `true`, and keep `VersionDowngradeRefused` at `false`. In `pkg/logicalbackup/precheck_test.go`, the case that expects `ClusterSuspended` for a cluster at `InvalidReference` now expects the pre-check to continue past the suspension check (assert the returned failure is not `ClusterSuspended`).

- [ ] **Step 2: Run them to verify they fail**

Run: `go -C api test ./... -run Suspended 2>&1 | tail -5 && flock /tmp/claude-1000/gates/lock go test ./pkg/logicalbackup/ 2>&1 | tail -5`
Expected: FAIL on the two rows and the backup case.

- [ ] **Step 3: Narrow the list**

```go
// suspendedReadyReasons are the Ready reasons under which the operator holds
// every workload of the cluster at zero: another cluster holds the storage
// claim of its backend, or the cluster waits for the pods of another cluster
// to leave that backend. A failed reference check is not one of them: the
// cluster keeps running on its last configuration. Neither is
// VersionDowngradeRefused: a refused cluster keeps running on the version
// it has.
var suspendedReadyReasons = []string{
	ReasonStorageAlreadyAttached,
	ReasonWaitingForHandover,
}
```

Update the GoDoc of `Suspended()` if it lists the reasons.

- [ ] **Step 4: Run the tests**

Run: `go -C api test ./... 2>&1 | tail -3 && flock /tmp/claude-1000/gates/lock go test ./pkg/logicalbackup/ ./internal/controller/backupschedule/ 2>&1 | tail -5`
Expected: `ok`.

- [ ] **Step 5: Commit**

```bash
git add api/v1 pkg/logicalbackup
git commit -m "fix(api): report a cluster suspended for the two claim states only (#370)"
```

### Task 10: Docs for the removed suspension

**Files:**
- Modify: `docs/crds/camundacluster.md:264-296` (reference check, the status block, the Suspend section), the reason table rows `InvalidReference` and `MissingSecret`
- Modify: `docs/crds/camundaoptimize.md:158`, `:164-190`, `:254`
- Modify: `docs/crds/backupschedule.md:94`
- Modify: `docs/guides/backup.md:330` if it lists the reasons

- [ ] **Step 1: Cluster reference**

Load `writing-operator-docs`. Replace the paragraph "When any of these checks fails for a running cluster, the cluster is suspended..." and its status block with: a running cluster keeps its workloads on the configuration of the last pass, keeps publishing `status.gateway` and `status.management`, and keeps its backend claim, while `Ready` reports the failure. Show a status block with `Ready=False/MissingSecret` and `ZeebeReady=True/Healthy`. In the Suspend section, the sentence "The operator also suspends a cluster on its own" lists `StorageAlreadyAttached` and `WaitingForHandover` only. In the reason table, drop "A running cluster is scaled to zero, with the volumes kept." from both rows.

- [ ] **Step 2: Optimize reference**

At line 158, the `VersionMismatch` paragraph says both workloads keep running on the previous minor until `spec.version` follows the cluster, and drop the sentence about scaling to zero. In the Suspension section, delete the paragraph "A failed check of a reference stops a running instance the same way..." and its status block, and the sentence "An instance whose first check fails has no workloads to stop." becomes a sentence that a failed check keeps the workloads and reports on `Ready`. At line 254 replace "Every reason above that reports a failed check scales both workloads to zero" with a sentence that a failed check keeps the workloads, and `ClusterAlreadyAttached` removes them because they belong to the holder.

- [ ] **Step 3: Backup pages**

In `docs/crds/backupschedule.md:94` the list of `Ready` reasons that suspend a cluster becomes `StorageAlreadyAttached` and `WaitingForHandover`. Check `docs/guides/backup.md`, `docs/crds/logicalbackupelasticsearch.md`, and `docs/crds/logicalbackuprdbms.md` with `grep -n "InvalidReference.*suspend\|suspend.*InvalidReference"` and fix any sentence that ties a failed reference to a suspension.

- [ ] **Step 4: Check**

Run: `mkdocs build --strict 2>&1 | tail -2 && grep -rn "scaled to zero\|scales.*to zero" docs | grep -v "spec.suspend\|StorageAlreadyAttached\|WaitingForHandover\|suspend:"`
Expected: a clean build, and the grep prints only lines about `spec.suspend` or the claim states. Read each hit.

- [ ] **Step 5: Commit**

```bash
git add docs
git commit -m "docs: a failed reference check keeps the workloads running (#370)"
```

### Task 11: Gates and the PR for #370

- [ ] **Step 1: Run every gate** (same commands as Task 6, Step 1).

- [ ] **Step 2: Open the PR** against `fix/suspend-only-for-shared-backends` with `Towards #370`. Run the review loop to clean, self-merge, `gh issue close 370`.

---

## Integration PR

After both sub-PRs are merged into `fix/suspend-only-for-shared-backends`: load `feature-dev-workflow:reviewing-feature-progress`, then open the integration PR from `fix/suspend-only-for-shared-backends` to `main` with `Closes #368`. The user merges it. The Copilot auto-review fires on it; budget one fix round (memory: balanced review earns its cost). Delete the plan and the state file in the last commit before the merge, keep the spec.
