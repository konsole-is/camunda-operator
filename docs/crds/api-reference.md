# API Reference

## Packages
- [core.camunda.io/v1](#corecamundaiov1)


## core.camunda.io/v1

Package v1 contains API Schema definitions for the core v1 API group.

### Resource Types
- [BackupSchedule](#backupschedule)
- [CamundaCluster](#camundacluster)
- [CamundaClusterPreset](#camundaclusterpreset)
- [CamundaManagementCluster](#camundamanagementcluster)
- [CamundaOptimize](#camundaoptimize)
- [CamundaPlatformConfig](#camundaplatformconfig)
- [CamundaRelease](#camundarelease)
- [Database](#database)
- [DatabaseConfig](#databaseconfig)
- [DatabaseServer](#databaseserver)
- [DatabaseServerConfig](#databaseserverconfig)
- [DatabaseServerPreset](#databaseserverpreset)
- [ElasticsearchCluster](#elasticsearchcluster)
- [ElasticsearchClusterPreset](#elasticsearchclusterpreset)
- [LogicalBackupElasticsearch](#logicalbackupelasticsearch)
- [LogicalBackupRDBMS](#logicalbackuprdbms)
- [LogicalRestoreElasticsearch](#logicalrestoreelasticsearch)
- [LogicalRestoreRDBMS](#logicalrestorerdbms)
- [ManagementAuthConfig](#managementauthconfig)
- [ObjectStorageConfig](#objectstorageconfig)
- [PointInTimeRestore](#pointintimerestore)
- [SecondaryStorageConfig](#secondarystorageconfig)



#### AdminMappingRule



AdminMappingRule gives the admin role to every token in which the claim
ClaimName holds the value ClaimValue.



_Appears in:_
- [ClusterAdminSpec](#clusteradminspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `id` _string_ | ID names the rule inside the cluster. The Admin web application lists it<br />under Mapping Rules. |  | MaxLength: 256 <br />MinLength: 1 <br /> |
| `claimName` _string_ | ClaimName is the name of a claim, or a JSONPath expression that points<br />at a claim. |  | MinLength: 1 <br /> |
| `claimValue` _string_ | ClaimValue is the value that the claim must hold for the rule to match. |  | MinLength: 1 <br /> |


#### AdminPasswordStatus



AdminPasswordStatus is the state of the admin credential of a basic-auth
cluster.



_Appears in:_
- [CamundaClusterStatus](#camundaclusterstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `rotation` _string_ | Rotation is the last admin password rotation that the operator<br />applied. It is the value of spec.auth.basic.passwordRotation, after the<br />preset merge, that produced the password in the admin Secret. A<br />rotation is in progress while that value is not empty and differs from<br />this one. A cleared value does not stop a rotation that has started.<br />That rotation completes, and this field then records the value that<br />started it. A cluster that gets the value from its preset shows none in<br />its own spec. |  | Optional: \{\} <br /> |


#### ArchiveBoundary



ArchiveBoundary is the moment when a server moved its archive to another
location while no interval was open. Only a base backup that began after
this moment belongs to the current archive. A base backup that began
before it wrote to the old location.



_Appears in:_
- [DatabaseServerArchiveStatus](#databaseserverarchivestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `at` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | At is when the server moved. |  |  |
| `location` _string_ | Location is the new location, in the form of ArchiveRecord.Location.<br />The boundary ends when the archive of that location opens a record. It<br />moves when the location moves again. |  |  |
| `objectStorageRef` _string_ | ObjectStorageRef is the ObjectStorageConfig of that location. It is<br />information for the reader only. |  | Optional: \{\} <br /> |


#### ArchiveRecord



ArchiveRecord is one continuous archive that a server wrote. A recovery
replays it from a base backup up to the requested point. A restore can
reach only a point inside the interval of a record.



_Appears in:_
- [DatabaseServerArchiveStatus](#databaseserverarchivestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serverName` _string_ | ServerName is the archive directory in the bucket, equal to the name of<br />the CloudNativePG cluster that wrote it. |  |  |
| `objectStorageRef` _string_ | ObjectStorageRef is the ObjectStorageConfig, in the namespace of this<br />server, that holds this archive. When the server moves to another<br />bucket, it closes this record and opens a new one. Thus every interval<br />names the bucket that a restore of that interval reads. |  | Optional: \{\} <br /> |
| `location` _string_ | Location is where in object storage the server wrote this archive: the<br />provider, the bucket, and the path, as one URL. Two intervals hold the<br />same archive only when their locations are equal. The name of an<br />ObjectStorageConfig is not sufficient, because you can edit the object<br />or create it again with the same name. A record without a location gets<br />the current location of the server. |  | Optional: \{\} <br /> |
| `from` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | From is the earliest point that a recovery from this archive can reach:<br />the time when its first base backup completed. |  |  |
| `to` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | To is the latest point that a recovery from this archive can reach. It<br />is unset while the server still writes to this archive. |  | Optional: \{\} <br /> |
| `unverifiedFrom` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | UnverifiedFrom is the point after which this archive can miss parts of<br />the write-ahead log. It is set while CloudNativePG reports that the<br />uploads of the server fail. A restore cannot reach a point after it.<br />When the uploads work again, the plugin uploads the segments that it<br />held back. The field is then cleared, and a restore can reach the whole<br />interval again. Only the record that the server writes to now has it. |  | Optional: \{\} <br /> |


#### AttachedClusterStatus



AttachedClusterStatus is one CamundaCluster that clusterSelector matched.



_Appears in:_
- [CamundaManagementClusterStatus](#camundamanagementclusterstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the CamundaCluster. |  |  |
| `namespace` _string_ | Namespace is the namespace of the CamundaCluster. |  |  |
| `attached` _boolean_ | Attached shows whether the management plane serves this cluster.<br />Console lists the cluster, and Web Modeler deploys to it, only while<br />this is true. |  |  |
| `reason` _string_ | Reason names what the management plane found on this cluster. It has<br />one of five values. Four values tell why the cluster is not attached.<br />ClaimedElsewhere means that another management plane holds the<br />cluster. NotReady means that the cluster publishes no gateway endpoints,<br />or that it changed while the operator claimed it. InvalidReference<br />means that the operator cannot read the platform config of the cluster.<br />It also means that the OIDC issuer of the cluster differs from the<br />issuer of the management plane. WriteFailed means that the API server refused the<br />write of the Console ping settings. The fifth value,<br />BasicAuthUserFailed, is on an attached row.<br />The management plane serves the cluster, but the Web Modeler user on<br />it is missing. |  | Optional: \{\} <br /> |
| `message` _string_ | Message explains the reason in one sentence. |  | Optional: \{\} <br /> |


#### AttachedOptimizeStatus



AttachedOptimizeStatus is one CamundaOptimize whose login callback this
management plane registers.



_Appears in:_
- [CamundaManagementClusterStatus](#camundamanagementclusterstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the CamundaOptimize. |  |  |
| `namespace` _string_ | Namespace is the namespace of the CamundaOptimize. |  |  |
| `externalUrl` _string_ | ExternalURL is spec.externalUrl of that CamundaOptimize. The registered<br />callback is this URL with the login path of Optimize. |  |  |


#### AuthenticationMethod

_Underlying type:_ _string_

AuthenticationMethod selects how users and clients authenticate against an
orchestration cluster.

_Validation:_
- Enum: [basic oidc]

_Appears in:_
- [PlatformAuthSpec](#platformauthspec)

| Field | Description |
| --- | --- |
| `basic` | AuthenticationMethodBasic selects username and password authentication<br />against users that the orchestration cluster stores itself.<br /> |
| `oidc` | AuthenticationMethodOIDC selects an external OIDC identity provider.<br /> |


#### AzureBlobCredentials



AzureBlobCredentials holds the static key of an Azure storage account.



_Appears in:_
- [AzureBlobStorageAuth](#azureblobstorageauth)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretRef` _[LocalSecretKeyRef](#localsecretkeyref)_ | SecretRef names the Secret key that holds the storage account key. |  |  |


#### AzureBlobStorage



AzureBlobStorage describes an Azure Blob Storage container.



_Appears in:_
- [ObjectStorageConfigSpec](#objectstorageconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `accountName` _string_ | AccountName is the storage account that holds the container. |  | MinLength: 1 <br /> |
| `container` _string_ | Container is the blob container that consumers write to. |  | MinLength: 1 <br /> |
| `basePath` _string_ | BasePath is the blob prefix under which consumers write objects,<br />without leading or trailing slashes. Empty means the container root. |  | Pattern: `^[^/]+(/[^/]+)*$` <br />Optional: \{\} <br /> |
| `endpoint` _string_ | Endpoint is the URL of the blob service. Empty means the public Azure<br />endpoint of the account. Set it for Azurite and sovereign clouds. |  | Optional: \{\} <br /> |
| `auth` _[AzureBlobStorageAuth](#azureblobstorageauth)_ | Auth selects how consumers authenticate. An absent block means<br />workload identity through the ServiceAccount. | \{ type:workloadIdentity \} | Optional: \{\} <br /> |


#### AzureBlobStorageAuth



AzureBlobStorageAuth selects how consumers authenticate against an Azure
Blob container.



_Appears in:_
- [AzureBlobStorage](#azureblobstorage)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ObjectStorageAuthType](#objectstorageauthtype)_ | Type is the authentication choice. Defaults to workloadIdentity. | workloadIdentity | Enum: [workloadIdentity credentials] <br />Optional: \{\} <br /> |
| `workloadIdentity` _[AzureBlobWorkloadIdentity](#azureblobworkloadidentity)_ | WorkloadIdentity names the trusted principal. It is valid only with<br />type workloadIdentity. With an empty or absent block, the operator adds<br />no annotation to the ServiceAccount. The operator still adds the<br />azure.workload.identity/use label to the pods. |  | Optional: \{\} <br /> |
| `credentials` _[AzureBlobCredentials](#azureblobcredentials)_ | Credentials is a static storage account key. Required with type<br />credentials. Forbidden with other types. |  | Optional: \{\} <br /> |


#### AzureBlobWorkloadIdentity



AzureBlobWorkloadIdentity names the Azure principal that the container
trusts. With an empty block, the operator adds no annotation to the
ServiceAccount of the consumer. The operator still adds the
azure.workload.identity/use label to the pods of the consumer.



_Appears in:_
- [AzureBlobStorageAuth](#azureblobstorageauth)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clientId` _string_ | ClientID is the managed identity that consumers use. When set, the<br />operator puts it in the azure.workload.identity/client-id annotation<br />of a ServiceAccount that it creates for the consumer. On an existing<br />ServiceAccount, add the annotation yourself. |  | Optional: \{\} <br /> |


#### BackupCredentialsSpec



BackupCredentialsSpec configures the backup credentials Secret. The
operator creates it unless it is disabled.



_Appears in:_
- [DatabaseSpec](#databasespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `disabled` _boolean_ | Disabled stops the operator from creating the backup user and Secret.<br />Defaults to false. |  | Optional: \{\} <br /> |
| `secretName` _string_ | SecretName is the name of the credentials Secret. The default comes<br />from the CR name, as the field that uses this type states. |  | Optional: \{\} <br /> |


#### BackupDumpSpec



BackupDumpSpec is the dump configuration of the cluster. It holds the pod
settings that a backup can also set for one run, and the image that runs
the dump. Only the cluster sets the image. The Job runs under the
ServiceAccount of the cluster, with the cloud identity and the database
credentials of the cluster. Thus the cluster owner chooses the program
that the Job runs, never the person who creates a backup. A
LogicalBackupRDBMS replaces all the pod settings and always uses the image
of the cluster.



_Appears in:_
- [ClusterBackupSpec](#clusterbackupspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the dump pod. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the dump pod.<br />In the spec.dump of a LogicalBackupRDBMS, every name that starts with<br />PG or UPLOAD_ is reserved, and the API server refuses it. A PG* name is<br />connection policy, because libpq reads PGHOSTADDR, PGSERVICE,<br />PGOPTIONS, and more. UPLOAD_* is the upload contract. The variables of<br />a backup reach only the dump container, never the upload container.<br />Cloud SDKs read endpoint, proxy, and configuration variables from the<br />environment, and the author of a backup must not change where the dump<br />goes.<br />In the spec.backup.dump of the cluster, no name is reserved, and the<br />variables reach every container. The cluster owner sets the connection<br />policy, PGSSLMODE included.<br />Unlike the extraEnv of a workload, this list is atomic. One field<br />manager owns the whole list, because no extension adds entries to a<br />dump pod. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources of the dump pod. In the<br />spec.dump of a LogicalBackupRDBMS, every source needs a prefix that<br />cannot make a PG* or UPLOAD_* name. The writer of the referenced object<br />chooses its keys. As with ExtraEnv, the sources of a<br />backup reach only the dump container. The block of the cluster needs no<br />prefix, and its sources reach every container. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the dump pod. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the dump pod. A service mesh<br />sidecar keeps running after the dump finishes, and the Job does not<br />complete. Set the injection annotation of the mesh to false here. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling holds the scheduling constraints of the dump pod. When set,<br />it replaces the block of a preset, with no merge. |  | Optional: \{\} <br /> |
| `scratchVolume` _[ScratchVolumeSpec](#scratchvolumespec)_ | ScratchVolume is where the pod writes the dump before the upload. When<br />set, it replaces the block of a preset, with no merge. The dump pod<br />runs with fsGroup 999, the postgres group. Thus pg_dump can write to a<br />volume that a storage class gives with the root owner. |  | Optional: \{\} <br /> |
| `activeDeadlineSeconds` _integer_ | ActiveDeadlineSeconds is the number of seconds that the dump Job can<br />run before it fails, counted from its start. When the dump block that<br />applies does not set it, the Job gets 86400 (24 hours). In the block of<br />the cluster, an unset value takes the value of the preset. The<br />spec.dump of a LogicalBackupRDBMS replaces the block of the cluster, so<br />an unset value there gives 86400, not the value of the cluster.<br />The default is large, so that a very large dump can complete. The Job<br />always has a deadline. A pod that cannot start uses no retry, so<br />without a deadline a broken Job stays active for the life of the<br />backup. A lower value fails a stuck dump sooner. A higher value gives a<br />long dump the time that it needs. |  | Minimum: 1 <br />Optional: \{\} <br /> |
| `postgresImage` _string_ | PostgresImage is the full image reference of the dump container. It<br />replaces the default postgres:<major> of the upstream registry. Set it<br />in an air-gapped installation, where the default image is not<br />available. |  | Optional: \{\} <br /> |


#### BackupPart



BackupPart is the observed state of one part of the backup set: the
web-application indices, the exported record indices, or the Zeebe
partitions.



_Appears in:_
- [LogicalBackupElasticsearchStatus](#logicalbackupelasticsearchstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _[BackupPartState](#backuppartstate)_ | State is the state of this part. |  | Enum: [Pending InProgress Completed Failed] <br />Optional: \{\} <br /> |
| `failureReason` _string_ | FailureReason is set when State is Failed. |  | Optional: \{\} <br /> |


#### BackupPartState

_Underlying type:_ _string_

BackupPartState is the state of one part of the backup set.

_Validation:_
- Enum: [Pending InProgress Completed Failed]

_Appears in:_
- [BackupPart](#backuppart)

| Field | Description |
| --- | --- |
| `Pending` |  |
| `InProgress` |  |
| `Completed` |  |
| `Failed` |  |


#### BackupSchedule



BackupSchedule creates logical backups of one CamundaCluster on a cron
schedule, and deletes old backups that have its label. At each trigger, the
operator creates the backup kind for the storage type of the cluster. The
backup has the name <schedule>-<unix-timestamp> and the labels
camunda.io/cluster and camunda.io/backup-schedule. If a name is too long
for a resource name or a label value, the operator shortens it. Then it
adds a hash of the full name, so two long names stay different.

The backups have no owner reference to the schedule, so a deletion of the
schedule never deletes its backups. The schedule skips a trigger while a
reference does not resolve, and the Ready condition shows the reason. It
also skips a trigger while the cluster is suspended or cannot start a
backup yet. It skips a trigger too while a backup of this schedule is not
in a final phase. It records an event for these three skips, and no event
for a reference that does not resolve.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `BackupSchedule` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[BackupScheduleSpec](#backupschedulespec)_ | spec defines the desired state of BackupSchedule |  | Required: \{\} <br /> |
| `status` _[BackupScheduleStatus](#backupschedulestatus)_ | status defines the observed state of BackupSchedule |  | Optional: \{\} <br /> |


#### BackupScheduleSpec



BackupScheduleSpec is the backup policy of one cluster: when the schedule
creates a backup, and how many backups it keeps.



_Appears in:_
- [BackupSchedule](#backupschedule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clusterRef` _[ClusterRef](#clusterref)_ | ClusterRef references the CamundaCluster to back up. At each trigger,<br />the operator creates the backup kind for the storage type of the<br />cluster: LogicalBackupElasticsearch or LogicalBackupRDBMS. |  | Required: \{\} <br /> |
| `schedule` _string_ | Schedule is when the backups run: a five-field cron expression<br />(minute, hour, day of month, month, day of week), evaluated in UTC. |  | Pattern: `^\s*([0-9A-Za-z*?,/-]+\s+)\{4\}[0-9A-Za-z*?,/-]+\s*$` <br />Required: \{\} <br /> |
| `retained` _[RetainedBackups](#retainedbackups)_ | Retained limits how many backups of this schedule stay. The schedule<br />counts and deletes the backups in its namespace whose<br />camunda.io/backup-schedule label names this schedule. A backup that you<br />create with that label counts too, and the schedule can delete it. The<br />schedule never deletes a backup that is not in a final phase. | \{  \} | Optional: \{\} <br /> |


#### BackupScheduleStatus



BackupScheduleStatus is the observed state of the schedule.



_Appears in:_
- [BackupSchedule](#backupschedule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `lastScheduleTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | LastScheduleTime is the last trigger that the schedule used, also when<br />it skipped the trigger. The schedule never tries a skipped trigger<br />again. |  | Optional: \{\} <br /> |
| `lastBackupName` _string_ | LastBackupName is the backup that the schedule created most recently. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. The Ready condition has the<br />reason Healthy when the references and the cron expression are valid,<br />and InvalidReference when one of them is not. |  | Optional: \{\} <br /> |


#### BasicAuthSpec



BasicAuthSpec configures the admin credential of a basic-auth cluster.



_Appears in:_
- [ClusterAuthSpec](#clusterauthspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `adminEmail` _string_ | AdminEmail is the email address of the admin user that the operator<br />seeds. The orchestration cluster stores it on that user, and the Admin<br />web application shows it. Set the address of the person or the team<br />that owns the cluster.<br />When empty, the operator uses admin@example.com. RFC 2606 reserves this<br />domain for documentation, so the default is not the address of a real<br />person.<br />The operator applies a changed value to the running cluster. The<br />cluster validates the address and refuses a domain without a dot. When<br />the cluster refuses an address, AdminSecretReady shows its answer. |  | MaxLength: 253 <br />Pattern: `^$\|^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$` <br />Optional: \{\} <br /> |
| `passwordRotation` _string_ | PasswordRotation requests one rotation of the admin password. Set it<br />to a value that differs from the applied one, for example a date. The<br />operator generates a new password and sets it on the admin user of the<br />running cluster. Then it publishes the password in the admin Secret.<br />The connectors Deployment restarts with the new password. An empty<br />value never rotates. A suspended cluster rotates after it resumes.<br />status.adminPassword.rotation shows the applied value. A preset can set<br />this field. Then every cluster that references the preset rotates, and<br />each of these clusters reports its own status. |  | MaxLength: 253 <br />Optional: \{\} <br /> |




#### CamundaCluster



CamundaCluster describes one Camunda orchestration cluster: the Zeebe
brokers, the gateway, the web applications, and, when enabled, the
connectors runtime. The operator creates the StatefulSets, Deployments,
and Services of the cluster and keeps them in the declared state.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `CamundaCluster` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[CamundaClusterSpec](#camundaclusterspec)_ | spec defines the desired state of CamundaCluster |  | Required: \{\} <br /> |
| `status` _[CamundaClusterStatus](#camundaclusterstatus)_ | status defines the observed state of CamundaCluster |  | Optional: \{\} <br /> |


#### CamundaClusterPreset



CamundaClusterPreset is a cluster-scoped baseline configuration for
CamundaCluster resources. It has no controller, creates nothing, and
reports no status. A CamundaCluster reads it through its presetRef and
merges its own fields over it, with the rules of the preset doc.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `CamundaClusterPreset` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[CamundaClusterPresetSpec](#camundaclusterpresetspec)_ | spec defines the desired state of CamundaClusterPreset |  | Required: \{\} <br /> |


#### CamundaClusterPresetSpec



CamundaClusterPresetSpec defines the desired state of CamundaClusterPreset.



_Appears in:_
- [CamundaClusterPreset](#camundaclusterpreset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cluster` _[CamundaClusterSpec](#camundaclusterspec)_ | Cluster is the configuration baseline that the clusters that reference<br />the preset get. It uses the CamundaCluster spec type. A preset must not<br />set the fields that belong to one cluster only: platformConfigRef,<br />presetRef, releaseRef, externalUrl, serviceAccount, storageRef,<br />backupStorageRef, documentStorageRef, monitoring, suspend, and pause.<br />It must also not set the fields that belong to a CamundaRelease:<br />version and connectors.version. An explicit zero value, such as an<br />empty presetRef or suspend: false, counts as unset. Templated YAML<br />often renders unset fields in this way. The CamundaCluster doc gives<br />the details of each field. The CamundaClusterPreset doc gives the merge<br />rules. |  | Required: \{\} <br /> |


#### CamundaClusterSpec



CamundaClusterSpec defines the desired state of CamundaCluster.

A CamundaClusterPreset uses the same type as its baseline. For this
reason, the schema marks storageRef and platformConfigRef as optional, and
the CamundaCluster requires them. A preset refuses the fields that belong
to one cluster only.



_Appears in:_
- [CamundaCluster](#camundacluster)
- [CamundaClusterPresetSpec](#camundaclusterpresetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `platformConfigRef` _string_ | PlatformConfigRef names the cluster-scoped CamundaPlatformConfig that<br />provides auth, license, and image repositories. Required on a<br />CamundaCluster, forbidden in a preset. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br />Optional: \{\} <br /> |
| `presetRef` _string_ | PresetRef names a cluster-scoped CamundaClusterPreset used as the<br />configuration baseline. The preset doc lists the merge rules. |  | Optional: \{\} <br /> |
| `releaseRef` _string_ | ReleaseRef names a cluster-scoped CamundaRelease that provides the<br />versions, the pinned images, and the environment of a version. It<br />merges over the preset and under this spec. Forbidden in a preset. |  | Optional: \{\} <br /> |
| `version` _string_ | Version is the Camunda version to deploy, as a full semantic version.<br />Required unless the resolved release provides it. Forbidden in a<br />preset. The operator refuses a version below 8.9.0, also when it comes<br />from the release. The operator also refuses a version below the<br />version that the brokers run, with Ready reason VersionDowngradeRefused.<br />The annotation camunda.io/allow-version-downgrade on the CamundaCluster,<br />set to that version, permits it. |  | Pattern: `^\d+\.\d+\.\d+$` <br />Optional: \{\} <br /> |
| `externalUrl` _string_ | ExternalURL is the external base URL of the cluster. The operator uses<br />it for OIDC redirect URLs and web application links. It creates no<br />Ingress. |  | Optional: \{\} <br /> |
| `serviceAccount` _[ServiceAccountSpec](#serviceaccountspec)_ | ServiceAccount configures the ServiceAccount of every workload. The<br />workloads get access to the buckets of backupStorageRef and<br />documentStorageRef through this identity. |  | Optional: \{\} <br /> |
| `auth` _[ClusterAuthSpec](#clusterauthspec)_ | Auth holds the credentials of this cluster and the identities that get<br />its admin role. Under OIDC, it holds the client credentials and the<br />administrators. Under basic authentication, it holds the admin<br />credential that the operator owns. |  | Optional: \{\} <br /> |
| `zeebe` _[ZeebeSpec](#zeebespec)_ | Zeebe configures the brokers. |  | Optional: \{\} <br /> |
| `gateway` _[GatewaySpec](#gatewayspec)_ | Gateway configures the gateway. |  | Optional: \{\} <br /> |
| `operate` _[WebAppSpec](#webappspec)_ | Operate configures the Operate web application. |  | Optional: \{\} <br /> |
| `tasklist` _[WebAppSpec](#webappspec)_ | Tasklist configures the Tasklist web application. |  | Optional: \{\} <br /> |
| `admin` _[WebAppSpec](#webappspec)_ | Admin configures the Admin web application (Orchestration Cluster<br />Identity before Camunda 8.9). Its Spring profile is admin. |  | Optional: \{\} <br /> |
| `connectors` _[ConnectorsSpec](#connectorsspec)_ | Connectors configures the connectors runtime. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of every workload. A<br />per-component entry wins over an entry here with the same name.<br />The list merges by name under server-side apply. Each field manager owns<br />only the entries that it applies, so an extension controller can add its<br />own entry next to yours. One applied manifest cannot hold two entries<br />with the same name.<br />A CamundaManagementCluster that serves this cluster owns the four<br />CAMUNDA_CONSOLE_PING_ entries (CAMUNDA_HUB_PING_ on Camunda 8.10 and<br />later). It replaces what you set under these names. If your entry under<br />one of these names has valueFrom, the management cluster removes it. One<br />entry cannot hold a value and a reference together.<br />Two field managers that apply the same name do not conflict, because the<br />merge is per field inside the entry. One manager can own value and the<br />other valueFrom. A container rejects an entry that has both, so the API<br />server refuses to store such an entry. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of<br />every workload. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of every workload pod. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of every workload pod. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling holds the scheduling constraints of every workload that does<br />not set its own. When set, it replaces the scheduling block of a preset,<br />with no merge. |  | Optional: \{\} <br /> |
| `storageRef` _string_ | StorageRef names the SecondaryStorageConfig, in the namespace of this<br />cluster, that describes the secondary storage backend. Required on a<br />CamundaCluster, forbidden in a preset. One backend serves one<br />CamundaCluster. Two contracts that name one address are one backend. If<br />another cluster already holds the backend, the operator suspends this<br />cluster with Ready reason StorageAlreadyAttached. The suspension stays<br />until the other cluster moves to another backend or is deleted. |  | Optional: \{\} <br /> |
| `indexReplicas` _integer_ | IndexReplicas is the replica count of each index that the cluster<br />creates in an Elasticsearch secondary storage. When it is not set, the<br />nodeCount of the storage contract gives the count: 0 on one node, 1 on<br />two or more nodes. Without a nodeCount, Camunda keeps its own default.<br />When indexReplicas is not set and the extraEnv of a process sets a<br />legacy replica key, that process keeps the value of the key.<br />The cluster applies the count to its existing indices when it starts.<br />If the nodes cannot place the count, the operator keeps it and records<br />an IndexReplicasExceedNodes Warning event on the CamundaCluster. A<br />relational secondary<br />storage ignores this field. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `backupStorageRef` _string_ | BackupStorageRef names an ObjectStorageConfig, in the namespace of<br />this cluster, for backups. |  | Optional: \{\} <br /> |
| `documentStorageRef` _string_ | DocumentStorageRef names an ObjectStorageConfig, in the namespace of<br />this cluster, for document storage. |  | Optional: \{\} <br /> |
| `backup` _[ClusterBackupSpec](#clusterbackupspec)_ | Backup configures the backups of this cluster. It sets the schedule and<br />the retention of the primary-storage backups that Zeebe takes, and the<br />pod of the database dump Job. A preset can set this block, but not<br />backupStorageRef. The block applies only to a cluster with relational<br />secondary storage. Only such a cluster has continuous and scheduled<br />primary-storage backups and the dump Job. |  | Optional: \{\} <br /> |
| `monitoring` _[ClusterMonitoringSpec](#clustermonitoringspec)_ | Monitoring configures the monitoring integrations. |  | Optional: \{\} <br /> |
| `suspend` _boolean_ | Suspend scales every workload to zero and keeps the data. Defaults to<br />false. An annotation with the prefix suspension-hold.camunda.io/ on the<br />CamundaCluster also suspends it, whatever this field says. |  | Optional: \{\} <br /> |
| `pause` _boolean_ | Pause stops the reconciliation of this cluster: the operator changes no<br />workload and writes no status. It records a Paused event instead. Other<br />resources that use the cluster, such as a BackupSchedule or a restore,<br />still act on it. When you delete a paused cluster, the operator still<br />releases its storage backends. Defaults to false. |  | Optional: \{\} <br /> |


#### CamundaClusterStatus



CamundaClusterStatus is the observed state of a CamundaCluster.



_Appears in:_
- [CamundaCluster](#camundacluster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `volumes` _[VolumeStatus](#volumestatus) array_ | Volumes lists the bound broker PersistentVolumeClaims and the capacity<br />that each one reports, sorted by name. |  | Optional: \{\} <br /> |
| `management` _[ManagementBinding](#managementbinding)_ | Management is the published address of the management API of this<br />cluster. Extensions read it and do not derive the Service name, the<br />port, and the authentication themselves. It is unset while the cluster<br />is suspended, so a consumer does not use an old endpoint. |  | Optional: \{\} <br /> |
| `gateway` _[GatewayBinding](#gatewaybinding)_ | Gateway is the published address of the client APIs of this cluster.<br />Extensions read it and do not derive the Service name and the ports<br />themselves. It is unset while the cluster is suspended, so a consumer<br />does not use an old endpoint. |  | Optional: \{\} <br /> |
| `adminPassword` _[AdminPasswordStatus](#adminpasswordstatus)_ | AdminPassword is the state of the admin credential of a basic-auth<br />cluster. It is unset under OIDC. |  | Optional: \{\} <br /> |
| `serviceAccountName` _string_ | ServiceAccountName is the ServiceAccount that the pods of this cluster<br />run under. It is empty when they run under the default account of the<br />namespace. An extension that runs a pod for this cluster reads it and<br />does not derive the name itself. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. Ready holds the reason of a<br />failed pre-check, or it follows the conditions of the components that<br />the cluster needs. The per-process conditions (ZeebeReady, GatewayReady,<br />OperateReady, TasklistReady, AdminReady, ConnectorsReady) also appear<br />here. |  | Optional: \{\} <br /> |


#### CamundaManagementCluster



CamundaManagementCluster describes one Camunda management plane: Management
Identity, an identity provider, and, when set, Console and Web Modeler. The
operator creates its Deployments and Services. It writes the
ManagementAuthConfig that Optimize reads. It attaches the management plane
to the orchestration clusters that clusterSelector matches.

The creation of a CamundaManagementCluster is a task for a platform
administrator. The selector reaches CamundaClusters in every namespace,
and the operator adds annotations to the clusters that it matches.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `CamundaManagementCluster` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[CamundaManagementClusterSpec](#camundamanagementclusterspec)_ | spec defines the desired state of CamundaManagementCluster |  | Required: \{\} <br /> |
| `status` _[CamundaManagementClusterStatus](#camundamanagementclusterstatus)_ | status defines the observed state of CamundaManagementCluster |  | Optional: \{\} <br /> |


#### CamundaManagementClusterSpec



CamundaManagementClusterSpec describes one management plane: Management
Identity, its identity provider, and, when set, Console and Web Modeler.



_Appears in:_
- [CamundaManagementCluster](#camundamanagementcluster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `platformConfigRef` _string_ | PlatformConfigRef names the cluster-scoped CamundaPlatformConfig that<br />holds the license and the image settings. In the oidc mode, it also<br />holds the identity provider and every client of the management plane. |  | MinLength: 1 <br /> |
| `suspend` _boolean_ | Suspend scales every workload of this management cluster to zero. The<br />ManagementAuthConfig, the claims on the orchestration clusters, and the<br />Console ping settings stay. Thus nothing else must change while the<br />management plane is down. |  | Optional: \{\} <br /> |
| `clusterSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#labelselector-v1-meta)_ | ClusterSelector selects the CamundaClusters that Console and Web<br />Modeler serve, in every namespace that namespaceSelector permits. An<br />unset selector selects no cluster. An empty selector (\{\}) selects every<br />cluster. |  | Optional: \{\} <br /> |
| `namespaceSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#labelselector-v1-meta)_ | NamespaceSelector limits clusterSelector to the namespaces whose labels<br />match. It selects on the labels of the Namespace objects, as the<br />namespaceSelector of an admission webhook does. An unset or empty (\{\})<br />selector permits every namespace. |  | Optional: \{\} <br /> |
| `managementAuthConfigName` _string_ | ManagementAuthConfigName is the name of the cluster-scoped<br />ManagementAuthConfig that this management cluster writes. A<br />CamundaOptimize reads it through its managementAuthRef. Empty means the<br />name of this resource. |  | Optional: \{\} <br /> |
| `identityProvider` _[IdentityProviderSpec](#identityproviderspec)_ | IdentityProvider selects where users authenticate. Exactly one of<br />keycloak, externalKeycloak, or oidc is set. |  |  |
| `identity` _[IdentitySpec](#identityspec)_ | Identity configures Management Identity. Console, Web Modeler, and<br />Optimize authenticate through it, so the operator always deploys it. |  |  |
| `console` _[ConsoleSpec](#consolespec)_ | Console configures Console. While it is unset, the operator does not<br />deploy Console. |  | Optional: \{\} <br /> |
| `webModeler` _[WebModelerSpec](#webmodelerspec)_ | WebModeler configures Web Modeler. While it is unset, the operator does<br />not deploy Web Modeler. |  | Optional: \{\} <br /> |


#### CamundaManagementClusterStatus



CamundaManagementClusterStatus is the observed state of a
CamundaManagementCluster.



_Appears in:_
- [CamundaManagementCluster](#camundamanagementcluster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `managementAuthConfig` _string_ | ManagementAuthConfig is the name of the ManagementAuthConfig that this<br />management cluster writes. A CamundaOptimize reads it through its<br />managementAuthRef. |  | Optional: \{\} <br /> |
| `clusters` _[AttachedClusterStatus](#attachedclusterstatus) array_ | Clusters lists every CamundaCluster that clusterSelector matched, and<br />shows whether the management plane serves it. |  | Optional: \{\} <br /> |
| `optimize` _[AttachedOptimizeStatus](#attachedoptimizestatus) array_ | Optimize lists every CamundaOptimize that names the ManagementAuthConfig<br />of this management cluster and sets spec.externalUrl, sorted by<br />namespace and name. The management plane registers the login callback<br />of each one on the Optimize client of the realm.<br />The list is empty in the oidc mode. In that mode, the identity provider<br />of the platform config holds the callback URLs. |  | Optional: \{\} <br /> |
| `callbackRealm` _[KeycloakRealmTarget](#keycloakrealmtarget)_ | CallbackRealm is the Keycloak realm that this management plane last<br />gave to Management Identity, in the externalKeycloak mode. Identity<br />registers the login callbacks of Optimize in this realm when it starts.<br />Thus the field appears with the realm, not with the first registration.<br />A value does not mean that the callbacks are still in the realm. When<br />the spec names another realm or the oidc mode, the operator first<br />removes the callbacks from this realm. The field stays until no<br />configuration of the old realm can write them back. That is the case<br />when no Management Identity pod of the old realm can run, and no<br />Deployment or ReplicaSet of it can start one. The field then disappears<br />or moves to the new realm. This change shows that the move is complete.<br />The field is absent after a move into the keycloak or the oidc mode.<br />It is also absent while no login callback of this operator is<br />registered anywhere and nothing can write one back. The operator runs the<br />Keycloak of the keycloak mode, and deletes it with the management plane<br />or when the plane leaves that mode. Thus the field never names the<br />realm of that Keycloak. During a move into either mode, the field still<br />names the realm that the plane leaves.<br />A Keycloak that is permanently gone never answers. To release its realm<br />with the callbacks in it, set the annotation<br />camunda.io/forget-callback-realm. Use the value that the<br />OptimizeCallbacksReady message names. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. Ready holds the reason of a<br />failed pre-check, or StepFailed for a failed step. When the write of<br />the ManagementAuthConfig is the first failed step, Ready holds<br />WriteFailed instead. In the oidc mode it holds ImmutableAfterStart when<br />spec.identity.admin changed after Management Identity first started.<br />Otherwise it follows the conditions of the deployed components. When<br />all of them are True, a failed OptimizeCallbacksReady makes Ready False<br />with its reason. The per-component conditions (KeycloakReady,<br />IdentityReady, ConsoleReady, WebModelerReady, ManagementAuthReady,<br />SecretsReady, MirroredSecretsReady) and OptimizeCallbacksReady also<br />appear here. |  | Optional: \{\} <br /> |


#### CamundaOptimize



CamundaOptimize describes one Camunda Optimize instance attached to one
CamundaCluster. The operator creates a webapp Deployment, an importer
Deployment, and their Services. It also enables the Elasticsearch exporter
of the referenced cluster, so Optimize has data to import.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `CamundaOptimize` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[CamundaOptimizeSpec](#camundaoptimizespec)_ | spec defines the desired state of CamundaOptimize |  | Required: \{\} <br /> |
| `status` _[CamundaOptimizeStatus](#camundaoptimizestatus)_ | status defines the observed state of CamundaOptimize |  | Optional: \{\} <br /> |


#### CamundaOptimizeSpec



CamundaOptimizeSpec is the desired state of one Optimize instance. It
attaches to one CamundaCluster and reads the data of that cluster from
Elasticsearch.

The spec has no platformConfigRef. The image repository and the license come
from the CamundaPlatformConfig of the referenced cluster, so Optimize and
the cluster cannot have different values.



_Appears in:_
- [CamundaOptimize](#camundaoptimize)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version is the Optimize version to deploy, as a full semantic version.<br />Optimize has its own patch line, so it does not follow the version of<br />the cluster. The major and the minor must match the effective version<br />of the referenced cluster. If they differ, Ready reports<br />VersionMismatch. |  | Pattern: `^\d+\.\d+\.\d+$` <br /> |
| `managementAuthRef` _string_ | ManagementAuthRef names the cluster-scoped ManagementAuthConfig that<br />provides the Management Identity OIDC configuration. Optimize<br />authenticates against Management Identity, not against the built-in<br />auth of the orchestration cluster. |  | MinLength: 1 <br /> |
| `externalUrl` _string_ | ExternalURL is the URL where browsers reach this Optimize.<br />In the two Keycloak modes, the management plane of managementAuthRef<br />registers <externalUrl>/api/authentication/callback on the optimize<br />client of the realm. Thus a person who signs in here comes back here.<br />Without a URL, the plane registers no callback, and Keycloak refuses<br />the return. The exception is a callback that somebody added to the<br />realm manually.<br />In the oidc mode, the field has no effect. The identity provider of the<br />platform config holds the callback URLs, so add<br /><externalUrl>/api/authentication/callback there. |  | Optional: \{\} <br /> |
| `clusterRef` _[ClusterRef](#clusterref)_ | ClusterRef names the CamundaCluster that this Optimize instance reads.<br />The secondary storage of that cluster must be Elasticsearch. No other<br />CamundaOptimize can be attached to it.<br />The reference is immutable. A change to another cluster changes the pod<br />selectors of the Deployments, and Kubernetes does not permit that.<br />Also, the old cluster keeps the exporter settings of this Optimize. |  |  |
| `webapp` _[WorkloadSpec](#workloadspec)_ | Webapp configures the Deployment that serves the Optimize user<br />interface. It runs with data import off. |  | Optional: \{\} <br /> |
| `importer` _[WorkloadSpec](#workloadspec)_ | Importer configures the Deployment that imports the exported cluster<br />data into the Optimize indices. Optimize supports one active importer,<br />so replicas must be 0 or 1. With 0, the import stops, for example<br />during a restore or an index rewrite. The webapp continues to serve the<br />data that it already imported. |  | Optional: \{\} <br /> |
| `indexReplicas` _integer_ | IndexReplicas is the replica count of each Optimize index, and of each<br />zeebe-record index that the exporter of the cluster writes for this<br />Optimize. When it is not set, the nodeCount of the storage contract of<br />the cluster gives the count. The count is 0 on one node, and 1 on two or<br />more nodes.<br />Without a nodeCount, Optimize and the exporter keep their own defaults.<br />Optimize applies the count to its existing indices when it starts. The<br />exporter applies it to the next zeebe-record indices that it creates.<br />If the nodes cannot place the count, the operator keeps it and records<br />an IndexReplicasExceedNodes Warning event on the CamundaOptimize. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `monitoring` _[OptimizeMonitoringSpec](#optimizemonitoringspec)_ | Monitoring configures the monitoring integrations. |  | Optional: \{\} <br /> |


#### CamundaOptimizeStatus



CamundaOptimizeStatus is the observed state of a CamundaOptimize.



_Appears in:_
- [CamundaOptimize](#camundaoptimize)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. Ready holds the reason of a<br />failed pre-check. Otherwise it follows the conditions of the two<br />workloads, and MirroredSecretsReady when the Optimize references a<br />Secret in another namespace. The per-workload conditions (WebappReady,<br />ImporterReady) and MirroredSecretsReady also appear here. |  | Optional: \{\} <br /> |
| `suspendedBy` _[OptimizeSuspension](#optimizesuspension)_ | SuspendedBy tells why the Optimize workloads are at zero with the<br />referenced cluster. It is empty while they follow their spec. It stays<br />set while a failed check keeps them at zero after the cluster resumed. |  | Enum: [Cluster StorageClaim] <br />Optional: \{\} <br /> |


#### CamundaPlatformConfig



CamundaPlatformConfig is the cluster-scoped CRD that holds the platform
settings of an environment: the identity provider, the license, and the
image repositories. Every orchestration cluster that references it uses
these settings.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `CamundaPlatformConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[CamundaPlatformConfigSpec](#camundaplatformconfigspec)_ | spec defines the desired state of CamundaPlatformConfig |  | Required: \{\} <br /> |
| `status` _[CamundaPlatformConfigStatus](#camundaplatformconfigstatus)_ | status defines the observed state of CamundaPlatformConfig |  | Optional: \{\} <br /> |


#### CamundaPlatformConfigSpec



CamundaPlatformConfigSpec holds the settings that are the same for all
orchestration clusters of an environment.



_Appears in:_
- [CamundaPlatformConfig](#camundaplatformconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `auth` _[PlatformAuthSpec](#platformauthspec)_ | Auth holds the authentication settings of the orchestration clusters.<br />Unset means basic authentication. |  | Optional: \{\} <br /> |
| `licenseSecretRef` _[SecretKeyRef](#secretkeyref)_ | LicenseSecretRef names the Secret key that holds the Camunda license<br />key. Without it, clusters run in unlicensed non-production mode. |  | Optional: \{\} <br /> |
| `images` _[ImagesSpec](#imagesspec)_ | Images changes the repositories of the images that the operator pulls,<br />one entry for each image, for example to a mirror. Each value is a full<br />repository with its registry and no tag. The tag always comes from the<br />version field of the resource that runs the image. A registry with a<br />port needs a path after the port, as in registry:5000/camunda/optimize.<br />The operator appends the tag to the value, as in<br />registry:5000/camunda/optimize:<version>. |  | Optional: \{\} <br /> |


#### CamundaPlatformConfigStatus



CamundaPlatformConfigStatus is the observed validation state of the
platform config.



_Appears in:_
- [CamundaPlatformConfig](#camundaplatformconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current validation state. The Ready condition<br />has the reasons Healthy or MissingSecret. |  | Optional: \{\} <br /> |


#### CamundaRelease



CamundaRelease is a cluster-scoped description of what a platform runs: the
versions, the pinned images, and the environment that a version needs. It
has no controller, creates nothing, and reports no status. A
CamundaCluster, an ElasticsearchCluster, and a DatabaseServer each read it
through their releaseRef. They merge it over their preset and under their
own spec.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `CamundaRelease` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[CamundaReleaseSpec](#camundareleasespec)_ | spec defines the desired state of CamundaRelease |  | Required: \{\} <br /> |


#### CamundaReleaseSpec



CamundaReleaseSpec holds what a platform runs: the version of the
orchestration cluster, and the versions of its storage. It also holds the
image references that replace the images of these versions, and the
environment that a version needs.



_Appears in:_
- [CamundaRelease](#camundarelease)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version is the Camunda version of the orchestration cluster processes,<br />as a full semantic version. A cluster that references the release<br />refuses an effective version below 8.9.0. Its own spec.version wins over<br />this value. |  | Pattern: `^\d+\.\d+\.\d+$` <br />Required: \{\} <br /> |
| `connectors` _[ReleaseConnectorsSpec](#releaseconnectorsspec)_ | Connectors holds the version and the environment of the connectors<br />runtime. |  | Optional: \{\} <br /> |
| `elasticsearch` _[ReleaseElasticsearchSpec](#releaseelasticsearchspec)_ | Elasticsearch holds the version of the Elasticsearch clusters of this<br />release. An ElasticsearchCluster takes it through its releaseRef. |  | Optional: \{\} <br /> |
| `databaseServer` _[ReleaseDatabaseServerSpec](#releasedatabaseserverspec)_ | DatabaseServer holds the version of the PostgreSQL servers of this<br />release. A DatabaseServer takes it through its releaseRef. |  | Optional: \{\} <br /> |
| `images` _[ReleaseImagesSpec](#releaseimagesspec)_ | Images replaces the image reference of a process. The operator uses an<br />entry as it is, with its tag or digest. It changes only the image that<br />the operator pulls. The operator still uses the version above for the<br />version rules, the downgrade rule, and the environment that it<br />computes. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of every workload. The entries<br />merge by name over the preset entries and under the cluster entries. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of<br />every workload. They follow the preset sources and precede the cluster<br />sources. |  | Optional: \{\} <br /> |
| `zeebe` _[ReleaseEnvSpec](#releaseenvspec)_ | Zeebe holds the environment of the brokers. |  | Optional: \{\} <br /> |
| `gateway` _[ReleaseEnvSpec](#releaseenvspec)_ | Gateway holds the environment of the gateway. |  | Optional: \{\} <br /> |
| `operate` _[ReleaseEnvSpec](#releaseenvspec)_ | Operate holds the environment of Operate. |  | Optional: \{\} <br /> |
| `tasklist` _[ReleaseEnvSpec](#releaseenvspec)_ | Tasklist holds the environment of Tasklist. |  | Optional: \{\} <br /> |
| `admin` _[ReleaseEnvSpec](#releaseenvspec)_ | Admin holds the environment of the Admin web application. |  | Optional: \{\} <br /> |


#### ClusterAdminSpec



ClusterAdminSpec holds the identities that get the admin role of one
cluster under OIDC. The identity provider authenticates a caller, and
nothing else authorizes one, so an OIDC cluster without this block has no
administrator. Basic authentication seeds its own administrator and ignores
the block.



_Appears in:_
- [ClusterAuthSpec](#clusterauthspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `users` _string array_ | Users are the values of the username claim that get the admin role. |  | items:MinLength: 1 <br />Optional: \{\} <br /> |
| `clients` _string array_ | Clients are the values of the client id claim that get the admin role.<br />A client entry matches only when the platform config sets<br />auth.oidc.clientIdClaim. |  | items:MinLength: 1 <br />Optional: \{\} <br /> |
| `mappingRules` _[AdminMappingRule](#adminmappingrule) array_ | MappingRules give the admin role to every token that holds a claim with<br />a given value. |  | Optional: \{\} <br /> |


#### ClusterAuthSpec



ClusterAuthSpec holds the credentials of one cluster and the identities
that get its admin role. Under OIDC it carries the client credentials,
which override the defaults of the platform config and of the preset, and
the identities of the administrators. A block that sets clientId replaces
the whole client: the audience and the client secret then come from this
block only. A block without clientId overrides the audience and the client
secret one by one. Under basic authentication it carries the basic block,
which configures the admin credential that the operator owns.



_Appears in:_
- [CamundaClusterSpec](#camundaclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clientId` _string_ | ClientID is the OIDC client ID of this cluster. A client ID replaces<br />the whole client: the audience and the client secret then come from<br />this block only. Requires clientSecretRef. |  | Optional: \{\} <br /> |
| `audience` _string_ | Audience is the audience that access tokens must carry. Defaults to<br />the client ID that the cluster uses. |  | Optional: \{\} <br /> |
| `clientSecretRef` _[LocalSecretKeyRef](#localsecretkeyref)_ | ClientSecretRef names the Secret that holds the OIDC client secret of<br />this cluster. Required when clientId is set. |  | Optional: \{\} <br /> |
| `admin` _[ClusterAdminSpec](#clusteradminspec)_ | Admin holds the identities that get the admin role of this cluster. It<br />applies under OIDC only. Basic authentication seeds its own<br />administrator and ignores this block. |  | Optional: \{\} <br /> |
| `basic` _[BasicAuthSpec](#basicauthspec)_ | Basic configures the admin credential that the operator owns. It<br />applies under basic authentication only. OIDC ignores this block, like<br />basic authentication ignores admin. |  | Optional: \{\} <br /> |


#### ClusterBackupSpec



ClusterBackupSpec configures the backups of one orchestration cluster.
spec.backupStorageRef sets where the backups go. This block is the policy,
so a preset can hold it for many clusters.



_Appears in:_
- [CamundaClusterSpec](#camundaclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `primaryStorage` _[PrimaryStorageBackupSpec](#primarystoragebackupspec)_ | PrimaryStorage configures the backups that Zeebe takes of its own<br />primary storage. A point-in-time restore uses them. |  | Optional: \{\} <br /> |
| `dump` _[BackupDumpSpec](#backupdumpspec)_ | Dump configures the Job that writes the logical database to the backup<br />bucket. A LogicalBackupRDBMS can replace this block as a whole. |  | Optional: \{\} <br /> |


#### ClusterMonitoringSpec



ClusterMonitoringSpec groups the monitoring integrations of a
CamundaCluster.



_Appears in:_
- [CamundaClusterSpec](#camundaclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serviceMonitor` _[ServiceMonitorSpec](#servicemonitorspec)_ | ServiceMonitor configures the Prometheus ServiceMonitors. When enabled,<br />the operator creates one ServiceMonitor for each process, with the name<br />of the workload. It scrapes /actuator/prometheus on the management port<br />9600 of a unified process, and on the HTTP port 8080 of connectors. |  | Optional: \{\} <br /> |


#### ClusterRef



ClusterRef references a CamundaCluster by name, in the namespace of the
referencing object. The reference cannot cross namespaces. For a backup,
the operator reads the Secrets of the cluster, runs a Job in its
namespace, and calls its management API. Thus the reference stays inside
the RBAC limits of the namespace of the CR. A user who can create the CR
in a namespace can back up the clusters of that namespace, and no others.



_Appears in:_
- [BackupScheduleSpec](#backupschedulespec)
- [CamundaOptimizeSpec](#camundaoptimizespec)
- [LogicalBackupElasticsearchSpec](#logicalbackupelasticsearchspec)
- [LogicalBackupRDBMSSpec](#logicalbackuprdbmsspec)
- [LogicalRestoreElasticsearchSpec](#logicalrestoreelasticsearchspec)
- [LogicalRestoreRDBMSSpec](#logicalrestorerdbmsspec)
- [PointInTimeRestoreSpec](#pointintimerestorespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the CamundaCluster, in the namespace of this<br />object. |  | MinLength: 1 <br /> |


#### ComponentMode

_Underlying type:_ _string_

ComponentMode says where a process of the unified binary runs.

_Validation:_
- Enum: [Standalone Embedded]

_Appears in:_
- [GatewaySpec](#gatewayspec)
- [WebAppSpec](#webappspec)

| Field | Description |
| --- | --- |
| `Standalone` | ComponentModeStandalone runs the process as its own Deployment of the<br />unified binary.<br /> |
| `Embedded` | ComponentModeEmbedded runs the process inside the nearest standalone<br />host up the chain: the gateway when it is standalone, otherwise zeebe.<br /> |


#### ConfidentialClientSpec



ConfidentialClientSpec is an identity provider client that authenticates
with a secret.



_Appears in:_
- [ManagementClients](#managementclients)
- [WebModelerAPIClientSpec](#webmodelerapiclientspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clientId` _string_ | ClientID is the client id at the identity provider. |  | MinLength: 1 <br /> |
| `audience` _string_ | Audience is the audience that the component validates in access tokens.<br />Empty means the client id. |  | Optional: \{\} <br /> |
| `clientSecretRef` _[SecretKeyRef](#secretkeyref)_ | ClientSecretRef names the Secret key that holds the client secret. |  |  |


#### ConnectorsSpec



ConnectorsSpec configures the connectors runtime. It is a separate
application, never part of the unified binary, and always its own
Deployment.



_Appears in:_
- [CamundaClusterSpec](#camundaclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled runs the connectors runtime when true. Defaults to false. |  | Optional: \{\} <br /> |
| `version` _string_ | Version is the version of the connectors bundle image, as a full<br />semantic version. The bundle has its own patch line, so it does not<br />follow the cluster version. Required when connectors are enabled,<br />unless the resolved release provides it. Forbidden in a preset. |  | Pattern: `^\d+\.\d+\.\d+$` <br />Optional: \{\} <br /> |
| `replicas` _integer_ | Replicas is the number of pods of this process. Defaults to 1. On a<br />CamundaCluster it has no effect on an embedded web application. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the container of this process. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the container of this<br />process. On a CamundaCluster, an entry here wins over a top-level entry<br />with the same name. The entries of an embedded web application apply to<br />its host process.<br />The list merges by name under server-side apply. Each field manager owns<br />only the entries that it applies, so an extension controller can add its<br />own entry next to yours. One applied manifest cannot hold two entries<br />with the same name.<br />Two field managers that apply the same name do not conflict, because the<br />merge is per field inside the entry. One manager can own value and the<br />other valueFrom. A container rejects an entry that has both, so the API<br />server refuses to store such an entry. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the<br />container of this process. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the pods of this process. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the pods of this process. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling holds the scheduling constraints of the pods of this process.<br />When set, it replaces all other scheduling blocks for this process, with<br />no merge. On a CamundaCluster, these are the top-level block and the<br />block of a preset. |  | Optional: \{\} <br /> |


#### ConsoleSpec



ConsoleSpec configures Console.



_Appears in:_
- [CamundaManagementClusterSpec](#camundamanagementclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version is the Console version, as a full semantic version. The<br />operator supports 8.9.0 and later. |  | Pattern: `^\d+\.\d+\.\d+$` <br /> |
| `externalUrl` _string_ | ExternalURL is the URL where browsers reach Console. Console serves<br />under the path of this URL. Every selected CamundaCluster reports to<br />Console through its Kubernetes Service. Thus the clusters do not need<br />an Ingress in front of Console. |  |  |
| `replicas` _integer_ | Replicas is the number of pods of this process. Defaults to 1. On a<br />CamundaCluster it has no effect on an embedded web application. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the container of this process. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the container of this<br />process. On a CamundaCluster, an entry here wins over a top-level entry<br />with the same name. The entries of an embedded web application apply to<br />its host process.<br />The list merges by name under server-side apply. Each field manager owns<br />only the entries that it applies, so an extension controller can add its<br />own entry next to yours. One applied manifest cannot hold two entries<br />with the same name.<br />Two field managers that apply the same name do not conflict, because the<br />merge is per field inside the entry. One manager can own value and the<br />other valueFrom. A container rejects an entry that has both, so the API<br />server refuses to store such an entry. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the<br />container of this process. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the pods of this process. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the pods of this process. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling holds the scheduling constraints of the pods of this process.<br />When set, it replaces all other scheduling blocks for this process, with<br />no merge. On a CamundaCluster, these are the top-level block and the<br />block of a preset. |  | Optional: \{\} <br /> |


#### CredentialsSpec



CredentialsSpec names the Secret to which the operator writes generated
credentials, with the keys username and password. The Secret is in the
namespace of the CR.



_Appears in:_
- [BackupCredentialsSpec](#backupcredentialsspec)
- [DatabaseSpec](#databasespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretName` _string_ | SecretName is the name of the credentials Secret. The default comes<br />from the CR name, as the field that uses this type states. |  | Optional: \{\} <br /> |


#### Database



Database creates a logical database and its users on an existing
PostgreSQL server with SQL. It publishes the result as a DatabaseConfig in
its own namespace, and, when set, as a SecondaryStorageConfig. When you
delete a Database, Kubernetes deletes the published contracts and Secrets.
The logical database and the SQL users stay on the server.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `Database` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[DatabaseSpec](#databasespec)_ | spec defines the desired state of Database |  | Required: \{\} <br /> |
| `status` _[DatabaseStatus](#databasestatus)_ | status defines the observed state of Database |  | Optional: \{\} <br /> |


#### DatabaseConfig



DatabaseConfig is the namespaced contract CRD that describes one logical
database: its server, its name, and its application credentials. The
components that connect to the database read it. Consumers find it by
name in their own namespace.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `DatabaseConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[DatabaseConfigSpec](#databaseconfigspec)_ | spec defines the desired state of DatabaseConfig |  | Required: \{\} <br /> |
| `status` _[DatabaseConfigStatus](#databaseconfigstatus)_ | status defines the observed state of DatabaseConfig |  | Optional: \{\} <br /> |


#### DatabaseConfigSpec



DatabaseConfigSpec describes one logical database: its server, its name,
and its application credentials.



_Appears in:_
- [DatabaseConfig](#databaseconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serverRef` _string_ | ServerRef names the DatabaseServerConfig of the server that holds this<br />database. |  | MinLength: 1 <br /> |
| `databaseName` _string_ | DatabaseName is the name of the logical database on the server. |  | MinLength: 1 <br /> |
| `credentialsSecretRef` _[LocalCredentialsSecretRef](#localcredentialssecretref)_ | CredentialsSecretRef names an application user with read and write<br />access to the database. |  |  |
| `backupCredentialsSecretRef` _[LocalCredentialsSecretRef](#localcredentialssecretref)_ | BackupCredentialsSecretRef names a separate user with the privileges<br />to dump the database. Backups use this user. A restore uses the<br />application user of CredentialsSecretRef. |  | Optional: \{\} <br /> |


#### DatabaseConfigStatus



DatabaseConfigStatus is the observed validation state of the contract.



_Appears in:_
- [DatabaseConfig](#databaseconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current validation state. The Ready condition<br />has the reasons Healthy, InvalidReference, or MissingSecret. |  | Optional: \{\} <br /> |


#### DatabaseEngine

_Underlying type:_ _string_

DatabaseEngine identifies the database engine of a server.

_Validation:_
- Enum: [postgres]

_Appears in:_
- [DatabaseServerConfigSpec](#databaseserverconfigspec)

| Field | Description |
| --- | --- |
| `postgres` |  |


#### DatabaseServer



DatabaseServer runs one PostgreSQL instance through the external
CloudNativePG operator. It archives the instance continuously to an object
storage bucket. It publishes the connection details as a
DatabaseServerConfig, which a Database and a PointInTimeRestore use. One or
more orchestration clusters use the instance, each through its own
Database.

A PointInTimeRestore rolls the whole instance back. It needs a server that
holds the database of its cluster and nothing else.

The name of the CR is also the name of the CloudNativePG cluster. On
create, the name must be a DNS-1035 label of at most 46 characters.
CloudNativePG accepts 50 characters, and a rollback appends up to four
("-r99"). While the recovery index is below 100, the cluster of a rollback
gets the full name. Above that, the operator shortens the name to a head
and a hash. Each archive record adds one to the index: a rollback, an
archive that is enabled again, and a change of bucket.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `DatabaseServer` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[DatabaseServerSpec](#databaseserverspec)_ | spec defines the desired state of DatabaseServer |  | Required: \{\} <br /> |
| `status` _[DatabaseServerStatus](#databaseserverstatus)_ | status defines the observed state of DatabaseServer |  | Optional: \{\} <br /> |


#### DatabaseServerArchiveSpec



DatabaseServerArchiveSpec points a server at the bucket of its continuous
archive. The archive holds the write-ahead log of every instance, and the
base backups that a recovery starts from. A server with an archive publishes
pitr.enabled true on its contract. A server without one publishes false,
and no point-in-time restore can reach it.

The archive is not the backup model of the operator. BackupSchedule and
LogicalBackupRDBMS take logical dumps and never use these base backups.



_Appears in:_
- [DatabaseServerSpec](#databaseserverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `objectStorageRef` _string_ | ObjectStorageRef names an ObjectStorageConfig in the namespace of this<br />server. The operator writes the archive under a prefix of that bucket<br />that only this server uses. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br /> |
| `retentionPeriodDays` _integer_ | RetentionPeriodDays is how far into the past a restore can reach. The<br />operator enforces this value on the bucket, and the contract of this<br />server publishes the same value.<br />The maximum is 36500 days, which is a hundred years. The operator counts<br />the reachable window in nanoseconds, and that count overflows at about<br />292 years. The maximum stays well below that limit. |  | Maximum: 36500 <br />Minimum: 1 <br /> |
| `baseBackupSchedule` _string_ | BaseBackupSchedule is when the server takes a base backup. It is the<br />six-field cron of CloudNativePG (seconds first, in UTC), or one of the<br />descriptors @yearly, @annually, @monthly, @weekly, @daily, @midnight,<br />@hourly and @every. The first base backup runs as soon as the server is<br />up, whatever the schedule says. A recovery from the archive is possible<br />only after that backup completes.<br />The API server refuses the five-field cron of a Kubernetes CronJob.<br />CloudNativePG reads the first field as seconds, so a five-field value<br />runs at a different time than intended.<br />Each field has the bounds that CloudNativePG accepts. Seconds and<br />minutes take 0-59, and hours take 0-23. The day of the month takes<br />1-31, and the month takes 1-12 or JAN-DEC. The day of the week takes<br />0-6 or SUN-SAT. The schema cannot find a range whose first value is<br />above its second, such as FRI-MON. The operator refuses such a range<br />with Ready reason InvalidReference, before it applies anything.<br />A step has at most three digits. An @every number has at most six<br />digits on each side of the point. These limits are stricter than the<br />parser of CloudNativePG. That parser accepts a longer number, then<br />overflows and stops taking base backups. | 0 0 2 * * * | Pattern: `^(\s*([*?]\|[0-5]?\d(-[0-5]?\d)?)(/[1-9]\d\{0,2\})?(,([*?]\|[0-5]?\d(-[0-5]?\d)?)(/[1-9]\d\{0,2\})?)*\s+([*?]\|[0-5]?\d(-[0-5]?\d)?)(/[1-9]\d\{0,2\})?(,([*?]\|[0-5]?\d(-[0-5]?\d)?)(/[1-9]\d\{0,2\})?)*\s+([*?]\|([01]?\d\|2[0-3])(-([01]?\d\|2[0-3]))?)(/[1-9]\d\{0,2\})?(,([*?]\|([01]?\d\|2[0-3])(-([01]?\d\|2[0-3]))?)(/[1-9]\d\{0,2\})?)*\s+([*?]\|([1-9]\|[12]\d\|3[01])(-([1-9]\|[12]\d\|3[01]))?)(/[1-9]\d\{0,2\})?(,([*?]\|([1-9]\|[12]\d\|3[01])(-([1-9]\|[12]\d\|3[01]))?)(/[1-9]\d\{0,2\})?)*\s+([*?]\|([1-9]\|1[0-2]\|[Jj]([Aa][Nn]\|[Uu][LlNn])\|[Ff][Ee][Bb]\|[Mm][Aa][RrYy]\|[Aa]([Pp][Rr]\|[Uu][Gg])\|[Ss][Ee][Pp]\|[Oo][Cc][Tt]\|[Nn][Oo][Vv]\|[Dd][Ee][Cc])(-([1-9]\|1[0-2]\|[Jj]([Aa][Nn]\|[Uu][LlNn])\|[Ff][Ee][Bb]\|[Mm][Aa][RrYy]\|[Aa]([Pp][Rr]\|[Uu][Gg])\|[Ss][Ee][Pp]\|[Oo][Cc][Tt]\|[Nn][Oo][Vv]\|[Dd][Ee][Cc]))?)(/[1-9]\d\{0,2\})?(,([*?]\|([1-9]\|1[0-2]\|[Jj]([Aa][Nn]\|[Uu][LlNn])\|[Ff][Ee][Bb]\|[Mm][Aa][RrYy]\|[Aa]([Pp][Rr]\|[Uu][Gg])\|[Ss][Ee][Pp]\|[Oo][Cc][Tt]\|[Nn][Oo][Vv]\|[Dd][Ee][Cc])(-([1-9]\|1[0-2]\|[Jj]([Aa][Nn]\|[Uu][LlNn])\|[Ff][Ee][Bb]\|[Mm][Aa][RrYy]\|[Aa]([Pp][Rr]\|[Uu][Gg])\|[Ss][Ee][Pp]\|[Oo][Cc][Tt]\|[Nn][Oo][Vv]\|[Dd][Ee][Cc]))?)(/[1-9]\d\{0,2\})?)*\s+([*?]\|([0-6]\|[Ss]([Uu][Nn]\|[Aa][Tt])\|[Mm][Oo][Nn]\|[Tt]([Uu][Ee]\|[Hh][Uu])\|[Ww][Ee][Dd]\|[Ff][Rr][Ii])(-([0-6]\|[Ss]([Uu][Nn]\|[Aa][Tt])\|[Mm][Oo][Nn]\|[Tt]([Uu][Ee]\|[Hh][Uu])\|[Ww][Ee][Dd]\|[Ff][Rr][Ii]))?)(/[1-9]\d\{0,2\})?(,([*?]\|([0-6]\|[Ss]([Uu][Nn]\|[Aa][Tt])\|[Mm][Oo][Nn]\|[Tt]([Uu][Ee]\|[Hh][Uu])\|[Ww][Ee][Dd]\|[Ff][Rr][Ii])(-([0-6]\|[Ss]([Uu][Nn]\|[Aa][Tt])\|[Mm][Oo][Nn]\|[Tt]([Uu][Ee]\|[Hh][Uu])\|[Ww][Ee][Dd]\|[Ff][Rr][Ii]))?)(/[1-9]\d\{0,2\})?)*\s*\|@((year\|annual\|month\|week\|dai\|hour)ly\|midnight\|every (\d\{1,6\}(\.\d\{1,6\})?[hms])+))$` <br />Optional: \{\} <br /> |


#### DatabaseServerArchiveStatus



DatabaseServerArchiveStatus is the observed state of the archive of a
server.



_Appears in:_
- [DatabaseServerStatus](#databaseserverstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `history` _[ArchiveRecord](#archiverecord) array_ | History lists every archive that the server wrote, oldest first. A<br />recovery uses the record whose interval holds the requested point. The<br />operator never removes a record. Thus a restore can reach back across<br />an earlier recovery for as long as the bucket keeps the objects. When<br />you remove spec.archive, the open record closes and the list stays.<br />When you add an archive again, a new record opens. A move of<br />spec.archive to another bucket does the same. No restore can reach a<br />point in a time without an archive. |  | Optional: \{\} <br /> |
| `boundary` _[ArchiveBoundary](#archiveboundary)_ | Boundary is the last move of the archive that no record holds yet. The<br />move happens when spec.archive is enabled again on another location,<br />or when the location moves again before a base backup opens a record.<br />It prevents a base backup of the old location from opening the<br />interval of the new location. It is cleared when that interval opens. |  | Optional: \{\} <br /> |
| `reachableFrom` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | ReachableFrom is the oldest point that the objects in the bucket still<br />cover. The retention period removes old objects from the bucket. A<br />longer retention period does not bring back what a shorter one already<br />removed. The window grows to the new retention period only as the<br />archive writes past this point. The server refuses a rollback to a<br />point before it. When it is unset, only the retention period limits<br />the window. |  | Optional: \{\} <br /> |


#### DatabaseServerConfig



DatabaseServerConfig is the contract CRD that describes a database server:
the engine, the endpoint, the admin credentials, and the point-in-time
recovery capability. The operator creates databases on the server and
validates the declared capabilities. A consumer reads it in its own
namespace. Thus all the RDBMS resources of a cluster are in the namespace
of that cluster.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `DatabaseServerConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[DatabaseServerConfigSpec](#databaseserverconfigspec)_ | spec defines the desired state of DatabaseServerConfig |  | Required: \{\} <br /> |
| `status` _[DatabaseServerConfigStatus](#databaseserverconfigstatus)_ | status defines the observed state of DatabaseServerConfig |  | Optional: \{\} <br /> |


#### DatabaseServerConfigSpec



DatabaseServerConfigSpec describes a database server: the engine, the
endpoint, the admin credentials, and the point-in-time recovery
capability.



_Appears in:_
- [DatabaseServerConfig](#databaseserverconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `engine` _[DatabaseEngine](#databaseengine)_ | Engine is the database engine of the server. See DatabaseEngine for<br />the accepted values. |  | Enum: [postgres] <br /> |
| `host` _string_ | Host is the host name or the address of the server. |  | MinLength: 1 <br /> |
| `port` _integer_ | Port is the port of the server. |  | Maximum: 65535 <br />Minimum: 1 <br /> |
| `adminCredentialsSecretRef` _[LocalCredentialsSecretRef](#localcredentialssecretref)_ | AdminCredentialsSecretRef names an admin user that can create databases<br />and roles. The operator uses it to probe the server and to create the<br />database and the roles of a Database. The Secret is in the namespace of<br />this contract. |  |  |
| `pitr` _[PITRCapability](#pitrcapability)_ | PITR declares the point-in-time recovery capability of the server. |  | Optional: \{\} <br /> |
| `recovery` _[RecoveryRequest](#recoveryrequest)_ | Recovery asks for a rollback of the server to a point in time. A<br />consumer writes it. The answer is in pitr.lastRecovery, only when<br />pitr.recovery is operator. The request stays on the contract after the<br />answer, as the record of the last request. |  | Optional: \{\} <br /> |


#### DatabaseServerConfigStatus



DatabaseServerConfigStatus is the observed validation state of the contract.
It holds what the operator read from the server the last time that it
reached the server.



_Appears in:_
- [DatabaseServerConfig](#databaseserverconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `serverVersion` _string_ | ServerVersion is the major version that the server reported the last<br />time that the operator reached it, for example "17". A dump of a<br />database on this server uses client tools of this major version. Thus<br />a backup waits until the operator publishes it. |  | Optional: \{\} <br /> |
| `systemIdentifier` _string_ | SystemIdentifier is the identity of the PostgreSQL instance behind this<br />endpoint, as the server reported it on the last probe. It identifies<br />the server itself. Thus two contracts that describe one server with<br />different hosts publish the same value. The operator uses it to keep<br />each logical database unique. A change to the endpoint or to<br />adminCredentialsSecretRef clears it. |  | Optional: \{\} <br /> |
| `probedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | ProbedAt is when the operator last reached the server and read<br />ServerVersion and SystemIdentifier. The operator probes the server<br />again when this is older than the probe interval, or when the admin<br />credentials Secret changed. Between probes, it does not change.<br />A change to the endpoint or to adminCredentialsSecretRef clears this<br />field, and also ServerVersion, SystemIdentifier, ProbedEndpoint,<br />ProbedSecretName, ProbedSecretKeys, and ProbedSecretVersion. These<br />fields describe the server of the old spec. A change to another field,<br />for example a recovery request, does not clear them, because it cannot<br />move the server. |  | Optional: \{\} <br /> |
| `probedEndpoint` _string_ | ProbedEndpoint is the host and the port that the last probe reached, as<br />"<host>:<port>". With it, the operator knows whether a spec change<br />moves the server. |  | Optional: \{\} <br /> |
| `probedSecretName` _string_ | ProbedSecretName is the admin credentials Secret that the last probe<br />read. When the spec names another Secret, the operator clears the<br />record of the probe. |  | Optional: \{\} <br /> |
| `probedSecretKeys` _string_ | ProbedSecretKeys are the keys of that Secret that the last probe read,<br />as "<usernameKey>/<passwordKey>". One Secret can hold the credentials<br />of more than one user, so the keys identify the user. |  | Optional: \{\} <br /> |
| `probedSecretVersion` _string_ | ProbedSecretVersion is the resourceVersion of the admin credentials<br />Secret that the last probe used. When the Secret changes, the operator<br />probes again before the interval ends, so it validates new credentials<br />quickly. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current validation state. The Ready condition<br />has the reasons Healthy, MissingSecret, or ConnectionFailed. |  | Optional: \{\} <br /> |


#### DatabaseServerMonitoringSpec



DatabaseServerMonitoringSpec groups the Prometheus scraping integration of
a database server.



_Appears in:_
- [DatabaseServerSpec](#databaseserverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podMonitor` _[PodMonitorSpec](#podmonitorspec)_ | PodMonitor configures the Prometheus PodMonitor over the instance pods. |  | Optional: \{\} <br /> |


#### DatabaseServerPreset



DatabaseServerPreset is a cluster-scoped baseline configuration for
DatabaseServer resources. It has no controller, creates nothing, and
reports no status. A server reads it through its presetRef. A field set on
the server replaces the value of the preset for that field completely.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `DatabaseServerPreset` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[DatabaseServerPresetSpec](#databaseserverpresetspec)_ | spec defines the desired state of DatabaseServerPreset |  | Required: \{\} <br /> |


#### DatabaseServerPresetSpec



DatabaseServerPresetSpec defines the desired state of DatabaseServerPreset.



_Appears in:_
- [DatabaseServerPreset](#databaseserverpreset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `server` _[DatabaseServerSpec](#databaseserverspec)_ | Server is the complete configuration baseline that the servers that<br />reference the preset get. It uses the DatabaseServer spec type. A<br />preset must not set the fields that belong to one server only:<br />presetRef, releaseRef, databaseServerConfig, and suspend. It must also<br />not set the version, which belongs to a CamundaRelease. An explicit<br />zero value, such as an empty presetRef or suspend: false, counts as<br />unset. Templated YAML often renders unset fields in this way. A preset<br />can set archive like any other field. One bucket can serve many<br />servers, because every server writes under its own prefix. |  | Required: \{\} <br /> |


#### DatabaseServerRecoveryStatus



DatabaseServerRecoveryStatus is the recovery request that the server works
on now, or the last one that it answered. With it, a recovery can
continue after an interruption.

It holds the whole answer, not a reference to it. The server publishes the
answer on a contract. If somebody deletes the contract and creates it
again, the server publishes the answer again from this status.



_Appears in:_
- [DatabaseServerStatus](#databaseserverstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requestID` _string_ | RequestID is the requestID of the request, as the contract carries it. |  |  |
| `contract` _string_ | Contract is the DatabaseServerConfig that carried the request. The<br />server answers on that contract. Until it answers, it does not change<br />the contract that it publishes. |  |  |
| `requestedBy` _string_ | RequestedBy is the requestedBy of the request, as the contract carries<br />it. |  |  |
| `targetTime` _string_ | TargetTime is the targetTime of the request, as the contract carries it. |  |  |
| `cluster` _string_ | Cluster is the CloudNativePG cluster that the recovery builds. It is<br />empty for a request that the server refused, also when the server had<br />built a cluster before. |  | Optional: \{\} <br /> |
| `previousCluster` _string_ | PreviousCluster is the cluster that the contract pointed at before the<br />recovery moved it. If a recovery fails after the move, the contract<br />points at this cluster again, because it still holds the data. |  | Optional: \{\} <br /> |
| `archive` _[RecoveryArchiveRef](#recoveryarchiveref)_ | Archive is the archive that the recovery reads. The server records it<br />before the recovery builds anything. A spec change to another bucket<br />does not move a recovery that is already running. |  | Optional: \{\} <br /> |
| `result` _[RecoveryResult](#recoveryresult)_ | Result is the result that the server published for the request. It is<br />unset while the recovery runs. |  | Enum: [Completed Failed Unavailable] <br />Optional: \{\} <br /> |
| `message` _string_ | Message is the message that the server published with Result. |  | Optional: \{\} <br /> |
| `completedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | CompletedAt is when the server answered the request. It is unset while<br />the recovery runs. |  | Optional: \{\} <br /> |


#### DatabaseServerServiceAccountSpec



DatabaseServerServiceAccountSpec configures the ServiceAccount of the
instance pods. The operator creates it with the name <server name>-postgres.
Every CloudNativePG cluster of the server runs under it, also the cluster
that a rollback builds. The operator adds the workload-identity
annotations of the archive bucket. An annotation set here wins over the
operator annotation with the same key.



_Appears in:_
- [DatabaseServerSpec](#databaseserverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `annotations` _object (keys:string, values:string)_ | Annotations to set on the ServiceAccount. Usually these are<br />workload-identity annotations (IRSA, GCP Workload Identity, and more)<br />that give the instance pods access to cloud resources. |  | Optional: \{\} <br /> |


#### DatabaseServerSpec



DatabaseServerSpec defines the desired state of DatabaseServer.

A DatabaseServerPreset uses the same type as its baseline. For this reason,
the schema marks databaseServerConfig as optional, and the DatabaseServer
requires it. A preset refuses the fields that belong to one server only.
A preset also refuses the version, which belongs to a CamundaRelease.



_Appears in:_
- [DatabaseServer](#databaseserver)
- [DatabaseServerPresetSpec](#databaseserverpresetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `presetRef` _string_ | PresetRef names a cluster-scoped DatabaseServerPreset to use as the<br />configuration baseline. A field set on the server replaces the value of<br />the preset for that field completely. |  | Optional: \{\} <br /> |
| `releaseRef` _string_ | ReleaseRef names a cluster-scoped CamundaRelease that provides the<br />PostgreSQL major version. It merges over the preset and under this spec.<br />Forbidden in a preset. |  | Optional: \{\} <br /> |
| `platformConfigRef` _string_ | PlatformConfigRef names a cluster-scoped CamundaPlatformConfig. The<br />server reads only its image settings. spec.images.postgres sets where<br />the server pulls the PostgreSQL image from, for example in an air-gapped<br />cluster. When empty, the server uses the default image repository. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br />Optional: \{\} <br /> |
| `version` _string_ | Version is the PostgreSQL major version to run, as a number such as<br />"17". It selects the image tag. Camunda 8.9 supports PostgreSQL 14 and<br />later. The operator refuses a version below 14, also when it comes<br />from the release. Required unless the resolved release provides it.<br />Forbidden in a preset.<br />The major version of a running server cannot change. The operator<br />refuses another major version, higher or lower, with Ready reason<br />VersionChangeRefused. The server then keeps the major version it has.<br />The same applies to a new major version from a release. To run another<br />major version, create a new server and move the data to it. |  | Pattern: `^\d+$` <br />Optional: \{\} <br /> |
| `instances` _integer_ | Instances is the number of PostgreSQL instances. Defaults to 1. One<br />instance has no failover. If its node goes away, the server is down<br />until the volume is attached again. |  | Minimum: 1 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of each instance. |  | Optional: \{\} <br /> |
| `storageSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#quantity-resource-api)_ | StorageSize is the size of the data volume of each instance. Required<br />unless the resolved preset provides it. It cannot shrink, because a<br />PostgreSQL data volume cannot become smaller in place. The API server<br />refuses a change of this field in a DatabaseServer to a smaller value.<br />A smaller value is accepted when the field was not set before, or when<br />a preset lowers the size. A server whose volumes are already larger<br />then keeps that size and records a StorageShrinkIgnored event. |  | Optional: \{\} <br /> |
| `storageClassName` _string_ | StorageClassName is the StorageClass of the data volumes. Defaults to<br />the default StorageClass of the Kubernetes cluster. |  | Optional: \{\} <br /> |
| `walStorageSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#quantity-resource-api)_ | WALStorageSize puts the write-ahead log on a separate volume of this<br />size. When unset, the log stays on the data volume. It cannot shrink,<br />as storageSize cannot. The server ignores a smaller value from a preset<br />in the same way.<br />You can add it to a running server, but you cannot remove it again.<br />CloudNativePG refuses a cluster that removes the volume. If the field<br />is cleared, on the server or in a preset, the volume keeps its size and<br />the server records a WALStorageKept event. |  | Optional: \{\} <br /> |
| `serviceAccount` _[DatabaseServerServiceAccountSpec](#databaseserverserviceaccountspec)_ | ServiceAccount configures the ServiceAccount of the instance pods. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling holds the scheduling constraints of the instance pods. When<br />set, it replaces the scheduling block of the preset, with no merge. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels applied to the instance pods. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations applied to the instance pods. |  | Optional: \{\} <br /> |
| `monitoring` _[DatabaseServerMonitoringSpec](#databaseservermonitoringspec)_ | Monitoring configures the Prometheus scraping integration. |  | Optional: \{\} <br /> |
| `databaseServerConfig` _string_ | DatabaseServerConfig names the DatabaseServerConfig that the operator<br />publishes in the namespace of this server. It holds the endpoint, the<br />admin credentials, and the point-in-time recovery capability of the<br />server. Required on a DatabaseServer, forbidden in a preset. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br />Optional: \{\} <br /> |
| `archive` _[DatabaseServerArchiveSpec](#databaseserverarchivespec)_ | Archive points the server at the bucket that holds its continuous<br />archive. Without it the server takes no part in point-in-time restore. |  | Optional: \{\} <br /> |
| `suspend` _boolean_ | Suspend stops the PostgreSQL instances and keeps their data volumes.<br />Defaults to false. The operator hibernates the CloudNativePG cluster:<br />it removes the instance pods and keeps the volumes. When you set the<br />field back to false, the instances start again on the same volumes. |  | Optional: \{\} <br /> |


#### DatabaseServerStatus



DatabaseServerStatus is the observed state of a DatabaseServer.



_Appears in:_
- [DatabaseServer](#databaseserver)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `version` _string_ | Version is the PostgreSQL major version that the server runs, as a<br />number such as "17". It comes from the merged spec, so it is correct<br />when the release or the server gives the version. While<br />the operator refuses a version change, it shows the major version of the<br />data directory. It is empty until the operator resolves the references<br />of the server for the first time. |  | Optional: \{\} <br /> |
| `cluster` _string_ | Cluster is the CloudNativePG cluster that the published contract points<br />at. It is the name of the server until a recovery replaces it. |  | Optional: \{\} <br /> |
| `systemIdentifier` _string_ | SystemIdentifier is the identity of the PostgreSQL instance behind the<br />contract, as CloudNativePG reports it. A recovery restores the<br />pg_control of its base backup, so the recovered instance keeps the<br />identity and this value does not change. A recovery replaces the<br />endpoint of the contract. |  | Optional: \{\} <br /> |
| `archive` _[DatabaseServerArchiveStatus](#databaseserverarchivestatus)_ | Archive is the observed state of the continuous archive of the server.<br />It is unset until the server writes an archive. A removal of<br />spec.archive does not clear it. The bucket still holds what the server<br />wrote, and a server that archives again can recover from it. |  | Optional: \{\} <br /> |
| `recovery` _[DatabaseServerRecoveryStatus](#databaseserverrecoverystatus)_ | Recovery is the recovery request that the server works on now, or the<br />last one it answered. The answer itself is published on the contract,<br />in spec.pitr.lastRecovery. |  | Optional: \{\} <br /> |
| `volumes` _[VolumeStatus](#volumestatus) array_ | Volumes lists the bound PersistentVolumeClaims of the current cluster<br />and the capacity that each one reports, sorted by name. A server with a<br />write-ahead log volume also shows that claim. A server that does not own<br />the cluster of its derived name shows no claims, because they belong<br />to the other cluster. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. Ready holds the reason of a<br />failed pre-check (InvalidReference, MissingSecret, CNPGNotInstalled,<br />BarmanPluginNotInstalled). Otherwise it follows the cluster, the<br />contract, and, when the server has an archive, the archive. The<br />per-component conditions (ClusterReady, ArchiveReady, ContractReady,<br />MonitoringReady) also appear here. MonitoringReady never affects Ready,<br />so a broken PodMonitor does not make the server not ready. ArchiveReady<br />without spec.archive does not affect Ready either. |  | Optional: \{\} <br /> |


#### DatabaseSpec



DatabaseSpec defines the desired state of Database.



_Appears in:_
- [Database](#database)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serverRef` _string_ | ServerRef names the DatabaseServerConfig, in this namespace, of the<br />server on which the operator creates the database. |  | MinLength: 1 <br /> |
| `databaseName` _string_ | DatabaseName is the name of the logical database to create, as a valid<br />PostgreSQL identifier. It must be unique on each PostgreSQL instance<br />that a contract reaches, not only on each contract. The operator<br />refuses a Database with the same databaseName as a Database of any<br />namespace on the same instance. |  | Pattern: `^[a-z_][a-z0-9_]\{0,62\}$` <br /> |
| `applicationCredentials` _[CredentialsSpec](#credentialsspec)_ | ApplicationCredentials configures the application credentials Secret.<br />The operator always creates it. The Secret name defaults to<br /><CR name>-credentials. |  | Optional: \{\} <br /> |
| `backupCredentials` _[BackupCredentialsSpec](#backupcredentialsspec)_ | BackupCredentials configures the backup credentials Secret. The<br />operator creates it unless it is disabled. The Secret name defaults to<br /><CR name>-backup-credentials. |  | Optional: \{\} <br /> |
| `databaseConfig` _string_ | DatabaseConfig names the DatabaseConfig that the operator creates in<br />the namespace of this Database. Defaults to the CR name. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br />Optional: \{\} <br /> |
| `secondaryStorageConfig` _string_ | SecondaryStorageConfig, when set, makes the operator also create a<br />SecondaryStorageConfig of type rdbms with this name in the namespace of<br />this Database. It references the DatabaseConfig. Omit it for a database<br />that is not Camunda secondary storage (Keycloak, Identity, Web<br />Modeler). |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br />Optional: \{\} <br /> |


#### DatabaseStatus



DatabaseStatus is the observed state of a Database.



_Appears in:_
- [Database](#database)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `collisionKey` _string_ | CollisionKey is the logical database that this Database last resolved:<br />the system identifier of the server and the database name. Every<br />Database records it, also a Database that loses the name to another.<br />Thus the field shows what a Database asked for, not what it owns. The<br />operator sets it only after it reaches the server. If the spec names a<br />missing server, the old key stays until that server answers. The same<br />applies to a server that is not probed for the current spec.<br />A Database whose Ready condition reports InvalidReference and names<br />another Database does not own the name that it shows here. The<br />operator never clears the field. Thus an owner whose server or<br />contract is gone keeps the logical database. Delete that Database to<br />release the name. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. Ready holds the reason of a<br />failed pre-check (InvalidReference, MissingSecret,<br />ServerIdentityUnknown, ConnectionFailed). Otherwise it takes the status<br />and the reason of the BindingsReady condition, which also appears here. |  | Optional: \{\} <br /> |


#### DumpPodSpec



DumpPodSpec configures the pod of the Job that dumps the logical database
of a relational cluster and uploads it to the backup bucket. It holds all
the settings that a backup can set for one run. The pod runs the dump and
then the upload, so one resource block sizes both. It never names the
image. BackupDumpSpec gives the reason.



_Appears in:_
- [BackupDumpSpec](#backupdumpspec)
- [LogicalBackupRDBMSSpec](#logicalbackuprdbmsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the dump pod. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the dump pod.<br />In the spec.dump of a LogicalBackupRDBMS, every name that starts with<br />PG or UPLOAD_ is reserved, and the API server refuses it. A PG* name is<br />connection policy, because libpq reads PGHOSTADDR, PGSERVICE,<br />PGOPTIONS, and more. UPLOAD_* is the upload contract. The variables of<br />a backup reach only the dump container, never the upload container.<br />Cloud SDKs read endpoint, proxy, and configuration variables from the<br />environment, and the author of a backup must not change where the dump<br />goes.<br />In the spec.backup.dump of the cluster, no name is reserved, and the<br />variables reach every container. The cluster owner sets the connection<br />policy, PGSSLMODE included.<br />Unlike the extraEnv of a workload, this list is atomic. One field<br />manager owns the whole list, because no extension adds entries to a<br />dump pod. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources of the dump pod. In the<br />spec.dump of a LogicalBackupRDBMS, every source needs a prefix that<br />cannot make a PG* or UPLOAD_* name. The writer of the referenced object<br />chooses its keys. As with ExtraEnv, the sources of a<br />backup reach only the dump container. The block of the cluster needs no<br />prefix, and its sources reach every container. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the dump pod. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the dump pod. A service mesh<br />sidecar keeps running after the dump finishes, and the Job does not<br />complete. Set the injection annotation of the mesh to false here. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling holds the scheduling constraints of the dump pod. When set,<br />it replaces the block of a preset, with no merge. |  | Optional: \{\} <br /> |
| `scratchVolume` _[ScratchVolumeSpec](#scratchvolumespec)_ | ScratchVolume is where the pod writes the dump before the upload. When<br />set, it replaces the block of a preset, with no merge. The dump pod<br />runs with fsGroup 999, the postgres group. Thus pg_dump can write to a<br />volume that a storage class gives with the root owner. |  | Optional: \{\} <br /> |
| `activeDeadlineSeconds` _integer_ | ActiveDeadlineSeconds is the number of seconds that the dump Job can<br />run before it fails, counted from its start. When the dump block that<br />applies does not set it, the Job gets 86400 (24 hours). In the block of<br />the cluster, an unset value takes the value of the preset. The<br />spec.dump of a LogicalBackupRDBMS replaces the block of the cluster, so<br />an unset value there gives 86400, not the value of the cluster.<br />The default is large, so that a very large dump can complete. The Job<br />always has a deadline. A pod that cannot start uses no retry, so<br />without a deadline a broken Job stays active for the life of the<br />backup. A lower value fails a stuck dump sooner. A higher value gives a<br />long dump the time that it needs. |  | Minimum: 1 <br />Optional: \{\} <br /> |


#### ElasticsearchCluster



ElasticsearchCluster runs an Elasticsearch cluster for secondary storage,
through the external ECK operator. It publishes the connection details and
generated credentials as a SecondaryStorageConfig.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `ElasticsearchCluster` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[ElasticsearchClusterSpec](#elasticsearchclusterspec)_ | spec defines the desired state of ElasticsearchCluster |  | Required: \{\} <br /> |
| `status` _[ElasticsearchClusterStatus](#elasticsearchclusterstatus)_ | status defines the observed state of ElasticsearchCluster |  | Optional: \{\} <br /> |


#### ElasticsearchClusterPreset



ElasticsearchClusterPreset is a cluster-scoped baseline configuration for
ElasticsearchCluster resources. It has no controller, creates nothing, and
reports no status. A cluster reads it through its presetRef. A field set on
the cluster replaces the value of the preset for that field completely.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `ElasticsearchClusterPreset` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[ElasticsearchClusterPresetSpec](#elasticsearchclusterpresetspec)_ | spec defines the desired state of ElasticsearchClusterPreset |  | Required: \{\} <br /> |


#### ElasticsearchClusterPresetSpec



ElasticsearchClusterPresetSpec defines the desired state of
ElasticsearchClusterPreset.



_Appears in:_
- [ElasticsearchClusterPreset](#elasticsearchclusterpreset)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `cluster` _[ElasticsearchClusterSpec](#elasticsearchclusterspec)_ | Cluster is the complete configuration baseline that the clusters that<br />reference the preset get. It uses the ElasticsearchCluster spec type. A<br />preset must not set the fields that belong to one cluster only:<br />presetRef, releaseRef, secondaryStorageConfig, and suspend. It must<br />also not set the version, which belongs to a CamundaRelease. An explicit<br />zero value, such as an empty presetRef or suspend: false, counts as<br />unset. Templated YAML often renders unset fields in this way. A preset<br />can set monitoring like any other field. Thus it can enable scraping<br />and set the exporter image and resources for every cluster that<br />references it. |  | Required: \{\} <br /> |


#### ElasticsearchClusterSpec



ElasticsearchClusterSpec defines the desired state of ElasticsearchCluster.

An ElasticsearchClusterPreset uses the same type as its baseline. For this
reason, the schema marks secondaryStorageConfig as optional, and the
ElasticsearchCluster requires it. A preset refuses the fields that belong
to one cluster only. A preset also refuses the version, which belongs to a
CamundaRelease.



_Appears in:_
- [ElasticsearchCluster](#elasticsearchcluster)
- [ElasticsearchClusterPresetSpec](#elasticsearchclusterpresetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `presetRef` _string_ | PresetRef names a cluster-scoped ElasticsearchClusterPreset to use as<br />the configuration baseline. A field set on the cluster replaces the<br />value of the preset for that field completely. |  | Optional: \{\} <br /> |
| `releaseRef` _string_ | ReleaseRef names a cluster-scoped CamundaRelease that provides the<br />Elasticsearch version. It merges over the preset and under this spec.<br />Forbidden in a preset. |  | Optional: \{\} <br /> |
| `version` _string_ | Version is the Elasticsearch version to deploy, as a full semantic<br />version. Camunda 8.9 supports Elasticsearch 8.x from 8.19, and 9.x from<br />9.2. The operator refuses a version below 8.19, and 9.0 and 9.1, also<br />when it comes from the release, with Ready reason InvalidReference. Required unless the<br />resolved release provides it. Forbidden in a preset. |  | Pattern: `^\d+\.\d+\.\d+$` <br />Optional: \{\} <br /> |
| `replicas` _integer_ | Replicas is the number of Elasticsearch nodes. Required unless the<br />resolved preset provides it. |  | Minimum: 1 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory for each Elasticsearch node. |  | Optional: \{\} <br /> |
| `storageSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#quantity-resource-api)_ | StorageSize is the size of the data volume of each node. Required<br />unless the resolved preset provides it. It cannot shrink, because an<br />Elasticsearch data volume cannot become smaller in place. The API<br />server refuses a change of this field in an ElasticsearchCluster to a<br />smaller value. A smaller value is accepted when the field was not set<br />before, or when a preset lowers the size. A cluster whose volumes are<br />already larger then keeps that size and records a StorageShrinkIgnored<br />event. |  | Optional: \{\} <br /> |
| `storageClassName` _string_ | StorageClassName is the StorageClass of the data volumes. Defaults to<br />the default StorageClass of the Kubernetes cluster. The class cannot<br />change after the ECK Elasticsearch resource exists. A cluster whose ECK<br />resource exists keeps its class and records a<br />StorageClassChangeIgnored event. A cluster without an ECK resource<br />takes the new class, also a suspended cluster when it resumes. Volumes<br />that a suspension or whenDeleted Retain kept keep their class. |  | Optional: \{\} <br /> |
| `serviceAccount` _[ServiceAccountSpec](#serviceaccountspec)_ | ServiceAccount configures the ServiceAccount of the Elasticsearch pods. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables for every Elasticsearch node. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) for<br />every Elasticsearch node. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels applied to the Elasticsearch pods. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations applied to the Elasticsearch pods. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling holds the scheduling constraints of the Elasticsearch pods.<br />When set, it replaces the scheduling block of the preset, with no merge. |  | Optional: \{\} <br /> |
| `snapshotStorageRef` _string_ | SnapshotStorageRef names an ObjectStorageConfig, in the namespace of<br />this cluster, for the bucket of the snapshot repository of this<br />cluster. When it is set, the operator does all the Elasticsearch work<br />for that bucket. It gives the nodes their credentials, registers the<br />repository, and publishes the repository name in the<br />SecondaryStorageConfig. A CamundaCluster on this storage needs it for<br />backups. The CamundaCluster must reference the same bucket. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br />Optional: \{\} <br /> |
| `secureSettings` _[SecureSettingsSource](#securesettingssource) array_ | SecureSettings are Secrets that ECK loads into the keystore of every<br />node. The operator adds the credentials of snapshotStorageRef to the<br />keystore itself. Use this field for all other keystore entries. |  | Optional: \{\} <br /> |
| `secondaryStorageConfig` _string_ | SecondaryStorageConfig names the SecondaryStorageConfig that the<br />operator creates in the namespace of this cluster. It holds the<br />connection details and the generated credentials. Required on an<br />ElasticsearchCluster, forbidden in a preset. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br />Optional: \{\} <br /> |
| `monitoring` _[MonitoringSpec](#monitoringspec)_ | Monitoring configures the Prometheus scraping integration. |  | Optional: \{\} <br /> |
| `persistentVolumeClaimRetentionPolicy` _[PersistentVolumeClaimRetentionPolicy](#persistentvolumeclaimretentionpolicy)_ | PersistentVolumeClaimRetentionPolicy says what happens to the data<br />volumes when the ElasticsearchCluster is deleted. Suspension always<br />keeps them. |  | Optional: \{\} <br /> |
| `suspend` _boolean_ | Suspend stops the Elasticsearch cluster and keeps its data volumes.<br />Defaults to false. The operator deletes the ECK Elasticsearch resource<br />and keeps the volumes. When you set the field back to false, the<br />operator creates the resource again, and ECK attaches the volumes<br />again. |  | Optional: \{\} <br /> |


#### ElasticsearchClusterStatus



ElasticsearchClusterStatus is the observed state of an ElasticsearchCluster.



_Appears in:_
- [ElasticsearchCluster](#elasticsearchcluster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `version` _string_ | Version is the effective Elasticsearch version of the cluster, as a<br />full semantic version. It comes from the merged spec, so it is correct<br />when the release or the cluster gives the version. During<br />an upgrade, or when the operator cannot apply the change, the old<br />version can still run. It is empty<br />until the operator resolves the references of the cluster for the first<br />time. |  | Optional: \{\} <br /> |
| `volumes` _[VolumeStatus](#volumestatus) array_ | Volumes lists the bound data PersistentVolumeClaims of the cluster and<br />the capacity that each one reports, sorted by name. |  | Optional: \{\} <br /> |
| `snapshotRepository` _string_ | SnapshotRepository is the snapshot repository that the operator<br />registered in Elasticsearch for this cluster. The published<br />SecondaryStorageConfig holds the same name. It is empty until the first<br />registration succeeds. It shows the last registration that succeeded,<br />not a new check that the repository still exists. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. Ready holds the reason of a<br />failed pre-check (InvalidReference, MissingSecret, ECKNotInstalled).<br />Otherwise it follows the component conditions and, when the cluster or<br />its preset sets snapshotStorageRef and the cluster is not suspended,<br />SnapshotRepositoryReady. The per-component conditions (CredentialsReady,<br />KeystoreReady, ElasticsearchReady, StorageContractReady) also appear<br />here.<br />MetricsReady reports the exporter and never affects Ready. |  | Optional: \{\} <br /> |


#### ElasticsearchStorage



ElasticsearchStorage holds Elasticsearch connection details.



_Appears in:_
- [SecondaryStorageConfigSpec](#secondarystorageconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `endpoint` _string_ | Endpoint is the HTTP(S) endpoint of the Elasticsearch cluster. |  |  |
| `credentialsSecretRef` _[LocalCredentialsSecretRef](#localcredentialssecretref)_ | CredentialsSecretRef names a basic-auth user with read and write<br />access to the Camunda indices. |  |  |
| `caSecretRef` _[LocalSecretKeyRef](#localsecretkeyref)_ | CASecretRef names the CA bundle that consumers use to verify the TLS<br />certificate of the endpoint. Set it when a well-known CA did not sign<br />the certificate of the endpoint. An example is the self-signed<br />certificate of a cluster that ECK runs. Omit it for an endpoint with a<br />publicly trusted certificate. It is valid only with an https endpoint. |  | Optional: \{\} <br /> |
| `snapshotRepository` _string_ | SnapshotRepository names the snapshot repository in this Elasticsearch<br />cluster that backups write to. An ElasticsearchCluster with a<br />snapshotStorageRef registers the repository and sets this field in the<br />contract that it publishes. For an Elasticsearch cluster that this<br />operator does not manage, register the repository yourself and then<br />set this field. A cluster that takes backups needs it. Without it, the<br />backup components have no place to write. The name is part of a URL<br />path of the Elasticsearch API, so it permits only a small set of<br />characters. |  | MaxLength: 253 <br />Pattern: `^[a-zA-Z0-9][a-zA-Z0-9._-]*$` <br />Optional: \{\} <br /> |
| `nodeCount` _integer_ | NodeCount is the number of data nodes of the Elasticsearch cluster. An<br />ElasticsearchCluster sets it in the contract that it publishes. For an<br />Elasticsearch cluster that this operator does not manage, set it<br />yourself.<br />A consumer that sets no index replica count of its own gets 0 replicas<br />on one node and 1 replica on two or more nodes. Without a node count,<br />the consumer keeps the default of the Camunda application. |  | Minimum: 1 <br />Optional: \{\} <br /> |


#### ExporterSpec



ExporterSpec tunes the elasticsearch_exporter Deployment that
ServiceMonitorSpec.Enabled deploys.



_Appears in:_
- [MonitoringSpec](#monitoringspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Image overrides the exporter image. Defaults to the pinned<br />quay.io/prometheuscommunity/elasticsearch-exporter release of the<br />operator. |  | Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the exporter container. |  | Optional: \{\} <br /> |


#### ExternalKeycloakSpec



ExternalKeycloakSpec connects Management Identity to a Keycloak that you
run.



_Appears in:_
- [IdentityProviderSpec](#identityproviderspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `url` _string_ | URL is the URL of Keycloak, with the /auth path when Keycloak has one.<br />Management Identity uses this URL, so it must resolve from inside the<br />Kubernetes cluster.<br />The operator appends /realms/<realm> to this URL, so the URL has no<br />query and no fragment. The URL is also in the annotations of the Lease<br />that claims the realm, so its length has a limit.<br />The URL has no user and no password. The operator does not support a<br />Keycloak behind a proxy that needs basic authentication. |  | MaxLength: 2048 <br /> |
| `realm` _string_ | Realm is the realm that Management Identity creates and uses. Empty<br />means camunda-platform. The realm is part of the issuer, the token, and<br />the JWKS URLs that Management Identity builds. Thus it holds only<br />letters, digits, dots, hyphens, and underscores, and its length has a<br />limit, as the URL has. |  | MaxLength: 255 <br />Optional: \{\} <br /> |
| `adminCredentialsSecretRef` _[LocalCredentialsSecretRef](#localcredentialssecretref)_ | AdminCredentialsSecretRef names the Secret with the Keycloak<br />administrator credentials. Management Identity uses them to create the<br />realm, the clients, and the initial administrator. |  |  |
| `caBundleSecretRef` _[LocalSecretKeyRef](#localsecretkeyref)_ | CABundleSecretRef names the Secret key that holds the certificate<br />authority of Keycloak, in PEM form. The operator signs in to Keycloak to<br />register the login callbacks of Optimize. It then trusts this authority<br />and the trust store of its own image. Set it when a public authority did<br />not sign the certificate of Keycloak. It is valid only with an https<br />url. |  | Optional: \{\} <br /> |


#### GCSCredentials



GCSCredentials holds the static key of a GCS bucket.



_Appears in:_
- [GCSStorageAuth](#gcsstorageauth)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretRef` _[LocalSecretKeyRef](#localsecretkeyref)_ | SecretRef names the Secret key that holds the service-account JSON<br />key. |  |  |


#### GCSStorage



GCSStorage describes a Google Cloud Storage bucket.



_Appears in:_
- [ObjectStorageConfigSpec](#objectstorageconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bucketName` _string_ | BucketName is the name of the bucket, as storage client SDKs use it. |  | MinLength: 1 <br /> |
| `basePath` _string_ | BasePath is the key prefix under which consumers write objects,<br />without leading or trailing slashes. Empty means the bucket root. |  | Pattern: `^[^/]+(/[^/]+)*$` <br />Optional: \{\} <br /> |
| `auth` _[GCSStorageAuth](#gcsstorageauth)_ | Auth selects how consumers authenticate. An absent block means<br />workload identity through the ServiceAccount. | \{ type:workloadIdentity \} | Optional: \{\} <br /> |


#### GCSStorageAuth



GCSStorageAuth selects how consumers authenticate against a GCS bucket.



_Appears in:_
- [GCSStorage](#gcsstorage)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ObjectStorageAuthType](#objectstorageauthtype)_ | Type is the authentication choice. Defaults to workloadIdentity. | workloadIdentity | Enum: [workloadIdentity credentials] <br />Optional: \{\} <br /> |
| `workloadIdentity` _[GCSWorkloadIdentity](#gcsworkloadidentity)_ | WorkloadIdentity names the trusted principal. It is valid only with<br />type workloadIdentity. With an empty or absent block, the operator adds<br />no annotation. The identity must then be bound on the cloud side, for<br />example with Workload Identity Federation for GKE. |  | Optional: \{\} <br /> |
| `credentials` _[GCSCredentials](#gcscredentials)_ | Credentials is a static service-account key. Required with type<br />credentials. Forbidden with other types. |  | Optional: \{\} <br /> |


#### GCSWorkloadIdentity



GCSWorkloadIdentity names the Google principal that the bucket trusts. With
an empty block, the operator adds no annotation to the ServiceAccount of the
consumer. The identity must then be bound on the cloud side, for example
with Workload Identity Federation for GKE.



_Appears in:_
- [GCSStorageAuth](#gcsstorageauth)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serviceAccountEmail` _string_ | ServiceAccountEmail is the Google service account that consumers<br />impersonate. When set, the operator puts it in the<br />iam.gke.io/gcp-service-account annotation of a ServiceAccount that it<br />creates for the consumer. On an existing ServiceAccount, add the<br />annotation yourself. |  | Optional: \{\} <br /> |


#### GatewayBinding



GatewayBinding is the published in-cluster address of the client APIs of a
cluster. A client library takes the gRPC address as a host and a port, and
the REST address as a base URL. Each field holds the form that a client
takes.



_Appears in:_
- [CamundaClusterStatus](#camundaclusterstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `grpcEndpoint` _string_ | GRPCEndpoint is the host and the port of the gRPC API, for example<br />my-cluster-gateway.my-cluster-ns.svc:26500. |  |  |
| `restEndpoint` _string_ | RESTEndpoint is the base URL of the orchestration cluster REST API, for<br />example http://my-cluster-gateway.my-cluster-ns.svc:8080. |  |  |


#### GatewaySpec



GatewaySpec configures the gateway.



_Appears in:_
- [CamundaClusterSpec](#camundaclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `mode` _[ComponentMode](#componentmode)_ | Mode selects a Deployment of the unified binary (Standalone) or the<br />embedded gateway of the brokers (Embedded). Defaults to Standalone.<br />The workload fields have an effect only in Standalone mode. The<br />exception is extraEnv and extraEnvFrom: in Embedded mode, they apply to<br />the brokers. |  | Enum: [Standalone Embedded] <br />Optional: \{\} <br /> |
| `replicas` _integer_ | Replicas is the number of pods of this process. Defaults to 1. On a<br />CamundaCluster it has no effect on an embedded web application. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the container of this process. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the container of this<br />process. On a CamundaCluster, an entry here wins over a top-level entry<br />with the same name. The entries of an embedded web application apply to<br />its host process.<br />The list merges by name under server-side apply. Each field manager owns<br />only the entries that it applies, so an extension controller can add its<br />own entry next to yours. One applied manifest cannot hold two entries<br />with the same name.<br />Two field managers that apply the same name do not conflict, because the<br />merge is per field inside the entry. One manager can own value and the<br />other valueFrom. A container rejects an entry that has both, so the API<br />server refuses to store such an entry. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the<br />container of this process. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the pods of this process. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the pods of this process. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling holds the scheduling constraints of the pods of this process.<br />When set, it replaces all other scheduling blocks for this process, with<br />no merge. On a CamundaCluster, these are the top-level block and the<br />block of a preset. |  | Optional: \{\} <br /> |


#### IdentityAdminSpec



IdentityAdminSpec names the first administrator of the management plane.
Management Identity reads it only on the first start and stores the result
in its database.

In the oidc mode, the administrator is a claim of the tokens that the
provider issues. Set claimName and claimValue. A later change of the claim
shows ImmutableAfterStart. In the two Keycloak modes, the administrator is
the first Keycloak user. Set username. A later change creates a second
user, and the first user keeps its access.



_Appears in:_
- [IdentitySpec](#identityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `claimName` _string_ | ClaimName is the token claim that identifies the administrator, for<br />example oid or sub. Set it in the oidc mode. The operator records the<br />claim as <claimName>=<claimValue> and splits it at the first equals<br />sign. Thus the name cannot hold an equals sign. |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `claimValue` _string_ | ClaimValue is the value of the claim for the administrator.<br />Set it in the oidc mode. |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `username` _string_ | Username is the name of the first Keycloak user. Set it in the keycloak<br />and the externalKeycloak modes. Management Identity creates the user on<br />its first start. A later change of this field creates a second user and<br />does not rename the first user. |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `passwordSecretRef` _[LocalSecretKeyRef](#localsecretkeyref)_ | PasswordSecretRef names the Secret key that holds the password of the<br />first Keycloak user. The operator generates a password when this is<br />unset. |  | Optional: \{\} <br /> |
| `email` _string_ | Email is the email address of the first Keycloak user. Web Modeler<br />needs an address for every person who signs in. Thus it is required<br />when webModeler is set in a Keycloak mode. |  | MinLength: 3 <br />Optional: \{\} <br /> |


#### IdentityProviderSpec



IdentityProviderSpec holds one of the three identity provider modes.



_Appears in:_
- [CamundaManagementClusterSpec](#camundamanagementclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `keycloak` _[ManagedKeycloakSpec](#managedkeycloakspec)_ | Keycloak runs Keycloak through the Keycloak Operator. The Keycloak<br />Operator must be installed on the Kubernetes cluster. |  | Optional: \{\} <br /> |
| `externalKeycloak` _[ExternalKeycloakSpec](#externalkeycloakspec)_ | ExternalKeycloak connects Management Identity to a Keycloak that you<br />run. Management Identity still creates the realm, the clients, and the<br />initial administrator in it. |  | Optional: \{\} <br /> |
| `oidc` _[ManagementOIDCSpec](#managementoidcspec)_ | OIDC connects Management Identity to the identity provider of the<br />referenced CamundaPlatformConfig. |  | Optional: \{\} <br /> |


#### IdentitySpec



IdentitySpec configures Management Identity.

The operator owns the Keycloak URL and the realm of the container. Both
come from spec.identityProvider, so the API server refuses an extraEnv
entry for KEYCLOAK_URL or KEYCLOAK_REALM. In a Keycloak mode, this
management plane claims the realm that they name. With an override,
Management Identity writes the login callbacks of Optimize to another
realm. status.callbackRealm does not name that realm, the operator does
not remove the callbacks from it, and another management plane can hold
it.



_Appears in:_
- [CamundaManagementClusterSpec](#camundamanagementclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version is the Management Identity version, as a full semantic version.<br />The operator supports 8.9.0 and later. |  | Pattern: `^\d+\.\d+\.\d+$` <br /> |
| `externalUrl` _string_ | ExternalURL is the URL where browsers reach Management Identity.<br />Identity registers it as the redirect URI of its own client. |  |  |
| `databaseConfigRef` _string_ | DatabaseConfigRef names the DatabaseConfig of the Management Identity<br />database, in the namespace of this resource. Identity needs its own<br />PostgreSQL database. |  | MinLength: 1 <br /> |
| `admin` _[IdentityAdminSpec](#identityadminspec)_ | Admin names the first administrator of the management plane. |  |  |
| `replicas` _integer_ | Replicas is the number of pods of this process. Defaults to 1. On a<br />CamundaCluster it has no effect on an embedded web application. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the container of this process. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the container of this<br />process. On a CamundaCluster, an entry here wins over a top-level entry<br />with the same name. The entries of an embedded web application apply to<br />its host process.<br />The list merges by name under server-side apply. Each field manager owns<br />only the entries that it applies, so an extension controller can add its<br />own entry next to yours. One applied manifest cannot hold two entries<br />with the same name.<br />Two field managers that apply the same name do not conflict, because the<br />merge is per field inside the entry. One manager can own value and the<br />other valueFrom. A container rejects an entry that has both, so the API<br />server refuses to store such an entry. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the<br />container of this process. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the pods of this process. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the pods of this process. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling holds the scheduling constraints of the pods of this process.<br />When set, it replaces all other scheduling blocks for this process, with<br />no merge. On a CamundaCluster, these are the top-level block and the<br />block of a preset. |  | Optional: \{\} <br /> |


#### ImagesSpec



ImagesSpec changes the repositories of the container images that the
operator pulls. Each field holds a repository without a tag or a digest,
for example mirror.example.com/camunda/optimize. The version of the
component gives the tag. A repository name is lowercase, as the container
registries require. An unset field means the default repository of that
image.



_Appears in:_
- [CamundaPlatformConfigSpec](#camundaplatformconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `camunda` _string_ | Camunda is the image of the orchestration cluster processes. Defaults<br />to camunda/camunda. |  | Pattern: `^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]+(/[a-z0-9]+([._-][a-z0-9]+)*)+\|(/[a-z0-9]+([._-][a-z0-9]+)*)*)/?$` <br />Optional: \{\} <br /> |
| `connectors` _string_ | Connectors is the image of the connectors runtime. Defaults to<br />camunda/connectors-bundle. |  | Pattern: `^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]+(/[a-z0-9]+([._-][a-z0-9]+)*)+\|(/[a-z0-9]+([._-][a-z0-9]+)*)*)/?$` <br />Optional: \{\} <br /> |
| `optimize` _string_ | Optimize is the image of Optimize. Defaults to camunda/optimize. |  | Pattern: `^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]+(/[a-z0-9]+([._-][a-z0-9]+)*)+\|(/[a-z0-9]+([._-][a-z0-9]+)*)*)/?$` <br />Optional: \{\} <br /> |
| `identity` _string_ | Identity is the image of Management Identity. Defaults to<br />camunda/identity. |  | Pattern: `^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]+(/[a-z0-9]+([._-][a-z0-9]+)*)+\|(/[a-z0-9]+([._-][a-z0-9]+)*)*)/?$` <br />Optional: \{\} <br /> |
| `console` _string_ | Console is the image of Console. Defaults to camunda/console. |  | Pattern: `^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]+(/[a-z0-9]+([._-][a-z0-9]+)*)+\|(/[a-z0-9]+([._-][a-z0-9]+)*)*)/?$` <br />Optional: \{\} <br /> |
| `webModelerRestapi` _string_ | WebModelerRestapi is the image of the Web Modeler restapi process.<br />Defaults to camunda/web-modeler-restapi below 8.10, and to camunda/hub<br />from 8.10 on. |  | Pattern: `^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]+(/[a-z0-9]+([._-][a-z0-9]+)*)+\|(/[a-z0-9]+([._-][a-z0-9]+)*)*)/?$` <br />Optional: \{\} <br /> |
| `webModelerWebsockets` _string_ | WebModelerWebsockets is the image of the Web Modeler websockets<br />process. Defaults to camunda/web-modeler-websockets below 8.10, and to<br />camunda/hub-websockets from 8.10 on. |  | Pattern: `^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]+(/[a-z0-9]+([._-][a-z0-9]+)*)+\|(/[a-z0-9]+([._-][a-z0-9]+)*)*)/?$` <br />Optional: \{\} <br /> |
| `keycloak` _string_ | Keycloak is the image of the Keycloak that the operator runs. Defaults<br />to camunda/keycloak. |  | Pattern: `^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]+(/[a-z0-9]+([._-][a-z0-9]+)*)+\|(/[a-z0-9]+([._-][a-z0-9]+)*)*)/?$` <br />Optional: \{\} <br /> |
| `postgres` _string_ | Postgres is the image of the PostgreSQL that a DatabaseServer runs.<br />Defaults to ghcr.io/cloudnative-pg/postgresql. The tag is the major<br />version of the server, so a replacement must publish the same tags. |  | Pattern: `^[a-z0-9]([a-z0-9.-]*[a-z0-9])?(:[0-9]+(/[a-z0-9]+([._-][a-z0-9]+)*)+\|(/[a-z0-9]+([._-][a-z0-9]+)*)*)/?$` <br />Optional: \{\} <br /> |


#### KeycloakRealmTarget



KeycloakRealmTarget is one Keycloak realm and how the operator signs in to
it. Two targets name the same realm when their url and their realm are
equal. Their administrators and certificate authorities can differ.



_Appears in:_
- [CamundaManagementClusterStatus](#camundamanagementclusterstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `url` _string_ | URL is the URL of Keycloak that the operator uses, with the /auth path<br />when Keycloak has one. |  |  |
| `realm` _string_ | Realm is the Keycloak realm. |  |  |
| `adminCredentialsSecretRef` _[LocalCredentialsSecretRef](#localcredentialssecretref)_ | AdminCredentialsSecretRef names the Secret with the Keycloak<br />administrator that the operator signs in with, in the namespace of this<br />resource. |  |  |
| `caBundleSecretRef` _[LocalSecretKeyRef](#localsecretkeyref)_ | CABundleSecretRef names the Secret key with the certificate authority of<br />Keycloak, in the namespace of this resource. It is absent when a public<br />authority signed the certificate of Keycloak. |  | Optional: \{\} <br /> |


#### LocalCredentialsSecretRef



LocalCredentialsSecretRef references a username and password pair in a
Secret in the namespace of the object that holds the reference. Every
namespaced kind uses it, so a reference never reaches the credentials of
another namespace.



_Appears in:_
- [DatabaseConfigSpec](#databaseconfigspec)
- [DatabaseServerConfigSpec](#databaseserverconfigspec)
- [ElasticsearchStorage](#elasticsearchstorage)
- [ExternalKeycloakSpec](#externalkeycloakspec)
- [KeycloakRealmTarget](#keycloakrealmtarget)
- [ManagementAuth](#managementauth)
- [WebModelerMailSpec](#webmodelermailspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the Secret that holds the credentials. |  | MinLength: 1 <br /> |
| `usernameKey` _string_ | UsernameKey is the key in the Secret that holds the plaintext username. | username | MinLength: 1 <br />Optional: \{\} <br /> |
| `passwordKey` _string_ | PasswordKey is the key in the Secret that holds the plaintext password. | password | MinLength: 1 <br />Optional: \{\} <br /> |


#### LocalSecretKeyRef



LocalSecretKeyRef references one value in a Secret in the namespace of the
object that holds the reference. Every namespaced kind uses it, so a
reference never reaches the Secrets of another namespace.



_Appears in:_
- [AzureBlobCredentials](#azureblobcredentials)
- [ClusterAuthSpec](#clusterauthspec)
- [ElasticsearchStorage](#elasticsearchstorage)
- [ExternalKeycloakSpec](#externalkeycloakspec)
- [GCSCredentials](#gcscredentials)
- [IdentityAdminSpec](#identityadminspec)
- [KeycloakRealmTarget](#keycloakrealmtarget)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the Secret that holds the value. |  | MinLength: 1 <br /> |
| `key` _string_ | Key is the key in the Secret that holds the value. |  | MinLength: 1 <br /> |


#### LogicalBackupElasticsearch



LogicalBackupElasticsearch is one backup of a CamundaCluster with
Elasticsearch secondary storage. The backup is one set under one backup ID:
the web-application indices, the exported Zeebe record indices, and the
Zeebe partitions. The cluster continues to run during the backup, with
exporting soft-paused. A restore reads a completed backup by its backup ID
and its recorded snapshot names.

When you delete the resource, the operator tries to delete the stored
backup data. The deletion waits while the cluster publishes no management
binding, for example while it is suspended. It also waits while the pinned
bucket points elsewhere. The operator removes the resource and can leave
the data when the cluster is gone or was created again. The same applies
when the pinned bucket is gone. It also applies when the management client
cannot be built and the backup holds no pause of exporting.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `LogicalBackupElasticsearch` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[LogicalBackupElasticsearchSpec](#logicalbackupelasticsearchspec)_ | spec identifies the cluster to back up. It is immutable. A backup runs<br />one time. To try again, create a new resource. |  | Required: \{\} <br /> |
| `status` _[LogicalBackupElasticsearchStatus](#logicalbackupelasticsearchstatus)_ | status defines the observed state of the backup |  | Optional: \{\} <br /> |


#### LogicalBackupElasticsearchSpec



LogicalBackupElasticsearchSpec identifies the cluster to back up. The whole
spec is immutable. A backup runs one time. To try again, create a new
resource.



_Appears in:_
- [LogicalBackupElasticsearch](#logicalbackupelasticsearch)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clusterRef` _[ClusterRef](#clusterref)_ | ClusterRef references the CamundaCluster to back up, in the namespace<br />of this backup. Its secondary storage must be Elasticsearch. |  | Required: \{\} <br /> |


#### LogicalBackupElasticsearchStatus



LogicalBackupElasticsearchStatus is the progress of the backup to a final
phase.



_Appears in:_
- [LogicalBackupElasticsearch](#logicalbackupelasticsearch)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[LogicalBackupPhase](#logicalbackupphase)_ | Phase is the phase of the backup. Completed and Failed are final. |  | Enum: [Pending Running Completed Failed] <br />Optional: \{\} <br /> |
| `step` _[LogicalBackupElasticsearchStep](#logicalbackupelasticsearchstep)_ | Step is the current step of the backup. After an interruption, the<br />backup continues at this step. |  | Enum: [PauseExporting BackupHistory SnapshotRecords BackupRuntime ResumeExporting] <br />Optional: \{\} <br /> |
| `backupId` _integer_ | BackupID identifies every part of the backup set: the web-application<br />snapshots, the record snapshot, and the partition backup. A restore<br />finds the set by this ID. |  | Optional: \{\} <br /> |
| `partitionsCount` _integer_ | PartitionsCount is the partition count of the cluster when the backup<br />started. A restore must match it. |  | Optional: \{\} <br /> |
| `storageSizes` _[LogicalBackupStorageSizes](#logicalbackupstoragesizes)_ | StorageSizes are the effective restore sizes. The operator computes<br />them when the backup starts. If a value is not available then, the<br />operator adds it later while exporting runs: before the pause, or after<br />the resume. It does not add a value while exporting is paused. Thus an<br />absent value can still appear later in the backup. |  | Optional: \{\} <br /> |
| `history` _[BackupPart](#backuppart)_ | History is the backup of the web-application indices. |  | Optional: \{\} <br /> |
| `records` _[BackupPart](#backuppart)_ | Records is the snapshot of the exported Zeebe record indices. |  | Optional: \{\} <br /> |
| `runtime` _[BackupPart](#backuppart)_ | Runtime is the backup of the Zeebe partitions. |  | Optional: \{\} <br /> |
| `historySnapshots` _string array_ | HistorySnapshots names the Elasticsearch snapshots of the<br />web-application indices. The operator records the names as soon as the<br />management API gives them. Thus the deletion of the backup and a<br />restore can find the snapshots after the cluster is gone. |  | Optional: \{\} <br /> |
| `repository` _string_ | Repository records the snapshot repository of every part of the set.<br />The operator records it when the backup starts, and every later step<br />and the deletion use this name. Thus the whole set goes to one<br />repository, also when the storage contract changes its repository<br />during the backup. The deletion also goes to the correct repository. |  | Optional: \{\} <br /> |
| `storage` _[PinnedStorage](#pinnedstorage)_ | Storage records the Elasticsearch destination of the set: the storage<br />contract and its endpoint when the backup started. The repository name<br />alone does not identify a cluster. If the storage contract or the<br />endpoint changes during the backup, the step fails. The deletion never<br />runs against another cluster. |  | Optional: \{\} <br /> |
| `clusterUID` _string_ | ClusterUID records the identity of the CamundaCluster of the backup. A<br />cluster that is deleted and created again with the same name is<br />another cluster. This backup never paused the exporting of the new<br />cluster. Every management call after the start compares the cluster<br />with this UID. If they differ, the backup ends and does not change the<br />new cluster. |  | Optional: \{\} <br /> |
| `version` _string_ | Version is the Camunda version of the cluster when the backup started,<br />as the management binding reported it. A restore compares it with the<br />version of its target. An Elasticsearch backup restores only to the<br />same version. A restore can read the version only here, because a<br />suspended cluster has no management binding. |  | Optional: \{\} <br /> |
| `historyRequestedTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | HistoryRequestedTime is when the operator decided to request the<br />backup of the web-application indices. The operator writes it before<br />it sends the request, so the decision stays after a lost response or a<br />restart. It shows that this backup intended to send the request. It<br />does not prove that a history backup with this ID belongs to this<br />backup. |  | Optional: \{\} <br /> |
| `historyAcceptedTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | HistoryAcceptedTime is when the cluster accepted the history backup<br />request of this backup. Only this field shows that the history backup<br />with this ID belongs to this backup. If such a history backup exists<br />without this field, the step fails, and the operator does not delete<br />its snapshots. A crash between the request and the write of this field<br />fails the backup. The cluster can then keep a history backup with this<br />ID, which you remove manually. |  | Optional: \{\} <br /> |
| `runtimeRequestedTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | RuntimeRequestedTime is when the operator decided to request the<br />runtime backup. The operator writes it before it sends the request, so<br />the decision stays after a lost response or a restart. It shows that<br />this backup intended to send the request. It does not prove that a<br />runtime backup with this ID belongs to this backup. |  | Optional: \{\} <br /> |
| `runtimeAcceptedTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | RuntimeAcceptedTime is when the cluster accepted the runtime backup<br />request of this backup. Only this field shows that the runtime backup<br />with this ID belongs to this backup. A runtime backup can exist without<br />this field, after a lost response or when another client used the ID<br />first. Then the step fails, and the operator does not delete that<br />runtime backup. A crash between the request and the write of this<br />field fails the backup. The cluster can then keep such a runtime<br />backup, which you remove manually.<br />The cluster registers the backup some time after it accepts it. For a<br />short grace period after this time, the operator waits for an absent<br />backup. After the grace period, an absent backup fails the step. |  | Optional: \{\} <br /> |
| `unreachableSince` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | UnreachableSince is when a step first failed to reach its endpoint:<br />the management API or Elasticsearch. Exporting can be paused during<br />every step, so the retries have a time limit. After the limit, the step<br />fails, and the backup resumes exporting. The field clears when all<br />calls succeed again. |  | Optional: \{\} <br /> |
| `failureMessage` _string_ | FailureMessage names the failed step and its error. The operator<br />records it when a step fails and exporting must still resume. Thus the<br />final condition shows the reason after the resume. |  | Optional: \{\} <br /> |
| `resumeStartedTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | ResumeStartedTime is the start of the resume deadline. Only the time<br />of active resume attempts counts against the deadline. A time in which<br />the backup waits, for example for a suspended cluster or an<br />unpublished binding, moves the start forward and does not count. The<br />value stays after a restart of the operator. |  | Optional: \{\} <br /> |
| `lastResumeAttemptTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | LastResumeAttemptTime is when the last resume attempt ended. The time<br />from it to the start of the next attempt decides whether the start of<br />the deadline moves. The time inside an attempt always counts. |  | Optional: \{\} <br /> |
| `terminalReason` _string_ | TerminalReason is the Ready reason that the operator recorded when the<br />backup reached its final phase: Completed, Failed, or ResumeFailed. |  | Optional: \{\} <br /> |
| `resumeFailureMessage` _string_ | ResumeFailureMessage is the last error of the resume of exporting,<br />when the backup stopped the attempts. A backup that failed a step and<br />then failed to resume reports this field and FailureMessage. |  | Optional: \{\} <br /> |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | CompletionTime is when the backup reached a final phase. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. The Ready condition shows the<br />backup with the reasons Progressing, Completed, Failed, ResumeFailed,<br />ClusterSuspended, BackupInProgress, StorageTypeMismatch,<br />InvalidReference, MissingSecret, and ConnectionFailed. |  | Optional: \{\} <br /> |


#### LogicalBackupElasticsearchStep

_Underlying type:_ _string_

LogicalBackupElasticsearchStep is the current step of the backup. After a
crash or a restart of the operator, the backup continues at this step.
The request to pause exporting can be sent again after a restart.

_Validation:_
- Enum: [PauseExporting BackupHistory SnapshotRecords BackupRuntime ResumeExporting]

_Appears in:_
- [LogicalBackupElasticsearchStatus](#logicalbackupelasticsearchstatus)

| Field | Description |
| --- | --- |
| `PauseExporting` |  |
| `BackupHistory` |  |
| `SnapshotRecords` |  |
| `BackupRuntime` |  |
| `ResumeExporting` |  |


#### LogicalBackupPhase

_Underlying type:_ _string_

LogicalBackupPhase is the phase of a backup that runs one time. Completed
and Failed are final. To try again, create a new CR.

_Validation:_
- Enum: [Pending Running Completed Failed]

_Appears in:_
- [LogicalBackupElasticsearchStatus](#logicalbackupelasticsearchstatus)
- [LogicalBackupRDBMSStatus](#logicalbackuprdbmsstatus)

| Field | Description |
| --- | --- |
| `Pending` | LogicalBackupPending means that the backup did not start its work. A<br />pre-check did not pass yet, or another backup of the same cluster runs.<br /> |
| `Running` | LogicalBackupRunning means that the backup runs.<br /> |
| `Completed` | LogicalBackupCompleted means that the backup finished and a restore can<br />use it.<br /> |
| `Failed` | LogicalBackupFailed means that the backup failed. The message of the<br />Ready condition names the failed step.<br /> |


#### LogicalBackupRDBMS



LogicalBackupRDBMS is one backup of an orchestration cluster with
relational secondary storage. It is a dump of the whole logical database in
the backup bucket, with one Zeebe backup. Camunda calls the Zeebe log and
snapshots its "primary storage", and the exported relational data its
"secondary storage". A Zeebe backup is the backup of the primary storage
that Camunda writes to the backup bucket, on a request through the
management API. A restore reads the exporter position from the restored
dump and selects the Zeebe backups that match it. Thus the pair is a
complete restore point.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `LogicalBackupRDBMS` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[LogicalBackupRDBMSSpec](#logicalbackuprdbmsspec)_ | spec defines the desired state of LogicalBackupRDBMS |  | Required: \{\} <br /> |
| `status` _[LogicalBackupRDBMSStatus](#logicalbackuprdbmsstatus)_ | status defines the observed state of LogicalBackupRDBMS |  | Optional: \{\} <br /> |


#### LogicalBackupRDBMSSpec



LogicalBackupRDBMSSpec identifies the cluster to back up. It is immutable.
A backup runs one time. To try again, create a new CR.



_Appears in:_
- [LogicalBackupRDBMS](#logicalbackuprdbms)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clusterRef` _[ClusterRef](#clusterref)_ | ClusterRef references the CamundaCluster to back up. The cluster must<br />store its data in a relational database and have a backupStorageRef. |  | Required: \{\} <br /> |
| `dump` _[DumpPodSpec](#dumppodspec)_ | Dump replaces all the pod settings of the spec.backup.dump block of<br />the cluster for this backup. When unset, the settings of the cluster<br />apply. The two never merge. The image of the dump is not one of these<br />settings. The Job runs under the ServiceAccount of the cluster, so the<br />image always comes from the block of the cluster.<br />The environment has limits too. The extraEnv of a backup cannot set a<br />name that starts with PG or UPLOAD_, and the API server refuses such a<br />name. Every extraEnvFrom source needs a prefix that cannot make such a<br />name. libpq prefers PGHOSTADDR over the PGHOST of the Job. Without the<br />limit, a source can send the dump, with the credentials of the Job, to<br />another host. The API server accepts a source without a safe prefix.<br />The backup then stays Pending with reason InvalidReference, and its Job<br />never starts.<br />The environment of this block reaches only the dump container, never<br />the upload container. Cloud SDKs read endpoint, proxy, and<br />configuration variables from the environment, and a backup must not<br />change where its dump goes. The block of the cluster has no prefix<br />limit, and its environment reaches every container. |  | Optional: \{\} <br /> |


#### LogicalBackupRDBMSStatus



LogicalBackupRDBMSStatus is the observed state of one backup operation.



_Appears in:_
- [LogicalBackupRDBMS](#logicalbackuprdbms)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[LogicalBackupPhase](#logicalbackupphase)_ | Phase is the phase of the backup. Completed and Failed are final. |  | Enum: [Pending Running Completed Failed] <br />Optional: \{\} <br /> |
| `step` _[LogicalBackupRDBMSStep](#logicalbackuprdbmsstep)_ | Step is the current step of the backup. After an interruption, the<br />backup continues at this step. |  | Enum: [Dumping ZeebeBackup] <br />Optional: \{\} <br /> |
| `backupId` _integer_ | BackupID identifies the dump object in the bucket. The operator sets<br />it one time, when the backup leaves Pending. |  | Optional: \{\} <br /> |
| `jobName` _string_ | JobName is the Job that dumps and uploads the database, while the Job<br />exists. It clears when the operator recorded the dump and removed the<br />Job. A failed Job stays until the backup is deleted, and its name stays<br />too. |  | Optional: \{\} <br /> |
| `objectKey` _string_ | ObjectKey is the full key of the dump in the backup bucket:<br /><basePath>/<namespace>/<cluster>/<backupId>/<uid>/camunda.dump. The<br />uid is the UID of this resource. Thus a backup id that is used again<br />never names the dump of another backup. |  | Optional: \{\} <br /> |
| `zeebeBackupId` _integer_ | ZeebeBackupID is the id of the Zeebe backup that the cluster generated<br />after the dump. It is unset until the operator requests that backup. |  | Optional: \{\} <br /> |
| `zeebeBackupRequestedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | ZeebeBackupRequestedAt is when the operator requested the Zeebe<br />backup. It limits how long the operator waits for the cluster to report<br />the backup. |  | Optional: \{\} <br /> |
| `workloadConfigHash` _string_ | WorkloadConfigHash records the configuration of Zeebe when the backup<br />started. It is the config hash of the Zeebe pod template. The operator<br />compares it with the current hash before it renders the dump Job, and<br />again before it requests the Zeebe backup. After the request, it does<br />not compare again. If the hash differs, the backup fails after the<br />grace period. The generation of the cluster is not sufficient, because a<br />change of a referenced object changes the hash but not the generation. |  | Optional: \{\} <br /> |
| `clusterUID` _[UID](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#uid-types-pkg)_ | ClusterUID records the CamundaCluster of the backup. A cluster that is<br />deleted and created again with the same name is another cluster, with<br />other primary storage. If the UID of the cluster changes, the backup<br />fails. Thus a dump never pairs with the Zeebe backup of another<br />cluster. |  | Optional: \{\} <br /> |
| `version` _string_ | Version is the Camunda version of the cluster when the backup started,<br />as the management binding reported it. A restore compares it with the<br />version of its target. A relational backup restores to the same Camunda<br />minor or to one minor newer. A restore can read the version only here,<br />because a suspended cluster has no management binding. |  | Optional: \{\} <br /> |
| `firstFailedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | FirstFailedAt is when a dependency of the running backup first stopped<br />resolving, or the management API first stopped answering. The grace<br />period starts at this time. It clears when the backup recovers. |  | Optional: \{\} <br /> |
| `bucketRef` _string_ | BucketRef records the ObjectStorageConfig through which the Job wrote<br />the dump. On deletion, the operator uses this contract, not the<br />current backupStorageRef of the cluster. It removes the object only<br />while the cluster and this contract exist, and the contract points to<br />the same place. For a bucket with static credentials, the credentials<br />Secret must also exist. |  | Optional: \{\} <br /> |
| `bucketLocation` _string_ | BucketLocation records where the Job wrote the object: the storage<br />type, the bucket, the base path, and the endpoint of the<br />ObjectStorageConfig at the start. On deletion, the operator removes the<br />object only while the contract still points there. If the contract<br />points somewhere else, the object stays. Thus the operator never<br />deletes an unrelated object with the same key. |  | Optional: \{\} <br /> |
| `bucketGeneration` _integer_ | BucketGeneration is the generation of the recorded ObjectStorageConfig<br />when the backup started. It is information only. BucketLocation<br />decides whether a deletion can run. |  | Optional: \{\} <br /> |
| `storageSizes` _[LogicalBackupStorageSizes](#logicalbackupstoragesizes)_ | StorageSizes are the effective restore sizes that the operator<br />recorded when the backup started. The RDBMS kind records only the Zeebe<br />size. |  | Optional: \{\} <br /> |
| `failureMessage` _string_ | FailureMessage tells why the backup failed. It is set with the Failed<br />phase. The Ready condition has the same message. |  | Optional: \{\} <br /> |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | CompletionTime is when the backup reached a final phase. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. The Ready condition has the<br />phase as its reason, and its message names a failed step. For a<br />Pending backup, Ready has the reason of the check that holds it, and<br />the message explains that check. |  | Optional: \{\} <br /> |


#### LogicalBackupRDBMSStep

_Underlying type:_ _string_

LogicalBackupRDBMSStep is the current step of the backup. After an
interruption, the backup continues at this step.

_Validation:_
- Enum: [Dumping ZeebeBackup]

_Appears in:_
- [LogicalBackupRDBMSStatus](#logicalbackuprdbmsstatus)

| Field | Description |
| --- | --- |
| `Dumping` | StepDumping runs the Job that writes the logical database to the<br />backup bucket.<br /> |
| `ZeebeBackup` | StepZeebeBackup requests one Zeebe backup directly after the dump, so<br />that the two make one restore point. A Zeebe backup is the backup that<br />Camunda takes of its primary storage: the Zeebe log and snapshots.<br /> |


#### LogicalBackupRef



LogicalBackupRef references a completed logical backup in the namespace of
the restore. The reference never crosses a namespace. The kind of the
restore sets the kind of the backup, so the reference holds only a name.



_Appears in:_
- [LogicalRestoreElasticsearchSpec](#logicalrestoreelasticsearchspec)
- [LogicalRestoreRDBMSSpec](#logicalrestorerdbmsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the backup, in the namespace of this restore. |  | MinLength: 1 <br />Required: \{\} <br /> |


#### LogicalBackupStorageSizes



LogicalBackupStorageSizes are the effective restore sizes of the
components that hold storage. The operator records them when a backup
starts, so that a restore can create volumes of the correct size. A value
that the operator cannot compute stays unset. An Elasticsearch backup can
add a missing value later, while exporting runs. The RDBMS kind never sets
Elasticsearch, because it does not back up Elasticsearch data.



_Appears in:_
- [LogicalBackupElasticsearchStatus](#logicalbackupelasticsearchstatus)
- [LogicalBackupRDBMSStatus](#logicalbackuprdbmsstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `elasticsearch` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#quantity-resource-api)_ | Elasticsearch is the effective restore size of one Elasticsearch data<br />volume. |  | Optional: \{\} <br /> |
| `zeebe` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#quantity-resource-api)_ | Zeebe is the effective restore size of one broker data volume. |  | Optional: \{\} <br /> |


#### LogicalRestoreElasticsearch



LogicalRestoreElasticsearch restores one completed
LogicalBackupElasticsearch into one suspended CamundaCluster. It deletes
the Camunda indices of the target and restores every snapshot of the backup
into its Elasticsearch. Then it gives the brokers empty data volumes and
runs the Camunda restore application one time for each broker.

The restore prepares the target itself. It suspends the target, waits for
its brokers to stop, and sets spec.version to the Camunda version of the
backup. When it completes, it removes the suspension, but only a
suspension that it applied itself. A failed restore leaves the target
suspended. A restore that somebody deletes while it runs also leaves the
target suspended. Empty or half-written broker volumes cause more damage
under running brokers.

The restore keeps spec.version, with the field manager
camunda-operator/restore-version. The target runs the version of the
backup until another manager takes over or removes that field. A manifest
without spec.version does not change it, because server-side apply
removes a field only for the manager that set it. This applies also to a
target that gets its version from a release. The value of the restore wins
over the release until somebody removes the field.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `LogicalRestoreElasticsearch` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[LogicalRestoreElasticsearchSpec](#logicalrestoreelasticsearchspec)_ | spec names the backup to restore and the cluster to restore into. It<br />is immutable. A restore runs one time. To try again, create a new<br />resource. |  | Required: \{\} <br /> |
| `status` _[LogicalRestoreElasticsearchStatus](#logicalrestoreelasticsearchstatus)_ | status defines the observed state of the restore |  | Optional: \{\} <br /> |


#### LogicalRestoreElasticsearchSpec



LogicalRestoreElasticsearchSpec names the backup to restore and the cluster
to restore into. The whole spec is immutable. A restore runs one time. To
try again, create a new resource.



_Appears in:_
- [LogicalRestoreElasticsearch](#logicalrestoreelasticsearch)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `backupRef` _[LogicalBackupRef](#logicalbackupref)_ | BackupRef references the completed LogicalBackupElasticsearch to<br />restore from, in the namespace of this restore. |  | Required: \{\} <br /> |
| `targetClusterRef` _[ClusterRef](#clusterref)_ | TargetClusterRef references the CamundaCluster to restore into. It<br />must name the cluster of the backup, because the restore application<br />reads the primary-storage backup under the prefix of that cluster. The<br />cluster must stay suspended for the whole restore. |  | Required: \{\} <br /> |


#### LogicalRestoreElasticsearchStatus



LogicalRestoreElasticsearchStatus is the progress of the restore to a
final phase.



_Appears in:_
- [LogicalRestoreElasticsearch](#logicalrestoreelasticsearch)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[LogicalRestorePhase](#logicalrestorephase)_ | Phase is the phase of the restore. After an interruption, the restore<br />continues at this phase. |  | Enum: [Pending ValidatingCompatibility RestoringSecondaryStorage RestoringPrimaryStorage Completed Failed] <br />Optional: \{\} <br /> |
| `backupId` _integer_ | BackupID is the backup that the restore reads. The operator records it<br />when the restore starts. Later phases of the restore still read the<br />backup resource, so keep it until the restore completes. |  | Optional: \{\} <br /> |
| `backend` _string_ | Backend is the Elasticsearch that the restore writes, as the scheme,<br />the host, and the port. The operator records it when the restore<br />starts. From then until the final phase, and after it while<br />recoveryHeld is true, no other CamundaCluster starts on this backend.<br />The restore waits while its target does not hold the backend. |  | Optional: \{\} <br /> |
| `contract` _string_ | Contract is the SecondaryStorageConfig that held the endpoint of<br />Backend when the restore started. When the endpoint of that contract<br />moves, the restore still holds this contract. |  | Optional: \{\} <br /> |
| `recoveryHeld` _boolean_ | RecoveryHeld is true while a failed or deleted restore keeps the<br />backend, because Elasticsearch can still recover snapshots that the<br />restore asked for. While it is true, no other CamundaCluster starts on<br />the backend, and the target stays suspended. No other backup or restore<br />of the target starts. A deleted restore stays while it is true. It is<br />unset on a restore that was never held, and false when the hold ends. |  | Optional: \{\} <br /> |
| `recoveryUnknownSince` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | RecoveryUnknownSince is the start of the current period in which a held<br />restore cannot read the recovery from Elasticsearch. If the recovery stays unknown for ten<br />minutes, the restore releases the backend. |  | Optional: \{\} <br /> |
| `repository` _string_ | Repository is the Elasticsearch snapshot repository that the restore<br />reads from, on the Elasticsearch of the target. |  | Optional: \{\} <br /> |
| `restoredSnapshots` _string array_ | RestoredSnapshots names every snapshot that the restore asked<br />Elasticsearch to restore. After the operator records the names, the<br />restore does not delete the indices again when it continues after an<br />interruption. |  | Optional: \{\} <br /> |
| `targetClusterUID` _[UID](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#uid-types-pkg)_ | TargetClusterUID records the identity of the target cluster. A cluster<br />that is deleted and created again with the same name is another<br />cluster, and this restore does not apply to it. |  | Optional: \{\} <br /> |
| `brokers` _integer_ | Brokers is the broker count of the broker StatefulSet. The operator<br />records it before the restore deletes a volume. It sets how many<br />volumes the restore creates again and how many Jobs run. |  | Optional: \{\} <br /> |
| `primaryJobNames` _string array_ | PrimaryJobNames are the restore application Jobs, one for each broker,<br />in broker order. The operator records them before it creates the Jobs.<br />A completed restore removes these Jobs. The logs of these Jobs explain<br />a failed restore. |  | Optional: \{\} <br /> |
| `recreatedClaims` _string array_ | RecreatedClaims names the broker data claims that the restore deleted<br />and created again. Thus the restore does not delete a claim two times. |  | Optional: \{\} <br /> |
| `firstFailedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | FirstFailedAt is when a dependency of the running restore first stopped<br />resolving. The grace period starts at this time. The field can clear if<br />the dependency recovers before the restore deletes target indices or<br />records a recreated broker volume or a broker Job. After that point, the<br />field stays set, so a dependency that fails and recovers again and<br />again does not reset the grace period. |  | Optional: \{\} <br /> |
| `clusterSuspended` _boolean_ | ClusterSuspended records that this restore suspended its target<br />cluster. The restore removes that suspension when it reaches<br />Completed. A cluster that its owner suspended has no such record, so<br />it stays suspended. The cluster of a failed restore also stays<br />suspended. |  | Optional: \{\} <br /> |
| `terminalReason` _string_ | TerminalReason is the Ready reason that the operator recorded when the<br />restore reached its final phase. |  | Optional: \{\} <br /> |
| `failureMessage` _string_ | FailureMessage names the failed phase and its error. The Ready<br />condition has the same message. |  | Optional: \{\} <br /> |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | CompletionTime is when the restore reached a final phase. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state of the restore. |  | Optional: \{\} <br /> |


#### LogicalRestorePhase

_Underlying type:_ _string_

LogicalRestorePhase is the phase of a logical restore that runs one time.
Completed and Failed are final. To try again, create a new resource. Both
logical restore kinds use it, because their phases are the same.

_Validation:_
- Enum: [Pending ValidatingCompatibility RestoringSecondaryStorage RestoringPrimaryStorage Completed Failed]

_Appears in:_
- [LogicalRestoreElasticsearchStatus](#logicalrestoreelasticsearchstatus)
- [LogicalRestoreRDBMSStatus](#logicalrestorerdbmsstatus)

| Field | Description |
| --- | --- |
| `Pending` | LogicalRestorePending means that the restore did not start its work. A<br />pre-check stops it: the target cluster runs, another operation holds<br />the cluster, or the backup is not completed.<br /> |
| `ValidatingCompatibility` | LogicalRestoreValidatingCompatibility means that the operator compares<br />the backup with the target. It compares the storage type, the backup<br />bucket, the Camunda version, and, on the Elasticsearch kind, the<br />partition count.<br /> |
| `RestoringSecondaryStorage` | LogicalRestoreRestoringSecondaryStorage means that the operator writes<br />the backup into the secondary storage of the target.<br /> |
| `RestoringPrimaryStorage` | LogicalRestoreRestoringPrimaryStorage means that the operator created<br />the broker data volumes again and runs the restore application on them.<br /> |
| `Completed` | LogicalRestoreCompleted means that the restore finished. The operator<br />removes the Jobs of the brokers, so their pods release the broker data<br />volumes. It removes the suspension that it applied, so the target runs<br />again, unless its owner suspended it.<br /> |
| `Failed` | LogicalRestoreFailed means that the restore failed. The Ready condition<br />names the failed phase. The operator keeps the Jobs of the brokers,<br />because their logs show the cause. Thus a restore that reached<br />RestoringPrimaryStorage holds the broker data volumes until you delete<br />it. A restore that failed in an earlier phase records no Job in<br />PrimaryJobNames and holds no broker volume.<br /> |


#### LogicalRestoreRDBMS



LogicalRestoreRDBMS restores one completed LogicalBackupRDBMS into one
suspended CamundaCluster. The operator writes the dump into the logical
database of the target with pg_restore. Then it gives the brokers empty
data volumes and runs the Camunda restore application on them, one time
for each broker.

The restore prepares the target itself. It suspends the target, waits for
its brokers to stop, and sets spec.version to the Camunda version of the
backup. When it completes, it removes the suspension, but only a
suspension that it applied itself. A failed restore leaves the target
suspended. A restore that somebody deletes while it runs also leaves the
target suspended. Empty or half-written broker volumes cause more damage
under running brokers.

The restore keeps spec.version, with the field manager
camunda-operator/restore-version. The target runs the version of the
backup until another manager takes over or removes that field. A manifest
without spec.version does not change it, because server-side apply
removes a field only for the manager that set it. Thus a target of a newer
minor stays on the minor of the backup, and the owner must upgrade it
again.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `LogicalRestoreRDBMS` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[LogicalRestoreRDBMSSpec](#logicalrestorerdbmsspec)_ | spec names the backup to read and the cluster to restore into. It is<br />immutable. A restore runs one time. To try again, create a new<br />resource. |  | Required: \{\} <br /> |
| `status` _[LogicalRestoreRDBMSStatus](#logicalrestorerdbmsstatus)_ | status defines the observed state of the restore |  | Optional: \{\} <br /> |


#### LogicalRestoreRDBMSSpec



LogicalRestoreRDBMSSpec names the backup to restore and the cluster to
restore into. The whole spec is immutable. A restore runs one time. To try
again, create a new resource.



_Appears in:_
- [LogicalRestoreRDBMS](#logicalrestorerdbms)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `backupRef` _[LogicalBackupRef](#logicalbackupref)_ | BackupRef references the completed LogicalBackupRDBMS to restore from,<br />in the namespace of this restore. |  | Required: \{\} <br /> |
| `targetClusterRef` _[ClusterRef](#clusterref)_ | TargetClusterRef references the CamundaCluster to restore into. It<br />must name the cluster of the backup, because the restore application<br />reads the primary-storage backup under the prefix of that cluster. The<br />cluster must be suspended for the whole restore. |  | Required: \{\} <br /> |


#### LogicalRestoreRDBMSStatus



LogicalRestoreRDBMSStatus is the progress of the restore to a final phase.



_Appears in:_
- [LogicalRestoreRDBMS](#logicalrestorerdbms)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[LogicalRestorePhase](#logicalrestorephase)_ | Phase is the phase of the restore. After an interruption, the restore<br />continues at this phase. |  | Enum: [Pending ValidatingCompatibility RestoringSecondaryStorage RestoringPrimaryStorage Completed Failed] <br />Optional: \{\} <br /> |
| `backupId` _integer_ | BackupID is the id of the dump that the restore reads, as the backup<br />records it. The operator records it when the restore starts. Later<br />phases of the restore still read the backup resource, so keep it until<br />the restore completes. |  | Optional: \{\} <br /> |
| `backend` _string_ | Backend is the logical database that the restore writes, as the host,<br />the port, and the database name. The operator records it when the<br />restore starts. From then until the final phase, no other<br />CamundaCluster starts on this backend. The restore waits while its<br />target does not hold the backend. |  | Optional: \{\} <br /> |
| `contract` _string_ | Contract is the DatabaseServerConfig and the database name of Backend<br />when the restore started. When the DatabaseServerConfig moves to<br />another address, the restore still holds this contract. |  | Optional: \{\} <br /> |
| `secondaryJobName` _string_ | SecondaryJobName is the Job that runs pg_restore, while the Job exists. |  | Optional: \{\} <br /> |
| `targetClusterUID` _[UID](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#uid-types-pkg)_ | TargetClusterUID records the identity of the target cluster. A cluster<br />that is deleted and created again with the same name is another<br />cluster, and this restore does not apply to it. |  | Optional: \{\} <br /> |
| `brokers` _integer_ | Brokers is the broker count of the broker StatefulSet. The operator<br />records it before the restore deletes a volume. It sets how many<br />volumes the restore creates again and how many Jobs run. |  | Optional: \{\} <br /> |
| `primaryJobNames` _string array_ | PrimaryJobNames are the restore application Jobs, one for each broker,<br />in broker order. The operator records them before it creates the Jobs.<br />A completed restore removes these Jobs. The logs of these Jobs explain<br />a failed restore. |  | Optional: \{\} <br /> |
| `recreatedClaims` _string array_ | RecreatedClaims names the broker data claims that the restore deleted<br />and created again. Thus the restore does not delete a claim two times. |  | Optional: \{\} <br /> |
| `firstFailedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | FirstFailedAt is when a dependency of the running restore first stopped<br />resolving. The grace period starts at this time. The field can clear if<br />the dependency recovers before the restore deletes target indices or<br />records a recreated broker volume or a broker Job. After that point, the<br />field stays set, so a dependency that fails and recovers again and<br />again does not reset the grace period. |  | Optional: \{\} <br /> |
| `clusterSuspended` _boolean_ | ClusterSuspended records that this restore suspended its target<br />cluster. The restore removes that suspension when it reaches<br />Completed. A cluster that its owner suspended has no such record, so<br />it stays suspended. The cluster of a failed restore also stays<br />suspended. |  | Optional: \{\} <br /> |
| `terminalReason` _string_ | TerminalReason is the Ready reason that the operator recorded when the<br />restore reached its final phase. |  | Optional: \{\} <br /> |
| `failureMessage` _string_ | FailureMessage names the failed phase and its error. The Ready<br />condition has the same message. |  | Optional: \{\} <br /> |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | CompletionTime is when the restore reached a final phase. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state of the restore. |  | Optional: \{\} <br /> |


#### ManagedKeycloakSpec



ManagedKeycloakSpec configures the Keycloak that the operator runs through
the Keycloak Operator.



_Appears in:_
- [IdentityProviderSpec](#identityproviderspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version is the Keycloak version, as a full semantic version. Camunda<br />8.9 supports only Keycloak 26. The image is<br />camunda/keycloak:quay-optimized-<version>, unless the platform config<br />sets another repository. |  | Pattern: `^\d+\.\d+\.\d+$` <br /> |
| `externalUrl` _string_ | ExternalURL is the URL where browsers reach Keycloak, with the /auth<br />path. It is the front-channel issuer of every token. Management<br />Identity uses the front-channel URL since 8.5.3, so the Identity pods<br />must also reach it. Management Identity administers Keycloak through<br />the Service that the Keycloak Operator creates, not through this URL.<br />The operator appends /realms/<realm> to this URL, so the URL has no<br />query and no fragment. |  |  |
| `databaseConfigRef` _string_ | DatabaseConfigRef names the DatabaseConfig of the Keycloak database, in<br />the namespace of this resource. Keycloak needs its own PostgreSQL<br />database. |  | MinLength: 1 <br /> |
| `replicas` _integer_ | Replicas is the number of Keycloak instances. Defaults to 1. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the Keycloak container. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling constraints of the Keycloak pods. |  | Optional: \{\} <br /> |


#### ManagementAuth



ManagementAuth is how a consumer of the management binding authenticates.



_Appears in:_
- [ManagementBinding](#managementbinding)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `method` _[ManagementAuthMethod](#managementauthmethod)_ | Method is the authentication method of the management port. Camunda<br />8.9 serves the actuator endpoints without authentication, so the<br />operator publishes none. The value basic is for a user who adds a<br />Spring Security configuration to the port, and for a later Camunda<br />version that secures it. |  | Enum: [none basic] <br /> |
| `credentialsSecretRef` _[LocalCredentialsSecretRef](#localcredentialssecretref)_ | CredentialsSecretRef names the username and password of the management<br />port. It is set only when Method is basic. |  | Optional: \{\} <br /> |


#### ManagementAuthConfig



ManagementAuthConfig is the contract CRD that holds the Management Identity
OIDC configuration: the endpoints, the client credentials, and the
audience. Components outside the orchestration cluster, such as Optimize,
read it.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `ManagementAuthConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[ManagementAuthConfigSpec](#managementauthconfigspec)_ | spec defines the desired state of ManagementAuthConfig |  | Required: \{\} <br /> |
| `status` _[ManagementAuthConfigStatus](#managementauthconfigstatus)_ | status defines the observed state of ManagementAuthConfig |  | Optional: \{\} <br /> |


#### ManagementAuthConfigSpec



ManagementAuthConfigSpec holds the Management Identity OIDC configuration:
the endpoints, the machine-to-machine client credentials, and the audience.



_Appears in:_
- [ManagementAuthConfig](#managementauthconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `baseUrl` _string_ | BaseURL is the base URL of the Management Identity service. |  |  |
| `issuerUrl` _string_ | IssuerURL is the OIDC issuer URL for the validation of tokens. |  |  |
| `issuerBackendUrl` _string_ | IssuerBackendURL is the issuer URL for calls between containers inside<br />the Kubernetes cluster. When empty, consumers use IssuerURL. |  | Optional: \{\} <br /> |
| `authUrl` _string_ | AuthURL is the OIDC authorization endpoint for the login redirects of<br />the browser. |  |  |
| `tokenUrl` _string_ | TokenURL is the OIDC token endpoint for machine-to-machine tokens. |  |  |
| `jwksUrl` _string_ | JwksURL is the JWKS endpoint that gives the token signing keys. |  |  |
| `clientId` _string_ | ClientID is the ID of the client that Optimize signs in with. |  | MinLength: 1 <br /> |
| `audience` _string_ | Audience is the audience that access tokens for this client must have. |  | MinLength: 1 <br /> |
| `clientSecretRef` _[SecretKeyRef](#secretkeyref)_ | ClientSecretRef names the Secret key that holds the secret of that<br />client. |  |  |


#### ManagementAuthConfigStatus



ManagementAuthConfigStatus is the observed validation state of the contract.



_Appears in:_
- [ManagementAuthConfig](#managementauthconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current validation state. The Ready condition<br />has the reasons Healthy or MissingSecret. |  | Optional: \{\} <br /> |


#### ManagementAuthMethod

_Underlying type:_ _string_

ManagementAuthMethod is how a consumer authenticates against the management
API of a cluster.

_Validation:_
- Enum: [none basic]

_Appears in:_
- [ManagementAuth](#managementauth)

| Field | Description |
| --- | --- |
| `none` |  |
| `basic` |  |


#### ManagementBinding



ManagementBinding is the published address of the management API of one
cluster. Extensions that operate the cluster, such as the backup kinds,
read it. They do not derive the Service name, the port, or the
authentication themselves. It is empty while the cluster is suspended,
because the management API is then not available.



_Appears in:_
- [CamundaClusterStatus](#camundaclusterstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `endpoint` _string_ | Endpoint is the base URL of the management API, for example<br />http://my-cluster-zeebe.my-namespace.svc:9600. |  |  |
| `auth` _[ManagementAuth](#managementauth)_ | Auth is how a consumer authenticates against the endpoint. |  |  |
| `version` _string_ | Version is the Camunda version of the cluster, for example 8.9.9. A<br />consumer selects its endpoint set from the minor version. |  |  |
| `partitions` _integer_ | Partitions is the partition count of the cluster. A restore must match<br />it, so a backup records it. |  |  |
| `backupRepository` _string_ | BackupRepository is the snapshot repository that the components write<br />backups to. It is set only on a cluster with Elasticsearch storage and a<br />backupStorageRef. It comes from the SecondaryStorageConfig. |  | Optional: \{\} <br /> |


#### ManagementClients



ManagementClients names the identity provider client of each component of
the management plane. A CamundaManagementCluster reports InvalidReference
when a component that it deploys has no client here.



_Appears in:_
- [ManagementOIDCClientsSpec](#managementoidcclientsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `identity` _[ConfidentialClientSpec](#confidentialclientspec)_ | Identity is the client of Management Identity. |  | Optional: \{\} <br /> |
| `optimize` _[ConfidentialClientSpec](#confidentialclientspec)_ | Optimize is the client of Optimize. The ManagementAuthConfig that the<br />management cluster writes holds it, and Optimize reads it from there. |  | Optional: \{\} <br /> |
| `webModeler` _[PublicClientSpec](#publicclientspec)_ | WebModeler is the client of the Web Modeler user interface. The browser<br />holds no secret, so it is a public client. |  | Optional: \{\} <br /> |
| `webModelerApi` _[WebModelerAPIClientSpec](#webmodelerapiclientspec)_ | WebModelerAPI is the client of the Web Modeler API. |  | Optional: \{\} <br /> |
| `console` _[PublicClientSpec](#publicclientspec)_ | Console is the client of Console. The browser holds no secret, so it is<br />a public client. |  | Optional: \{\} <br /> |


#### ManagementOIDCClientsSpec



ManagementOIDCClientsSpec holds the identity provider clients of the
management plane.



_Appears in:_
- [OIDCSpec](#oidcspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clients` _[ManagementClients](#managementclients)_ | Clients holds one entry for each component of the management plane. |  |  |


#### ManagementOIDCSpec



ManagementOIDCSpec selects the identity provider of the referenced
CamundaPlatformConfig. The clients of the management plane are in that
config, under spec.auth.oidc.management.clients, so this block has no
fields.



_Appears in:_
- [IdentityProviderSpec](#identityproviderspec)



#### MonitoringSpec



MonitoringSpec groups the Prometheus scraping integration of the
Elasticsearch cluster.



_Appears in:_
- [ElasticsearchClusterSpec](#elasticsearchclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serviceMonitor` _[ServiceMonitorSpec](#servicemonitorspec)_ | ServiceMonitor configures the exporter and the Prometheus<br />ServiceMonitor. |  | Optional: \{\} <br /> |
| `exporter` _[ExporterSpec](#exporterspec)_ | Exporter tunes the exporter Deployment. |  | Optional: \{\} <br /> |


#### OIDCProviderType

_Underlying type:_ _string_

OIDCProviderType names the product behind an OIDC connection. Management
Identity reads it and changes how it resolves users and groups.



_Appears in:_
- [OIDCSpec](#oidcspec)

| Field | Description |
| --- | --- |
| `generic` | OIDCProviderGeneric fits any OIDC-compliant provider.<br /> |
| `microsoft` | OIDCProviderMicrosoft selects Microsoft Entra ID.<br /> |


#### OIDCSpec



OIDCSpec is the identity provider connection of a platform config. The
fields use the terms of OIDC discovery and work with any OIDC-compliant
provider.



_Appears in:_
- [PlatformAuthSpec](#platformauthspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `issuerUrl` _string_ | IssuerURL is the issuer URL of the identity provider. Consumers get the<br />endpoints from its OIDC discovery document, unless the endpoint fields<br />below set them. |  |  |
| `providerType` _[OIDCProviderType](#oidcprovidertype)_ | ProviderType names the kind of identity provider. Management Identity<br />reads it and changes how it resolves users and groups. Empty means<br />generic, which fits any OIDC-compliant provider. Set microsoft for<br />Microsoft Entra ID. |  | Enum: [generic microsoft] <br />Optional: \{\} <br /> |
| `jwksUrl` _string_ | JWKSURL is an explicit JWKS endpoint. It overrides the value from OIDC<br />discovery. |  | Optional: \{\} <br /> |
| `tokenUrl` _string_ | TokenURL is an explicit token endpoint. It overrides the value from OIDC<br />discovery. |  | Optional: \{\} <br /> |
| `authUrl` _string_ | AuthURL is an explicit authorization endpoint. It overrides the value<br />from OIDC discovery. |  | Optional: \{\} <br /> |
| `clientId` _string_ | ClientID is the default OIDC client ID of all clusters. A preset or a<br />cluster can set another value. |  | MinLength: 1 <br /> |
| `audience` _string_ | Audience is the audience that consumers validate in access tokens.<br />When empty, consumers use ClientID. |  | Optional: \{\} <br /> |
| `usernameClaim` _string_ | UsernameClaim is the token claim that holds the username of a person.<br />Empty means the default of the orchestration cluster, which is "sub". |  | Optional: \{\} <br /> |
| `clientIdClaim` _string_ | ClientIDClaim is the token claim that holds the id of a machine client.<br />Empty means that no claim identifies a client, and every token is a<br />person. The tokens of persons must not have this claim, because a<br />token with it is always a client. |  | Optional: \{\} <br /> |
| `clientSecretRef` _[SecretKeyRef](#secretkeyref)_ | ClientSecretRef names the Secret key that holds the default OIDC client<br />secret. |  |  |
| `management` _[ManagementOIDCClientsSpec](#managementoidcclientsspec)_ | Management holds the clients that the management plane uses at this<br />identity provider. A CamundaManagementCluster in the oidc mode reads<br />them. First register one client for each component at the provider. |  | Optional: \{\} <br /> |


#### ObjectStorageAuthType

_Underlying type:_ _string_

ObjectStorageAuthType selects how consumers authenticate against a bucket.

_Validation:_
- Enum: [workloadIdentity credentials]

_Appears in:_
- [AzureBlobStorageAuth](#azureblobstorageauth)
- [GCSStorageAuth](#gcsstorageauth)
- [S3StorageAuth](#s3storageauth)

| Field | Description |
| --- | --- |
| `workloadIdentity` |  |
| `credentials` |  |


#### ObjectStorageConfig



ObjectStorageConfig is the contract CRD that describes a bucket for
backups or document storage. It also describes how consumers
authenticate against the bucket: with workload identity on the
ServiceAccount of the consumer, or with static credentials in a Secret.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `ObjectStorageConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[ObjectStorageConfigSpec](#objectstorageconfigspec)_ | spec defines the desired state of ObjectStorageConfig |  | Required: \{\} <br /> |
| `status` _[ObjectStorageConfigStatus](#objectstorageconfigstatus)_ | status defines the observed state of ObjectStorageConfig |  | Optional: \{\} <br /> |


#### ObjectStorageConfigSpec



ObjectStorageConfigSpec describes a bucket and how consumers authenticate
against it. The type field selects exactly one of the s3, gcs, and
azureBlob blocks.



_Appears in:_
- [ObjectStorageConfig](#objectstorageconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ObjectStorageType](#objectstoragetype)_ | Type selects the storage API of the bucket. |  | Enum: [S3 GCS AzureBlob] <br /> |
| `s3` _[S3Storage](#s3storage)_ | S3 describes an S3 or S3-compatible bucket. Required when type is S3.<br />Forbidden with other types. |  | Optional: \{\} <br /> |
| `gcs` _[GCSStorage](#gcsstorage)_ | GCS describes a Google Cloud Storage bucket. Required when type is<br />GCS. Forbidden with other types. |  | Optional: \{\} <br /> |
| `azureBlob` _[AzureBlobStorage](#azureblobstorage)_ | AzureBlob describes an Azure Blob Storage container. Required when<br />type is AzureBlob. Forbidden with other types. |  | Optional: \{\} <br /> |


#### ObjectStorageConfigStatus



ObjectStorageConfigStatus is the observed validation state of the contract.



_Appears in:_
- [ObjectStorageConfig](#objectstorageconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current validation state. The Ready<br />condition has the reasons Healthy and MissingSecret. |  | Optional: \{\} <br /> |




#### ObjectStorageType

_Underlying type:_ _string_

ObjectStorageType identifies the storage API of a bucket.

_Validation:_
- Enum: [S3 GCS AzureBlob]

_Appears in:_
- [ObjectStorageConfigSpec](#objectstorageconfigspec)

| Field | Description |
| --- | --- |
| `S3` |  |
| `GCS` |  |
| `AzureBlob` |  |


#### OptimizeMonitoringSpec



OptimizeMonitoringSpec groups the monitoring integrations of a
CamundaOptimize.



_Appears in:_
- [CamundaOptimizeSpec](#camundaoptimizespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serviceMonitor` _[ServiceMonitorSpec](#servicemonitorspec)_ | ServiceMonitor configures the Prometheus ServiceMonitors. When enabled,<br />the operator creates one ServiceMonitor for each Deployment, with the<br />name of the workload. It scrapes /actuator/prometheus on the management<br />port. |  | Optional: \{\} <br /> |


#### OptimizeSuspension

_Underlying type:_ _string_

OptimizeSuspension names why the Optimize workloads are at zero with the
cluster that they attach to.

_Validation:_
- Enum: [Cluster StorageClaim]

_Appears in:_
- [CamundaOptimizeStatus](#camundaoptimizestatus)

| Field | Description |
| --- | --- |
| `Cluster` | OptimizeSuspensionCluster means that the referenced cluster is<br />suspended, by spec.suspend or by a state in which the operator keeps it<br />at zero.<br /> |
| `StorageClaim` | OptimizeSuspensionStorageClaim means that the referenced cluster does<br />not hold the storage claim of its backend, or that another writer still<br />writes to that backend. The writer can be pods of another cluster or of<br />a previous Optimize instance, or a restore into another cluster.<br /> |


#### PITRCapability



PITRCapability declares the point-in-time recovery capability of a server:
continuous WAL archiving with the given retention.



_Appears in:_
- [DatabaseServerConfigSpec](#databaseserverconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled reports whether the server performs continuous WAL archiving. | false | Optional: \{\} <br /> |
| `retentionPeriodDays` _integer_ | RetentionPeriodDays is how many days into the past a point-in-time<br />restore can go. Required when enabled is true.<br />The maximum is 36500 days, which is a hundred years. A reader counts<br />the reachable window in nanoseconds, and that count overflows at about<br />292 years. The maximum stays well below that limit. |  | Maximum: 36500 <br />Optional: \{\} <br /> |
| `recovery` _[RecoveryMode](#recoverymode)_ | Recovery says who rolls the server back to a point in time. Defaults to<br />external. The value operator means that the publisher of this contract<br />answers spec.recovery. It requires enabled: true. The value external<br />means that nobody answers, and you roll the server back manually. | external | Enum: [operator external] <br />Optional: \{\} <br /> |
| `lastRecovery` _[RecoveryOutcome](#recoveryoutcome)_ | LastRecovery is how the last recovery request ended. It is unset until<br />the first answer. The answer to each later request replaces it. |  | Optional: \{\} <br /> |


#### PartitionPosition



PartitionPosition is the exporter position of one partition, as the
pre-check read it from the restored database.



_Appears in:_
- [PointInTimeRestoreStatus](#pointintimerestorestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `partitionId` _integer_ | PartitionID is the Zeebe partition. |  |  |
| `lastUpdated` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | LastUpdated is the LAST_UPDATED value of the row of the partition in the<br />EXPORTER_POSITION table. |  |  |


#### PersistentVolumeClaimRetentionPolicy



PersistentVolumeClaimRetentionPolicy is like the StatefulSet field of the
same name, with only whenDeleted. There is no whenScaled choice. ECK
deletes the volume of each Elasticsearch node that it removes in a scale
down. The operator always keeps the volume of a broker that it removes in
a scale down.



_Appears in:_
- [ElasticsearchClusterSpec](#elasticsearchclusterspec)
- [ZeebeSpec](#zeebespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `whenDeleted` _[PersistentVolumeClaimRetentionPolicyType](#persistentvolumeclaimretentionpolicytype)_ | WhenDeleted is what happens to the data volumes when the resource is<br />deleted. Defaults to Delete. Delete removes them with the resource.<br />Retain keeps them, and a later resource with the same name attaches<br />them again. | Delete | Enum: [Retain Delete] <br />Optional: \{\} <br /> |


#### PersistentVolumeClaimRetentionPolicyType

_Underlying type:_ _string_

PersistentVolumeClaimRetentionPolicyType is what happens to the data
volumes of a resource (an ElasticsearchCluster, the brokers of a
CamundaCluster) when the resource is deleted.

_Validation:_
- Enum: [Retain Delete]

_Appears in:_
- [PersistentVolumeClaimRetentionPolicy](#persistentvolumeclaimretentionpolicy)

| Field | Description |
| --- | --- |
| `Retain` | RetainPersistentVolumeClaimRetentionPolicyType keeps the data volumes.<br />You remove the data manually. For an ElasticsearchCluster, the ECK<br />resource has the volume claim delete policy DeleteOnScaledownOnly. For<br />a CamundaCluster, the broker StatefulSet has whenDeleted Retain.<br /> |
| `Delete` | DeletePersistentVolumeClaimRetentionPolicyType deletes the data volumes<br />with the resource. For an ElasticsearchCluster, the ECK resource has<br />the volume claim delete policy DeleteOnScaledownAndClusterDeletion, the<br />ECK default. For a CamundaCluster, the broker StatefulSet has<br />whenDeleted Delete.<br /> |


#### PinnedStorage



PinnedStorage is the destination of a backup set, recorded when the backup
starts. It names the storage contract and the Elasticsearch endpoint that
hold the snapshots, and the backup bucket that holds the runtime backup.
status.repository records the name of the snapshot repository. Every step
checks the destination before it writes, and the operator checks it before
it deletes. Thus a destination that moves during the backup does not split
the set or send a deletion to the wrong place.



_Appears in:_
- [LogicalBackupElasticsearchStatus](#logicalbackupelasticsearchstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secondaryStorageConfig` _string_ | SecondaryStorageConfig is the name of the storage contract, in the<br />namespace of the backup. |  |  |
| `endpoint` _string_ | Endpoint is the Elasticsearch endpoint that the contract named. |  |  |
| `bucketRef` _string_ | BucketRef is the ObjectStorageConfig that the cluster used for backups<br />when the backup started: its spec.backupStorageRef at that time. The<br />runtime backup goes to that bucket. |  |  |
| `bucketLocation` _string_ | BucketLocation is where that contract pointed: the storage type, the<br />bucket, the base path, and the endpoint. The steps write, and the<br />operator deletes, only while the contract still points there. |  |  |


#### PlatformAuthSpec



PlatformAuthSpec selects the authentication method of every orchestration
cluster that references the platform config.



_Appears in:_
- [CamundaPlatformConfigSpec](#camundaplatformconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `method` _[AuthenticationMethod](#authenticationmethod)_ | Method is the authentication method. An unset auth block and an unset<br />method both mean basic. | basic | Enum: [basic oidc] <br />Optional: \{\} <br /> |
| `oidc` _[OIDCSpec](#oidcspec)_ | OIDC is the identity provider connection. Required when method is<br />oidc. Forbidden with other methods. |  | Optional: \{\} <br /> |


#### PodMonitorSpec



PodMonitorSpec configures the Prometheus PodMonitor of a resource whose
pods serve their own metrics endpoint. A DatabaseServer scrapes the
metrics port of every CloudNativePG instance. The PodMonitor is created
only when the Kubernetes cluster serves the kind.



_Appears in:_
- [DatabaseServerMonitoringSpec](#databaseservermonitoringspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled creates the PodMonitor when true. Defaults to false. |  | Optional: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | Labels are extra labels applied to the PodMonitor. |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations are extra annotations applied to the PodMonitor. |  | Optional: \{\} <br /> |
| `interval` _string_ | Interval is how often Prometheus scrapes the pods, as a Prometheus<br />duration such as 30s. Empty leaves the interval to the Prometheus<br />configuration. |  | Pattern: `^(0\|(([0-9]+)y)?(([0-9]+)w)?(([0-9]+)d)?(([0-9]+)h)?(([0-9]+)m)?(([0-9]+)s)?(([0-9]+)ms)?)$` <br />Optional: \{\} <br /> |


#### PointInTimeRestore



PointInTimeRestore brings the primary storage of a suspended CamundaCluster
with relational secondary storage to the same point in time as its
database. It reads the exporter position of every partition from that
database. Then it deletes and creates the broker data volumes again. Then
it runs the Camunda restore application with the requested point, one
time for each broker.

The database reaches that point in one of two ways. A DatabaseServerConfig
that declares pitr.recovery: operator is rolled back by its publisher. The
restore writes the request on the contract and waits for the answer. A
contract that declares external, the default, is rolled back before you
create the restore. The restore reads the database as it is.

The restore prepares the cluster itself. It suspends the cluster and waits
for its brokers to stop. When it completes, it removes the suspension, but
only a suspension that it applied itself. A failed restore leaves the
cluster suspended. A restore that somebody deletes while it runs also
leaves the cluster suspended. Empty or half-written broker volumes cause
more damage under running brokers.

It writes no version. This kind restores the primary storage of the
cluster from the continuous backups of the same cluster. Thus no backup
names a version other than the version that the cluster runs.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `PointInTimeRestore` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[PointInTimeRestoreSpec](#pointintimerestorespec)_ | spec names the cluster to restore and the point that its database<br />holds. It is immutable. A restore runs one time. To try again, create<br />a new resource. |  | Required: \{\} <br /> |
| `status` _[PointInTimeRestoreStatus](#pointintimerestorestatus)_ | status defines the observed state of the restore |  | Optional: \{\} <br /> |


#### PointInTimeRestorePhase

_Underlying type:_ _string_

PointInTimeRestorePhase is the phase of a restore that runs one time.
Completed and Failed are final. To try again, create a new resource.

_Validation:_
- Enum: [Pending RestoringDatabase ValidatingDatabaseState RestoringPrimaryStorage Completed Failed]

_Appears in:_
- [PointInTimeRestoreStatus](#pointintimerestorestatus)

| Field | Description |
| --- | --- |
| `Pending` | PointInTimeRestorePending means that the restore did not start its<br />work. A pre-check stops it: the cluster runs, the storage chain does<br />not resolve, or the database is ahead of spec.timestamp.<br /> |
| `RestoringDatabase` | PointInTimeRestoreRestoringDatabase means that the operator asked the<br />database server to roll back to spec.timestamp and waits for the<br />answer. The restore has this phase only when the DatabaseServerConfig<br />declares pitr.recovery: operator. A server that declares external rolls<br />back before you create the restore. The restore then goes directly to<br />ValidatingDatabaseState.<br /> |
| `ValidatingDatabaseState` | PointInTimeRestoreValidatingDatabaseState means that the operator reads<br />the exporter position of every partition from the restored database.<br />This happens before the operator changes a volume.<br /> |
| `RestoringPrimaryStorage` | PointInTimeRestoreRestoringPrimaryStorage means that the operator<br />created the broker data volumes again and runs the restore application<br />on them.<br /> |
| `Completed` | PointInTimeRestoreCompleted means that the restore finished. The<br />operator removes the Jobs of the brokers, so their pods release the<br />broker data volumes. It removes the suspension that it applied, so the<br />cluster runs again, unless its owner suspended it.<br /> |
| `Failed` | PointInTimeRestoreFailed means that the restore failed. The Ready<br />condition names the failed phase. The operator keeps the Jobs of the<br />brokers, because their logs show the cause. Thus a restore that reached<br />RestoringPrimaryStorage holds the broker data volumes until you delete<br />it. A restore that failed in an earlier phase records no Job in<br />PrimaryJobNames and holds nothing.<br /> |


#### PointInTimeRestoreSpec



PointInTimeRestoreSpec names the cluster to roll back and the point in time
to roll it back to. The whole spec is immutable.



_Appears in:_
- [PointInTimeRestore](#pointintimerestore)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clusterRef` _[ClusterRef](#clusterref)_ | ClusterRef references the CamundaCluster to restore, in the namespace<br />of this restore. Its secondary storage must be a relational database. |  | Required: \{\} <br /> |
| `timestamp` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | Timestamp is the point to restore to.<br />DatabaseServerConfig.spec.pitr.recovery sets who rolls the database<br />server back to it. With operator, the restore asks the server to roll<br />back to this point. With external, you roll the server back to it<br />before you create the restore.<br />Choose a point at least one backup interval before the cluster stopped<br />writing. Also choose a point inside the window in which Zeebe keeps its<br />primary-storage backups. The CamundaCluster sets these with<br />spec.backup.primaryStorage.schedule and retention.window. The defaults<br />are one hour and seven days. If no backup covers the point, the restore<br />fails after it erased the broker volumes.<br />The point must be inside the retention period of the database server,<br />and it must not be in the future. The operator does these two checks. |  | Required: \{\} <br /> |


#### PointInTimeRestoreStatus



PointInTimeRestoreStatus tracks the restore to a terminal phase.



_Appears in:_
- [PointInTimeRestore](#pointintimerestore)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[PointInTimeRestorePhase](#pointintimerestorephase)_ | Phase is the phase of the restore. The restore continues from it after<br />an interruption. |  | Enum: [Pending RestoringDatabase ValidatingDatabaseState RestoringPrimaryStorage Completed Failed] <br />Optional: \{\} <br /> |
| `storage` _[PointInTimeRestoreStorage](#pointintimerestorestorage)_ | Storage records the storage chain that the restore validated. The<br />operator records it before it reads the database, and again after a<br />rollback that the restore asked for. If a later read does not agree,<br />the restore fails. |  | Optional: \{\} <br /> |
| `backend` _string_ | Backend names the database that the restore holds while its server<br />rolls back: the host, the port, and the database name. The operator<br />records it just before it asks for the rollback. It follows each<br />endpoint that the contract names. From then until the final phase, no<br />other CamundaCluster starts on this database, also when the contract<br />names another endpoint. A restore whose server<br />rolls back outside the operator records no backend. |  | Optional: \{\} <br /> |
| `observedPositions` _[PartitionPosition](#partitionposition) array_ | ObservedPositions are the exporter positions that the pre-check read,<br />in partition order. They show what the operator saw when the restore<br />passed the database-state check, or what stopped it. |  | Optional: \{\} <br /> |
| `targetClusterUID` _[UID](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#uid-types-pkg)_ | TargetClusterUID records the identity of the target cluster. A cluster<br />that is deleted and created again with the same name is another<br />cluster, and this restore does not apply to it. |  | Optional: \{\} <br /> |
| `brokers` _integer_ | Brokers is the broker count of the broker StatefulSet. The operator<br />records it before the restore deletes a volume. It sets how many<br />volumes the restore creates again and how many Jobs run. |  | Optional: \{\} <br /> |
| `primaryJobNames` _string array_ | PrimaryJobNames are the restore application Jobs, one for each broker,<br />in broker order. The operator records them before it creates the Jobs.<br />A completed restore removes these Jobs. The logs of these Jobs explain<br />a failed restore. |  | Optional: \{\} <br /> |
| `recreatedClaims` _string array_ | RecreatedClaims names the broker data claims that the restore deleted<br />and created again. Thus the restore does not delete a claim two times. |  | Optional: \{\} <br /> |
| `firstFailedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | FirstFailedAt is when a dependency of the running restore first stopped<br />resolving. The grace period starts at this time. The field can clear if<br />the dependency recovers before the restore deletes target indices or<br />records a recreated broker volume or a broker Job. After that point, the<br />field stays set, so a dependency that fails and recovers again and<br />again does not reset the grace period. |  | Optional: \{\} <br /> |
| `clusterSuspended` _boolean_ | ClusterSuspended records that this restore suspended its target<br />cluster. The restore removes that suspension when it reaches<br />Completed. A cluster that its owner suspended has no such record, so<br />it stays suspended. The cluster of a failed restore also stays<br />suspended. |  | Optional: \{\} <br /> |
| `terminalReason` _string_ | TerminalReason is the Ready reason that the operator recorded when the<br />restore reached its final phase. |  | Optional: \{\} <br /> |
| `failureMessage` _string_ | FailureMessage names the failed phase and its error. The Ready<br />condition has the same message. |  | Optional: \{\} <br /> |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | CompletionTime is when the restore reached a final phase. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state of the restore. |  | Optional: \{\} <br /> |


#### PointInTimeRestoreStorage



PointInTimeRestoreStorage is the identity of the storage chain that the
restore validated. It holds the contracts that the restore resolved, the
logical database that it read, and the server of that database. Each part
of the chain can change. The restore checks the server and the database
before it deletes anything. If a later read does not agree with this
record, it is another database, and the restore fails. A rollback that the
restore asked for replaces the record.



_Appears in:_
- [PointInTimeRestoreStatus](#pointintimerestorestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secondaryStorageConfig` _string_ | SecondaryStorageConfig is the storage contract of the cluster. |  |  |
| `secondaryStorageConfigUID` _[UID](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#uid-types-pkg)_ | SecondaryStorageConfigUID records the identity of that contract. Thus<br />the restore finds a contract that was deleted and created again with<br />the same name. |  | Optional: \{\} <br /> |
| `databaseConfig` _string_ | DatabaseConfig is the contract of the logical database. |  |  |
| `databaseConfigUID` _[UID](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#uid-types-pkg)_ | DatabaseConfigUID records the identity of that contract. |  | Optional: \{\} <br /> |
| `databaseServerConfig` _string_ | DatabaseServerConfig is the contract of the server that holds the<br />database and declares its point-in-time recovery. |  |  |
| `databaseServerConfigUID` _[UID](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#uid-types-pkg)_ | DatabaseServerConfigUID records the identity of that contract. |  | Optional: \{\} <br /> |
| `databaseName` _string_ | DatabaseName is the logical database whose exporter position the<br />pre-check read. |  |  |
| `endpoint` _string_ | Endpoint is the host and port of the server, as the pre-check used it.<br />A contract that now names another endpoint names another server. |  |  |
| `systemIdentifier` _string_ | SystemIdentifier is the identity of the PostgreSQL instance behind that<br />endpoint, as the contract published it. The rule for a dedicated server<br />counted this identity. An endpoint that later reports another identity<br />is another instance. |  |  |


#### PrimaryStorageBackupSpec



PrimaryStorageBackupSpec configures the backup scheduler of Zeebe. On a
relational cluster, Camunda takes these backups itself, without a call
from the operator. They pair with the database dump. A restore reads the
exporter position from the restored database and selects the
primary-storage backups that match it.



_Appears in:_
- [ClusterBackupSpec](#clusterbackupspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `continuous` _boolean_ | Continuous keeps every log segment until it is backed up, so that a<br />restore finds a range without gaps. Defaults to true on a relational<br />cluster with a backupStorageRef. A preset can turn it on for many<br />clusters, and one cluster can still turn it off. |  | Optional: \{\} <br /> |
| `schedule` _string_ | Schedule is the interval at which Zeebe takes a primary-storage<br />backup: an ISO 8601 duration, a CRON expression, or "none". Defaults<br />to PT1H. Always use a schedule with continuous. Otherwise the log grows<br />without limit. |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `checkpointInterval` _string_ | CheckpointInterval is the interval at which Zeebe writes marker<br />checkpoints into the log stream, as an ISO 8601 duration of days and<br />time (P2DT3H, PT15M). Camunda parses no week, month, or year units.<br />It is the granularity of a point-in-time restore. Defaults to PT15M. |  | Pattern: `^P([0-9]+D(T(([0-9]+H)([0-9]+M)?([0-9]+([.][0-9]+)?S)?\|([0-9]+M)([0-9]+([.][0-9]+)?S)?\|[0-9]+([.][0-9]+)?S))?\|T(([0-9]+H)([0-9]+M)?([0-9]+([.][0-9]+)?S)?\|([0-9]+M)([0-9]+([.][0-9]+)?S)?\|[0-9]+([.][0-9]+)?S))$` <br />Optional: \{\} <br /> |
| `retention` _[PrimaryStorageRetentionSpec](#primarystorageretentionspec)_ | Retention bounds how long Zeebe keeps its primary-storage backups. |  | Optional: \{\} <br /> |


#### PrimaryStorageRetentionSpec



PrimaryStorageRetentionSpec bounds the primary-storage backups that Zeebe
keeps. Zeebe always keeps at least one backup, even outside the window.



_Appears in:_
- [PrimaryStorageBackupSpec](#primarystoragebackupspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `window` _string_ | Window is how far back the backups stay available for a restore, as an<br />ISO 8601 duration of days and time (P7D, PT12H). Camunda parses no<br />week, month, or year units. Defaults to P7D. It limits the restore<br />window, so set it at least as long as the recovery point that the<br />cluster needs. A BackupSchedule can keep database dumps for longer than<br />this window. A restore of such an old dump is not possible. |  | Pattern: `^P([0-9]+D(T(([0-9]+H)([0-9]+M)?([0-9]+([.][0-9]+)?S)?\|([0-9]+M)([0-9]+([.][0-9]+)?S)?\|[0-9]+([.][0-9]+)?S))?\|T(([0-9]+H)([0-9]+M)?([0-9]+([.][0-9]+)?S)?\|([0-9]+M)([0-9]+([.][0-9]+)?S)?\|[0-9]+([.][0-9]+)?S))$` <br />Optional: \{\} <br /> |
| `cleanupSchedule` _string_ | CleanupSchedule is the interval at which Zeebe looks for backups<br />outside the window: an ISO 8601 duration, a CRON expression, or<br />"none". Defaults to PT1H. |  | MinLength: 1 <br />Optional: \{\} <br /> |


#### PublicClientSpec



PublicClientSpec is an identity provider client that a browser uses, so it
has no secret.



_Appears in:_
- [ManagementClients](#managementclients)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clientId` _string_ | ClientID is the client id at the identity provider. |  | MinLength: 1 <br /> |
| `audience` _string_ | Audience is the audience that the component validates in access tokens.<br />Empty means the client id. |  | Optional: \{\} <br /> |


#### RDBMSStorage



RDBMSStorage holds relational database backend details.



_Appears in:_
- [SecondaryStorageConfigSpec](#secondarystorageconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `databaseConfigRef` _string_ | DatabaseConfigRef names the DatabaseConfig of the logical database to<br />use, in the namespace of this contract. |  | MinLength: 1 <br /> |


#### RecoveryArchiveIdentity



RecoveryArchiveIdentity is the workload identity of a bucket: what the pods
of a consumer use to read the objects in it. A bucket with static
credentials has none.



_Appears in:_
- [RecoveryArchiveRef](#recoveryarchiveref)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `annotations` _object (keys:string, values:string)_ | Annotations are the annotations of the ServiceAccount of the pods. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are the labels that the pods need. Only Azure needs one. |  | Optional: \{\} <br /> |


#### RecoveryArchiveRef



RecoveryArchiveRef names the archive that a recovery reads: the directory in
the bucket, the location of that bucket, and the bucket contract that
names it. It also holds the archive settings of the server at that moment.
Until the rollback has an answer, the operator keeps the archive as it
was. It does not apply an edit of spec.archive in that time: a moved
bucket, a changed retention or schedule, or a removal.



_Appears in:_
- [DatabaseServerRecoveryStatus](#databaseserverrecoverystatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serverName` _string_ | ServerName is the archive directory, equal to the name of the<br />CloudNativePG cluster that wrote it. |  |  |
| `objectStorageRef` _string_ | ObjectStorageRef is the ObjectStorageConfig, in the namespace of this<br />server, that the archive lives in. |  |  |
| `location` _string_ | Location is where in object storage the archive is, in the form of<br />ArchiveRecord.Location. A running recovery reads the location that it<br />recorded, so an edit of the ObjectStorageConfig does not move it. |  | Optional: \{\} <br /> |
| `retentionPeriodDays` _integer_ | RetentionPeriodDays is spec.archive.retentionPeriodDays as it stood<br />when the rollback started. |  | Optional: \{\} <br /> |
| `baseBackupSchedule` _string_ | BaseBackupSchedule is spec.archive.baseBackupSchedule as it stood when<br />the rollback started. |  | Optional: \{\} <br /> |
| `identity` _[RecoveryArchiveIdentity](#recoveryarchiveidentity)_ | Identity is the workload identity of that bucket when the rollback<br />started. The pods keep this identity while they read the bucket. It is<br />unset for a bucket with static credentials, and for a bucket that<br />names no identity. |  | Optional: \{\} <br /> |


#### RecoveryMode

_Underlying type:_ _string_

RecoveryMode says who rolls the server back to a point in time.

_Validation:_
- Enum: [operator external]

_Appears in:_
- [PITRCapability](#pitrcapability)

| Field | Description |
| --- | --- |
| `operator` | RecoveryModeOperator means that the publisher of this contract rolls<br />the server back when spec.recovery asks for it.<br /> |
| `external` | RecoveryModeExternal means that nobody answers spec.recovery. You roll<br />the server back manually, before the restore starts.<br /> |


#### RecoveryOutcome



RecoveryOutcome is how a recovery request ended. It repeats the request
that it answers. Thus a consumer knows whether it is the answer to its own
request.



_Appears in:_
- [PITRCapability](#pitrcapability)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requestID` _string_ | RequestID is the requestID of the request that this outcome answers. |  | Pattern: `^[0-9a-f]\{8\}-[0-9a-f]\{4\}-[0-9a-f]\{4\}-[0-9a-f]\{4\}-[0-9a-f]\{12\}$` <br /> |
| `requestedBy` _string_ | RequestedBy is the requestedBy of the request that this outcome<br />answers. |  | MaxLength: 507 <br />Pattern: `^[a-z0-9]([-a-z0-9.]*[a-z0-9])?/[a-z0-9]([-a-z0-9.]*[a-z0-9])?$` <br /> |
| `targetTime` _string_ | TargetTime is the targetTime of the request that this outcome answers,<br />in the same form as in the request. Thus a consumer can compare the<br />two as text. |  | Format: date-time <br />Pattern: `^\d\{4\}-\d\{2\}-\d\{2\}T\d\{2\}:\d\{2\}:\d\{2\}(\.\d+)?(Z\|[+-]\d\{2\}:\d\{2\})$` <br /> |
| `completedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | CompletedAt is when the request ended. |  |  |
| `result` _[RecoveryResult](#recoveryresult)_ | Result is how the request ended. See RecoveryResult for the values. |  | Enum: [Completed Failed Unavailable] <br /> |
| `message` _string_ | Message says what happened. It is empty for a result of Completed. |  | Optional: \{\} <br /> |


#### RecoveryRequest



RecoveryRequest asks the publisher of this contract to roll the server back
to a point in time. A consumer writes it with its own field manager. The
publisher of the contract never writes this field, so the two writers
never conflict.



_Appears in:_
- [DatabaseServerConfigSpec](#databaseserverconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requestID` _string_ | RequestID identifies this request, and only this one. A controller sets<br />the uid of the resource that asks. A manual request can use any UUID.<br />Two requests can name the same resource and the same point. The<br />RequestID tells them apart. A resource that is deleted and created<br />again with the same name is another requester. The answer to the first<br />request says nothing about the state that the second requester asks<br />for. |  | Pattern: `^[0-9a-f]\{8\}-[0-9a-f]\{4\}-[0-9a-f]\{4\}-[0-9a-f]\{4\}-[0-9a-f]\{12\}$` <br /> |
| `requestedBy` _string_ | RequestedBy is the namespace and the name of the resource that asks, as<br />"<namespace>/<name>". It comes back in pitr.lastRecovery. Thus the<br />requester can find its own answer. |  | MaxLength: 507 <br />Pattern: `^[a-z0-9]([-a-z0-9.]*[a-z0-9])?/[a-z0-9]([-a-z0-9.]*[a-z0-9])?$` <br /> |
| `targetTime` _string_ | TargetTime is the point to roll back to, as RFC 3339 with a zone, for<br />example 2026-08-20T14:30:00Z. The API server refuses a timestamp<br />without a zone. PostgreSQL reads such a timestamp as the local time of<br />the server, which can be a different point from the one that the<br />writer intended. |  | Format: date-time <br />Pattern: `^\d\{4\}-\d\{2\}-\d\{2\}T\d\{2\}:\d\{2\}:\d\{2\}(\.\d+)?(Z\|[+-]\d\{2\}:\d\{2\})$` <br /> |


#### RecoveryResult

_Underlying type:_ _string_

RecoveryResult is how a recovery request ended.

_Validation:_
- Enum: [Completed Failed Unavailable]

_Appears in:_
- [DatabaseServerRecoveryStatus](#databaseserverrecoverystatus)
- [RecoveryOutcome](#recoveryoutcome)

| Field | Description |
| --- | --- |
| `Completed` | RecoveryResultCompleted means that the server now holds the state of<br />the requested point.<br /> |
| `Failed` | RecoveryResultFailed means that the recovery started and did not<br />finish.<br /> |
| `Unavailable` | RecoveryResultUnavailable means that the server holds no copy of the<br />requested point, so it attempted no recovery.<br /> |


#### ReleaseConnectorsSpec



ReleaseConnectorsSpec is the connectors runtime of a release.



_Appears in:_
- [CamundaReleaseSpec](#camundareleasespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version is the version of the connectors bundle image, as a full<br />semantic version. The bundle has its own patch line, so it does not<br />follow the cluster version. A cluster that runs connectors needs it<br />from the release or from its own spec. |  | Pattern: `^\d+\.\d+\.\d+$` <br />Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the container of this<br />process. An entry here wins over a top-level entry of the release with<br />the same name. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the<br />container of this process. |  | Optional: \{\} <br /> |


#### ReleaseDatabaseServerSpec



ReleaseDatabaseServerSpec is the PostgreSQL server of a release.



_Appears in:_
- [CamundaReleaseSpec](#camundareleasespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version is the PostgreSQL major version of the servers of this release,<br />as a number such as "17". A server that references the release refuses<br />an effective version below 14. Its own spec.version wins over this<br />value. A server that already runs another major version refuses the<br />change and keeps its major version. |  | Pattern: `^\d+$` <br />Optional: \{\} <br /> |


#### ReleaseElasticsearchSpec



ReleaseElasticsearchSpec is the Elasticsearch of a release.



_Appears in:_
- [CamundaReleaseSpec](#camundareleasespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version is the Elasticsearch version of the clusters of this release,<br />as a full semantic version. Elasticsearch has its own patch line, so it<br />does not follow the Camunda version. A cluster that references the<br />release refuses an effective version below 8.19 in the 8 line or below<br />9.2 in the 9 line. Its own spec.version wins over this value. |  | Pattern: `^\d+\.\d+\.\d+$` <br />Optional: \{\} <br /> |


#### ReleaseEnvSpec



ReleaseEnvSpec is the environment of one component that a release adds.



_Appears in:_
- [CamundaReleaseSpec](#camundareleasespec)
- [ReleaseConnectorsSpec](#releaseconnectorsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the container of this<br />process. An entry here wins over a top-level entry of the release with<br />the same name. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the<br />container of this process. |  | Optional: \{\} <br /> |


#### ReleaseImagesSpec



ReleaseImagesSpec holds the image references that a release pins. Each
value is a complete reference, with its tag or digest.



_Appears in:_
- [CamundaReleaseSpec](#camundareleasespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `camunda` _string_ | Camunda is the image of every orchestration cluster process. Unset<br />means the camunda image of the platform config at the version of the<br />release. |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `connectors` _string_ | Connectors is the image of the connectors runtime. Unset means the<br />connectors image of the platform config at connectors.version. |  | MinLength: 1 <br />Optional: \{\} <br /> |


#### RestoreProgress



RestoreProgress is the part of a restore status that every restore kind
has. Each restore status embeds it inline, so its fields appear directly
in the status.



_Appears in:_
- [LogicalRestoreElasticsearchStatus](#logicalrestoreelasticsearchstatus)
- [LogicalRestoreRDBMSStatus](#logicalrestorerdbmsstatus)
- [PointInTimeRestoreStatus](#pointintimerestorestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `targetClusterUID` _[UID](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#uid-types-pkg)_ | TargetClusterUID records the identity of the target cluster. A cluster<br />that is deleted and created again with the same name is another<br />cluster, and this restore does not apply to it. |  | Optional: \{\} <br /> |
| `brokers` _integer_ | Brokers is the broker count of the broker StatefulSet. The operator<br />records it before the restore deletes a volume. It sets how many<br />volumes the restore creates again and how many Jobs run. |  | Optional: \{\} <br /> |
| `primaryJobNames` _string array_ | PrimaryJobNames are the restore application Jobs, one for each broker,<br />in broker order. The operator records them before it creates the Jobs.<br />A completed restore removes these Jobs. The logs of these Jobs explain<br />a failed restore. |  | Optional: \{\} <br /> |
| `recreatedClaims` _string array_ | RecreatedClaims names the broker data claims that the restore deleted<br />and created again. Thus the restore does not delete a claim two times. |  | Optional: \{\} <br /> |
| `firstFailedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | FirstFailedAt is when a dependency of the running restore first stopped<br />resolving. The grace period starts at this time. The field can clear if<br />the dependency recovers before the restore deletes target indices or<br />records a recreated broker volume or a broker Job. After that point, the<br />field stays set, so a dependency that fails and recovers again and<br />again does not reset the grace period. |  | Optional: \{\} <br /> |
| `clusterSuspended` _boolean_ | ClusterSuspended records that this restore suspended its target<br />cluster. The restore removes that suspension when it reaches<br />Completed. A cluster that its owner suspended has no such record, so<br />it stays suspended. The cluster of a failed restore also stays<br />suspended. |  | Optional: \{\} <br /> |
| `terminalReason` _string_ | TerminalReason is the Ready reason that the operator recorded when the<br />restore reached its final phase. |  | Optional: \{\} <br /> |
| `failureMessage` _string_ | FailureMessage names the failed phase and its error. The Ready<br />condition has the same message. |  | Optional: \{\} <br /> |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | CompletionTime is when the restore reached a final phase. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state of the restore. |  | Optional: \{\} <br /> |


#### RetainedBackups



RetainedBackups limits the backups that a schedule keeps, for each final
phase. When the count of a phase is more than its limit, the operator
deletes the oldest backups of that phase. The deletion also tries to
remove their stored backup data.



_Appears in:_
- [BackupScheduleSpec](#backupschedulespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `completed` _integer_ | Completed is how many completed backups the schedule keeps. | 7 | Minimum: 1 <br />Optional: \{\} <br /> |
| `failed` _integer_ | Failed is how many failed backups the schedule keeps. With zero, the<br />operator deletes a failed backup soon after it fails. | 3 | Minimum: 0 <br />Optional: \{\} <br /> |


#### S3Credentials



S3Credentials holds the static keys of an S3 bucket.



_Appears in:_
- [S3StorageAuth](#s3storageauth)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretRef` _[S3CredentialsSecretRef](#s3credentialssecretref)_ | SecretRef names the Secret keys that hold the access-key pair. |  |  |


#### S3CredentialsSecretRef



S3CredentialsSecretRef references an access-key pair in a Secret in the
namespace of the contract.



_Appears in:_
- [S3Credentials](#s3credentials)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the Secret that holds the keys. |  | MinLength: 1 <br /> |
| `accessKeyIdKey` _string_ | AccessKeyIDKey is the key in the Secret that holds the access key ID. |  | MinLength: 1 <br /> |
| `secretAccessKeyKey` _string_ | SecretAccessKeyKey is the key in the Secret that holds the secret<br />access key. |  | MinLength: 1 <br /> |


#### S3Storage



S3Storage describes an S3 or S3-compatible bucket.



_Appears in:_
- [ObjectStorageConfigSpec](#objectstorageconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bucketName` _string_ | BucketName is the name of the bucket, as storage client SDKs use it. |  | MinLength: 1 <br /> |
| `basePath` _string_ | BasePath is the key prefix under which consumers write objects,<br />without leading or trailing slashes. Empty means the bucket root. |  | Pattern: `^[^/]+(/[^/]+)*$` <br />Optional: \{\} <br /> |
| `region` _string_ | Region is the region of the bucket. Required unless endpoint is set. |  | Optional: \{\} <br /> |
| `endpoint` _string_ | Endpoint is the URL of an S3-compatible store (MinIO, Ceph, and more).<br />Empty means AWS S3, addressed through region. |  | Optional: \{\} <br /> |
| `forcePathStyle` _boolean_ | ForcePathStyle forces path-style bucket addressing. Set it for<br />S3-compatible stores that do not serve virtual-hosted-style requests. |  | Optional: \{\} <br /> |
| `auth` _[S3StorageAuth](#s3storageauth)_ | Auth selects how consumers authenticate. An absent block means<br />workload identity through the ServiceAccount. | \{ type:workloadIdentity \} | Optional: \{\} <br /> |


#### S3StorageAuth



S3StorageAuth selects how consumers authenticate against an S3 bucket.



_Appears in:_
- [S3Storage](#s3storage)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ObjectStorageAuthType](#objectstorageauthtype)_ | Type is the authentication choice. Defaults to workloadIdentity. | workloadIdentity | Enum: [workloadIdentity credentials] <br />Optional: \{\} <br /> |
| `workloadIdentity` _[S3WorkloadIdentity](#s3workloadidentity)_ | WorkloadIdentity names the trusted principal. It is valid only with<br />type workloadIdentity. With an empty or absent block, the operator adds<br />no annotation. The identity must then be bound on the cloud side, for<br />example with EKS Pod Identity. |  | Optional: \{\} <br /> |
| `credentials` _[S3Credentials](#s3credentials)_ | Credentials are static keys. Required with type credentials. Forbidden<br />with other types. |  | Optional: \{\} <br /> |


#### S3WorkloadIdentity



S3WorkloadIdentity names the AWS principal that the bucket trusts. With an
empty block, the operator adds no annotation to the ServiceAccount of the
consumer. The identity must then be bound on the cloud side, for example
with EKS Pod Identity.



_Appears in:_
- [S3StorageAuth](#s3storageauth)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `roleArn` _string_ | RoleARN is the IAM role that consumers assume. When set, the operator<br />puts it in the eks.amazonaws.com/role-arn annotation of a ServiceAccount<br />that it creates for the consumer (IRSA). On an existing ServiceAccount,<br />add the annotation yourself. |  | Optional: \{\} <br /> |


#### SchedulingSpec



SchedulingSpec holds the scheduling constraints of the pods of a resource.
When a resource sets its own block, it replaces the complete scheduling
block of its preset. The two blocks never merge field by field.



_Appears in:_
- [BackupDumpSpec](#backupdumpspec)
- [CamundaClusterSpec](#camundaclusterspec)
- [ConnectorsSpec](#connectorsspec)
- [ConsoleSpec](#consolespec)
- [DatabaseServerSpec](#databaseserverspec)
- [DumpPodSpec](#dumppodspec)
- [ElasticsearchClusterSpec](#elasticsearchclusterspec)
- [GatewaySpec](#gatewayspec)
- [IdentitySpec](#identityspec)
- [ManagedKeycloakSpec](#managedkeycloakspec)
- [WebAppSpec](#webappspec)
- [WorkloadSpec](#workloadspec)
- [ZeebeSpec](#zeebespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `nodeAffinity` _[NodeAffinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#nodeaffinity-v1-core)_ | NodeAffinity rules for the pods. |  | Optional: \{\} <br /> |
| `podAffinity` _[PodAffinity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#podaffinity-v1-core)_ | PodAffinity rules for the pods. |  | Optional: \{\} <br /> |
| `tolerations` _[Toleration](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#toleration-v1-core) array_ | Tolerations for the pods. |  | Optional: \{\} <br /> |


#### ScratchVolumeSpec



ScratchVolumeSpec sizes the volume that holds a database dump until the
upload. An unset block is an emptyDir with the limits of the node. A large
database can fill it.



_Appears in:_
- [BackupDumpSpec](#backupdumpspec)
- [DumpPodSpec](#dumppodspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `sizeLimit` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#quantity-resource-api)_ | SizeLimit is the size of the emptyDir that holds the dump. Set it to<br />the size the dump needs, with room to spare. |  | Optional: \{\} <br /> |
| `storageClassName` _string_ | StorageClassName makes the scratch volume a PersistentVolumeClaim of<br />this class, not an emptyDir. Use it when the dump is larger than the<br />ephemeral storage of a node. |  | Optional: \{\} <br /> |


#### SecondaryStorageConfig



SecondaryStorageConfig is the namespaced contract CRD that tells an
orchestration cluster where its secondary storage is and how to
authenticate against it. The storage is an Elasticsearch cluster or a
relational database. Consumers find it by name in their own namespace.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `SecondaryStorageConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[SecondaryStorageConfigSpec](#secondarystorageconfigspec)_ | spec defines the desired state of SecondaryStorageConfig |  | Required: \{\} <br /> |
| `status` _[SecondaryStorageConfigStatus](#secondarystorageconfigstatus)_ | status defines the observed state of SecondaryStorageConfig |  | Optional: \{\} <br /> |


#### SecondaryStorageConfigSpec



SecondaryStorageConfigSpec tells an orchestration cluster where its
secondary storage is and how to authenticate against it.



_Appears in:_
- [SecondaryStorageConfig](#secondarystorageconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[SecondaryStorageType](#secondarystoragetype)_ | Type selects the secondary storage backend that this contract<br />describes. |  | Enum: [elasticsearch rdbms] <br /> |
| `elasticsearch` _[ElasticsearchStorage](#elasticsearchstorage)_ | Elasticsearch holds the Elasticsearch connection details. Required<br />when type is elasticsearch. Forbidden with other types. |  | Optional: \{\} <br /> |
| `rdbms` _[RDBMSStorage](#rdbmsstorage)_ | RDBMS holds the relational database details. Required when type is<br />rdbms. Forbidden with other types. |  | Optional: \{\} <br /> |


#### SecondaryStorageConfigStatus



SecondaryStorageConfigStatus is the observed validation state of the contract.



_Appears in:_
- [SecondaryStorageConfig](#secondarystorageconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation that the operator processed. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current validation state. The Ready condition<br />has the reasons Healthy, MissingSecret, or InvalidReference. |  | Optional: \{\} <br /> |


#### SecondaryStorageType

_Underlying type:_ _string_

SecondaryStorageType identifies the secondary storage backend that a
contract describes.

_Validation:_
- Enum: [elasticsearch rdbms]

_Appears in:_
- [SecondaryStorageConfigSpec](#secondarystorageconfigspec)

| Field | Description |
| --- | --- |
| `elasticsearch` | SecondaryStorageTypeElasticsearch selects an Elasticsearch backend.<br /> |
| `rdbms` | SecondaryStorageTypeRDBMS selects a relational database backend.<br /> |


#### SecretKeyRef



SecretKeyRef references one value in a Secret in a named namespace. A
cluster-scoped kind uses it, because it has no namespace of its own. A
namespaced kind uses LocalSecretKeyRef.



_Appears in:_
- [CamundaPlatformConfigSpec](#camundaplatformconfigspec)
- [ConfidentialClientSpec](#confidentialclientspec)
- [ManagementAuthConfigSpec](#managementauthconfigspec)
- [OIDCSpec](#oidcspec)
- [WebModelerAPIClientSpec](#webmodelerapiclientspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the Secret that holds the value. |  | MinLength: 1 <br /> |
| `namespace` _string_ | Namespace is the namespace of the Secret. |  | MinLength: 1 <br /> |
| `key` _string_ | Key is the key in the Secret that holds the value. |  | MinLength: 1 <br /> |


#### SecureSettingEntry



SecureSettingEntry projects one key of a Secret to a named entry of the
Elasticsearch keystore.



_Appears in:_
- [SecureSettingsSource](#securesettingssource)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `key` _string_ | Key in the Secret. |  | MinLength: 1 <br /> |
| `path` _string_ | Path is the keystore entry that the key becomes, for example<br />s3.client.default.access_key. |  | MinLength: 1 <br /> |


#### SecureSettingsSource



SecureSettingsSource references a Secret that ECK loads into the keystore
of every Elasticsearch node. Elasticsearch reads credentials only from the
keystore, never from the settings of a snapshot repository.



_Appears in:_
- [ElasticsearchClusterSpec](#elasticsearchclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretName` _string_ | SecretName is the Secret, in the namespace of the ElasticsearchCluster. |  | MinLength: 1 <br /> |
| `entries` _[SecureSettingEntry](#securesettingentry) array_ | Entries maps single keys to keystore entries. An empty list loads<br />every key of the Secret under its own name. |  | Optional: \{\} <br /> |


#### ServiceAccountSpec



ServiceAccountSpec configures the ServiceAccount of the pods of a resource.



_Appears in:_
- [CamundaClusterSpec](#camundaclusterspec)
- [ElasticsearchClusterSpec](#elasticsearchclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the ServiceAccount. When empty, the operator<br />derives the name from the name of the resource, as the doc of each kind<br />states. The cloud provider uses this name. A workload identity that<br />needs no annotation, such as EKS Pod Identity, binds the principal<br />system:serviceaccount:<namespace>:<name>. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br />Optional: \{\} <br /> |
| `create` _boolean_ | Create makes the operator create and own the ServiceAccount. Defaults<br />to true. False names a ServiceAccount that already exists. The operator<br />then does not create, annotate, or own it. If it does not exist, Ready<br />reports InvalidReference, except on a suspended ElasticsearchCluster. The operator never takes ownership of a<br />ServiceAccount that it did not create, because it deletes an owned one<br />with the resource. |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations to set on the ServiceAccount that the operator creates.<br />Usually these are workload-identity annotations (IRSA, GCP Workload<br />Identity, and more) that give the pods access to cloud resources, such<br />as the snapshot bucket for backups. The operator also adds the identity<br />annotation of each bucket contract that names an identity. An annotation<br />set here wins over the operator annotation with the same key. With<br />create false, the operator sets no annotation. Add them to the existing<br />ServiceAccount yourself. |  | Optional: \{\} <br /> |


#### ServiceMonitorSpec



ServiceMonitorSpec configures the Prometheus ServiceMonitors of a resource
that runs workloads. The kind of the resource sets what Prometheus scrapes.
Elasticsearch serves no Prometheus endpoint, so an ElasticsearchCluster
deploys the prometheus-community elasticsearch_exporter and scrapes it. A
CamundaCluster scrapes /actuator/prometheus of every process. The operator
creates the ServiceMonitor only when the Kubernetes cluster serves the kind.



_Appears in:_
- [ClusterMonitoringSpec](#clustermonitoringspec)
- [MonitoringSpec](#monitoringspec)
- [OptimizeMonitoringSpec](#optimizemonitoringspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled creates the ServiceMonitors (and, for an ElasticsearchCluster,<br />the exporter) when true. Defaults to false. |  | Optional: \{\} <br /> |
| `labels` _object (keys:string, values:string)_ | Labels are extra labels applied to the ServiceMonitor. |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations are extra annotations applied to the ServiceMonitor. |  | Optional: \{\} <br /> |




#### VolumeStatus



VolumeStatus is the observed size of one data PersistentVolumeClaim of a
cluster. A status lists one entry for each bound claim, sorted by name.
Thus it also shows a resize of one claim outside the spec, for example by
an auto-resize controller.



_Appears in:_
- [CamundaClusterStatus](#camundaclusterstatus)
- [DatabaseServerStatus](#databaseserverstatus)
- [ElasticsearchClusterStatus](#elasticsearchclusterstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the PersistentVolumeClaim. |  |  |
| `capacity` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#quantity-resource-api)_ | Capacity is the storage capacity that the bound claim reports. |  |  |


#### WebAppSpec



WebAppSpec configures one of the web applications Operate, Tasklist, and
Admin.



_Appears in:_
- [CamundaClusterSpec](#camundaclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `mode` _[ComponentMode](#componentmode)_ | Mode selects a Deployment of the unified binary that serves only this<br />application (Standalone) or the nearest standalone host up the chain<br />(Embedded). Defaults to Embedded. The workload fields have an effect<br />only in Standalone mode. The exception is extraEnv and extraEnvFrom: in<br />Embedded mode, they apply to the host process. |  | Enum: [Standalone Embedded] <br />Optional: \{\} <br /> |
| `replicas` _integer_ | Replicas is the number of pods of this process. Defaults to 1. On a<br />CamundaCluster it has no effect on an embedded web application. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the container of this process. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the container of this<br />process. On a CamundaCluster, an entry here wins over a top-level entry<br />with the same name. The entries of an embedded web application apply to<br />its host process.<br />The list merges by name under server-side apply. Each field manager owns<br />only the entries that it applies, so an extension controller can add its<br />own entry next to yours. One applied manifest cannot hold two entries<br />with the same name.<br />Two field managers that apply the same name do not conflict, because the<br />merge is per field inside the entry. One manager can own value and the<br />other valueFrom. A container rejects an entry that has both, so the API<br />server refuses to store such an entry. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the<br />container of this process. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the pods of this process. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the pods of this process. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling holds the scheduling constraints of the pods of this process.<br />When set, it replaces all other scheduling blocks for this process, with<br />no merge. On a CamundaCluster, these are the top-level block and the<br />block of a preset. |  | Optional: \{\} <br /> |


#### WebModelerAPIClientSpec



WebModelerAPIClientSpec is the client of the Web Modeler API. Web Modeler
validates two audiences: one for the API that its own user interface calls,
and one for the public API that your applications call.



_Appears in:_
- [ManagementClients](#managementclients)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clientId` _string_ | ClientID is the client id at the identity provider. |  | MinLength: 1 <br /> |
| `audience` _string_ | Audience is the audience that the component validates in access tokens.<br />Empty means the client id. |  | Optional: \{\} <br /> |
| `clientSecretRef` _[SecretKeyRef](#secretkeyref)_ | ClientSecretRef names the Secret key that holds the client secret. |  |  |
| `publicApiAudience` _string_ | PublicAPIAudience is the audience of the Web Modeler public API. Empty<br />means web-modeler-public-api. |  | Optional: \{\} <br /> |


#### WebModelerMailSpec



WebModelerMailSpec configures the SMTP server of Web Modeler.



_Appears in:_
- [WebModelerSpec](#webmodelerspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `smtpHost` _string_ | SMTPHost is the host name of the SMTP server. |  | MinLength: 1 <br /> |
| `smtpPort` _integer_ | SMTPPort is the port of the SMTP server. Defaults to 587. | 587 | Maximum: 65535 <br />Minimum: 1 <br />Optional: \{\} <br /> |
| `fromAddress` _string_ | FromAddress is the address that Web Modeler sends from. |  | MinLength: 3 <br /> |
| `fromName` _string_ | FromName is the display name of the sender of Web Modeler mail. |  | Optional: \{\} <br /> |
| `tls` _boolean_ | TLS turns STARTTLS on. Defaults to true. |  | Optional: \{\} <br /> |
| `credentialsSecretRef` _[LocalCredentialsSecretRef](#localcredentialssecretref)_ | CredentialsSecretRef names the Secret with the user and the password of<br />the SMTP server. Leave it unset for a server that needs no credentials. |  | Optional: \{\} <br /> |


#### WebModelerSpec



WebModelerSpec configures Web Modeler. Web Modeler runs as two workloads:
the restapi process and the websockets process.



_Appears in:_
- [CamundaManagementClusterSpec](#camundamanagementclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version is the Web Modeler version, as a full semantic version. The<br />operator supports 8.9.0 and later. |  | Pattern: `^\d+\.\d+\.\d+$` <br /> |
| `externalUrl` _string_ | ExternalURL is the URL where browsers reach Web Modeler. |  |  |
| `websocketsExternalUrl` _string_ | WebsocketsExternalURL is the URL where browsers reach the websockets<br />process. Web Modeler sends live updates through it. |  |  |
| `databaseConfigRef` _string_ | DatabaseConfigRef names the DatabaseConfig of the Web Modeler database,<br />in the namespace of this resource. Web Modeler needs its own PostgreSQL<br />database. |  | MinLength: 1 <br /> |
| `mail` _[WebModelerMailSpec](#webmodelermailspec)_ | Mail configures the SMTP server that Web Modeler sends notifications<br />through. Web Modeler does not start without it. |  |  |
| `restapi` _[WorkloadSpec](#workloadspec)_ | Restapi configures the workload of the restapi process. |  | Optional: \{\} <br /> |
| `websockets` _[WorkloadSpec](#workloadspec)_ | Websockets configures the workload of the websockets process. |  | Optional: \{\} <br /> |


#### WorkloadSpec



WorkloadSpec holds the settings of one process: the size, the environment,
the pod metadata, and the scheduling. Every component block of a
CamundaCluster uses it, and so does each workload of a CamundaOptimize.



_Appears in:_
- [CamundaOptimizeSpec](#camundaoptimizespec)
- [ConnectorsSpec](#connectorsspec)
- [ConsoleSpec](#consolespec)
- [GatewaySpec](#gatewayspec)
- [IdentitySpec](#identityspec)
- [WebAppSpec](#webappspec)
- [WebModelerSpec](#webmodelerspec)
- [ZeebeSpec](#zeebespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ | Replicas is the number of pods of this process. Defaults to 1. On a<br />CamundaCluster it has no effect on an embedded web application. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the container of this process. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the container of this<br />process. On a CamundaCluster, an entry here wins over a top-level entry<br />with the same name. The entries of an embedded web application apply to<br />its host process.<br />The list merges by name under server-side apply. Each field manager owns<br />only the entries that it applies, so an extension controller can add its<br />own entry next to yours. One applied manifest cannot hold two entries<br />with the same name.<br />Two field managers that apply the same name do not conflict, because the<br />merge is per field inside the entry. One manager can own value and the<br />other valueFrom. A container rejects an entry that has both, so the API<br />server refuses to store such an entry. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the<br />container of this process. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the pods of this process. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the pods of this process. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling holds the scheduling constraints of the pods of this process.<br />When set, it replaces all other scheduling blocks for this process, with<br />no merge. On a CamundaCluster, these are the top-level block and the<br />block of a preset. |  | Optional: \{\} <br /> |


#### ZeebeSpec



ZeebeSpec configures the brokers. Zeebe is always a standalone StatefulSet
with persistent volumes.



_Appears in:_
- [CamundaClusterSpec](#camundaclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ | Replicas is the number of pods of this process. Defaults to 1. On a<br />CamundaCluster it has no effect on an embedded web application. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the container of this process. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the container of this<br />process. On a CamundaCluster, an entry here wins over a top-level entry<br />with the same name. The entries of an embedded web application apply to<br />its host process.<br />The list merges by name under server-side apply. Each field manager owns<br />only the entries that it applies, so an extension controller can add its<br />own entry next to yours. One applied manifest cannot hold two entries<br />with the same name.<br />Two field managers that apply the same name do not conflict, because the<br />merge is per field inside the entry. One manager can own value and the<br />other valueFrom. A container rejects an entry that has both, so the API<br />server refuses to store such an entry. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the<br />container of this process. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the pods of this process. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the pods of this process. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling holds the scheduling constraints of the pods of this process.<br />When set, it replaces all other scheduling blocks for this process, with<br />no merge. On a CamundaCluster, these are the top-level block and the<br />block of a preset. |  | Optional: \{\} <br /> |
| `partitions` _integer_ | Partitions is the number of partitions. Defaults to 1. Once set, it can<br />neither be decreased nor removed. |  | Minimum: 1 <br />Optional: \{\} <br /> |
| `replicationFactor` _integer_ | ReplicationFactor is the number of brokers that hold a copy of each<br />partition. Defaults to 1. It must not exceed replicas. |  | Minimum: 1 <br />Optional: \{\} <br /> |
| `storageClassName` _string_ | StorageClassName is the StorageClass of the broker volumes. Defaults to<br />the default StorageClass of the Kubernetes cluster. The class cannot<br />change after the broker StatefulSet exists. The API server refuses a<br />change of a value that the CamundaCluster sets itself. A preset can<br />change the class, and a cluster that got its class from a preset can<br />set another one. A cluster whose broker StatefulSet exists, also a<br />suspended one, then keeps its class and records a<br />StorageClassChangeIgnored event. A cluster without a broker StatefulSet<br />takes the new class. Volumes that whenDeleted Retain kept from a<br />deleted cluster of the same name keep their class. |  | Optional: \{\} <br /> |
| `storageSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#quantity-resource-api)_ | StorageSize is the size of the data volume of each broker. Defaults to<br />10Gi. It can only grow. The API server refuses a change of this field<br />in a CamundaCluster to a smaller value. A smaller value is accepted when<br />the field was not set before, or when a preset lowers the size. A<br />cluster whose volumes are already larger then keeps that size and<br />records a StorageShrinkIgnored event.<br />When the size grows, the operator expands the existing claims in place.<br />The storage class must support volume expansion. |  | Optional: \{\} <br /> |
| `persistentVolumeClaimRetentionPolicy` _[PersistentVolumeClaimRetentionPolicy](#persistentvolumeclaimretentionpolicy)_ | PersistentVolumeClaimRetentionPolicy says what happens to the broker<br />volumes when the CamundaCluster is deleted. A scale-down and a<br />suspension always keep them. |  | Optional: \{\} <br /> |


