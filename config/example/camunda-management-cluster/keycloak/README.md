# Management plane with the operator's Keycloak

A complete management plane in which the operator runs Keycloak, plus one
orchestration cluster it serves and the Optimize that reads its contract.

The management plane is `my-management` in the namespace `my-management-ns`.
The orchestration cluster is `my-cluster` in the namespace `my-cluster-ns`.
The manifests use the names of
[Management plane](https://konsole-is.github.io/camunda-operator/guides/management-plane/).

## Before you start

- Install the Keycloak Operator, the CloudNativePG operator, the ECK
  operator, and the Camunda operator. See
  [Installation](https://konsole-is.github.io/camunda-operator/installation/).
  Install the Keycloak Operator release that matches
  `spec.identityProvider.keycloak.version`.
- Replace the placeholder values in `02-secrets.yaml`. Both Secrets hold
  values that no cluster accepts.
- Point the four `externalUrl` hostnames at your own domain, and route each
  one to the Service of its component.

## Apply

One command applies the whole inventory:

```sh
kubectl apply -k config/example/camunda-management-cluster/keycloak
```

To see each resource become ready, apply the files in their number order:

1. `01-namespaces.yaml` creates both namespaces.
2. `02-secrets.yaml` creates the license Secret and the SMTP Secret.
3. `03-database-server.yaml` creates the `DatabaseServer` `my-db`. Wait for it:

    ```sh
    kubectl wait databaseserver/my-db -n my-management-ns \
      --for=condition=Ready --timeout=10m
    ```

4. `04-databases.yaml` creates the three `Database` resources.
5. `05-platform-config.yaml` creates the cluster-scoped
   `CamundaPlatformConfig` `my-platform-config`.
6. `06-management-cluster.yaml` creates the `CamundaManagementCluster`
   `my-management`. Wait for it:

    ```sh
    kubectl wait camundamanagementcluster/my-management -n my-management-ns \
      --for=condition=Ready --timeout=15m
    ```

7. `07-elasticsearch-cluster.yaml` creates the `ElasticsearchCluster`
   `my-cluster-es`.
8. `08-camunda-cluster.yaml` creates the `CamundaCluster` `my-cluster`. Wait
   for it:

    ```sh
    kubectl wait camundacluster/my-cluster -n my-cluster-ns \
      --for=condition=Ready --timeout=15m
    ```

9. `09-optimize.yaml` creates the `CamundaOptimize` `my-cluster-optimize`.

## What you get

- `my-management` publishes the cluster-scoped `ManagementAuthConfig`
  `my-management`. The inventory holds no such file, because the operator
  writes it. A file of that name would make the plane report `Conflict`.
- `my-management-keycloak-service` serves Keycloak under the path `/auth`.
  `my-management-identity`, `my-management-console`, and the Web Modeler
  Services serve the other components.
- `my-management-identity-admin` is the Secret with the password of the first
  administrator, under the key `password`.
- `my-cluster` carries the annotation `camunda.io/management-cluster` once the
  plane attaches to it.
- `status.optimize` of `my-management` lists `my-cluster-optimize`, because it
  names this contract and sets an `externalUrl`.

## Why the cluster runs on Elasticsearch

Optimize reads the secondary storage of its cluster, and it reads
Elasticsearch only. A cluster on a relational database makes its
`CamundaOptimize` report `StorageTypeMismatch`. Drop `07`, `08`, and `09` if
you want the management plane alone.

## Remove

```sh
kubectl delete camundaoptimize/my-cluster-optimize -n my-cluster-ns
kubectl delete camundacluster/my-cluster -n my-cluster-ns
kubectl delete elasticsearchcluster/my-cluster-es -n my-cluster-ns
kubectl delete camundamanagementcluster/my-management -n my-management-ns
kubectl delete database --all -n my-management-ns
kubectl delete databaseserver/my-db -n my-management-ns
kubectl delete camundaplatformconfig/my-platform-config
kubectl delete namespace my-management-ns my-cluster-ns
```

## Related

- [Management plane](https://konsole-is.github.io/camunda-operator/guides/management-plane/#step-3a-the-operator-runs-keycloak)
- [CamundaManagementCluster](https://konsole-is.github.io/camunda-operator/crds/camundamanagementcluster/)
- [CamundaOptimize](https://konsole-is.github.io/camunda-operator/crds/camundaoptimize/)
