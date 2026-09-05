# Camunda Operator

A Kubernetes operator that runs [Camunda 8.9+](https://docs.camunda.io/) orchestration clusters.
You describe a cluster in one resource. The operator creates the workloads, wires the storage, and keeps the cluster healthy.

The operator also manages what a cluster needs around it: an Elasticsearch cluster through [ECK](https://www.elastic.co/guide/en/cloud-on-k8s/current/index.html), a PostgreSQL server through [CloudNativePG](https://cloudnative-pg.io/), and backups to a bucket.

> The operator is in early development. The API group is `core.camunda.io/v1`, but the API can still change before the first stable release.

## What it manages

| Resource | What you get |
| --- | --- |
| `CamundaCluster` | One orchestration cluster: Zeebe brokers, gateway, Operate, Tasklist, Admin, and optionally Connectors. |
| `CamundaPlatformConfig` | Settings that all clusters in an environment share: authentication, license, image repositories. |
| `CamundaClusterPreset` | A reusable baseline (sizing, topology) that clusters inherit. |
| `CamundaRelease` | Every version of the platform, the pinned images, and the environment those versions need. |
| `ElasticsearchCluster` | An Elasticsearch cluster run by ECK, with a generated user, ready to be used as secondary storage. |
| `DatabaseServer` | A PostgreSQL server run by CloudNativePG, with a continuous archive in a bucket that a point-in-time restore replays. |
| `Database` | A logical database and its users on a PostgreSQL server. |
| `LogicalBackupElasticsearch`, `LogicalBackupRDBMS` | One backup of a cluster, written to a bucket. |
| `SecondaryStorageConfig`, `ObjectStorageConfig`, `DatabaseServerConfig`, `DatabaseConfig` | Contracts that carry connection details, so the resource that provides a backend and the resource that uses it stay independent. |

The [CRD reference](docs/crds/index.md) lists every kind with every field.

## Requirements

- Kubernetes 1.30 or later.
- The [ECK operator](https://www.elastic.co/guide/en/cloud-on-k8s/current/k8s-deploy-eck.html), if you use `ElasticsearchCluster`. The operator looks for the ECK CRDs when it starts. Install ECK before you create an `ElasticsearchCluster`, then restart the operator. The operator does not install ECK.
- The [CloudNativePG operator](https://cloudnative-pg.io/documentation/current/installation_upgrade/), if you use `DatabaseServer`. An archive also needs the [Barman Cloud plugin](https://cloudnative-pg.io/plugin-barman-cloud/docs/installation/) and [cert-manager](https://cert-manager.io/docs/installation/). The operator looks for their CRDs when it starts. Install them first, then restart the operator. The operator installs none of them.
- A PostgreSQL server, if you use `Database` without a `DatabaseServer`.

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

## Use the API types from Go

The CRD types are a Go module of their own, with `k8s.io/api` and
`k8s.io/apimachinery` as their only dependencies:

```bash
go get github.com/konsole-is/camunda-operator/api@v<version>
```

```go
import camundav1 "github.com/konsole-is/camunda-operator/api/v1"
```

See [Use the API types from Go](docs/go-api.md) for the version scheme and the
root module.

## Documentation

- [Getting started](docs/getting-started.md)
- [Installation](docs/installation.md)
- [Architecture](docs/architecture.md): how the resources relate and how the operator is built
- [Observability](docs/observability.md): the metrics of the operator, and the dashboards and alerts that ship with it
- [Use the API types from Go](docs/go-api.md): the api module for programs that create or read the CRs
- Guides: [presets](docs/guides/presets.md), [secondary storage](docs/guides/secondary-storage.md), [authentication](docs/guides/authentication.md), [backup](docs/guides/backup.md), [operations](docs/guides/operations.md)
- [CRD reference](docs/crds/index.md)

## Development

```bash
make test           # unit and envtest suites (needs Docker for the PostgreSQL testcontainer)
make lint           # golangci-lint, callsplit, and the version pins of config/example
make all            # generate manifests and deepcopy, fmt, vet, build
make test-e2e       # kind cluster with ECK, CloudNativePG, PostgreSQL, and MinIO (needs Docker and kind)
make helm-generate  # regenerate dist/chart/ from config/
make docs-serve     # preview the documentation site
```

`dist/chart/values.yaml` and `dist/chart/templates/` are generated. Change the defaults in `config/` and run `make helm-generate`.

Issues and pull requests are welcome at [github.com/konsole-is/camunda-operator](https://github.com/konsole-is/camunda-operator).

## License

Apache License 2.0.
