---
feature: restore-controllers
spec: docs/superpowers/specs/2026-08-20-restore-controllers-design.md
plan: docs/superpowers/plans/2026-08-20-restore-controllers-plan.md
tracking_issue: #109
feature_branch: feat/restore-controllers
feature_worktree: .claude/worktrees/restore-controllers
sub_pr_approval: autonomous for #110 and #113; manual for #111 and #112 (user reviews the controller PRs; keep them open once their review loops are clean)
sub_pr_review_loop: on
sub_pr_target: feature-branch
integration_pr:
status: foundational-wave
---

# Restore controllers — orchestration state

## Phases

- **Phase 1 (foundational)** — `#110` (API types + shared restore package)
- **Phase 2 (consumers, parallel)** — `#111` (LogicalRestore), `#112` (PointInTimeRestore)
- **Phase 3 (proof)** — `#113` (e2e)

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #110 | feat/restore-controllers--api-machinery | .claude/worktrees/restore-controllers--api | → feat/restore-controllers | in-progress |
| #111 | feat/restore-controllers--logicalrestore | .claude/worktrees/restore-controllers--logical | → feat/restore-controllers | not-started |
| #112 | feat/restore-controllers--pointintimerestore | .claude/worktrees/restore-controllers--pitr | → feat/restore-controllers | not-started |
| #113 | test/restore-controllers--e2e | .claude/worktrees/restore-controllers--e2e | → feat/restore-controllers | not-started |

## Contracts (from the plan)

The plan's `## Contracts` table names seven contracts. All but one realize as "merged code (PR1 lands first)"; `esadmin-restore-api` merges in PR2 and PR4 consumes it only through the controller. This table tracks realization status only — the shapes live in the plan.

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `restore-api-types` | merged code (PR1 first) | n/a | pending |
| `restore-shared-package` | merged code (PR1 first) | n/a | pending |
| `pod-stuck-helper` | merged code (PR1 first) | n/a | pending |
| `backup-version-field` | merged code (PR1 first) | n/a | pending |
| `backup-artifact-naming` | merged code (PR1 first) | n/a | pending |
| `pg-open-seam` | merged code (PR1 first) | n/a | pending |
| `esadmin-restore-api` | merged code (PR2) | n/a | pending |

## Bubble-up log

- 2026-08-20 (from the #110 worker, for #111/#112/#113 dispatch): (1) RBAC markers for logicalrestores and pointintimerestores (+/status, +/finalizers) were deleted with the scaffold controllers — PR2 and PR3 must each carry their own +kubebuilder:rbac markers and re-register their kind in cmd/main.go. (2) pgbootstrap.Connection renamed AdminUser/AdminPassword to User/Password. (3) RecreateClaims is a two-call contract: flush status.recreatedClaims from Progress.Recreated between calls and stop calling once Progress.Done. (4) The restore Job container is named restore (restore.ComponentRestore), not camunda — pod-status checks and e2e log scraping select on labels.Managed(owner, ComponentRestore) plus the owner key. (5) status.version exists only on backups taken after PR1 merges; a versionless backup fails compatibility with IncompatibleTarget (resolved question). (6) Deviations recorded in the spec by the worker: topology spread constraints are retargeted at the restore pods, SPRING_PROFILES_ACTIVE=restore is set on the Job, Optimize indices are optimize-* not camunda-optimize*, BuildJob copies the whole broker PodSpec, Target gained ClusterName, ContainerCamunda is exported.

- 2026-08-20 (orchestrator): the plan agent raised four open questions; resolved in the plan's `## Resolved questions` section, each in the plan's own safe direction (same-bucket cross-cluster scope, fail-closed on a versionless backup, short PITR e2e fallback, DROP SCHEMA wipe with manual re-grant). The user was AFK with full autonomy granted.

## Pending snapshot

- Phase 1 in flight: an Opus worker implements #110 in .claude/worktrees/restore-controllers--api. On its ready report: copilot review loop, two-stage orchestrator review, self-merge, close #110, lock the PR1 contracts, checkpoint, then fan out #111 and #112 in parallel.
- User instructions (2026-08-20, AFK with full autonomy): run the Copilot review loop on every sub-PR. Self-merge #110 when clean. Keep #111 and #112 OPEN once their loops are clean — the user reviews the controller PRs personally. Hold #113 until #111 and #112 merge.
- Token note: the user runs close to the Fable limit. Dispatch implementation subagents on `model: opus`.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature` (parallel waves still in flight; the orchestrator watch loop continues), or work the `## Pending snapshot` when development is past.
