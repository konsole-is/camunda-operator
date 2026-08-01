---
name: verifying-camunda-app-config
description: Use when configuring or changing Camunda application config (env vars / Spring properties) emitted by camunda-operator. Cross-references keys and values against the camunda-docs MCP and the pinned config files in the camunda/camunda repo before trusting them.
---

# Verifying Camunda App Config

## Why this exists

Camunda apps (Operate, Tasklist, Zeebe gateway, ...) are Spring Boot apps. They read config via Spring **relaxed binding**, and the operator emits that config as **environment variables**. A wrong key is not an error: Spring silently ignores an env var it does not recognize, the operator reports success, and the feature does nothing. **You cannot tell a correct key from a typo by looking at the operator alone.** Every config key and value semantics MUST be cross-checked against an upstream source of truth, never guessed from the relaxed-binding pattern. Refs camunda-operator #5531, #5477.

## Headline: no source reachable = STOP, do not guess

This skill verifies config against sources. **If no source is reachable, it verifies nothing, and a confident-sounding guess is worse than no answer, because it looks verified.**

Two sources, in priority order:

1. The **`camunda/camunda` repo**: normally an additional working directory in this environment.
2. The **`camunda-docs` MCP**: tool `search_camunda_knowledge_sources` (present in some sessions only).

Before answering, check which are reachable:

| camunda/camunda repo | camunda-docs MCP | Action |
| --- | --- | --- |
| present | either | Verify against the repo (most precise). Use docs to confirm semantics. |
| absent | available | Verify via `search_camunda_knowledge_sources`. Say the repo was unavailable. |
| **absent** | **unavailable** | **STOP. Do not answer from memory or the relaxed-binding pattern.** Tell the user exactly what to add (below). |

When only one source is available, use it and state which one you used and which was unavailable.

### What to say when both sources are missing

Do not fabricate. Reply with exactly what is missing and how to supply it, e.g.:

> I can't verify this config. Neither source is reachable: the `camunda/camunda` repo isn't in my working directories (add it with `--add-dir /path/to/camunda/camunda`), and the `camunda-docs` MCP (`search_camunda_knowledge_sources`) isn't available in this session. Add one of those and I'll cross-check the property name and value semantics against it.

## Env var ↔ Spring property mapping

The operator emits env vars; upstream config files are YAML dotted properties. Relaxed binding maps between them: uppercase, dots → underscores, and **hyphens within a segment are removed** in the env form.

- `CAMUNDA_OPERATE_ELASTICSEARCH_URL` ↔ `camunda.operate.elasticsearch.url`
- `CAMUNDA_OPERATE_ELASTICSEARCH_BATCHSIZE` ↔ `camunda.operate.elasticsearch.batch-size` (note: `batch-size`, not `batchsize`)

The mapping tells you *what to look for*. It does **not** confirm the property exists for the target version: confirm that against a source. Conveniently, `defaults.yaml` annotates each property with its exact env-var name (`# ... Env: CAMUNDA_OPERATE_ELASTICSEARCH_URL`), so you can match on either form.

## Discover the right config file (do not hardcode a path)

There is **no single `application.yaml`** in camunda/camunda. The orchestration-core defaults live in `dist/src/main/config/defaults.yaml`; per-component config lives elsewhere (e.g. `operate/config/application.yml`). Discover it, then grep for the property stem:

```bash
CAMUNDA=/path/to/camunda/camunda   # an additional working dir; absolute path
find "$CAMUNDA" \( -path '*/config/*application*.y*ml' -o -name defaults.yaml \) 2>/dev/null
grep -rn 'elasticsearch.*url\|CAMUNDA_OPERATE_ELASTICSEARCH_URL' "$CAMUNDA/dist/src/main/config/defaults.yaml"
```

A hit on `dist/src/main/config/defaults.yaml` like `url: null # Type: String, Env: CAMUNDA_OPERATE_ELASTICSEARCH_URL` confirms **both** the dotted property and the exact env var for the version checked out in that working dir.

**Targeting a specific Camunda version/tag** (the working-dir checkout may differ from the deployed version): read the file at that tag instead, e.g. `https://raw.githubusercontent.com/camunda/camunda/<tag>/dist/src/main/config/defaults.yaml`.

## Verify via the docs MCP

When the repo is absent (or to confirm value semantics), query the docs: `search_camunda_knowledge_sources` with the property name and component, and confirm it applies to the targeted version.

## Integrating a feature built on the camunda/camunda side

When the config you are wiring is for a feature being developed upstream, pull `camunda/camunda` into context (it is already a working dir) and read the **actual** config shape from its source: the new property may not be in docs or a released `defaults.yaml` yet. Read the upstream code/config, do not infer the key from the feature description.

## Verification checklist

- [ ] Confirmed at least one source (repo or docs MCP) is reachable; if neither, STOPPED and told the user what to add.
- [ ] Located the property in a source (config file or docs), not inferred from the env-var pattern.
- [ ] Confirmed the property exists for the **targeted Camunda version** (checked-out tag or `raw.githubusercontent.com/<tag>`).
- [ ] Mapped env var ↔ dotted property correctly (hyphens removed in env form).
- [ ] Stated which source verified it, and named any source that was unavailable.

## Red flags (STOP)

- "The relaxed-binding pattern is obvious, so the key must be right." → Spring ignores wrong keys silently; confirm against a source.
- About to answer with no source reachable. → STOP, ask for `--add-dir` the camunda/camunda repo and/or the docs MCP.
- Hardcoding `dist/src/main/config/application.yaml`. → That path does not exist; discover the file.
- Verified against a working-dir checkout but the deployed version differs. → Check the property at the targeted tag.
