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
status: consumer-wave
---

# Restore controllers — orchestration state

## Phases

- **Phase 1 (foundational)** — `#110` (API types + shared restore package)
- **Phase 2 (consumers, parallel)** — `#111` (LogicalRestore), `#112` (PointInTimeRestore)
- **Phase 3 (proof)** — `#113` (e2e)

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #110 | feat/restore-controllers--api-machinery | .claude/worktrees/restore-controllers--api | #120 → feat/restore-controllers | self-merged |
| #111 | feat/restore-controllers--logicalrestore | .claude/worktrees/restore-controllers--logical | #123 → feat/restore-controllers | ready |
| #112 | feat/restore-controllers--pointintimerestore | .claude/worktrees/restore-controllers--pitr | #122 → feat/restore-controllers | ready |
| #113 | test/restore-controllers--e2e | .claude/worktrees/restore-controllers--e2e | → feat/restore-controllers | not-started |

## Contracts (from the plan)

The plan's `## Contracts` table names seven contracts. All but one realize as "merged code (PR1 lands first)"; `esadmin-restore-api` merges in PR2 and PR4 consumes it only through the controller. This table tracks realization status only — the shapes live in the plan.

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `restore-api-types` | merged code (PR1 first) | #120 | locked |
| `restore-shared-package` | merged code (PR1 first) | #120 | locked |
| `pod-stuck-helper` | merged code (PR1 first) | #120 | locked |
| `backup-version-field` | merged code (PR1 first) | #120 | locked |
| `backup-artifact-naming` | merged code (PR1 first) | #120 | locked |
| `pg-open-seam` | merged code (PR1 first) | #120 | locked |
| `esadmin-restore-api` | merged code (PR2) | n/a | pending |

## Bubble-up log

- 2026-08-20 (from the #111 worker): PR #123 open. Deviations in its PR body: status.terminalReason added to LogicalRestoreStatus (re-staging after write conflicts; mirrors LogicalBackupElasticsearchStatus); DeleteIndices resolves patterns to names (action.destructive_requires_name defaults true since ES 8.0); the ES phase gates on restored indices existing, not the first RestoreDone; mid-run holds requeue at PollInterval; the download subcommand reuses the UPLOAD_STORAGE_* contract; the pg_restore Job uses component pg-restore so JobSelector stays broker-Jobs-only. Shared packages touched: pkg/objectstore gained Bucket.Download, pkg/components/elasticsearchcluster gained RepositoryConfigAt (allowed: additive, outside pkg/restore). Coherence action: asked the #112 worker to mirror status.terminalReason on the PITR status (its own deviation 2 named the gap; #123 shipped the pattern). Known merge conflict: api/v1/zz_generated.deepcopy.go between #122 and #123 — regenerate at whichever merges second. Follow-up documented, not filed: same-name ElasticsearchCluster in two namespaces sharing one ES can have a restore repoint the target's repository until its controller reconverges. For #113: e2e must not delete indices by wildcard itself (destructive_requires_name), and the create-to-recovery window is only provable in the round trip.
- 2026-08-20 (from the #112 worker): PR #122 open. Deviations recorded in its PR body: exporter_position SQL identifiers unquoted lowercase (verified vs camunda source Liquibase changesets); PitrUnavailable and SharedServer HOLD in Pending instead of failing (no TerminalReason field on the PITR status; user-fixable; nothing destructive ran) — reconcile issue #112 wording at the wave checkpoint if needed; extra admission hold InvalidReference on empty backupStorageRef; DatabaseNotRestored steady-state is Pending (admission falls through in one reconcile). Cross-PR: field-index names are manager-global (prefix them); never clear firstFailedAt on resolve; envtest needs a PVC-finalizer-clearing helper; RecreateClaims callers must diff Progress.Recreated vs recorded names (a Progress.Grew flag is a possible pkg/restore follow-up, not filed). For #113: prefer waiting for RestoringPrimaryStorage/Completed over "left ValidatingDatabaseState"; refusal spec asserts Pending + DatabaseNotRestored with Consistently.
- 2026-08-20 (from the #110 worker, for #111/#112/#113 dispatch): (1) RBAC markers for logicalrestores and pointintimerestores (+/status, +/finalizers) were deleted with the scaffold controllers — PR2 and PR3 must each carry their own +kubebuilder:rbac markers and re-register their kind in cmd/main.go. (2) pgbootstrap.Connection renamed AdminUser/AdminPassword to User/Password. (3) RecreateClaims is a two-call contract: flush status.recreatedClaims from Progress.Recreated between calls and stop calling once Progress.Done. (4) The restore Job container is named restore (restore.ComponentRestore), not camunda — pod-status checks and e2e log scraping select on labels.Managed(owner, ComponentRestore) plus the owner key. (5) status.version exists only on backups taken after PR1 merges; a versionless backup fails compatibility with IncompatibleTarget (resolved question). (6) Deviations recorded in the spec by the worker: topology spread constraints are retargeted at the restore pods, SPRING_PROFILES_ACTIVE=restore is set on the Job, Optimize indices are optimize-* not camunda-optimize*, BuildJob copies the whole broker PodSpec, Target gained ClusterName, ContainerCamunda is exported.

- 2026-08-20 (orchestrator): the plan agent raised four open questions; resolved in the plan's `## Resolved questions` section, each in the plan's own safe direction (same-bucket cross-cluster scope, fail-closed on a versionless backup, short PITR e2e fallback, DROP SCHEMA wipe with manual re-grant). The user was AFK with full autonomy granted.

## Pending snapshot

- Phase 1 done: PR #120 self-merged (squash eb0b970) after a 3-round Copilot loop and a two-stage orchestrator review; #110 closed; all six PR1 contracts locked. Phase 2 next: fan out #111 and #112 in parallel off feat/restore-controllers. Their review loops run to clean, then the PRs STAY OPEN for the user. The contract deltas the workers must receive are in the bubble-up log (JobName scheme <restore>-lr|pitr-<ordinal>, JobLabels/JobSelector helpers, SetControllerReference before restore.Apply, flush Progress.Recreated before Jobs, Owner.GetName must equal OwnerLabel.Name, hard-fail on malformed-Target errors, digest-only images fail ReadTarget).
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
