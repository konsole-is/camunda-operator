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
status: foundational-wave
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
| #3 | crd-docs/contracts | .claude/worktrees/crd-docs--contracts | → feature/crd-docs | in-progress |
| #4 | crd-docs/core | .claude/worktrees/crd-docs--core | → feature/crd-docs | in-progress |
| #5 | crd-docs/storage | .claude/worktrees/crd-docs--storage | → feature/crd-docs | in-progress |
| #6 | crd-docs/backup-restore | .claude/worktrees/crd-docs--backup | → feature/crd-docs | in-progress |
| #7 | crd-docs/mgmt-extensions | .claude/worktrees/crd-docs--mgmt | → feature/crd-docs | in-progress |

## Contracts

| Name | Realization | Realized in | Status |
| --- | --- | --- | --- |
| `crd-doc-template` | pre-merge PR | #8 | locked |
| `crd-file-paths` | data-only (plan Task 1 inventory table) | n/a | locked |
| `mkdocs-nav-groups` | pre-merge PR | #8 | locked |
| `api-vocabulary` | data-only (plan Conventions section) | n/a | locked |

## Bubble-up log

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
