# Shared release

One `CamundaRelease`, written once and named by every `CamundaCluster` in
these inventories that follows a fleet version. A release holds what runs: the
Camunda version, the connectors version, an optional pinned image per process,
and the environment that a version needs.

| File | Kind | Name | Inherited through |
| --- | --- | --- | --- |
| `camunda-release.yaml` | `CamundaRelease` | `camunda-8-9` | `CamundaCluster.spec.releaseRef` |

`CamundaRelease` is cluster scoped and passive. No controller reconciles it,
it creates nothing, and it reports no status. The file carries no number,
because nothing here depends on anything else.

A preset holds the shape of a cluster, and a release holds what runs on that
shape. The two split apart on purpose: a fleet has a few shapes and one
version at a time. The shapes are in [`config/example/presets`](../presets).

## Apply

```sh
kubectl apply -k config/example/releases
```

Apply this once per Kubernetes cluster. Every inventory whose `CamundaCluster`
names the release also lists this directory in its `kustomization.yaml`, so
`kubectl apply -k <inventory>` applies it too.

## What the release holds

```yaml
spec:
  version: "8.9.18"
  connectors:
    version: "8.9.9"
```

- **`version`** is the Camunda version of the orchestration cluster processes.
  A cluster that names this release runs it and leaves `spec.version` unset.
- **`connectors.version`** is the version of the connectors bundle image. The
  bundle has a patch line of its own, so it does not follow `version`. No
  inventory here enables connectors, and a cluster that turns them on in a
  preset gets the bundle version from here.

Both values are the ones the end-to-end suite of this repository runs, which
`test/e2e/matrix/8.9.env` pins.

The release sets no `images` and no `extraEnv`. A release also pins an exact
image reference per process, and carries the environment that a version needs,
which the [CamundaRelease](https://konsole-is.github.io/camunda-operator/crds/camundarelease/)
page shows.

## Roll the fleet

Raise `spec.version` in this one file, and every cluster that names
`camunda-8-9` rolls to the new version on its next reconcile. That is the
point of a release: the version set of a platform lives in one place.

To roll in steps, create a second release named for its rollout, for example
`camunda-8-9-19`, and move clusters to it one at a time by changing
`releaseRef`. That is the name the presets guide gives two releases that stand
side by side.

A cluster whose brokers run a higher version refuses a lower one and reports
`Ready: False` with the reason `VersionDowngradeRefused`. The
[CamundaCluster](https://konsole-is.github.io/camunda-operator/crds/camundacluster/#version)
page states the rule and the annotation that sanctions a downgrade.

## What the release does not hold

- **The shape.** The sizing, the topology, and the resources are in a preset.
  A release that carries them ties one version to one shape.
- **The Elasticsearch and PostgreSQL versions.** `ElasticsearchCluster` and
  `DatabaseServer` still pin their versions on the instance in these
  inventories.
- **The versions of the management plane.** Management Identity, Console, and
  Web Modeler each carry a patch line of their own on the
  `CamundaManagementCluster`.

## Related

- [CamundaRelease](https://konsole-is.github.io/camunda-operator/crds/camundarelease/)
- [Presets](https://konsole-is.github.io/camunda-operator/guides/presets/)
- [CamundaCluster](https://konsole-is.github.io/camunda-operator/crds/camundacluster/)
