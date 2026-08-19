# Camunda Operator

A Kubernetes operator that runs [Camunda 8.9+](https://docs.camunda.io/) orchestration clusters.
You describe a cluster in one resource. The operator creates the workloads, wires the storage, and keeps the cluster healthy.

The operator also manages what a cluster needs around it: an Elasticsearch cluster through ECK, a logical database on a PostgreSQL server you provide, and backups to a bucket.

## Start here

- [Getting started](getting-started.md): from an empty cluster to a running Camunda cluster you can log in to.
- [Installation](installation.md): Helm, plain manifests, signatures, upgrades.
- [Architecture](architecture.md): how the resources relate and the rules the operator follows.
- [Use the API types from Go](go-api.md): the Go module for programs that create or read the CRs.

## Guides

- [Secondary storage](guides/secondary-storage.md): Elasticsearch or PostgreSQL, and how each connects to a cluster.
- [Authentication](guides/authentication.md): basic authentication, OIDC, administrators.
- [Backup](guides/backup.md): set up a bucket and take backups.
- [Operations](guides/operations.md): status, suspend, storage growth, password rotation, monitoring.

## Reference

- [CRD reference](crds/index.md): every custom resource with every field, condition, and rule.

## Requirements

- Kubernetes 1.30 or later.
- The [ECK operator](https://www.elastic.co/guide/en/cloud-on-k8s/current/k8s-deploy-eck.html), if you use `ElasticsearchCluster`. Install ECK before you create one, then restart the operator.
- A PostgreSQL server, if you use `Database`.

The source is at [github.com/konsole-is/camunda-operator](https://github.com/konsole-is/camunda-operator).
