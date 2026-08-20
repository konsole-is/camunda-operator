---
feature: restore-controllers
spec: docs/superpowers/specs/2026-08-20-restore-controllers-design.md
plan: docs/superpowers/plans/2026-08-20-restore-controllers-plan.md
tracking_issue: #109
feature_branch: feat/restore-controllers
feature_worktree: .claude/worktrees/restore-controllers
sub_pr_approval: autonomous for the e2e PR; manual for every controller PR (the user reviews and merges PR-A, PR-B, and PR-C personally; keep them open once their review loops are clean)
sub_pr_review_loop: on
sub_pr_target: feature-branch
integration_pr:
status: split-rebreak
---

# Restore controllers — orchestration state

## The split (2026-08-20)

`LogicalRestore` became `LogicalRestoreElasticsearch` and `LogicalRestoreRDBMS`. The spec records
the three decisions that drive the rest of this file: the split, the shared machinery moving into
`pkg/restore`, and the per-cluster claim that every restore kind now takes. PR #123 is closed, and
its branch is the raw material for PR-B and PR-C.

## Phases

- **Phase 1 (foundational)** — `#110` (API types + shared restore package). Done.
- **Phase 2 (PointInTimeRestore)** — `#112`. Done.
- **Phase 3 (unify)** — PR-A. One driver in `pkg/restore` for all three restore kinds.
- **Phase 4 (logical kinds, parallel)** — PR-B (Elasticsearch), PR-C (RDBMS).
- **Phase 5 (proof)** — `#113` (e2e, both new kinds), then the integration PR.

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #110 | feat/restore-controllers--api-machinery | .claude/worktrees/restore-controllers--api | #120 → feat/restore-controllers | MERGED |
| #112 | feat/restore-controllers--pointintimerestore | .claude/worktrees/restore-controllers--pitr | #122 → feat/restore-controllers | MERGED (075746a) |
| #111 | feat/restore-controllers--logicalrestore | .claude/worktrees/restore-controllers--logical | #123 | CLOSED — raw material for PR-B and PR-C, head 14a1545 |
| #129 (PR-A) | feat/restore-controllers--unify | .claude/worktrees/restore-controllers--unify | #134 → feat/restore-controllers | MERGED (1316df0) |
| #111 (PR-B) | feat/restore-controllers--lres | .claude/worktrees/restore-controllers--lres | → feat/restore-controllers | in-progress |
| #130 (PR-C) | feat/restore-controllers--lrrdbms | .claude/worktrees/restore-controllers--lrrdbms | → feat/restore-controllers | in-progress |
| #113 | test/restore-controllers--e2e | .claude/worktrees/restore-controllers--e2e | → feat/restore-controllers | not-started; must cover BOTH new kinds |

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

- 2026-08-20 (orchestrator, CI): `go test ./...` NEVER compiles `test/e2e` — that package is behind the `e2e` build tag, so a moved or renamed symbol breaks CI while every local gate stays green. PR #120 moved `RecordsSnapshotName` into `pkg/logicalbackup` and left `test/e2e/backup_test.go` calling it through the controller package; CI failed on both #122 and #123, which inherited it from the base. Fixed on the feature branch (a409a42) and merged forward into both PR branches. ADD `go vet -tags=e2e ./test/e2e/` to the verification set of every future worker, next to `make lint`.
- 2026-08-20 (orchestrator): `make all` does NOT lint — Makefile has `all: build` while CLAUDE.md claims "make all # lint and format". Workers now run `make lint` explicitly. FLAG TO THE USER: either point `all` at lint or fix the CLAUDE.md comment. Also queued: run `make lint` on the merged feature branch at the wave checkpoint (PR #120 merged with lint claims based on the misleading target).
- 2026-08-20 (PR #122 hardening round): the DB-state precheck assumed the broker JVM writes LAST_UPDATED in UTC (RdbmsExporter uses LocalDateTime.now(); column is timestamp without time zone). Operator-rendered brokers default to UTC, but spec.zeebe.extraEnv can set TZ or -Duser.timezone — admission now holds with PitrUnavailable when it does. Plus: postgres testcontainer coverage for the readPositions SQL, a 30s read deadline, literal --to assertion, Jobs iterate the recorded status.primaryJobNames (live-count drift), dedicated-server gate reads live via APIReader (duplicate field index removed), decide reports missing rows before ahead rows, slack boundary pinned at +60s, managedFields assertion pins the field manager. ValidatingDatabaseState is documented as visible only while the DB is unreachable (persisting it would flap against #113's Consistently spec).
- 2026-08-20 (from the #111 worker): PR #123 open. Deviations in its PR body: status.terminalReason added to LogicalRestoreStatus (re-staging after write conflicts; mirrors LogicalBackupElasticsearchStatus); DeleteIndices resolves patterns to names (action.destructive_requires_name defaults true since ES 8.0); the ES phase gates on restored indices existing, not the first RestoreDone; mid-run holds requeue at PollInterval; the download subcommand reuses the UPLOAD_STORAGE_* contract; the pg_restore Job uses component pg-restore so JobSelector stays broker-Jobs-only. Shared packages touched: pkg/objectstore gained Bucket.Download, pkg/components/elasticsearchcluster gained RepositoryConfigAt (allowed: additive, outside pkg/restore). Coherence action: asked the #112 worker to mirror status.terminalReason on the PITR status (its own deviation 2 named the gap; #123 shipped the pattern). Known merge conflict: api/v1/zz_generated.deepcopy.go between #122 and #123 — regenerate at whichever merges second. Follow-up documented, not filed: same-name ElasticsearchCluster in two namespaces sharing one ES can have a restore repoint the target's repository until its controller reconverges. For #113: e2e must not delete indices by wildcard itself (destructive_requires_name), and the create-to-recovery window is only provable in the round trip.
- 2026-08-20 (from the #112 worker): PR #122 open. Deviations recorded in its PR body: exporter_position SQL identifiers unquoted lowercase (verified vs camunda source Liquibase changesets); PitrUnavailable and SharedServer HOLD in Pending instead of failing (no TerminalReason field on the PITR status; user-fixable; nothing destructive ran) — reconcile issue #112 wording at the wave checkpoint if needed; extra admission hold InvalidReference on empty backupStorageRef; DatabaseNotRestored steady-state is Pending (admission falls through in one reconcile). Cross-PR: field-index names are manager-global (prefix them); never clear firstFailedAt on resolve; envtest needs a PVC-finalizer-clearing helper; RecreateClaims callers must diff Progress.Recreated vs recorded names (a Progress.Grew flag is a possible pkg/restore follow-up, not filed). For #113: prefer waiting for RestoringPrimaryStorage/Completed over "left ValidatingDatabaseState"; refusal spec asserts Pending + DatabaseNotRestored with Consistently.
- 2026-08-20 (from the #110 worker, for #111/#112/#113 dispatch): (1) RBAC markers for logicalrestores and pointintimerestores (+/status, +/finalizers) were deleted with the scaffold controllers — PR2 and PR3 must each carry their own +kubebuilder:rbac markers and re-register their kind in cmd/main.go. (2) pgbootstrap.Connection renamed AdminUser/AdminPassword to User/Password. (3) RecreateClaims is a two-call contract: flush status.recreatedClaims from Progress.Recreated between calls and stop calling once Progress.Done. (4) The restore Job container is named restore (restore.ComponentRestore), not camunda — pod-status checks and e2e log scraping select on labels.Managed(owner, ComponentRestore) plus the owner key. (5) status.version exists only on backups taken after PR1 merges; a versionless backup fails compatibility with IncompatibleTarget (resolved question). (6) Deviations recorded in the spec by the worker: topology spread constraints are retargeted at the restore pods, SPRING_PROFILES_ACTIVE=restore is set on the Job, Optimize indices are optimize-* not camunda-optimize*, BuildJob copies the whole broker PodSpec, Target gained ClusterName, ContainerCamunda is exported.

- 2026-08-20 (orchestrator): the plan agent raised four open questions; resolved in the plan's `## Resolved questions` section, each in the plan's own safe direction (same-bucket cross-cluster scope, fail-closed on a versionless backup, short PITR e2e fallback, DROP SCHEMA wipe with manual re-grant). The user was AFK with full autonomy granted.

## Pending snapshot

- The split is DECIDED and recorded. Spec amended at d53f70a, plan rebroken at a325561. Both are on the feature branch. Read them before dispatching any worker.
- USER CONSTRAINT, still binding: the Zeebe primary-storage restore (PVC recreation plus the per-broker restore-application Jobs) is SHARED by both storage paths and by `PointInTimeRestore`. It lives in `pkg/restore`. The split covers the API kinds and the secondary-storage phase only. Never duplicate the primary phase.
- Issues are filed and parented to #109: `#129` (PR-A, unify), `#111` (PR-B, `LogicalRestoreElasticsearch`, reconciled through Step 2D), `#130` (PR-C, `LogicalRestoreRDBMS`).
- PR-A is MERGED (#134, 1316df0). `pkg/restore` is now the shared driver of every restore kind, `PointInTimeRestore` runs on it, the cluster claim is wired, and the `LogicalRestore` kind is gone. One balanced Copilot round found a real defect, fixed in f814756: the claim and the broker count were trusted before they were durable.
- CORRECTION to an earlier note in this file: PR-B and PR-C do NOT touch disjoint files. Their controllers, types, and secondary phases are disjoint, but both edit `PROJECT`, `config/{crd,rbac,samples}/kustomization.yaml`, `cmd/main.go`, `mkdocs.yml`, `pkg/labels/labels.go`, `pkg/restore/{apply,job}.go`, `pkg/clusterclaim/claim.go`, `api/v1/restore_shared.go`, and the generated `zz_generated.deepcopy.go` and `role.yaml`. They still run in parallel. Whichever merges SECOND rebases onto the first and re-runs `make manifests generate`. This is the same collision #122 and #123 hit.
- NEXT ACTION: PR-B and PR-C are in flight. When each opens, run a Copilot review loop at the BALANCED level. Then the user merges both. Then #113 (e2e, both new kinds), then the integration PR `feat/restore-controllers` -> main with `Closes #109`, stopping at ready-to-merge for the user.
- The user reviews and merges every controller PR personally. An agent never merges them.
- CLEANUP, not done: the worktrees `restore-controllers--api`, `restore-controllers--pitr`, and `restore-controllers--unify` are spent and their branches are merged. Removing them also clears a gopls artifact that type-checks stale worktrees against the current `api` module and reports phantom compile errors. KEEP `restore-controllers--logical` at 14a1545 until PR-B and PR-C are merged — it is the source of both secondary-storage phases. Read the worktree-removal trap first: envtest binaries under `bin` are read-only, so `chmod -R u+w bin` before `git worktree remove`.
- PR-A RISK: it rewrites the destructive primary-storage phase of `PointInTimeRestore` after that controller merged. Its envtest suite (1325 lines) is the safety net and stays as it is. A test that PR-A changes must name the resolved divergence that forced the change, in the PR body. Never adjust a test quietly to make the refactor pass.
- The Lease is UNBUILT. No restore kind calls `clusterclaim.Claim`/`Release`. PR-A puts it in the shared driver: claim when admission passes, release at the terminal transition, hold in `Pending` with `ClusterClaimed` when another holder has it. Copilot thread #122/3822851527 is still open for this and gets answered in PR-A. Thread #123/3822837179 was answered in the closing comment of #123.
- Base branch `feat/restore-controllers` carries, beyond PR #120: the e2e compile fix (a409a42), the cluster claim moved to `pkg/clusterclaim` with a neutral Lease prefix (e787f7c), a kind-agnostic claim liveness rule (dc01466), an elasticsearchcluster test-flake fix (81ff766), and the merged `PointInTimeRestore` controller (075746a).
- REMOVAL SURFACE for PR-A, verified by grep. Beyond the obvious types file, CRD, RBAC role files, sample, and docs page: `config/{crd,rbac,samples}/kustomization.yaml` entries; the `kind: LogicalRestore` block in `PROJECT`; the `resources=logicalrestores;pointintimerestores` RBAC markers in BOTH backup controllers (`logicalbackupelasticsearch/controller.go:149`, `logicalbackuprdbms/controller.go:182`); `internal/controller/samples_schema_test.go`; `jobKindInfixes[labels.LogicalRestoreKey]` in `pkg/restore/job.go`; all three `pkg/restore/testdata/golden/*.yaml` (they carry `camunda.io/logical-restore` labels and `-lr-` Job names); the hardcoded field manager in `pkg/restore/claims_test.go`; the package comment in `pkg/restore/doc.go`; two links to `logicalrestore.md` in `docs/crds/pointintimerestore.md` (lines 7 and 185 — `mkdocs build --strict` catches these); and user-visible prose in `pointintimerestore/{admit.go:187,primary.go:332}`. `v1.ReasonIncompatibleTarget` must SURVIVE the deletion and move to `restore_shared.go`.
- OPEN QUESTION for the user: `PROJECT` has no supported removal path. AGENTS.md forbids hand-editing it and kubebuilder v4 has no `delete api` verb. PR-A hand-edits it and says so in its body. Decide whether AGENTS.md records the exception.
- FLAG TO THE USER: two verification gaps found the hard way. (1) `make all` does not lint (Makefile `all: build`) while CLAUDE.md claims it does. (2) `go test ./...` never compiles `test/e2e` (build tag), so symbol moves break CI silently — worth adding both `make lint` and `go vet -tags=e2e ./test/e2e/` to a single documented pre-PR gate.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature` (parallel waves still in flight; the orchestrator watch loop continues), or work the `## Pending snapshot` when development is past.
