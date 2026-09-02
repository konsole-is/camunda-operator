# Example inventories

Each directory below holds one complete, apply-able setup. `config/samples/`
holds one manifest per kind, in isolation. These inventories hold the whole
resource chain of one scenario instead, with the references between the
resources already made.

| Inventory | What it stands up | Shape |
| --- | --- | --- |
| [`presets`](presets) | The three preset kinds, shared by the inventories below. | Applied once |
| [`camunda-cluster/elasticsearch`](camunda-cluster/elasticsearch) | One orchestration cluster with Elasticsearch as its secondary storage. | Every field inline |
| [`camunda-cluster/rdbms`](camunda-cluster/rdbms) | One orchestration cluster with PostgreSQL as its secondary storage, run by CloudNativePG. | Presets |
| [`camunda-management-cluster/keycloak`](camunda-management-cluster/keycloak) | A management plane whose Keycloak the operator runs, one cluster it serves, and Optimize. | Presets |
| [`camunda-management-cluster/oidc`](camunda-management-cluster/oidc) | A management plane against an identity provider you run. | Presets |

Each directory carries a README with the apply order and a link to the guide
it condenses.

## Why the presets come first

A preset is the shape of a resource: the sizing, the topology, the resources.
It is cluster scoped, and it is written once. The three inventories that use
presets share the one copy in [`presets`](presets), so a `CamundaCluster`
shrinks to its references, its URL, and its version:

```yaml
spec:
  presetRef: small
  version: "8.9.17"
  platformConfigRef: my-platform-config
  storageRef: my-storage-config
```

That is the point of a preset. A platform team writes `small` once, and a
second cluster of the same shape is the four lines above with another name.
Change `small`, and every cluster that names it follows.

Because a preset is cluster scoped, one copy per inventory would collide by
name and would undo that. There is one copy, and the inventories point at it.

`camunda-cluster/elasticsearch` deliberately sets every field inline and names
no preset. It is the one example of the explicit shape, so a reader sees both
and can compare its `CamundaCluster` with the one in `camunda-cluster/rdbms`.

## How to read a directory

- The manifests are numbered in the order the README applies them. One file
  holds one step of the chain. The files in [`presets`](presets) carry no
  numbers, because nothing there depends on anything else.
- A `kustomization.yaml` lists the same files in the same order, so
  `kubectl apply -k <directory>` applies the whole inventory at once. An
  inventory that uses presets lists `../../presets` first, so one command is
  still enough.
- A resource that the operator publishes has no file. The
  `SecondaryStorageConfig` of an `ElasticsearchCluster`, the
  `DatabaseServerConfig` of a `DatabaseServer`, the `DatabaseConfig` of a
  `Database`, and the `ManagementAuthConfig` of a `CamundaManagementCluster`
  are all written by the operator. A file of the same name makes the producer
  report a conflict.

## Before you apply

- Install the operator, and the third-party operators the inventory needs.
  Each README names them. See
  [Installation](https://konsole-is.github.io/camunda-operator/installation/).
- Replace every placeholder value. A value in angle brackets, such as
  `<your Camunda license key>`, is not valid.
- The four scenarios are alternatives. Each one creates the cluster-scoped
  `CamundaPlatformConfig` `my-platform-config` with contents of its own, so
  apply one at a time. The presets are the exception: all four agree on them,
  so they are shared.

## Versions

The `version` fields hold the versions that the end-to-end suite of this
repository runs, which `test/e2e/matrix/8.9.env` pins. Keep the two in step
when you raise one.

In these inventories a version stays on the instance, so one cluster can move
before the fleet does. A preset can carry `cluster.version` instead, and the
[presets guide](https://konsole-is.github.io/camunda-operator/guides/presets/)
shows that shape.
