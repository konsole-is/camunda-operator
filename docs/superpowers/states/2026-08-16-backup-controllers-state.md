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
status: foundational-wave
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
| #65 | feat/backup-controllers--foundation | .claude/worktrees/backup-controllers--foundation | → feat/backup-controllers | in-progress |
| #66 | feat/backup-controllers--es-snapshot-repository | .claude/worktrees/backup-controllers--es-snapshot-repository | → feat/backup-controllers | not-started |
| #67 | feat/backup-controllers--cluster-backup-wiring | .claude/worktrees/backup-controllers--cluster-backup-wiring | → feat/backup-controllers | not-started |
| #68 | feat/backup-controllers--lbes-controller | .claude/worktrees/backup-controllers--lbes-controller | → feat/backup-controllers | not-started |
| #69 | feat/backup-controllers--lbrdbms-controller | .claude/worktrees/backup-controllers--lbrdbms-controller | → feat/backup-controllers | not-started |
| #70 | feat/backup-controllers--backup-schedule | .claude/worktrees/backup-controllers--backup-schedule | → feat/backup-controllers | not-started |
| #71 | test/backup-controllers--e2e-minio | .claude/worktrees/backup-controllers--e2e-minio | → feat/backup-controllers | not-started |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| bucket-contract | merged producer PR | #65's PR | pending |
| objectstore-api | merged producer PR | #65's PR | pending |
| esadmin-api | merged producer PR | #65's PR | pending |
| camundaadmin-api | merged producer PR | #65's PR | pending |
| logicalbackup-skeleton | merged producer PR (complete in PR1, no stub) | #65's PR | pending |
| snapshot-repository-field | merged producer PR | #66's PR | pending |
| management-binding | merged producer PR | #67's PR | pending |
| backup-kind-types | PR6 branches after #68+#69 merge | n/a | pending |

## Bubble-up log

- _No concerns yet._

## Pending snapshot

1. Phase 1 in flight: #65 implementer dispatched in worktree `backup-controllers--foundation`; on ready — copilot-review-loop to clean (no cap), two-stage review, self-merge, `gh issue close 65`, lock the five PR1 contracts.
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
