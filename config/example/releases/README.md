# Shared release

One `CamundaRelease`, written once and named by every resource in these
inventories that follows a fleet version. A release holds what runs: the
Camunda version, the connectors version, the Elasticsearch version, the
PostgreSQL version, an optional pinned image per Camunda process, and the
environment that a version needs.

| File | Kind | Name | Inherited through |
| --- | --- | --- | --- |
| `camunda-release.yaml` | `CamundaRelease` | `camunda-8-9` | `CamundaCluster.spec.releaseRef`, `ElasticsearchCluster.spec.releaseRef`, `DatabaseServer.spec.releaseRef` |

`CamundaRelease` is cluster scoped and passive. No controller reconciles it,
it creates nothing, and it reports no status. The file carries no number,
because nothing here depends on anything else.

A preset holds the shape of a resource, and a release holds what runs on that
shape. The two split apart on purpose: a fleet has a few shapes and one
version set at a time. The shapes are in
[`config/example/presets`](../presets).

## Apply

```sh
kubectl apply -k config/example/releases
```

Apply this once per Kubernetes cluster. Every inventory that names the release
also lists this directory in its `kustomization.yaml`, so
`kubectl apply -k <inventory>` applies it too.

## What the release holds

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaRelease
metadata:
  name: camunda-8-9
spec:
  version: "8.9.18"
  connectors:
    version: "8.9.9"
  elasticsearch:
    version: "9.2.8"
  databaseServer:
    version: "17"
```

- **`version`** is the Camunda version of the orchestration cluster processes.
  A cluster that names this release runs it and leaves `spec.version` unset.
- **`connectors.version`** is the version of the connectors bundle image. The
  bundle has a patch line of its own, so it does not follow `version`. No
  inventory here enables connectors, and a cluster that turns them on in a
  preset gets the bundle version from here.
- **`elasticsearch.version`** is the Elasticsearch version. An
  `ElasticsearchCluster` that names this release runs it and leaves
  `spec.version` unset.
- **`databaseServer.version`** is the PostgreSQL major version. A
  `DatabaseServer` that names this release runs it and leaves `spec.version`
  unset.

The Camunda, connectors, and Elasticsearch versions are the ones the
end-to-end suite of this repository runs, which `test/e2e/matrix/8.9.env`
pins. The PostgreSQL major is the one that suite runs as well. Renovate raises
these versions and that file together, and `make lint` fails when the two
differ.

The release sets no `images` and no `extraEnv`. A release also pins an exact
image reference per process, and carries the environment that a version needs,
which the [CamundaRelease](https://konsole-is.github.io/camunda-operator/crds/camundarelease/)
page shows.

## Roll the fleet

Raise a version in this one file, and every resource that names `camunda-8-9`
rolls to it. That is the point of a release: the version set of a platform
lives in one place.

To roll in steps, create a second release and move resources to it one at a
time by changing `releaseRef`. Name that second release for the version it
runs, `camunda-8-9-19` for Camunda 8.9.19, which is the convention the presets
guide states for two releases side by side.

Each kind keeps its own rule about a version it refuses. A cluster whose
brokers run a higher version refuses a lower one and reports `Ready: False`
with the reason `VersionDowngradeRefused`. A server whose data directory runs
another PostgreSQL major refuses the change and reports `Ready: False` with the
reason `VersionChangeRefused`. The
[CamundaCluster](https://konsole-is.github.io/camunda-operator/crds/camundacluster/#version)
and
[DatabaseServer](https://konsole-is.github.io/camunda-operator/crds/databaseserver/)
pages state each rule.

## What the release does not hold

- **The shape.** The sizing, the topology, and the resources are in a preset.
  A release that carries them ties one version to one shape.
- **A pinned storage image.** `images` pins the Camunda image and the
  connectors image. Elasticsearch takes its image from its version through the
  ECK operator, and PostgreSQL takes its repository from the
  `CamundaPlatformConfig`, so neither has a pull reference for a release to
  replace.
- **The versions of the management plane.** Management Identity, Console, and
  Web Modeler each carry a patch line of their own on the
  `CamundaManagementCluster`.

## Related

- [CamundaRelease](https://konsole-is.github.io/camunda-operator/crds/camundarelease/)
- [Presets](https://konsole-is.github.io/camunda-operator/guides/presets/)
- [CamundaCluster](https://konsole-is.github.io/camunda-operator/crds/camundacluster/)
- [ElasticsearchCluster](https://konsole-is.github.io/camunda-operator/crds/elasticsearchcluster/)
- [DatabaseServer](https://konsole-is.github.io/camunda-operator/crds/databaseserver/)
