---
feature: optimize-controller
spec: docs/superpowers/specs/2026-08-20-optimize-controller-design.md
plan: docs/superpowers/plans/2026-08-20-optimize-controller-plan.md
tracking_issue: #114
feature_branch: feat/optimize-controller
feature_worktree: .claude/worktrees/optimize-controller
sub_pr_approval: autonomous
sub_pr_review_loop: on
sub_pr_target: feature-branch
integration_pr:
status: foundational-wave
---

# CamundaOptimize controller — orchestration state

The user granted full autonomy on 2026-08-20: run review loops on the sub-PRs, self-merge them into the feature branch, and finish with one clean integration PR against main.

## Phases

Strictly sequential; each PR needs the one before it.

- **Phase 1** — `#115` (API types + extraEnv map lists)
- **Phase 2** — `#116` (components + controller)
- **Phase 3** — `#117` (data-flow e2e + docs finalization)

## PRs / worktrees

| Issue | Branch                                   | Worktree path                                      | PR (→ base)                | Status      |
| ----- | ---------------------------------------- | -------------------------------------------------- | -------------------------- | ----------- |
| #115  | feat/optimize-controller--api-types      | .claude/worktrees/optimize-controller--api-types   | #119 → feat/optimize-controller | self-merged |
| #116  | feat/optimize-controller--reconciler     | .claude/worktrees/optimize-controller--reconciler  | #121 → feat/optimize-controller | in-progress |
| #117  | test/optimize-controller--data-flow-e2e  | .claude/worktrees/optimize-controller--data-flow-e2e | → feat/optimize-controller | not-started |

## Contracts

None — the PRs are sequential; each consumes the merged result of the one before it.

## Bubble-up log

- 2026-08-20 — A resume found the config-hash fix present in the worktree but never committed, so PR 2 still carried the defect while the snapshot said the fix was requested. The implementer that wrote it was still running, in the same worktree, seventeen hours into its session. Two lessons, both the same shape as the cached-`go test` one below. First: a subagent's report is a claim, and "the work is done" needs the same evidence as "the tests pass" — `git log` and the PR head, not the report. Second: check for a live agent in a worktree before writing to it (`readlink /proc/<pid>/cwd` over the `go`/`git` processes), because two agents on one branch lose work. Resolution: the user stopped the other session; its implementer had by then pushed `5e15124`, which this session re-verified from scratch rather than trusting.
- 2026-08-20 — PR 2 verification caught a defect the subagent's own run missed, because its `go test` reported cached results for the changed packages. Always use `-count=1`. Under CPU load the "reports Ready" spec failed 3 of 3: the config hash included the referenced CamundaCluster's generation, and this controller patches that same cluster, so each attach re-rendered the Deployments a second time and every unrelated cluster edit would roll every Optimize pod. Resolution: drop the cluster generation from the hash, add a `Consistently` guard on the Deployment generation and the hash annotation, and stop the test helper from pinning a stale `observedGeneration`.
- 2026-08-20 — PR 2 balanced review: nothing stopped two `CamundaOptimize` CRs from attaching to one cluster, which broke three things at once — identical Service selectors, one shared SSA field manager whose withdrawal stripped the other CR's exporter entries, and a mutable `clusterRef` with no migration path. Resolution: enforce one Optimize per cluster at reconcile time and make `clusterRef` immutable. Two instances on one cluster were never valid anyway: the Optimize index prefix is fixed by design, so both would write the same analytics indices, which is the same reason the importer is pinned to one replica. Propagation: PR 2 implements it; the spec, the docs, and issues #114/#116 change with it; PR 3 covers it in the e2e docs pass.
- 2026-08-20 — PR 2 balanced review: rejected the finding that the controller must own or watch ServiceMonitors. `camundacluster` does not watch that kind either. An informer on a kind that the API server does not serve breaks cache sync, so support is re-evaluated each reconcile instead. Deliberate, and consistent across both controllers.
- 2026-08-20 — PR 1 quality review: `ReasonUnsupportedStorageType` duplicated the backup kinds' `ReasonStorageTypeMismatch`. Resolution: reuse `ReasonStorageTypeMismatch`, promoted to `api/v1/conditions.go`; spec, plan, and docs renamed in PR 1. Propagation: wave-2 dispatch prompt must use `StorageTypeMismatch`; issues #114/#116 bodies reconciled after the fix commit lands.
- 2026-08-20 — PR 1 quality review: docs pages must use the ocf component reason vocabulary (Healthy/Creating/Updating/...), never `Progressing`. Applies to the PR 3 docs finalization too.
- 2026-08-20 — PR 1 implementer notes: fresh worktrees need `make setup-envtest` (and `chmod -R u+w bin` before worktree removal); `make lint` re-runs the golangci-lint install each time and can hit a transient sum.golang.org 404 — `GOSUMDB=off` works around it. Propagate to wave-2/3 dispatch prompts.
- 2026-08-20 — PR 1 deviation accepted: no printcolumns on CamundaOptimize (no long-lived kind has them); the known printcolumns follow-up covers all kinds at once. `spec.backup.dump.extraEnv` (DumpSpec) deliberately stays atomic.

## Pending snapshot

Phase 1 is done: #119 self-merged as 4f7a302, #115 closed, CI green on its head branch. Phase 2 is PR #121, open, head `5e15124`. Steps 1 to 3 are closed out. Work the rest in order.

1. ~~**Confirm the config-hash fix landed on #121.**~~ Done. It landed as `5e15124` ("keep the config hash out of its own feedback loop"). `resolver.get` splits into `get` (records the generation as a hash input) and `exists` (records nothing); the referenced CamundaCluster reads through `exists`, because this controller writes that object. `resolver.secret` already hashed the source Secret rather than the copy it applies, which is the same rule, and now says so.
2. ~~**Verify #121 for real.**~~ Done, all uncached: `-count=1` green on every changed package and on both modules; 3 of 3 green under one busy loop per core, which is the condition that failed 3 of 3 before the fix; `GOSUMDB=off make lint` clean; `make manifests generate` leaves no diff.
3. ~~**Close out the review threads on #121.**~~ Done. All four threads carry a reply naming the fix and are resolved. The four suppressed findings have their dispositions in one PR comment (`#issuecomment-5358298219`); each was verified against the code before the disposition was written.
4. **Re-request Copilot on #121** — in flight. Requested at watermark `2026-08-20T16:04:45Z`; attachment confirmed through GraphQL, because `gh pr view --json reviewRequests` does not show Bot reviewers. This repo does not auto-review on push, so a re-trigger needs remove-then-re-add. Triage posted **and** suppressed findings; stop at the 3-round cap. Round 2 of 3.
5. **Then the merge bundle:** wait for #121's head-branch CI to go green, `gh pr merge 121 --squash`, `gh issue close 116`, fast-forward the feature worktree, and update this file.
6. **Then wave 3 (#117):** create `.claude/worktrees/optimize-controller--data-flow-e2e` on `test/optimize-controller--data-flow-e2e` off the feature branch and dispatch plan Tasks 9-10. The e2e suite conventions are in the plan; `dumpDiagnostics` in `test/e2e/helpers_test.go` needs `camundaoptimizes` and `managementauthconfigs` added.
7. **Finally the integration PR** `feat/optimize-controller` → `main` with `Closes #114`, review-loop it, tear down the plan and this state file in the last commit once CI is green. Leave it open: the user merges it, not you.

Standing constraints: the user runs close to the Fable limit, so dispatch every subagent with an explicit `model` override (`opus`, or `sonnet`/`haiku` for mechanical work) and never a fork. Fresh worktrees need `make setup-envtest`, and `chmod -R u+w bin` before removal. Flakes get root-caused, never re-run to green.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, check the worktree with `git -C <path> status -sb`.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature`, or work the `## Pending snapshot`.
