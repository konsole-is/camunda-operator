# Camunda Operator

A Kubernetes operator that runs [Camunda 8.9+](https://docs.camunda.io/) orchestration clusters.
You describe a cluster in one resource. The operator creates the workloads, wires the storage, and keeps the cluster healthy.

> The operator is in early development. The API group is `core.camunda.io/v1`, but the API can still change before the first stable release.

## What it runs

- Orchestration clusters (`CamundaCluster`): Zeebe brokers, the gateway, Operate, Tasklist, Admin, and optionally Connectors.
- The management plane (`CamundaManagementCluster`): Management Identity, Console, and Web Modeler.
- Optimize (`CamundaOptimize`), one for each cluster that needs it.
- Storage backends: Elasticsearch through ECK (`ElasticsearchCluster`), PostgreSQL through CloudNativePG (`DatabaseServer`), and logical databases on a PostgreSQL server (`Database`).
- Backup and restore: logical backups to a bucket, on demand or on a schedule, and restores into a suspended cluster.
- Shared settings: authentication and license (`CamundaPlatformConfig`), presets for sizing, and releases that pin versions and images.

The [CRD reference](docs/crds/index.md) lists every kind with every field.

## Requirements

- Kubernetes 1.30 or later.
- For each backend you use, the operator that runs it: ECK for `ElasticsearchCluster`, CloudNativePG for `DatabaseServer`. The operator does not install them.

[Installation](docs/installation.md#requirements) lists the supported versions and the other requirements.

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
