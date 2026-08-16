---
name: verifying-camunda-app-config
description: Use when you write, change, or review Camunda application config (environment variables, Spring properties, application.yaml keys) for Zeebe, Operate, Tasklist, Identity, Optimize, Connectors, or Web Modeler, in an operator, Helm chart, or deployment manifest.
---

# Verifying Camunda App Config

## Why this skill exists

Camunda applications are Spring Boot applications. They read config through Spring relaxed binding.
A wrong key is not an error. Spring ignores a key that it does not know, the deployment starts, and the
feature does nothing. You cannot tell a correct key from a typo by reading your own code.

**Never guess a key or a value from the naming pattern. Find it in a source of truth first.**

## Sources of truth

Use the sources in this order. The first one that is reachable is enough.

| Order | Source | How to reach it |
| --- | --- | --- |
| 1 | The `camunda/camunda` repository, at the tag that matches the target version | A local checkout, or a working directory added with `--add-dir` |
| 2 | The `camunda/camunda` repository on GitHub, at the target tag | Fetch `https://raw.githubusercontent.com/camunda/camunda/<tag>/dist/src/main/config/defaults.yaml` |
| 3 | The Camunda docs MCP | Tool `search_camunda_knowledge_sources` |

Use the docs MCP also when you have the repository. The repository proves that the key exists. The docs
explain what the value does.

**If no source is reachable, stop.** Do not answer from memory. Tell the user which sources you tried,
and how to add one:

> I cannot verify this config. The `camunda/camunda` repository is not available (add it with
> `--add-dir /path/to/camunda/camunda`), I cannot fetch GitHub, and the `camunda-docs` MCP is not
> in this session. Add one of these and I will check the key and its meaning.

## Procedure

1. Write down the target Camunda version. The key must exist in that version, not only in `main`.
   If you only know the minor version, use the newest patch tag of that minor.
2. Convert the key to both forms. Env form: uppercase, dots become underscores, hyphens are removed.
   Property form: lowercase, dots, hyphens kept.
   - `CAMUNDA_OPERATE_ELASTICSEARCH_CLUSTERNAME` <-> `camunda.operate.elasticsearch.cluster-name`
3. Find the config file. Do not hardcode a path. There is no single `application.yaml`.
   - `dist/src/main/config/defaults.yaml` holds the orchestration cluster defaults. Each line names
     its env var: `cluster-name: null # Type: String, Env: CAMUNDA_OPERATE_ELASTICSEARCH_CLUSTERNAME`.
   - Other components keep their own file. Find them with:
     ```bash
     find "$CAMUNDA" \( -path '*/config/*application*.y*ml' -o -name defaults.yaml \) -not -path '*/target/*'
     ```
4. Search the file for the key stem in either form:
   ```bash
   grep -rn 'cluster-name\|CAMUNDA_OPERATE_ELASTICSEARCH_CLUSTERNAME' "$CAMUNDA/dist/src/main/config/defaults.yaml"
   ```
   A hit confirms the property, the exact env var, and the type for that checkout.
5. If the local checkout is not at the target version, read the same file at the target tag:
   ```bash
   git -C "$CAMUNDA" show 8.9.9:dist/src/main/config/defaults.yaml | grep -n 'cluster-name'
   ```
   If the tag is not in the local checkout, fetch the file from GitHub (source 2).
6. Make sure that the component still uses the key at that version. A key can exist and be superseded.
   For example, in 8.9 the Camunda Exporter writes to Elasticsearch, not the Operate importer, so the
   importer keys have no effect on the export path. The docs MCP is the best source for this.
7. If the value has non-obvious meaning (units, enum values, defaults), search the docs MCP for the key.
8. If the key belongs to a feature that is not released yet, read the upstream Java config class or
   test config in `camunda/camunda`. Do not derive the key from the feature description.
9. In your answer, name the source and the version that you used. Name any source that was not
   reachable.

## Checklist before you deliver

- [ ] At least one source was reachable. If none was, you stopped and asked for one.
- [ ] The key was found in a source, not derived from the naming pattern.
- [ ] The key exists at the target version, and the component still uses it at that version.
- [ ] The env form and the property form match (hyphens removed in the env form).
- [ ] The answer names the source, the version, and any source that was unavailable.

## Red flags

| Thought | Reality |
| --- | --- |
| "The pattern is obvious, the key must be right." | Spring ignores unknown keys. Confirm it in a source. |
| "I know this key from memory." | Keys change between versions. Confirm it at the target tag. |
| "I will read `dist/src/main/config/application.yaml`." | That file does not exist. Find the file. |
| "The local checkout has it, so the release has it." | The checkout can be ahead of the release. Check the tag. |
| "No source is reachable, but I am fairly sure." | A confident guess looks verified. Stop and ask for a source. |
