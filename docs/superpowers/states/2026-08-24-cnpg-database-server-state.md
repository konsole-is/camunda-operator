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
| #237 | ci/cnpg-database-server--e2e | .claude/worktrees/cnpg-database-server/.claude/worktrees/e2e | #244 → feat/cnpg-database-server | in-progress (head 6a4bcf3, E2E CI running) |
| #243 | ci/cnpg-database-server--split-e2e-by-backend | .claude/worktrees/cnpg-database-server/.claude/worktrees/split-e2e-by-backend | → feat/cnpg-database-server | not-started |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `namespaced-rdbms-kinds` | stub-on-producer-branch (PR 3 branches after PR 1 merges) | #239 (2d812a8) | locked |
| `cnpg-wrappers` | stub-on-producer-branch (PR 3 branches after PR 2 merges) | #238 (beee38d) | locked |
| `contract-recovery-fields` | data-only (both writers in PR 4) | n/a | locked |

## Bubble-up log

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

Session ended 2026-08-24 ~17:45Z with #237 mid-CI. Nothing is waiting on a human except the integration PR at the very end.

1. **#237 / PR #244** (`ci/cnpg-database-server--e2e`, worktree `.claude/worktrees/e2e`, head `6a4bcf3`, six commits, pushed). The implementer's fourth iteration is on CI: `E2E Tests` in progress on 6a4bcf3 (the run on b8542e8 failed its e2e; the fix-forward commits are `125e54f` "keep a part the spec switched off out of Ready" and `6a4bcf3` "keep CloudNativePG off the jobs that run no DatabaseServer"). Next: read the result of that run (`gh pr checks 244`; on failure read the `Camunda 8.9` job log and root-cause, never re-run to green); when green, the normal ripening — Copilot loop (request with a watermark; this repo does not auto-review on push; if Copilot goes silent for two 30-minute waits, stop and ask the user, who chose "merge on CI green" for #242 in that situation), spec pass then quality pass (opus, read-only, mutation-checked fixes), self-merge, `gh issue close 237`, remove the sub-worktree, update this table. Note `125e54f` touches the `databaseserver` controller (Ready aggregation of a switched-off part) — the spec pass must check it against #235's contract.
2. **#243** (`ci/cnpg-database-server--split-e2e-by-backend`): branch after #237 merges; PR 6 in the plan. Labels per job are in the issue body; #244's `6a4bcf3` already keeps CNPG off jobs without a DatabaseServer, so read it before designing the split.
3. `feature-dev-workflow:reviewing-feature-progress` on the feature worktree after #243 merges (all gates via the scratch `gates.sh` pattern: setup-envtest, make test, api test, lint, lint-renovate, manifests + clean porcelain, vet e2e, mkdocs), then report ready for the user's integration PR `feat/cnpg-database-server → main` with `Closes #127`. **Do not open or merge the integration PR; it is the user's.**

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature` (parallel waves still in flight; the orchestrator watch loop continues), or work the `## Pending snapshot` when development is past.
