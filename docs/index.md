# Camunda Operator

The Camunda Operator is the core Kubernetes operator for the Camunda platform: it manages the lifecycle of Camunda 8.9+ orchestration clusters and everything that attaches to them — storage backends, backup and restore, Optimize, PVC auto-resizing, and the management plane.

It is the bottom layer of a three-operator stack. Cloud- and SaaS-level operators may sit above it and create this operator's custom resources, but this operator has zero knowledge of them and works standalone on any Kubernetes cluster.

## Where to go

- [Architecture](architecture.md) — the extension model, how features connect to the core, and the support policy.
- [Installation](installation.md) — installing the operator with Helm or plain manifests.
- [CRD Overview](crds/index.md) — the full inventory of custom resource definitions, their dependency graph, and per-CRD reference documentation.

## Installation

See the [installation guide](installation.md) for Helm and manifest-based installation, signature verification, and upgrades.
