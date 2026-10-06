# Camunda Operator

A Kubernetes operator that runs [Camunda 8.9+](https://docs.camunda.io/) orchestration clusters, and the storage, backups, and management plane around them.
You describe a cluster in one resource. The operator creates the workloads, connects the storage, and keeps the cluster in the state that you described.

> The operator is in early development. The API group is `core.camunda.io/v1`, but the API can still change before the first stable release.

## At a glance

- **It runs on any Kubernetes.** Bare metal, on premises, or a managed cloud cluster. Elasticsearch, PostgreSQL, and Keycloak run inside your cluster through their own operators. Backups go to a bucket that you provide: S3, an S3-compatible store such as MinIO or Ceph, GCS, or Azure Blob Storage.
- **It is a base layer, not a full platform.** The operator never creates cloud resources such as buckets, IAM roles, or keys. You bring those, or a tool above it does. That tool can use the API types from Go through a separate module that has no operator dependencies.
- **Features attach to a cluster.** A backup (`LogicalBackupElasticsearch`), a schedule (`BackupSchedule`), or Optimize (`CamundaOptimize`) is its own resource that names the cluster. You add or remove one without an edit to the cluster spec. [Architecture](docs/architecture.md) explains the rule.
- **It handles operations after the install.** Version upgrades that refuse a downgrade, suspend and resume, storage growth, password rotation, and restores, including a point-in-time restore of PostgreSQL.
- **Many clusters share one definition.** A preset holds the sizing and the defaults. A release pins the versions and the images. Each cluster then sets only its own references.
- **It is observable and signed.** The operator exports metrics and ships Grafana dashboards and Prometheus alert rules. Each release signs its images and its chart with cosign.

## What it runs

- Orchestration clusters (`CamundaCluster`): Zeebe brokers, the gateway, Operate, Tasklist, Admin, and optionally Connectors.
- The management plane (`CamundaManagementCluster`): Management Identity, Console, and Web Modeler.
- Optimize (`CamundaOptimize`), one for each cluster that needs it.
- Storage backends: Elasticsearch through ECK (`ElasticsearchCluster`), PostgreSQL through CloudNativePG (`DatabaseServer`), and logical databases on a PostgreSQL server (`Database`).
- Backup and restore: logical backups to a bucket, on demand or on a schedule, and restores into a suspended cluster.
- Shared settings: authentication and license (`CamundaPlatformConfig`), presets for sizing, and releases that pin versions and images.

The [CRD reference](docs/crds/index.md) lists every kind with every field.

## Examples

[`config/example`](config/example) holds complete setups that you can apply: a cluster on Elasticsearch, a cluster on PostgreSQL, and a management plane with Keycloak or with your own identity provider. Each directory has a README with the apply order.

With the shared presets and release of those examples in place, this is a complete cluster:

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

Check the [requirements](docs/installation.md#requirements) first. A few optional features need another operator, and only when you use them: ECK for an `ElasticsearchCluster`, CloudNativePG for a `DatabaseServer`, and the Keycloak Operator for a Keycloak that the operator runs. This operator does not install them.

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
