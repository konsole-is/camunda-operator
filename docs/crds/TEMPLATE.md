<!-- This is the authoring template; copy it to docs/crds/<kind>.md -->
<!--
Every CRD doc has exactly these seven H2 sections, in this order:
Purpose, How it works, API reference, Status, Validation, Relationships, Examples.
Do not add, remove, rename, or reorder H2 sections.
Prose conventions (see the plan's Conventions block): one line per paragraph and per list item, no hard-wrapping; audience is the platform operator ("you"); controller behavior is narrated as "the operator ..." in third person.
Shared vocabulary — use exactly these terms: "orchestration cluster", "secondary storage", "contract CRD", "management plane", "composition layer".
API group is always core.camunda.io/v1.
-->

# Kind

<!-- H1 is the CRD kind name, e.g. "CamundaCluster". Follow it with a one-sentence summary line. -->

One-sentence summary of what this CRD does.

## Purpose

<!--
One paragraph: what this CRD is and why it exists.
State who creates it: the user, a peer controller (name it), a composition layer above, or the control plane.
Contract CRDs add a small producer/consumer table here.
Cloud/SaaS operators may be mentioned only as external actors ("a composition layer above may create this CR") — never document their design.
-->

## How it works

<!--
Controller behavior as numbered reconciliation steps: what the operator does, in order, each cycle.
Passive CRDs (presets) have no controller — describe how consumers resolve and merge them instead.
Include one mermaid relationship diagram: `graph LR` or `graph TD`; solid arrows `-->` mean "creates/provisions"; dotted arrows `-.->` mean "reads/references/patches"; suffix external systems with `(external)` in the node label; use real kind names as node labels.
Spell out "Server-Side Apply (SSA)" on first use; if this controller patches via SSA, name its field manager.
-->

1. Step one.
2. Step two.

```mermaid
graph LR
    A[ThisKind] -.->|clusterRef| CC[CamundaCluster]
```

## API reference

<!--
One fenced YAML block containing the full spec with every field present.
Each field gets a comment line directly above it: `# <type>. <Required|Optional>[, default: <value>]. <one-line meaning>`
Reference-field shapes (api-vocabulary contract):
- namespaced CR ref: object {name (required), namespace (optional, defaults to the referencing CR's namespace)}, field named <thing>Ref
- contract CR ref (shared-infra contracts, presets, platform config are cluster-scoped; binding contracts — SecondaryStorageConfig, DatabaseConfig — are namespaced and resolved in the consumer's own namespace): plain string with the target's name, e.g. storageRef: "my-storage-config"
- single-value secret ref: {name, namespace, key}, field named <thing>SecretRef
- credentials secret ref (username+password): {name, namespace, usernameKey, passwordKey}, field named credentialsSecretRef / adminCredentialsSecretRef / backupCredentialsSecretRef
- output-name field (CR the controller creates): plain string named after the created kind, e.g. secondaryStorageConfig: "my-storage-config"
-->

```yaml
apiVersion: core.camunda.io/v1
kind: Kind
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  # object. Required. Reference to the CamundaCluster this CR attaches to.
  clusterRef:
    # string. Required. Name of the CamundaCluster.
    name: my-cluster
    # string. Optional, default: this CR's namespace. Namespace of the CamundaCluster.
    namespace: my-cluster-ns
```

## Status

<!--
Conditions table first: columns Type | Reason | Meaning (one row per type/reason pair worth documenting).
Conditions are the primary status mechanism; every active CRD has an aggregate `Ready` condition.
Per-component conditions are PascalCase `<Component>Ready` (e.g. `ZeebeReady`). A suspendable CRD reports suspension on `Ready` with reason `Suspended`, never as a separate condition.
Reasons are PascalCase single words: the pre-check reasons `InvalidReference`, `MissingSecret`, `ConnectionFailed`, the validators' `Healthy`, and, on a CRD that runs components, the component framework statuses that `Ready` takes from the governing component (`Healthy`, `Creating`, `Degraded`, `Down`, `Suspended`, `Error`, and more).
Long-running operations (logical backups, restores) additionally document `status.phase` with its enum values.
Every status documents `observedGeneration`.
-->

| Type | Reason | Meaning |
| --- | --- | --- |
| `Ready` | `Healthy` | The resource is fully reconciled. |

The operator records the last reconciled generation in `status.observedGeneration`.

## Validation

<!--
Admission rules enforced when the CR is created or updated: field constraints beyond simple types, cross-field rules, and cross-resource rules (e.g. PointInTimeRestore's dedicated-server rule, Database's name-collision rule).
If there are no rules beyond schema validation, say so in one line.
-->

## Relationships

<!--
Bulleted links to the docs of every CRD this kind references, and every CRD that references this kind.
Keep it bidirectional across the doc set: if A links B here, B links A in its own Relationships section.
External actors (composition layers, external operators like ECK) are named in prose without links.
The bullets are real markdown links; shown here in a code fence only because the target pages do not exist while this template lives alone.
-->

```markdown
- [CamundaCluster](camundacluster.md) — referenced via `clusterRef`.
- LogicalBackupElasticsearch — references this CR via `clusterRef`.
```

## Examples

<!--
Exactly two manifests: one minimal (only required fields) and one realistic (a plausible production shape).
Naming conventions: cluster `my-cluster` in namespace `my-cluster-ns`; derived names prefixed with it (`my-cluster-es`, `my-cluster-backup-001`); config CRs named for their role (`my-storage-config`, `my-db-server`).
-->

A minimal manifest:

```yaml
apiVersion: core.camunda.io/v1
kind: Kind
metadata:
  name: my-cluster-example
  namespace: my-cluster-ns
spec:
  clusterRef:
    name: my-cluster
```

A realistic manifest:

```yaml
apiVersion: core.camunda.io/v1
kind: Kind
metadata:
  name: my-cluster-example
  namespace: my-cluster-ns
spec:
  clusterRef:
    name: my-cluster
    namespace: my-cluster-ns
```
