# Platform config and connections

## Connections

A [connection contract](../crds/index.md#contracts) tells a cluster where its storage and its identity provider are. The page **Setup > Connections** of the fleet manager lists the contracts of every namespace. For each contract, it shows who wrote it, which objects use it, and its status. The page shows the names and the keys of Secrets, never a value.

### The tabs

The page has one tab for each kind of contract.

| Tab | Kind | What the contract holds |
| --- | --- | --- |
| **Object storage** | [ObjectStorageConfig](../crds/objectstorageconfig.md) | A bucket for backups, documents, Elasticsearch snapshots, or the archive of a PostgreSQL server. |
| **Secondary storage** | [SecondaryStorageConfig](../crds/secondarystorageconfig.md) | The Elasticsearch or the relational database that a cluster writes its data to. |
| **Database servers** | [DatabaseServerConfig](../crds/databaseserverconfig.md) | A PostgreSQL server and its admin credentials. |
| **Databases** | [DatabaseConfig](../crds/databaseconfig.md) | One database on a database server, with its credentials. |
| **Management Identity** | [ManagementAuthConfig](../crds/managementauthconfig.md) | How Optimize signs in to the Management Identity of a management plane. This kind has no namespace. |

Each row shows the name, the namespace, the main settings of the kind, the writer, the number of users, and the status. Select the name to open the page of the contract. Add `?tab=` to the address to open a tab directly, for example `/setup/connections?tab=databases`.

The page of a contract shows the panels **Status**, **Writer**, **Settings**, and **Used by**. **Settings** shows each Secret reference with the key of each value that the contract reads. A username key or a password key that the contract does not set shows its default, `username` or `password`. For an Elasticsearch contract, **Settings** shows `nodeCount` as **Node count**, with the note `Sets the index replicas when the cluster sets no replica count.` The [SecondaryStorageConfig](../crds/secondarystorageconfig.md#node-count) reference gives the replica count for each node count. For a database server contract with `pitr`, **Settings** shows the retention days and who rolls the server back. The [DatabaseServerConfig](../crds/databaseserverconfig.md#point-in-time-recovery) reference describes both.

### Who wrote a contract

The column **Writer** names the writer of each contract. The page uses the first rule in this table that matches the contract.

| Order | The contract has | Writer |
| --- | --- | --- |
| 1 | The label `plugins.camunda.io/registration` | `Registration <registration>` |
| 2 | The label `camunda.io/management-cluster` | `Management plane <namespace>/<plane>` |
| 3 | An owner reference with `controller: true` | `<Kind> <name>` of the owner, for example `ElasticsearchCluster orders-es` |
| 4 | None of these | `Created by hand` |

A registration puts its own name and the name of the request in two labels on each contract that it writes. This contract comes from the registration `aws-s3` for the request `orders-backup`:

```yaml
apiVersion: core.camunda.io/v1
kind: ObjectStorageConfig
metadata:
  name: orders-backup
  namespace: my-cluster-ns
  labels:
    plugins.camunda.io/registration: aws-s3
    plugins.camunda.io/request: orders-backup
spec:
  type: S3
  # ... the rest of your contract
```

A contract without a namespace, such as a Management Identity contract, also gets the label `plugins.camunda.io/request-namespace`. It holds the namespace of the request.

When the request that `plugins.camunda.io/request` names does not exist any more, the writer is `Kept from <registration>`. This occurs when a claim is deleted with the deletion policy `Orphan` and the registration keeps the contract. If the fleet manager cannot read the requests, an error shows above the tabs. Until it can read them again, the page shows `Registration <registration>` for every contract with the label.

A contract that a registration, a management plane, or an owner wrote shows `Managed by <writer>. Change it there.` Change the contract at its writer. For a registration, change the claim that holds the request. For a management plane, change the plane. For an owner, change the owner. A contract created by hand or kept from a registration shows no such line, and you change it yourself.

The filter **Writer** above the table shows the contracts of one writer type. For example, pick **Created by hand** to see the contracts that no claim, plane, or owner manages.

### Who uses a contract

The panel **Used by** lists each object that names the contract, with the fields that name it, its environment, and its own status. A namespaced contract has users only in its own namespace. A Management Identity contract has users in every namespace.

| Contract | User | Field |
| --- | --- | --- |
| Object storage | `CamundaCluster` | `spec.backupStorageRef`, `spec.documentStorageRef` |
| Object storage | `CamundaClusterClaim` | `spec.cluster.documentStorageRef` |
| Object storage | `ElasticsearchCluster` | `spec.snapshotStorageRef` |
| Object storage | `DatabaseServer` | `spec.archive.objectStorageRef` |
| Object storage | `DatabaseServer` | `status.archive.boundary.objectStorageRef`, `status.archive.history.objectStorageRef`, `status.recovery.archive.objectStorageRef` |
| Object storage | `LogicalBackupElasticsearch` | `status.storage.bucketRef` |
| Object storage | `LogicalBackupRDBMS` | `status.bucketRef` |
| Secondary storage | `CamundaCluster` | `spec.storageRef` |
| Secondary storage | `LogicalBackupElasticsearch` | `status.storage.secondaryStorageConfig` |
| Secondary storage | `PointInTimeRestore` | `status.storage.secondaryStorageConfig` |
| Database servers | `Database`, `DatabaseConfig` | `spec.serverRef` |
| Database servers | `PointInTimeRestore` | `status.storage.databaseServerConfig` |
| Database servers | `DatabaseServer` | `status.recovery.contract`, until the server answers the rollback |
| Databases | `SecondaryStorageConfig` | `spec.rdbms.databaseConfigRef` |
| Databases | `PointInTimeRestore` | `status.storage.databaseConfig` |
| Databases | `CamundaManagementCluster` | `spec.identityProvider.keycloak.databaseConfigRef`, `spec.identity.databaseConfigRef`, `spec.webModeler.databaseConfigRef` |
| Management Identity | `CamundaOptimize` | `spec.managementAuthRef` |
| Management Identity | `CamundaClusterClaim` | `spec.attachments.optimize.managementAuthConfigRef` |

An `ElasticsearchCluster` or a `DatabaseServer` can take its reference from its preset. It is then a user of the contract too, and it shows `from preset <preset>` under its field.

A backup and the archive history of a database server keep their bucket in their status. They count as users, because a restore reads from that bucket. A database server that moved its archive to another bucket still uses the old bucket for as long as its history names it.

An Elasticsearch backup also keeps its secondary storage contract in its status. The deletion of the backup removes its snapshots through that contract, also after the cluster moved to another contract. A point-in-time restore keeps the contracts that it read. Each of these counts as a user.

If the fleet manager cannot read a kind that can use a contract, the column **Users** and the panel **Used by** name that kind. The count can then be too low.

### Status

The panel **Status** shows the reason and the message of the `Ready` condition of the contract. The operator writes this condition. The page does not test a connection or a bucket itself. A contract without `Ready` shows `No status yet`. The reference page of each kind says what the operator checks and lists the reasons.

Each user in **Used by** shows its own `Ready`. When the user is not ready, its message shows under its status. One cluster holds one secondary storage backend. A second cluster on the same contract shows in **Used by** with this status:

```yaml
status:
  conditions:
    - type: Ready
      status: "False"
      reason: StorageAlreadyAttached
      message: >-
        CamundaCluster "my-cluster-ns/my-other-cluster" already holds the
        backend "elasticsearch|https://es-http.my-cluster-ns.svc:9200". One
        CamundaCluster holds one backend, so this cluster stays suspended
        until that cluster moves to another backend or is deleted
```

The contract itself stays ready. The message names the cluster that holds the backend. [A second cluster on a held backend](../crds/camundacluster.md#a-second-cluster-on-a-held-backend) tells you what to do.
