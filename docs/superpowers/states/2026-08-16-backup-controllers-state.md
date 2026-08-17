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
| #66 | feat/backup-controllers--es-snapshot-repository | .claude/worktrees/backup-controllers--es-snapshot-repository | #79 → feat/backup-controllers | draft |
| #67 | feat/backup-controllers--cluster-backup-wiring | .claude/worktrees/backup-controllers--cluster-backup-wiring | #77 → feat/backup-controllers | draft (verified: 29 pkgs, make all, 129840B gzipped) |
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

- **2026-08-17 — Azure workload identity is half-covered by the shared switch; #67 must add the pod label.** `v1.ObjectStorageConfig.WorkloadIdentityAnnotations()` returns SA annotations only, but Azure WI also needs the pod label `azure.workload.identity/use: "true"` (spec names it; #77 already renders it via `derivedPodLabels`). When #77 refactors onto the shared method after #79 merges, the pod-label half stays CamundaCluster-side — verify it survives the refactor. #79 documents the limitation in the method GoDoc; the ES side rejects non-S3 buckets so it is unreachable there.
- **2026-08-17 — #79 spec review: three blocking items routed back.** (1) Endpoint seam + convergence envtest for `SnapshotRepositoryReady` (was untestable as written); (2) the bucket-credentials Secret watch was missing — on a healthy cluster a rotated Secret never re-rendered the keystore (the 30s requeue is gated on not-Ready); (3) the Ready-override, suspend, and set-then-cleared behaviors were asserted in prose but untested. Non-blocking: stale doc comment, schema tests for new field validations, PR-thread evidence.

- **2026-08-17 — Annotation-derivation divergence between #79 and #77 (watch-loop catch).** Both PRs implemented the per-cloud workload-identity switch independently: #79 as `v1.ObjectStorageConfig.WorkloadIdentityAnnotations()` (single bucket, api/v1), #77 as `DerivedServiceAccountAnnotations(buckets...)` with its own copies of the annotation keys. Resolution: the api/v1 method is the one switch (its GoDoc says consumers never repeat it); the multi-bucket merge and two-identity rejection stay CamundaCluster policy but call the method per bucket. Merge order: #79 first; #77 then merges the feature branch and refactors before its own merge. Also: `KUBEBUILDER_ASSETS` must be an absolute path — a relative one makes every envtest suite fail etcd startup in ~10ms (the earlier "flake" signature).
- **2026-08-17 — Snapshot deletion needs the `manage` cluster privilege (verified, Elastic docs).** `create_snapshot` covers create/list/view only, and the Delete Snapshot API states `manage` outright; no narrower named privilege exists. #79 grants `manage` with the reasoning in the role comment and citations in the PR body. The earlier code comment claiming `create_snapshot` covers deletion was corrected — PR4's finalizer depends on this.

- **2026-08-17 — Envtest control planes leak across worktrees and starved the machine.** Load average reached 851 with zero free memory; #66's agent stalled and #67 could not finish verification. Cause: 14 orphaned `kube-apiserver`/`etcd` processes from three backup worktrees, six of them from `backup-controllers--foundation` which had already merged and been removed hours earlier — `git worktree remove` does not reap them. Resolution: orchestrator reaped all 14 (leaving the sibling OIDC session's kind cluster untouched); implementers are now told to run `go test -p 1`, check `uptime` before a full run, and verify their own worktree leaks nothing afterwards. Standing rule for the rest of the epic: reap `pgrep -af "/.claude/worktrees/<wt>/bin/k8s/"` before removing any worktree.
- **2026-08-17 — Two errors in the Camunda 8.9 property reference, found while wiring #67.** The page spells the continuous-backup key `continous` (missing `u`) in one table, contradicted by the env-var table, the broker-config section, and two chaos reports; and it gives `gcs.endpoint` the text belonging to `base-path` while omitting `gcs.base-path`, which the broker-config page and its YAML snippet both list. The corroborated spellings were used. Anything reading that page later (PR5, PR7) should not "correct" the code back to it.
- **2026-08-17 — Copilot review unavailable on this repo; orchestrator review is the gate.** `gh pr edit --add-reviewer "@copilot"` exits 0 but the reviewer never attaches (verified twice on #74), which is the documented signal for "unavailable for this repo/plan"; consistent with the org's known billing state. Effect: `sub_pr_review_loop: on` cannot run for any PR in this epic. Resolution: the orchestrator's two-stage review (spec-compliance gate, then code quality) is the gate on every sub-PR, run harder to compensate; the user's "no round cap" directive applies to that loop instead. Durable fix if the entitlement is restored: a repo ruleset running Copilot code review on push.
- **2026-08-17 — PR1 contract deviations reviewed and accepted; plan amended.** Status vocabulary (`LogicalBackupPhase`, shared reasons, `ClusterRef`, `LogicalBackupStorageSizes`) lives in `api/v1`, not `pkg/logicalbackup` — it is CRD status surface needing deepcopy and enum markers, per CLAUDE.md and `api/v1/conditions.go`. `PreCheck` takes a request struct with an injected `InProgress` because the backup kinds do not exist until PR4/PR5. Per-kind reasons deferred to their kinds. Propagated: the `logicalbackup-skeleton` contract row now records the shipped shape; PR4 and PR5 sections carry the reason declarations and the "InProgress lists both kinds" requirement.
- **2026-08-17 — `make fmt` corrupts CEL in declaration doc comments.** gofmt normalizes `''` to typographic quotes in doc comments above type declarations, producing a rule Kubernetes cannot compile; tests stayed green because the committed CRD YAML still held the old rule. Found and fixed in #74 (`size() > 0`). Propagated to the plan's Conventions block — PR2 and PR3 both add CEL to types and must re-run `make manifests` and confirm the regenerated YAML holds the rule as written.
- **2026-08-17 — CRD doc ownership settled.** The plan (PR3), the Conventions block (owning PR), and #74's body (PR7) gave three different answers for `docs/crds/objectstorageconfig.md`, and the page contradicted its own shipped CRD. Rule now in Conventions: the PR that changes a CRD's schema rewrites that CRD's page in the same PR; PR7 owns only the backup-page split/removal, the prose sweep, and `mkdocs.yml` nav. PR3's step list updated.
- **2026-08-17 — GCS/Azure credential-shape verification was missed in PR1.** Spec §ObjectStorageConfig requires it before the types are final; #74 verified only the management-port auth. Routed back to the #65 implementer; if it forces a type change it lands in #74 before merge, since PR2/PR3/PR5 all build on those types.

## Pending snapshot

1. Phase 2: #77 spec-clean (lint fixed, verification complete: 56 specs, helm 129840B) — quality pass running. #79 spec review BLOCKED on three items, routed to its author (see bubble-up); on its report re-run the spec pass over the changed surface, then #79's quality pass.
2. Merge order: #79 FIRST (owns `WorkloadIdentityAnnotations()`), close #66, lock `snapshot-repository-field`; then message #77's agent to merge the feature branch, refactor `DerivedServiceAccountAnnotations` onto the shared method (keeping the Azure pod label — see bubble-up), reconcile its stale PR-body verification paragraph, wire `serviceAccount.name`/`create` on the CamundaCluster side (the shared type now has the fields); re-review the delta, merge #77, close #67, lock `management-binding`.
3. Then `reviewing-feature-progress` wave checkpoint; then fan out #68/#69 (Phase 3 — user-reviewed PRs).
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
