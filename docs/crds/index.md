# CRD reference

The operator defines the custom resources below in the API group `core.camunda.io/v1`.
Each page opens with what the kind is and a minimal manifest, then covers one topic per section, and ends with the status conditions and the full spec reference.

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
| [DatabaseServer](databaseserver.md) | Namespaced | A PostgreSQL server run by CloudNativePG, archived to a bucket, published as a `DatabaseServerConfig`. |
| [DatabaseServerPreset](databaseserverpreset.md) | Cluster | A baseline spec that database servers inherit. No controller. |
| [Database](database.md) | Namespaced | A logical database and its users on an existing PostgreSQL server, published as a `DatabaseConfig`. |

## Contracts

A contract carries connection details and credential references. The operator validates it and reports `Ready`. It provisions nothing from it. You can write a contract by hand or let a resource above write it.

| Kind | Scope | What it carries |
| --- | --- | --- |
| [SecondaryStorageConfig](secondarystorageconfig.md) | Namespaced | The secondary storage of a cluster: Elasticsearch or a relational database. |
| [ObjectStorageConfig](objectstorageconfig.md) | Cluster | One bucket on S3, GCS, or Azure Blob, and how to authenticate. |
| [DatabaseServerConfig](databaseserverconfig.md) | Namespaced | A database server, its admin credentials, and its point-in-time-recovery capability. |
| [DatabaseConfig](databaseconfig.md) | Namespaced | One logical database and its credentials. |
| [ManagementAuthConfig](managementauthconfig.md) | Cluster | The OIDC configuration of Management Identity. `CamundaOptimize` reads it. |

## Management

| Kind | Scope | What it is |
| --- | --- | --- |
| [CamundaManagementCluster](camundamanagementcluster.md) | Namespaced | The management plane: Management Identity, its identity provider, Console, and Web Modeler. |

## Analytics

| Kind | Scope | What it is |
| --- | --- | --- |
| [CamundaOptimize](camundaoptimize.md) | Namespaced | Camunda Optimize for one cluster: the webapp, the importer, and the exporter settings they need. |

## Backup

| Kind | Scope | What it is |
| --- | --- | --- |
| [LogicalBackupElasticsearch](logicalbackupelasticsearch.md) | Namespaced | One backup of a cluster on Elasticsearch. |
| [LogicalBackupRDBMS](logicalbackuprdbms.md) | Namespaced | One backup of a cluster on a relational database. |
| [BackupSchedule](backupschedule.md) | Namespaced | Create logical backups of a cluster on a cron schedule and prune the ones it created. |

## Restore

| Kind | Scope | What it is |
| --- | --- | --- |
| [LogicalRestoreElasticsearch](logicalrestoreelasticsearch.md) | Namespaced | Restore one Elasticsearch backup into one suspended cluster. |
| [LogicalRestoreRDBMS](logicalrestorerdbms.md) | Namespaced | Restore one relational logical backup into a suspended cluster. |
| [PointInTimeRestore](pointintimerestore.md) | Namespaced | Align the Zeebe primary storage of a PostgreSQL cluster with a database restored to a timestamp. |

## How the kinds relate

Solid arrows mean "creates". Dotted arrows mean "references".

```mermaid
graph LR
    CCP[CamundaClusterPreset]
    PFC[CamundaPlatformConfig]
    ESCP[ElasticsearchClusterPreset]
    ESC[ElasticsearchCluster]
    DBS[DatabaseServer]
    DBSP[DatabaseServerPreset]
    DB[Database]
    DBSC[DatabaseServerConfig]
    DBC[DatabaseConfig]
    SSC[SecondaryStorageConfig]
    OSC[ObjectStorageConfig]
    CC[CamundaCluster]
    LBE[LogicalBackupElasticsearch]
    LBR[LogicalBackupRDBMS]
    BS[BackupSchedule]
    LRE[LogicalRestoreElasticsearch]
    LRR[LogicalRestoreRDBMS]
    MAC[ManagementAuthConfig]
    OPT[CamundaOptimize]
    MC[CamundaManagementCluster]

    ESC -.->|presetRef| ESCP
    ESC -->|creates| SSC
    ESC -.->|snapshotStorageRef| OSC
    DBS -.->|presetRef| DBSP
    DBS -.->|archive.objectStorageRef| OSC
    DBS -->|creates| DBSC
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
    LRE -.->|backupRef| LBE
    LRE -.->|targetClusterRef| CC
    LRR -.->|backupRef| LBR
    LRR -.->|targetClusterRef| CC

    OPT -.->|clusterRef| CC
    OPT -.->|managementAuthRef| MAC

    MC -.->|platformConfigRef| PFC
    MC -.->|databaseConfigRef| DBC
    MC -.->|clusterSelector| CC
    MC -->|creates| MAC

    BS -.->|clusterRef| CC
    BS -->|creates| LBE
    BS -->|creates| LBR
```
