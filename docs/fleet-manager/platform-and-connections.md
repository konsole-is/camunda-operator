# Platform config and connections

## Platform config

A platform config holds the authentication, the license, and the image repositories of the clusters, management planes, and database servers that name it. The page **Setup > Platform** of the fleet manager lists each `CamundaPlatformConfig` and edits it. The [CamundaPlatformConfig](../crds/camundaplatformconfig.md) reference lists the fields.

The list shows each platform config with its authentication method, the number of its users, and its status. Select a platform config to open its page.

The fleet manager does not create a platform config. Apply one with `kubectl apply` or your GitOps tool, for example:

```yaml
apiVersion: core.camunda.io/v1
kind: CamundaPlatformConfig
metadata:
  name: camundaplatformconfig-sample
spec:
  auth:
    method: oidc
    oidc:
      issuerUrl: "https://login.example.com/realms/camunda"
      clientId: "camunda-orchestration"
      audience: "camunda-orchestration"
      clientSecretRef:
        name: "oidc-credentials"
        namespace: "camunda-system"
        key: "client-secret"
  licenseSecretRef:
    name: "camunda-license"
    namespace: "camunda-system"
    key: "license-key"
  images:
    camunda: "registry.example.com/camunda/camunda"
    optimize: "registry.example.com/camunda/optimize"
```

If the fleet manager cannot read its settings, the page shows `Changes are off: the fleet manager could not read its settings.` and accepts no change.

### Users

The section **Used by** of the page lists each user of the platform config. The column **Users** of the list counts the same users.

| User | When it uses the platform config |
| --- | --- |
| Cluster | Its `spec.platformConfigRef` names the platform config. |
| Management plane | Its `spec.platformConfigRef` names the platform config. |
| Database server | Its own `spec.platformConfigRef` names the platform config, or its preset names it and the server sets no other one. The page then shows the preset, as in `team-a/orders-db from the preset pg-standard`. |
| Claim | Its `spec.platformConfigRef` names the platform config. The cluster or management plane of the claim has the same namespace and name. It does not name the platform config yet, for example because it does not exist yet. When the cluster names the platform config too, the pair counts once, as the cluster. |
| Environment | Its `spec.platformConfigRef` names the platform config, so each claim of the environment without its own platform config takes it. The environment counts as one user. |

### Change a platform config

The page edits the authentication method, the OIDC fields, the management plane clients, the license Secret, and the image repositories. The management plane clients are the OIDC clients of Management Identity, Optimize, Web Modeler, and Console, as [The clients of the management plane](../crds/camundaplatformconfig.md#the-clients-of-the-management-plane) describes. The field **PostgreSQL** of the image repositories is the image of the database servers. A note above the form says what a change does:

`A change rolls the pods of every cluster and management plane that uses this config, and of the Optimize of each such cluster. A database server changes only when the postgres image changes.`

Each Secret reference names a namespace, a Secret, and a key. The page shows the names only, never a Secret value. A reference can name a Secret in any namespace.

When you change a field, a bar at the bottom of the page offers **Review**. The review shows the change and a dry run of it. Select **Save** to write it. In Git mode, the button reads **Push to Git**, and the page shows `Waiting for GitOps to apply` until your GitOps tool applies the commit. [Fleet manager settings](settings.md#git-mode) says how to turn on Git mode.

To turn off OIDC, select **Basic**. The save sends the basic method and no OIDC block. When another tool, such as `kubectl apply` or your GitOps tool, set the OIDC block, the Kubernetes API keeps that block. A platform config with the basic method and an OIDC block is not valid, so the dry run fails, and the review shows the error. Then use that tool to set the method to basic and to remove the OIDC block.

When a referenced Secret or key is missing, the platform config is not ready, and the page shows the message of its `Ready` condition. The message starts with the spec path of the reference:

```yaml
status:
  conditions:
  - type: Ready
    status: "False"
    reason: MissingSecret
    message: "spec.licenseSecretRef: Secret camunda-system/camunda-license not found"
```

The Secret stays picked and shows as `camunda-license (not found)`. A key that the Secret does not hold shows the same way, as `license-key (not found)`. A save keeps the reference. Create the Secret with the named key, or pick another one. When the fleet manager cannot read the Secrets of a namespace, the picker shows the read error. It then marks no Secret or key as not found.

### Default images

The section **Image repositories** shows the default repository under each field. A field that you leave empty uses its default. If another tool set the value of that field, the save keeps it, and the review names the field that stays. The page shows repositories only, never a tag. The tag always comes from the version of the cluster, management plane, or database server that runs the image.

The defaults come from the images table of the operator, for each Camunda minor that the clusters and management planes of the platform config run. When they run more than one minor, each default shows the minors that it covers. For example, from Camunda 8.10 the Web Modeler images move to `camunda/hub` and `camunda/hub-websockets`:

```text
camunda/web-modeler-restapi for Camunda 8.9
camunda/hub for Camunda 8.10
```

A database server runs a PostgreSQL version, not a Camunda minor, so it adds no minor. When no cluster or management plane uses the platform config, the page shows the defaults of the newest minor of the images table. The minors of the users can be incomplete while the fleet manager cannot read a kind that gives them. A warning then names that kind, such as `CamundaCluster`. When the images table has no defaults for a minor that a user runs, a warning above the fields names that minor. The fields still show the defaults of the other minors.

### Delete a platform config

The page offers the delete only when nothing uses the platform config. To delete, type the name of the platform config and select **Review the delete**. The review says:

`Users of a deleted config report InvalidReference and keep running.`

Select **Delete the platform config** to delete it. In Git mode, the button reads **Push the delete to Git**, and the commit removes the file of the platform config. Your GitOps tool must prune to delete it from the cluster. Nothing uses the platform config when you delete it. The warning of the review is for a cluster or claim that names it later. For example, someone applies an old manifest after the delete. Such a user reports `InvalidReference`. The [Deletion](../crds/camundaplatformconfig.md#deletion) section of the reference says more.

The fleet manager must read every kind that can use a platform config to know that nothing uses it. For example, it needs permission to list database servers. When it cannot read one of these kinds, the page offers no delete, and an open delete review closes. The section **Used by** and the column **Users** of the list then say that the users are not known. The page names the kinds that it cannot read:

> Delete is possible when the fleet manager can read DatabaseServer.

The page also shows this line for a short time after it opens or connects again.

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
| Database servers | `Database`, `DatabaseConfig` | `spec.serverRef` |
| Database servers | `PointInTimeRestore` | `status.storage.databaseServerConfig`, until the restore is `Completed` or `Failed` |
| Database servers | `DatabaseServer` | `status.recovery.contract`, until the server answers the rollback |
| Databases | `SecondaryStorageConfig` | `spec.rdbms.databaseConfigRef` |
| Databases | `CamundaManagementCluster` | `spec.identityProvider.keycloak.databaseConfigRef`, `spec.identity.databaseConfigRef`, `spec.webModeler.databaseConfigRef` |
| Management Identity | `CamundaOptimize` | `spec.managementAuthRef` |
| Management Identity | `CamundaClusterClaim` | `spec.attachments.optimize.managementAuthConfigRef` |
| Object storage, secondary storage | `CamundaClusterClaim` | `spec.plugins.<registration>.parameters.name`, for the registrations `existing-object-storage` and `existing-secondary-storage` |
| Databases | `CamundaManagementClusterClaim` | `spec.plugins.existing-database-configs.parameters.<component>`, for example `parameters.keycloak` |
| Object storage, secondary storage, databases | `PluginRequest` | `spec.parameters.name` or `spec.parameters.<component>` of a request of these registrations |

An `ElasticsearchCluster` or a `DatabaseServer` can take its reference from its preset. It is then a user of the contract too, and it shows `from preset <preset>` under its field.

A backup and the archive history of a database server keep their bucket in their status. They count as users, because a restore reads from that bucket. A database server that moved its archive to another bucket still uses the old bucket for as long as its history names it.

An Elasticsearch backup also keeps its secondary storage contract in its status. The deletion of the backup removes its snapshots through that contract, also after the cluster moved to another contract. A point-in-time restore keeps its database server contract in its status, and uses it until the restore is `Completed` or `Failed`. Each of these counts as a user.

A claim that selects a contract through `existing-object-storage`, `existing-secondary-storage`, or `existing-database-configs` is a user too, also before its cluster or plane exists. The claim can name the contract in its own parameters, or it can get the name from the policy of its Environment. This claim uses the bucket contract `backups` of its namespace:

```yaml
apiVersion: fleet.camunda.io/v1alpha1
kind: CamundaClusterClaim
metadata:
  name: my-cluster
  namespace: my-cluster-ns
spec:
  plugins:
    - section: backup-storage
      registration: existing-object-storage
      parameters:
        name: backups
  # ... the rest of your claim
```

If the fleet manager cannot read a kind that can use a contract, the column **Users** names that kind. The count can then be too low. On the page of the contract, **Used by** says that the users are not known, and it names the kind. The page of a contract also says this for a short time after it opens or connects again. A claim gets its contract names from its Environment, its namespace, and the registrations. So `Environment`, `Namespace`, `PluginRegistration`, and `PluginRequest` count as kinds that can use a contract too.

### Change a contract

You can change a contract that is `Created by hand` or `Kept from <registration>`. On its page, the panel **Settings** is a form. The page of any other contract shows **Settings** as text, with no form and no delete. A contract that is `Kept from <registration>` goes back to the registration when the claim that held its request is created again. The values of the registration then replace your change.

The form edits the values of the contract. For example, you can change a bucket, a region, an endpoint, the credentials, or the host and port of a server. It shows the storage type and the auth type as text, and a save keeps them. To change a type, apply the contract again with `kubectl` or with your GitOps tool.

Each Secret reference names a Secret of the namespace of the contract and the key of each value. A Management Identity contract has no namespace, so its client secret also names the namespace of the Secret. A username key or a password key that you leave at **Default** is left out, and the operator reads `username` or `password`. The page shows names and keys only, never a Secret value. A Secret or key that does not exist stays picked and shows as `<name> (not found)`. When the fleet manager cannot read the Secrets of the namespace, the picker shows the read error and marks nothing as not found.

When you change a field, a bar at the bottom of the page offers **Review**. The bar names the first required field that is empty, and **Review** stays off until you fill it. The review shows a dry run of the change. It also names each user of the contract and what the user does after the save. For example:

`CamundaCluster my-cluster-ns/my-cluster restarts its pods, all but Connectors.`

A cluster, its Optimize, and a management plane restart their pods on any change of a contract that they use. When you point a database server contract at another server, the operator creates each database and its roles on the new server. It does not remove them from the old server. A backup that runs fails when the location of its bucket changes.

While the fleet manager cannot read a kind that can use the contract, the form shows the values, but **Review** stays off. An open review closes. The page names the kinds that it cannot read:

> A change is possible when the fleet manager can read LogicalBackupRDBMS.

The page shows this line for a short time after it opens or connects again. If the line stays, the error above the page gives the cause, as for a delete below.

Select **Save** to write the change. The users react to it without a further step. In Git mode, the button reads **Push to Git**. The save keeps the labels and the annotations of the contract. A database server contract can hold `spec.recovery` and `spec.pitr.lastRecovery`. The operator writes these two fields, and the save keeps them.

The save applies only if the contract did not change after the page read it. When a person or a tool changed it in the meantime, the save writes nothing, and the review shows this error:

`The ObjectStorageConfig backups changed since you opened it.`

Go back, and make the change again on the new copy.

When you clear a field that another tool set, such as `kubectl apply` or your GitOps tool, the Kubernetes API keeps the old value. The review names each such field, for example `spec.s3.region stays, because another field manager set it.` Remove the field with that tool.

### Delete a contract

The page offers the delete only for a contract that you can change and that nothing uses. **Used by** counts the backups and the archive records of database servers, so a bucket that holds data that a restore needs is not offered. To delete, type the name of the contract and select **Review the delete**. Then select **Delete the contract**. In Git mode, the button reads **Push the delete to Git**, and the commit removes the file of the contract. Your GitOps tool must prune the contract to delete it from the cluster.

When a user appears while the review is open, the review closes. The fleet manager checks the users again before the delete, and it refuses the delete of a contract in use. A user that is created at the same moment as the delete can be missed.

The fleet manager must read every kind that can use a contract to know that nothing uses it. When it cannot read one of these kinds, the page offers no delete, and an open delete review closes. The page names the kinds that it cannot read:

> Delete is possible when the fleet manager can read CamundaManagementCluster.

The page shows this line for a short time after it opens or connects again. If the line stays, the error above the page gives the cause, for example a missing permission to list that kind. Give the fleet manager that permission, then open the page again.

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
