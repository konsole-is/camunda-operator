# Shared presets

The three preset kinds, written once and shared by every inventory here. A
preset holds the shape of a resource: the sizing, the topology, and the
resources. A platform team writes it. Each cluster then names it and sets only
what belongs to that one cluster.

What runs on that shape is in [`config/example/releases`](../releases), which
holds one `CamundaRelease`.

| File | Kind | Name | Inherited through |
| --- | --- | --- | --- |
| `camunda-cluster-preset.yaml` | `CamundaClusterPreset` | `small` | `CamundaCluster.spec.presetRef` |
| `elasticsearch-cluster-preset.yaml` | `ElasticsearchClusterPreset` | `standard` | `ElasticsearchCluster.spec.presetRef` |
| `database-server-preset.yaml` | `DatabaseServerPreset` | `standard` | `DatabaseServer.spec.presetRef` |

All three kinds are cluster scoped and passive. No controller reconciles them,
they create nothing, and they report no status. The files carry no numbers,
because nothing here depends on anything else.

## Apply

```sh
kubectl apply -k config/example/presets
```

Apply this once per Kubernetes cluster. Every inventory that uses a preset
also lists this directory in its `kustomization.yaml`, so
`kubectl apply -k <inventory>` applies these too. Because all three come from
this one directory, a second inventory reuses them instead of writing a second
copy.

## What a preset does not hold

- **The instance-bound fields.** Each kind rejects the fields that belong to
  one resource. `CamundaClusterPreset` rejects `platformConfigRef`,
  `presetRef`, `releaseRef`, `externalUrl`, `serviceAccount`, `storageRef`,
  `backupStorageRef`, `documentStorageRef`, `monitoring`, `suspend`, and
  `pause`. The other two reject `presetRef`, `releaseRef`, `suspend`, and the
  name of the contract they publish.
- **A version.** All three preset kinds reject one. `CamundaClusterPreset`
  rejects `version` and `connectors.version`, and the other two reject
  `version`. Every version belongs to a `CamundaRelease`, which
  [`config/example/releases`](../releases) holds.

## Change the fleet

Edit a preset, and every resource that references it reads the new baseline on
its next reconcile. To roll out in steps, create a second preset, for example
`small-v2`, and move resources to it one at a time.

## Related

- [Presets](https://konsole-is.github.io/camunda-operator/guides/presets/)
- [Shared release](../releases): the versions the resources of these inventories run.
- [CamundaClusterPreset](https://konsole-is.github.io/camunda-operator/crds/camundaclusterpreset/)
- [ElasticsearchClusterPreset](https://konsole-is.github.io/camunda-operator/crds/elasticsearchclusterpreset/)
- [DatabaseServerPreset](https://konsole-is.github.io/camunda-operator/crds/databaseserverpreset/)
