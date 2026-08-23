---
feature: management-cluster
spec: docs/superpowers/specs/2026-08-23-management-cluster-design.md
plan: docs/superpowers/plans/2026-08-23-management-cluster-plan.md
tracking_issue: #185
feature_branch: feat/management-cluster
feature_worktree: .claude/worktrees/management-cluster
sub_pr_approval: autonomous
sub_pr_review_loop: on
sub_pr_target: feature-branch
integration_pr:
status: consumer-wave
---

# Management plane — orchestration state

The user granted full autonomy on 2026-08-23: file issues, write the plan, implement, review-loop every sub-PR until clean, self-merge, open the integration PR. The user is not available for questions; the orchestrator decides and records the decision here. The merge to main stays the user's.

## Phases

- **Wave 1 (foundational)** — `#186` API types and shared contracts
- **Wave 2 (foundational)** — `#187` Identity on oidc, contract output, cluster discovery and claim (depends on #186)
- **Wave 3 (consumers, parallel)** — `#188` Keycloak modes, `#189` Console + ping, `#190` Web Modeler + cluster list + user (depend on #187)
- **Wave 4 (parallel)** — `#191` e2e flows, `#192` docs (depend on wave 3)

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #186 | feat/management-cluster--api-types | .claude/worktrees/management-cluster/.claude/worktrees/api-types | #193 → feat/management-cluster | self-merged (64ccf29) |
| #187 | feat/management-cluster--identity-oidc-contract | .claude/worktrees/management-cluster/.claude/worktrees/identity-oidc-contract | #200 → feat/management-cluster | self-merged (b80fd1a) |
| #188 | feat/management-cluster--keycloak-modes | .claude/worktrees/management-cluster/.claude/worktrees/keycloak-modes | #205 → feat/management-cluster | ready |
| #189 | feat/management-cluster--console-ping | .claude/worktrees/management-cluster/.claude/worktrees/console-ping | #203 → feat/management-cluster | ready |
| #190 | feat/management-cluster--web-modeler | .claude/worktrees/management-cluster/.claude/worktrees/web-modeler | #204 → feat/management-cluster | ready |
| #191 | test/management-cluster--e2e-flows | .claude/worktrees/management-cluster/.claude/worktrees/e2e-flows | | not-started |
| #192 | docs/management-cluster--user-docs | .claude/worktrees/management-cluster/.claude/worktrees/user-docs | | not-started |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `management-api-types` | pre-merge stub PR | #193 (64ccf29) | locked |
| `platform-oidc-management` | pre-merge stub PR | #193 (64ccf29) | locked |
| `platform-images` | pre-merge stub PR | #193 (64ccf29) | locked |
| `cluster-gateway-status` | pre-merge stub PR | #193 (64ccf29) | locked |
| `keycloak-cr-types` | pre-merge stub PR | #193 (64ccf29) | locked |
| `management-render-core` | pre-merge stub PR | #200 (b80fd1a) | locked |
| `management-controller-core` | pre-merge stub PR | #200 (b80fd1a) | locked |
| `ping-and-list-env` | data-only | plan table | locked |
| `e2e-flow-names` | data-only | plan table | locked |

## Bubble-up log

- 2026-08-23 — #205 (Keycloak modes) decisions: Identity re-applies the whole client representation on every start (verified in Identity source, camunda/camunda#59963), so redirect URIs can change across restarts; but `CamundaOptimize` has no `externalUrl` field, so the Optimize client root URL cannot be discovered. `spec.optimize.externalUrl` is added and required in the Keycloak modes (the contract always publishes the Optimize client; its Keycloak client needs a root URL), forbidden in oidc. Follow-up (not this epic): `CamundaOptimize.spec.externalUrl` + discovery by the management cluster, then the field becomes optional. Console is bootstrapped through the `KEYCLOAK_INIT_CONSOLE_*` preset (chart parity), not `KEYCLOAK_CLIENTS_<n>_*`; `KEYCLOAK_SETUP_REALM`/`CLIENT_ID` rendered with the documented defaults, no new field. `identity.admin` gains `email` (+ `firstName`/`lastName` optional with defaults, needed by Web Modeler for the first user). #203 (Console) decisions: ping endpoint = Console's in-cluster Service URL (spec corrected on the branch); 8.10 hub ping needs M2M credentials (unfiled follow-up); `ForceOwnership` on the four ping entries kept and documented on `CamundaClusterSpec.ExtraEnv`. #204 (Web Modeler): `BASIC` carries no credentials (the Web Modeler docs: the person enters them in the UI), the dedicated user is still created with the documented authorizations; `URL_GRPC` gets `grpc://`; pusher app id is the fixed identifier `web-modeler`; `SERVER_HTTPS_ONLY`/`PUSHER_APP_PATH` recorded in the spec.
- 2026-08-23 — Wave-2 checkpoint (union of #193+#200 at b80fd1a): not blocking. Decisions: the ping entries get their own field manager `camunda-operator/camundamanagementcluster-ping` (claim and ping under one manager would strip each other's fields and loop); wave-3 rules added to the plan Conventions (app-prefixed helpers, own `MirrorPurpose` constants, own golden fixture dirs, branches in `workload`/`replicas`/`componentEnv`/`resolveDatabases`/`finalize`/`attach`, hooks after `recordInitialClaim`, per-component hash scoping); `mirroredSecretComponent` moves into the builder list with its name in `names.go` as part of #188; Identity readiness `/actuator/health` (chart parity) and the `MirroredSecretsReady` condition recorded in the spec as shipped. Known limitations logged: `ownedBy` compares the bounded owner name (two >63-char same-prefix names in one namespace could both claim a contract); ping writes bump the cluster generation and roll its pods (expected — the env must reach the processes; #192 documents it).
- 2026-08-23 — #200 reviews. Decisions: `Ready` reflects a failed contract write (`Failed/WriteFailed` or `Conflict`) because the contract is what Optimize consumes; one cluster's claim conflict no longer stalls the plane (row reason, continue); the duplicated `WorkloadSpec` mutations move to a shared `pkg/workloadmutations` used by `camundaoptimize` and the management package (waves 3/4 use it too, never copy); the contract gets its own component value `management-auth`; `status.clusters` lists selected clusters only (issue #187 reconciled with a decision comment); the speculative names in `names.go` stay because they are the wave-3 contract; global `HashInputs` per component is a known limitation — wave-3 implementers scope hash inputs per component where a rotation must not roll every pod. Copilot's two "nil selector selects all" comments were wrong (`LabelSelectorAsSelector(nil)` is `Nothing()`), pinned by new specs.
- 2026-08-23 — Copilot on #193 flagged a confused-deputy risk: a namespaced kind selecting and patching CamundaClusters in every namespace. Decision: keep cross-namespace selection (one management plane serves a platform; creating the kind is a platform-administrator action, the RBAC boundary) but adopt LabelSelector semantics: unset selector = no cluster, `{}` = all. Propagated: spec Decision 6 and API section, plan Task 2.3 Step 0 (GoDoc fix + platform-admin note), #187 dispatch prompt. The #192 docs state it.
- 2026-08-23 — #193 Copilot loop reached the 3-round cap still producing "previously missed" findings on unchanged code. Decision: the correct round-3 items were applied in a final commit set (passwordSecretRef forbidden in oidc mode, repository-name pattern on `spec.images`, named `OIDCProviderType`, `hub-websockets` 8.10 default if the chart confirms it); the owner-label-with-namespace item was rejected (the contract carries `camunda.io/management-cluster-namespace` by design; #187 writes it). No round 4 requested; the orchestrator verified the final diff and merged. `reviewing-feature-progress` at the wave-2 boundary re-checks the api types.
- 2026-08-23 — Worktree subagents inherit the orchestrator's Bash isolation pin (`.claude/worktrees/management-cluster`) and cannot run commands in a sibling worktree. Decision: sub-worktrees live nested under the feature worktree at `.claude/worktrees/management-cluster/.claude/worktrees/<sub-name>` (`**/.claude/worktrees/` is gitignored). The #186 worktree was moved there; waves 2+ are created there. The PR-table paths below are the nested ones.
- 2026-08-23 — Copilot on #193: `MinLength=1` added on `IdentityAdminSpec.claimName/claimValue/username` (`has()` admits an explicit empty string). Pushed back on "GoDoc promises behaviour the scaffold does not implement": the kind's GoDoc is its contract and the controller lands on this feature branch before the integration PR.
- 2026-08-23 — Cluster discovery and the claim annotation moved from #189 (Console) into #187 (core) so that #189 and #190 can run in parallel against one attachment. Propagated: plan Task 2.3, issue bodies #187 and #189 updated.

## Pending snapshot

1. Wave-2 checkpoint (coherence audit of #193+#200 at b80fd1a) is running; its findings become bubble-up entries, a follow-up sub-PR, or notes in the wave-3 dispatch prompts.
2. Dispatch wave 3 (#188 `keycloak-modes`, #189 `console-ping`, #190 `web-modeler`; worktrees exist at b80fd1a) in parallel with plan Tasks 3.1–3.7, the Conventions block, the locked contracts (shapes as shipped by #200: `Build(in) (Built, error)`, builder list, `resolved{Input, ContractName}`, `preCheck`, `attachedClusters`), `pkg/workloadmutations`, and the decisions in the Bubble-up log. Per PR: copilot-review-loop (3-round cap, background wait via Monitor + `scratchpad/await-review.sh`) → spec pass → quality pass → squash-merge → `gh issue close` → state row. Resolve the one-line conflicts in `components.go`/`controller.go` at merge.
3. Wave-3 checkpoint, then dispatch wave 4 (#191 e2e, #192 docs) in parallel.
4. Final `reviewing-feature-progress`, integration PR `feat/management-cluster` → `main` with `Closes #185`, copilot-review-loop on it, teardown of plan and state after CI is green (spec stays), report ready-to-merge. The merge to main is the user's.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature` (parallel waves still in flight; the orchestrator watch loop continues), or work the `## Pending snapshot` when development is past.
