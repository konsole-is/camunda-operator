# CamundaRelease

`CamundaRelease` is a cluster-scoped description of what a platform runs. It holds the Camunda version, the connectors version, the Elasticsearch version, the PostgreSQL version, an optional pinned image per Camunda process, and the environment that a version needs. You create it, or another tool creates it for you.

A release separates what runs from the shape of a resource. The shape lives in a preset: [CamundaClusterPreset](camundaclusterpreset.md), [ElasticsearchClusterPreset](elasticsearchclusterpreset.md), or [DatabaseServerPreset](databaseserverpreset.md). A platform team keeps a handful of presets, such as `small` and `medium`, and one release per rollout, such as `camunda-8-9-4`. To move a fleet to a new version set, the team edits one release. To move one resource, the owner changes one `releaseRef`.

A release is passive data. It creates nothing, and it reports no status. A resource that fits no release leaves `releaseRef` unset and sets `version` inline.

The smallest release names one version:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaRelease
metadata:
  name: camunda-8-9-4
spec:
  version: "8.9.4"
```

```mermaid
graph LR
    CC[CamundaCluster] -.->|presetRef| CCP[CamundaClusterPreset]
    CC -.->|releaseRef| CR[CamundaRelease]
    CC -.->|platformConfigRef| PFC[CamundaPlatformConfig]
    EC[ElasticsearchCluster] -.->|releaseRef| CR
    DS[DatabaseServer] -.->|releaseRef| CR
```

## Merge position

A `CamundaCluster`, an `ElasticsearchCluster`, and a `DatabaseServer` each merge their configuration in this order: the preset, then the release, then their own spec. A later layer wins. For a cluster, a release `extraEnv` entry replaces a preset entry with the same name, and a cluster entry replaces both. `extraEnvFrom` concatenates in the same order. The [merge rules](camundaclusterpreset.md#merge-rules) of the preset apply unchanged.

A resource that sets `spec.version` next to a `releaseRef` runs its own version. Leave `spec.version` unset to follow the release.

A `releaseRef` that names no existing release gives `Ready: False` with reason `InvalidReference` on the resource that names it.

## Versions

No version below follows `spec.version`. Each one moves on a line of its own, and `spec.databaseServer.version` is a bare PostgreSQL major rather than three segments.

| Field | What runs it | Kind that reads it |
| --- | --- | --- |
| `spec.version` | The orchestration cluster processes | [CamundaCluster](camundacluster.md) |
| `spec.connectors.version` | The connectors bundle image | [CamundaCluster](camundacluster.md) |
| `spec.elasticsearch.version` | The Elasticsearch nodes | [ElasticsearchCluster](elasticsearchcluster.md) |
| `spec.databaseServer.version` | The PostgreSQL instances, as a major such as `17` | [DatabaseServer](databaseserver.md) |

A cluster that runs connectors needs the bundle version from the release or from its own spec.

One release names the whole set:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaRelease
metadata:
  name: camunda-8-9-4
spec:
  version: "8.9.4"
  connectors:
    version: "8.9.7"
  elasticsearch:
    version: "9.2.8"
  databaseServer:
    version: "17"
```

When you edit a release, every resource that references it rolls to the new version. Each kind keeps its own rule about a version it refuses:

- A cluster whose brokers run a higher version refuses the move and reports `Ready: False` with reason `VersionDowngradeRefused`. The [CamundaCluster page](camundacluster.md#version) states the rule and the annotation that sanctions a downgrade.
- A server whose data directory runs another PostgreSQL major refuses the change and reports `Ready: False` with reason `VersionChangeRefused`. It keeps the major it has. The [DatabaseServer page](databaseserver.md#the-postgresql-version) states the rule.
- An Elasticsearch cluster below the Camunda 8.9 floor of 8.19 or 9.2 reports `Ready: False` with reason `InvalidReference`.

## Pinned images

`spec.images` accepts two entries. `camunda` is the image of every orchestration cluster process, because they all run one image. `connectors` is the image of the connectors runtime. Each value is a complete reference, tag or digest included, and the operator pulls it as it is:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaRelease
metadata:
  name: camunda-8-9-4-cve-2026-0001
spec:
  version: "8.9.4"
  images:
    camunda: "mirror.example.com/camunda/camunda@sha256:7d865e959b2466918c9863afca942d0fb89d7c9ac0c99bafc3749504ded97730"
```

A pinned image changes only what is pulled. The version gates, the downgrade rule, and the computed environment still read `spec.version`, not the image. Pin an image of the same version that you name, for example a digest of it or a patched build of it.

A pin belongs to the version of the release. A cluster that runs another version does not pull it: when `spec.version` on the cluster wins over the release, or a restore sets the version, the cluster pulls the normal repository at that version. The `connectors` pin follows `connectors.version` the same way.

`spec.images` holds these two entries and no more. Elasticsearch takes its image from `elasticsearch.version` through the ECK operator, and PostgreSQL takes its repository from `images.postgres` on the [CamundaPlatformConfig](camundaplatformconfig.md#images), so neither has a pull reference for a release to replace.

To pin an image for a few clusters only, create a second release with the pin and point the `releaseRef` of those clusters at it.

To rename every image of an environment to a mirror, use `spec.images` on the [CamundaPlatformConfig](camundaplatformconfig.md#images) instead. A release pins one exact reference for the clusters that use this release. A platform config renames the repository for every cluster and lets the version supply the tag.

## Environment

A version bump often needs a configuration change with it. A release carries `extraEnv` and `extraEnvFrom` at the top level, per component (`zeebe`, `gateway`, `operate`, `tasklist`, `admin`), and under `connectors`, so the two ship together:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaRelease
metadata:
  name: camunda-8-9-4
spec:
  version: "8.9.4"
  extraEnv:
    - name: CAMUNDA_SOME_NEW_FLAG
      value: "true"
  zeebe:
    extraEnv:
      - name: JAVA_OPTS
        value: "-Xmx8g"
```

## Status

A release reports no status. Reference errors appear on the referencing `CamundaCluster`: a missing release gives `Ready: False` with reason `InvalidReference`.

## Spec reference

Every field, with its type, whether it is required, and its default:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaRelease
metadata:
  # Cluster-scoped: no namespace.
  name: camunda-8-9-4
spec:
  # string. Required. Camunda version as x.y.z. The referencing cluster checks the floor of 8.9.0.
  version: "8.9.4"
  # object. Optional. The connectors runtime of this release.
  connectors:
    # string. Optional. Version of the connectors bundle as x.y.z. Required for clusters that enable connectors, unless each cluster sets its own.
    version: "8.9.7"
    # []EnvVar. Optional. Environment variables of the connectors runtime.
    extraEnv: []
    # []EnvFromSource. Optional. Environment sources of the connectors runtime.
    extraEnvFrom: []
  # object. Optional. The Elasticsearch of this release.
  elasticsearch:
    # string. Optional. Elasticsearch version as x.y.z. The referencing cluster checks the floor of 8.19 or 9.2.
    version: "9.2.8"
  # object. Optional. The PostgreSQL server of this release.
  databaseServer:
    # string. Optional. PostgreSQL major version as a bare number. The referencing server checks the floor of 14.
    version: "17"
  # object. Optional. Complete image references, pulled as they are. A pin applies only while the version of the release is the effective one, and the version stays what the operator believes the process runs.
  images:
    # string. Optional. Image of every orchestration cluster process.
    camunda: "mirror.example.com/camunda/camunda@sha256:7d865e959b2466918c9863afca942d0fb89d7c9ac0c99bafc3749504ded97730"
    # string. Optional. Image of the connectors runtime.
    connectors: "mirror.example.com/camunda/connectors-bundle:8.9.7"
  # []EnvVar. Optional. Environment variables of every workload. They merge by name over the preset entries and under the cluster entries.
  extraEnv:
    - name: CAMUNDA_SOME_NEW_FLAG
      value: "true"
  # []EnvFromSource. Optional. Environment sources of every workload. They follow the preset sources and precede the cluster sources.
  extraEnvFrom: []
  # object. Optional. Environment of the brokers. The same shape as zeebe applies to gateway, operate, tasklist, and admin.
  zeebe:
    # []EnvVar. Optional. Environment variables of this process. An entry here wins over a top-level entry with the same name.
    extraEnv: []
    # []EnvFromSource. Optional. Environment sources of this process.
    extraEnvFrom: []
```

### Validation rules

- `spec.version` is required and must be of the form `x.y.z`. The floor of 8.9.0 is checked by the referencing cluster on the merged spec.
- `spec.connectors.version` and `spec.elasticsearch.version` must be of the form `x.y.z`.
- `spec.databaseServer.version` must be a bare number.
- An `extraEnv` entry sets `value` or `valueFrom`, never both.

### A production-shaped example

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaRelease
metadata:
  name: camunda-8-9-4
spec:
  version: "8.9.4"
  connectors:
    version: "8.9.7"
  elasticsearch:
    version: "9.2.8"
  databaseServer:
    version: "17"
  extraEnv:
    - name: CAMUNDA_SOME_NEW_FLAG
      value: "true"
  zeebe:
    extraEnv:
      - name: JAVA_OPTS
        value: "-Xmx8g"
```

## Related

- [`config/example/releases`](https://github.com/konsole-is/camunda-operator/tree/<version>/config/example/releases): a ready-to-apply release that the example inventories name.
- [CamundaCluster](camundacluster.md): references this resource through `releaseRef` and runs what it names.
- [ElasticsearchCluster](elasticsearchcluster.md): takes `elasticsearch.version` through `releaseRef`.
- [DatabaseServer](databaseserver.md): takes `databaseServer.version` through `releaseRef`.
- [CamundaClusterPreset](camundaclusterpreset.md): the shape of a cluster. The release sits between the preset and the cluster in the merge.
- [CamundaPlatformConfig](camundaplatformconfig.md): renames image repositories for every cluster of an environment.
- [Presets guide](../guides/presets.md): how presets and releases split the configuration of a fleet.
