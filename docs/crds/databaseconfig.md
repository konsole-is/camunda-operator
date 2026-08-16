# DatabaseConfig

`DatabaseConfig` is the contract CRD that describes one logical database — its server, name, and application credentials — for the controllers and components that connect to it.

## Purpose

Several consumers need to connect to a specific logical database: an orchestration cluster using RDBMS secondary storage, the management plane's Keycloak, Identity, and Web Modeler databases, and the backup and restore controllers.
This namespaced contract CRD carries the logical database's coordinates and credentials, decoupling whoever created the database from whoever connects to it, as described in the [architecture](../architecture.md).
It lives in the consumer's namespace: consumers resolve it by name in their own namespace, and a producing [Database](database.md) creates it in its `targetNamespace`.

| Role | Who |
| --- | --- |
| Producers | [Database](database.md) (as output of bootstrapping a logical database, named by its `databaseConfig` output field), or you, by hand, for databases created outside the operator |
| Consumers | [SecondaryStorageConfig](secondarystorageconfig.md) (via `rdbms.databaseConfigRef`, same-namespace), [CamundaManagementCluster](camundamanagementcluster.md) (via `keycloakDbRef`, `identityDbRef`, and `webModelerDbRef`, resolved in its target namespace), and the backup and restore controllers, which resolve it through the consuming cluster's storage chain |

## How it works

The contract has a lightweight validation-only controller: it never provisions anything.

1. The operator watches every `DatabaseConfig`, the [DatabaseServerConfig](databaseserverconfig.md) it references, and the Secrets it references, and re-runs validation whenever any of them change.
2. It checks that the [DatabaseServerConfig](databaseserverconfig.md) named by `serverRef` exists.
3. It checks that the Secret named by `credentialsSecretRef` — and by `backupCredentialsSecretRef`, when set — exists and contains the configured `usernameKey` and `passwordKey`.
4. It sets the `Ready` condition: `Healthy` when all checks pass, `InvalidReference` or `MissingSecret` otherwise.

Consumers read the contract by name and never care who produced it.
Connection endpoints are deliberately not duplicated here: consumers resolve `serverRef` to the [DatabaseServerConfig](databaseserverconfig.md) for the engine, host, and port, then combine them with this contract's `databaseName` and credentials.

!!! note "Security posture: Secret references cross namespaces"
    `credentialsSecretRef` and `backupCredentialsSecretRef` carry an explicit namespace and may name a Secret in any namespace, and the validation controller reports precise existence and missing-key messages in status.
    Anyone permitted to create or update this kind can therefore learn whether an arbitrary Secret exists and which keys it lacks — an accepted existence oracle, unchanged from when the contract was cluster-scoped.
    Now that the kind is namespaced, namespace-level editors (not only cluster operators) may hold that permission; RBAC on this kind is the mitigation, so grant write access to it deliberately.

```mermaid
graph LR
    DB[Database] -->|creates| DBC[DatabaseConfig]
    DBC -.->|serverRef| DBSC[DatabaseServerConfig]
    DBC -.->|credentialsSecretRef| SEC[Secret]
    SSC["SecondaryStorageConfig (rdbms)"] -.->|databaseConfigRef| DBC
    CMC[CamundaManagementCluster] -.->|"keycloakDbRef / identityDbRef / webModelerDbRef"| DBC
```

## API reference

```yaml
apiVersion: core.camunda.io/v1
kind: DatabaseConfig
# Namespaced: consumers resolve this contract by name in their own namespace.
metadata:
  name: my-camunda-db
  namespace: my-cluster-ns
spec:
  # string. Required. Name of the DatabaseServerConfig describing the server hosting this database.
  serverRef: my-db-server
  # string. Required. Name of the logical database on the server.
  databaseName: camunda
  # object. Required. Application user with read/write access to the database.
  credentialsSecretRef:
    # string. Required. Name of the Secret holding the application credentials.
    name: my-camunda-db-credentials
    # string. Required. Namespace of the Secret (always explicit; it never defaults to this CR's namespace).
    namespace: my-cluster-ns
    # string. Required. Key in the Secret holding the plaintext username.
    usernameKey: username
    # string. Required. Key in the Secret holding the plaintext password.
    passwordKey: password
  # object. Optional. Separate user with dump/restore privileges, used by the backup and restore controllers.
  backupCredentialsSecretRef:
    # string. Required. Name of the Secret holding the backup credentials.
    name: my-camunda-db-backup-credentials
    # string. Required. Namespace of the Secret (always explicit; it never defaults to this CR's namespace).
    namespace: my-cluster-ns
    # string. Required. Key in the Secret holding the plaintext username.
    usernameKey: username
    # string. Required. Key in the Secret holding the plaintext password.
    passwordKey: password
```

## Status

| Type | Reason | Meaning |
| --- | --- | --- |
| `Ready` | `Healthy` | The referenced `DatabaseServerConfig` and all referenced Secrets exist with the required keys. |
| `Ready` | `InvalidReference` | The `DatabaseServerConfig` named by `serverRef` does not exist. |
| `Ready` | `MissingSecret` | A Secret named by `credentialsSecretRef` or `backupCredentialsSecretRef` is missing or lacks the configured keys. |

The operator records the last reconciled generation in `status.observedGeneration`.

## Validation

There are no admission rules beyond schema validation; reference and Secret existence are checked continuously by the validation controller and surfaced as conditions.

## Relationships

- [DatabaseServerConfig](databaseserverconfig.md) — referenced via `serverRef` for the server's engine, host, port, and admin credentials.
- [SecondaryStorageConfig](secondarystorageconfig.md) — references this contract via `rdbms.databaseConfigRef` when the secondary storage type is `rdbms`; the reference resolves in the referencing contract's own namespace.
- [Database](database.md) — creates this contract as output of bootstrapping the logical database and its users.
- [CamundaManagementCluster](camundamanagementcluster.md) — consumes this contract via `keycloakDbRef`, `identityDbRef`, and `webModelerDbRef` for its component databases.
- [Backup](backup.md), [LogicalRestore](logicalrestore.md) — resolve this contract through the target cluster's storage chain and use `backupCredentialsSecretRef` for dump and restore operations.
- [PointInTimeRestore](pointintimerestore.md) — resolves this contract through the target cluster's storage chain; its restore-app Jobs use `credentialsSecretRef` (the application credentials) to read the exporter position from the restored database.

See the [CRD overview](index.md) for where this contract sits in the reconciler dependency graph.

## Examples

A minimal manifest:

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
    namespace: my-cluster-ns
    usernameKey: username
    passwordKey: password
```

A realistic manifest with a dedicated backup user for dump and restore operations:

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
    namespace: my-cluster-ns
    usernameKey: username
    passwordKey: password
  backupCredentialsSecretRef:
    name: my-camunda-db-backup-credentials
    namespace: my-cluster-ns
    usernameKey: username
    passwordKey: password
```
