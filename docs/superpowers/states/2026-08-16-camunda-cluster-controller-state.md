---
feature: camunda-cluster-controller
spec: docs/superpowers/specs/2026-08-16-camunda-cluster-controller-design.md
plan: docs/superpowers/plans/2026-08-16-camunda-cluster-controller-plan.md
tracking_issue: #47
feature_branch: feat/camunda-cluster-controller
feature_worktree: .claude/worktrees/camunda-cluster-controller
sub_pr_approval: autonomous
sub_pr_review_loop: on
sub_pr_target: feature-branch
integration_pr: #58
status: review
---

# CamundaCluster controller (Batch C) — orchestration state

## Phases

- **Wave 1 (foundational, parallel)** — `#48` platform config, `#49` cluster API types
- **Wave 2** — `#50` components (pure rendering, goldens)
- **Wave 3** — `#51` controller
- **Wave 4** — `#52` e2e and docs
- **Integration** — `feat/camunda-cluster-controller` → `main` (`Closes #47`), stop for the user's review

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #48 | batch-c/platform-config | .claude/worktrees/camunda-cluster-controller--platform-config | #53 → feat/camunda-cluster-controller | self-merged |
| #49 | batch-c/cluster-api-types | .claude/worktrees/camunda-cluster-controller--cluster-api-types | #54 → feat/camunda-cluster-controller | self-merged |
| #50 | batch-c/cluster-components | .claude/worktrees/camunda-cluster-controller--cluster-components | #55 → feat/camunda-cluster-controller | self-merged |
| #51 | batch-c/cluster-controller | .claude/worktrees/camunda-cluster-controller--cluster-controller | #56 → feat/camunda-cluster-controller | self-merged |
| follow-up (#47) | batch-c/crd-size | .claude/worktrees/camunda-cluster-controller--crd-size | #59 → feat/camunda-cluster-controller | draft (parked alternative) |
| follow-up (#47) | batch-c/chart-size-guard | .claude/worktrees/camunda-cluster-controller--chart-size-guard | — → feat/camunda-cluster-controller | in-progress |
| #52 | batch-c/cluster-e2e | .claude/worktrees/camunda-cluster-controller--cluster-e2e | #57 → feat/camunda-cluster-controller | self-merged |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `samples-allowlist` | data-only | plan "Contracts" | locked |
| `secondary-storage-chain` | data-only (Batch B types on main) | plan "Contracts" | locked |

The interfaces between the sequential PRs (B → C → D) are the **Interfaces** blocks of the plan tasks; they are locked when the producing PR self-merges.

## Bubble-up log

- 2026-08-16 — Chart budget decision REVISED: Copilot on #59 named the cost of schemaless `scheduling` (wrongly typed values break the typed decode → the cluster silently stops reconciling; typos ignored). Measured the real bound: worst-case chart gzips to 113,472 B against etcd's ~1 MiB (9× headroom); the Makefile guard says it is a proxy/tripwire. Decision: keep typed schemas; make `helm-verify` measure gzipped bytes with a 512 KiB limit (sub-PR `batch-c/chart-size-guard`); #59 parked as a draft alternative for the user. USER DECISION POINT: pick one; the CRD-split into its own chart remains the durable fix before Batch D.
- 2026-08-16 — Integration PR #58: CI is live again (billing fixed); the `Chart` workflow fails: rendered chart 1,170,321 B (worst 1,205,659) > the 1,048,576 B proxy limit, because the CamundaCluster/preset CRDs carry the `scheduling` schema (~22 KB) in 8 places each. Decision: schemaless `scheduling` on the CamundaCluster types (typed Go, validated at workload apply); follow-up sub-PR `batch-c/crd-size`. Copilot cannot review #58 (diff > 20k lines); each sub-PR was reviewed. Recommendation for Batch D: split the CRDs into a separate chart before the set grows again.
- 2026-08-16 — PR E (#57): `make test-e2e` green locally on a fresh kind cluster (23/23 specs, 809 s; ES flow ~3 min to Ready, RDBMS ~2 min); no operator fix was needed. Docs reconciled (endpoints, env layering, ServiceMonitor paths, mirrored Secrets, imageRegistry example, JAVA_TOOL_OPTIONS); spec 'Watches and indexes' and the PVC-patch sentence amended to what shipped. CI could not be observed (konsole-is Actions billing).
- 2026-08-16 — PR D (#56) merged after 2 Copilot rounds + spec/quality passes. Quality pass fixed: apply skipped while the orphan-deleted StatefulSet terminates (sentinel, no backoff); cross-namespace binding/DatabaseConfig credential Secrets now watched (Batch B index constants exported); clamp folds applied template + requests; PVC RBAC trimmed. Round-2 suppressed 'grow Pending PVCs' declined (API rejects requests changes on unbound claims).
- 2026-08-16 — From PR D (#56): (1) ocf records an `Updated <Kind>` event on every reconcile (DeepEqual against a defaulted live object), which trips client-go's event spam filter and drops the operator's own events (`Paused`, `StorageShrinkIgnored`, `StatefulSetRecreated`) in real clusters; `internal/testenv` raises the burst for suites only. Follow-up for the user: fix in ocf (sourcehawk/operator-component-framework) or set a manager-level EventBroadcaster. (2) After resume, ocf keeps `Ready=True` with reason `Updating` until healthy (documented in the test). (3) `MirroredSecretComponent(cluster, map[purpose]data)` — one component for all mirrored Secrets; plan text differs, code wins. (4) A preset's `auth.clientSecretRef` is indexed and watched too. (5) envtest has no GC: the orphan-delete test removes the finalizer itself; growth needs a StorageClass with `allowVolumeExpansion`.
- 2026-08-16 — PR C (#55) merged after 3 Copilot rounds (round 1: 3 false 'won't compile' comments on Go 1.26 `new(expr)`, declined; 2 comment fixes applied) and the quality pass (per-process `ConfigHash(in, p)` — contract updated in the plan; embedded-gateway env layered onto zeebe; ServiceMonitor path `/actuator/prometheus`). Round-3 suppressed 'validate in Build' declined by design (pre-check owns validation, D1).
- 2026-08-16 — From PR C (#55): (1) cross-namespace Secrets cannot be `secretKeyRef`ed → decision: PR D mirrors them into the cluster namespace (`<name>-camunda-<purpose>`), renderer gets local names; plan Task D1 updated. (2) `imageRegistry` is a prefix before `camunda/camunda`; the platform-config doc example `registry.example.com/camunda` would double `camunda` — PR E changes the doc example to `registry.example.com` (semantics unchanged). (3) Embedded web-app `extraEnv` applies to the host process, layered global → embedded apps → the process's own block — PR E documents it. (4) Doc examples still show `JAVA_OPTS`; PR E unifies on `JAVA_TOOL_OPTIONS`. (5) Admin Secret uses `Data` (ocf normalizes StringData); the `suspended` golden equals `minimal` because suspension is a runtime mutation.
- 2026-08-16 — Copilot loop on #54 hit the 3-round cap with one trivial suppressed nit left (nil-preset deep copy); the author applied it (`90c323e`) and the PR merged without a fourth round. Loop on #53 converged in round 3.
- 2026-08-16 — PR B (#54): the six CamundaCluster CEL rules moved from the shared `CamundaClusterSpec` type to the `Spec` field of `CamundaCluster`, so a preset can still lower partitions/storageSize (the controller clamps). Plan Task B1 updated to match; C and D unaffected. Also from B: `Effective.Replicas`/`Workload` switch on literal component names — PR C replaces them with the `Component*` constants when `names.go` lands; the ES-specific GoDoc of the shared `PersistentVolumeClaimRetentionPolicy`/`ServiceMonitorSpec` types is generalized in PR E. From PR A (#53): the RBAC role narrows to get/list/watch for platform configs; PR C must render `jwk-set-uri`/`token-uri` (doc advice for split-horizon) and default `audiences` to the client id.
- 2026-08-16 — Spec amended before planning: `camunda.mode` replaced by Spring profiles (gateway mode loses `consolidated-auth` when the auth method is set), node id through a command wrapper, JDBC bundled, redirect `<externalUrl>/sso-callback`, `issuerBackendUrl` dropped, connectors image `camunda/connectors-bundle` with its own `spec.connectors.version`, `CamundaPlatformConfig` types and controller added to the batch (they were still scaffolds). Propagated to the spec, the epic, and the plan.
- 2026-08-16 — Watch strategy for the deep Secret chain decided in the plan (Task D2): a Secret watch with a map handler (same namespace + own auth index + platform config index), `DatabaseConfig` by namespace, `DatabaseServerConfig` to all clusters. Reason: a validation controller re-checking a Secret writes nothing when the condition is unchanged, so status bumps cannot fan out.

## Pending snapshot

1. All five sub-PRs (#53–#57) self-merged; #48–#52 closed. Run `feature-dev-workflow:reviewing-feature-progress` on the feature worktree (full `make test`, `make lint`; e2e already green on #57's head).
2. Open the integration PR `feat/camunda-cluster-controller` → `main` with `Closes #47`; Copilot loop; stop at ready-to-merge for the user's review (the merge to main is the user's).
3. After the user merges: tear down plan + state file, keep the spec, delete worktrees/branches, update memory.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature` (parallel waves still in flight; the orchestrator watch loop continues), or work the `## Pending snapshot` when development is past.
