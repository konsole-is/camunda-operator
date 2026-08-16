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
integration_pr:
status: consumer-wave
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
| #52 | batch-c/cluster-e2e | .claude/worktrees/camunda-cluster-controller--cluster-e2e | — → feat/camunda-cluster-controller | not-started |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `samples-allowlist` | data-only | plan "Contracts" | locked |
| `secondary-storage-chain` | data-only (Batch B types on main) | plan "Contracts" | locked |

The interfaces between the sequential PRs (B → C → D) are the **Interfaces** blocks of the plan tasks; they are locked when the producing PR self-merges.

## Bubble-up log

- 2026-08-16 — PR D (#56) merged after 2 Copilot rounds + spec/quality passes. Quality pass fixed: apply skipped while the orphan-deleted StatefulSet terminates (sentinel, no backoff); cross-namespace binding/DatabaseConfig credential Secrets now watched (Batch B index constants exported); clamp folds applied template + requests; PVC RBAC trimmed. Round-2 suppressed 'grow Pending PVCs' declined (API rejects requests changes on unbound claims).
- 2026-08-16 — From PR D (#56): (1) ocf records an `Updated <Kind>` event on every reconcile (DeepEqual against a defaulted live object), which trips client-go's event spam filter and drops the operator's own events (`Paused`, `StorageShrinkIgnored`, `StatefulSetRecreated`) in real clusters; `internal/testenv` raises the burst for suites only. Follow-up for the user: fix in ocf (sourcehawk/operator-component-framework) or set a manager-level EventBroadcaster. (2) After resume, ocf keeps `Ready=True` with reason `Updating` until healthy (documented in the test). (3) `MirroredSecretComponent(cluster, map[purpose]data)` — one component for all mirrored Secrets; plan text differs, code wins. (4) A preset's `auth.clientSecretRef` is indexed and watched too. (5) envtest has no GC: the orphan-delete test removes the finalizer itself; growth needs a StorageClass with `allowVolumeExpansion`.
- 2026-08-16 — PR C (#55) merged after 3 Copilot rounds (round 1: 3 false 'won't compile' comments on Go 1.26 `new(expr)`, declined; 2 comment fixes applied) and the quality pass (per-process `ConfigHash(in, p)` — contract updated in the plan; embedded-gateway env layered onto zeebe; ServiceMonitor path `/actuator/prometheus`). Round-3 suppressed 'validate in Build' declined by design (pre-check owns validation, D1).
- 2026-08-16 — From PR C (#55): (1) cross-namespace Secrets cannot be `secretKeyRef`ed → decision: PR D mirrors them into the cluster namespace (`<name>-camunda-<purpose>`), renderer gets local names; plan Task D1 updated. (2) `imageRegistry` is a prefix before `camunda/camunda`; the platform-config doc example `registry.example.com/camunda` would double `camunda` — PR E changes the doc example to `registry.example.com` (semantics unchanged). (3) Embedded web-app `extraEnv` applies to the host process, layered global → embedded apps → the process's own block — PR E documents it. (4) Doc examples still show `JAVA_OPTS`; PR E unifies on `JAVA_TOOL_OPTIONS`. (5) Admin Secret uses `Data` (ocf normalizes StringData); the `suspended` golden equals `minimal` because suspension is a runtime mutation.
- 2026-08-16 — Copilot loop on #54 hit the 3-round cap with one trivial suppressed nit left (nil-preset deep copy); the author applied it (`90c323e`) and the PR merged without a fourth round. Loop on #53 converged in round 3.
- 2026-08-16 — PR B (#54): the six CamundaCluster CEL rules moved from the shared `CamundaClusterSpec` type to the `Spec` field of `CamundaCluster`, so a preset can still lower partitions/storageSize (the controller clamps). Plan Task B1 updated to match; C and D unaffected. Also from B: `Effective.Replicas`/`Workload` switch on literal component names — PR C replaces them with the `Component*` constants when `names.go` lands; the ES-specific GoDoc of the shared `PersistentVolumeClaimRetentionPolicy`/`ServiceMonitorSpec` types is generalized in PR E. From PR A (#53): the RBAC role narrows to get/list/watch for platform configs; PR C must render `jwk-set-uri`/`token-uri` (doc advice for split-horizon) and default `audiences` to the client id.
- 2026-08-16 — Spec amended before planning: `camunda.mode` replaced by Spring profiles (gateway mode loses `consolidated-auth` when the auth method is set), node id through a command wrapper, JDBC bundled, redirect `<externalUrl>/sso-callback`, `issuerBackendUrl` dropped, connectors image `camunda/connectors-bundle` with its own `spec.connectors.version`, `CamundaPlatformConfig` types and controller added to the batch (they were still scaffolds). Propagated to the spec, the epic, and the plan.
- 2026-08-16 — Watch strategy for the deep Secret chain decided in the plan (Task D2): a Secret watch with a map handler (same namespace + own auth index + platform config index), `DatabaseConfig` by namespace, `DatabaseServerConfig` to all clusters. Reason: a validation controller re-checking a Secret writes nothing when the condition is unchanged, so status bumps cannot fan out.

## Pending snapshot

1. Push the feature branch (`git push -u origin feat/camunda-cluster-controller`).
2. Invoke `feature-dev-workflow:developing-a-feature`; dispatch Wave 1 (#48, #49) in parallel with `feature-dev-workflow:fanning-out-with-worktrees`; Copilot loop on each sub-PR; self-merge into the feature branch.
3. Waves 2, 3, 4 sequentially; update this file after each burst.
4. `feature-dev-workflow:reviewing-feature-progress`, then the integration PR with `Closes #47`; Copilot loop; stop and hand to the user for review.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature` (parallel waves still in flight; the orchestrator watch loop continues), or work the `## Pending snapshot` when development is past.
