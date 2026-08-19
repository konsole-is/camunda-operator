# CRD reference

The operator defines the custom resources below in the API group `core.camunda.io/v1`.
Each page has the same sections: Purpose, What it does, Spec, Status, Validation, Related, Examples.

## Cluster

| Kind | Scope | What it is |
| --- | --- | --- |
| [CamundaCluster](camundacluster.md) | Namespaced | One orchestration cluster: Zeebe, gateway, web applications, connectors. |
| [CamundaPlatformConfig](camundaplatformconfig.md) | Cluster | Settings shared by all clusters: authentication, license, image registry. |
| [CamundaClusterPreset](camundaclusterpreset.md) | Cluster | A baseline spec that clusters inherit. No controller. |

## Storage backends

| Kind | Scope | What it is |
| --- | --- | --- |
| [ElasticsearchCluster](elasticsearchcluster.md) | Namespaced | An Elasticsearch cluster run by ECK, published as a `SecondaryStorageConfig`. |
| [ElasticsearchClusterPreset](elasticsearchclusterpreset.md) | Cluster | A baseline spec that Elasticsearch clusters inherit. No controller. |
| [Database](database.md) | Cluster | A logical database and its users on an existing PostgreSQL server, published as a `DatabaseConfig`. |

## Contracts

A contract carries connection details and credential references. The operator validates it and reports `Ready`. It provisions nothing from it. You can write a contract by hand or let a resource above write it.

| Kind | Scope | What it carries |
| --- | --- | --- |
| [SecondaryStorageConfig](secondarystorageconfig.md) | Namespaced | The secondary storage of a cluster: Elasticsearch or a relational database. |
| [ObjectStorageConfig](objectstorageconfig.md) | Cluster | One bucket on S3, GCS, or Azure Blob, and how to authenticate. |
| [DatabaseServerConfig](databaseserverconfig.md) | Cluster | A database server, its admin credentials, and its point-in-time-recovery capability. |
| [DatabaseConfig](databaseconfig.md) | Namespaced | One logical database and its credentials. |
| [ManagementAuthConfig](managementauthconfig.md) | Cluster | The OIDC configuration of Management Identity. The operator validates it, but nothing reads it yet. |

## Backup

| Kind | Scope | What it is |
| --- | --- | --- |
| [LogicalBackupElasticsearch](logicalbackupelasticsearch.md) | Namespaced | One backup of a cluster on Elasticsearch. |
| [LogicalBackupRDBMS](logicalbackuprdbms.md) | Namespaced | One backup of a cluster on a relational database. |

## How the kinds relate

Solid arrows mean "creates". Dotted arrows mean "references".

```mermaid
graph TD
    CCP[CamundaClusterPreset]
    PFC[CamundaPlatformConfig]
    ESCP[ElasticsearchClusterPreset]
    ESC[ElasticsearchCluster]
    DB[Database]
    DBSC[DatabaseServerConfig]
    DBC[DatabaseConfig]
    SSC[SecondaryStorageConfig]
    OSC[ObjectStorageConfig]
    CC[CamundaCluster]
    LBE[LogicalBackupElasticsearch]
    LBR[LogicalBackupRDBMS]

    ESC -.->|presetRef| ESCP
    ESC -->|creates| SSC
    ESC -.->|snapshotStorageRef| OSC
    DB -->|creates| DBC
    DB -->|"creates (optional)"| SSC
    DB -.->|serverRef| DBSC
    DBC -.->|serverRef| DBSC
    SSC -.->|databaseConfigRef| DBC

    CC -.->|presetRef| CCP
    CC -.->|platformConfigRef| PFC
    CC -.->|storageRef| SSC
    CC -.->|"backupStorageRef / documentStorageRef"| OSC

    LBE -.->|clusterRef| CC
    LBR -.->|clusterRef| CC
```

## Planned kinds

The CRDs `BackupSchedule`, `LogicalRestore`, `PointInTimeRestore`, `CamundaOptimize`, `CamundaManagementCluster`, and `PVCAutoResize` are installed with the operator but have no controller yet. Their spec is a placeholder. Do not create them. Their documentation follows when they are implemented.
