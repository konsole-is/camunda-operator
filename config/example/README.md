# Example inventories

Each directory below holds one complete, apply-able setup. `config/samples/`
holds one manifest per kind, in isolation. These inventories hold the whole
resource chain of one scenario instead, with the references between the
resources already made.

| Inventory | What it stands up | Shape |
| --- | --- | --- |
| [`presets`](presets) | The three preset kinds, shared by the inventories below. | Applied once |
| [`releases`](releases) | One `CamundaRelease`, named by the clusters and the storage of the three inventories below it. | Applied once |
| [`camunda-cluster/elasticsearch`](camunda-cluster/elasticsearch) | One orchestration cluster with Elasticsearch as its secondary storage. | Every field inline |
| [`camunda-cluster/rdbms`](camunda-cluster/rdbms) | One orchestration cluster with PostgreSQL as its secondary storage, run by CloudNativePG. | Presets and release |
| [`camunda-management-cluster/keycloak`](camunda-management-cluster/keycloak) | A management plane whose Keycloak the operator runs, one cluster it serves, and Optimize. | Presets and release |
| [`camunda-management-cluster/oidc`](camunda-management-cluster/oidc) | A management plane against an identity provider you run. | Presets and release |

Each directory carries a README with the apply order and a link to the guide
it condenses.

## Why the presets and the release come first

A preset is the shape of a resource: the sizing, the topology, the resources.
A release is what runs on that shape: the Camunda version, the connectors
version, the Elasticsearch version, and the PostgreSQL version. Both kinds are
cluster scoped, and both are written once. The inventories share the one copy
in [`presets`](presets) and the one copy in [`releases`](releases), so a
`CamundaCluster` shrinks to its references:

```yaml
spec:
  presetRef: small
  releaseRef: camunda-8-9
  platformConfigRef: my-platform-config
  storageRef: my-storage-config
```

That is the point. A platform team writes `small` and `camunda-8-9` once, and
a second cluster of the same shape is the four lines above with another name.
Change `small`, and every cluster that names it follows. Raise the version in
`camunda-8-9`, and every cluster that names it rolls.

Both kinds are cluster scoped, so a copy in each inventory collides by name.
That collision undoes the sharing. There is one copy of each, and the
inventories point at them.

`camunda-cluster/elasticsearch` deliberately sets every field inline and names
neither a preset nor a release. It is the one example of the explicit shape,
so a reader sees both and can compare its `CamundaCluster` with the one in
`camunda-cluster/rdbms`.

## How to read a directory

- The manifests are numbered in the order the README applies them. One file
  holds one step of the chain. The files in [`presets`](presets) and
  [`releases`](releases) carry no numbers, because nothing there depends on
  anything else.
- A `kustomization.yaml` lists the same files in the same order, so
  `kubectl apply -k <directory>` applies the whole inventory at once. An
  inventory that uses a shared kind lists its directory first, `../../presets`
  or `../../releases`, so one command is still enough.
- A resource that the operator publishes has no file. The
  `SecondaryStorageConfig` of an `ElasticsearchCluster`, the
  `DatabaseServerConfig` of a `DatabaseServer`, the `DatabaseConfig` of a
  `Database`, and the `ManagementAuthConfig` of a `CamundaManagementCluster`
  are all written by the operator. A file of the same name makes the producer
  report a conflict.

## Before you apply

- Get the files. Clone this repository, or point `kubectl apply -k` at the
  directory on GitHub and name the operator version you run:

    ```sh
    kubectl apply -k "https://github.com/konsole-is/camunda-operator//config/example/camunda-cluster/rdbms?ref=<version>"
    ```

    The remote form reads the shared presets and the shared release too, so
    one command is still enough. Every other command below assumes a local
    clone.

- Install the operator, and the third-party operators the inventory needs.
  Each README names them. See
  [Installation](https://konsole-is.github.io/camunda-operator/installation/).
- Replace every placeholder value. A value in angle brackets, such as
  `<your Camunda license key>`, is not valid.
- The four scenarios are alternatives. Each one creates the cluster-scoped
  `CamundaPlatformConfig` `my-platform-config` with contents of its own, so
  apply one at a time. The presets and the release are the exception: every
  inventory that names them agrees on them, so they are shared.

## Versions

Every version here is one that the end-to-end suite of this repository runs,
which `test/e2e/matrix/8.9.env` pins. Keep the two in step when you raise one.

The Camunda, Elasticsearch, and PostgreSQL versions are in the shared release
[`releases/camunda-release.yaml`](releases/camunda-release.yaml). Every
`CamundaCluster`, `ElasticsearchCluster`, and `DatabaseServer` that names it
takes them, and one edit to that file rolls all of them. An instance that must
move before the fleet does sets `spec.version`, which wins over the release.
The `ElasticsearchCluster` and the `CamundaCluster` of
`camunda-cluster/elasticsearch` name no release and pin their versions inline.

The other versions stay on the instance. `CamundaOptimize` pins its own.
Management Identity, Console, and Web Modeler each carry a patch line of their
own on the `CamundaManagementCluster`.
