# Helm Chart Distribution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `camunda-operator` installable by publishing a signed Helm chart and multi-arch image to `ghcr.io` on every published GitHub Release.

**Architecture:** The chart is generated, never hand-written — `kubebuilder edit --plugins=helm/v2-alpha --force` builds it from kustomize output. Only three things are committed: `Chart.yaml`, a chart `README.md`, and `.gitignore` exceptions. CI regenerates the chart before touching it; the release workflow regenerates, stamps the version, packages, pushes to an OCI registry, and signs with cosign keyless.

**Tech Stack:** kubebuilder 4.13.1 (`helm/v2-alpha` plugin), Helm 4.1.4, kustomize, GitHub Actions, cosign (keyless OIDC), ghcr.io, mkdocs-material.

**Design spec:** `docs/superpowers/specs/2026-08-01-helm-chart-distribution-design.md`

## Global Constraints

- Tool versions come from `.tool-versions`: kubebuilder `4.13.1`, Helm `4.1.4`, kind `0.31.0`, kubectl `1.35.3`. CI must pin these exact versions, never `latest`.
- Chart registry: `oci://ghcr.io/konsole-is/charts` → full reference `ghcr.io/konsole-is/charts/camunda-operator`.
- Image registry: `ghcr.io/konsole-is/camunda-operator`.
- Release tags are **unprefixed SemVer** (`0.1.0`, not `v0.1.0`). Chart `version`, chart `appVersion`, and image tag are all the same string.
- Chart icon URL: `https://avatars.githubusercontent.com/u/200405113?v=4` (the konsole-is org avatar). No image file is committed.
- Build platforms: `linux/amd64,linux/arm64` only.
- **Never hand-edit `dist/chart/values.yaml` or anything under `dist/chart/templates/`.** `--force` overwrites `values.yaml` on every regeneration. Shipped defaults must originate in `config/`. A `values.yaml` diff appearing in a PR is a mistake.
- `dist/chart/Chart.yaml` *is* hand-maintained — `--force` preserves it. This is verified behaviour and the whole design rests on it.
- Do not enable cert-manager or webhooks. Those stay commented out in `config/default` and their chart values stay `false`.
- Do not bump `actions/checkout@v4` → `v5` or `setup-go@v5` → `v6`. The repo is uniformly on v4/v5; that is separate cleanup.
- `make test` must stay green. No task in this plan touches Go code.

---

### Task 1: Committed chart surface

Track exactly three things under `dist/`: `Chart.yaml`, `README.md`, and the `.gitignore` exceptions that permit them. Everything else regenerates.

**Files:**
- Modify: `.gitignore:32-33`
- Create: `dist/chart/Chart.yaml`
- Create: `dist/chart/README.md`

**Interfaces:**
- Consumes: nothing.
- Produces: a tracked `dist/chart/Chart.yaml` carrying `version: 0.0.1` / `appVersion: "0.0.1"` placeholder lines, both anchored at the start of a line. Task 5's release workflow rewrites those two lines with `sed`; the anchors must stay exactly as written.

- [ ] **Step 1: Write the failing test**

Create `/tmp/chart-surface-check.sh` (scratch, not committed):

```bash
#!/usr/bin/env bash
# Verifies the committed chart surface is exactly three paths and that
# regeneration does not clobber the hand-maintained Chart.yaml.
set -euo pipefail

echo "--- tracked files under dist/ ---"
tracked="$(git ls-files dist | sort)"
echo "$tracked"
expected="dist/chart/Chart.yaml
dist/chart/README.md"
[ "$tracked" = "$expected" ] || { echo "FAIL: tracked set mismatch"; exit 1; }

echo "--- Chart.yaml survives --force regeneration ---"
before="$(sha256sum dist/chart/Chart.yaml | cut -d' ' -f1)"
make helm-generate >/dev/null 2>&1
after="$(sha256sum dist/chart/Chart.yaml | cut -d' ' -f1)"
[ "$before" = "$after" ] || { echo "FAIL: Chart.yaml was overwritten by regeneration"; exit 1; }

echo "--- no generated file became tracked or dirty ---"
dirty="$(git status --porcelain dist)"
[ -z "$dirty" ] || { echo "FAIL: dist is dirty after regeneration:"; echo "$dirty"; exit 1; }

echo "--- version anchors present for release-time sed ---"
grep -qE '^version: 0\.0\.1$' dist/chart/Chart.yaml || { echo "FAIL: no anchored version line"; exit 1; }
grep -qE '^appVersion: "0\.0\.1"$' dist/chart/Chart.yaml || { echo "FAIL: no anchored appVersion line"; exit 1; }

echo "PASS"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `chmod +x /tmp/chart-surface-check.sh && /tmp/chart-surface-check.sh`
Expected: FAIL at the first check — `git ls-files dist` is empty, because `.gitignore` currently ignores `dist` wholesale.

- [ ] **Step 3: Replace the blanket `dist` ignore with selective exceptions**

In `.gitignore`, replace these two lines:

```
# helm dist
dist
```

with:

```
# helm dist — everything under dist/ is generated except the hand-maintained
# chart metadata below. See docs/superpowers/specs/2026-08-01-helm-chart-distribution-design.md
dist/**/*
!dist/chart/
!dist/chart/Chart.yaml
!dist/chart/README.md
```

- [ ] **Step 4: Write `dist/chart/Chart.yaml`**

```yaml
apiVersion: v2
name: camunda-operator
description: Core Kubernetes operator for the Camunda platform
type: application

# version and appVersion are placeholders. The release workflow
# (.github/workflows/release.yml) rewrites both lines with the release tag.
# Keep them anchored at the start of a line — the sed expressions depend on it.
version: 0.0.1
appVersion: "0.0.1"

kubeVersion: ">= 1.30.0-0"

keywords:
  - kubernetes
  - operator
  - camunda

home: https://github.com/konsole-is/camunda-operator
sources:
  - https://github.com/konsole-is/camunda-operator
icon: https://avatars.githubusercontent.com/u/200405113?v=4

maintainers:
  - name: konsole-is
    url: https://github.com/konsole-is

annotations:
  kubebuilder.io/generated-by: kubebuilder
```

- [ ] **Step 5: Write `dist/chart/README.md`**

```markdown
# camunda-operator Helm chart

Core Kubernetes operator for the Camunda platform.

```bash
helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system --create-namespace
```

Full installation guide, including verification and the large-CRD install path:
<https://github.com/konsole-is/camunda-operator/blob/main/docs/installation.md>

## Values

| Key | Default | Description |
|---|---|---|
| `manager.replicas` | `1` | Controller manager replica count. |
| `manager.image.repository` | `controller` | Manager image repository. Set at release time. |
| `manager.image.tag` | `latest` | Manager image tag. Set at release time. |
| `manager.image.pullPolicy` | `IfNotPresent` | Image pull policy. |
| `manager.args` | `["--leader-elect"]` | Arguments passed to the manager. |
| `manager.env` | `[]` | Environment variables. |
| `manager.envOverrides` | `{}` | Per-variable overrides; wins over `manager.env`. |
| `manager.imagePullSecrets` | `[]` | Image pull secrets. |
| `manager.podSecurityContext` | `runAsNonRoot: true`, `seccompProfile.type: RuntimeDefault` | Pod security context. |
| `manager.securityContext` | drops `ALL`, read-only root FS, no privilege escalation | Container security context. |
| `manager.resources` | requests `10m`/`64Mi`, limits `500m`/`128Mi` | Resource requests and limits. |
| `manager.affinity` | `{}` | Pod affinity. |
| `manager.nodeSelector` | `{}` | Pod node selector. |
| `manager.tolerations` | `[]` | Pod tolerations. |
| `crd.enable` | `true` | Install CRDs with the chart. Set `false` when applying CRDs out of band. |
| `crd.keep` | `true` | Annotate CRDs `helm.sh/resource-policy: keep` so uninstall does not delete custom resources. |
| `rbacHelpers.enable` | `false` | Install convenience admin/editor/viewer ClusterRoles for each CRD. |
| `metrics.enable` | `true` | Expose the RBAC-protected `/metrics` endpoint. |
| `metrics.port` | `8443` | Metrics server port. |
| `certManager.enable` | `false` | cert-manager integration. Not currently wired up — leave `false`. |
| `prometheus.enable` | `false` | Install a `ServiceMonitor`. Requires prometheus-operator CRDs. |
| `nameOverride` | unset | Partially override the generated resource name. |
| `fullnameOverride` | unset | Fully override the generated resource name. |

`values.yaml` is regenerated by `make helm-generate` and must not be hand-edited.
Change defaults in `config/` instead.
```

- [ ] **Step 6: Run the test to verify it passes**

Run: `/tmp/chart-surface-check.sh`
Expected: PASS — four sections, ending in `PASS`.

- [ ] **Step 7: Commit**

```bash
git add .gitignore dist/chart/Chart.yaml dist/chart/README.md
git commit -m "feat: track hand-maintained chart metadata

Replaces the blanket dist/ ignore with selective exceptions so Chart.yaml
and the chart README are versioned while everything generated stays out
of review diffs. Verified that kubebuilder edit --force preserves
Chart.yaml, which is what makes this surface safe."
```

---

### Task 2: Makefile correctness

Three fixes the release pipeline depends on: emit `crds.yaml`, stop `helm-deploy` from silently deploying an empty image reference, and stop `docker-buildx` from swallowing build failures.

**Files:**
- Modify: `Makefile:132` (PLATFORMS), `Makefile:139` (buildx error suppression), `Makefile:143-147` (build-installer), `Makefile:257-268` (IMG export)

**Interfaces:**
- Consumes: `Chart.yaml` from Task 1 (not directly, but `helm-generate` must not clobber it).
- Produces: `make build-installer` writes both `dist/install.yaml` and `dist/crds.yaml`. Tasks 4 and 5 depend on `dist/crds.yaml` existing after `build-installer`.

- [ ] **Step 1: Write the failing test**

Create `/tmp/makefile-check.sh` (scratch, not committed):

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "--- build-installer emits crds.yaml ---"
rm -f dist/crds.yaml
make build-installer >/dev/null 2>&1
[ -s dist/crds.yaml ] || { echo "FAIL: dist/crds.yaml missing or empty"; exit 1; }
count="$(grep -c '^kind: CustomResourceDefinition$' dist/crds.yaml)"
[ "$count" -eq 19 ] || { echo "FAIL: expected 19 CRDs in crds.yaml, got $count"; exit 1; }

echo "--- IMG expands in recipes without being passed on the command line ---"
out="$(make -n helm-deploy 2>/dev/null | grep -o 'manager.image.repository=[^ ]*' | head -1)"
[ "$out" = "manager.image.repository=controller" ] || {
  echo "FAIL: expected repository=controller from the IMG default, got '$out'"; exit 1; }

echo "--- buildx build failures are not suppressed ---"
grep -qE '^\s+\$\(CONTAINER_TOOL\) buildx build --push' Makefile || {
  echo "FAIL: buildx build line still has a leading '-' (errors ignored)"; exit 1; }

echo "--- PLATFORMS narrowed ---"
grep -qE '^PLATFORMS \?= linux/amd64,linux/arm64$' Makefile || {
  echo "FAIL: PLATFORMS not narrowed to amd64+arm64"; exit 1; }

echo "PASS"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `chmod +x /tmp/makefile-check.sh && /tmp/makefile-check.sh`
Expected: FAIL at the first check — `build-installer` only writes `install.yaml` today.

- [ ] **Step 3: Emit `crds.yaml` from `build-installer`**

In `Makefile`, replace the `build-installer` recipe body (currently three lines after the target) so it reads:

```make
.PHONY: build-installer
build-installer: manifests generate kustomize ## Generate a consolidated YAML with CRDs and deployment.
	mkdir -p dist
	cd config/manager && "$(KUSTOMIZE)" edit set image controller=${IMG}
	"$(KUSTOMIZE)" build config/default > dist/install.yaml
	"$(KUSTOMIZE)" build config/crd > dist/crds.yaml
```

`dist/crds.yaml` is what makes `crd.enable=false` usable and is published as a release asset.

- [ ] **Step 4: Export `IMG` so recipe shell expansion works**

`helm-deploy` uses `$${IMG%:*}` and `$${IMG##*:}`, which read the *environment*. Make exports command-line variables automatically but not `?=` defaults, so `make helm-deploy` with no `IMG=` silently sets `manager.image.repository=""`. Verified.

In `Makefile`, immediately after line 2 (`IMG ?= controller:latest`), add:

```make
# Recipes below expand IMG in the shell (e.g. $${IMG%:*}), which reads the
# environment. Make exports command-line variables automatically but not this
# default, so without the export `make helm-deploy` deploys an empty image
# reference. Exporting also keeps registry-with-port values like
# localhost:5000/img:tag correct, since ${IMG%:*} strips only the final colon.
export IMG
```

Do **not** rewrite the expansions as make-level `$(subst)` — splitting on `:` breaks registry ports.

- [ ] **Step 5: Stop `docker-buildx` swallowing build failures**

In `Makefile`, the buildx build line carries a leading `-`, which tells make to ignore a non-zero exit. In a release workflow that means a failed image build is reported as success. Remove only that `-`; the `-` on `buildx create` and `buildx rm` are intentional idempotency and stay.

Change:

```make
	- $(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
```

to:

```make
	$(CONTAINER_TOOL) buildx build --push --platform=$(PLATFORMS) --tag ${IMG} -f Dockerfile.cross .
```

- [ ] **Step 6: Narrow `PLATFORMS`**

Change:

```make
PLATFORMS ?= linux/arm64,linux/amd64,linux/s390x,linux/ppc64le
```

to:

```make
PLATFORMS ?= linux/amd64,linux/arm64
```

- [ ] **Step 7: Run the test to verify it passes**

Run: `/tmp/makefile-check.sh`
Expected: PASS.

- [ ] **Step 8: Confirm nothing generated became tracked**

Run: `git status --porcelain dist`
Expected: empty. `dist/crds.yaml` is new but matches `dist/**/*` and has no exception.

- [ ] **Step 9: Commit**

```bash
git add Makefile
git commit -m "fix: emit crds.yaml, export IMG, unsuppress buildx failures

build-installer now writes dist/crds.yaml, needed for the crd.enable=false
install path and as a release asset.

IMG was expanded in the shell but never exported, so 'make helm-deploy'
without IMG= on the command line silently deployed an empty image
reference. Exporting fixes it and keeps registry ports intact.

The buildx build line ignored its exit status, which would report a failed
release image build as success.

Narrows PLATFORMS to amd64+arm64; s390x and ppc64le were scaffold defaults."
```

---

### Task 3: `helm-verify` target

A cluster-free gate: lint, render every branching value permutation, and trip a size wire before the CRD set outgrows in-chart delivery.

**Files:**
- Modify: `Makefile` (Helm Deployment section, after `helm-generate`)

**Interfaces:**
- Consumes: `HELM`, `HELM_CHART_DIR` (already defined at `Makefile:260,266`).
- Produces: `make helm-verify`, used by Tasks 4 and 5. Exits non-zero on lint failure, render failure, or size overrun. Threshold overridable via `HELM_MAX_RENDER_BYTES`.

- [ ] **Step 1: Write the failing test**

Create `/tmp/helm-verify-check.sh` (scratch, not committed):

```bash
#!/usr/bin/env bash
set -euo pipefail

echo "--- helm-verify passes on the current chart ---"
make helm-generate >/dev/null 2>&1
make helm-verify >/tmp/hv.out 2>&1 || { echo "FAIL: helm-verify errored"; cat /tmp/hv.out; exit 1; }
grep -q "rendered size:" /tmp/hv.out || { echo "FAIL: no size report"; cat /tmp/hv.out; exit 1; }

echo "--- all six permutations rendered ---"
n="$(grep -c 'helm template' /tmp/hv.out)"
[ "$n" -eq 6 ] || { echo "FAIL: expected 6 permutations, got $n"; exit 1; }

echo "--- the size tripwire actually fires ---"
if make helm-verify HELM_MAX_RENDER_BYTES=1000 >/tmp/hv2.out 2>&1; then
  echo "FAIL: tripwire did not fire at a 1000-byte limit"; exit 1
fi
grep -q "exceeds" /tmp/hv2.out || { echo "FAIL: no explanatory error"; cat /tmp/hv2.out; exit 1; }

echo "--- missing chart gives a useful error, not a stack trace ---"
mv dist/chart /tmp/chart-stash
if make helm-verify >/tmp/hv3.out 2>&1; then
  mv /tmp/chart-stash dist/chart; echo "FAIL: passed with no chart"; exit 1
fi
mv /tmp/chart-stash dist/chart
grep -q "helm-generate" /tmp/hv3.out || { echo "FAIL: error does not point at helm-generate"; cat /tmp/hv3.out; exit 1; }

echo "PASS"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `chmod +x /tmp/helm-verify-check.sh && /tmp/helm-verify-check.sh`
Expected: FAIL — `make: *** No rule to make target 'helm-verify'`.

- [ ] **Step 3: Add the target**

In `Makefile`, insert immediately after the `helm-generate` target:

```make
## Maximum rendered chart size in bytes before helm-verify fails.
## The real bound is the gzipped Helm release Secret against etcd's ~1MB object
## limit. This uncompressed proxy is deliberately conservative — gzip on CRD text
## buys several-fold headroom, so the wire trips well before an install would
## actually fail. It is an early warning to trigger the CRD-split conversation,
## not a correctness check. See
## docs/superpowers/specs/2026-08-01-helm-chart-distribution-design.md
HELM_MAX_RENDER_BYTES ?= 1048576

.PHONY: helm-verify
helm-verify: install-helm ## Lint and render the Helm chart across value permutations. No cluster required.
	@test -d "$(HELM_CHART_DIR)" || { \
		echo "$(HELM_CHART_DIR) not found; run 'make helm-generate' first." >&2; exit 1; }
	$(HELM) lint "$(HELM_CHART_DIR)"
	@set -e; \
	for opts in \
		"" \
		"--set crd.enable=false" \
		"--set rbacHelpers.enable=true" \
		"--set prometheus.enable=true" \
		"--set certManager.enable=true" \
		"--set crd.enable=false --set rbacHelpers.enable=true --set prometheus.enable=true --set certManager.enable=true" \
	; do \
		echo "  helm template $${opts:-<defaults>}"; \
		$(HELM) template verify "$(HELM_CHART_DIR)" $$opts >/dev/null; \
	done
	@bytes="$$( $(HELM) template verify "$(HELM_CHART_DIR)" | wc -c )"; \
	echo "  rendered size: $$bytes bytes (limit $(HELM_MAX_RENDER_BYTES))"; \
	if [ "$$bytes" -gt "$(HELM_MAX_RENDER_BYTES)" ]; then \
		echo "ERROR: rendered chart exceeds $(HELM_MAX_RENDER_BYTES) bytes." >&2; \
		echo "The CRD set has outgrown in-chart delivery. Consider splitting CRDs" >&2; \
		echo "into a separate chart; see the design spec for the follow-up." >&2; \
		exit 1; \
	fi
```

Note: `certManager.enable=true` renders zero objects today, because cert-manager is commented out in `config/default`. The permutation is kept as a standing guard for when it is enabled.

- [ ] **Step 4: Run the test to verify it passes**

Run: `/tmp/helm-verify-check.sh`
Expected: PASS. The size report reads roughly `rendered size: 119859 bytes (limit 1048576)`.

- [ ] **Step 5: Commit**

```bash
git add Makefile
git commit -m "feat: add helm-verify target

Cluster-free chart gate: lint, render the six value permutations that
actually branch, and fail if the rendered chart approaches the Helm
release Secret size ceiling. Current render is ~120KiB against a 1MiB
tripwire."
```

---

### Task 4: Fix `test-chart.yml`

This job cannot pass today. It runs `helm lint ./dist/chart` and `make helm-deploy`, but nothing generates the chart and `dist` is gitignored — so `dist/chart` does not exist on a fresh checkout.

**Files:**
- Modify: `.github/workflows/test-chart.yml` (full rewrite)

**Interfaces:**
- Consumes: `make helm-verify` (Task 3), `make helm-generate`, `make helm-deploy` with the `IMG` export fix (Task 2).
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Write the failing test**

Create `/tmp/workflow-check.sh` (scratch, not committed):

```bash
#!/usr/bin/env bash
set -euo pipefail
f=.github/workflows/test-chart.yml

echo "--- chart is generated before anything reads it ---"
gen_line="$(grep -n 'helm-generate' "$f" | head -1 | cut -d: -f1)"
read_line="$(grep -nE 'helm-verify|helm lint|helm-deploy' "$f" | head -1 | cut -d: -f1)"
[ -n "$gen_line" ] || { echo "FAIL: workflow never generates the chart"; exit 1; }
[ "$gen_line" -lt "$read_line" ] || { echo "FAIL: chart read at line $read_line before generation at $gen_line"; exit 1; }

echo "--- push trigger is scoped to main ---"
grep -qE '^\s+branches:\s*\[\s*main\s*\]' "$f" || { echo "FAIL: bare 'on: push' double-runs on PR branches"; exit 1; }

echo "--- tool versions pinned to .tool-versions ---"
grep -q '4.13.1' "$f" || { echo "FAIL: kubebuilder version not pinned"; exit 1; }
grep -q '4.1.4' "$f" || { echo "FAIL: helm version not pinned"; exit 1; }
grep -q 'kind/dl/latest' "$f" && { echo "FAIL: kind still installed from 'latest'"; exit 1; }

echo "--- upgrade path exercised, not just install ---"
grep -q 'helm upgrade' "$f" || { echo "FAIL: no upgrade step; CRD upgrades are the likeliest breakage"; exit 1; }

echo "PASS"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `chmod +x /tmp/workflow-check.sh && /tmp/workflow-check.sh`
Expected: FAIL at the first check — no `helm-generate` anywhere in the workflow.

- [ ] **Step 3: Rewrite the workflow**

Replace the entire contents of `.github/workflows/test-chart.yml`:

```yaml
name: Test Chart

on:
  push:
    branches: [main]
  pull_request:

jobs:
  test-chart:
    name: Run on Ubuntu
    runs-on: ubuntu-latest
    steps:
      - name: Clone the code
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      # Versions below are pinned to .tool-versions. Keep them in sync.
      - name: Install kubebuilder
        run: |
          VERSION=4.13.1
          OS=$(go env GOOS)
          ARCH=$(go env GOARCH)
          curl -fsSL -o kubebuilder \
            "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${VERSION}/kubebuilder_${OS}_${ARCH}"
          chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/
          kubebuilder version

      - name: Install Helm
        uses: azure/setup-helm@v4.3.1
        with:
          version: v4.1.4

      - name: Generate the Helm chart
        run: make helm-generate IMG=controller:latest

      # Cluster-free gate first: fail fast on template breakage before paying
      # for a kind cluster and an image build.
      - name: Verify the Helm chart
        run: make helm-verify

      - name: Install kind
        run: |
          VERSION=0.31.0
          curl -fsSLo ./kind \
            "https://kind.sigs.k8s.io/dl/v${VERSION}/kind-linux-$(go env GOARCH)"
          chmod +x ./kind
          sudo mv ./kind /usr/local/bin/kind
          kind version

      - name: Create kind cluster
        run: kind create cluster

      - name: Build and load the operator image
        run: |
          make docker-build IMG=controller:latest
          kind load docker-image controller:latest

      - name: Install the chart
        run: make helm-deploy IMG=controller:latest

      - name: Check release status
        run: make helm-status

      # CRD upgrades are the most likely breakage as the API grows. Exercise
      # the upgrade path in CI rather than discovering it in a user's cluster.
      - name: Upgrade the chart
        run: make helm-deploy IMG=controller:latest HELM_EXTRA_ARGS="--set rbacHelpers.enable=true"

      - name: Check upgrade revision
        run: |
          make helm-status
          rev=$(helm get metadata camunda-operator \
            --namespace camunda-operator-system -o json | jq -r '.version')
          echo "release revision: $rev"
          [ "$rev" -ge 2 ] || { echo "expected revision >= 2 after upgrade"; exit 1; }

      - name: Uninstall the chart
        run: make helm-uninstall

      # crd.keep defaults to true, so CRDs must survive uninstall.
      - name: Verify CRDs survived uninstall
        run: |
          count=$(kubectl get crd -o name | grep -c 'core\.camunda\.io' || true)
          echo "CRDs remaining: $count"
          [ "$count" -eq 19 ] || { echo "expected 19 CRDs to be kept, found $count"; exit 1; }
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `/tmp/workflow-check.sh`
Expected: PASS.

- [ ] **Step 5: Validate the YAML parses**

Run: `python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/test-chart.yml')); print('valid YAML')"`
Expected: `valid YAML`

- [ ] **Step 6: Dry-run the cluster-free portion locally**

Run: `make helm-generate IMG=controller:latest && make helm-verify`
Expected: lint passes, six permutations render, size reported under the limit. This is exactly what the workflow's first gate runs.

- [ ] **Step 7: Commit**

```bash
git add .github/workflows/test-chart.yml
git commit -m "fix: make the chart workflow able to pass

The job read dist/chart without ever generating it, and dist is
gitignored — so it could not have succeeded on a fresh checkout.
Generates the chart first, pins kubebuilder/helm/kind to .tool-versions
instead of 'latest', scopes the push trigger to main so PR branches stop
double-running, and extends kind coverage from install-only to
install/upgrade/uninstall with a CRD-retention assertion."
```

---

### Task 5: Release workflow

Publish a signed chart and multi-arch image to ghcr on every published GitHub Release.

**Files:**
- Create: `.github/workflows/release.yml`

**Interfaces:**
- Consumes: `make build-installer` producing `dist/install.yaml` + `dist/crds.yaml` (Task 2), `make helm-verify` (Task 3), the anchored `version:`/`appVersion:` lines in `Chart.yaml` (Task 1).
- Produces: `ghcr.io/konsole-is/charts/camunda-operator:<tag>`, `ghcr.io/konsole-is/camunda-operator:<tag>`, and release assets `install.yaml`, `crds.yaml`, `camunda-operator-<tag>.tgz`.

- [ ] **Step 1: Write the failing test**

Create `/tmp/release-check.sh` (scratch, not committed):

```bash
#!/usr/bin/env bash
set -euo pipefail
f=.github/workflows/release.yml

[ -f "$f" ] || { echo "FAIL: $f does not exist"; exit 1; }

echo "--- keyless cosign needs id-token: write ---"
grep -q 'id-token: write' "$f" || { echo "FAIL: no id-token permission; keyless signing will fail"; exit 1; }

echo "--- both artifacts signed ---"
[ "$(grep -c 'cosign sign' "$f")" -ge 2 ] || { echo "FAIL: expected chart and image to both be signed"; exit 1; }

echo "--- signing by digest, never by mutable tag ---"
grep -qE 'cosign sign .*\$\{\{? ?[A-Za-z_]*DIGEST' "$f" || { echo "FAIL: cosign must sign digests"; exit 1; }

echo "--- version stamped into Chart.yaml ---"
grep -q 'appVersion' "$f" || { echo "FAIL: appVersion never stamped"; exit 1; }

echo "--- release assets uploaded ---"
for asset in install.yaml crds.yaml; do
  grep -q "$asset" "$f" || { echo "FAIL: $asset not uploaded"; exit 1; }
done

echo "--- triggered by published release ---"
grep -q 'types: \[published\]' "$f" || { echo "FAIL: wrong trigger"; exit 1; }

echo "PASS"
```

- [ ] **Step 2: Run it to verify it fails**

Run: `chmod +x /tmp/release-check.sh && /tmp/release-check.sh`
Expected: FAIL — `.github/workflows/release.yml does not exist`.

- [ ] **Step 3: Write the workflow**

Create `.github/workflows/release.yml`:

```yaml
name: Release

on:
  release:
    types: [published]

jobs:
  release:
    name: Build and publish artifacts
    runs-on: ubuntu-latest

    permissions:
      contents: write   # upload release assets
      packages: write   # push to ghcr
      id-token: write   # keyless cosign signing via OIDC

    env:
      # Release tags are unprefixed SemVer (0.1.0), because Helm chart
      # versions must be bare SemVer.
      TAG: ${{ github.ref_name }}
      IMG: ghcr.io/${{ github.repository }}:${{ github.ref_name }}
      CHART_REGISTRY: oci://ghcr.io/konsole-is/charts
      CHART_REPO: ghcr.io/konsole-is/charts/camunda-operator

    steps:
      - name: Clone the code
        uses: actions/checkout@v4

      - name: Setup Go
        uses: actions/setup-go@v5
        with:
          go-version-file: go.mod

      - name: Reject a v-prefixed tag
        run: |
          if [[ "${TAG}" == v* ]]; then
            echo "Tag '${TAG}' is v-prefixed. Helm chart versions must be bare SemVer." >&2
            exit 1
          fi

      # Versions below are pinned to .tool-versions. Keep them in sync.
      - name: Install kubebuilder
        run: |
          VERSION=4.13.1
          OS=$(go env GOOS)
          ARCH=$(go env GOARCH)
          curl -fsSL -o kubebuilder \
            "https://github.com/kubernetes-sigs/kubebuilder/releases/download/v${VERSION}/kubebuilder_${OS}_${ARCH}"
          chmod +x kubebuilder && sudo mv kubebuilder /usr/local/bin/

      - name: Install Helm
        uses: azure/setup-helm@v4.3.1
        with:
          version: v4.1.4

      - name: Install cosign
        uses: sigstore/cosign-installer@v3

      - name: Set up Docker buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Generate the chart and installer manifests
        run: make helm-generate IMG=${IMG}

      - name: Stamp the release version into Chart.yaml
        run: |
          sed -i -e "s/^version: .*/version: ${TAG}/" dist/chart/Chart.yaml
          sed -i -e "s/^appVersion: .*/appVersion: \"${TAG}\"/" dist/chart/Chart.yaml
          grep -E '^(version|appVersion):' dist/chart/Chart.yaml

      - name: Verify the chart
        run: make helm-verify

      - name: Build and push the operator image
        run: make docker-buildx IMG=${IMG}

      - name: Resolve the image digest
        id: image
        run: |
          digest=$(docker buildx imagetools inspect "${IMG}" \
            --format '{{.Manifest.Digest}}')
          echo "digest=${digest}" >> "$GITHUB_OUTPUT"
          echo "image digest: ${digest}"

      - name: Package and push the chart
        id: chart
        run: |
          helm package dist/chart --destination dist
          helm push "dist/camunda-operator-${TAG}.tgz" "${CHART_REGISTRY}" 2>&1 | tee push.log
          digest=$(grep -oE 'sha256:[0-9a-f]{64}' push.log | head -1)
          if [ -z "$digest" ]; then
            echo "Could not parse the chart digest from helm push output." >&2
            exit 1
          fi
          echo "digest=${digest}" >> "$GITHUB_OUTPUT"
          echo "chart digest: ${digest}"

      # Sign by digest, never by tag — tags are mutable.
      - name: Sign the image
        run: cosign sign --yes "ghcr.io/${{ github.repository }}@${{ steps.image.outputs.digest }}"

      - name: Sign the chart
        run: cosign sign --yes "${CHART_REPO}@${{ steps.chart.outputs.digest }}"

      - name: Upload release assets
        uses: softprops/action-gh-release@v2
        with:
          files: |
            dist/install.yaml
            dist/crds.yaml
            dist/camunda-operator-${{ github.ref_name }}.tgz
```

- [ ] **Step 4: Run the test to verify it passes**

Run: `/tmp/release-check.sh`
Expected: PASS.

- [ ] **Step 5: Validate the YAML parses**

Run: `python3 -c "import yaml,sys; yaml.safe_load(open('.github/workflows/release.yml')); print('valid YAML')"`
Expected: `valid YAML`

- [ ] **Step 6: Verify the version-stamping sed against the real Chart.yaml**

```bash
cp dist/chart/Chart.yaml /tmp/Chart.yaml.bak
TAG=1.2.3
sed -i -e "s/^version: .*/version: ${TAG}/" dist/chart/Chart.yaml
sed -i -e "s/^appVersion: .*/appVersion: \"${TAG}\"/" dist/chart/Chart.yaml
grep -E '^(version|appVersion):' dist/chart/Chart.yaml
helm lint dist/chart
cp /tmp/Chart.yaml.bak dist/chart/Chart.yaml
git diff --exit-code dist/chart/Chart.yaml && echo "Chart.yaml restored"
```

Expected: `version: 1.2.3` and `appVersion: "1.2.3"`, lint passes, Chart.yaml restored with no diff. Confirms the sed anchors from Task 1 hold and that `helm package` will accept the stamped file.

- [ ] **Step 7: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "feat: publish signed chart and image on release

On a published GitHub Release, builds the amd64/arm64 operator image and
the Helm chart, pushes both to ghcr, and signs both by digest with cosign
keyless. Attaches install.yaml, crds.yaml, and the packaged chart to the
release for non-Helm installs.

Rejects v-prefixed tags up front, since Helm chart versions must be bare
SemVer."
```

---

### Task 6: Installation documentation

No installation documentation exists — `README.md` is one line, and `docs/index.md` points at the README for installation.

**Files:**
- Create: `docs/installation.md`
- Modify: `README.md`, `docs/index.md:14`, `mkdocs.yml:71-72`, `AGENTS.md:270-295`

**Interfaces:**
- Consumes: the published references and release assets from Task 5.
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Write the failing test**

Run: `make docs-build`
Expected: currently succeeds, but `docs/installation.md` does not exist. After Step 2 adds the nav entry before the file exists, `mkdocs build --strict` fails on the missing target — that is the failing state this task closes.

- [ ] **Step 2: Add the nav entry**

In `mkdocs.yml`, in the `nav:` block, insert between `Home` and `Architecture`:

```yaml
  - Installation: installation.md
```

- [ ] **Step 3: Run the build to verify it fails**

Run: `make docs-build`
Expected: FAIL — strict mode reports a nav entry pointing at a non-existent `installation.md`.

- [ ] **Step 4: Write `docs/installation.md`**

```markdown
# Installation

The operator is distributed as a Helm chart and a plain Kubernetes manifest, both
published with every release. Charts and images live in GitHub Container Registry
and are signed with [cosign](https://docs.sigstore.dev/) keyless signatures.

**Requirements:** Kubernetes 1.30 or later, and Helm 3.8+ for the OCI registry
(the repository pins Helm 4.1.4).

## Install with Helm

```bash
helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --create-namespace
```

This installs the controller manager and all 19 custom resource definitions.

Replace `<version>` with a released version — for example `0.1.0`. Release tags
are unprefixed SemVer, and the chart version, chart `appVersion`, and operator
image tag are always the same string.

### Configuration

The chart's values and their defaults are documented in the
[chart README](https://github.com/konsole-is/camunda-operator/blob/main/dist/chart/README.md).
The values you are most likely to touch:

```bash
helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system --create-namespace \
  --set manager.replicas=2 \
  --set prometheus.enable=true \
  --set rbacHelpers.enable=true
```

- `prometheus.enable` installs a `ServiceMonitor`. It requires the
  prometheus-operator CRDs to already be present in the cluster.
- `rbacHelpers.enable` installs convenience admin, editor, and viewer
  `ClusterRole`s for each custom resource.

## Install without Helm

Every release attaches a rendered manifest:

```bash
kubectl apply -f https://github.com/konsole-is/camunda-operator/releases/download/<version>/install.yaml
```

This is the same content the chart renders at its defaults, with the namespace
fixed to `camunda-operator-system`.

## Installing CRDs out of band

By default the chart installs the CRDs. Everything a chart renders is stored in
the Helm release Secret, which is bounded by etcd's object size limit, so
clusters that also carry large custom resources may prefer to apply the CRDs
separately:

```bash
kubectl apply --server-side -f https://github.com/konsole-is/camunda-operator/releases/download/<version>/crds.yaml

helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system --create-namespace \
  --set crd.enable=false
```

With `crd.enable=false`, CRD lifecycle is yours to manage: apply the updated
`crds.yaml` before upgrading the chart.

## Verifying signatures

Both the chart and the operator image are signed with cosign keyless signatures
tied to the release workflow's GitHub identity. There is no public key to
distribute — verification checks the signing identity instead:

```bash
cosign verify ghcr.io/konsole-is/charts/camunda-operator:<version> \
  --certificate-identity-regexp '^https://github\.com/konsole-is/camunda-operator/\.github/workflows/release\.yml@refs/tags/.+$' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com

cosign verify ghcr.io/konsole-is/camunda-operator:<version> \
  --certificate-identity-regexp '^https://github\.com/konsole-is/camunda-operator/\.github/workflows/release\.yml@refs/tags/.+$' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

A successful verification prints the certificate subject and the matched claims.

## Upgrading

```bash
helm upgrade camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <new-version> \
  --namespace camunda-operator-system
```

CRDs are annotated `helm.sh/resource-policy: keep` by default, so Helm updates
them in place and never deletes them.

## Uninstalling

```bash
helm uninstall camunda-operator --namespace camunda-operator-system
```

Because of the `keep` policy, the CRDs and every custom resource stored in them
survive an uninstall. To remove them — **this deletes every Camunda cluster the
operator manages** — delete the CRDs explicitly:

```bash
kubectl delete crd -l app.kubernetes.io/name=camunda-operator
```

## Installing from source

```bash
git clone https://github.com/konsole-is/camunda-operator
cd camunda-operator
make helm-generate IMG=<registry>/camunda-operator:<tag>
make helm-deploy   IMG=<registry>/camunda-operator:<tag>
```

`make helm-generate` regenerates `dist/chart/` from `config/`; the chart is not
checked into the repository.
```

- [ ] **Step 5: Run the build to verify it passes**

Run: `make docs-build`
Expected: build succeeds in strict mode, no warnings.

- [ ] **Step 6: Point `docs/index.md` at the new page**

In `docs/index.md`, replace:

```markdown
See the [README](https://github.com/konsole-is/camunda-operator#readme) for installation and quick-start instructions.
```

with:

```markdown
See the [installation guide](installation.md) for Helm and manifest-based installation, signature verification, and upgrades.
```

Also add to the "Where to go" list, after the Architecture bullet:

```markdown
- [Installation](installation.md) — installing the operator with Helm or plain manifests.
```

- [ ] **Step 7: Write `README.md`**

Replace the entire contents:

```markdown
# Camunda Operator

Core Kubernetes operator for the Camunda platform. Manages the orchestration
cluster lifecycle, storage backends, backup and restore, Optimize, and the
management plane.

It is the bottom layer of a three-operator stack: cloud- and SaaS-level
operators may create this operator's custom resources, but this operator has no
knowledge of them and runs standalone on any Kubernetes cluster.

## Install

```bash
helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --create-namespace
```

Requires Kubernetes 1.30+. For plain-manifest installation, signature
verification, out-of-band CRD installation, and upgrades, see the
[installation guide](docs/installation.md).

## Documentation

- [Installation](docs/installation.md) — Helm and manifest installation.
- [Architecture](docs/architecture.md) — the extension model and how features
  attach to workloads.
- [CRD reference](docs/crds/index.md) — all 19 custom resource definitions.

## Development

```bash
make test           # run unit and envtest suites
make all            # lint and format
make helm-generate  # regenerate dist/chart/ from config/
make helm-verify    # lint and render the chart, no cluster needed
```

The Helm chart is generated, not checked in — only `dist/chart/Chart.yaml` and
`dist/chart/README.md` are versioned. Never hand-edit `dist/chart/values.yaml`
or `dist/chart/templates/`; regeneration overwrites them. Change defaults in
`config/` instead.
```

Note: no License section. This repository has no `LICENSE` file, so linking one
would be a broken link. Choosing a license is a separate decision — flagged in
"Open questions" below.

- [ ] **Step 8: Update the Helm section in `AGENTS.md`**

Replace the "**Important:**" block at `AGENTS.md:292-295` — its advice to back up and restore `values.yaml` customizations contradicts this project's rule — with:

```markdown
**Important:** `dist/chart/values.yaml` and `dist/chart/templates/` are
regenerated by `kubebuilder edit --plugins=helm/v2-alpha --force` and must never
be hand-edited. Shipped defaults come from `config/`. `dist/chart/Chart.yaml` is
preserved by `--force` and *is* hand-maintained.

**Releasing:** publishing a GitHub Release on an unprefixed SemVer tag (`0.1.0`)
triggers `.github/workflows/release.yml`, which pushes the chart to
`oci://ghcr.io/konsole-is/charts`, pushes the amd64/arm64 image to
`ghcr.io/konsole-is/camunda-operator`, signs both with cosign keyless, and
attaches `install.yaml`, `crds.yaml`, and the packaged chart to the release.
```

- [ ] **Step 9: Verify docs build and links resolve**

Run: `make docs-build`
Expected: succeeds in strict mode. Strict mode fails on broken internal links, so this also validates the `installation.md` cross-references.

- [ ] **Step 10: Commit**

```bash
git add docs/installation.md docs/index.md mkdocs.yml README.md AGENTS.md
git commit -m "docs: add installation guide

Installation documentation did not exist — the README was a single
heading and docs/index.md pointed at it. Adds a canonical guide covering
Helm OCI install, plain manifests, out-of-band CRD installation, cosign
verification, upgrades, and uninstall.

Corrects the AGENTS.md advice to back up and restore values.yaml
customizations, which contradicts this project's generate-only rule."
```

---

## Post-implementation: manual steps

Neither can be automated; both block a working first release.

1. **Make the chart package public.** ghcr packages are private on first publish. After the first release, open the `charts/camunda-operator` package in the konsole-is org and set its visibility to public. Until then `helm install` fails with an auth error even though the release succeeded.
2. **Dry-run a release before the first real one.** Publish a prerelease tag (`0.0.1-rc.1`), then confirm: the workflow completes, `helm pull oci://ghcr.io/konsole-is/charts/camunda-operator --version 0.0.1-rc.1` succeeds, `cosign verify` passes with the command from `docs/installation.md`, and a kind install from the published reference comes up healthy.

Note that konsole-is GitHub Actions billing was reported broken as of 2026-07-31. Both workflows in this plan need Actions minutes; confirm billing is restored before relying on CI.

## Open questions

Neither blocks implementation; both want a decision before the first public release.

1. **No `LICENSE` file.** `fqdn-controller` ships Apache-2.0; this repository has none. A published chart and image with no license is ambiguous for consumers. Task 6's README deliberately omits a License section rather than link a file that does not exist.
2. **`kubeVersion: ">= 1.30.0-0"`** in Task 1 is a conservative judgment call, not a measured floor. Nothing in the codebase pins a minimum Kubernetes version. If a real floor is established later, update `Chart.yaml` — Helm enforces this constraint at install time, so setting it too high locks out working clusters.
