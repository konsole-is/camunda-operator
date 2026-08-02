# Helm Chart Distribution Design

**Date:** 2026-08-01
**Status:** Approved

---

## Overview

Make `camunda-operator` installable. The chart itself already generates correctly from the
kubebuilder `helm/v2-alpha` plugin — what is missing is everything around it: nothing chart-related
is committed, there is no release pipeline, and the one workflow that exercises the chart cannot
succeed.

This feature adds the distribution layer: a small committed chart surface, a fixed chart CI job, and
a release workflow that publishes a signed chart and image to `ghcr.io` on every published GitHub
Release.

The reference implementation is `konsole-is/fqdn-controller`, which uses the older `helm/v1-alpha`
plugin and a GPG-signed `gh-pages` Helm repository. This design keeps that project's *shape* — a
tiny committed surface, generate-at-release, a single release workflow — while diverging on
distribution (OCI instead of `gh-pages`) and signing (cosign keyless instead of GPG).

### Starting state (verified 2026-08-01, at `c680f90`)

Confirmed by running the generator against `HEAD` in a throwaway worktree:

- `PROJECT` registers `helm.kubebuilder.io/v2-alpha` with `manifests: dist/install.yaml`,
  `output: dist`.
- `make helm-generate` succeeds and produces a complete chart: 19 CRD templates, full RBAC, manager
  deployment, metrics service, `ServiceMonitor`, `NOTES.txt`, `_helpers.tpl`.
- `helm lint` passes (one INFO: no icon). `helm template` renders cleanly at defaults and with
  `prometheus.enable=true rbacHelpers.enable=true`.
- The generated `values.yaml` exposes `manager.*`, `crd.enable`/`crd.keep`, `rbacHelpers.enable`,
  `metrics.*`, `certManager.enable`, `prometheus.enable`.
- `Makefile` has `helm-generate`, `helm-deploy`, `helm-uninstall`, `helm-status`, `helm-history`,
  `helm-rollback`, `install-helm`.
- `.gitignore` ignores `dist` wholesale — no chart file is tracked.
- No release workflow exists. `README.md` is a single heading. No installation documentation exists
  anywhere.

### Generator semantics (verified, load-bearing)

`kubebuilder edit --plugins=helm/v2-alpha --force`:

| File | Behaviour under `--force` |
|---|---|
| `dist/chart/Chart.yaml` | **Preserved** if it exists — hand edits survive |
| `dist/chart/values.yaml` | **Overwritten** |
| `dist/chart/templates/**` (plugin-owned) | Overwritten |
| `dist/chart/templates/**` (unknown files) | Preserved |

Without `--force`, `values.yaml` is preserved too. The whole committed-surface decision rests on the
first row: committing `Chart.yaml` is safe, and committing a hand-tuned `values.yaml` is not.

## Goals

- A published, signed Helm chart at `oci://ghcr.io/konsole-is/charts/camunda-operator`, versioned in
  lockstep with the operator image.
- A published multi-arch operator image at `ghcr.io/konsole-is/camunda-operator`.
- `install.yaml` and `crds.yaml` attached to every GitHub Release for non-Helm installs.
- A chart CI job that actually runs, covering lint, template permutations, and a kind
  install-then-upgrade.
- Installation documentation, which does not currently exist.

## Non-Goals

- No `gh-pages` Helm repository. That branch is reserved for the mkdocs site, which has no deploy
  workflow yet.
- No `values.schema.json`. `values.yaml` is fully regenerated on every build, so a schema would
  validate a file nobody hand-writes, at the cost of a Helm plugin install in CI.
- No snapshot/nightly publishing from `main`.
- No separate `camunda-operator-crds` chart. The CRDs ship in the chart, with an out-of-band escape
  hatch (see below).
- No changes to `config/` beyond what is required to emit `crds.yaml`.
- No cert-manager or webhook enablement. Those sections stay commented out in `config/default` and
  the corresponding chart values stay `false`.

## Design decisions

### 1. Distribution: OCI registry

Charts publish to `oci://ghcr.io/konsole-is/charts`, giving the full reference
`ghcr.io/konsole-is/charts/camunda-operator:<version>`.

The `/charts` path segment keeps the chart from colliding with the operator image in the org
namespace. OCI removes the `index.yaml` maintenance and the `gh-pages` checkout dance entirely, uses
the same registry auth as the image, and leaves `gh-pages` free for the docs site. Consumers need
Helm 3.8+; the repo pins Helm 4.1.4, so this is not a practical constraint.

**One-time manual step:** ghcr packages are private on first publish. Someone must set the
`charts/camunda-operator` package to public after the first release. This cannot be automated from
the release workflow.

### 2. CRDs ship in the chart, gated

Keep the `v2-alpha` default: CRDs live in `templates/crd/` behind `{{- if .Values.crd.enable }}`
(default `true`) with `crd.keep` (default `true`) so an uninstall does not delete custom resources.

CRDs total 152K today only because the API types are still stubs. Once `CamundaCluster` and friends
carry real specs, the set will grow by an order of magnitude, and everything under `templates/`
lands in the Helm release Secret, which is bounded by etcd's ~1MB object limit. CRD text compresses
well, so this is a risk rather than a certainty — but it is the classic failure mode for multi-CRD
operators and must have an exit before it is hit, not after.

The exit is `dist/crds.yaml`, published as a release asset: apply CRDs with `kubectl` and install
the chart with `crd.enable=false`. `helm-verify` carries a rendered-size tripwire so the ceiling is
visible as it is approached rather than discovered by a failed install.

### 3. Committed surface: three files

`.gitignore` changes from blanket `dist` to the selective form:

```
dist/**/*
!dist/chart/
!dist/chart/Chart.yaml
!dist/chart/README.md
```

Tracked: `dist/chart/Chart.yaml` and `dist/chart/README.md`. Everything else regenerates. No
generated YAML appears in review diffs, and `helm-generate --force` stays safe to run at any time.

`Chart.yaml` becomes hand-maintained — the generated fields plus `home`, `sources`, `icon`,
`maintainers`, `kubeVersion`. `version` and `appVersion` stay at `0.0.1` as placeholders that the
release workflow stamps.

**Chart icon:** no image file is committed. `icon:` points at the konsole-is org avatar,
`https://avatars.githubusercontent.com/u/200405113?v=4` — the org's own mark, which avoids both a
tracked binary and any third-party trademark question. `fqdn-controller` commits a
`dist/chart_icon.png`; this project deliberately does not.

**Accepted constraint:** the shipped `values.yaml` defaults are not directly editable, because
`--force` overwrites that file. Anything that must differ in the defaults has to originate in
`config/` and flow through `install.yaml` into the chart. This is deliberate: it keeps kustomize as
the single source of truth for what the operator deploys.

### 4. Versioning: unprefixed SemVer tags

Tags are bare SemVer (`0.1.0`), matching `fqdn-controller`. Helm chart versions must be valid SemVer
without a `v` prefix, so prefixed tags would need stripping at every stamp site. Chart `version`,
chart `appVersion`, and the image tag are all the same string.

### 5. Signing: cosign keyless

Both the chart and image are signed with cosign keyless (OIDC via the GitHub Actions identity). No
secrets to provision or rotate, verifiable with `cosign verify`. Requires `id-token: write` on the
release job. This drops the `HELM_GPG_*` secret handling that `fqdn-controller` carries.

### 6. Release trigger: published GitHub Release

`on: release: types: [published]`, with `TAG=${{ github.ref_name }}`. Release notes get staged
before artifacts go out, and chart and operator versions stay locked together by construction.

Prereleases are not special-cased. OCI has no shared index to poison, so a prerelease simply
publishes under its prerelease SemVer tag.

## Components

### `Makefile`

- `build-installer` additionally emits `dist/crds.yaml` via `kustomize build config/crd`. Required
  for the `crd.enable=false` path and as a release asset.
- `PLATFORMS` narrows from the scaffold default
  `linux/arm64,linux/amd64,linux/s390x,linux/ppc64le` to `linux/amd64,linux/arm64`. The dropped
  architectures cost cross-compile time on every release for platforms Camunda workloads do not run
  on.
- Two latent bugs found while planning, both of which the release path depends on:
  - **`helm-deploy` deploys an empty image reference.** The recipe expands `$${IMG%:*}` in the
    shell, which reads the environment. Make exports command-line variables automatically but not
    the `IMG ?=` default, so `make helm-deploy` without `IMG=` sets
    `manager.image.repository=""`. Fixed with `export IMG`, which also keeps registry-with-port
    values correct (`${IMG%:*}` strips only the final colon) — unlike a make-level `$(subst)`,
    which would split `localhost:5000/img:tag` at the wrong colon.
  - **`docker-buildx` swallows build failures.** The `buildx build --push` line carries a leading
    `-`, telling make to ignore a non-zero exit, so a failed release image build would report
    success. The `-` on `buildx create`/`buildx rm` is intentional idempotency and stays.
- New `helm-verify`: `helm lint`, plus `helm template` across the value permutations that actually
  branch — `crd.enable=false`, `rbacHelpers.enable=true`, `prometheus.enable=true`,
  `certManager.enable=true` — plus a rendered-size tripwire. No cluster required.

  The tripwire fails the target when the worst-case `helm template` output across all value
  permutations exceeds **1 MiB uncompressed**. The real bound is the gzipped release Secret
  against etcd's ~1MB object limit, so 1 MiB uncompressed is a deliberately conservative proxy:
  gzip on CRD text buys several-fold headroom, meaning the tripwire fires well before an install
  would actually fail. That is the intent — it is an early warning to trigger the CRD-split
  conversation, not a hard correctness check. Today's worst case is ~155 KiB (with all optional
  features enabled plus CRDs); the default render is ~120 KiB.
- `helm-generate` keeps `--force`.

### `.github/workflows/chart.yml` (fix)

This job cannot currently pass: it runs `helm lint ./dist/chart` and `make helm-deploy`, but nothing
in the workflow generates the chart, and `dist` is gitignored, so `dist/chart` does not exist on a
fresh checkout. Neither step declares `helm-generate` as a prerequisite.

The kubebuilder `helm/v2-alpha` plugin regenerates `.github/workflows/test-chart.yml` from its own
scaffold on every `make helm-generate`, silently reverting any edits. To avoid the collision, the
workflow has been moved to `.github/workflows/chart.yml`, at a path the plugin does not own, and
`.github/workflows/test-chart.yml` is gitignored.

Changes to the moved workflow:
- Insert `make helm-generate IMG=controller:latest` before any step that reads `dist/chart`.
- Trigger becomes `push: branches: [main]` plus `pull_request`. The current bare `on: push`
  double-runs on every PR branch.
- Pin kubebuilder and Helm from `.tool-versions` (4.13.1 / 4.1.4) so CI cannot drift from local.
- Run `helm-verify` before provisioning a cluster — fast failure on template breakage.
- Extend the kind step from install-only to **install then upgrade**. Helm CRD upgrades are the most
  likely breakage as the API grows, and that must fail in CI rather than for a user.

Out of scope: the `actions/checkout@v4` → `v5` and `setup-go@v5` → `v6` bumps. The whole repo is on
v4/v5; bumping one workflow creates inconsistency. Separate cleanup.

### `.github/workflows/release.yml` (new)

Trigger `release: types: [published]`. Environment: `TAG=${{ github.ref_name }}`,
`IMG=ghcr.io/${{ github.repository }}:${TAG}`. Permissions: `contents: write`, `packages: write`,
`id-token: write`.

1. Checkout; set up Go from `go.mod`; install kubebuilder 4.13.1 and Helm 4.1.4 per `.tool-versions`.
2. `make build-installer IMG=$IMG` → `dist/install.yaml`, `dist/crds.yaml`.
3. `make helm-generate IMG=$IMG`; stamp `version` and `appVersion` in `Chart.yaml` to `$TAG`.
4. Log in to ghcr; `make docker-buildx` multi-arch push; capture the image digest.
5. `helm package dist/chart`; `helm push` to `oci://ghcr.io/konsole-is/charts`; capture the chart
   digest.
6. `cosign sign --yes` both digests.
7. `softprops/action-gh-release` uploads `dist/install.yaml`, `dist/crds.yaml`, and the packaged
   `.tgz`.

### Documentation

Installation documentation does not exist today — `README.md` is one line and `docs/index.md` points
at the README for installation.

- **`docs/installation.md`** (new, canonical, added to the mkdocs nav): OCI Helm install, the raw
  `install.yaml` path, upgrade and uninstall, the `crd.enable=false` escape hatch, and cosign
  verification.
- **`README.md`**: short quickstart that links to `docs/installation.md`.
- **`docs/index.md`**: point the installation link at `docs/installation.md` instead of the README.
- **`dist/chart/README.md`** (new, tracked): chart values reference.
- **`AGENTS.md`**: extend the existing Helm section with the release flow.

## Testing

Chart correctness is not reachable from Go tests, so verification is layered:

1. **`make helm-verify`** — `helm lint` plus `helm template` across branching value permutations,
   plus the render-size tripwire. Runs locally and as the first CI gate. No cluster.
2. **`test-chart.yml`** — kind cluster, `helm install` then `helm upgrade`, release status assertion.
3. **Manual dry run** — one release against a scratch tag, verifying the ghcr push, the cosign
   signature, and a clean install from the published OCI reference, before the first real release.

`make test` must stay green throughout; nothing in this feature touches Go code.

## Risks

- **Release-Secret size ceiling.** Mitigated by `crds.yaml`, `crd.enable=false`, and the tripwire.
  Not eliminated — if the CRD set outgrows the limit, the follow-up is splitting out a separate CRDs
  chart, which this design leaves open.
- **First-publish package visibility** is manual and easy to forget; the release will appear to
  succeed while the chart is unpullable.
- **Cosign keyless verification** requires consumers to know the expected OIDC issuer and identity;
  `docs/installation.md` must state the exact `cosign verify` invocation or the signatures are
  decorative.
- **`--force` overwriting `values.yaml`** is invisible when it happens. The committed-surface rule
  is the guard; a reviewer seeing a `values.yaml` diff in a PR should treat it as a mistake.
- **No `LICENSE` file exists** in this repository. Publishing a chart and image without one leaves
  consumers without terms. Out of scope here, but it should be settled before the first public
  release.
- **`kubeVersion: ">= 1.30.0-0"`** is a conservative judgment call, not a measured floor — nothing
  in the codebase pins a minimum Kubernetes version. Helm enforces it at install time, so setting
  it too high would lock out working clusters.
