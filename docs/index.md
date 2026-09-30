# Camunda Operator

A Kubernetes operator that runs [Camunda 8.9+](https://docs.camunda.io/) orchestration clusters.
You describe a cluster in one resource. The operator creates the workloads, wires the storage, and keeps the cluster healthy.

The operator also runs what a cluster needs around it: Elasticsearch through ECK, PostgreSQL through CloudNativePG, and backups to a bucket.
It runs the management plane too: Management Identity, Console, and Web Modeler, over as many orchestration clusters as you give it. A `CamundaOptimize` runs Optimize for one cluster.

## Start here

- [Getting started](getting-started.md): from an empty cluster to a running Camunda cluster you can log in to.
- [Installation](installation.md): Helm, plain manifests, signatures, upgrades.
- [Architecture](architecture.md): how the resources relate and the rules the operator follows.
- [Observability](observability.md): the metrics of the operator, and the dashboards and alerts that ship with it.
- [Use the API types from Go](go-api.md): the Go module for programs that create or read the CRs.

## Guides

- [Presets](guides/presets.md): write the sizing and the defaults once, then create a cluster in a few lines.
- [Secondary storage](guides/secondary-storage.md): Elasticsearch or PostgreSQL, and how each connects to a cluster.
- [Authentication](guides/authentication.md): basic authentication, OIDC, administrators.
- [Management plane](guides/management-plane.md): Management Identity, Console, and Web Modeler over your clusters.
- [Backup](guides/backup.md): set up a bucket and take backups.
- [Operations](guides/operations.md): status, suspend, storage growth, password rotation, monitoring.

## Reference

- [CRD reference](crds/index.md): every custom resource with every field, condition, and rule.

## Requirements

Kubernetes 1.30 or later. A storage backend that the operator runs for you needs another operator, which this operator does not install. [Installation](installation.md#requirements) lists them.

The source is at [github.com/konsole-is/camunda-operator](https://github.com/konsole-is/camunda-operator).
