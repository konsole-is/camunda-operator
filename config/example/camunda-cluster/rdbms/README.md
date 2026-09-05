# Camunda cluster on PostgreSQL

A complete orchestration cluster that keeps its secondary storage in a
PostgreSQL database. The operator runs the server through CloudNativePG. The
cluster is `my-cluster` in the namespace `my-cluster-ns`.

The manifests use the names of
[Secondary storage](https://konsole-is.github.io/camunda-operator/guides/secondary-storage/#postgresql).
The sizes suit one small test cluster, such as kind.

The sizing lives in [`config/example/presets`](../../presets). The
`DatabaseServer` names the preset `standard`, the `CamundaCluster` names
`small`, and each one keeps only what belongs to it.

The versions live in [`config/example/releases`](../../releases). The
`CamundaCluster` and the `DatabaseServer` both name the release `camunda-8-9`
and set no `version` of their own. Raise a version in that one file, and every
resource that names the release follows.

## Before you start

- Install the CloudNativePG operator and the Camunda operator. See
  [Installation](https://konsole-is.github.io/camunda-operator/installation/).
- Give the cluster a default StorageClass.

The `DatabaseServer` here has no `spec.archive`, so it needs neither the
Barman Cloud plugin nor cert-manager. Point-in-time recovery needs both, and
an `ObjectStorageConfig` for the bucket.

## Apply

One command applies the whole inventory, presets and release included:

```sh
kubectl apply -k config/example/camunda-cluster/rdbms
```

To see each resource become ready, apply the shared resources once, then the
files in their number order:

1. The shared presets and the shared release, once per Kubernetes cluster:

    ```sh
    kubectl apply -k config/example/presets
    kubectl apply -k config/example/releases
    ```

2. `01-namespace.yaml` creates the namespace `my-cluster-ns`.
3. `02-database-server.yaml` creates the `DatabaseServer` `my-db`. It inherits
   the instance count, the volume size, and the resources from the preset
   `standard`, and its PostgreSQL version from the release `camunda-8-9`. Wait
   for it:

    ```sh
    kubectl wait databaseserver/my-db -n my-cluster-ns \
      --for=condition=Ready --timeout=10m
    ```

4. `03-database.yaml` creates the `Database` `my-camunda-db`. Wait for it:

    ```sh
    kubectl wait database/my-camunda-db -n my-cluster-ns \
      --for=condition=Ready --timeout=5m
    ```

5. `04-platform-config.yaml` creates the cluster-scoped
   `CamundaPlatformConfig` `my-platform-config`.
6. `05-camunda-cluster.yaml` creates the `CamundaCluster` `my-cluster`. It
   inherits the broker count, the partitions, the volumes, and the resources
   from the preset `small`, and its version from the release `camunda-8-9`.
   Wait for it:

    ```sh
    kubectl wait camundacluster/my-cluster -n my-cluster-ns \
      --for=condition=Ready --timeout=15m
    ```

## What you get

- `my-db` publishes the `DatabaseServerConfig` `my-db-server`. It names the
  superuser Secret `my-db-superuser` that CloudNativePG writes.
- `my-camunda-db` publishes the `DatabaseConfig` `my-camunda-db` and the
  `SecondaryStorageConfig` `my-storage-config`. It creates the database
  `camunda` and the Secret `my-camunda-db-credentials`.
- The inventory holds no contract file, because the operator writes all three.
- `my-cluster-gateway` is the Service of the cluster, on port 8080.
- `my-cluster-camunda-admin` is the Secret with the first administrator. Read
  the keys `username` and `password` from it to log in.

## Add a second cluster

The preset and the release are the point. A second cluster of the same shape
is `05-camunda-cluster.yaml` again with another name, another `storageRef`,
and its own `Database`. The sizing stays in `small`, and the versions stay in
`camunda-8-9`.

## Remove

Delete in the reverse order, so that the cluster stops before its database:

```sh
kubectl delete camundacluster/my-cluster -n my-cluster-ns
kubectl delete database/my-camunda-db -n my-cluster-ns
kubectl delete databaseserver/my-db -n my-cluster-ns
kubectl delete camundaplatformconfig/my-platform-config
kubectl delete namespace my-cluster-ns
```

Deletion of the `Database` removes the published contracts and Secrets. It
never drops the database or its roles.

The presets and the release are shared, so leave them unless no inventory uses
them any more.

## Related

- [Presets](https://konsole-is.github.io/camunda-operator/guides/presets/)
- [CamundaRelease](https://konsole-is.github.io/camunda-operator/crds/camundarelease/)
- [Secondary storage](https://konsole-is.github.io/camunda-operator/guides/secondary-storage/#postgresql)
- [DatabaseServer](https://konsole-is.github.io/camunda-operator/crds/databaseserver/)
- [Database](https://konsole-is.github.io/camunda-operator/crds/database/)
- [CamundaCluster](https://konsole-is.github.io/camunda-operator/crds/camundacluster/)
