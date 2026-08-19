# Camunda Operator

A Kubernetes operator that runs [Camunda 8.9+](https://docs.camunda.io/) orchestration clusters.
You describe a cluster in one resource. The operator creates the workloads, wires the storage, and keeps the cluster healthy.

The operator also manages what a cluster needs around it: an Elasticsearch cluster through [ECK](https://www.elastic.co/guide/en/cloud-on-k8s/current/index.html), a logical database on a PostgreSQL server you provide, and backups to a bucket.

> The operator is in early development. The API group is `core.camunda.io/v1`, but the API can still change before the first stable release.

## What it manages

| Resource | What you get |
| --- | --- |
| `CamundaCluster` | One orchestration cluster: Zeebe brokers, gateway, Operate, Tasklist, Admin, and optionally Connectors. |
| `CamundaPlatformConfig` | Settings that all clusters in an environment share: authentication, license, image registry. |
| `CamundaClusterPreset` | A reusable baseline (sizing, topology) that clusters inherit. |
| `ElasticsearchCluster` | An Elasticsearch cluster run by ECK, with a generated user, ready to be used as secondary storage. |
| `Database` | A logical database and its users on an existing PostgreSQL server. |
| `LogicalBackupElasticsearch`, `LogicalBackupRDBMS` | One backup of a cluster, written to a bucket. |
| `SecondaryStorageConfig`, `ObjectStorageConfig`, `DatabaseServerConfig`, `DatabaseConfig` | Contracts that carry connection details, so the resource that provides a backend and the resource that uses it stay independent. |

The [CRD reference](docs/crds/index.md) lists every kind with every field.

## Requirements

- Kubernetes 1.30 or later.
- The [ECK operator](https://www.elastic.co/guide/en/cloud-on-k8s/current/k8s-deploy-eck.html). The operator watches ECK `Elasticsearch` resources and does not start without the ECK CRDs, even if you never create an `ElasticsearchCluster`. The operator does not install ECK.
- A PostgreSQL server, if you use `Database`. The operator does not run PostgreSQL.

## Install

```bash
helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --create-namespace
```

The [installation guide](docs/installation.md) covers plain manifests, signature verification, CRDs installed separately, upgrades, and removal.

## Run your first cluster

[Getting started](docs/getting-started.md) takes you from an empty namespace to a running cluster you can log in to. It takes about 15 minutes on a local cluster.

## Documentation

- [Getting started](docs/getting-started.md)
- [Installation](docs/installation.md)
- [Architecture](docs/architecture.md): how the resources relate and how the operator is built
- Guides: [secondary storage](docs/guides/secondary-storage.md), [authentication](docs/guides/authentication.md), [backup](docs/guides/backup.md), [operations](docs/guides/operations.md)
- [CRD reference](docs/crds/index.md)

## Development

```bash
make test           # unit and envtest suites (needs Docker for the PostgreSQL testcontainer)
make lint           # golangci-lint
make all            # generate manifests and deepcopy, fmt, vet, build
make test-e2e       # kind cluster with ECK, PostgreSQL, and MinIO (needs Docker and kind)
make helm-generate  # regenerate dist/chart/ from config/
make docs-serve     # preview the documentation site
```

`dist/chart/values.yaml` and `dist/chart/templates/` are generated. Change the defaults in `config/` and run `make helm-generate`.

Issues and pull requests are welcome at [github.com/konsole-is/camunda-operator](https://github.com/konsole-is/camunda-operator).

## License

Apache License 2.0.
