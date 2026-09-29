# DatabaseConfig

`DatabaseConfig` is a namespaced [contract](index.md#contracts) kind that describes one logical database: its server, its name, and the application credentials. A `Database` creates it, or you create it by hand.

An orchestration cluster with a relational database as secondary storage connects to one logical database. This kind carries the name and the credentials of that database. The operator checks the references of the contract and reports the result on `Ready`. It creates nothing from it.

The contract is in the namespace of the consumer. A `SecondaryStorageConfig` finds it by name in its own namespace. The host and the port are not in this contract. Consumers read them from the `DatabaseServerConfig` that `serverRef` names in the same namespace.

| Role | Who |
| --- | --- |
| Producers | [Database](database.md) (after it creates the logical database, named by its `databaseConfig` field), or you, by hand, for a database created outside the operator |
| Consumers | [SecondaryStorageConfig](secondarystorageconfig.md) (through `rdbms.databaseConfigRef`), [CamundaCluster](camundacluster.md) (through its `storageRef` and that contract), [LogicalBackupRDBMS](logicalbackuprdbms.md) (through the same chain, with `backupCredentialsSecretRef`) |

The smallest contract names the server, the database, and the application credentials:

```yaml
apiVersion: core.camunda.io/v1
kind: DatabaseConfig
metadata:
  name: my-camunda-db
  namespace: my-cluster-ns
spec:
  serverRef: my-db-server
  databaseName: camunda
  credentialsSecretRef:
    name: my-camunda-db-credentials
    usernameKey: username
    passwordKey: password
```

```mermaid
graph LR
    DB[Database] --> DBC[DatabaseConfig]
    DBC -.->|serverRef| DBSC[DatabaseServerConfig]
    DBC -.->|credentialsSecretRef, backupCredentialsSecretRef| SEC[Secret]
    SSC["SecondaryStorageConfig (rdbms)"] -.->|rdbms.databaseConfigRef| DBC
    CC[CamundaCluster] -.->|through storageRef| DBC
    LBR[LogicalBackupRDBMS] -.->|through the cluster storageRef| DBC
```

## Validation checks

The operator checks these references, in the namespace of the contract:

- The [DatabaseServerConfig](databaseserverconfig.md) named in `serverRef` exists.
- The Secret in `credentialsSecretRef` exists and holds `usernameKey` and `passwordKey`.
- If `backupCredentialsSecretRef` is set, that Secret exists and holds its `usernameKey` and `passwordKey`.

If the `DatabaseServerConfig` is missing, `Ready` is `False` with reason `InvalidReference`. If a Secret or a key is missing, `Ready` is `False` with reason `MissingSecret`. The message names the missing object. The operator checks again when the contract, a referenced Secret, or the referenced `DatabaseServerConfig` changes.

## Backups

A `LogicalBackupRDBMS` dumps the database with the user in `backupCredentialsSecretRef`. If the field is not set, the backup reports `MissingSecret` and does not run. Set it on every database that you want to back up.

## Status

`kubectl get databaseconfig` shows `Ready`, its reason, the name of the logical database, and the age.

| Type | Reason | Meaning | What to do |
| --- | --- | --- | --- |
| `Ready` | `Healthy` | The `DatabaseServerConfig` and all referenced Secrets exist and hold the required keys. | Nothing. |
| `Ready` | `InvalidReference` | The `DatabaseServerConfig` named by `serverRef` does not exist in the namespace of this contract. | Create the `DatabaseServerConfig` in this namespace, or fix the name. |
| `Ready` | `MissingSecret` | A Secret named by `credentialsSecretRef` or `backupCredentialsSecretRef` is missing, or it lacks a configured key. | Create the Secret, or add the key. The message names the Secret and the key. |

`status.observedGeneration` is the last generation of the contract that the operator validated.

## Spec reference

Every field, with its type, whether it is required, and its default:

```yaml
apiVersion: core.camunda.io/v1
kind: DatabaseConfig
metadata:
  name: my-camunda-db
  namespace: my-cluster-ns
spec:
  # string. Required. Name of the DatabaseServerConfig of this namespace that describes the server of this database.
  serverRef: my-db-server
  # string. Required. Name of the logical database on the server.
  databaseName: camunda
  # object. Required. Application user with read and write access to the database.
  credentialsSecretRef:
    # string. Required. Name of the Secret that holds the application credentials, in the namespace of this contract.
    name: my-camunda-db-credentials
    # string. Optional, default: username. Key in the Secret that holds the username.
    usernameKey: username
    # string. Optional, default: password. Key in the Secret that holds the password.
    passwordKey: password
  # object. Optional. Separate user with dump and restore privileges. A LogicalBackupRDBMS needs it.
  backupCredentialsSecretRef:
    # string. Required. Name of the Secret that holds the backup credentials, in the namespace of this contract.
    name: my-camunda-db-backup-credentials
    # string. Optional, default: username. Key in the Secret that holds the username.
    usernameKey: username
    # string. Optional, default: password. Key in the Secret that holds the password.
    passwordKey: password
```

### Validation rules

- `spec.serverRef` and `spec.databaseName` must not be empty.
- Every field of a Secret reference must not be empty.
- No field is immutable.

### A production-shaped example

A contract with a separate backup user for database dumps:

```yaml
apiVersion: core.camunda.io/v1
kind: DatabaseConfig
metadata:
  name: my-camunda-db
  namespace: my-cluster-ns
spec:
  serverRef: my-db-server
  databaseName: camunda
  credentialsSecretRef:
    name: my-camunda-db-credentials
    usernameKey: username
    passwordKey: password
  backupCredentialsSecretRef:
    name: my-camunda-db-backup-credentials
    usernameKey: username
    passwordKey: password
```

## Related

- [DatabaseServerConfig](databaseserverconfig.md): the server of this database, named by `serverRef`. It carries the engine, the host, and the port.
- [Database](database.md): creates the logical database and its users, then creates this contract.
- [SecondaryStorageConfig](secondarystorageconfig.md): a `rdbms` contract names this kind through `rdbms.databaseConfigRef`.
- [CamundaCluster](camundacluster.md): connects to this database when its `storageRef` names a `rdbms` contract.
- [LogicalBackupRDBMS](logicalbackuprdbms.md): dumps this database with `backupCredentialsSecretRef`.
- [Secondary storage guide](../guides/secondary-storage.md): how to set up a relational database for a cluster.
- [Backup guide](../guides/backup.md): how to back up a relational cluster.
- [Getting started](../getting-started.md): the order in which you create the resources.
