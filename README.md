# Camunda Operator

Core Kubernetes operator for the Camunda platform. Manages the orchestration
cluster lifecycle, storage backends, backup and restore, Optimize, and the
management plane.

It is the bottom layer of a three-operator stack: cloud- and SaaS-level
operators may create this operator's custom resources, but this operator has no
knowledge of them and runs standalone on any Kubernetes cluster.

## Install

```bash
helm install camunda-operator \
  oci://ghcr.io/konsole-is/charts/camunda-operator \
  --version <version> \
  --namespace camunda-operator-system \
  --create-namespace
```

Requires Kubernetes 1.30+. `ElasticsearchCluster` resources also require the
[Elastic Cloud on Kubernetes (ECK)](https://www.elastic.co/guide/en/cloud-on-k8s/current/index.html)
operator, which this operator does not install. For plain-manifest
installation, signature verification, out-of-band CRD installation, and
upgrades, see the [installation guide](docs/installation.md).

## Documentation

- [Installation](docs/installation.md) — Helm and manifest installation.
- [Architecture](docs/architecture.md) — the extension model and how features
  attach to workloads.
- [CRD reference](docs/crds/index.md) — every custom resource definition.

## Development

```bash
make test           # run unit and envtest suites
make lint           # run golangci-lint
make all            # generate manifests/deepcopy, fmt, vet, and build the manager binary
make helm-generate  # regenerate dist/chart/ from config/
make helm-verify    # lint and render the chart, no cluster needed
make test-e2e       # kind cluster with the real ECK operator and PostgreSQL
```

`make test` needs Docker: the Database specs start PostgreSQL in a
testcontainer. `make test-e2e` needs Docker and `kind`.

The Helm chart is generated, not checked in — only `dist/chart/Chart.yaml` and
`dist/chart/README.md` are versioned. Never hand-edit `dist/chart/values.yaml`
or `dist/chart/templates/`; regeneration overwrites them. Change defaults in
`config/` instead.
