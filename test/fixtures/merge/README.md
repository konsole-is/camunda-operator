# Merge fixtures

The cases in this directory show how the operator merges a preset, a release, and a resource spec. The tests in this directory run each case against the merge function of the operator. The fleet manager runs its own merge against the same cases.

## Layout

There is one directory for each kind:

| Directory | Merge function | Rules table |
| --- | --- | --- |
| `camundacluster/` | `MergeSpec` in `pkg/components/camundacluster` | "Merge rules" in `docs/crds/camundaclusterpreset.md` |
| `elasticsearchcluster/` | `MergeSpec` in `pkg/components/elasticsearchcluster` | "Merge rules" in `docs/crds/elasticsearchclusterpreset.md` |
| `databaseserver/` | `MergeSpec` in `pkg/components/databaseserver` | "Merge rules" in `docs/crds/databaseserverpreset.md` |

The Go files in this directory are tests only. They are not part of the operator.

## Format

Each `<case>.json` file holds one case:

```json
{
  "schemaVersion": 1,
  "name": "a cluster client id replaces the whole client of the preset",
  "rule": "`auth.clientId`, `auth.audience`, `auth.clientSecretRef`",
  "preset": {},
  "release": {},
  "spec": {},
  "merged": {}
}
```

- `schemaVersion` is the version of this format. It changes only when the format changes.
- `name` says what the case shows.
- `rule` is the first cell of a row of the rules table of the kind, character for character.
- `preset` is the spec of the preset. Without it, the merge has no preset.
- `release` is the spec of the `CamundaRelease`. Only `camundacluster` cases have it.
- `spec` is the spec of the resource.
- `merged` is the spec that the merge function returns. Do not write it by hand.

The field names are the field names of the API. A wrong name or a wrong case makes the test fail.

The `elasticsearchcluster` and `databaseserver` merges also take the version from the release. Their cases have no `release`, so these cases do not show that rule yet. Their `version` cases show only a version that the resource sets.

## Add a case

1. Write the file without `merged`.
2. Run `make merge-fixtures`. It writes `merged` into every case and formats each file.
3. Read `merged`. If it is not what the rule says, the case or the rule is wrong. Do not commit it.
4. Commit the case.

## Add or change a rule

Each row of a rules table needs at least one case. The tests fail when a row has no case, or when a case names a rule that is not a row. When you add a row to a table, add a case for it in the same change. When you change the text of the first cell, change the `rule` of its cases too. A first cell must not contain a `|` character.

When you change a merge function, run `make merge-fixtures` and commit the changed cases. The Lint workflow fails when the cases differ from the output of `make merge-fixtures`.
