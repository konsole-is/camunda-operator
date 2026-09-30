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
| `rotation` _string_ | Rotation is the last admin password rotation that the operator<br />applied: the effective spec.auth.basic.passwordRotation value, after<br />the preset merge, that produced the password in the admin Secret. A<br />rotation is in progress while that effective value is not empty and<br />differs from this one. A cleared value does not stop a rotation that<br />the operator already staged. That rotation completes, and this field<br />then records the value that staged it. A cluster that inherits the<br />value from its preset carries none of its own in the spec. |  | Optional: \{\} <br /> |


#### ArchiveBoundary



ArchiveBoundary is the moment a server moved its archive to another
location while no interval was open. Only a base backup that began after it
belongs to the archive the server writes now: one that began before it wrote
to the location the server left.



_Appears in:_
- [DatabaseServerArchiveStatus](#databaseserverarchivestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `at` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | At is when the server moved. |  |  |
| `location` _string_ | Location is the location it moved to, in the form ArchiveRecord.Location<br />takes. The boundary is spent once the archive of that location opens a<br />record, and it moves again when the location moves again. |  |  |
| `objectStorageRef` _string_ | ObjectStorageRef is the ObjectStorageConfig of that location, for the<br />reader. |  | Optional: \{\} <br /> |


#### ArchiveRecord



ArchiveRecord is one continuous archive that a server has written. A
recovery replays it from a base backup up to the requested point, so only a
point inside the interval of a record can be reached.



_Appears in:_
- [DatabaseServerArchiveStatus](#databaseserverarchivestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serverName` _string_ | ServerName is the archive directory in the bucket, equal to the name of<br />the CloudNativePG cluster that wrote it. |  |  |
| `objectStorageRef` _string_ | ObjectStorageRef is the ObjectStorageConfig, in the namespace of this<br />server, that holds this archive. A server that is pointed at another bucket closes this<br />record and opens one of its own, so every interval names the bucket a<br />restore of that interval has to read. |  | Optional: \{\} <br /> |
| `location` _string_ | Location is where in object storage this archive was written: the<br />provider, the bucket, and the path, as one URL. It is what decides<br />whether two intervals hold the same archive. An ObjectStorageConfig can<br />be edited in place or removed and created again under its name, so the<br />name alone does not say that. A record written before this field<br />existed carries none, and takes the location of the server on first<br />sight. |  | Optional: \{\} <br /> |
| `from` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | From is the earliest point this archive can be recovered to: when its<br />first base backup completed. |  |  |
| `to` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | To is the latest point this archive can be recovered to. It is unset<br />while the archive is the one the server writes to now. |  | Optional: \{\} <br /> |
| `unverifiedFrom` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | UnverifiedFrom is the point from which this archive can be missing<br />write-ahead log. It is set while CloudNativePG reports that the uploads<br />of the server are failing, and a restore to a point after it can reach<br />nothing. The plugin uploads the segments it held back once the uploads<br />run again, so the field is cleared then and the whole interval can be<br />reached again. Only the record the server writes to now carries it. |  | Optional: \{\} <br /> |


#### AttachedClusterStatus



AttachedClusterStatus is one CamundaCluster that clusterSelector matched.



_Appears in:_
- [CamundaManagementClusterStatus](#camundamanagementclusterstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the CamundaCluster. |  |  |
| `namespace` _string_ | Namespace is the namespace of the CamundaCluster. |  |  |
| `attached` _boolean_ | Attached reports whether the management plane serves this cluster.<br />Console lists it and Web Modeler deploys to it only while this is true. |  |  |
| `reason` _string_ | Reason names what the management plane found on this cluster. It is one<br />of five values. Four of them say why the cluster is not attached:<br />ClaimedElsewhere, another management plane holds it; NotReady, it<br />publishes no gateway endpoints or it changed while the operator claimed<br />it; InvalidReference, its platform config cannot be read, or the cluster<br />authenticates with OIDC and names another issuer than the management<br />plane; WriteFailed, the Console ping settings were refused. The fifth,<br />BasicAuthUserFailed, accompanies an attached row: the management plane<br />serves the cluster, and only the Web Modeler user on it is missing. |  | Optional: \{\} <br /> |
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
| `externalUrl` _string_ | ExternalURL is spec.externalUrl of that CamundaOptimize. The registered<br />callback is this URL plus the login path of Optimize. |  |  |


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
| `auth` _[AzureBlobStorageAuth](#azureblobstorageauth)_ | Auth selects how consumers authenticate. An absent block means<br />workload identity through the ServiceAccount chain. | \{ type:workloadIdentity \} | Optional: \{\} <br /> |


#### AzureBlobStorageAuth



AzureBlobStorageAuth selects how consumers authenticate against an Azure
Blob container.



_Appears in:_
- [AzureBlobStorage](#azureblobstorage)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ObjectStorageAuthType](#objectstorageauthtype)_ | Type is the authentication choice. Defaults to workloadIdentity. | workloadIdentity | Enum: [workloadIdentity credentials] <br />Optional: \{\} <br /> |
| `workloadIdentity` _[AzureBlobWorkloadIdentity](#azureblobworkloadidentity)_ | WorkloadIdentity names the trusted principal. Only valid with type<br />workloadIdentity; an empty or absent block means "trust the<br />ServiceAccount chain, add nothing". |  | Optional: \{\} <br /> |
| `credentials` _[AzureBlobCredentials](#azureblobcredentials)_ | Credentials is a static storage account key. Required with type<br />credentials, forbidden otherwise. |  | Optional: \{\} <br /> |


#### AzureBlobWorkloadIdentity



AzureBlobWorkloadIdentity names the Azure principal that the container
trusts. An empty block means the consumer's ServiceAccount chain already
carries the identity, so the operator adds nothing.



_Appears in:_
- [AzureBlobStorageAuth](#azureblobstorageauth)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clientId` _string_ | ClientID is the managed identity that consumers use. When set, the<br />operator puts it in the azure.workload.identity/client-id annotation<br />of the consumer's ServiceAccount. |  | Optional: \{\} <br /> |


#### BackupCredentialsSpec



BackupCredentialsSpec configures the backup credentials Secret, which is
created unless disabled.



_Appears in:_
- [DatabaseSpec](#databasespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `disabled` _boolean_ | Disabled skips creating the backup user and Secret. Defaults to false. |  | Optional: \{\} <br /> |
| `secretName` _string_ | SecretName is the name of the credentials Secret. Each controller<br />documents its kind-specific default derived from the CR name. |  | Optional: \{\} <br /> |


#### BackupDumpSpec



BackupDumpSpec is the cluster-level dump configuration. It holds the pod
settings that a backup can also set per run, plus the image that runs the
dump. The image is cluster-level policy on purpose. The Job runs under the
ServiceAccount of the cluster, with the cloud identity of the cluster and
its database credentials mounted. So the executable that the Job runs is
the choice of the cluster owner, never of the person who creates a backup.
A LogicalBackupRDBMS replaces the pod settings as a whole and always
inherits the image.



_Appears in:_
- [ClusterBackupSpec](#clusterbackupspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the dump pod. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the dump pod. In the<br />spec.dump of a LogicalBackupRDBMS, every name under PG or UPLOAD_ is<br />reserved, and admission rejects it. Any PG* name is connection policy,<br />because libpq reads PGHOSTADDR, PGSERVICE, PGOPTIONS, and more.<br />UPLOAD_* is the upload contract. The variables of a backup reach the<br />dump container only, never the container that uploads. Cloud SDKs<br />read endpoint, proxy, and configuration variables from the<br />environment, and a backup author must not steer where the dump goes.<br />In the spec.backup.dump of the cluster nothing is reserved, and the<br />variables reach every container. The cluster owner sets connection<br />policy, PGSSLMODE included, inside their own boundary.<br />This list stays atomic, unlike the extraEnv of a workload. One field<br />manager owns the whole list, because nothing else writes it: no<br />extension attaches to a dump pod. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources of the dump pod, at most<br />8. The cap applies to every block that uses this type, the cluster<br />and preset blocks included. It keeps the admission rule of a<br />LogicalBackupRDBMS inside the cost budget of the API server. In the<br />spec.dump of a LogicalBackupRDBMS, every source also needs a prefix<br />that cannot spell a PG* or UPLOAD_* name. The reason is that the<br />writer of the referenced object chooses its keys. Like ExtraEnv, the<br />sources of a backup reach the dump container only. The block of the<br />cluster has no prefix requirement, and its sources reach every<br />container. |  | MaxItems: 8 <br />Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the dump pod. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the dump pod. Set the<br />injection annotation of a service mesh to false here: a sidecar that<br />keeps running after the dump finishes stops the Job from completing. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling constraints of the dump pod. When set, it replaces the<br />block of a preset entirely (no merge). |  | Optional: \{\} <br /> |
| `scratchVolume` _[ScratchVolumeSpec](#scratchvolumespec)_ | ScratchVolume is where the dump is written before it is uploaded. When<br />set, it replaces the block of a preset entirely (no merge). The dump<br />pod runs with fsGroup 999, the postgres group. So pg_dump can write to<br />a volume that a storage class hands over root-owned. |  | Optional: \{\} <br /> |
| `activeDeadlineSeconds` _integer_ | ActiveDeadlineSeconds is the number of seconds that the dump Job can<br />run before it fails, counted from its start. When it is unset, the<br />operator applies 86400 (24 hours) when it renders the Job. The schema<br />does not apply the default, so an unset value inherits the value of a<br />preset. The default is large so that a very large dump completes. It<br />is not unbounded because a pod that cannot start uses no retry.<br />Without a deadline, a broken Job stays active as long as the backup<br />lives. A lower value fails a stuck dump sooner. A higher value gives a<br />long dump the time it needs. |  | Minimum: 1 <br />Optional: \{\} <br /> |
| `postgresImage` _string_ | PostgresImage is the full image reference of the dump container. It<br />replaces the default postgres:<major> of the upstream registry. An<br />air-gapped installation sets it, because the default reference cannot<br />be pulled there. |  | Optional: \{\} <br /> |


#### BackupPart



BackupPart is the observed state of one part of the backup set: the
web-application indices, the exported record indices, or the Zeebe
partitions.



_Appears in:_
- [LogicalBackupElasticsearchStatus](#logicalbackupelasticsearchstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `state` _[BackupPartState](#backuppartstate)_ | State of this part. |  | Enum: [Pending InProgress Completed Failed] <br />Optional: \{\} <br /> |
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
schedule and prunes the backups it created. At each trigger the operator
creates the backup kind that matches the storage type of the cluster,
named <schedule>-<unix-timestamp> and labeled camunda.io/cluster and
camunda.io/backup-schedule. A name that does not fit the bound of a
resource name or of a label value is cut, and a hash of the full name is
added, so two long names stay apart. The backups carry no owner reference
to the schedule, so deleting a schedule never deletes its backups. A
trigger is skipped, with an event, while the cluster is suspended or while
a backup of this schedule has not reached a terminal phase.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `BackupSchedule` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[BackupScheduleSpec](#backupschedulespec)_ | spec defines the desired state of BackupSchedule |  | Required: \{\} <br /> |
| `status` _[BackupScheduleStatus](#backupschedulestatus)_ | status defines the observed state of BackupSchedule |  | Optional: \{\} <br /> |


#### BackupScheduleSpec



BackupScheduleSpec is the backup policy of one cluster: when a backup is
created and how many the schedule keeps.



_Appears in:_
- [BackupSchedule](#backupschedule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clusterRef` _[ClusterRef](#clusterref)_ | ClusterRef references the CamundaCluster to back up. At each trigger<br />the operator creates the backup kind that matches the storage type of<br />the cluster: LogicalBackupElasticsearch or LogicalBackupRDBMS. |  | Required: \{\} <br /> |
| `schedule` _string_ | Schedule is when the backups run: a five-field cron expression<br />(minute, hour, day of month, month, day of week), evaluated in UTC. |  | Pattern: `^\s*([0-9A-Za-z*?,/-]+\s+)\{4\}[0-9A-Za-z*?,/-]+\s*$` <br />Required: \{\} <br /> |
| `retained` _[RetainedBackups](#retainedbackups)_ | Retained bounds how many backups of this schedule are kept. The<br />schedule prunes only the backups it created, matched by the<br />camunda.io/backup-schedule label. It never touches a backup that a<br />human created, and it never touches a backup that has not reached a<br />terminal phase. | \{  \} | Optional: \{\} <br /> |


#### BackupScheduleStatus



BackupScheduleStatus is the observed state of the schedule.



_Appears in:_
- [BackupSchedule](#backupschedule)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `lastScheduleTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | LastScheduleTime is the most recent trigger that the schedule<br />consumed, whether it created a backup or skipped. A skipped trigger is<br />never retried. |  | Optional: \{\} <br /> |
| `lastBackupName` _string_ | LastBackupName is the backup that the schedule created most recently. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. The Ready condition is Healthy<br />while the schedule can run its backups, and InvalidReference while a<br />reference does not resolve. |  | Optional: \{\} <br /> |


#### BasicAuthSpec



BasicAuthSpec configures the admin credential of a basic-auth cluster.



_Appears in:_
- [ClusterAuthSpec](#clusterauthspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `adminEmail` _string_ | AdminEmail is the email address of the admin user that the operator<br />seeds. The orchestration cluster stores it on that user, and the Admin<br />web application shows it. Set the address of the person or the team<br />that owns the cluster.<br />When empty the operator uses admin@example.com. The domain is the one<br />RFC 2606 reserves for documentation, so an unset value never claims an<br />address that somebody owns.<br />A changed value is applied to the running cluster through the user<br />API. That endpoint validates the address and refuses a domain without<br />a dot, and an address it refuses surfaces on AdminSecretReady with the<br />answer of the cluster. |  | MaxLength: 253 <br />Pattern: `^$\|^[^@[:space:]]+@[^@[:space:]]+\.[^@[:space:]]+$` <br />Optional: \{\} <br /> |
| `passwordRotation` _string_ | PasswordRotation requests one rotation of the admin password. Set it<br />to a value that differs from the applied one, for example a date. The<br />operator generates a new password, sets it on the admin user through<br />the user API of the running cluster, and then publishes it in the<br />admin Secret. The connectors Deployment restarts with the new<br />password. An empty value never rotates. A suspended cluster rotates<br />after it resumes.<br />The applied value of a cluster is status.adminPassword.rotation on<br />that cluster. This field takes its effective value from the preset<br />merge, so a preset that sets it rotates every cluster that references<br />the preset, and each of those clusters reports its own status. |  | MaxLength: 253 <br />Optional: \{\} <br /> |




#### CamundaCluster



CamundaCluster describes one Camunda orchestration cluster: the Zeebe
brokers, the gateway, the web applications, and optionally the connectors
runtime. The operator turns it into StatefulSets, Deployments, and Services
and keeps them converged.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `CamundaCluster` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[CamundaClusterSpec](#camundaclusterspec)_ | spec defines the desired state of CamundaCluster |  | Required: \{\} <br /> |
| `status` _[CamundaClusterStatus](#camundaclusterstatus)_ | status defines the observed state of CamundaCluster |  | Optional: \{\} <br /> |


#### CamundaClusterPreset



CamundaClusterPreset is a cluster-scoped, passive baseline configuration
for CamundaCluster resources: no controller reconciles it, it provisions
nothing and reports no status. A CamundaCluster resolves it through its
presetRef and merges its own fields over it under the rules of the preset
doc.





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
| `cluster` _[CamundaClusterSpec](#camundaclusterspec)_ | Cluster is the configuration baseline that referencing clusters<br />inherit. It reuses the CamundaCluster spec type so the two never drift<br />apart. The instance-bound fields of that type (platformConfigRef,<br />presetRef, releaseRef, externalUrl, serviceAccount, storageRef,<br />backupStorageRef, documentStorageRef, monitoring, suspend, pause) must<br />be left unset inside a preset, and so must the fields that belong to a<br />CamundaRelease (version, connectors.version). Explicit zero values (an<br />empty presetRef, suspend: false), as templated YAML renders unset<br />fields, count as unset. The CamundaCluster doc lists the field details,<br />the CamundaClusterPreset doc lists the merge rules. |  | Required: \{\} <br /> |


#### CamundaClusterSpec



CamundaClusterSpec defines the desired state of CamundaCluster.

The type doubles as the configuration baseline of a CamundaClusterPreset,
so fields that are required on a CamundaCluster (storageRef,
platformConfigRef) are optional at the schema level here and enforced on
the CamundaCluster usage instead. The instance-bound fields are cluster-only
and rejected in a preset.



_Appears in:_
- [CamundaCluster](#camundacluster)
- [CamundaClusterPresetSpec](#camundaclusterpresetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `platformConfigRef` _string_ | PlatformConfigRef names the cluster-scoped CamundaPlatformConfig that<br />provides auth, license, and image repositories. Required on a<br />CamundaCluster, forbidden in a preset. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br />Optional: \{\} <br /> |
| `presetRef` _string_ | PresetRef names a cluster-scoped CamundaClusterPreset used as the<br />configuration baseline. The preset doc lists the merge rules. |  | Optional: \{\} <br /> |
| `releaseRef` _string_ | ReleaseRef names a cluster-scoped CamundaRelease that provides the<br />versions, the pinned images, and the environment of a version. It<br />merges over the preset and under this spec. Forbidden in a preset. |  | Optional: \{\} <br /> |
| `version` _string_ | Version is the Camunda version to deploy, as a full semantic version.<br />The floor of 8.9.0 is enforced by the controller on the merged result,<br />the schema pins only the three-segment shape. Required unless the<br />resolved release provides it, and forbidden in a preset. A value below<br />the version that the brokers run is refused with Ready<br />VersionDowngradeRefused unless the annotation<br />camunda.io/allow-version-downgrade on the CamundaCluster names it. |  | Pattern: `^\d+\.\d+\.\d+$` <br />Optional: \{\} <br /> |
| `externalUrl` _string_ | ExternalURL is the external base URL of the cluster. The operator uses<br />it for OIDC redirect URLs and web application links. It creates no<br />Ingress. |  | Optional: \{\} <br /> |
| `serviceAccount` _[ServiceAccountSpec](#serviceaccountspec)_ | ServiceAccount configures the ServiceAccount of every workload. Bucket<br />access for backupStorageRef and documentStorageRef flows from this<br />identity. |  | Optional: \{\} <br /> |
| `auth` _[ClusterAuthSpec](#clusterauthspec)_ | Auth holds the credentials of this cluster and the identities that get<br />its admin role: the OIDC client credentials and administrators under<br />OIDC, the operator-owned admin credential under basic authentication. |  | Optional: \{\} <br /> |
| `zeebe` _[ZeebeSpec](#zeebespec)_ | Zeebe configures the brokers. |  | Optional: \{\} <br /> |
| `gateway` _[GatewaySpec](#gatewayspec)_ | Gateway configures the gateway. |  | Optional: \{\} <br /> |
| `operate` _[WebAppSpec](#webappspec)_ | Operate configures the Operate web application. |  | Optional: \{\} <br /> |
| `tasklist` _[WebAppSpec](#webappspec)_ | Tasklist configures the Tasklist web application. |  | Optional: \{\} <br /> |
| `admin` _[WebAppSpec](#webappspec)_ | Admin configures the Admin web application (Orchestration Cluster<br />Identity before Camunda 8.9). Its Spring profile is admin. |  | Optional: \{\} <br /> |
| `connectors` _[ConnectorsSpec](#connectorsspec)_ | Connectors configures the connectors runtime. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of every workload. A<br />per-component entry wins over an entry here with the same name.<br />The list merges by name under server-side apply, so each field manager<br />owns only the entries that it applies. An extension controller can add<br />its own entry next to yours. One applied manifest cannot hold two<br />entries with the same name.<br />A CamundaManagementCluster that serves this cluster owns the four<br />CAMUNDA_CONSOLE_PING_ entries (CAMUNDA_HUB_PING_ on Camunda 8.10 and<br />later) and replaces what you set under those names. It removes an entry<br />under one of those names that carries valueFrom, because one entry<br />cannot hold a value and a reference together.<br />Two field managers that apply the same name do not conflict: the merge<br />is per field inside the entry, so one manager can own value while the<br />other owns valueFrom. A container rejects an entry that carries both,<br />so the rule below refuses to store that combination. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of<br />every workload. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of every workload pod. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of every workload pod. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling constraints of every workload, unless a component sets its<br />own. When set, it replaces the scheduling block of a preset entirely<br />(no merge). |  | Optional: \{\} <br /> |
| `storageRef` _string_ | StorageRef names the SecondaryStorageConfig, in the namespace of this<br />cluster, that describes the secondary storage backend. Required on a<br />CamundaCluster, forbidden in a preset. One CamundaCluster holds one<br />backend, and two contracts that name one address are one backend. If<br />another cluster already holds the backend, the operator suspends this<br />cluster and reports Ready reason StorageAlreadyAttached until that<br />cluster moves to another backend or is deleted. |  | Optional: \{\} <br /> |
| `indexReplicas` _integer_ | IndexReplicas is the replica count of each index that the cluster<br />creates in an Elasticsearch secondary storage. When it is not set, the<br />nodeCount of the storage contract gives the count: 0 on one node, 1 on<br />two or more nodes. Without a nodeCount, Camunda keeps its own default.<br />When indexReplicas is not set, a process whose extraEnv sets a legacy<br />replica key keeps that value.<br />The cluster applies the count to its existing indices when it starts.<br />A count that the nodes cannot place is kept, and the cluster records an<br />IndexReplicasExceedNodes Warning event. A relational secondary storage<br />ignores it. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `backupStorageRef` _string_ | BackupStorageRef names an ObjectStorageConfig, in the namespace of<br />this cluster, for backups. |  | Optional: \{\} <br /> |
| `documentStorageRef` _string_ | DocumentStorageRef names an ObjectStorageConfig, in the namespace of<br />this cluster, for document storage. |  | Optional: \{\} <br /> |
| `backup` _[ClusterBackupSpec](#clusterbackupspec)_ | Backup configures how backups of this cluster behave: the schedule and<br />the retention of the primary-storage backups that Zeebe takes, and the<br />pod of the database dump Job. It is allowed in a preset, because it is<br />policy; backupStorageRef, which says where backups go, is not. The<br />block applies to a relational cluster: continuous and scheduled<br />primary-storage backups and the dump Job exist only there. |  | Optional: \{\} <br /> |
| `monitoring` _[ClusterMonitoringSpec](#clustermonitoringspec)_ | Monitoring configures the monitoring integrations. |  | Optional: \{\} <br /> |
| `suspend` _boolean_ | Suspend scales every workload to zero and keeps the data. Defaults to<br />false. An annotation with the prefix suspension-hold.camunda.io/ on the<br />CamundaCluster also suspends it, whatever this field says. |  | Optional: \{\} <br /> |
| `pause` _boolean_ | Pause halts the reconciliation of this cluster entirely and leaves the<br />workloads as they are. Defaults to false. |  | Optional: \{\} <br /> |


#### CamundaClusterStatus



CamundaClusterStatus is the observed state of a CamundaCluster.



_Appears in:_
- [CamundaCluster](#camundacluster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `volumes` _[VolumeStatus](#volumestatus) array_ | Volumes lists the bound broker PersistentVolumeClaims and the capacity<br />that each one reports, sorted by name. |  | Optional: \{\} <br /> |
| `management` _[ManagementBinding](#managementbinding)_ | Management is the published address of the management API of this<br />cluster. Extensions read it instead of rebuilding the Service name,<br />the port, and the authentication from the internals of this<br />controller. It is unset while the cluster is suspended, so a consumer<br />sees an unreachable cluster instead of a stale endpoint. |  | Optional: \{\} <br /> |
| `gateway` _[GatewayBinding](#gatewaybinding)_ | Gateway is the published address of the client APIs of this cluster.<br />Extensions read it instead of rebuilding the Service name and the ports<br />from the internals of this controller. It is unset while the cluster is<br />suspended, so a consumer sees an unreachable cluster instead of a stale<br />endpoint. |  | Optional: \{\} <br /> |
| `adminPassword` _[AdminPasswordStatus](#adminpasswordstatus)_ | AdminPassword is the state of the admin credential of a basic-auth<br />cluster. It is unset under OIDC. |  | Optional: \{\} <br /> |
| `serviceAccountName` _string_ | ServiceAccountName is the ServiceAccount that the pods of this cluster<br />run under, or empty when they run under the default account of the<br />namespace. Extensions that render a pod against this cluster read it<br />instead of rebuilding the rule from the spec and the buckets the<br />cluster references. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. Ready carries a pre-check<br />reason, or it is derived from the conditions of the components that the<br />cluster needs. The per-process conditions (ZeebeReady, GatewayReady,<br />OperateReady, TasklistReady, AdminReady, ConnectorsReady) also appear<br />here. |  | Optional: \{\} <br /> |


#### CamundaManagementCluster



CamundaManagementCluster describes one Camunda management plane: Management
Identity, an identity provider, and optionally Console and Web Modeler. The
operator turns it into Deployments and Services, writes the
ManagementAuthConfig that Optimize reads, and attaches the management plane
to the orchestration clusters that clusterSelector matches.

Creating a CamundaManagementCluster is a platform-administrator action,
because the selector reaches CamundaClusters in every namespace and the
operator annotates the ones it matches.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `CamundaManagementCluster` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[CamundaManagementClusterSpec](#camundamanagementclusterspec)_ | spec defines the desired state of CamundaManagementCluster |  | Required: \{\} <br /> |
| `status` _[CamundaManagementClusterStatus](#camundamanagementclusterstatus)_ | status defines the observed state of CamundaManagementCluster |  | Optional: \{\} <br /> |


#### CamundaManagementClusterSpec



CamundaManagementClusterSpec describes one management plane: Management
Identity, its identity provider, and optionally Console and Web Modeler.



_Appears in:_
- [CamundaManagementCluster](#camundamanagementcluster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `platformConfigRef` _string_ | PlatformConfigRef names the cluster-scoped CamundaPlatformConfig that<br />carries the license, the image settings, and, in the oidc mode, the<br />identity provider and every client of the management plane. |  | MinLength: 1 <br /> |
| `suspend` _boolean_ | Suspend scales every workload of this management cluster to zero. The<br />ManagementAuthConfig, the claims on the orchestration clusters, and the<br />Console ping settings stay, so nothing else has to change while the<br />management plane is down. |  | Optional: \{\} <br /> |
| `clusterSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#labelselector-v1-meta)_ | ClusterSelector selects the CamundaClusters, in every namespace that<br />namespaceSelector admits, that Console and Web Modeler serve. It<br />follows the Kubernetes label selector convention: an unset selector<br />selects no cluster, and an empty selector (\{\}) selects every cluster. |  | Optional: \{\} <br /> |
| `namespaceSelector` _[LabelSelector](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#labelselector-v1-meta)_ | NamespaceSelector narrows clusterSelector to the namespaces whose<br />labels match. It selects on the labels of the Namespace objects, the<br />way the namespaceSelector of an admission webhook does. An unset or<br />empty (\{\}) selector puts no bound on the namespace, so clusterSelector<br />alone decides. |  | Optional: \{\} <br /> |
| `managementAuthConfigName` _string_ | ManagementAuthConfigName is the name of the cluster-scoped<br />ManagementAuthConfig that this management cluster writes. A<br />CamundaOptimize reads it through its managementAuthRef. Empty means the<br />name of this resource. |  | Optional: \{\} <br /> |
| `identityProvider` _[IdentityProviderSpec](#identityproviderspec)_ | IdentityProvider selects where users authenticate. Exactly one of<br />keycloak, externalKeycloak, or oidc is set. |  |  |
| `identity` _[IdentitySpec](#identityspec)_ | Identity configures Management Identity. Console, Web Modeler, and<br />Optimize all authenticate through it, so it is always deployed. |  |  |
| `console` _[ConsoleSpec](#consolespec)_ | Console configures Console. Console is not deployed while this is unset. |  | Optional: \{\} <br /> |
| `webModeler` _[WebModelerSpec](#webmodelerspec)_ | WebModeler configures Web Modeler. Web Modeler is not deployed while<br />this is unset. |  | Optional: \{\} <br /> |


#### CamundaManagementClusterStatus



CamundaManagementClusterStatus is the observed state of a
CamundaManagementCluster.



_Appears in:_
- [CamundaManagementCluster](#camundamanagementcluster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `managementAuthConfig` _string_ | ManagementAuthConfig is the name of the ManagementAuthConfig that this<br />management cluster writes. A CamundaOptimize reads it through its<br />managementAuthRef. |  | Optional: \{\} <br /> |
| `clusters` _[AttachedClusterStatus](#attachedclusterstatus) array_ | Clusters lists every CamundaCluster that clusterSelector matched, and<br />reports whether the management plane serves it. |  | Optional: \{\} <br /> |
| `optimize` _[AttachedOptimizeStatus](#attachedoptimizestatus) array_ | Optimize lists every CamundaOptimize that names the ManagementAuthConfig<br />of this management cluster and sets spec.externalUrl, ordered by<br />namespace and name. The management plane registers the login callback of<br />each one on the Optimize client of the realm.<br />The list holds no row in the oidc mode, where the identity provider of<br />the platform config holds the callback URLs. |  | Optional: \{\} <br /> |
| `callbackRealm` _[KeycloakRealmTarget](#keycloakrealmtarget)_ | CallbackRealm is the Keycloak realm that this management plane last<br />pointed Management Identity at, in the externalKeycloak mode. Identity<br />registers the login callbacks of Optimize there while it starts, so the<br />field appears with the realm and not with the first registration. A<br />non-nil value does not say that the callbacks are still there: when the<br />spec names another realm, or the oidc mode, the operator removes them<br />from this realm first, and the field then outlives the emptied realm<br />until nothing of that realm's configuration can put them back: no<br />Management Identity pod of it that can still run, and no Deployment or<br />ReplicaSet of it that can still start one. It goes when that drain<br />is over, so its disappearance, or its move to the new realm, is the<br />completion signal of a move.<br />It is absent once a move into the keycloak or the oidc mode is over,<br />and while no login callback of this operator is registered anywhere and<br />nothing remains that could write them back. The operator runs the<br />Keycloak of the keycloak mode and deletes it with the management plane<br />or with a move away from that mode, so that realm is never recorded.<br />During a move into either mode the field still names the realm that the<br />plane is leaving. A Keycloak that is gone for good never answers, so<br />the annotation camunda.io/forget-callback-realm, set to the value that<br />the OptimizeCallbacksReady message names, lets go of it with the<br />callbacks still in it. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. Ready carries a pre-check<br />reason, or it is derived from the conditions of the deployed components.<br />The per-component conditions (KeycloakReady, IdentityReady,<br />ConsoleReady, WebModelerReady, ManagementAuthReady, SecretsReady,<br />MirroredSecretsReady) and OptimizeCallbacksReady also appear here. |  | Optional: \{\} <br /> |


#### CamundaOptimize



CamundaOptimize describes one Camunda Optimize instance attached to one
CamundaCluster. The operator turns it into a webapp Deployment, an importer
Deployment, and their Services. It also turns on the Elasticsearch exporter
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
from the CamundaPlatformConfig of the referenced cluster, so the two cannot
disagree.



_Appears in:_
- [CamundaOptimize](#camundaoptimize)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version is the Optimize version to deploy, as a full semantic version.<br />Optimize has its own patch line, so it does not follow the version of<br />the cluster. The major and the minor must match the effective version<br />of the referenced cluster; the controller reports VersionMismatch when<br />they differ. |  | Pattern: `^\d+\.\d+\.\d+$` <br /> |
| `managementAuthRef` _string_ | ManagementAuthRef names the cluster-scoped ManagementAuthConfig that<br />provides the Management Identity OIDC configuration. Optimize<br />authenticates against Management Identity, not against the built-in<br />auth of the orchestration cluster. |  | MinLength: 1 <br /> |
| `externalUrl` _string_ | ExternalURL is the URL that browsers reach this Optimize at.<br />In the two Keycloak modes the management plane behind managementAuthRef<br />registers <externalUrl>/api/authentication/callback on the optimize<br />client of the realm, so a person who signs in here comes back here. An<br />Optimize that sets no URL gets no callback from that plane, so Keycloak<br />refuses the return, unless somebody put that callback in the realm by<br />hand.<br />In the oidc mode the field has no effect. The identity provider of the<br />platform config holds the callback URLs, so add this one there. |  | Optional: \{\} <br /> |
| `clusterRef` _[ClusterRef](#clusterref)_ | ClusterRef names the CamundaCluster that this Optimize instance reads.<br />The secondary storage of that cluster must be Elasticsearch, and no<br />other CamundaOptimize may be attached to it.<br />The reference is immutable. A repoint would apply the exporter settings<br />to the new cluster while the old cluster keeps the settings this<br />operator applied, and it would change the pod selectors of the<br />Deployments, which Kubernetes does not allow. |  |  |
| `webapp` _[WorkloadSpec](#workloadspec)_ | Webapp configures the Deployment that serves the Optimize user<br />interface. It runs with data import off. |  | Optional: \{\} <br /> |
| `importer` _[WorkloadSpec](#workloadspec)_ | Importer configures the Deployment that imports the exported cluster<br />data into the Optimize indices. Optimize supports one active importer,<br />so replicas must be 0 or 1. Set 0 to stop the import, for example while<br />a restore or an index rewrite runs; the webapp keeps serving what is<br />already imported. |  | Optional: \{\} <br /> |
| `indexReplicas` _integer_ | IndexReplicas is the replica count of each Optimize index, and of each<br />zeebe-record index that the exporter of the cluster writes for this<br />Optimize. When it is not set, the nodeCount of the storage contract of<br />the cluster gives the count: 0 on one node, 1 on two or more nodes.<br />Without a nodeCount, Optimize and the exporter keep their own defaults.<br />Optimize applies the count to its existing indices when it starts. The<br />exporter applies it to the zeebe-record indices that it creates next. A<br />count that the nodes cannot place is kept, and Optimize records an<br />IndexReplicasExceedNodes Warning event. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `monitoring` _[OptimizeMonitoringSpec](#optimizemonitoringspec)_ | Monitoring configures the monitoring integrations. |  | Optional: \{\} <br /> |


#### CamundaOptimizeStatus



CamundaOptimizeStatus is the observed state of a CamundaOptimize.



_Appears in:_
- [CamundaOptimize](#camundaoptimize)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. Ready carries a pre-check<br />reason, or it is derived from the conditions of the two workloads.<br />The per-workload conditions (WebappReady, ImporterReady) also appear<br />here. |  | Optional: \{\} <br /> |
| `suspendedBy` _[OptimizeSuspension](#optimizesuspension)_ | SuspendedBy says why the Optimize workloads follow the referenced<br />cluster to zero. It is empty while they follow their spec. It stays set<br />while a failed check keeps them at zero after the cluster resumed. |  | Enum: [Cluster StorageClaim] <br />Optional: \{\} <br /> |


#### CamundaPlatformConfig



CamundaPlatformConfig is the cluster-scoped CRD that holds the
environment-wide platform settings, the identity provider, the license, and
the image repositories, that every orchestration cluster referencing it
shares.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `CamundaPlatformConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[CamundaPlatformConfigSpec](#camundaplatformconfigspec)_ | spec defines the desired state of CamundaPlatformConfig |  | Required: \{\} <br /> |
| `status` _[CamundaPlatformConfigStatus](#camundaplatformconfigstatus)_ | status defines the observed state of CamundaPlatformConfig |  | Optional: \{\} <br /> |


#### CamundaPlatformConfigSpec



CamundaPlatformConfigSpec holds the settings that are identical across all
orchestration clusters of an environment.



_Appears in:_
- [CamundaPlatformConfig](#camundaplatformconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `auth` _[PlatformAuthSpec](#platformauthspec)_ | Auth holds the authentication settings of the orchestration clusters.<br />Unset means basic authentication. |  | Optional: \{\} <br /> |
| `licenseSecretRef` _[SecretKeyRef](#secretkeyref)_ | LicenseSecretRef names the Secret key that holds the Camunda license<br />key. Without it, clusters run in unlicensed non-production mode. |  | Optional: \{\} <br /> |
| `images` _[ImagesSpec](#imagesspec)_ | Images renames the images that the operator pulls, one entry per<br />image, for example to a mirror. Each value is a full repository with<br />its registry and no tag. The tag always comes from the version field<br />of the resource that runs the image. A value that names a registry<br />with a port needs a path after the port, as in<br />registry:5000/camunda/optimize. The tag goes on the end of the value,<br />so the bare registry:5000 becomes the image registry:5000:<version>. |  | Optional: \{\} <br /> |


#### CamundaPlatformConfigStatus



CamundaPlatformConfigStatus is the observed validation state of the
platform config.



_Appears in:_
- [CamundaPlatformConfig](#camundaplatformconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current validation state; the Ready condition<br />carries reasons Healthy or MissingSecret. |  | Optional: \{\} <br /> |


#### CamundaRelease



CamundaRelease is a cluster-scoped, passive description of what a platform
runs: the versions, the pinned images, and the environment a version needs.
No controller reconciles it, it provisions nothing and reports no status. A
CamundaCluster, an ElasticsearchCluster, and a DatabaseServer each resolve
it through their releaseRef and merge it between their preset and their own
spec.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `CamundaRelease` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[CamundaReleaseSpec](#camundareleasespec)_ | spec defines the desired state of CamundaRelease |  | Required: \{\} <br /> |


#### CamundaReleaseSpec



CamundaReleaseSpec holds what a platform runs: the version of the
orchestration cluster, the versions of the storage it runs on, the image
references that replace the ones the versions produce, and the environment
that a version needs.



_Appears in:_
- [CamundaRelease](#camundarelease)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version is the Camunda version of the orchestration cluster processes,<br />as a full semantic version. The floor of 8.9.0 is enforced by the<br />controller of each referencing cluster on the merged spec. |  | Pattern: `^\d+\.\d+\.\d+$` <br />Required: \{\} <br /> |
| `connectors` _[ReleaseConnectorsSpec](#releaseconnectorsspec)_ | Connectors holds the version and the environment of the connectors<br />runtime. |  | Optional: \{\} <br /> |
| `elasticsearch` _[ReleaseElasticsearchSpec](#releaseelasticsearchspec)_ | Elasticsearch holds the version of the Elasticsearch clusters of this<br />release. An ElasticsearchCluster takes it through its releaseRef. |  | Optional: \{\} <br /> |
| `databaseServer` _[ReleaseDatabaseServerSpec](#releasedatabaseserverspec)_ | DatabaseServer holds the version of the PostgreSQL servers of this<br />release. A DatabaseServer takes it through its releaseRef. |  | Optional: \{\} <br /> |
| `images` _[ReleaseImagesSpec](#releaseimagesspec)_ | Images replaces the image reference of a process. An entry is used as<br />it is, tag or digest included. It changes only what is pulled: the<br />version above stays the version that the operator believes the process<br />runs, for the version gates, the downgrade rule, and the environment<br />that the operator computes. |  | Optional: \{\} <br /> |
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
the identities of the administrators. Under basic authentication it
carries the basic block, which configures the admin credential that the
operator owns.



_Appears in:_
- [CamundaClusterSpec](#camundaclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clientId` _string_ | ClientID is the OIDC client ID of this cluster. |  | Optional: \{\} <br /> |
| `audience` _string_ | Audience is the audience that access tokens must carry. Defaults to<br />the clientId. |  | Optional: \{\} <br /> |
| `clientSecretRef` _[LocalSecretKeyRef](#localsecretkeyref)_ | ClientSecretRef names the Secret that holds the OIDC client secret of<br />this cluster. |  | Optional: \{\} <br /> |
| `admin` _[ClusterAdminSpec](#clusteradminspec)_ | Admin holds the identities that get the admin role of this cluster. It<br />applies under OIDC only. Basic authentication seeds its own<br />administrator and ignores this block. |  | Optional: \{\} <br /> |
| `basic` _[BasicAuthSpec](#basicauthspec)_ | Basic configures the admin credential that the operator owns. It<br />applies under basic authentication only. OIDC ignores this block, like<br />basic authentication ignores admin. |  | Optional: \{\} <br /> |


#### ClusterBackupSpec



ClusterBackupSpec configures how backups of one orchestration cluster
behave. Where they are written is spec.backupStorageRef; this block is the
policy, so a preset can hold it for a fleet of clusters.



_Appears in:_
- [CamundaClusterSpec](#camundaclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `primaryStorage` _[PrimaryStorageBackupSpec](#primarystoragebackupspec)_ | PrimaryStorage configures the backups that Zeebe takes of its own<br />primary storage. They are the feed of a point-in-time restore. |  | Optional: \{\} <br /> |
| `dump` _[BackupDumpSpec](#backupdumpspec)_ | Dump configures the Job that writes the logical database to the backup<br />bucket. A LogicalBackupRDBMS can replace this block as a whole. |  | Optional: \{\} <br /> |


#### ClusterMonitoringSpec



ClusterMonitoringSpec groups the monitoring integrations of a
CamundaCluster.



_Appears in:_
- [CamundaClusterSpec](#camundaclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serviceMonitor` _[ServiceMonitorSpec](#servicemonitorspec)_ | ServiceMonitor configures the Prometheus ServiceMonitors. When<br />enabled, the operator creates one ServiceMonitor per process, named<br />like the workload, that scrapes /actuator/prometheus on the management<br />port 9600 of a unified process and on the HTTP port 8080 of<br />connectors. |  | Optional: \{\} <br /> |


#### ClusterRef



ClusterRef references a CamundaCluster by name, in the namespace of the
referencing object. The reference cannot cross namespaces on purpose: for
a backup the operator reads the cluster's Secrets, runs a Job in its
namespace, and calls its management API, so the reference stays inside
the RBAC boundary the CR itself lives in. Whoever may create the CR in a
namespace may back up the clusters of that namespace, and no others.



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
| `name` _string_ | Name of the CamundaCluster, in the namespace of this object. |  | MinLength: 1 <br /> |


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
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the container of this<br />process. On a CamundaCluster, an entry here wins over a top-level entry<br />with the same name. The entries of an embedded web application apply to<br />its host process.<br />The list merges by name under server-side apply, so each field manager<br />owns only the entries that it applies. An extension controller can add<br />its own entry next to yours. One applied manifest cannot hold two<br />entries with the same name.<br />Two field managers that apply the same name do not conflict: the merge<br />is per field inside the entry, so one manager can own value while the<br />other owns valueFrom. A container rejects an entry that carries both,<br />so the rule below refuses to store that combination. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the<br />container of this process. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the pods of this process. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the pods of this process. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling constraints of the pods of this process. When set, it<br />replaces every scheduling block that would otherwise apply to this<br />process, with no merge. On a CamundaCluster those are the top-level<br />block and the block of a preset. |  | Optional: \{\} <br /> |


#### ConsoleSpec



ConsoleSpec configures Console.



_Appears in:_
- [CamundaManagementClusterSpec](#camundamanagementclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version is the Console version, as a full semantic version. The<br />operator supports 8.9.0 and later. |  | Pattern: `^\d+\.\d+\.\d+$` <br /> |
| `externalUrl` _string_ | ExternalURL is the URL that browsers reach Console at. Console serves<br />under the path of this URL. Every selected CamundaCluster reports to<br />Console over the Service of the Kubernetes cluster, so the operator<br />needs no Ingress in front of it. |  |  |
| `replicas` _integer_ | Replicas is the number of pods of this process. Defaults to 1. On a<br />CamundaCluster it has no effect on an embedded web application. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the container of this process. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the container of this<br />process. On a CamundaCluster, an entry here wins over a top-level entry<br />with the same name. The entries of an embedded web application apply to<br />its host process.<br />The list merges by name under server-side apply, so each field manager<br />owns only the entries that it applies. An extension controller can add<br />its own entry next to yours. One applied manifest cannot hold two<br />entries with the same name.<br />Two field managers that apply the same name do not conflict: the merge<br />is per field inside the entry, so one manager can own value while the<br />other owns valueFrom. A container rejects an entry that carries both,<br />so the rule below refuses to store that combination. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the<br />container of this process. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the pods of this process. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the pods of this process. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling constraints of the pods of this process. When set, it<br />replaces every scheduling block that would otherwise apply to this<br />process, with no merge. On a CamundaCluster those are the top-level<br />block and the block of a preset. |  | Optional: \{\} <br /> |


#### CredentialsSpec



CredentialsSpec names the Secret a controller writes generated credentials
to (keys username and password). The Secret lives in the namespace of the
CR.



_Appears in:_
- [BackupCredentialsSpec](#backupcredentialsspec)
- [DatabaseSpec](#databasespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretName` _string_ | SecretName is the name of the credentials Secret. Each controller<br />documents its kind-specific default derived from the CR name. |  | Optional: \{\} <br /> |


#### Database



Database bootstraps a logical database and its users on an existing
PostgreSQL server over plain SQL and publishes the result as a
DatabaseConfig — and optionally a SecondaryStorageConfig — in its own
namespace. Deletion garbage-collects the published bindings and Secrets
through owner references but never drops the logical database or the SQL
users.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `Database` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[DatabaseSpec](#databasespec)_ | spec defines the desired state of Database |  | Required: \{\} <br /> |
| `status` _[DatabaseStatus](#databasestatus)_ | status defines the observed state of Database |  | Optional: \{\} <br /> |


#### DatabaseConfig



DatabaseConfig is the namespaced contract CRD that describes one logical
database — its server, name, and application credentials — for the
controllers and components that connect to it. Consumers resolve references
to it by name in their own namespace.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `DatabaseConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[DatabaseConfigSpec](#databaseconfigspec)_ | spec defines the desired state of DatabaseConfig |  | Required: \{\} <br /> |
| `status` _[DatabaseConfigStatus](#databaseconfigstatus)_ | status defines the observed state of DatabaseConfig |  | Optional: \{\} <br /> |


#### DatabaseConfigSpec



DatabaseConfigSpec describes one logical database: its server, name, and
application credentials.



_Appears in:_
- [DatabaseConfig](#databaseconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serverRef` _string_ | ServerRef names the DatabaseServerConfig describing the server hosting<br />this database. |  | MinLength: 1 <br /> |
| `databaseName` _string_ | DatabaseName is the name of the logical database on the server. |  | MinLength: 1 <br /> |
| `credentialsSecretRef` _[LocalCredentialsSecretRef](#localcredentialssecretref)_ | CredentialsSecretRef names an application user with read/write access to<br />the database. |  |  |
| `backupCredentialsSecretRef` _[LocalCredentialsSecretRef](#localcredentialssecretref)_ | BackupCredentialsSecretRef names a separate user with dump/restore<br />privileges, used by the backup and restore controllers. |  | Optional: \{\} <br /> |


#### DatabaseConfigStatus



DatabaseConfigStatus is the observed validation state of the contract.



_Appears in:_
- [DatabaseConfig](#databaseconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current validation state; the Ready condition<br />carries reasons Healthy, InvalidReference, or MissingSecret. |  | Optional: \{\} <br /> |


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
CloudNativePG operator, archives it continuously to an object storage
bucket, and publishes the connection details as a DatabaseServerConfig that
a Database and a PointInTimeRestore consume. One or more orchestration
clusters use the instance, each through a Database of its own.

A PointInTimeRestore rolls the whole instance back, so it needs a server
that holds the database of its cluster and nothing else.

The name of the CR names the CloudNativePG cluster, so admission holds it to
a DNS-1035 label of at most 46 characters: the 50 that CloudNativePG accepts
for a cluster name, less the four of the "-r99" that a rollback appends. A
name inside that bound reaches the cluster of a rollback whole while the
recovery index stays below 100, and is shortened to a head and a hash above
that. The index counts the archive records of the server, so a rollback, an
archive the spec re-enables, and a change of bucket each add one.

The rule runs on create only. A name never changes on update. If the rule
runs there too, it rejects an edit of another field on an object that
predates it, and nothing more.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `DatabaseServer` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[DatabaseServerSpec](#databaseserverspec)_ | spec defines the desired state of DatabaseServer |  | Required: \{\} <br /> |
| `status` _[DatabaseServerStatus](#databaseserverstatus)_ | status defines the observed state of DatabaseServer |  | Optional: \{\} <br /> |


#### DatabaseServerArchiveSpec



DatabaseServerArchiveSpec points a server at the bucket that holds its
continuous archive: the write-ahead log of every instance, plus the base
backups a recovery starts from. A server with an archive publishes
pitr.enabled true on its contract; a server without one publishes false and
no point-in-time restore can reach it.

The archive is not the backup model of the operator. BackupSchedule and
LogicalBackupRDBMS take logical dumps and never see these base backups.



_Appears in:_
- [DatabaseServerSpec](#databaseserverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `objectStorageRef` _string_ | ObjectStorageRef names an ObjectStorageConfig in the namespace of this<br />server. The operator writes the archive under a prefix of that bucket<br />that holds this server alone. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br /> |
| `retentionPeriodDays` _integer_ | RetentionPeriodDays is how far into the past a restore can reach. It is<br />what the operator enforces on the bucket and what the contract of this<br />server publishes, so the declared value and the enforced value are one.<br />The maximum is 36500 days, which is a hundred years. The operator counts<br />the reachable window in nanoseconds, and a longer period overflows that<br />count and puts the oldest reachable point in the future, which makes<br />every request unreachable. |  | Maximum: 36500 <br />Minimum: 1 <br /> |
| `baseBackupSchedule` _string_ | BaseBackupSchedule is when a base backup is taken, as the six-field<br />cron of CloudNativePG (seconds first, in UTC), or as one of the<br />descriptors @yearly, @annually, @monthly, @weekly, @daily, @midnight,<br />@hourly and @every. The first base backup runs as soon as the server is<br />up, whatever the schedule says, because the archive can be recovered<br />from only after it completes.<br />The five-field cron of a Kubernetes CronJob is refused. CloudNativePG<br />reads the first field as seconds, so a five-field value runs at a<br />different time from the one its author meant.<br />Each field is bounded to what CloudNativePG takes there: 0-59 for<br />seconds and minutes, 0-23 for hours, 1-31 for the day of the month,<br />1-12 or JAN-DEC for the month, and 0-6 or SUN-SAT for the day of the<br />week. A range whose first value is above its second, such as FRI-MON,<br />cannot be caught by a pattern. The operator refuses it instead, and<br />Ready reports InvalidReference before anything is applied.<br />A step takes at most three digits and an @every number at most six<br />digits on each side of the point. Both are stricter than the parser of<br />CloudNativePG, which reads a longer number and then overflows on it and<br />stops taking base backups. | 0 0 2 * * * | Pattern: `^(\s*([*?]\|[0-5]?\d(-[0-5]?\d)?)(/[1-9]\d\{0,2\})?(,([*?]\|[0-5]?\d(-[0-5]?\d)?)(/[1-9]\d\{0,2\})?)*\s+([*?]\|[0-5]?\d(-[0-5]?\d)?)(/[1-9]\d\{0,2\})?(,([*?]\|[0-5]?\d(-[0-5]?\d)?)(/[1-9]\d\{0,2\})?)*\s+([*?]\|([01]?\d\|2[0-3])(-([01]?\d\|2[0-3]))?)(/[1-9]\d\{0,2\})?(,([*?]\|([01]?\d\|2[0-3])(-([01]?\d\|2[0-3]))?)(/[1-9]\d\{0,2\})?)*\s+([*?]\|([1-9]\|[12]\d\|3[01])(-([1-9]\|[12]\d\|3[01]))?)(/[1-9]\d\{0,2\})?(,([*?]\|([1-9]\|[12]\d\|3[01])(-([1-9]\|[12]\d\|3[01]))?)(/[1-9]\d\{0,2\})?)*\s+([*?]\|([1-9]\|1[0-2]\|[Jj]([Aa][Nn]\|[Uu][LlNn])\|[Ff][Ee][Bb]\|[Mm][Aa][RrYy]\|[Aa]([Pp][Rr]\|[Uu][Gg])\|[Ss][Ee][Pp]\|[Oo][Cc][Tt]\|[Nn][Oo][Vv]\|[Dd][Ee][Cc])(-([1-9]\|1[0-2]\|[Jj]([Aa][Nn]\|[Uu][LlNn])\|[Ff][Ee][Bb]\|[Mm][Aa][RrYy]\|[Aa]([Pp][Rr]\|[Uu][Gg])\|[Ss][Ee][Pp]\|[Oo][Cc][Tt]\|[Nn][Oo][Vv]\|[Dd][Ee][Cc]))?)(/[1-9]\d\{0,2\})?(,([*?]\|([1-9]\|1[0-2]\|[Jj]([Aa][Nn]\|[Uu][LlNn])\|[Ff][Ee][Bb]\|[Mm][Aa][RrYy]\|[Aa]([Pp][Rr]\|[Uu][Gg])\|[Ss][Ee][Pp]\|[Oo][Cc][Tt]\|[Nn][Oo][Vv]\|[Dd][Ee][Cc])(-([1-9]\|1[0-2]\|[Jj]([Aa][Nn]\|[Uu][LlNn])\|[Ff][Ee][Bb]\|[Mm][Aa][RrYy]\|[Aa]([Pp][Rr]\|[Uu][Gg])\|[Ss][Ee][Pp]\|[Oo][Cc][Tt]\|[Nn][Oo][Vv]\|[Dd][Ee][Cc]))?)(/[1-9]\d\{0,2\})?)*\s+([*?]\|([0-6]\|[Ss]([Uu][Nn]\|[Aa][Tt])\|[Mm][Oo][Nn]\|[Tt]([Uu][Ee]\|[Hh][Uu])\|[Ww][Ee][Dd]\|[Ff][Rr][Ii])(-([0-6]\|[Ss]([Uu][Nn]\|[Aa][Tt])\|[Mm][Oo][Nn]\|[Tt]([Uu][Ee]\|[Hh][Uu])\|[Ww][Ee][Dd]\|[Ff][Rr][Ii]))?)(/[1-9]\d\{0,2\})?(,([*?]\|([0-6]\|[Ss]([Uu][Nn]\|[Aa][Tt])\|[Mm][Oo][Nn]\|[Tt]([Uu][Ee]\|[Hh][Uu])\|[Ww][Ee][Dd]\|[Ff][Rr][Ii])(-([0-6]\|[Ss]([Uu][Nn]\|[Aa][Tt])\|[Mm][Oo][Nn]\|[Tt]([Uu][Ee]\|[Hh][Uu])\|[Ww][Ee][Dd]\|[Ff][Rr][Ii]))?)(/[1-9]\d\{0,2\})?)*\s*\|@((year\|annual\|month\|week\|dai\|hour)ly\|midnight\|every (\d\{1,6\}(\.\d\{1,6\})?[hms])+))$` <br />Optional: \{\} <br /> |


#### DatabaseServerArchiveStatus



DatabaseServerArchiveStatus is the observed state of the archive of a
server.



_Appears in:_
- [DatabaseServerStatus](#databaseserverstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `history` _[ArchiveRecord](#archiverecord) array_ | History lists every archive the server has written, oldest first. A<br />recovery picks the record whose interval holds the requested point.<br />Records are never removed, so a restore can reach back across an<br />earlier recovery for as long as the bucket keeps the objects. Removing<br />spec.archive closes the record that is open and keeps the list, and<br />asking for an archive again opens a record of its own. Pointing<br />spec.archive at another bucket does the same. The window with no<br />archive therefore lies inside no interval, and no restore can reach a<br />point in it. |  | Optional: \{\} <br /> |
| `boundary` _[ArchiveBoundary](#archiveboundary)_ | Boundary is the last move of the archive that no record holds yet:<br />spec.archive was re-enabled on another location, or the location moved<br />again before a base backup opened a record. It is what keeps a base<br />backup of the location the server left from opening the interval of the<br />one it moved to. It is cleared when that interval opens. |  | Optional: \{\} <br /> |
| `reachableFrom` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | ReachableFrom is the oldest point the objects in the bucket still go<br />back to. The retention period prunes the bucket as it runs, and a<br />raised retention period does not bring back what a shorter one already<br />pruned. The window grows to the new retention period only as the<br />archive writes past this point, and a rollback to a point before it is<br />refused. It is unset on a server that archived before this field<br />existed, and the retention period alone bounds that one. |  | Optional: \{\} <br /> |


#### DatabaseServerConfig



DatabaseServerConfig is the contract CRD that describes a database server —
engine, endpoint, admin credentials, and point-in-time-recovery capability —
for controllers that bootstrap databases on it or validate its declared
capabilities. A consumer resolves it in its own namespace, so the whole
RDBMS chain of a cluster lives with that cluster.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `DatabaseServerConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[DatabaseServerConfigSpec](#databaseserverconfigspec)_ | spec defines the desired state of DatabaseServerConfig |  | Required: \{\} <br /> |
| `status` _[DatabaseServerConfigStatus](#databaseserverconfigstatus)_ | status defines the observed state of DatabaseServerConfig |  | Optional: \{\} <br /> |


#### DatabaseServerConfigSpec



DatabaseServerConfigSpec describes a database server: engine, endpoint,
admin credentials, and point-in-time-recovery capability.



_Appears in:_
- [DatabaseServerConfig](#databaseserverconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `engine` _[DatabaseEngine](#databaseengine)_ | Engine is the database engine of the server. See DatabaseEngine for<br />the accepted values. |  | Enum: [postgres] <br /> |
| `host` _string_ | Host the server is reachable at. |  | MinLength: 1 <br /> |
| `port` _integer_ | Port the server listens on. |  | Maximum: 65535 <br />Minimum: 1 <br /> |
| `adminCredentialsSecretRef` _[LocalCredentialsSecretRef](#localcredentialssecretref)_ | AdminCredentialsSecretRef names an admin user with permission to create<br />databases and roles; used by the Database controller to bootstrap. The<br />Secret lives in the namespace of this contract. |  |  |
| `pitr` _[PITRCapability](#pitrcapability)_ | PITR declares the server's point-in-time-recovery capability. |  | Optional: \{\} <br /> |
| `recovery` _[RecoveryRequest](#recoveryrequest)_ | Recovery asks for the server to be rolled back to a point in time. A<br />consumer writes it. It is answered in pitr.lastRecovery, and only when<br />pitr.recovery is operator. It stays on the contract after the answer,<br />as the record of the last request. |  | Optional: \{\} <br /> |


#### DatabaseServerConfigStatus



DatabaseServerConfigStatus is the observed validation state of the contract.
It holds what the operator read from the server the last time it reached it.



_Appears in:_
- [DatabaseServerConfig](#databaseserverconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `serverVersion` _string_ | ServerVersion is the major version that the server reported the last<br />time the operator reached it, for example "17". A dump of a database on<br />this server runs client tools of this major, so a backup waits until<br />the operator publishes it. |  | Optional: \{\} <br /> |
| `systemIdentifier` _string_ | SystemIdentifier is the identity of the PostgreSQL instance behind this<br />endpoint, as the server reported it on the last probe. It names the<br />server itself, so two contracts that describe one server under<br />different hosts publish one value. The Database controller keys the<br />uniqueness of a logical database on it. A change to the endpoint or to<br />the admin credentials clears it. |  | Optional: \{\} <br /> |
| `probedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | ProbedAt is when the operator last reached the server and read<br />ServerVersion and SystemIdentifier. The operator probes the server again<br />when this is older than the probe interval, or when the admin<br />credentials Secret changed. A reconcile in between leaves it untouched.<br />A change to the endpoint or to the admin credentials clears it, together<br />with ServerVersion, SystemIdentifier, ProbedEndpoint, ProbedSecretName,<br />ProbedSecretKeys, and ProbedSecretVersion, because the whole record<br />describes the server the old spec named. A change to any other field,<br />for example a recovery request, leaves the record alone: it cannot move<br />the server. |  | Optional: \{\} <br /> |
| `probedEndpoint` _string_ | ProbedEndpoint is the host and the port that the last probe reached, as<br />"<host>:<port>". It is what tells a spec change that moves the server<br />from one that does not. |  | Optional: \{\} <br /> |
| `probedSecretName` _string_ | ProbedSecretName is the admin credentials Secret that the last probe<br />read. A spec that names another Secret names other credentials, so the<br />record of the probe goes with it. |  | Optional: \{\} <br /> |
| `probedSecretKeys` _string_ | ProbedSecretKeys are the keys of that Secret that the last probe read,<br />as "<usernameKey>/<passwordKey>". One Secret can hold the credentials of<br />more than one user, so the keys name the user the way the Secret names<br />the credentials. |  | Optional: \{\} <br /> |
| `probedSecretVersion` _string_ | ProbedSecretVersion is the resourceVersion of the admin credentials<br />Secret that the last probe used. The operator probes a changed Secret<br />again before the interval, so it validates rotated credentials promptly. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current validation state. The Ready condition<br />carries reasons Healthy, MissingSecret, or ConnectionFailed. |  | Optional: \{\} <br /> |


#### DatabaseServerMonitoringSpec



DatabaseServerMonitoringSpec groups the Prometheus scraping integration of
a database server.



_Appears in:_
- [DatabaseServerSpec](#databaseserverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `podMonitor` _[PodMonitorSpec](#podmonitorspec)_ | PodMonitor configures the Prometheus PodMonitor over the instance pods. |  | Optional: \{\} <br /> |


#### DatabaseServerPreset



DatabaseServerPreset is a cluster-scoped, passive baseline configuration
for DatabaseServer resources: no controller reconciles it, it provisions
nothing and reports no status. Consumers resolve it via their presetRef and
overlay inline fields wholesale.





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
| `server` _[DatabaseServerSpec](#databaseserverspec)_ | Server is the full configuration baseline consumers inherit. It reuses<br />the DatabaseServer spec type so the two never drift apart. The<br />instance-bound fields of that type, presetRef, releaseRef,<br />databaseServerConfig, and suspend, must be left unset inside a preset,<br />and so must the version, which belongs to a CamundaRelease. Explicit<br />zero values (an empty presetRef, suspend: false), as templated YAML<br />renders unset fields, count as unset. archive is a baseline like any<br />other field: one bucket serves a fleet, because every server writes<br />under a prefix of its own. |  | Required: \{\} <br /> |


#### DatabaseServerRecoveryStatus



DatabaseServerRecoveryStatus is the recovery request that the server works
on now, or the last one it answered. It is what makes a recovery resumable:
the steps read it to tell which cluster they build and whether the request
still needs an answer.

It holds the whole answer, not a reference to it. The answer is published on
a contract that somebody can delete and create again, and the server has to
be able to publish it a second time from what it remembers.



_Appears in:_
- [DatabaseServerStatus](#databaseserverstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requestID` _string_ | RequestID is the requestID of the request, as the contract carries it. |  |  |
| `contract` _string_ | Contract is the DatabaseServerConfig that carried the request. The<br />server answers on that contract, and it does not change the contract it<br />publishes while the request is unanswered. |  |  |
| `requestedBy` _string_ | RequestedBy is the requestedBy of the request, as the contract carries<br />it. |  |  |
| `targetTime` _string_ | TargetTime is the targetTime of the request, as the contract carries it. |  |  |
| `cluster` _string_ | Cluster is the CloudNativePG cluster that the recovery builds. It is<br />empty for a request that the server refused, whether or not it had built<br />one by then. |  | Optional: \{\} <br /> |
| `previousCluster` _string_ | PreviousCluster is the cluster that the contract pointed at before the<br />recovery moved it. A recovery that fails after the move puts the<br />contract back on it, because that cluster still holds the data. |  | Optional: \{\} <br /> |
| `archive` _[RecoveryArchiveRef](#recoveryarchiveref)_ | Archive is the archive that the recovery reads. It is recorded before<br />the recovery builds anything and read on every look after that, so a<br />spec that names another bucket in the middle does not move a recovery<br />that is already running. |  | Optional: \{\} <br /> |
| `result` _[RecoveryResult](#recoveryresult)_ | Result is the result the server published for the request. It is unset<br />while the recovery runs. |  | Enum: [Completed Failed Unavailable] <br />Optional: \{\} <br /> |
| `message` _string_ | Message is the message the server published with Result. |  | Optional: \{\} <br /> |
| `completedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | CompletedAt is when the server answered the request. It is unset while<br />the recovery runs. |  | Optional: \{\} <br /> |


#### DatabaseServerServiceAccountSpec



DatabaseServerServiceAccountSpec configures the ServiceAccount that
CloudNativePG creates for the instance pods. CloudNativePG owns that
account and names it after the server, so only its metadata is
configurable. The operator adds the workload-identity annotations of the
archive bucket on its own; an annotation set here wins over the derived one
on the same key.



_Appears in:_
- [DatabaseServerSpec](#databaseserverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `annotations` _object (keys:string, values:string)_ | Annotations to set on the ServiceAccount, typically workload-identity<br />annotations (IRSA, GCP Workload Identity, and more) that grant the<br />instance pods access to cloud resources. |  | Optional: \{\} <br /> |


#### DatabaseServerSpec



DatabaseServerSpec defines the desired state of DatabaseServer.

The type doubles as the configuration baseline of a DatabaseServerPreset,
so the field that is required on a DatabaseServer, databaseServerConfig, is
optional at the schema level here and enforced on the DatabaseServer usage
instead. The instance-bound fields are server-only and rejected in a preset,
and so is the version, which belongs to a CamundaRelease.



_Appears in:_
- [DatabaseServer](#databaseserver)
- [DatabaseServerPresetSpec](#databaseserverpresetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `presetRef` _string_ | PresetRef names a cluster-scoped DatabaseServerPreset used as the<br />configuration baseline; fields set inline override the preset's value<br />for that field wholesale. |  | Optional: \{\} <br /> |
| `releaseRef` _string_ | ReleaseRef names a cluster-scoped CamundaRelease that provides the<br />PostgreSQL major version. It merges over the preset and under this spec.<br />Forbidden in a preset. |  | Optional: \{\} <br /> |
| `platformConfigRef` _string_ | PlatformConfigRef names a cluster-scoped CamundaPlatformConfig. Only<br />its image settings are read: spec.images.postgres decides where the<br />PostgreSQL image is pulled from, which is what an air-gapped cluster<br />needs. Empty leaves the image at its default repository. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br />Optional: \{\} <br /> |
| `version` _string_ | Version is the PostgreSQL major version to run, as a bare number such<br />as "17". It selects the image tag. Camunda 8.9 supports PostgreSQL 14<br />and later. The controller enforces that floor on the merged result.<br />Required unless the resolved release provides it, and forbidden in a<br />preset.<br />The major of a running server cannot change. A value that names another<br />major, higher or lower, is refused on the Ready condition with reason<br />VersionChangeRefused, and the server keeps running the major it has. A<br />release can raise it the same way and is refused the same way. To run<br />another major, create a server on it and move the data over. |  | Pattern: `^\d+$` <br />Optional: \{\} <br /> |
| `instances` _integer_ | Instances is the number of PostgreSQL instances. One instance has no<br />failover: a node that goes away takes the server with it until the<br />volume is reattached. Defaults to 1. |  | Minimum: 1 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of each instance. |  | Optional: \{\} <br /> |
| `storageSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#quantity-resource-api)_ | StorageSize is the size of the data volume of each instance. It cannot<br />shrink, because PostgreSQL data volumes cannot be reduced in place.<br />Admission rejects a lower inline value on a DatabaseServer through a<br />CEL transition rule. That rule does not bind this shared field, so a<br />preset baseline can be lowered: a server that already applied a larger<br />size keeps it and records a StorageShrinkIgnored event. Required unless<br />the resolved preset provides it. |  | Optional: \{\} <br /> |
| `storageClassName` _string_ | StorageClassName is the StorageClass of the data volumes. Defaults to<br />the cluster's default StorageClass. |  | Optional: \{\} <br /> |
| `walStorageSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#quantity-resource-api)_ | WALStorageSize puts the write-ahead log on a volume of its own, of this<br />size. Unset keeps the log on the data volume. It cannot shrink, for the<br />same reason storageSize cannot, and a lowered preset baseline is<br />ignored the same way.<br />It can be added to a running server, and it cannot be taken away again:<br />CloudNativePG refuses a cluster that gives up the volume. A cleared<br />field, inline or from a preset, keeps the volume at the size it has and<br />records a WALStorageKept event. |  | Optional: \{\} <br /> |
| `serviceAccount` _[DatabaseServerServiceAccountSpec](#databaseserverserviceaccountspec)_ | ServiceAccount configures the ServiceAccount of the instance pods. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling constraints for the instance pods; when set, it replaces the<br />preset's scheduling block entirely (no merge). |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels applied to the instance pods. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations applied to the instance pods. |  | Optional: \{\} <br /> |
| `monitoring` _[DatabaseServerMonitoringSpec](#databaseservermonitoringspec)_ | Monitoring configures the Prometheus scraping integration. |  | Optional: \{\} <br /> |
| `databaseServerConfig` _string_ | DatabaseServerConfig names the DatabaseServerConfig the operator<br />publishes in this CR's own namespace with the endpoint, the admin<br />credentials, and the point-in-time-recovery capability of the server.<br />Required on a DatabaseServer, forbidden in a preset. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br />Optional: \{\} <br /> |
| `archive` _[DatabaseServerArchiveSpec](#databaseserverarchivespec)_ | Archive points the server at the bucket that holds its continuous<br />archive. Without it the server takes no part in point-in-time restore. |  | Optional: \{\} <br /> |
| `suspend` _boolean_ | Suspend stops the PostgreSQL instances and keeps their data volumes.<br />The operator hibernates the CloudNativePG cluster: the instance pods<br />are removed, the volumes stay, and setting the field back to false<br />brings the instances back on the same volumes. Defaults to false. |  | Optional: \{\} <br /> |


#### DatabaseServerStatus



DatabaseServerStatus is the observed state of a DatabaseServer.



_Appears in:_
- [DatabaseServer](#databaseserver)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `version` _string_ | Version is the PostgreSQL major version that the server runs, as a bare<br />number such as "17". It is the version of the merged spec, so it names<br />what runs whether the release, the preset, or the server supplies it,<br />and it stays on the major of the data directory while a version change<br />is refused. It is empty until the first reconcile resolves the<br />references of the server. |  | Optional: \{\} <br /> |
| `cluster` _string_ | Cluster is the CloudNativePG cluster that the published contract points<br />at. It is the name of the server until a recovery replaces it. |  | Optional: \{\} <br /> |
| `systemIdentifier` _string_ | SystemIdentifier is the identity of the PostgreSQL instance that runs<br />behind the contract, as CloudNativePG reports it. A recovery restores<br />the pg_control of the base backup it reads, so the recovered instance<br />reports the identity it recovered from and this value stays. The<br />endpoint of the contract is what a recovery replaces. |  | Optional: \{\} <br /> |
| `archive` _[DatabaseServerArchiveStatus](#databaseserverarchivestatus)_ | Archive is the observed state of the continuous archive of the server.<br />It is unset until the server has written one. Removing spec.archive<br />does not clear it: the bucket still holds what the server wrote, and a<br />server that archives again can recover from it. |  | Optional: \{\} <br /> |
| `recovery` _[DatabaseServerRecoveryStatus](#databaseserverrecoverystatus)_ | Recovery is the recovery request that the server works on now, or the<br />last one it answered. The answer itself is published on the contract,<br />in spec.pitr.lastRecovery. |  | Optional: \{\} <br /> |
| `volumes` _[VolumeStatus](#volumestatus) array_ | Volumes lists the bound PersistentVolumeClaims of the current cluster<br />and the capacity that each one reports, sorted by name. A server with a<br />write-ahead log volume reports that claim here too. A server that does<br />not own the cluster of the name it derives reports none: those claims<br />belong to the cluster that holds the name. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. Ready carries a pre-check<br />reason (InvalidReference, MissingSecret, CNPGNotInstalled,<br />BarmanPluginNotInstalled), or it is derived from the cluster, the<br />contract, and the archive of a server that asks for one. The<br />per-component conditions (ClusterReady, ArchiveReady, ContractReady,<br />MonitoringReady) also appear here. MonitoringReady, and ArchiveReady<br />without spec.archive, are reported on their own and never on Ready. A<br />PodMonitor observes the server rather than runs it, so a broken one<br />never makes the server not ready. |  | Optional: \{\} <br /> |


#### DatabaseSpec



DatabaseSpec defines the desired state of Database.



_Appears in:_
- [Database](#database)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serverRef` _string_ | ServerRef names the DatabaseServerConfig of this namespace describing<br />the server to create the database in. |  | MinLength: 1 <br /> |
| `databaseName` _string_ | DatabaseName is the name of the logical database to create, a valid<br />PostgreSQL identifier. It must be unique per server, and the server is<br />the PostgreSQL instance that the contract reaches, not the contract:<br />the controller rejects a Database whose databaseName collides with a<br />Database of any namespace on the same instance. |  | Pattern: `^[a-z_][a-z0-9_]\{0,62\}$` <br /> |
| `applicationCredentials` _[CredentialsSpec](#credentialsspec)_ | ApplicationCredentials configures the application credentials Secret,<br />always created. The Secret name defaults to <CR name>-credentials. |  | Optional: \{\} <br /> |
| `backupCredentials` _[BackupCredentialsSpec](#backupcredentialsspec)_ | BackupCredentials configures the backup credentials Secret, created<br />unless disabled. The Secret name defaults to<br /><CR name>-backup-credentials. |  | Optional: \{\} <br /> |
| `databaseConfig` _string_ | DatabaseConfig names the DatabaseConfig the operator creates in the<br />namespace of this Database. Defaults to the CR name. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br />Optional: \{\} <br /> |
| `secondaryStorageConfig` _string_ | SecondaryStorageConfig, when set, makes the operator also create a<br />SecondaryStorageConfig of type rdbms with this name in the namespace of<br />this Database, wired to the DatabaseConfig. Omit it for databases not<br />used as Camunda secondary storage (Keycloak, Identity, Web Modeler). |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br />Optional: \{\} <br /> |


#### DatabaseStatus



DatabaseStatus is the observed state of a Database.



_Appears in:_
- [Database](#database)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `collisionKey` _string_ | CollisionKey is the logical database that this Database last resolved:<br />the system identifier of the server and the database name. Every<br />claimant records it, the one that loses included, so the field says<br />what a Database asked for and not what it owns. The operator resolves<br />it only after it reaches the server, so a spec that names a server<br />which is missing, or one that is not probed for the spec it has now,<br />keeps the key from before until that server answers. A Database whose<br />Ready condition reports InvalidReference and names another Database<br />does not own the name it shows here. The operator never clears the<br />field, so an owner whose server or contract is gone keeps the logical<br />database. Delete that Database to release the name. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. Ready carries a pre-check<br />reason (InvalidReference, MissingSecret, ServerIdentityUnknown,<br />ConnectionFailed), or it takes the status and the reason of the<br />BindingsReady component condition, which also appears here. |  | Optional: \{\} <br /> |


#### DumpPodSpec



DumpPodSpec shapes the pod of the Job that dumps the logical database of a
relational cluster and uploads it to the backup bucket. It holds everything
that a backup can set per run. The pod runs the dump and the upload in
turn, so one resource block sizes both. It never names the image. See
BackupDumpSpec for the reason.



_Appears in:_
- [BackupDumpSpec](#backupdumpspec)
- [LogicalBackupRDBMSSpec](#logicalbackuprdbmsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the dump pod. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the dump pod. In the<br />spec.dump of a LogicalBackupRDBMS, every name under PG or UPLOAD_ is<br />reserved, and admission rejects it. Any PG* name is connection policy,<br />because libpq reads PGHOSTADDR, PGSERVICE, PGOPTIONS, and more.<br />UPLOAD_* is the upload contract. The variables of a backup reach the<br />dump container only, never the container that uploads. Cloud SDKs<br />read endpoint, proxy, and configuration variables from the<br />environment, and a backup author must not steer where the dump goes.<br />In the spec.backup.dump of the cluster nothing is reserved, and the<br />variables reach every container. The cluster owner sets connection<br />policy, PGSSLMODE included, inside their own boundary.<br />This list stays atomic, unlike the extraEnv of a workload. One field<br />manager owns the whole list, because nothing else writes it: no<br />extension attaches to a dump pod. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources of the dump pod, at most<br />8. The cap applies to every block that uses this type, the cluster<br />and preset blocks included. It keeps the admission rule of a<br />LogicalBackupRDBMS inside the cost budget of the API server. In the<br />spec.dump of a LogicalBackupRDBMS, every source also needs a prefix<br />that cannot spell a PG* or UPLOAD_* name. The reason is that the<br />writer of the referenced object chooses its keys. Like ExtraEnv, the<br />sources of a backup reach the dump container only. The block of the<br />cluster has no prefix requirement, and its sources reach every<br />container. |  | MaxItems: 8 <br />Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the dump pod. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the dump pod. Set the<br />injection annotation of a service mesh to false here: a sidecar that<br />keeps running after the dump finishes stops the Job from completing. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling constraints of the dump pod. When set, it replaces the<br />block of a preset entirely (no merge). |  | Optional: \{\} <br /> |
| `scratchVolume` _[ScratchVolumeSpec](#scratchvolumespec)_ | ScratchVolume is where the dump is written before it is uploaded. When<br />set, it replaces the block of a preset entirely (no merge). The dump<br />pod runs with fsGroup 999, the postgres group. So pg_dump can write to<br />a volume that a storage class hands over root-owned. |  | Optional: \{\} <br /> |
| `activeDeadlineSeconds` _integer_ | ActiveDeadlineSeconds is the number of seconds that the dump Job can<br />run before it fails, counted from its start. When it is unset, the<br />operator applies 86400 (24 hours) when it renders the Job. The schema<br />does not apply the default, so an unset value inherits the value of a<br />preset. The default is large so that a very large dump completes. It<br />is not unbounded because a pod that cannot start uses no retry.<br />Without a deadline, a broken Job stays active as long as the backup<br />lives. A lower value fails a stuck dump sooner. A higher value gives a<br />long dump the time it needs. |  | Minimum: 1 <br />Optional: \{\} <br /> |


#### ElasticsearchCluster



ElasticsearchCluster provisions and operates an Elasticsearch cluster for
use as secondary storage, deployed through the external ECK operator, and
publishes the connection details as a SecondaryStorageConfig with generated
credentials.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `ElasticsearchCluster` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[ElasticsearchClusterSpec](#elasticsearchclusterspec)_ | spec defines the desired state of ElasticsearchCluster |  | Required: \{\} <br /> |
| `status` _[ElasticsearchClusterStatus](#elasticsearchclusterstatus)_ | status defines the observed state of ElasticsearchCluster |  | Optional: \{\} <br /> |


#### ElasticsearchClusterPreset



ElasticsearchClusterPreset is a cluster-scoped, passive baseline
configuration for ElasticsearchCluster resources: no controller reconciles
it, it provisions nothing and reports no status. Consumers resolve it via
their presetRef and overlay inline fields wholesale.





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
| `cluster` _[ElasticsearchClusterSpec](#elasticsearchclusterspec)_ | Cluster is the full configuration baseline consumers inherit. It reuses<br />the ElasticsearchCluster spec type so the two never drift apart. The<br />instance-bound fields of that type, presetRef, releaseRef,<br />secondaryStorageConfig, and suspend, must be left unset inside a preset,<br />and so must the version, which belongs to a CamundaRelease. Explicit<br />zero values (an empty presetRef, suspend: false), as templated YAML<br />renders unset fields, count as unset. monitoring is a baseline like any<br />other field: a preset can enable scraping and pin the exporter image and<br />resources for every cluster that references it. |  | Required: \{\} <br /> |


#### ElasticsearchClusterSpec



ElasticsearchClusterSpec defines the desired state of ElasticsearchCluster.

The type doubles as the configuration baseline of an
ElasticsearchClusterPreset, so the field that is required on an
ElasticsearchCluster, secondaryStorageConfig, is optional at the schema
level here and enforced on the ElasticsearchCluster usage instead. The
instance-bound fields are cluster-only and rejected in a preset, and so is
the version, which belongs to a CamundaRelease.



_Appears in:_
- [ElasticsearchCluster](#elasticsearchcluster)
- [ElasticsearchClusterPresetSpec](#elasticsearchclusterpresetspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `presetRef` _string_ | PresetRef names a cluster-scoped ElasticsearchClusterPreset used as the<br />configuration baseline; fields set inline override the preset's value<br />for that field wholesale. |  | Optional: \{\} <br /> |
| `releaseRef` _string_ | ReleaseRef names a cluster-scoped CamundaRelease that provides the<br />Elasticsearch version. It merges over the preset and under this spec.<br />Forbidden in a preset. |  | Optional: \{\} <br /> |
| `version` _string_ | Version is the Elasticsearch version to deploy, as a full semantic<br />version. Camunda 8.9 supports Elasticsearch 8.19+ and 9.2+. The<br />controller enforces that floor on the merged result, and the schema<br />pins only the three-segment shape. Required unless the resolved release<br />provides it, and forbidden in a preset. |  | Pattern: `^\d+\.\d+\.\d+$` <br />Optional: \{\} <br /> |
| `replicas` _integer_ | Replicas is the number of Elasticsearch nodes. Required unless the<br />resolved preset provides it. |  | Minimum: 1 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory for each Elasticsearch node. |  | Optional: \{\} <br /> |
| `storageSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#quantity-resource-api)_ | StorageSize is the size of the data volume of each node. It cannot<br />shrink, because Elasticsearch data volumes cannot be reduced in place.<br />Admission rejects a lower inline value on an ElasticsearchCluster<br />through a CEL transition rule. That rule does not bind this shared<br />field, so a preset baseline can be resized freely: a cluster that<br />applied a larger size keeps it and records a StorageShrinkIgnored<br />event. Required unless the resolved preset provides it. |  | Optional: \{\} <br /> |
| `storageClassName` _string_ | StorageClassName is the StorageClass for the data volumes. Defaults to<br />the cluster's default StorageClass. |  | Optional: \{\} <br /> |
| `serviceAccount` _[ServiceAccountSpec](#serviceaccountspec)_ | ServiceAccount configures the Elasticsearch pods' ServiceAccount,<br />applied through the ECK podTemplate. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables for every Elasticsearch node. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) for<br />every Elasticsearch node. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels applied to the Elasticsearch pods. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations applied to the Elasticsearch pods. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling constraints for the Elasticsearch pods; when set, it replaces<br />the preset's scheduling block entirely (no merge). |  | Optional: \{\} <br /> |
| `snapshotStorageRef` _string_ | SnapshotStorageRef names an ObjectStorageConfig, in the namespace of<br />this cluster, that holds the bucket of the snapshot repository of this<br />cluster. When it is set,<br />the operator owns the whole Elasticsearch side of that bucket: it gives<br />the nodes their credentials, registers the repository, and publishes the<br />repository name in the SecondaryStorageConfig it produces. Backups of a<br />CamundaCluster on this storage need it. The bucket must be the one that<br />the CamundaCluster references as well. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br />Optional: \{\} <br /> |
| `secureSettings` _[SecureSettingsSource](#securesettingssource) array_ | SecureSettings are Secrets that ECK loads into the keystore of every<br />node. The operator adds the credentials of snapshotStorageRef to the<br />keystore on its own, so this field is for everything else a keystore<br />holds. |  | Optional: \{\} <br /> |
| `secondaryStorageConfig` _string_ | SecondaryStorageConfig names the SecondaryStorageConfig the operator<br />creates in this CR's own namespace with the connection details and<br />generated credentials. Required on an ElasticsearchCluster, forbidden in<br />a preset. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br />Optional: \{\} <br /> |
| `monitoring` _[MonitoringSpec](#monitoringspec)_ | Monitoring configures the Prometheus scraping integration. |  | Optional: \{\} <br /> |
| `persistentVolumeClaimRetentionPolicy` _[PersistentVolumeClaimRetentionPolicy](#persistentvolumeclaimretentionpolicy)_ | PersistentVolumeClaimRetentionPolicy says what happens to the data<br />volumes when the ElasticsearchCluster is deleted. Suspension always<br />keeps them. |  | Optional: \{\} <br /> |
| `suspend` _boolean_ | Suspend stops the Elasticsearch cluster and keeps its data volumes.<br />The operator sets the volume claim delete policy of the ECK<br />Elasticsearch resource to DeleteOnScaledownOnly, waits until ECK has<br />observed it, and deletes the resource. Setting the field back to<br />false recreates the resource, and ECK reattaches the volumes.<br />Defaults to false. |  | Optional: \{\} <br /> |


#### ElasticsearchClusterStatus



ElasticsearchClusterStatus is the observed state of an ElasticsearchCluster.



_Appears in:_
- [ElasticsearchCluster](#elasticsearchcluster)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `version` _string_ | Version is the Elasticsearch version that the cluster runs, as a full<br />semantic version. It is the version of the merged spec, so it names what<br />runs whether the release, the preset, or the cluster supplies it. It is<br />empty until the first reconcile resolves the references of the cluster. |  | Optional: \{\} <br /> |
| `volumes` _[VolumeStatus](#volumestatus) array_ | Volumes lists the bound data PersistentVolumeClaims of the cluster and<br />the capacity that each one reports, sorted by name. |  | Optional: \{\} <br /> |
| `snapshotRepository` _string_ | SnapshotRepository is the snapshot repository that the operator has<br />registered in Elasticsearch for this cluster. It is the name that the<br />published SecondaryStorageConfig carries, and it is empty until the<br />first registration converges. A cluster whose name a later operator<br />version derives differently therefore publishes nothing until the new<br />repository exists, rather than a name that Elasticsearch does not hold. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. Ready carries a pre-check<br />reason (InvalidReference, MissingSecret, ECKNotInstalled), or it is<br />derived from the component conditions. The per-component conditions<br />(CredentialsReady, ElasticsearchReady, StorageContractReady) also<br />appear here. |  | Optional: \{\} <br /> |


#### ElasticsearchStorage



ElasticsearchStorage holds Elasticsearch connection details.



_Appears in:_
- [SecondaryStorageConfigSpec](#secondarystorageconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `endpoint` _string_ | Endpoint is the HTTP(S) endpoint of the Elasticsearch cluster. |  |  |
| `credentialsSecretRef` _[LocalCredentialsSecretRef](#localcredentialssecretref)_ | CredentialsSecretRef names a basic-auth user with read/write access to<br />the Camunda indices. |  |  |
| `caSecretRef` _[LocalSecretKeyRef](#localsecretkeyref)_ | CASecretRef names the CA bundle consumers use to verify the endpoint's<br />TLS certificate. Set it when the endpoint serves a certificate not<br />signed by a well-known CA, such as the self-signed certificate of an<br />ECK-managed cluster. Omit it for publicly trusted endpoints; it is only<br />valid with an https endpoint. |  | Optional: \{\} <br /> |
| `snapshotRepository` _string_ | SnapshotRepository names the snapshot repository, registered in this<br />Elasticsearch cluster, that backups write to. An ElasticsearchCluster<br />with a snapshotStorageRef registers the repository and fills this field<br />in the contract it produces. Set it by hand for an Elasticsearch cluster<br />that this operator does not manage, after you register the repository<br />yourself. A cluster that takes backups needs it: without a repository<br />name, the backup components have nowhere to write. The name is a URL<br />path segment of the Elasticsearch API, so it is restricted to a<br />conservative character set. |  | MaxLength: 253 <br />Pattern: `^[a-zA-Z0-9][a-zA-Z0-9._-]*$` <br />Optional: \{\} <br /> |
| `nodeCount` _integer_ | NodeCount is the number of data nodes of the Elasticsearch cluster. An<br />ElasticsearchCluster fills it in the contract it produces. Set it by<br />hand for an Elasticsearch cluster that this operator does not manage.<br />A consumer that sets no index replica count of its own gets 0 replicas<br />on one node and 1 replica on two or more nodes. Without a node count,<br />the consumer keeps the default of the Camunda application. |  | Minimum: 1 <br />Optional: \{\} <br /> |


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
| `url` _string_ | URL is the URL of Keycloak, including the /auth path when it has one.<br />Management Identity reaches this URL, so it must resolve from inside<br />the Kubernetes cluster.<br />The operator appends /realms/<realm> to this URL, so the URL carries no<br />query and no fragment. The URL also lands in the annotations of the<br />Lease that claims the realm, so it is bounded.<br />The URL carries no user and no password. The operator does not support<br />a Keycloak behind a proxy that needs basic authentication. |  | MaxLength: 2048 <br /> |
| `realm` _string_ | Realm is the realm that Management Identity uses and creates. Empty<br />means camunda-platform. The realm lands in the issuer, the token, and<br />the JWKS path of every URL that Management Identity builds, so it holds<br />letters, digits, dots, hyphens, and underscores only, and it is bounded<br />the way the URL is. |  | MaxLength: 255 <br />Optional: \{\} <br /> |
| `adminCredentialsSecretRef` _[LocalCredentialsSecretRef](#localcredentialssecretref)_ | AdminCredentialsSecretRef names the Secret with the Keycloak<br />administrator credentials. Management Identity uses them to create the<br />realm, the clients, and the initial administrator. |  |  |
| `caBundleSecretRef` _[LocalSecretKeyRef](#localsecretkeyref)_ | CABundleSecretRef names the Secret key that holds the certificate<br />authority of Keycloak, in PEM form. The operator trusts it in addition<br />to the trust store of its own image when it signs in to Keycloak to<br />register the login callbacks of Optimize. Set it when Keycloak serves a<br />certificate that a public authority did not sign. It is only valid with<br />an https url. |  | Optional: \{\} <br /> |


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
| `bucketName` _string_ | BucketName is the bucket name as used by storage client SDKs. |  | MinLength: 1 <br /> |
| `basePath` _string_ | BasePath is the key prefix under which consumers write objects,<br />without leading or trailing slashes. Empty means the bucket root. |  | Pattern: `^[^/]+(/[^/]+)*$` <br />Optional: \{\} <br /> |
| `auth` _[GCSStorageAuth](#gcsstorageauth)_ | Auth selects how consumers authenticate. An absent block means<br />workload identity through the ServiceAccount chain. | \{ type:workloadIdentity \} | Optional: \{\} <br /> |


#### GCSStorageAuth



GCSStorageAuth selects how consumers authenticate against a GCS bucket.



_Appears in:_
- [GCSStorage](#gcsstorage)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ObjectStorageAuthType](#objectstorageauthtype)_ | Type is the authentication choice. Defaults to workloadIdentity. | workloadIdentity | Enum: [workloadIdentity credentials] <br />Optional: \{\} <br /> |
| `workloadIdentity` _[GCSWorkloadIdentity](#gcsworkloadidentity)_ | WorkloadIdentity names the trusted principal. Only valid with type<br />workloadIdentity; an empty or absent block means "trust the<br />ServiceAccount chain, add nothing". |  | Optional: \{\} <br /> |
| `credentials` _[GCSCredentials](#gcscredentials)_ | Credentials is a static service-account key. Required with type<br />credentials, forbidden otherwise. |  | Optional: \{\} <br /> |


#### GCSWorkloadIdentity



GCSWorkloadIdentity names the Google principal that the bucket trusts. An
empty block means the consumer's ServiceAccount chain already carries the
identity (Workload Identity Federation for GKE), so the operator adds
nothing.



_Appears in:_
- [GCSStorageAuth](#gcsstorageauth)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serviceAccountEmail` _string_ | ServiceAccountEmail is the Google service account that consumers<br />impersonate. When set, the operator puts it in the<br />iam.gke.io/gcp-service-account annotation of the consumer's<br />ServiceAccount. |  | Optional: \{\} <br /> |


#### GatewayBinding



GatewayBinding is the published in-cluster address of the client APIs of a
cluster. A client library takes the gRPC address as a host and a port, and
the REST address as a base URL, so the two fields carry the forms that the
consumers pass on unchanged.



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
| `mode` _[ComponentMode](#componentmode)_ | Mode selects a Deployment of the unified binary (Standalone) or the<br />embedded gateway of the brokers (Embedded). Defaults to Standalone.<br />The workload fields have an effect only when the mode is Standalone,<br />except extraEnv and extraEnvFrom, which apply to the brokers when the<br />mode is Embedded. |  | Enum: [Standalone Embedded] <br />Optional: \{\} <br /> |
| `replicas` _integer_ | Replicas is the number of pods of this process. Defaults to 1. On a<br />CamundaCluster it has no effect on an embedded web application. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the container of this process. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the container of this<br />process. On a CamundaCluster, an entry here wins over a top-level entry<br />with the same name. The entries of an embedded web application apply to<br />its host process.<br />The list merges by name under server-side apply, so each field manager<br />owns only the entries that it applies. An extension controller can add<br />its own entry next to yours. One applied manifest cannot hold two<br />entries with the same name.<br />Two field managers that apply the same name do not conflict: the merge<br />is per field inside the entry, so one manager can own value while the<br />other owns valueFrom. A container rejects an entry that carries both,<br />so the rule below refuses to store that combination. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the<br />container of this process. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the pods of this process. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the pods of this process. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling constraints of the pods of this process. When set, it<br />replaces every scheduling block that would otherwise apply to this<br />process, with no merge. On a CamundaCluster those are the top-level<br />block and the block of a preset. |  | Optional: \{\} <br /> |


#### IdentityAdminSpec



IdentityAdminSpec names the first administrator of the management plane.
Management Identity reads it on the first start only and stores the result
in its database.

In the oidc mode the administrator is a claim of the tokens that the
provider issues, so set claimName and claimValue; a later change of the
claim reports ImmutableAfterStart. In the two Keycloak modes the
administrator is the first Keycloak user, so set username; a later change
creates a second user and the first one keeps its access.



_Appears in:_
- [IdentitySpec](#identityspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `claimName` _string_ | ClaimName is the token claim that identifies the administrator, for<br />example oid or sub. Set it in the oidc mode. The operator records the<br />claim as <claimName>=<claimValue> and reads it back at the first equals<br />sign, so the name holds no equals sign. |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `claimValue` _string_ | ClaimValue is the value that the claim carries for the administrator.<br />Set it in the oidc mode. |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `username` _string_ | Username is the name of the first Keycloak user. Set it in the keycloak<br />and the externalKeycloak mode. Management Identity creates the user on<br />its first start, so a later change to this field creates a second user<br />rather than renaming the first one. |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `passwordSecretRef` _[LocalSecretKeyRef](#localsecretkeyref)_ | PasswordSecretRef names the Secret key that holds the password of the<br />first Keycloak user. The operator generates a password when this is<br />unset. |  | Optional: \{\} <br /> |
| `email` _string_ | Email is the email address of the first Keycloak user. Web Modeler<br />needs an address for every person who signs in, so it is required when<br />webModeler is set in a Keycloak mode. |  | MinLength: 3 <br />Optional: \{\} <br /> |


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
follow from spec.identityProvider, so an entry of extraEnv that replaces
either one is refused. In a Keycloak mode the operator renders them and this
management plane claims exactly the realm they name, so an override would
have Management Identity write the login callbacks of Optimize into a realm
that status.callbackRealm never names, that no withdrawal reaches, and that
another management plane can hold.



_Appears in:_
- [CamundaManagementClusterSpec](#camundamanagementclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version is the Management Identity version, as a full semantic version.<br />The operator supports 8.9.0 and later. |  | Pattern: `^\d+\.\d+\.\d+$` <br /> |
| `externalUrl` _string_ | ExternalURL is the URL that browsers reach Management Identity at.<br />Identity registers it as the redirect URI of its own client. |  |  |
| `databaseConfigRef` _string_ | DatabaseConfigRef names the DatabaseConfig of the Management Identity<br />database, in the namespace of this resource. Identity needs its own<br />PostgreSQL database. |  | MinLength: 1 <br /> |
| `admin` _[IdentityAdminSpec](#identityadminspec)_ | Admin names the first administrator of the management plane. |  |  |
| `replicas` _integer_ | Replicas is the number of pods of this process. Defaults to 1. On a<br />CamundaCluster it has no effect on an embedded web application. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the container of this process. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the container of this<br />process. On a CamundaCluster, an entry here wins over a top-level entry<br />with the same name. The entries of an embedded web application apply to<br />its host process.<br />The list merges by name under server-side apply, so each field manager<br />owns only the entries that it applies. An extension controller can add<br />its own entry next to yours. One applied manifest cannot hold two<br />entries with the same name.<br />Two field managers that apply the same name do not conflict: the merge<br />is per field inside the entry, so one manager can own value while the<br />other owns valueFrom. A container rejects an entry that carries both,<br />so the rule below refuses to store that combination. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the<br />container of this process. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the pods of this process. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the pods of this process. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling constraints of the pods of this process. When set, it<br />replaces every scheduling block that would otherwise apply to this<br />process, with no merge. On a CamundaCluster those are the top-level<br />block and the block of a preset. |  | Optional: \{\} <br /> |


#### ImagesSpec



ImagesSpec renames the container images that the operator pulls. Each field
holds a repository without a tag or a digest, for example
mirror.example.com/camunda/optimize. The version of the component supplies
the tag. A repository name is lowercase, as the container registries
require. An unset field means the default repository of that image.



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
it. Two targets name the same realm when their url and their realm are the
same, whichever administrator and certificate authority each of them
carries.



_Appears in:_
- [CamundaManagementClusterStatus](#camundamanagementclusterstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `url` _string_ | URL is the URL of Keycloak that the operator reaches, the /auth path<br />included when Keycloak serves one. |  |  |
| `realm` _string_ | Realm is the Keycloak realm. |  |  |
| `adminCredentialsSecretRef` _[LocalCredentialsSecretRef](#localcredentialssecretref)_ | AdminCredentialsSecretRef names the Secret with the Keycloak<br />administrator that the operator signs in with, in the namespace of this<br />resource. |  |  |
| `caBundleSecretRef` _[LocalSecretKeyRef](#localsecretkeyref)_ | CABundleSecretRef names the Secret key with the certificate authority of<br />Keycloak, in the namespace of this resource. It is absent for a Keycloak<br />whose certificate a public authority signed. |  | Optional: \{\} <br /> |


#### LocalCredentialsSecretRef



LocalCredentialsSecretRef references a username/password pair stored in a
Secret of the namespace of the object that holds the reference. Every
namespaced kind uses it, so a reference can never reach the credentials of
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
| `name` _string_ | Name of the Secret holding the credentials. |  | MinLength: 1 <br /> |
| `usernameKey` _string_ | UsernameKey is the key in the Secret holding the plaintext username. | username | MinLength: 1 <br />Optional: \{\} <br /> |
| `passwordKey` _string_ | PasswordKey is the key in the Secret holding the plaintext password. | password | MinLength: 1 <br />Optional: \{\} <br /> |


#### LocalSecretKeyRef



LocalSecretKeyRef references a single value inside a Secret of the namespace
of the object that holds the reference. Every namespaced kind uses it, so a
reference can never reach the Secrets of another namespace.



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
| `name` _string_ | Name of the Secret holding the value. |  | MinLength: 1 <br /> |
| `key` _string_ | Key in the Secret holding the value. |  | MinLength: 1 <br /> |


#### LogicalBackupElasticsearch



LogicalBackupElasticsearch is one backup of an Elasticsearch-backed
CamundaCluster. The backup is one coordinated set under one backup ID: the
web-application indices, the exported Zeebe record indices, and the Zeebe
partitions. It is taken hot, with exporting soft-paused. A restore reads a
completed backup by its backup ID and its recorded snapshot names. When you
delete the resource, a finalizer deletes the stored artifacts.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `LogicalBackupElasticsearch` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[LogicalBackupElasticsearchSpec](#logicalbackupelasticsearchspec)_ | spec identifies the cluster to back up. It is immutable: a backup is a<br />one-shot operation, retried by creating a new resource. |  | Required: \{\} <br /> |
| `status` _[LogicalBackupElasticsearchStatus](#logicalbackupelasticsearchstatus)_ | status defines the observed state of the backup |  | Optional: \{\} <br /> |


#### LogicalBackupElasticsearchSpec



LogicalBackupElasticsearchSpec identifies the cluster to back up. The whole
spec is immutable: a backup is a one-shot operation, retried by creating a
new resource.



_Appears in:_
- [LogicalBackupElasticsearch](#logicalbackupelasticsearch)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clusterRef` _[ClusterRef](#clusterref)_ | ClusterRef references the CamundaCluster to back up, in the namespace<br />of this backup. Its secondary storage must be Elasticsearch. |  | Required: \{\} <br /> |


#### LogicalBackupElasticsearchStatus



LogicalBackupElasticsearchStatus tracks the one-shot backup procedure to
completion.



_Appears in:_
- [LogicalBackupElasticsearch](#logicalbackupelasticsearch)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[LogicalBackupPhase](#logicalbackupphase)_ | Phase of the backup. Completed and Failed are terminal. |  | Enum: [Pending Running Completed Failed] <br />Optional: \{\} <br /> |
| `step` _[LogicalBackupElasticsearchStep](#logicalbackupelasticsearchstep)_ | Step is the resume marker of the running procedure. |  | Enum: [PauseExporting BackupHistory SnapshotRecords BackupRuntime ResumeExporting] <br />Optional: \{\} <br /> |
| `backupId` _integer_ | BackupID keys every part of the backup set: the web-application<br />snapshots, the record snapshot, and the partition backup. A restore<br />locates the set by it. |  | Optional: \{\} <br /> |
| `partitionsCount` _integer_ | PartitionsCount is the partition count of the cluster when the backup<br />started. A restore must match it. |  | Optional: \{\} <br /> |
| `storageSizes` _[LogicalBackupStorageSizes](#logicalbackupstoragesizes)_ | StorageSizes are the effective restore sizes, recorded best effort.<br />They are computed when the backup starts. A value that was not<br />available then is backfilled while exporting runs: before the pause,<br />and after the resume. No value is backfilled while exporting is<br />paused. A value that is absent can therefore still arrive later in<br />the run. |  | Optional: \{\} <br /> |
| `history` _[BackupPart](#backuppart)_ | History is the backup of the web-application indices. |  | Optional: \{\} <br /> |
| `records` _[BackupPart](#backuppart)_ | Records is the snapshot of the exported Zeebe record indices. |  | Optional: \{\} <br /> |
| `runtime` _[BackupPart](#backuppart)_ | Runtime is the backup of the Zeebe partitions. |  | Optional: \{\} <br /> |
| `historySnapshots` _string array_ | HistorySnapshots names the Elasticsearch snapshots of the<br />web-application indices. The names are recorded as soon as the<br />management API names them. The answer to the start names the scheduled<br />snapshots, and every status report names them again. The finalizer and<br />a restore can then locate the snapshots after the cluster is gone. |  | Optional: \{\} <br /> |
| `repository` _string_ | Repository pins the snapshot repository that every part of the set is<br />written to. It is recorded when the backup starts. Every later step and<br />the finalizer use the pinned name. A repository that changes on the<br />storage contract mid-run can then neither split the set nor aim the<br />deletion at the wrong repository. |  | Optional: \{\} <br /> |
| `storage` _[PinnedStorage](#pinnedstorage)_ | Storage pins the Elasticsearch destination of the set: the storage<br />contract and the endpoint it named when the backup started. The<br />repository name alone does not identify a cluster. A storage contract<br />or endpoint that changes mid-run fails the step, and the finalizer<br />never deletes against a different cluster. |  | Optional: \{\} <br /> |
| `clusterUID` _string_ | ClusterUID pins the identity of the CamundaCluster that the backup<br />started against. A cluster that is deleted and recreated under the<br />same name is a different cluster. Its exporting was never paused by<br />this backup, and its artifacts are not this backup's. Every<br />management call after the start verifies the live cluster against<br />this UID. A mismatch ends the backup without touching the<br />replacement. |  | Optional: \{\} <br /> |
| `version` _string_ | Version is the Camunda version of the cluster when the backup started,<br />as the management binding reported it. A restore compares it against<br />the version of its target: an Elasticsearch backup restores only with<br />the exact same version, and a relational backup restores with the same<br />Camunda minor or one minor newer. It is the only place a restore can read<br />the version, because the management binding of a suspended cluster is<br />unset. |  | Optional: \{\} <br /> |
| `historyRequestedTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | HistoryRequestedTime is when the controller decided to request the<br />backup of the web-application indices. It is written before the<br />request is sent, so the intent survives a lost response or a restart.<br />It proves that this backup meant to request, not that a history<br />backup under its ID is its own. |  | Optional: \{\} <br /> |
| `historyAcceptedTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | HistoryAcceptedTime is when the cluster accepted the history backup<br />request of this backup, as this controller observed it. It is the<br />only evidence that the history backup under this ID is this backup's.<br />A history backup that exists without it is not adopted: the step<br />fails, and the finalizer does not delete its snapshots. A crash<br />between the request and the write of this field fails the backup<br />safely. It can leave a history backup under this ID in the cluster<br />for the user to remove by hand. |  | Optional: \{\} <br /> |
| `runtimeRequestedTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | RuntimeRequestedTime is when the controller decided to request the<br />runtime backup. It is written before the request is sent, so the<br />intent survives a lost response or a restart. It proves that this<br />backup meant to request, not that a runtime backup under its ID is<br />its own. |  | Optional: \{\} <br /> |
| `runtimeAcceptedTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | RuntimeAcceptedTime is when the cluster accepted the request of this<br />backup, as this controller observed it. It is the only evidence that<br />the runtime backup under this ID is this backup's. A runtime backup<br />that exists without it, after a lost response or because another<br />actor won the ID, is not adopted. The step fails, and the finalizer<br />leaves that runtime backup alone. A crash between the request and<br />the write of this field fails the backup safely. It can leave such a<br />runtime backup in the cluster for the user to remove by hand. The<br />cluster registers the backup asynchronously and can report it absent<br />for a moment after the acceptance. Within a registration grace after<br />this time, an absent backup is polled. After the grace, an absent<br />backup fails the step. |  | Optional: \{\} <br /> |
| `unreachableSince` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | UnreachableSince is when a working step first found its endpoint<br />unreachable. The endpoint is the management API or Elasticsearch,<br />whichever the step calls. Exporting can be paused at every working<br />step, so the retry is bounded. After the bound the step fails and the<br />procedure resumes exporting. It clears once every call of a reconcile<br />answered. |  | Optional: \{\} <br /> |
| `failureMessage` _string_ | FailureMessage names the failing step and its error. It is recorded<br />when a step fails and exporting still has to be resumed. The reason<br />then survives the resume and reaches the terminal condition. |  | Optional: \{\} <br /> |
| `resumeStartedTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | ResumeStartedTime anchors the resume deadline. Only the accumulated<br />time of active resume attempts counts against the deadline. A gap in<br />which the procedure was parked slides the anchor forward and does not<br />count, for example a suspended cluster or an unpublished binding. The<br />anchor survives an operator restart. |  | Optional: \{\} <br /> |
| `lastResumeAttemptTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | LastResumeAttemptTime is when the last resume attempt ended. The gap<br />from it to the start of the next attempt decides whether the deadline<br />anchor slides. The time inside an attempt always counts. |  | Optional: \{\} <br /> |
| `terminalReason` _string_ | TerminalReason is the Ready reason recorded at the terminal<br />transition: Completed, Failed, or ResumeFailed. The controller<br />re-stages the terminal condition from it when a write conflict<br />restored an older one. |  | Optional: \{\} <br /> |
| `resumeFailureMessage` _string_ | ResumeFailureMessage is the last error of resume-exporting when the<br />procedure gave up on it. It stands beside FailureMessage, so a backup<br />that failed a step and then failed to resume reports both. |  | Optional: \{\} <br /> |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | CompletionTime is when the backup reached a terminal phase. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. The Ready condition tracks the<br />backup with the reasons Progressing, Completed, Failed, ResumeFailed,<br />ClusterSuspended, BackupInProgress, StorageTypeMismatch,<br />InvalidReference, MissingSecret, and ConnectionFailed. |  | Optional: \{\} <br /> |


#### LogicalBackupElasticsearchStep

_Underlying type:_ _string_

LogicalBackupElasticsearchStep is the resume marker of the backup
procedure. A crash or an operator restart re-enters at the recorded step.
Every step queries the current state before it acts, so no call is
repeated.

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

LogicalBackupPhase tracks a one-shot backup operation. Completed and Failed
are terminal; a retry is a new CR.

_Validation:_
- Enum: [Pending Running Completed Failed]

_Appears in:_
- [LogicalBackupElasticsearchStatus](#logicalbackupelasticsearchstatus)
- [LogicalBackupRDBMSStatus](#logicalbackuprdbmsstatus)

| Field | Description |
| --- | --- |
| `Pending` | LogicalBackupPending means the backup has not started real work: the<br />pre-checks have not all passed yet, or another backup of the same<br />cluster runs.<br /> |
| `Running` | LogicalBackupRunning means the backup procedure is in progress.<br /> |
| `Completed` | LogicalBackupCompleted means the backup finished and is restorable.<br /> |
| `Failed` | LogicalBackupFailed means the backup failed. The Ready condition<br />message names the failing step.<br /> |


#### LogicalBackupRDBMS



LogicalBackupRDBMS is one backup of a relational orchestration cluster. It
is a dump of the entire logical database, uploaded to the backup bucket and
paired with one Zeebe backup. Camunda calls the Zeebe log and snapshots its
"primary storage" and the exported relational data its "secondary storage".
A Zeebe backup is Camunda's own backup of that primary storage to the
backup bucket, requested through the management API. A restore reads the
exporter position from the restored dump and picks the Zeebe backups that
match it. So the pair is a complete restore point.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `LogicalBackupRDBMS` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[LogicalBackupRDBMSSpec](#logicalbackuprdbmsspec)_ | spec defines the desired state of LogicalBackupRDBMS |  | Required: \{\} <br /> |
| `status` _[LogicalBackupRDBMSStatus](#logicalbackuprdbmsstatus)_ | status defines the observed state of LogicalBackupRDBMS |  | Optional: \{\} <br /> |


#### LogicalBackupRDBMSSpec



LogicalBackupRDBMSSpec identifies the cluster to back up. It is immutable.
A backup is one operation, and a retry is a new CR.



_Appears in:_
- [LogicalBackupRDBMS](#logicalbackuprdbms)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clusterRef` _[ClusterRef](#clusterref)_ | ClusterRef references the CamundaCluster to back up. The cluster must<br />store its data in a relational database and have a backupStorageRef. |  | Required: \{\} <br /> |
| `dump` _[DumpPodSpec](#dumppodspec)_ | Dump replaces the pod settings of the cluster's spec.backup.dump block<br />as a whole for this backup. Unset means the settings of the cluster.<br />The two never merge. The image that runs the dump is not among them.<br />The Job runs under the cluster's ServiceAccount, so the executable is<br />the choice of the cluster owner and always comes from the cluster<br />block. The environment is bounded the same way. The extraEnv of a<br />backup cannot name anything under PG or UPLOAD_. Every extraEnvFrom<br />source needs a prefix that cannot spell such a name. libpq prefers<br />PGHOSTADDR over the PGHOST of the Job, so an unbounded source can<br />redirect the dump with the injected credentials. The environment of<br />this block reaches the dump container only, never the container that<br />uploads: cloud SDKs read endpoint, proxy, and configuration variables<br />from the environment, and a backup must not steer where its dump<br />goes. The cluster's own block has no prefix requirement, and its<br />environment reaches every container. Its owner sets policy inside<br />their own boundary. |  | Optional: \{\} <br /> |


#### LogicalBackupRDBMSStatus



LogicalBackupRDBMSStatus is the observed state of one backup operation.



_Appears in:_
- [LogicalBackupRDBMS](#logicalbackuprdbms)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[LogicalBackupPhase](#logicalbackupphase)_ | Phase tracks the one-shot operation. Completed and Failed are<br />terminal. |  | Enum: [Pending Running Completed Failed] <br />Optional: \{\} <br /> |
| `step` _[LogicalBackupRDBMSStep](#logicalbackuprdbmsstep)_ | Step is where the procedure stands and where a crashed reconcile<br />resumes. |  | Enum: [Dumping ZeebeBackup] <br />Optional: \{\} <br /> |
| `backupId` _integer_ | BackupID identifies the dump object in the bucket. The operator<br />allocates it once, when the backup leaves Pending. |  | Optional: \{\} <br /> |
| `jobName` _string_ | JobName is the Job that dumps and uploads the database, while it<br />exists. It clears when the dump is recorded and the Job is released. A<br />failed Job stays until the backup is deleted, and its name stays with<br />it. |  | Optional: \{\} <br /> |
| `objectKey` _string_ | ObjectKey is the full key of the dump in the backup bucket:<br /><basePath>/<namespace>/<cluster>/<backupId>/<uid>/camunda.dump, where<br />uid is the UID of this resource. Because of the UID, a reused backup<br />id can never name the dump of another backup. |  | Optional: \{\} <br /> |
| `zeebeBackupId` _integer_ | ZeebeBackupID is the id of the Zeebe backup that the cluster generated<br />after the dump. It is unset until the operator requests that backup. |  | Optional: \{\} <br /> |
| `zeebeBackupRequestedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | ZeebeBackupRequestedAt is when the operator requested the Zeebe<br />backup. It bounds how long the poll tolerates a backup that the<br />cluster does not report yet. |  | Optional: \{\} <br /> |
| `workloadConfigHash` _string_ | WorkloadConfigHash pins the configuration that Zeebe ran when the<br />backup started. It is the config hash of the live Zeebe pod<br />template. The operator requests the Zeebe backup only while the hash<br />is unchanged. If a database is swapped in between, the dump pairs with<br />a Zeebe backup of another configuration. The generation of the cluster<br />alone cannot tell, because mutable referents enter the hash without a<br />bump of the generation. |  | Optional: \{\} <br /> |
| `clusterUID` _[UID](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#uid-types-pkg)_ | ClusterUID pins the CamundaCluster that admitted the backup. The<br />running steps resolve the cluster by name, and a cluster that was<br />deleted and created again under the same name is another cluster with<br />other primary storage, whatever its config hash says. A backup whose<br />cluster changed UID fails, so a dump never pairs with the Zeebe backup<br />of a replacement. |  | Optional: \{\} <br /> |
| `version` _string_ | Version is the Camunda version of the cluster when the backup started,<br />as the management binding reported it. A restore compares it against<br />the version of its target: an Elasticsearch backup restores only with<br />the exact same version, and a relational backup restores with the same<br />Camunda minor or one minor newer. It is the only place a restore can read<br />the version, because the management binding of a suspended cluster is<br />unset. |  | Optional: \{\} <br /> |
| `firstFailedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | FirstFailedAt is when a dependency of the running backup first stopped<br />resolving, or the management API first stopped answering. The operator<br />measures the mid-run grace from it. It clears when the backup recovers. |  | Optional: \{\} <br /> |
| `bucketRef` _string_ | BucketRef pins the ObjectStorageConfig through which the Job wrote the<br />dump. Deletion then cleans up against the bucket that holds the<br />object, even after the backupStorageRef of the cluster moved elsewhere. |  | Optional: \{\} <br /> |
| `bucketLocation` _string_ | BucketLocation pins where the Job wrote the object. It holds the<br />storage type, bucket, base path, and endpoint of the ObjectStorageConfig<br />at the start, as ObjectStorageConfig.Location renders them. Deletion runs<br />only while the contract still points there. A retargeted contract<br />leaves the object behind. It does not delete the object of a stranger<br />at the same key. |  | Optional: \{\} <br /> |
| `bucketGeneration` _integer_ | BucketGeneration is the generation of the pinned ObjectStorageConfig<br />when the backup started, for reference. BucketLocation decides whether<br />deletion can run. |  | Optional: \{\} <br /> |
| `storageSizes` _[LogicalBackupStorageSizes](#logicalbackupstoragesizes)_ | StorageSizes are the effective restore sizes recorded when the backup<br />started. The RDBMS kind records the Zeebe size only. |  | Optional: \{\} <br /> |
| `failureMessage` _string_ | FailureMessage is why the backup failed. It is set with a Failed phase.<br />The Ready condition carries the same message. The operator stages the<br />condition again from this field, so a write conflict can never lose it. |  | Optional: \{\} <br /> |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | CompletionTime is when the backup reached a terminal phase. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state. The Ready condition carries<br />the phase as its reason and names a failing step in its message. |  | Optional: \{\} <br /> |


#### LogicalBackupRDBMSStep

_Underlying type:_ _string_

LogicalBackupRDBMSStep is the resume marker of the backup procedure. A
reconcile that re-enters after a crash continues at the recorded step. It
does not repeat a step that already ran.

_Validation:_
- Enum: [Dumping ZeebeBackup]

_Appears in:_
- [LogicalBackupRDBMSStatus](#logicalbackuprdbmsstatus)

| Field | Description |
| --- | --- |
| `Dumping` | StepDumping runs the Job that writes the logical database to the<br />backup bucket.<br /> |
| `ZeebeBackup` | StepZeebeBackup requests one Zeebe backup right after the dump, so the<br />two pair into one restore point. A Zeebe backup is Camunda's own backup<br />of its primary storage, the Zeebe log and snapshots.<br /> |


#### LogicalBackupRef



LogicalBackupRef references a completed logical backup in the namespace of
the restore. The reference never crosses a namespace. The kind of the
restore says which backup kind it reads, so the reference carries a name
alone.



_Appears in:_
- [LogicalRestoreElasticsearchSpec](#logicalrestoreelasticsearchspec)
- [LogicalRestoreRDBMSSpec](#logicalrestorerdbmsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the backup, in the namespace of this restore. |  | MinLength: 1 <br />Required: \{\} <br /> |


#### LogicalBackupStorageSizes



LogicalBackupStorageSizes are the effective restore sizes of the
storage-bearing components, recorded when a backup starts so a restore can
create right-sized volumes instead of guessing. Recording is best effort: a
value that could not be computed stays unset. The RDBMS kind never sets
Elasticsearch, whose data it does not back up.



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
the Camunda indices of the target, restores every snapshot of the backup
into its Elasticsearch, gives the brokers empty data volumes, and runs the
Camunda restore application once per broker.

The restore prepares the target itself. It suspends the target, waits for
its brokers to stop, and sets spec.version to the Camunda version that the
backup was taken with. It withdraws the suspension when it completes, and
only when it applied that suspension itself. A failed restore leaves the
target suspended, and so does a restore that somebody deletes while it
runs: broker volumes that are empty or half written are worse under running
brokers than under none.

The restore keeps spec.version, and it owns the field under the manager
camunda-operator/restore-version. The target runs the version of the backup
until another manager takes that field over or removes it. A manifest that
leaves spec.version out takes nothing back, because server-side apply
removes a field only from the manager that declared it. Watch for that on a
target whose version came from a preset: the value the restore wrote wins
over the preset until somebody removes the field.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `LogicalRestoreElasticsearch` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[LogicalRestoreElasticsearchSpec](#logicalrestoreelasticsearchspec)_ | spec names the backup to restore and the cluster to restore into. It is<br />immutable: a restore is one-shot, retried by creating a new resource. |  | Required: \{\} <br /> |
| `status` _[LogicalRestoreElasticsearchStatus](#logicalrestoreelasticsearchstatus)_ | status defines the observed state of the restore |  | Optional: \{\} <br /> |


#### LogicalRestoreElasticsearchSpec



LogicalRestoreElasticsearchSpec names the backup to restore and the cluster
to restore into. The whole spec is immutable: a restore is one operation,
retried by creating a new resource.



_Appears in:_
- [LogicalRestoreElasticsearch](#logicalrestoreelasticsearch)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `backupRef` _[LogicalBackupRef](#logicalbackupref)_ | backupRef references the completed LogicalBackupElasticsearch to<br />restore from. It lives in the namespace of this restore. |  | Required: \{\} <br /> |
| `targetClusterRef` _[ClusterRef](#clusterref)_ | targetClusterRef references the CamundaCluster to restore into. It must<br />name the cluster the backup was taken from. The restore application<br />reads the primary-storage backup under the prefix of the cluster it<br />runs as. The cluster must stay suspended for the whole restore. |  | Required: \{\} <br /> |


#### LogicalRestoreElasticsearchStatus



LogicalRestoreElasticsearchStatus tracks the restore to a terminal phase.



_Appears in:_
- [LogicalRestoreElasticsearch](#logicalrestoreelasticsearch)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[LogicalRestorePhase](#logicalrestorephase)_ | phase of the restore. It is the resume marker: a reconcile that<br />re-enters after a crash continues at the recorded phase. |  | Enum: [Pending ValidatingCompatibility RestoringSecondaryStorage RestoringPrimaryStorage Completed Failed] <br />Optional: \{\} <br /> |
| `backupId` _integer_ | backupId is the backup that the restore reads, pinned when the restore<br />starts. The backup resource can be deleted afterwards without moving<br />the restore to another set of artifacts. |  | Optional: \{\} <br /> |
| `backend` _string_ | Backend is the Elasticsearch that the restore writes, pinned when the<br />restore starts, in the form of the storage claim key of the target<br />(the scheme, the host, and the port). From the end of admission to the<br />terminal phase, and after it while recoveryHeld is true, no other<br />CamundaCluster starts on this backend. The<br />restore holds while its target does not hold the backend. |  | Optional: \{\} <br /> |
| `contract` _string_ | contract is the SecondaryStorageConfig that held the endpoint of<br />Backend when the restore started. The hold stays on this contract when<br />its endpoint moves. |  | Optional: \{\} <br /> |
| `recoveryHeld` _boolean_ | recoveryHeld is true while a restore that failed, or that is being<br />deleted, keeps the backend because Elasticsearch can still recover<br />snapshots that the restore asked for. While it is true, no other<br />CamundaCluster starts on the backend, the target stays suspended, and no<br />other backup or restore of the target starts. A deleted restore stays<br />while it is true. It is unset on a restore that was never held for a<br />recovery, and false once the hold is over. |  | Optional: \{\} <br /> |
| `recoveryUnknownSince` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | recoveryUnknownSince is the time since which a held restore cannot read<br />the recovery from Elasticsearch. When the recovery stays unknown for ten<br />minutes, the restore gives the backend back. |  | Optional: \{\} <br /> |
| `repository` _string_ | repository is the Elasticsearch snapshot repository that the restore<br />reads from, on the Elasticsearch of the target. |  | Optional: \{\} <br /> |
| `restoredSnapshots` _string array_ | restoredSnapshots names every snapshot that the restore asked<br />Elasticsearch to restore. It is also the resume marker of the<br />secondary-storage phase: a look that finds it deletes no index again. |  | Optional: \{\} <br /> |
| `targetClusterUID` _[UID](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#uid-types-pkg)_ | TargetClusterUID pins the identity of the target cluster. A cluster<br />that is deleted and created again under the same name is another<br />cluster, and this restore is not its restore. |  | Optional: \{\} <br /> |
| `brokers` _integer_ | Brokers is the broker count read from the live broker StatefulSet and<br />recorded before the restore deletes a volume. It fixes how many volumes<br />are recreated and how many Jobs run. |  | Optional: \{\} <br /> |
| `primaryJobNames` _string array_ | PrimaryJobNames are the per-broker restore-application Jobs, in broker<br />order. The operator records them before it applies the Jobs, so the<br />record covers every Job that the next look finds. It is also the list<br />that a restore which completed removes, and the list whose logs explain<br />a restore that failed. |  | Optional: \{\} <br /> |
| `recreatedClaims` _string array_ | RecreatedClaims names the broker data claims that the restore deleted<br />and created again. A reconcile that re-enters does not delete a claim<br />twice. |  | Optional: \{\} <br /> |
| `firstFailedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | FirstFailedAt is when a dependency of the running restore first stopped<br />resolving. The operator measures the mid-run grace from it. It stays<br />set once the restore starts, because a dependency that flaps must not<br />reset the grace. |  | Optional: \{\} <br /> |
| `clusterSuspended` _boolean_ | ClusterSuspended records that this restore suspended its target<br />cluster. The restore withdraws that suspension when it reaches<br />Completed. A cluster that its owner suspended carries no such record,<br />so it stays suspended, and so does the cluster of a failed restore. |  | Optional: \{\} <br /> |
| `terminalReason` _string_ | TerminalReason is the Ready reason recorded at the terminal transition.<br />The operator stages the terminal condition again from this field, so a<br />write conflict cannot replace the reason with a weaker one. |  | Optional: \{\} <br /> |
| `failureMessage` _string_ | FailureMessage names the failing phase and its error. The Ready<br />condition carries the same message. |  | Optional: \{\} <br /> |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | CompletionTime is when the restore reached a terminal phase. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state of the restore. |  | Optional: \{\} <br /> |


#### LogicalRestorePhase

_Underlying type:_ _string_

LogicalRestorePhase tracks a one-shot logical restore. Completed and Failed
are terminal. A retry is a new resource. Both logical restore kinds use it,
because their phase values are the same.

_Validation:_
- Enum: [Pending ValidatingCompatibility RestoringSecondaryStorage RestoringPrimaryStorage Completed Failed]

_Appears in:_
- [LogicalRestoreElasticsearchStatus](#logicalrestoreelasticsearchstatus)
- [LogicalRestoreRDBMSStatus](#logicalrestorerdbmsstatus)

| Field | Description |
| --- | --- |
| `Pending` | LogicalRestorePending means that the restore did not start real work.<br />A pre-check still holds it: the target cluster runs, another operation<br />holds the cluster, or the backup is not completed.<br /> |
| `ValidatingCompatibility` | LogicalRestoreValidatingCompatibility means that the operator compares<br />the backup against the target: the storage type, the backup bucket, the<br />Camunda version, and, on the Elasticsearch kind, the partition count.<br /> |
| `RestoringSecondaryStorage` | LogicalRestoreRestoringSecondaryStorage means that the operator writes<br />the backup into the target's secondary storage.<br /> |
| `RestoringPrimaryStorage` | LogicalRestoreRestoringPrimaryStorage means that the operator recreated<br />the broker data volumes and runs the restore application on them.<br /> |
| `Completed` | LogicalRestoreCompleted means that the restore finished. The operator<br />removes the per-broker Jobs here, so their pods release the broker data<br />volumes, and it withdraws the suspension it applied, so the target runs<br />again unless its owner suspended it.<br /> |
| `Failed` | LogicalRestoreFailed means that the restore failed. The Ready condition<br />names the failing phase. The operator keeps the per-broker Jobs, because<br />their logs are the diagnosis, so a restore that reached<br />RestoringPrimaryStorage holds the broker data volumes until somebody<br />deletes it. A restore that failed in an earlier phase records no Job in<br />PrimaryJobNames and holds nothing.<br /> |


#### LogicalRestoreRDBMS



LogicalRestoreRDBMS restores one completed LogicalBackupRDBMS into one
suspended CamundaCluster. The operator writes the dump back into the
logical database of the target with pg_restore, gives the brokers empty
data volumes, and runs the Camunda restore application on them once per
broker.

The restore prepares the target itself. It suspends the target, waits for
its brokers to stop, and sets spec.version to the Camunda version that the
backup was taken with. It withdraws the suspension when it completes, and
only when it applied that suspension itself. A failed restore leaves the
target suspended, and so does a restore that somebody deletes while it
runs: broker volumes that are empty or half written are worse under running
brokers than under none.

The restore keeps spec.version, and it owns the field under the manager
camunda-operator/restore-version. The target runs the version of the backup
until another manager takes that field over or removes it. A manifest that
leaves spec.version out takes nothing back, because server-side apply
removes a field only from the manager that declared it. A target of a newer
minor is therefore left on the minor of the backup, and the owner upgrades
it forward again.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `LogicalRestoreRDBMS` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[LogicalRestoreRDBMSSpec](#logicalrestorerdbmsspec)_ | spec names the backup to read and the cluster to restore into. It is<br />immutable: a restore is one-shot, retried by creating a new resource. |  | Required: \{\} <br /> |
| `status` _[LogicalRestoreRDBMSStatus](#logicalrestorerdbmsstatus)_ | status defines the observed state of the restore |  | Optional: \{\} <br /> |


#### LogicalRestoreRDBMSSpec



LogicalRestoreRDBMSSpec names the backup to restore and the cluster to
restore into. The whole spec is immutable: a restore is one operation,
retried by creating a new resource.



_Appears in:_
- [LogicalRestoreRDBMS](#logicalrestorerdbms)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `backupRef` _[LogicalBackupRef](#logicalbackupref)_ | BackupRef references the completed LogicalBackupRDBMS to restore from,<br />in the namespace of this restore. |  | Required: \{\} <br /> |
| `targetClusterRef` _[ClusterRef](#clusterref)_ | TargetClusterRef references the CamundaCluster to restore into. It must<br />name the cluster the backup was taken from. The restore application<br />reads the primary-storage backup under the prefix of the cluster it<br />runs as. The cluster must be suspended for the whole restore. |  | Required: \{\} <br /> |


#### LogicalRestoreRDBMSStatus



LogicalRestoreRDBMSStatus tracks the restore to a terminal phase.



_Appears in:_
- [LogicalRestoreRDBMS](#logicalrestorerdbms)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[LogicalRestorePhase](#logicalrestorephase)_ | Phase of the restore. It is the resume marker: a reconcile that<br />re-enters after a crash continues at the recorded phase. |  | Enum: [Pending ValidatingCompatibility RestoringSecondaryStorage RestoringPrimaryStorage Completed Failed] <br />Optional: \{\} <br /> |
| `backupId` _integer_ | BackupID is the Zeebe backup id that the restore reads, pinned when the<br />restore starts. A backup that is deleted and created again under one<br />name carries another id, and this restore is not its restore. |  | Optional: \{\} <br /> |
| `backend` _string_ | Backend is the logical database that the restore writes, pinned when<br />the restore starts, in the form of the storage claim key of the target<br />(the host, the port, and the database name). From the end of admission<br />to the terminal phase, no other CamundaCluster starts on this backend.<br />The restore holds while its target does not hold the backend. |  | Optional: \{\} <br /> |
| `contract` _string_ | Contract is the DatabaseServerConfig and the database name that held the<br />address of Backend when the restore started. The hold stays on this<br />contract when the DatabaseServerConfig moves to another address. |  | Optional: \{\} <br /> |
| `secondaryJobName` _string_ | SecondaryJobName is the Job that runs pg_restore, while it exists. |  | Optional: \{\} <br /> |
| `targetClusterUID` _[UID](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#uid-types-pkg)_ | TargetClusterUID pins the identity of the target cluster. A cluster<br />that is deleted and created again under the same name is another<br />cluster, and this restore is not its restore. |  | Optional: \{\} <br /> |
| `brokers` _integer_ | Brokers is the broker count read from the live broker StatefulSet and<br />recorded before the restore deletes a volume. It fixes how many volumes<br />are recreated and how many Jobs run. |  | Optional: \{\} <br /> |
| `primaryJobNames` _string array_ | PrimaryJobNames are the per-broker restore-application Jobs, in broker<br />order. The operator records them before it applies the Jobs, so the<br />record covers every Job that the next look finds. It is also the list<br />that a restore which completed removes, and the list whose logs explain<br />a restore that failed. |  | Optional: \{\} <br /> |
| `recreatedClaims` _string array_ | RecreatedClaims names the broker data claims that the restore deleted<br />and created again. A reconcile that re-enters does not delete a claim<br />twice. |  | Optional: \{\} <br /> |
| `firstFailedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | FirstFailedAt is when a dependency of the running restore first stopped<br />resolving. The operator measures the mid-run grace from it. It stays<br />set once the restore starts, because a dependency that flaps must not<br />reset the grace. |  | Optional: \{\} <br /> |
| `clusterSuspended` _boolean_ | ClusterSuspended records that this restore suspended its target<br />cluster. The restore withdraws that suspension when it reaches<br />Completed. A cluster that its owner suspended carries no such record,<br />so it stays suspended, and so does the cluster of a failed restore. |  | Optional: \{\} <br /> |
| `terminalReason` _string_ | TerminalReason is the Ready reason recorded at the terminal transition.<br />The operator stages the terminal condition again from this field, so a<br />write conflict cannot replace the reason with a weaker one. |  | Optional: \{\} <br /> |
| `failureMessage` _string_ | FailureMessage names the failing phase and its error. The Ready<br />condition carries the same message. |  | Optional: \{\} <br /> |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | CompletionTime is when the restore reached a terminal phase. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state of the restore. |  | Optional: \{\} <br /> |


#### ManagedKeycloakSpec



ManagedKeycloakSpec configures the Keycloak that the operator runs through
the Keycloak Operator.



_Appears in:_
- [IdentityProviderSpec](#identityproviderspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version is the Keycloak version, as a full semantic version. Camunda<br />8.9 supports Keycloak 26 only. The image is<br />camunda/keycloak:quay-optimized-<version> unless the platform config<br />overrides the repository. |  | Pattern: `^\d+\.\d+\.\d+$` <br /> |
| `externalUrl` _string_ | ExternalURL is the URL that browsers reach Keycloak at, including the<br />/auth path. It is the front-channel issuer of every token. Management<br />Identity uses the front-channel URL since 8.5.3, so the Identity pods<br />must also reach it. Management Identity administers Keycloak through<br />the Service that the Keycloak Operator creates, not through this URL.<br />The operator appends /realms/<realm> to this URL, so the URL carries no<br />query and no fragment. |  |  |
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
| `method` _[ManagementAuthMethod](#managementauthmethod)_ | Method is the authentication method of the management port. Camunda<br />8.9 serves the actuator endpoints without authentication, so the<br />operator publishes none. The field carries basic for a user who puts<br />their own Spring Security configuration in front of the port, and for<br />a later Camunda version that secures it. |  | Enum: [none basic] <br /> |
| `credentialsSecretRef` _[LocalCredentialsSecretRef](#localcredentialssecretref)_ | CredentialsSecretRef names the username and password of the management<br />port. It is set only when Method is basic. |  | Optional: \{\} <br /> |


#### ManagementAuthConfig



ManagementAuthConfig is the contract CRD that carries the Management
Identity OIDC configuration — endpoints, client credentials, and audience —
for components that live outside the orchestration cluster.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `ManagementAuthConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[ManagementAuthConfigSpec](#managementauthconfigspec)_ | spec defines the desired state of ManagementAuthConfig |  | Required: \{\} <br /> |
| `status` _[ManagementAuthConfigStatus](#managementauthconfigstatus)_ | status defines the observed state of ManagementAuthConfig |  | Optional: \{\} <br /> |


#### ManagementAuthConfigSpec



ManagementAuthConfigSpec carries the Management Identity OIDC configuration:
endpoints, machine-to-machine client credentials, and audience.



_Appears in:_
- [ManagementAuthConfig](#managementauthconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `baseUrl` _string_ | BaseURL is the base URL of the Management Identity service. |  |  |
| `issuerUrl` _string_ | IssuerURL is the OIDC issuer URL used to validate tokens. |  |  |
| `issuerBackendUrl` _string_ | IssuerBackendURL is the issuer URL for in-cluster container-to-container<br />communication. Consumers default it to IssuerURL when empty. |  | Optional: \{\} <br /> |
| `authUrl` _string_ | AuthURL is the OIDC authorization endpoint used for browser login<br />redirects. |  |  |
| `tokenUrl` _string_ | TokenURL is the OIDC token endpoint used to acquire machine-to-machine<br />tokens. |  |  |
| `jwksUrl` _string_ | JwksURL is the JWKS endpoint used to fetch token signing keys. |  |  |
| `clientId` _string_ | ClientID is the ID of the client that Optimize signs in with. |  | MinLength: 1 <br /> |
| `audience` _string_ | Audience expected in access tokens issued for this client. |  | MinLength: 1 <br /> |
| `clientSecretRef` _[SecretKeyRef](#secretkeyref)_ | ClientSecretRef names the Secret key that holds the secret of that<br />client. |  |  |


#### ManagementAuthConfigStatus



ManagementAuthConfigStatus is the observed validation state of the contract.



_Appears in:_
- [ManagementAuthConfig](#managementauthconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current validation state; the Ready condition<br />carries reasons Healthy or MissingSecret. |  | Optional: \{\} <br /> |


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
cluster. Extensions that drive the cluster — the backup kinds first — read
it and never rebuild the Service name, the port, or the authentication from
the internals of the CamundaCluster controller. It is empty while the
cluster is suspended, because the management API is then unreachable.



_Appears in:_
- [CamundaClusterStatus](#camundaclusterstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `endpoint` _string_ | Endpoint is the base URL of the management API, for example<br />http://my-cluster-zeebe.my-namespace.svc:9600. |  |  |
| `auth` _[ManagementAuth](#managementauth)_ | Auth is how a consumer authenticates against the endpoint. |  |  |
| `version` _string_ | Version is the Camunda version of the cluster, for example 8.9.9. A<br />consumer selects its endpoint set from the minor version. |  |  |
| `partitions` _integer_ | Partitions is the partition count of the cluster. A restore must match<br />it, so a backup records it. |  |  |
| `backupRepository` _string_ | BackupRepository is the snapshot repository that the components write<br />backups to. It is set only on an Elasticsearch-backed cluster with a<br />backupStorageRef, and comes from the SecondaryStorageConfig. |  | Optional: \{\} <br /> |


#### ManagementClients



ManagementClients names the identity provider client of each component of
the management plane. A CamundaManagementCluster reports InvalidReference
when a component it deploys has no client here.



_Appears in:_
- [ManagementOIDCClientsSpec](#managementoidcclientsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `identity` _[ConfidentialClientSpec](#confidentialclientspec)_ | Identity is the client of Management Identity. |  | Optional: \{\} <br /> |
| `optimize` _[ConfidentialClientSpec](#confidentialclientspec)_ | Optimize is the client of Optimize. The ManagementAuthConfig that the<br />management cluster writes carries it, and Optimize reads it from there. |  | Optional: \{\} <br /> |
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
| `clients` _[ManagementClients](#managementclients)_ | Clients holds one entry per component of the management plane. |  |  |


#### ManagementOIDCSpec



ManagementOIDCSpec selects the identity provider of the referenced
CamundaPlatformConfig. The clients of the management plane live there, under
spec.auth.oidc.management.clients, so this block carries no fields.



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
fields follow the OIDC discovery vocabulary and work with any OIDC-compliant
provider.



_Appears in:_
- [PlatformAuthSpec](#platformauthspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `issuerUrl` _string_ | IssuerURL is the issuer URL of the identity provider. Consumers resolve<br />the endpoints from its OIDC discovery document unless the explicit<br />endpoint fields override them. |  |  |
| `providerType` _[OIDCProviderType](#oidcprovidertype)_ | ProviderType names the kind of identity provider. Management Identity<br />reads it and changes how it resolves users and groups. Empty means<br />generic, which fits any OIDC-compliant provider. Set microsoft for<br />Microsoft Entra ID. |  | Enum: [generic microsoft] <br />Optional: \{\} <br /> |
| `jwksUrl` _string_ | JWKSURL is an explicit JWKS endpoint. It overrides the value from OIDC<br />discovery. |  | Optional: \{\} <br /> |
| `tokenUrl` _string_ | TokenURL is an explicit token endpoint. It overrides the value from OIDC<br />discovery. |  | Optional: \{\} <br /> |
| `authUrl` _string_ | AuthURL is an explicit authorization endpoint. It overrides the value<br />from OIDC discovery. |  | Optional: \{\} <br /> |
| `clientId` _string_ | ClientID is the default OIDC client ID that all clusters share unless a<br />preset or a cluster overrides it. |  | MinLength: 1 <br /> |
| `audience` _string_ | Audience is the audience that consumers validate in access tokens.<br />Consumers default it to ClientID when empty. |  | Optional: \{\} <br /> |
| `usernameClaim` _string_ | UsernameClaim is the token claim that holds the username of a person.<br />Empty means the default of the orchestration cluster, which is "sub". |  | Optional: \{\} <br /> |
| `clientIdClaim` _string_ | ClientIDClaim is the token claim that holds the id of a machine client.<br />Empty means that no claim identifies a client, and every token becomes a<br />person. The claim must be absent from the tokens of persons, because a<br />token that carries it always becomes a client. |  | Optional: \{\} <br /> |
| `clientSecretRef` _[SecretKeyRef](#secretkeyref)_ | ClientSecretRef names the Secret key that holds the default OIDC client<br />secret. |  |  |
| `management` _[ManagementOIDCClientsSpec](#managementoidcclientsspec)_ | Management holds the clients that the management plane uses at this<br />identity provider. A CamundaManagementCluster in the oidc mode reads<br />them. Register one client per component at the provider first. |  | Optional: \{\} <br /> |


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



ObjectStorageConfig is the contract CRD that describes a bucket — for
backups or document storage — and how consumers authenticate against it:
workload identity on the consumer's ServiceAccount, or static credentials
in a Secret.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `ObjectStorageConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[ObjectStorageConfigSpec](#objectstorageconfigspec)_ | spec defines the desired state of ObjectStorageConfig |  | Required: \{\} <br /> |
| `status` _[ObjectStorageConfigStatus](#objectstorageconfigstatus)_ | status defines the observed state of ObjectStorageConfig |  | Optional: \{\} <br /> |


#### ObjectStorageConfigSpec



ObjectStorageConfigSpec describes a bucket and how consumers authenticate
against it. It is a discriminated union: type selects exactly one of the
s3, gcs, and azureBlob blocks.



_Appears in:_
- [ObjectStorageConfig](#objectstorageconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ObjectStorageType](#objectstoragetype)_ | Type selects the storage API of the bucket. |  | Enum: [S3 GCS AzureBlob] <br /> |
| `s3` _[S3Storage](#s3storage)_ | S3 describes an S3 or S3-compatible bucket. Required when type is S3,<br />forbidden otherwise. |  | Optional: \{\} <br /> |
| `gcs` _[GCSStorage](#gcsstorage)_ | GCS describes a Google Cloud Storage bucket. Required when type is<br />GCS, forbidden otherwise. |  | Optional: \{\} <br /> |
| `azureBlob` _[AzureBlobStorage](#azureblobstorage)_ | AzureBlob describes an Azure Blob Storage container. Required when<br />type is AzureBlob, forbidden otherwise. |  | Optional: \{\} <br /> |


#### ObjectStorageConfigStatus



ObjectStorageConfigStatus is the observed validation state of the contract.



_Appears in:_
- [ObjectStorageConfig](#objectstorageconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current validation state; the Ready condition<br />carries the reasons Healthy and MissingSecret. |  | Optional: \{\} <br /> |




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
| `serviceMonitor` _[ServiceMonitorSpec](#servicemonitorspec)_ | ServiceMonitor configures the Prometheus ServiceMonitors. When enabled,<br />the operator creates one ServiceMonitor per Deployment, named like the<br />workload, that scrapes /actuator/prometheus on the management port. |  | Optional: \{\} <br /> |


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
| `Cluster` | OptimizeSuspensionCluster means that the referenced cluster reports<br />itself suspended, by spec.suspend or in a state in which the operator<br />holds it at zero.<br /> |
| `StorageClaim` | OptimizeSuspensionStorageClaim means that the referenced cluster does not<br />hold the storage claim of its backend, or that another writer still<br />writes that backend: pods of another cluster or of a previous Optimize<br />instance, or a restore into another cluster.<br /> |


#### PITRCapability



PITRCapability declares a server's point-in-time-recovery capability: that it
performs continuous WAL archiving with the given retention.



_Appears in:_
- [DatabaseServerConfigSpec](#databaseserverconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled reports whether the server performs continuous WAL archiving. | false | Optional: \{\} <br /> |
| `retentionPeriodDays` _integer_ | RetentionPeriodDays is how many days into the past a point-in-time<br />restore can target. Required when enabled is true.<br />The maximum is 36500 days, which is a hundred years. A reader counts the<br />reachable window in nanoseconds, and a longer period overflows that<br />count and puts the oldest reachable point in the future, which makes<br />every restore unreachable. |  | Maximum: 36500 <br />Optional: \{\} <br /> |
| `recovery` _[RecoveryMode](#recoverymode)_ | Recovery says who rolls the server back to a point in time. operator<br />means that whoever publishes this contract answers spec.recovery, and<br />it requires enabled: true. Defaults to external, which means that<br />nobody does and the server is rolled back by hand. | external | Enum: [operator external] <br />Optional: \{\} <br /> |
| `lastRecovery` _[RecoveryOutcome](#recoveryoutcome)_ | LastRecovery is how the last recovery request ended. It is unset until<br />the first request is answered, and it is replaced by the answer to<br />every later one. |  | Optional: \{\} <br /> |


#### PartitionPosition



PartitionPosition is the exporter position of one partition, as the
pre-check read it from the restored database.



_Appears in:_
- [PointInTimeRestoreStatus](#pointintimerestorestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `partitionId` _integer_ | PartitionID is the Zeebe partition. |  |  |
| `lastUpdated` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | LastUpdated is the LAST_UPDATED value of the partition's row in the<br />EXPORTER_POSITION table. |  |  |


#### PersistentVolumeClaimRetentionPolicy



PersistentVolumeClaimRetentionPolicy mirrors the StatefulSet field of the
same name. Only whenDeleted exists: ECK deletes the volume of every
Elasticsearch node that it scales away, and the operator always keeps the
volume of a broker that is scaled away, so there is no whenScaled choice.



_Appears in:_
- [ElasticsearchClusterSpec](#elasticsearchclusterspec)
- [ZeebeSpec](#zeebespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `whenDeleted` _[PersistentVolumeClaimRetentionPolicyType](#persistentvolumeclaimretentionpolicytype)_ | WhenDeleted is what happens to the data volumes when the resource is<br />deleted. Delete removes them with the resource. Retain keeps them, and<br />a later resource with the same name reattaches them. Defaults to<br />Delete. | Delete | Enum: [Retain Delete] <br />Optional: \{\} <br /> |


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
| `Retain` | RetainPersistentVolumeClaimRetentionPolicyType keeps the data volumes.<br />Removing the data is a manual act. For an ElasticsearchCluster the ECK<br />resource carries the volume claim delete policy DeleteOnScaledownOnly;<br />for a CamundaCluster the broker StatefulSet carries whenDeleted Retain.<br /> |
| `Delete` | DeletePersistentVolumeClaimRetentionPolicyType deletes the data volumes<br />with the resource. For an ElasticsearchCluster the ECK resource carries<br />the volume claim delete policy DeleteOnScaledownAndClusterDeletion, the<br />ECK default; for a CamundaCluster the broker StatefulSet carries<br />whenDeleted Delete.<br /> |


#### PinnedStorage



PinnedStorage is the destination of a backup set, recorded when the backup
starts. It names the storage contract and the Elasticsearch endpoint that
hold the snapshots. It names the backup bucket that holds the runtime
backup. The snapshot repository is pinned by name in status.repository.
Every step verifies the destination before it writes. The finalizer
verifies it before it deletes. A destination that moved mid-run therefore
splits no set and aims no delete at the wrong place.



_Appears in:_
- [LogicalBackupElasticsearchStatus](#logicalbackupelasticsearchstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secondaryStorageConfig` _string_ | SecondaryStorageConfig is the name of the storage contract, in the<br />namespace of the backup. |  |  |
| `endpoint` _string_ | Endpoint is the Elasticsearch endpoint that the contract named. |  |  |
| `bucketRef` _string_ | BucketRef is the ObjectStorageConfig that the cluster backed up<br />through when the backup started: its spec.backupStorageRef then. The<br />runtime backup lands in that bucket. |  |  |
| `bucketLocation` _string_ | BucketLocation is where that contract pointed: the storage type,<br />bucket, base path, and endpoint. The steps write, and the finalizer<br />deletes, only while the contract still points there. |  |  |


#### PlatformAuthSpec



PlatformAuthSpec selects the authentication method of every orchestration
cluster that references the platform config.



_Appears in:_
- [CamundaPlatformConfigSpec](#camundaplatformconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `method` _[AuthenticationMethod](#authenticationmethod)_ | Method is the authentication method. An unset auth block and an unset<br />method both mean basic. | basic | Enum: [basic oidc] <br />Optional: \{\} <br /> |
| `oidc` _[OIDCSpec](#oidcspec)_ | OIDC is the identity provider connection. Required when method is oidc,<br />forbidden otherwise. |  | Optional: \{\} <br /> |


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



PointInTimeRestore aligns the primary storage of a suspended,
relational-backed CamundaCluster with a database at a point in time. It
reads the exporter position of every partition from that database, deletes
and creates the broker data volumes again, and runs the Camunda restore
application with the requested point once per broker.

The database reaches that point in one of two ways. A DatabaseServerConfig
that declares pitr.recovery: operator is rolled back by whoever publishes
it: the restore writes the request on the contract and waits for the
answer. A contract that declares external, the default, is rolled back
before the restore is created, and the restore reads the database as it
finds it.

The restore prepares the cluster itself. It suspends the cluster and waits
for its brokers to stop. It withdraws the suspension when it completes, and
only when it applied that suspension itself. A failed restore leaves the
cluster suspended, and so does a restore that somebody deletes while it
runs: broker volumes that are empty or half written are worse under running
brokers than under none.

It writes no version. This kind restores the primary storage of the cluster
from the continuous backups of that same cluster, so no backup names a
version that the cluster is not already running.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `PointInTimeRestore` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[PointInTimeRestoreSpec](#pointintimerestorespec)_ | spec names the cluster to align and the point its database holds. It is<br />immutable: a restore is one-shot, retried by creating a new resource. |  | Required: \{\} <br /> |
| `status` _[PointInTimeRestoreStatus](#pointintimerestorestatus)_ | status defines the observed state of the restore |  | Optional: \{\} <br /> |


#### PointInTimeRestorePhase

_Underlying type:_ _string_

PointInTimeRestorePhase tracks the one-shot restore. Completed and Failed
are terminal. A retry is a new resource.

_Validation:_
- Enum: [Pending RestoringDatabase ValidatingDatabaseState RestoringPrimaryStorage Completed Failed]

_Appears in:_
- [PointInTimeRestoreStatus](#pointintimerestorestatus)

| Field | Description |
| --- | --- |
| `Pending` | PointInTimeRestorePending means that the restore did not start real<br />work. A pre-check still holds it: the cluster runs, the storage chain<br />does not resolve, or the database is ahead of spec.timestamp.<br /> |
| `RestoringDatabase` | PointInTimeRestoreRestoringDatabase means that the operator asked the<br />database server to roll itself back to spec.timestamp and waits for the<br />answer. The restore reaches this phase only when the<br />DatabaseServerConfig declares pitr.recovery: operator. A server that<br />declares external is rolled back before the restore is created, and the<br />restore goes straight to ValidatingDatabaseState.<br /> |
| `ValidatingDatabaseState` | PointInTimeRestoreValidatingDatabaseState means that the operator reads<br />the exporter position of every partition from the restored database.<br />It runs before the operator touches a volume.<br /> |
| `RestoringPrimaryStorage` | PointInTimeRestoreRestoringPrimaryStorage means that the operator<br />recreated the broker data volumes and runs the restore application on<br />them.<br /> |
| `Completed` | PointInTimeRestoreCompleted means that the restore finished. The<br />operator removes the per-broker Jobs here, so their pods release the<br />broker data volumes, and it withdraws the suspension it applied, so the<br />cluster runs again unless its owner suspended it.<br /> |
| `Failed` | PointInTimeRestoreFailed means that the restore failed. The Ready<br />condition names the failing phase. The operator keeps the per-broker<br />Jobs, because their logs are the diagnosis, so a restore that reached<br />RestoringPrimaryStorage holds the broker data volumes until somebody<br />deletes it. A restore that failed in an earlier phase records no Job in<br />PrimaryJobNames and holds nothing.<br /> |


#### PointInTimeRestoreSpec



PointInTimeRestoreSpec names the cluster to roll back and the point in time
to roll the cluster back to. The whole spec is immutable.



_Appears in:_
- [PointInTimeRestore](#pointintimerestore)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `clusterRef` _[ClusterRef](#clusterref)_ | ClusterRef references the CamundaCluster to align, in the namespace of<br />this restore. Its secondary storage must be a relational database. |  | Required: \{\} <br /> |
| `timestamp` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | Timestamp is the point to restore to. DatabaseServerConfig.spec.pitr.recovery<br />decides who rolls the database server back to it. With operator, the<br />restore asks the server to roll back to this point. With external, you<br />roll the server back to it before you create the restore.<br />Choose that point at least one backup interval before the cluster<br />stopped writing, and inside the window that Zeebe keeps its<br />primary-storage backups for. Those are spec.backup.primaryStorage<br />schedule and retention.window of the CamundaCluster, which default to<br />one hour and seven days. A point that no backup covers fails the<br />restore after it erased the broker volumes.<br />It must also lie within the retention period that the database server<br />declares, and it must not lie in the future. The operator checks both<br />at reconcile time, because a CEL rule has no clock. |  | Required: \{\} <br /> |


#### PointInTimeRestoreStatus



PointInTimeRestoreStatus tracks the restore to a terminal phase.



_Appears in:_
- [PointInTimeRestore](#pointintimerestore)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[PointInTimeRestorePhase](#pointintimerestorephase)_ | Phase of the restore. It is the resume marker. |  | Enum: [Pending RestoringDatabase ValidatingDatabaseState RestoringPrimaryStorage Completed Failed] <br />Optional: \{\} <br /> |
| `storage` _[PointInTimeRestoreStorage](#pointintimerestorestorage)_ | Storage pins the storage chain that the restore validated. The operator<br />records it once, before it reads the database, and fails the restore<br />when a later look disagrees. |  | Optional: \{\} <br /> |
| `backend` _string_ | Backend names the database that the restore holds while its server rolls<br />back: the host, the port, and the database name. The operator records it<br />just before it asks for the rollback, and it follows each endpoint that<br />the contract names. From then to the terminal phase, no other<br />CamundaCluster starts on this database, whatever endpoint the contract<br />names. A restore whose server is rolled back<br />outside the operator records none. |  | Optional: \{\} <br /> |
| `observedPositions` _[PartitionPosition](#partitionposition) array_ | ObservedPositions are the exporter positions the pre-check read, in<br />partition order. They record what the operator saw when it let the<br />restore past the database-state check, or what held it. |  | Optional: \{\} <br /> |
| `targetClusterUID` _[UID](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#uid-types-pkg)_ | TargetClusterUID pins the identity of the target cluster. A cluster<br />that is deleted and created again under the same name is another<br />cluster, and this restore is not its restore. |  | Optional: \{\} <br /> |
| `brokers` _integer_ | Brokers is the broker count read from the live broker StatefulSet and<br />recorded before the restore deletes a volume. It fixes how many volumes<br />are recreated and how many Jobs run. |  | Optional: \{\} <br /> |
| `primaryJobNames` _string array_ | PrimaryJobNames are the per-broker restore-application Jobs, in broker<br />order. The operator records them before it applies the Jobs, so the<br />record covers every Job that the next look finds. It is also the list<br />that a restore which completed removes, and the list whose logs explain<br />a restore that failed. |  | Optional: \{\} <br /> |
| `recreatedClaims` _string array_ | RecreatedClaims names the broker data claims that the restore deleted<br />and created again. A reconcile that re-enters does not delete a claim<br />twice. |  | Optional: \{\} <br /> |
| `firstFailedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | FirstFailedAt is when a dependency of the running restore first stopped<br />resolving. The operator measures the mid-run grace from it. It stays<br />set once the restore starts, because a dependency that flaps must not<br />reset the grace. |  | Optional: \{\} <br /> |
| `clusterSuspended` _boolean_ | ClusterSuspended records that this restore suspended its target<br />cluster. The restore withdraws that suspension when it reaches<br />Completed. A cluster that its owner suspended carries no such record,<br />so it stays suspended, and so does the cluster of a failed restore. |  | Optional: \{\} <br /> |
| `terminalReason` _string_ | TerminalReason is the Ready reason recorded at the terminal transition.<br />The operator stages the terminal condition again from this field, so a<br />write conflict cannot replace the reason with a weaker one. |  | Optional: \{\} <br /> |
| `failureMessage` _string_ | FailureMessage names the failing phase and its error. The Ready<br />condition carries the same message. |  | Optional: \{\} <br /> |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | CompletionTime is when the restore reached a terminal phase. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state of the restore. |  | Optional: \{\} <br /> |


#### PointInTimeRestoreStorage



PointInTimeRestoreStorage is the identity of the storage chain that the
restore validated: the contracts it resolved, the logical database it read,
and the server that holds it. Every link of the chain is mutable, and the
rules of the server and the state of the database are checked once, before
the restore deletes anything. A later look that disagrees with this record
is another database, so the restore ends instead of acting on it.



_Appears in:_
- [PointInTimeRestoreStatus](#pointintimerestorestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secondaryStorageConfig` _string_ | SecondaryStorageConfig is the storage contract of the cluster. |  |  |
| `secondaryStorageConfigUID` _[UID](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#uid-types-pkg)_ | SecondaryStorageConfigUID pins the identity of that contract, so a<br />contract that was deleted and created again under one name is caught. |  | Optional: \{\} <br /> |
| `databaseConfig` _string_ | DatabaseConfig is the contract of the logical database. |  |  |
| `databaseConfigUID` _[UID](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#uid-types-pkg)_ | DatabaseConfigUID pins the identity of that contract. |  | Optional: \{\} <br /> |
| `databaseServerConfig` _string_ | DatabaseServerConfig is the contract of the server that holds the<br />database and declares its point-in-time recovery. |  |  |
| `databaseServerConfigUID` _[UID](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#uid-types-pkg)_ | DatabaseServerConfigUID pins the identity of that contract. |  | Optional: \{\} <br /> |
| `databaseName` _string_ | DatabaseName is the logical database whose exporter position the<br />pre-check read. |  |  |
| `endpoint` _string_ | Endpoint is the host and port of the server, as the pre-check reached<br />it. A server that is repointed in place is another server. |  |  |
| `systemIdentifier` _string_ | SystemIdentifier is the identity of the PostgreSQL instance behind that<br />endpoint, as the contract published it. It is what the dedicated-server<br />rule counted, and an endpoint that starts reporting another identity<br />holds another instance. |  |  |


#### PrimaryStorageBackupSpec



PrimaryStorageBackupSpec configures the backup scheduler of Zeebe. Camunda
takes these backups without an API call from this operator, on a relational
cluster. They pair with the database dump: a restore reads the exporter
position from the restored database and picks the primary-storage backups
that match it.



_Appears in:_
- [ClusterBackupSpec](#clusterbackupspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `continuous` _boolean_ | Continuous keeps every log segment until it is backed up, so that a<br />restore finds an unbroken range. Defaults to true on a relational<br />cluster with a backupStorageRef. It is a pointer, so a preset can turn<br />it on for a fleet and one cluster can still turn it off. |  | Optional: \{\} <br /> |
| `schedule` _string_ | Schedule is the interval at which Zeebe takes a primary-storage<br />backup: an ISO 8601 duration, a CRON expression, or "none". Defaults<br />to PT1H. Always pair a schedule with continuous, or the log grows<br />without bound. |  | MinLength: 1 <br />Optional: \{\} <br /> |
| `checkpointInterval` _string_ | CheckpointInterval is the interval at which Zeebe writes marker<br />checkpoints into the log stream, as an ISO 8601 duration of days and<br />time (P2DT3H, PT15M). Camunda parses no week, month, or year units.<br />It is the granularity of a point-in-time restore. Defaults to PT15M. |  | Pattern: `^P([0-9]+D(T(([0-9]+H)([0-9]+M)?([0-9]+([.][0-9]+)?S)?\|([0-9]+M)([0-9]+([.][0-9]+)?S)?\|[0-9]+([.][0-9]+)?S))?\|T(([0-9]+H)([0-9]+M)?([0-9]+([.][0-9]+)?S)?\|([0-9]+M)([0-9]+([.][0-9]+)?S)?\|[0-9]+([.][0-9]+)?S))$` <br />Optional: \{\} <br /> |
| `retention` _[PrimaryStorageRetentionSpec](#primarystorageretentionspec)_ | Retention bounds how long Zeebe keeps its primary-storage backups. |  | Optional: \{\} <br /> |


#### PrimaryStorageRetentionSpec



PrimaryStorageRetentionSpec bounds the primary-storage backups that Zeebe
keeps. Zeebe always keeps at least one backup, even outside the window.



_Appears in:_
- [PrimaryStorageBackupSpec](#primarystoragebackupspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `window` _string_ | Window is how far back the backups stay available for a restore, as an<br />ISO 8601 duration of days and time (P7D, PT12H). Camunda parses no<br />week, month, or year units. Defaults to P7D. It bounds the restore<br />window, so set it at least as long as the recovery point the cluster<br />needs. A BackupSchedule that keeps database dumps for longer than this<br />window keeps dumps that can no longer be restored. |  | Pattern: `^P([0-9]+D(T(([0-9]+H)([0-9]+M)?([0-9]+([.][0-9]+)?S)?\|([0-9]+M)([0-9]+([.][0-9]+)?S)?\|[0-9]+([.][0-9]+)?S))?\|T(([0-9]+H)([0-9]+M)?([0-9]+([.][0-9]+)?S)?\|([0-9]+M)([0-9]+([.][0-9]+)?S)?\|[0-9]+([.][0-9]+)?S))$` <br />Optional: \{\} <br /> |
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
| `databaseConfigRef` _string_ | DatabaseConfigRef names the DatabaseConfig, in this contract's own<br />namespace, describing the logical database to use. |  | MinLength: 1 <br /> |


#### RecoveryArchiveIdentity



RecoveryArchiveIdentity is the workload identity of a bucket: what the pods
of a consumer present to read the objects in it. A bucket that holds static
credentials has none.



_Appears in:_
- [RecoveryArchiveRef](#recoveryarchiveref)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `annotations` _object (keys:string, values:string)_ | Annotations are the annotations of the ServiceAccount of the pods. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are the labels the pods themselves need. Only Azure has one. |  | Optional: \{\} <br /> |


#### RecoveryArchiveRef



RecoveryArchiveRef names the archive that a recovery reads: the directory in
the bucket, where that bucket is, and the bucket contract that names it. It
also carries the archive settings of the server at that moment. Every edit
of spec.archive while the rollback is unanswered is held against them: a
moved bucket, a changed retention or schedule, and a removal. The archive
keeps being rendered as it was until the rollback is answered.



_Appears in:_
- [DatabaseServerRecoveryStatus](#databaseserverrecoverystatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serverName` _string_ | ServerName is the archive directory, equal to the name of the<br />CloudNativePG cluster that wrote it. |  |  |
| `objectStorageRef` _string_ | ObjectStorageRef is the ObjectStorageConfig, in the namespace of this<br />server, that the archive lives in. |  |  |
| `location` _string_ | Location is where in object storage the archive lives, in the form<br />ArchiveRecord.Location takes. A running recovery keeps reading the<br />location it recorded, so an ObjectStorageConfig edited in the middle<br />does not move it. |  | Optional: \{\} <br /> |
| `retentionPeriodDays` _integer_ | RetentionPeriodDays is spec.archive.retentionPeriodDays as it stood<br />when the rollback started. |  | Optional: \{\} <br /> |
| `baseBackupSchedule` _string_ | BaseBackupSchedule is spec.archive.baseBackupSchedule as it stood when<br />the rollback started. |  | Optional: \{\} <br /> |
| `identity` _[RecoveryArchiveIdentity](#recoveryarchiveidentity)_ | Identity is the workload identity of that bucket when the rollback<br />started. It is held with the archive, so the pods keep presenting the<br />identity of the bucket they read. It is unset for a bucket that holds<br />static credentials, and for one that names no identity. |  | Optional: \{\} <br /> |


#### RecoveryMode

_Underlying type:_ _string_

RecoveryMode says who rolls the server back to a point in time.

_Validation:_
- Enum: [operator external]

_Appears in:_
- [PITRCapability](#pitrcapability)

| Field | Description |
| --- | --- |
| `operator` | RecoveryModeOperator means that whoever publishes this contract rolls<br />the server back when spec.recovery asks for it.<br /> |
| `external` | RecoveryModeExternal means that nobody answers spec.recovery. The<br />server is rolled back by hand, before the restore starts.<br /> |


#### RecoveryOutcome



RecoveryOutcome is how a recovery request ended. It repeats the request it
answers, so a consumer knows whether the answer is the answer to its own
request.



_Appears in:_
- [PITRCapability](#pitrcapability)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requestID` _string_ | RequestID is the requestID of the request this outcome answers. |  | Pattern: `^[0-9a-f]\{8\}-[0-9a-f]\{4\}-[0-9a-f]\{4\}-[0-9a-f]\{4\}-[0-9a-f]\{12\}$` <br /> |
| `requestedBy` _string_ | RequestedBy is the requestedBy of the request this outcome answers. |  | MaxLength: 507 <br />Pattern: `^[a-z0-9]([-a-z0-9.]*[a-z0-9])?/[a-z0-9]([-a-z0-9.]*[a-z0-9])?$` <br /> |
| `targetTime` _string_ | TargetTime is the targetTime of the request this outcome answers. It<br />carries the shape of the request, so a consumer that compares the two<br />as text compares two values of one form. |  | Format: date-time <br />Pattern: `^\d\{4\}-\d\{2\}-\d\{2\}T\d\{2\}:\d\{2\}:\d\{2\}(\.\d+)?(Z\|[+-]\d\{2\}:\d\{2\})$` <br /> |
| `completedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | CompletedAt is when the request ended. |  |  |
| `result` _[RecoveryResult](#recoveryresult)_ | Result is how the request ended. See RecoveryResult for the values. |  | Enum: [Completed Failed Unavailable] <br /> |
| `message` _string_ | Message says what happened. It is empty for a result of Completed. |  | Optional: \{\} <br /> |


#### RecoveryRequest



RecoveryRequest asks whoever publishes this contract to roll the server back
to a point in time. A consumer writes it under a field manager of its own,
and the publisher of the contract never carries the field, so the two
writers never meet on it.



_Appears in:_
- [DatabaseServerConfigSpec](#databaseserverconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `requestID` _string_ | RequestID identifies this request, and only this one. A controller sets<br />the uid of the resource that asks. A request written by hand carries<br />any UUID. It is what tells two requests apart that name one resource<br />and one point: a resource that is deleted and created again under its<br />name is another requester, and the answer to the request of the first<br />says nothing about the state the second asks for. |  | Pattern: `^[0-9a-f]\{8\}-[0-9a-f]\{4\}-[0-9a-f]\{4\}-[0-9a-f]\{4\}-[0-9a-f]\{12\}$` <br /> |
| `requestedBy` _string_ | RequestedBy is the namespace and the name of the resource that asks, as<br />"<namespace>/<name>". It comes back in pitr.lastRecovery, which is how<br />the requester tells its own answer from somebody else's. |  | MaxLength: 507 <br />Pattern: `^[a-z0-9]([-a-z0-9.]*[a-z0-9])?/[a-z0-9]([-a-z0-9.]*[a-z0-9])?$` <br /> |
| `targetTime` _string_ | TargetTime is the point to roll back to, as RFC 3339 with a zone, for<br />example 2026-08-20T14:30:00Z. A timestamp without a zone is rejected:<br />PostgreSQL reads it as the local time of the server, so a request that<br />means one point to the writer means another to the server. |  | Format: date-time <br />Pattern: `^\d\{4\}-\d\{2\}-\d\{2\}T\d\{2\}:\d\{2\}:\d\{2\}(\.\d+)?(Z\|[+-]\d\{2\}:\d\{2\})$` <br /> |


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
| `version` _string_ | Version is the PostgreSQL major version of the servers of this release,<br />as a bare number such as "17". The floor of PostgreSQL 14 is enforced<br />by the controller of each referencing server on the merged spec. A<br />server that already runs another major refuses the change and keeps the<br />major it has. |  | Pattern: `^\d+$` <br />Optional: \{\} <br /> |


#### ReleaseElasticsearchSpec



ReleaseElasticsearchSpec is the Elasticsearch of a release.



_Appears in:_
- [CamundaReleaseSpec](#camundareleasespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ | Version is the Elasticsearch version of the clusters of this release,<br />as a full semantic version. Elasticsearch has a patch line of its own,<br />so it does not follow the Camunda version. The floor of Elasticsearch<br />8.19 or 9.2 is enforced by the controller of each referencing cluster<br />on the merged spec. |  | Pattern: `^\d+\.\d+\.\d+$` <br />Optional: \{\} <br /> |


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
has. It is embedded with json:",inline", so each status keeps the field
names it had before and the CRD schema does not change. controller-gen
flattens an inline embedded struct the same way encoding/json does.

pkg/restore reads and writes this struct in place, through the driver
calls that every restore kind makes. TargetClusterUID is the exception:
each controller pins it during its own admission, before the driver first
runs, so that every rule it checks is measured against one cluster. A
PointInTimeRestore pins Brokers in its admission too, because its rules
read the live broker StatefulSet there. For the other kinds the driver
pins Brokers on its first primary-storage pass. Each kind owns its own
phase and the fields of its own procedure.



_Appears in:_
- [LogicalRestoreElasticsearchStatus](#logicalrestoreelasticsearchstatus)
- [LogicalRestoreRDBMSStatus](#logicalrestorerdbmsstatus)
- [PointInTimeRestoreStatus](#pointintimerestorestatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `targetClusterUID` _[UID](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#uid-types-pkg)_ | TargetClusterUID pins the identity of the target cluster. A cluster<br />that is deleted and created again under the same name is another<br />cluster, and this restore is not its restore. |  | Optional: \{\} <br /> |
| `brokers` _integer_ | Brokers is the broker count read from the live broker StatefulSet and<br />recorded before the restore deletes a volume. It fixes how many volumes<br />are recreated and how many Jobs run. |  | Optional: \{\} <br /> |
| `primaryJobNames` _string array_ | PrimaryJobNames are the per-broker restore-application Jobs, in broker<br />order. The operator records them before it applies the Jobs, so the<br />record covers every Job that the next look finds. It is also the list<br />that a restore which completed removes, and the list whose logs explain<br />a restore that failed. |  | Optional: \{\} <br /> |
| `recreatedClaims` _string array_ | RecreatedClaims names the broker data claims that the restore deleted<br />and created again. A reconcile that re-enters does not delete a claim<br />twice. |  | Optional: \{\} <br /> |
| `firstFailedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | FirstFailedAt is when a dependency of the running restore first stopped<br />resolving. The operator measures the mid-run grace from it. It stays<br />set once the restore starts, because a dependency that flaps must not<br />reset the grace. |  | Optional: \{\} <br /> |
| `clusterSuspended` _boolean_ | ClusterSuspended records that this restore suspended its target<br />cluster. The restore withdraws that suspension when it reaches<br />Completed. A cluster that its owner suspended carries no such record,<br />so it stays suspended, and so does the cluster of a failed restore. |  | Optional: \{\} <br /> |
| `terminalReason` _string_ | TerminalReason is the Ready reason recorded at the terminal transition.<br />The operator stages the terminal condition again from this field, so a<br />write conflict cannot replace the reason with a weaker one. |  | Optional: \{\} <br /> |
| `failureMessage` _string_ | FailureMessage names the failing phase and its error. The Ready<br />condition carries the same message. |  | Optional: \{\} <br /> |
| `completionTime` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#time-v1-meta)_ | CompletionTime is when the restore reached a terminal phase. |  | Optional: \{\} <br /> |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current state of the restore. |  | Optional: \{\} <br /> |


#### RetainedBackups



RetainedBackups bounds the backups that a schedule keeps, by terminal
phase. When the count of a phase exceeds its bound, the oldest backups
beyond it are deleted through the backup finalizer, which removes the
stored artifacts too.



_Appears in:_
- [BackupScheduleSpec](#backupschedulespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `completed` _integer_ | Completed is how many completed backups the schedule keeps. | 7 | Minimum: 1 <br />Optional: \{\} <br /> |
| `failed` _integer_ | Failed is how many failed backups the schedule keeps. Zero deletes a<br />failed backup at the first look after it fails. | 3 | Minimum: 0 <br />Optional: \{\} <br /> |


#### S3Credentials



S3Credentials holds the static keys of an S3 bucket.



_Appears in:_
- [S3StorageAuth](#s3storageauth)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretRef` _[S3CredentialsSecretRef](#s3credentialssecretref)_ | SecretRef names the Secret keys that hold the access-key pair. |  |  |


#### S3CredentialsSecretRef



S3CredentialsSecretRef references an access-key pair stored in a Secret of
the namespace of the contract.



_Appears in:_
- [S3Credentials](#s3credentials)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the Secret holding the keys. |  | MinLength: 1 <br /> |
| `accessKeyIdKey` _string_ | AccessKeyIDKey is the key in the Secret holding the access key ID. |  | MinLength: 1 <br /> |
| `secretAccessKeyKey` _string_ | SecretAccessKeyKey is the key in the Secret holding the secret access<br />key. |  | MinLength: 1 <br /> |


#### S3Storage



S3Storage describes an S3 or S3-compatible bucket.

The rule below compares with size() rather than the empty string literal:
gofmt rewrites a doubled single quote in the doc comment of a declaration
into a typographic quote, which would silently invalidate the expression.



_Appears in:_
- [ObjectStorageConfigSpec](#objectstorageconfigspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `bucketName` _string_ | BucketName is the bucket name as used by storage client SDKs. |  | MinLength: 1 <br /> |
| `basePath` _string_ | BasePath is the key prefix under which consumers write objects,<br />without leading or trailing slashes. Empty means the bucket root. |  | Pattern: `^[^/]+(/[^/]+)*$` <br />Optional: \{\} <br /> |
| `region` _string_ | Region of the bucket. Required unless endpoint is set. |  | Optional: \{\} <br /> |
| `endpoint` _string_ | Endpoint is the URL of an S3-compatible store (MinIO, Ceph, and more).<br />Empty means AWS S3, addressed through region. |  | Optional: \{\} <br /> |
| `forcePathStyle` _boolean_ | ForcePathStyle forces path-style bucket addressing. Set it for<br />S3-compatible stores that do not serve virtual-hosted-style requests. |  | Optional: \{\} <br /> |
| `auth` _[S3StorageAuth](#s3storageauth)_ | Auth selects how consumers authenticate. An absent block means<br />workload identity through the ServiceAccount chain. | \{ type:workloadIdentity \} | Optional: \{\} <br /> |


#### S3StorageAuth



S3StorageAuth selects how consumers authenticate against an S3 bucket.



_Appears in:_
- [S3Storage](#s3storage)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[ObjectStorageAuthType](#objectstorageauthtype)_ | Type is the authentication choice. Defaults to workloadIdentity. | workloadIdentity | Enum: [workloadIdentity credentials] <br />Optional: \{\} <br /> |
| `workloadIdentity` _[S3WorkloadIdentity](#s3workloadidentity)_ | WorkloadIdentity names the trusted principal. Only valid with type<br />workloadIdentity; an empty or absent block means "trust the<br />ServiceAccount chain, add nothing". |  | Optional: \{\} <br /> |
| `credentials` _[S3Credentials](#s3credentials)_ | Credentials are static keys. Required with type credentials, forbidden<br />otherwise. |  | Optional: \{\} <br /> |


#### S3WorkloadIdentity



S3WorkloadIdentity names the AWS principal that the bucket trusts. An empty
block means the consumer's ServiceAccount chain already carries the
identity (EKS Pod Identity), so the operator adds nothing.



_Appears in:_
- [S3StorageAuth](#s3storageauth)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `roleArn` _string_ | RoleARN is the IAM role that consumers assume. When set, the operator<br />puts it in the eks.amazonaws.com/role-arn annotation of the consumer's<br />ServiceAccount (IRSA). |  | Optional: \{\} <br /> |


#### SchedulingSpec



SchedulingSpec groups the scheduling constraints applied to the pods a
controller manages. A consumer resolving a preset replaces the preset's
entire scheduling block when it sets its own; the blocks are never merged
field by field.



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



ScratchVolumeSpec sizes the volume that holds a database dump until it is
uploaded. An unset block is an emptyDir that the node bounds, which a large
database can exhaust.



_Appears in:_
- [BackupDumpSpec](#backupdumpspec)
- [DumpPodSpec](#dumppodspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `sizeLimit` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#quantity-resource-api)_ | SizeLimit is the size of the emptyDir that holds the dump. Set it to<br />the size the dump needs, with room to spare. |  | Optional: \{\} <br /> |
| `storageClassName` _string_ | StorageClassName makes the scratch volume a PersistentVolumeClaim of<br />this class instead of an emptyDir. Use it when the dump is larger than<br />the ephemeral storage of a node. |  | Optional: \{\} <br /> |


#### SecondaryStorageConfig



SecondaryStorageConfig is the namespaced contract CRD that tells an
orchestration cluster where its secondary storage lives — an Elasticsearch
cluster or a relational database — and how to authenticate against it.
Consumers resolve references to it by name in their own namespace.





| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `core.camunda.io/v1` | | |
| `kind` _string_ | `SecondaryStorageConfig` | | |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  | Optional: \{\} <br /> |
| `spec` _[SecondaryStorageConfigSpec](#secondarystorageconfigspec)_ | spec defines the desired state of SecondaryStorageConfig |  | Required: \{\} <br /> |
| `status` _[SecondaryStorageConfigStatus](#secondarystorageconfigstatus)_ | status defines the observed state of SecondaryStorageConfig |  | Optional: \{\} <br /> |


#### SecondaryStorageConfigSpec



SecondaryStorageConfigSpec tells an orchestration cluster where its
secondary storage lives and how to authenticate against it.



_Appears in:_
- [SecondaryStorageConfig](#secondarystorageconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _[SecondaryStorageType](#secondarystoragetype)_ | Type selects which secondary storage backend this contract describes. |  | Enum: [elasticsearch rdbms] <br /> |
| `elasticsearch` _[ElasticsearchStorage](#elasticsearchstorage)_ | Elasticsearch connection details. Required when type is elasticsearch,<br />forbidden otherwise. |  | Optional: \{\} <br /> |
| `rdbms` _[RDBMSStorage](#rdbmsstorage)_ | RDBMS backend details. Required when type is rdbms, forbidden otherwise. |  | Optional: \{\} <br /> |


#### SecondaryStorageConfigStatus



SecondaryStorageConfigStatus is the observed validation state of the contract.



_Appears in:_
- [SecondaryStorageConfig](#secondarystorageconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `observedGeneration` _integer_ | ObservedGeneration is the last generation reconciled by the operator. |  | Optional: \{\} <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#condition-v1-meta) array_ | Conditions represent the current validation state; the Ready condition<br />carries reasons Healthy, MissingSecret, or InvalidReference. |  | Optional: \{\} <br /> |


#### SecondaryStorageType

_Underlying type:_ _string_

SecondaryStorageType identifies which secondary storage backend a contract
describes.

_Validation:_
- Enum: [elasticsearch rdbms]

_Appears in:_
- [SecondaryStorageConfigSpec](#secondarystorageconfigspec)

| Field | Description |
| --- | --- |
| `elasticsearch` | SecondaryStorageTypeElasticsearch selects an Elasticsearch backend.<br /> |
| `rdbms` | SecondaryStorageTypeRDBMS selects a relational database backend.<br /> |


#### SecretKeyRef



SecretKeyRef references a single value inside a Secret of a named namespace.
A cluster-scoped kind uses it, because it has no namespace of its own to
resolve a reference in. A namespaced kind uses LocalSecretKeyRef instead.



_Appears in:_
- [CamundaPlatformConfigSpec](#camundaplatformconfigspec)
- [ConfidentialClientSpec](#confidentialclientspec)
- [ManagementAuthConfigSpec](#managementauthconfigspec)
- [OIDCSpec](#oidcspec)
- [WebModelerAPIClientSpec](#webmodelerapiclientspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the Secret holding the value. |  | MinLength: 1 <br /> |
| `namespace` _string_ | Namespace of the Secret. |  | MinLength: 1 <br /> |
| `key` _string_ | Key in the Secret holding the value. |  | MinLength: 1 <br /> |


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



SecureSettingsSource references a Secret whose contents ECK loads into the
keystore of every Elasticsearch node. Elasticsearch reads credentials from
the keystore only, never from the settings of a snapshot repository.



_Appears in:_
- [ElasticsearchClusterSpec](#elasticsearchclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `secretName` _string_ | SecretName is the Secret, in the namespace of the ElasticsearchCluster. |  | MinLength: 1 <br /> |
| `entries` _[SecureSettingEntry](#securesettingentry) array_ | Entries projects single keys to keystore entries. An empty list loads<br />every key of the Secret under its own name. |  | Optional: \{\} <br /> |


#### ServiceAccountSpec



ServiceAccountSpec configures the ServiceAccount of the pods a controller
manages.



_Appears in:_
- [CamundaClusterSpec](#camundaclusterspec)
- [ElasticsearchClusterSpec](#elasticsearchclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name of the ServiceAccount. Empty means the name that the controller<br />derives from the name of the resource, which each CRD doc states. The<br />name is part of the contract with the cloud provider: a workload<br />identity that needs no annotation, such as EKS Pod Identity, binds the<br />principal system:serviceaccount:<namespace>:<name>. |  | MaxLength: 253 <br />Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$` <br />Optional: \{\} <br /> |
| `create` _boolean_ | Create renders and owns the ServiceAccount. Defaults to true. False<br />names one that already exists: the operator neither creates, annotates,<br />nor owns it, and a ServiceAccount that is absent then fails the<br />pre-check instead of leaving the pods unschedulable. The operator never<br />adopts a foreign ServiceAccount, because an owned one is deleted with<br />the resource. |  | Optional: \{\} <br /> |
| `annotations` _object (keys:string, values:string)_ | Annotations to set on the ServiceAccount, typically workload-identity<br />annotations (IRSA, GCP Workload Identity, ...) granting the pods access<br />to cloud resources such as the snapshot bucket used for backups. A<br />controller that resolves a bucket contract derives the annotation of<br />that contract's identity as well; an annotation set here wins over the<br />derived one on the same key. |  | Optional: \{\} <br /> |


#### ServiceMonitorSpec



ServiceMonitorSpec configures the Prometheus ServiceMonitors of a resource
that runs workloads. The owning kind says what is scraped: an
ElasticsearchCluster deploys the prometheus-community elasticsearch_exporter
(Elasticsearch serves no Prometheus endpoint itself) and scrapes it; a
CamundaCluster scrapes /actuator/prometheus of every process. The
ServiceMonitor is created only when the Kubernetes cluster serves the kind.



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
cluster. A status lists one entry per bound claim, sorted by name, so a
resize of a single claim outside the spec, for example by an auto-resize
controller, shows here.



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
| `mode` _[ComponentMode](#componentmode)_ | Mode selects a Deployment of the unified binary that serves only this<br />application (Standalone) or the nearest standalone host up the chain<br />(Embedded). Defaults to Embedded. The workload fields have an effect<br />only when the mode is Standalone, except extraEnv and extraEnvFrom,<br />which apply to the host process when the mode is Embedded. |  | Enum: [Standalone Embedded] <br />Optional: \{\} <br /> |
| `replicas` _integer_ | Replicas is the number of pods of this process. Defaults to 1. On a<br />CamundaCluster it has no effect on an embedded web application. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the container of this process. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the container of this<br />process. On a CamundaCluster, an entry here wins over a top-level entry<br />with the same name. The entries of an embedded web application apply to<br />its host process.<br />The list merges by name under server-side apply, so each field manager<br />owns only the entries that it applies. An extension controller can add<br />its own entry next to yours. One applied manifest cannot hold two<br />entries with the same name.<br />Two field managers that apply the same name do not conflict: the merge<br />is per field inside the entry, so one manager can own value while the<br />other owns valueFrom. A container rejects an entry that carries both,<br />so the rule below refuses to store that combination. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the<br />container of this process. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the pods of this process. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the pods of this process. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling constraints of the pods of this process. When set, it<br />replaces every scheduling block that would otherwise apply to this<br />process, with no merge. On a CamundaCluster those are the top-level<br />block and the block of a preset. |  | Optional: \{\} <br /> |


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
| `fromName` _string_ | FromName is the display name that Web Modeler sends under. |  | Optional: \{\} <br /> |
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
| `externalUrl` _string_ | ExternalURL is the URL that browsers reach Web Modeler at. |  |  |
| `websocketsExternalUrl` _string_ | WebsocketsExternalURL is the URL that browsers reach the websockets<br />process at. Web Modeler pushes live updates over it. |  |  |
| `databaseConfigRef` _string_ | DatabaseConfigRef names the DatabaseConfig of the Web Modeler database,<br />in the namespace of this resource. Web Modeler needs its own PostgreSQL<br />database. |  | MinLength: 1 <br /> |
| `mail` _[WebModelerMailSpec](#webmodelermailspec)_ | Mail configures the SMTP server that Web Modeler sends notifications<br />through. Web Modeler does not start without it. |  |  |
| `restapi` _[WorkloadSpec](#workloadspec)_ | Restapi configures the workload of the restapi process. |  | Optional: \{\} <br /> |
| `websockets` _[WorkloadSpec](#workloadspec)_ | Websockets configures the workload of the websockets process. |  | Optional: \{\} <br /> |


#### WorkloadSpec



WorkloadSpec is the override surface that a workload block shares. Every
component block of a CamundaCluster uses it, and so does each workload of a
CamundaOptimize. It tunes the size, the environment, the pod metadata, and
the scheduling of one process.



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
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the container of this<br />process. On a CamundaCluster, an entry here wins over a top-level entry<br />with the same name. The entries of an embedded web application apply to<br />its host process.<br />The list merges by name under server-side apply, so each field manager<br />owns only the entries that it applies. An extension controller can add<br />its own entry next to yours. One applied manifest cannot hold two<br />entries with the same name.<br />Two field managers that apply the same name do not conflict: the merge<br />is per field inside the entry, so one manager can own value while the<br />other owns valueFrom. A container rejects an entry that carries both,<br />so the rule below refuses to store that combination. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the<br />container of this process. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the pods of this process. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the pods of this process. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling constraints of the pods of this process. When set, it<br />replaces every scheduling block that would otherwise apply to this<br />process, with no merge. On a CamundaCluster those are the top-level<br />block and the block of a preset. |  | Optional: \{\} <br /> |


#### ZeebeSpec



ZeebeSpec configures the brokers. Zeebe is always a standalone StatefulSet
with persistent volumes.



_Appears in:_
- [CamundaClusterSpec](#camundaclusterspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ | Replicas is the number of pods of this process. Defaults to 1. On a<br />CamundaCluster it has no effect on an embedded web application. |  | Minimum: 0 <br />Optional: \{\} <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#resourcerequirements-v1-core)_ | Resources are the CPU and memory of the container of this process. |  | Optional: \{\} <br /> |
| `extraEnv` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envvar-v1-core) array_ | ExtraEnv are extra environment variables of the container of this<br />process. On a CamundaCluster, an entry here wins over a top-level entry<br />with the same name. The entries of an embedded web application apply to<br />its host process.<br />The list merges by name under server-side apply, so each field manager<br />owns only the entries that it applies. An extension controller can add<br />its own entry next to yours. One applied manifest cannot hold two<br />entries with the same name.<br />Two field managers that apply the same name do not conflict: the merge<br />is per field inside the entry, so one manager can own value while the<br />other owns valueFrom. A container rejects an entry that carries both,<br />so the rule below refuses to store that combination. |  | Optional: \{\} <br /> |
| `extraEnvFrom` _[EnvFromSource](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#envfromsource-v1-core) array_ | ExtraEnvFrom are extra environment sources (ConfigMaps, Secrets) of the<br />container of this process. |  | Optional: \{\} <br /> |
| `podLabels` _object (keys:string, values:string)_ | PodLabels are extra labels of the pods of this process. |  | Optional: \{\} <br /> |
| `podAnnotations` _object (keys:string, values:string)_ | PodAnnotations are extra annotations of the pods of this process. |  | Optional: \{\} <br /> |
| `scheduling` _[SchedulingSpec](#schedulingspec)_ | Scheduling constraints of the pods of this process. When set, it<br />replaces every scheduling block that would otherwise apply to this<br />process, with no merge. On a CamundaCluster those are the top-level<br />block and the block of a preset. |  | Optional: \{\} <br /> |
| `partitions` _integer_ | Partitions is the number of partitions. Defaults to 1. Once set, it can<br />neither be decreased nor removed. |  | Minimum: 1 <br />Optional: \{\} <br /> |
| `replicationFactor` _integer_ | ReplicationFactor is the number of brokers that hold a copy of each<br />partition. Defaults to 1. It must not exceed replicas. |  | Minimum: 1 <br />Optional: \{\} <br /> |
| `storageClassName` _string_ | StorageClassName is the StorageClass of the broker volumes. Defaults to<br />the default StorageClass of the Kubernetes cluster. It is immutable<br />after creation, because a StatefulSet volume claim template cannot<br />change its storage class. The CEL transition rule that rejects a change<br />sits on the spec field of CamundaCluster ("zeebe.storageClassName is<br />immutable"), not here: this type is shared with CamundaClusterPreset,<br />and a preset baseline stays free to change. |  | Optional: \{\} <br /> |
| `storageSize` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.36/#quantity-resource-api)_ | StorageSize is the size of the data volume of each broker. Defaults to<br />10Gi. It can only grow. Admission rejects a lower inline value on a<br />CamundaCluster through a CEL transition rule. That rule does not bind<br />this shared field, so a preset baseline can be resized freely: a<br />cluster that applied a larger size keeps it and records a<br />StorageShrinkIgnored event. On growth the operator expands the existing<br />claims in place, so the storage class must support volume expansion. |  | Optional: \{\} <br /> |
| `persistentVolumeClaimRetentionPolicy` _[PersistentVolumeClaimRetentionPolicy](#persistentvolumeclaimretentionpolicy)_ | PersistentVolumeClaimRetentionPolicy says what happens to the broker<br />volumes when the CamundaCluster is deleted. A scale-down and a<br />suspension always keep them. |  | Optional: \{\} <br /> |


