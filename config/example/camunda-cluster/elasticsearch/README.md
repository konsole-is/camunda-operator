# Camunda cluster on Elasticsearch

A complete orchestration cluster that keeps its secondary storage in
Elasticsearch. The cluster is `my-cluster` in the namespace `my-cluster-ns`.

The manifests use the names and the values of
[Getting started](https://konsole-is.github.io/camunda-operator/getting-started/).
The sizes suit one small test cluster, such as kind.

This inventory sets every field inline and names no preset. It is the one
example of the explicit shape. The other three inherit their sizing from
[`config/example/presets`](../../presets). Compare its `CamundaCluster` with
the one in [`camunda-cluster/rdbms`](../rdbms) to see the difference.

## Before you start

- Install the ECK operator and the Camunda operator. See
  [Installation](https://konsole-is.github.io/camunda-operator/installation/).
- Give the cluster about 4 GB of free memory and a default StorageClass.
- On kind, raise the map count that Elasticsearch needs:
  `sudo sysctl -w vm.max_map_count=262144`.

## Apply

One command applies the whole inventory:

```sh
kubectl apply -k config/example/camunda-cluster/elasticsearch
```

To see each resource become ready, apply the files in their number order:

1. `01-namespace.yaml` creates the namespace `my-cluster-ns`.
2. `02-elasticsearch-cluster.yaml` creates the `ElasticsearchCluster`
   `my-cluster-es`. Wait for it:

    ```sh
    kubectl wait elasticsearchcluster/my-cluster-es -n my-cluster-ns \
      --for=condition=Ready --timeout=15m
    ```

3. `03-platform-config.yaml` creates the cluster-scoped
   `CamundaPlatformConfig` `my-platform-config`.
4. `04-camunda-cluster.yaml` creates the `CamundaCluster` `my-cluster`.
   Wait for it:

    ```sh
    kubectl wait camundacluster/my-cluster -n my-cluster-ns \
      --for=condition=Ready --timeout=15m
    ```

## What you get

- `my-cluster-es` publishes the `SecondaryStorageConfig` `my-storage-config`.
  The inventory holds no such file, because the operator writes it.
- `my-cluster-gateway` is the Service of the cluster, on port 8080.
- `my-cluster-camunda-admin` is the Secret with the first administrator. Read
  the keys `username` and `password` from it to log in.

## Remove

Delete in the reverse order, so that the cluster stops before its storage:

```sh
kubectl delete camundacluster/my-cluster -n my-cluster-ns
kubectl delete elasticsearchcluster/my-cluster-es -n my-cluster-ns
kubectl delete camundaplatformconfig/my-platform-config
kubectl delete namespace my-cluster-ns
```

## Related

- [Getting started](https://konsole-is.github.io/camunda-operator/getting-started/)
- [Secondary storage](https://konsole-is.github.io/camunda-operator/guides/secondary-storage/#elasticsearch)
- [CamundaCluster](https://konsole-is.github.io/camunda-operator/crds/camundacluster/)
- [ElasticsearchCluster](https://konsole-is.github.io/camunda-operator/crds/elasticsearchcluster/)
