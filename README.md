# Camunda Operator

A Kubernetes operator that runs [Camunda 8.9+](https://docs.camunda.io/) orchestration clusters, and the storage, backups, and management plane around them.
You describe a cluster in one resource. The operator creates the workloads, connects the storage, and keeps the cluster in the state that you described.

The operator runs on any Kubernetes cluster, on bare metal, on premises, or in a managed cloud. Elasticsearch, PostgreSQL, and Keycloak run inside the cluster through their own operators. Backups go to a bucket that you provide, for example in MinIO, Ceph, S3, GCS, or Azure Blob Storage.

One operator runs the whole Camunda platform. That includes the orchestration clusters, their Elasticsearch or PostgreSQL storage, the logical databases, the management plane with Console, Web Modeler, and optionally Keycloak, and Optimize. The operator also connects these parts. It creates the database users and their passwords, and gives each component the addresses and credentials that it needs. A cluster that the management plane selects shows up in Console without a change on your side.

The operator stops at the cloud account. It never creates buckets, IAM roles, or keys. You bring those yourself, or a tool on top of the operator does. That tool can use the API types from Go through a module that does not pull in the dependencies of the operator.

After the install, the operator handles version upgrades, suspend and resume, storage growth, password rotation, and restores, including a point-in-time restore of PostgreSQL.

> The operator is in early development. The API group is `core.camunda.io/v1`, but the API can still change before the first stable release.

## What it runs

- Orchestration clusters (`CamundaCluster`): Zeebe brokers, the gateway, Operate, Tasklist, Admin, and optionally Connectors.
- The management plane (`CamundaManagementCluster`): Management Identity, Console, and Web Modeler.
- Optimize (`CamundaOptimize`), one for each cluster that needs it.
- Storage backends: Elasticsearch through ECK (`ElasticsearchCluster`), PostgreSQL through CloudNativePG (`DatabaseServer`), and logical databases on a PostgreSQL server (`Database`).
- Backup and restore: logical backups to a bucket, on demand or on a schedule, and restores into a suspended cluster.
- Shared settings: authentication and license (`CamundaPlatformConfig`), presets for sizing, and releases that pin versions and images.

The [CRD reference](docs/crds/index.md) lists every kind with every field.

## Examples

[`config/example`](config/example) holds complete setups that you can apply. There is a cluster on Elasticsearch, a cluster on PostgreSQL, and a management plane with Keycloak or with your own identity provider. Each directory has a README with the apply order.

With the shared presets and release of those examples in place, the manifest below is a complete cluster.

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaCluster
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  presetRef: small
  releaseRef: camunda-8-9
  platformConfigRef: my-platform-config
  storageRef: my-storage-config
```

The broker count, the volumes, and the resources come from the preset `small`. The Camunda version comes from the release `camunda-8-9`. The [presets guide](docs/guides/presets.md) explains both kinds.

## Install

Check the [requirements](docs/installation.md#requirements) first. A few optional features need another operator, and only when you use them. An `ElasticsearchCluster` needs ECK, a `DatabaseServer` needs CloudNativePG, and a Keycloak that the operator runs needs the Keycloak Operator. This operator does not install them.

```bash
helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --create-namespace
```

The [installation guide](docs/installation.md) covers plain manifests, signature verification, CRDs installed separately, upgrades, and removal.

## Run your first cluster

[Getting started](docs/getting-started.md) takes you from an empty Kubernetes cluster to a running Camunda cluster that you can log in to.

## Documentation

- [Getting started](docs/getting-started.md)
- [Installation](docs/installation.md)
- [Architecture](docs/architecture.md): how the resources relate and the rules the operator follows
- [Observability](docs/observability.md): the metrics of the operator, and the dashboards and alerts that ship with it
- [Use the API types from Go](docs/go-api.md): the api module for programs that create or read the CRs
- Guides: [presets](docs/guides/presets.md), [secondary storage](docs/guides/secondary-storage.md), [authentication](docs/guides/authentication.md), [management plane](docs/guides/management-plane.md), [backup](docs/guides/backup.md), [operations](docs/guides/operations.md)
- [CRD reference](docs/crds/index.md)

## Development

```bash
make test           # unit and envtest suites (needs Docker for the PostgreSQL testcontainer)
make lint           # golangci-lint, the line-split shape of calls, and the version pins of config/example
make all            # generate manifests and deepcopy, fmt, vet, build
make test-e2e       # end-to-end suite on a new kind cluster (needs Docker and kind)
make helm-generate  # regenerate dist/chart/ from config/
make docs-serve     # preview the documentation site
```

`dist/chart/values.yaml` and `dist/chart/templates/` are generated. Change the defaults in `config/` and run `make helm-generate`.

Issues and pull requests are welcome at [github.com/konsole-is/camunda-operator](https://github.com/konsole-is/camunda-operator).

## License

Apache License 2.0.
