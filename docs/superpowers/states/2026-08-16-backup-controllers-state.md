---
feature: backup-controllers
spec: docs/superpowers/specs/2026-08-16-backup-controllers-design.md
plan: docs/superpowers/plans/2026-08-16-backup-controllers-plan.md
tracking_issue: #64
feature_branch: feat/backup-controllers
feature_worktree: .claude/worktrees/backup-controllers
sub_pr_approval: autonomous   # EXCEPT #68, #69, integration PR: manual (user reviews; see Standing user directives)
sub_pr_review_loop: on        # no round cap — loop until clean
sub_pr_target: feature-branch
integration_pr:
status: consumer-wave
---

# Backup controllers — orchestration state

## Standing user directives

- Full autonomy through implementation; review-loop every PR until clean with **no round cap** — keep going while findings remain.
- PRs for #68 (LogicalBackupElasticsearch) and #69 (LogicalBackupRDBMS) and the integration PR: get clean, then **stop and request the user's own review; never merge these three without it**. All other sub-PRs self-merge after a clean loop.

## Phases

- **Phase 1 (foundation)** — #65
- **Phase 2 (wiring, fan-out)** — #66, #67
- **Phase 3 (controllers, fan-out; user-reviewed)** — #68, #69
- **Phase 4 (schedule)** — #70
- **Phase 5 (e2e + docs)** — #71

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #65 | feat/backup-controllers--foundation | (removed) | #74 → feat/backup-controllers | self-merged |
| #66 | feat/backup-controllers--es-snapshot-repository | .claude/worktrees/backup-controllers--es-snapshot-repository | → feat/backup-controllers | not-started |
| #67 | feat/backup-controllers--cluster-backup-wiring | .claude/worktrees/backup-controllers--cluster-backup-wiring | → feat/backup-controllers | not-started |
| #68 | feat/backup-controllers--lbes-controller | .claude/worktrees/backup-controllers--lbes-controller | → feat/backup-controllers | not-started |
| #69 | feat/backup-controllers--lbrdbms-controller | .claude/worktrees/backup-controllers--lbrdbms-controller | → feat/backup-controllers | not-started |
| #70 | feat/backup-controllers--backup-schedule | .claude/worktrees/backup-controllers--backup-schedule | → feat/backup-controllers | not-started |
| #71 | test/backup-controllers--e2e-minio | .claude/worktrees/backup-controllers--e2e-minio | → feat/backup-controllers | not-started |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| bucket-contract | merged producer PR | #74 | locked |
| objectstore-api | merged producer PR (`List` became streaming `Walk`; `Upload` aborts partial writes) | #74 | locked |
| esadmin-api | merged producer PR (`New` now returns `(*Client, error)`) | #74 | locked |
| camundaadmin-api | merged producer PR (+`ErrConflict`; four verified 8.9 protocol corrections) | #74 | locked |
| logicalbackup-skeleton | merged producer PR (status vocabulary in `api/v1`; `PreCheckRequest` with injected `InProgress`) | #74 | locked |
| snapshot-repository-field | pre-merge stub committed to the feature branch so #66 and #67 build in parallel; #66 fills it, #67 reads it | feat/backup-controllers@1ba9d1c | locked |
| management-binding | merged producer PR | #67's PR | pending |
| backup-kind-types | PR6 branches after #68+#69 merge | n/a | pending |

## Bubble-up log

- **2026-08-17 — Copilot review unavailable on this repo; orchestrator review is the gate.** `gh pr edit --add-reviewer "@copilot"` exits 0 but the reviewer never attaches (verified twice on #74), which is the documented signal for "unavailable for this repo/plan"; consistent with the org's known billing state. Effect: `sub_pr_review_loop: on` cannot run for any PR in this epic. Resolution: the orchestrator's two-stage review (spec-compliance gate, then code quality) is the gate on every sub-PR, run harder to compensate; the user's "no round cap" directive applies to that loop instead. Durable fix if the entitlement is restored: a repo ruleset running Copilot code review on push.
- **2026-08-17 — PR1 contract deviations reviewed and accepted; plan amended.** Status vocabulary (`LogicalBackupPhase`, shared reasons, `ClusterRef`, `LogicalBackupStorageSizes`) lives in `api/v1`, not `pkg/logicalbackup` — it is CRD status surface needing deepcopy and enum markers, per CLAUDE.md and `api/v1/conditions.go`. `PreCheck` takes a request struct with an injected `InProgress` because the backup kinds do not exist until PR4/PR5. Per-kind reasons deferred to their kinds. Propagated: the `logicalbackup-skeleton` contract row now records the shipped shape; PR4 and PR5 sections carry the reason declarations and the "InProgress lists both kinds" requirement.
- **2026-08-17 — `make fmt` corrupts CEL in declaration doc comments.** gofmt normalizes `''` to typographic quotes in doc comments above type declarations, producing a rule Kubernetes cannot compile; tests stayed green because the committed CRD YAML still held the old rule. Found and fixed in #74 (`size() > 0`). Propagated to the plan's Conventions block — PR2 and PR3 both add CEL to types and must re-run `make manifests` and confirm the regenerated YAML holds the rule as written.
- **2026-08-17 — CRD doc ownership settled.** The plan (PR3), the Conventions block (owning PR), and #74's body (PR7) gave three different answers for `docs/crds/objectstorageconfig.md`, and the page contradicted its own shipped CRD. Rule now in Conventions: the PR that changes a CRD's schema rewrites that CRD's page in the same PR; PR7 owns only the backup-page split/removal, the prose sweep, and `mkdocs.yml` nav. PR3's step list updated.
- **2026-08-17 — GCS/Azure credential-shape verification was missed in PR1.** Spec §ObjectStorageConfig requires it before the types are final; #74 verified only the management-port auth. Routed back to the #65 implementer; if it forces a type change it lands in #74 before merge, since PR2/PR3/PR5 all build on those types.

## Pending snapshot

1. Phase 2 in flight: #66 and #67 dispatched in parallel worktrees. On each ready — two-stage orchestrator review (Copilot is unavailable, see bubble-up), fix-loop to clean with no round cap, self-merge, `gh issue close <n>`, lock `snapshot-repository-field` and `management-binding`.
3. Phase 2: fan out #66 and #67 in parallel worktrees; loop to clean; self-merge; close.
4. Phase 3: fan out #68 and #69; loop to clean; **stop for user review of both PRs**; merge on approval; close.
5. Phase 4: #70; loop; self-merge; close.
6. Phase 5: #71; loop; self-merge; close.
7. `reviewing-feature-progress` checkpoint, open integration PR (`Closes #64`), loop to clean, **stop for user review**; on approval merge, delete plan+state in the final commit, update memory.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb`.
6. Re-dispatch subagents per `feature-dev-workflow:developing-a-feature`, or work the `## Pending snapshot`.
