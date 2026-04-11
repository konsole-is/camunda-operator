# Kubebuilder Bootstrap Design

**Date:** 2026-04-11
**Status:** Approved

---

## Overview

Bootstrap `camunda-operator` as a kubebuilder v4 project. The result is a compilable, runnable operator
skeleton with all 19 CRDs typed and 17 of them wired to controller stubs — ready for incremental
implementation without further scaffolding.

---

## Init

```
kubebuilder init \
  --domain camunda.io \
  --repo github.com/camunda/camunda-operator
```

- **Domain:** `camunda.io`
- **Group:** `core` → full API group `core.camunda.io`
- **Version:** `v1`
- **Module:** `github.com/camunda/camunda-operator`
- **Toolchain:** kubebuilder v4.13.1, Go 1.26.1

All CRDs use `apiVersion: core.camunda.io/v1`, which mirrors the convention used by
`camunda-cloud-operator` (`cloud.camunda.io`) at the layer above.

---

## CRD Inventory

### Active CRDs — type + controller stub (17)

Controllers watch the CRD, validate cross-resource references, and set status conditions.

| Kind | Scope | Controller purpose |
|---|---|---|
| CamundaPlatformConfig | Cluster | Validate `clientSecretRef`, `licenseSecretRef` |
| CamundaCluster | Namespace | Core orchestration reconciliation loop |
| ElasticsearchCluster | Namespace | Elasticsearch lifecycle management |
| Database | Cluster | Logical database and user bootstrapping |
| DatabaseServerConfig | Cluster | Validate credential secret references |
| DatabaseConfig | Cluster | Validate `serverRef` → DatabaseServerConfig exists |
| SecondaryStorageConfig | Cluster | Validate credential refs and optional `databaseConfigRef` |
| ObjectStorageConfig | Cluster | Validate credential secret references |
| Backup | Namespace | Backup trigger and status tracking |
| BackupSchedule | Namespace | Scheduled backup management |
| BackupRetention | Namespace | Retention policy enforcement |
| PointInTimeRestore | Namespace | Point-in-time restore operation |
| LogicalRestore | Namespace | Logical restore operation |
| CamundaOptimize | Namespace | Optimize deployment lifecycle |
| CamundaManagementCluster | Cluster | Management plane (Console, Web Modeler, Identity) |
| ManagementAuthConfig | Cluster | Validate OIDC secret references |
| PVCAutoResize | Namespace | PVC auto-resize annotation management |

### Passive CRDs — type only, no controller (2)

Pure data CRDs referenced by other CRDs. No cross-resource references to validate; no own
reconciliation loop.

| Kind | Scope | Referenced by |
|---|---|---|
| CamundaClusterPreset | Cluster | CamundaCluster via `presetRef` |
| ElasticsearchClusterPreset | Cluster | ElasticsearchCluster via `presetRef` |

---

## Scaffolding Sequence

`kubebuilder create api` is run once per CRD. The `--resource` flag creates the type file;
`--controller` adds the reconciler stub.

```bash
# Init
kubebuilder init --domain camunda.io --repo github.com/camunda/camunda-operator

# Active CRDs (type + controller)
kubebuilder create api --group core --version v1 --kind CamundaPlatformConfig --namespaced=false --resource --controller
kubebuilder create api --group core --version v1 --kind CamundaCluster --resource --controller
kubebuilder create api --group core --version v1 --kind ElasticsearchCluster --resource --controller
kubebuilder create api --group core --version v1 --kind Database --namespaced=false --resource --controller
kubebuilder create api --group core --version v1 --kind DatabaseServerConfig --namespaced=false --resource --controller
kubebuilder create api --group core --version v1 --kind DatabaseConfig --namespaced=false --resource --controller
kubebuilder create api --group core --version v1 --kind SecondaryStorageConfig --namespaced=false --resource --controller
kubebuilder create api --group core --version v1 --kind ObjectStorageConfig --namespaced=false --resource --controller
kubebuilder create api --group core --version v1 --kind Backup --resource --controller
kubebuilder create api --group core --version v1 --kind BackupSchedule --resource --controller
kubebuilder create api --group core --version v1 --kind BackupRetention --resource --controller
kubebuilder create api --group core --version v1 --kind PointInTimeRestore --resource --controller
kubebuilder create api --group core --version v1 --kind LogicalRestore --resource --controller
kubebuilder create api --group core --version v1 --kind CamundaOptimize --resource --controller
kubebuilder create api --group core --version v1 --kind CamundaManagementCluster --namespaced=false --resource --controller
kubebuilder create api --group core --version v1 --kind ManagementAuthConfig --namespaced=false --resource --controller
kubebuilder create api --group core --version v1 --kind PVCAutoResize --resource --controller

# Passive CRDs (type only)
kubebuilder create api --group core --version v1 --kind CamundaClusterPreset --namespaced=false --resource --controller=false
kubebuilder create api --group core --version v1 --kind ElasticsearchClusterPreset --namespaced=false --resource --controller=false
```

---

## Resulting Project Structure

```
camunda-operator/
├── api/
│   └── v1/
│       ├── camundaplatformconfig_types.go
│       ├── camundacluster_types.go
│       ├── camundaclusterpreset_types.go
│       ├── elasticsearchcluster_types.go
│       ├── elasticsearchclusterpreset_types.go
│       ├── database_types.go
│       ├── databaseserverconfig_types.go
│       ├── databaseconfig_types.go
│       ├── secondarystorageconfig_types.go
│       ├── objectstorageconfig_types.go
│       ├── backup_types.go
│       ├── backupschedule_types.go
│       ├── backupretention_types.go
│       ├── pointintimerestore_types.go
│       ├── logicalrestore_types.go
│       ├── camundaoptimize_types.go
│       ├── camundamanagementcluster_types.go
│       ├── managementauthconfig_types.go
│       ├── pvcautoresize_types.go
│       └── groupversion_info.go
├── internal/
│   └── controller/
│       ├── camundaplatformconfig_controller.go
│       ├── camundacluster_controller.go
│       ├── elasticsearchcluster_controller.go
│       ├── database_controller.go
│       ├── databaseserverconfig_controller.go
│       ├── databaseconfig_controller.go
│       ├── secondarystorageconfig_controller.go
│       ├── objectstorageconfig_controller.go
│       ├── backup_controller.go
│       ├── backupschedule_controller.go
│       ├── backupretention_controller.go
│       ├── pointintimerestore_controller.go
│       ├── logicalrestore_controller.go
│       ├── camundaoptimize_controller.go
│       ├── camundamanagementcluster_controller.go
│       ├── managementauthconfig_controller.go
│       └── pvcautoresize_controller.go
├── config/
│   ├── crd/
│   ├── rbac/
│   ├── manager/
│   └── default/
├── cmd/
│   └── main.go
└── Makefile
```

---

## Constraints

- **No legacy types.** No ZeebeCluster, no migration layer. This is a clean slate.
- **No cloud infrastructure.** IAM, KMS, buckets — those belong in `camunda-cloud-operator`.
- **Contract CRDs are typed interfaces.** `DatabaseServerConfig`, `DatabaseConfig`,
  `SecondaryStorageConfig`, `ObjectStorageConfig`, and `ManagementAuthConfig` are written by
  other controllers (cloud operator or peer controllers in this operator). Their controllers here
  exist only to validate references and surface status conditions — not to provision resources.
