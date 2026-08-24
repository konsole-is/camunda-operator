---
feature: cnpg-database-server
spec: docs/superpowers/specs/2026-08-24-cnpg-database-server-design.md
plan: docs/superpowers/plans/2026-08-24-cnpg-database-server-plan.md
tracking_issue: #127
feature_branch: feat/cnpg-database-server
feature_worktree: .claude/worktrees/cnpg-database-server
sub_pr_approval: autonomous
sub_pr_review_loop: on
sub_pr_target: feature-branch
integration_pr:
status: consumer-wave
---

# DatabaseServer on CloudNativePG — orchestration state

The user's standing instruction for this feature (2026-08-24): full autonomy through every sub-PR and its review loop; the integration PR to `main` is the user's to open and merge.

## Phases

- **Phase 1 (foundational, parallel)** — `#128` (namespaced chain + system identifier), `#234` (wrappers)
- **Phase 2** — `#235` (DatabaseServer kind)
- **Phase 3** — `#236` (recovery through the contract)
- **Phase 4** — `#237` (e2e + installation docs)
- **Phase 5** — `#243` (split the Camunda e2e job by storage backend; filed 2026-08-24 at the user's request, runs after #237)

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #128 | fix/cnpg-database-server--namespaced-chain | .claude/worktrees/cnpg-database-server/.claude/worktrees/namespaced-chain | #239 → feat/cnpg-database-server | self-merged (2d812a8) |
| #234 | chore/cnpg-database-server--cnpg-wrappers | .claude/worktrees/cnpg-database-server/.claude/worktrees/cnpg-wrappers | #238 → feat/cnpg-database-server | self-merged (beee38d) |
| #235 | feat/cnpg-database-server--database-server | .claude/worktrees/cnpg-database-server/.claude/worktrees/database-server | #241 → feat/cnpg-database-server | self-merged (3b6aef2) |
| #236 | feat/cnpg-database-server--contract-recovery | .claude/worktrees/cnpg-database-server/.claude/worktrees/contract-recovery | #242 → feat/cnpg-database-server | self-merged (6194813) |
| #237 | ci/cnpg-database-server--e2e | (removed) | #244 → feat/cnpg-database-server | self-merged (c6366c5) |
| #243 | ci/cnpg-database-server--split-e2e-by-backend | (removed) | #246 → feat/cnpg-database-server | self-merged (bccf012) |
| checkpoint | refactor/cnpg-database-server--converge | .claude/worktrees/cnpg-database-server/.claude/worktrees/converge | → feat/cnpg-database-server | in-progress (dispatched 2026-08-24 ~23:05Z; PR to open when pushed) |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `namespaced-rdbms-kinds` | stub-on-producer-branch (PR 3 branches after PR 1 merges); two row details superseded later: `pinnedChainCurrent` blanks endpoint and identifier across a recovery (#242), and the probed identity clears only when host, port, Secret name, or Secret keys move (#242 rounds 1-2) | #239 (2d812a8) | locked |
| `cnpg-wrappers` | stub-on-producer-branch (PR 3 branches after PR 2 merges); `failingPhases` later lost the two plugin phases (#244), and testenv also has `WithoutBarmanPlugin` | #238 (beee38d) | locked |
| `contract-recovery-fields` | data-only (both writers in PR 4); shipped wider than the row: `requestID` on request and outcome, `AnsweredBy` instead of `Matches` (bubble-up, #242 round 1) | #242 (6194813) | locked |

## Bubble-up log

- 2026-08-24 — Pre-integration checkpoint (`reviewing-feature-progress`) on bccf012: all nine gates green on the feature worktree; #246's run measured the split (elasticsearch 29m21s / postgres 28m38s / management 19m46s, install skips confirmed in both logs). The coherence sweep found no behavior drift; it found the spec describing the pre-#242 recovery shape in four places, condition types in `pkg/components` against the `api/v1` rule, three event-reason names off the API vocabulary, and no `suspended` golden. All go into one convergence sub-PR (`refactor/cnpg-database-server--converge`). Deliberate and recorded in code: `database`/`databaseserverconfig` read their owner cached (no terminal step keyed on the last status write), `RFC3339Nano` for the target time. Deliberate and recorded in the spec: suspension, deletion, archive re-enable, and the shared-server refusal are envtest-only (each e2e flow costs a CloudNativePG bootstrap per PR). The GitHub-surface sweep found 21 stale claims across #127 and the six sub-issue bodies; they are being reconciled through `writing-github-issues` (decision comments for material changes).
- 2026-08-24 — #244 e2e on bef2d91 failed the same spec with a marker checkpoint between the point and `lost`: Zeebe came back at the first backed-up checkpoint past the point, not the marker (the recovered database was right: `observedPositions.lastUpdated` before the point). Empirical rule on 8.9: the restore point is the first primary-storage backup at or after the point. Fixed as 2536c45 (the flow waits for that backup before it starts `lost`, no marker wait) and 4d0f877 (docs say the schedule and the checkpoint interval together set the granularity). Copilot rounds 4-5 clean (one suppressed GoDoc nit fixed as 4885148).
- 2026-08-24 — The user asked (21:45Z) to run only the relevant part of the suite locally and move on, because CI is too slow. The `databaseserver` flow ran green on a local kind cluster (`ECK_INSTALL_SKIP`, `KEYCLOAK_OPERATOR_INSTALL_SKIP`, label filter `databaseserver`, 13/13 in 13 min) on 4d0f877. #244 was squash-merged as c6366c5 on Lint/Tests/Chart green plus that local run, without waiting for the e2e job on 4d0f877. This is a one-flow exception to the standing "e2e runs in CI" instruction, made by the user.
- 2026-08-24 — #244 e2e on 1fbf09b passed the recovery cutover and failed the next spec: the instance started 11 s after the point came back. Not a defect: Camunda restores Zeebe to the first checkpoint at or after the database position and re-exports up to it (rdbms-restore docs), so `checkpointInterval` is the granularity, as the API GoDoc already said. Fixed as 06ecd90 (the flow starts `lost` only after every partition wrote a checkpoint past the point) and bef2d91 (`docs/crds/pointintimerestore.md` states the granularity; the external-path specs never over-claimed). The user asked (20:45Z) to do #243 right after #244 merges because CI is too slow; #243 was implemented in parallel, commit-only, branched from 1fbf09b.
- 2026-08-24 — #244 e2e on a656a2c refused the recovery two seconds after the candidate was applied: CloudNativePG reported `PhaseFailurePlugin`, which `pkg/wrappers/cnpgcluster` `failingPhases` listed as terminal. CloudNativePG (release-1.30 `internal/controller/cluster_controller.go`) registers `PhaseFailurePlugin` and `PhaseUnknownPlugin` on any plugin error and requeues in 10-15 s, and its `plugins.go` describes the phase oscillating with Healthy (#8582). Fixed as 1fbf09b: both phases grade as converging; an envtest reproduces the refusal with the old set. This amends the #234 wrapper decision. A plugin absent for good is still caught by the `BarmanPluginNotInstalled` pre-check. Propagated: nothing else grades a CloudNativePG phase.
- 2026-08-24 — #244 quality pass (opus) folded in as f0775a3..895def2: the answered-record reads in the e2e sit in an `Eventually` because the server flushes its status after the contract answer the restore keys on; the archive gate is one exported predicate `components.Archiving(merged)` at both sites (replaces the `archiving bool` parameter of batch 1); monitoring stays out of `Ready` by design and the API GoDoc says why; the coverage poll asserts every partition from `NewEffective(spec).Partitions()`; `recoveryMatches` governs both writers of `status.recovery`; `ParseCheckpointTime` lives in test/utils with a table test for both timestamp layouts. Skipped: the 1.26 floor in docs is set by the spec, the suite tests the pin, same as ECK.
- 2026-08-24 — #244 e2e on 6a4bcf3 reached the recovery cutover and failed one spec: `status.recovery.previousCluster` was empty after `result: Completed`. Root cause (not re-run): `answerRecovery` rebuilt the record and dropped `previousCluster` and `archive` while `recordPublishedOutcome` kept them. Fixed as 9ab8115 (`answeredRecovery` is the one builder for both writers, envtest asserts both fields after a completed recovery and fails without the fix). The specs after the cutover (unsuspend, instance visibility, `-r1` archive Healthy) are still unproven on CI.
- 2026-08-24 — #244 review round 1 (Copilot 2 posted, spec pass 3 should-fix + 9 nits, no blockers), all folded in as fa5b07c..a656a2c: the fixed 3-minute sleep before the restore became a poll of `/actuator/backupRuntime/state` until every partition holds a backup past the point (worst case with PT2M/PT1M is 3 min + one backup, so the sleep had no margin); the archive gate is computed once (`archiving`) for the component and for `Ready`; the spec sentence "`Ready` on the owner aggregates them" was amended for 125e54f; `docs/installation.md` floor aligned to the spec (1.26, 1.27 with the plugin; #241's 1.27 had no recorded reason). Skipped on purpose: renovate coverage of version strings in docs prose.
- 2026-08-24 — The `copilot-auto-review` ruleset fires only on PRs whose base is `main` (`ref_name include ["~DEFAULT_BRANCH"]`). Sub-PRs into the feature branch get no review until requested; request once per round with a watermark, and filter the wait on the bot login (the user's own thread replies register as COMMENTED reviews).
- 2026-08-24 — #242 Copilot loop stalled after round 2 (13:23Z): re-requests at 14:00Z and 14:31Z produced no review across two 30-minute waits although Copilot reviewed other heads in the repo meanwhile. Heads 341e434 and 0f98d92 are covered by the second and third opus quality passes (mutation-checked fixes) and CI. Loop stopped per the skill; the user decides whether to merge on CI green or re-request from the UI.
- 2026-08-24 — #242 rounds 1–2 added: `DatabaseServerConfig.status.probedEndpoint`/`probedSecretName` (the probed identity clears only when host, port, or the admin Secret name moves, not on recovery-field writes); `DatabaseServer.status.recovery` carries `requestID`, `contract`, `result`, `message`, `previousCluster`, and the selected archive `{serverName, objectStorageRef}`; a candidate that fails after promotion rolls the pointer back to `previousCluster`; every delete carries a UID precondition; `-rN` names are bounded so `<name>-rN-rw` stays a DNS label. Propagated: #237's e2e must expect two probe cycles and write `requestID`.
- 2026-08-24 — #242 round 1 changed the contract shape: `RecoveryRequest` gains `requestID` (the restore's UID, required, UUID pattern) echoed in `RecoveryOutcome`; `AnsweredBy` compares requestedBy, targetTime, requestID. A retry is a new restore resource. The superseded archive record closes at the cutover; points between the cutover and the new archive's first base backup are Unavailable. Propagated: spec amended (f2a85b4); #237's dispatch must write `requestID` in any hand-written request and budget two contract probe cycles before a restore leaves RestoringDatabase.
- 2026-08-24 — #241 surfaced an ocf bug: a re-enabled component inherits the Disabled condition's True and reports True/Blocked forever. Filed upstream as sourcehawk/operator-component-framework#194; the consumer workaround is `clearReenabledArchiveCondition` in the databaseserver controller. Propagated: #236's dispatch must apply the same treatment to any component it gates off and on.
- 2026-08-24 — #241's Tests check failed on the downgrade-guard spec (from #184, not this epic's code). Root-caused, not re-run: the CamundaCluster reconciler read its owner cached but wrote status uncached, so its own write could 409 and the never-persisted Ready=VersionDowngradeRefused was dropped, duplicating refusal events. Fixed on the feature branch as 3d48879 (APIReader read, house style; 2/66 reproductions before, 0/120 after). #241's next CI run picks it up through the base.
- 2026-08-24 — #235 shipped four spec deviations, all accepted and folded into the spec: `spec.persistentVolumeClaimRetentionPolicy` dropped (CloudNativePG has no retention knob; suspension keeps the volumes); `spec.platformConfigRef` added so the image resolves through `images.Resolve`; `spec.serviceAccount` is `{annotations}` only (CloudNativePG names the ServiceAccount); `ContractReady` means "contract published and superuser Secret present", not the contract's own Ready. `status.recovery` moves to #236 with its writer. Propagated: #236 dispatch must add `DatabaseServerRecoveryStatus`, delete the old `ScheduledBackup` with the old `Cluster` (`BaseBackupName == ClusterName`), keep `barmanObjectName` fixed and change only `serverName`, and answer a request during suspension before it needs the bucket (preCheck returns a nil `ArchiveStorage` while suspended).
- 2026-08-24 — #128 round 2: the dedicated-server rule now holds a restore for a `Database` it cannot place. The strict form (hold whenever any contract in the cluster is unprobed) lets one tenant stall every restore, so `databaseIdentity` falls back to the identity recorded in `status.collisionKey` when the contract is unprobed; only a `Database` that never resolved is unplaceable. Accepted; the retention rule (collisionKey never cleared) is what makes the fallback sound. The strict rule also exposed a `Database` leak in the PITR test fixture (41 failures until `DeferCleanup` was added). Propagated: #236 dispatch prompt must keep this fallback when it adds the operator-recovery exception to the pin check.
- 2026-08-24 — #234 round 4: a Cluster is Suspended only when the `cnpg.io/hibernation` condition is True (no readyInstances fallback). #235 must not wait on Suspended for anything time-critical; DeleteOnSuspend is false so nothing destructive is gated on it.
- 2026-08-24 — #234 spec review: the CNPG CRD declares `minimum: 1` on `spec.instances`, so the scale-to-zero suspend in the spec can never apply. Resolution: suspend through declarative hibernation (`cnpg.io/hibernation: "on"` annotation, status from the `cnpg.io/hibernation` condition). Propagated: spec and plan amended; fix routed to the #234 implementer; the #235 dispatch will carry it.

## Pending snapshot

Session burst ended 2026-08-24 ~22:10Z with #244 merged and #246 (issue #243) open.

1. **#243 / PR #246** (`ci/cnpg-database-server--split-e2e-by-backend`, sub-worktree `.claude/worktrees/split-e2e-by-backend`, head `d7554ba`, two commits replayed onto c6366c5, pushed). Waiting on: (a) Copilot round 1, requested ~22:08Z (watermark in the session scratchpad; sub-PRs into the feature branch get no auto review, request once per round); (b) the `E2E Tests` run with three jobs `Camunda 8.9 elasticsearch|postgres|management` — on green, confirm the logs show `Skipping CloudNativePG installation` on the elasticsearch job and `Skipping ECK installation` on the postgres job, and note the longest job's wall-clock against the ~50 min single job; on red, root-cause, never re-run. Then: Copilot loop to clean, list the reviews once more before merging, self-merge (squash), `gh issue close 243`, remove the sub-worktree, update this table.
2. `feature-dev-workflow:reviewing-feature-progress` on the feature worktree after #246 merges (all gates via the scratch `gates.sh` pattern: setup-envtest, make test, api test, lint, lint-renovate, manifests + clean porcelain, vet e2e, mkdocs), then report ready for the user's integration PR `feat/cnpg-database-server → main` with `Closes #127`. **Do not open or merge the integration PR; it is the user's.** The plan and this state file are deleted in the teardown commit; the spec stays.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature` (parallel waves still in flight; the orchestrator watch loop continues), or work the `## Pending snapshot` when development is past.
