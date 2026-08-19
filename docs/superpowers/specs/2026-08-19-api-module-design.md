# A separate Go module for the API types

**Status:** approved in dialogue (2026-08-19)
**Date:** 2026-08-19
**Scope:** `api/` becomes the Go module `github.com/konsole-is/camunda-operator/api`. The root
module, the Makefile, the Dockerfiles, the CI workflows, the release workflow, and the docs change
to match.

## Summary

Today `api/v1` is a package of the root module. A project that wants only the CRD types must
require the whole operator. That pulls in ocf, controller-runtime, the ECK and CloudNativePG
APIs, and every other dependency of the operator.

This design moves `api/` into its own Go module. The module depends only on `k8s.io/api` and
`k8s.io/apimachinery`. A consumer runs `go get github.com/konsole-is/camunda-operator/api@v0.1.0`
and imports `github.com/konsole-is/camunda-operator/api/v1`. No consumer writes a `replace`
directive.

The root module stays importable too. The cloud operator will import packages from `pkg/` later.
The root module names a real, published version of the api module in its `go.mod`. The release
flow keeps that version current.

This is the layout that the Flux controllers use (`github.com/fluxcd/source-controller/api`) and
that prometheus-operator uses (`pkg/apis/monitoring`).

## What exists today

- `api/v1` imports `k8s.io/api/core/v1`, `k8s.io/apimachinery/...`,
  `sigs.k8s.io/controller-runtime/pkg/scheme`, and testify in tests. Nothing from `pkg/` or ocf.
- `groupversion_info.go` builds `SchemeBuilder` with `scheme.Builder` from controller-runtime.
  Each `*_types.go` file calls `SchemeBuilder.Register(&X{}, &XList{})` in `init`.
- The Makefile runs `controller-gen`, `go vet`, `go test`, and `golangci-lint` with `./...` from
  the root. A nested module is not part of `./...`.
- `release.yml` runs when a GitHub release is published. The release tag is bare SemVer
  (`0.1.0`) because Helm chart versions must be bare SemVer. The workflow rejects a tag that
  starts with `v`.
- No release exists yet.

## Design

### 1. The api module

`api/go.mod`:

```
module github.com/konsole-is/camunda-operator/api

go <same as root>

require (
    github.com/stretchr/testify <same as root>
    k8s.io/api <same as root>
    k8s.io/apimachinery <same as root>
)
```

`api/v1/groupversion_info.go` drops controller-runtime. `SchemeBuilder` becomes a small local
type around `runtime.SchemeBuilder` from apimachinery. It keeps the `Register(...)` method that the
`*_types.go` files call, and `AddToScheme` keeps the signature `func(*runtime.Scheme) error`. The
`*_types.go` files do not change.

A rule for the module, written in `docs/architecture.md`: the api module imports only
`k8s.io/api`, `k8s.io/apimachinery`, and the standard library. It never imports `pkg/`,
`internal/`, ocf, or controller-runtime. A test in the root module (`go list -deps` over
`./api/...`) enforces the rule.

`pkg/labels` and other `pkg/` packages stay in the root module. The cloud operator imports them
from the root module at a released version. Nothing moves in this change.

### 2. The root module

Root `go.mod` gains two lines:

```
require github.com/konsole-is/camunda-operator/api v0.0.0-00010101000000-000000000000
replace github.com/konsole-is/camunda-operator/api => ./api
```

The `replace` line makes every local build, test, and `go mod tidy` use `./api` from disk. A type
and the controller that uses it change in one PR, with no published tag in between.

The `require` line names the api version that consumers of the root module get. Go ignores
`replace` directives in dependencies, so this line must name a published tag before anyone
imports the root module. Section 4 describes how the release keeps it current. Until the first
release it holds the placeholder shown above.

Import paths do not change. `github.com/konsole-is/camunda-operator/api/v1` stays valid in
`internal/`, `pkg/`, `cmd/`, and `test/`.

### 3. Tooling

The Makefile treats the repository as two modules. A variable lists them:

```
MODULES := . ./api
```

- `vet`, `test`, `lint`, `lint-fix`, `fmt`: loop over `MODULES` and run the tool in each
  directory. `.golangci.yml` at the root applies to both because golangci-lint finds it by
  walking up from the module directory.
- `generate`: run `controller-gen object` in `./api` (the DeepCopy functions) and in `.`
  (any other generated code).
- `manifests`: run `controller-gen crd` in `./api` with
  `output:crd:artifacts:config=../config/crd/bases`, and `controller-gen rbac webhook` in `.`.
  Running it per module avoids the question of whether `paths="./api/..."` resolves across the
  module boundary from the root.
- `tidy` (new target): `go mod tidy` in each module. CI runs it and fails on a diff.
- `Dockerfile` and `Dockerfile.cli`: copy `api/go.mod` and `api/go.sum` next to the root
  `go.mod` and `go.sum` before `go mod download`, so the dependency layer still caches.
- CI workflows (`test.yml`, `test-e2e.yml`, `test-chart.yml`, `lint.yml`): replace the inline
  `go mod tidy` with `make tidy`. `go-version-file: go.mod` does not change.

### 4. Release

A release produces three tags on one commit:

| Tag | For |
| --- | --- |
| `X.Y.Z` | Helm chart, images, release assets (exists today) |
| `api/vX.Y.Z` | Go consumers of the api module |
| `vX.Y.Z` | Go consumers of the root module |

`release.yml` pushes `api/v${TAG}` and `v${TAG}` at the release commit after the v-prefix check.
The step is idempotent: if the tag exists and points at the same commit, the step passes.

Before the release, one commit sets the `require` line in the root `go.mod` to the version that
the release will get:

```
go mod edit -require=github.com/konsole-is/camunda-operator/api@vX.Y.Z
```

A new Makefile target `api-version VERSION=X.Y.Z` runs this command. `release.yml` reads the
`require` line and fails if it does not equal `v${TAG}`. The failure message names the target to
run. This check makes a forgotten bump a loud error instead of a broken root module on the proxy.

The `replace` line stays. `go mod tidy` with a `replace` to a local directory never fetches the
api module, so the bump works before the tag exists.

A note in `docs/installation.md` (release section) lists the order: run `make api-version`,
merge, publish the release.

### 5. Docs

- `README.md`: a short section "Use the API types from Go" with the `go get` line and an import
  example.
- A new page `docs/go-api.md`, in the mkdocs nav: what the api module contains, how to get it,
  how versions map to operator releases, and that the root module is importable at `vX.Y.Z`.
- `docs/architecture.md`: the module boundary and the import rule from section 1.
- `docs/installation.md`: the release order from section 4.
- `AGENTS.md` project structure: `api/go.mod` and the `MODULES` loop.

## Testing

- `cd api && go build ./... && go test ./...` passes with no root module on the path.
- `go list -deps ./api/...` from the api module shows no `sigs.k8s.io/controller-runtime` and no
  `github.com/konsole-is/camunda-operator/pkg`. A unit test in the root module asserts this.
- `make tidy` leaves no diff in either module.
- `make generate manifests` leaves no diff.
- `make test` and `make lint` pass in both modules.
- Both Docker images build.
- A scratch module outside the repository requires the api module through a `replace` to the
  local checkout (test only) and compiles an import of `api/v1`.

## Out of scope

- Moving `pkg/labels` or any other package into the api module.
- A `go.work` file. `go mod tidy` does not read it, and consumers never see it.
- Changes to the in-flight backup work in the working tree.
