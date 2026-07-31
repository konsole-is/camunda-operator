---
feature: crd-docs
spec: docs/superpowers/specs/2026-07-31-crd-docs-design.md
plan: docs/superpowers/plans/2026-07-31-crd-docs-plan.md
tracking_issue: #1
feature_branch: feature/crd-docs
feature_worktree: .claude/worktrees/crd-docs
sub_pr_approval: autonomous
sub_pr_review_loop: off
sub_pr_target: feature-branch
integration_pr:
status: review
---

# CRD documentation foundation — orchestration state

## Phases

- **Phase 1 (foundational)** — `#2` (docs tooling, template, architecture rewrite, CRD index — the conventions everything inherits)
- **Phase 2 (parallel docs batches)** — `#3` (contracts), `#4` (core cluster), `#5` (storage backends), `#6` (backup & restore), `#7` (management & extensions)
- **Phase 3 (review + integration)** — coherence review, deviation audit, integration PR → main (plan Task 7)

## PRs / worktrees

| Issue | Branch | Worktree path | PR (→ base) | Status |
| --- | --- | --- | --- | --- |
| #2 | crd-docs/tooling | .claude/worktrees/crd-docs--tooling | #8 → feature/crd-docs | self-merged |
| #3 | crd-docs/contracts | .claude/worktrees/crd-docs--contracts | #10 → feature/crd-docs | self-merged |
| #4 | crd-docs/core | .claude/worktrees/crd-docs--core | #13 → feature/crd-docs | self-merged |
| #5 | crd-docs/storage | .claude/worktrees/crd-docs--storage | #9 → feature/crd-docs | self-merged |
| #6 | crd-docs/backup-restore | .claude/worktrees/crd-docs--backup | #12 → feature/crd-docs | self-merged |
| #7 | crd-docs/mgmt-extensions | .claude/worktrees/crd-docs--mgmt | #11 → feature/crd-docs | self-merged |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `crd-doc-template` | pre-merge PR | #8 | locked |
| `crd-file-paths` | data-only (plan Task 1 inventory table) | n/a | locked |
| `mkdocs-nav-groups` | pre-merge PR | #8 | locked |
| `api-vocabulary` | data-only (plan Conventions section) | n/a | locked |

## Bubble-up log

- **2026-07-31 — Phase 3 coherence checklist accumulated during ripening**: (a) audit every mermaid edge on all 19 pages + architecture.md + index.md — recurring backwards-dotted-edge defect found in PRs #8, #9, #11, #13; (b) settle "acts-on" edge style: BackupRetention deletes-edge drawn solid in backupretention.md but dotted in backup.md; PITR WAL-replay edge solid vs backup.md's dotted management-API edge; (c) linkify cross-batch backticked kind references (bidirectional Relationships) now that all pages exist, incl. elasticsearchcluster.md ↔ pvcautoresize.md discovery story and managementauthconfig.md naming the `managementAuthConfig` output field; (d) confirm component-label vocabulary across pages (`optimize-webapp`, `keycloak`, ... vs core page's component names); (e) linkify the File column of crds/index.md if wanted.

- **2026-07-31 — CamundaManagementCluster API additions** (raised by #7's subagent at PR #11): optional `platformConfigRef` (string) + output-name field `managementAuthConfig` (string, default: CR name), both per api-vocabulary conventions, filling proposal gaps. Coherence review must confirm camundaplatformconfig.md lists CamundaManagementCluster as a consumer and managementauthconfig.md's producer story matches. Also: management-plane component label values (`optimize-webapp`, `optimize-importer`, `keycloak`, `identity`, `console`, `web-modeler`) need Phase 3 vocabulary check against the core batch; singleton admission rule ("at most one CamundaManagementCluster per Kubernetes cluster") accepted as design decision. Console self-registration is experimental in 8.9 — recorded in page.
- **2026-07-31 — elasticsearchcluster.md ↔ pvcautoresize.md bidirectionality** (raised by #7): PVCAutoResize discovers the ES cluster via storageRef → SecondaryStorageConfig → producing ElasticsearchCluster; elasticsearchcluster.md's PVCAutoResize bullet needs to match at Phase 3.

- **2026-07-31 — cluster-scoped secret refs: namespace Required** (raised by #3's subagent at PR #10). Template's "defaults to referencing CR's namespace" only works for namespaced CRs. Resolution: convention added to plan; propagated to #4 (CamundaPlatformConfig) and #7 (CamundaManagementCluster) agents. Also propagated: ObjectStorageConfig workload-identity-only pairing with CamundaCluster serviceAccount.annotations (#4); ManagementAuthConfig gained authUrl per verified 8.9 Identity surface (#7); Optimize-on-rdbms must yield a defined failure condition (#7).
- **2026-07-31 — recurring defect pattern: backwards dotted diagram edges** (PR #8 fix, PR #9 findings ×3). Proposal diagrams drew referenced → referencing; our convention is referencing-CR → referenced-CR. Resolution: fix-loops per PR; Phase 3 coherence review must audit every dotted edge across all 19 pages + architecture.md + index.md.

- **2026-07-31 — storage batch conventions adopted feature-wide** (raised by #5's subagent at PR #9): field-manager scheme `camunda-operator/<kind lowercase>`; deviation admonition `!!! note "Deviation from the original proposal"`; preset pages reject instance-bound fields and report no status; verified 8.9 facts (ES 8.19+/9.2+ rec., RDBMS GA, Database bootstrap postgres-only, Optimize needs ES/OS). Resolution: added to plan Conventions. Propagation: SendMessage to #3/#4/#6/#7 agents.
- **2026-07-31 — PVCAutoResize discovery-story wrinkle** (raised by #5): ES data PVCs carry the ElasticsearchCluster's name in `camunda.io/cluster`, but PVCAutoResize's `clusterRef` points at a CamundaCluster. Resolution: #7's agent instructed to make a concrete, implementable discovery choice and flag it; Phase 3 reconciles the bidirectional Relationships bullets.
- **2026-07-31 — RDBMS continuous backup lead** (raised by #5): 8.9 docs mention continuous backup/restore for RDBMS, possibly conflicting with the proposal's pg_dump-Job framing. Resolution: #6's agent instructed to verify specifically and document reality.

- **2026-07-31 — cross-batch links break per-batch strict builds** (raised by #2's subagent). mkdocs strict fails on links to pages another unmerged batch owns. Resolution: batches write real links only to their own batch's pages plus `index.md`/`architecture.md`; other kinds are named in backticks; Phase 3 (plan Task 7) converts backticked names to links once all 19 pages exist. Propagation: goes into every Phase 2 dispatch prompt; plan Task 7 Step 1 covers the linkify pass.
- **2026-07-31 — `site/` added to `.gitignore`** by #2 (mkdocs build output; not in plan's file list). Accepted; batches inherit it.
- **2026-07-31 — protected-branch guard keys off parent checkout** — sub-worktree commits must use `git -C <worktree>` instead of `cd` + `git commit`. Propagation: goes into every Phase 2 dispatch prompt.
- **2026-07-31 — empty nav groups pass strict** — mkdocs.yml ships all five CRD nav groups as empty lists; each batch replaces only its own group's `[]`. Propagation: Phase 2 dispatch prompts.

## Pending snapshot

1. Phase 2 in flight: five subagents dispatched (issues #3–#7, plan Tasks 2–6) in the worktrees listed above. As each PR opens: orchestrator two-pass review (spec gate, then quality + conventions vs siblings), fix-loop via the author agent, squash-merge into `feature/crd-docs`, `gh issue close <n>`, update the row. mkdocs.yml nav-group lines are adjacent — later merges may conflict; on conflict have the author rebase onto latest `feature/crd-docs` and re-push.
2. Wave checkpoint after all five self-merge (`feature-dev-workflow:reviewing-feature-progress`): cross-batch coherence sweep is the big one (five parallel authors), plus strict build on the integrated feature branch.
3. Phase 3: plan Task 7 — coherence fixes, backtick-to-link conversion for cross-batch Relationships (see bubble-up log), deviation audit, integration PR `feature/crd-docs` → `main` with `Closes #1`. Merge to main is the user's.

## Resume checklist

For a fresh Claude session resuming this work: invoke `feature-dev-workflow:resuming-a-feature` — it executes the steps below, routes by the `status:` frontmatter, and works the `## Pending snapshot`. Fallback if that skill is unavailable:

1. Read this state file in full.
2. Read the plan at the path in the `plan:` frontmatter.
3. Read the spec at the path in the `spec:` frontmatter.
4. Verify each open PR's actual state via `gh pr view <num>`.
5. For each `in-progress` or `draft` row, `cd` to the worktree path and check `git status -sb` — an `ahead` count in the header means unpushed commits, and no upstream in the header means the branch was never pushed at all.
6. Re-dispatch subagents as needed per `feature-dev-workflow:developing-a-feature` (parallel waves still in flight; the orchestrator watch loop continues), or work the `## Pending snapshot` when development is past.
