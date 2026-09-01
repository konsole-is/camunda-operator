# Example inventories

Each directory below holds one complete, apply-able setup. `config/samples/`
holds one manifest per kind, in isolation. These inventories hold the whole
resource chain of one scenario instead, with the references between the
resources already made.

| Inventory | What it stands up |
| --- | --- |
| [`camunda-cluster/elasticsearch`](camunda-cluster/elasticsearch) | One orchestration cluster with Elasticsearch as its secondary storage. |
| [`camunda-cluster/rdbms`](camunda-cluster/rdbms) | One orchestration cluster with PostgreSQL as its secondary storage, run by CloudNativePG. |
| [`camunda-management-cluster/keycloak`](camunda-management-cluster/keycloak) | A management plane whose Keycloak the operator runs, one cluster it serves, and Optimize. |
| [`camunda-management-cluster/oidc`](camunda-management-cluster/oidc) | A management plane against an identity provider you run. |

Each directory carries a README with the apply order and a link to the guide
it condenses.

## How to read a directory

- The manifests are numbered in the order the README applies them. One file
  holds one step of the chain.
- A `kustomization.yaml` lists the same files in the same order, so
  `kubectl apply -k <directory>` applies the whole inventory at once.
- A resource that the operator publishes has no file. The `SecondaryStorageConfig`
  of an `ElasticsearchCluster`, the `DatabaseServerConfig` of a
  `DatabaseServer`, and the `ManagementAuthConfig` of a
  `CamundaManagementCluster` are all written by the operator. A file of the
  same name makes the producer report a conflict.

## Before you apply

- Install the operator, and the third-party operators the inventory needs.
  Each README names them. See
  [Installation](https://konsole-is.github.io/camunda-operator/installation/).
- Replace every placeholder value. A value in angle brackets, such as
  `<your Camunda license key>`, is not valid.
- The four inventories are alternatives. Each one creates the cluster-scoped
  `CamundaPlatformConfig` `my-platform-config` with contents of its own, so
  apply one at a time.

## Versions

The `version` fields hold the versions that the end-to-end suite of this
repository runs, which `test/e2e/matrix/8.9.env` pins. Keep the two in step
when you raise one.
