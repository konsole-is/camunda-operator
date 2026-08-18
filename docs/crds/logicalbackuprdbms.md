# LogicalBackupRDBMS

A LogicalBackupRDBMS is one backup of a relational orchestration cluster: a dump of the entire logical database, uploaded to the backup bucket, paired with one Zeebe backup.

Camunda calls the Zeebe log and snapshots its *primary storage* and the exported relational data its *secondary storage*. A **Zeebe backup** is Camunda's own backup of that primary storage to the backup bucket, requested through the cluster's management API. This page uses "Zeebe backup" throughout; the management API and the `CamundaCluster` backup settings call the same thing a primary-storage backup.

!!! note "Stub"
    The backup epic writes the full page when the end-to-end suite lands. This stub states the shipped schema and behavior.

## Purpose

A LogicalBackupRDBMS captures a complete restore point of a `CamundaCluster` that stores its data in a relational database. The dump holds the exporter position, and the Zeebe backup taken right after it holds the matching Zeebe state; a restore reads the position from the restored dump and picks the Zeebe backups that pair with it. You create one per backup you want taken.

## How it works

1. The operator resolves `clusterRef` and checks: the cluster exists, has a `backupStorageRef`, is not suspended, stores its data in a relational database, and no other backup of it holds the cluster (see *Serialization* below). The cluster must also have converged on its current spec — `status.observedGeneration` equals `metadata.generation` and `Ready` is `True` for that generation — because a backup taken while Zeebe still rolls out a previous configuration could pair a dump written to the new backup store with a Zeebe backup that lands in the old one; until then the backup waits with `Progressing`. The `DatabaseConfig` must name a `backupCredentialsSecretRef`, and the `DatabaseServerConfig` must have been probed for its current spec — `Ready=True` at its current generation with `status.serverVersion` published — because the dump runs client tools of the server's major version and a version left over from a retargeted server would pick the wrong ones. Zeebe must run the database those two name: the live Zeebe pod template carries the relational storage URL it is configured with, and it must equal the URL rendered from the `DatabaseServerConfig` host and port and the `DatabaseConfig` database name as resolved now — between an edit of either referent and the cluster controller's rendering the two differ, and a dump taken then would capture a database Zeebe does not run; until they agree the backup waits with `Progressing`. This CR's dump block may not supply anything under the `PG` or `UPLOAD_` prefixes: not in `extraEnv`, and not through an `extraEnvFrom` source — every source needs a prefix that cannot spell such a name, because libpq prefers `PGHOSTADDR` over the Job's own `PGHOST`, so an unbounded source could redirect the dump with the injected credentials. The cluster's own `spec.backup.dump` carries no such bound: its owner sets connection policy (`PGSSLMODE` included) inside their own boundary. These checks run only until the backup starts; a backup that fails them parks in `Pending` with the reason on `Ready`.
2. It allocates the backup id — a millisecond timestamp, or one more than the highest id any other backup of this kind and cluster holds, whichever is larger, so a clock that stepped backwards cannot reuse an id that a visible sibling holds; nothing arbitrates the ids of deleted backups, because the Zeebe request carries no id, which is why the dump key also carries the UID of the backup resource: a reused id can never name another backup's dump, and the finalizer never deletes one — pins the bucket it writes through (`status.bucketRef` and `status.bucketLocation` — the type, bucket, base path, and endpoint) and the configuration Zeebe runs (`status.workloadConfigHash`, the config hash of the live Zeebe pod template — the cluster's generation alone cannot tell a swapped database, because mutable referents enter the hash without bumping it), and records the object key `<basePath>/<namespace>/<cluster>/<id>/<uid>/camunda.dump`. The pinned hash is required unchanged before the Job is rendered and before the Zeebe backup is requested, and the referents must still name the database Zeebe runs before the Job is rendered; a rolled-out configuration change, or a referent edited under an unrolled Zeebe, fails the backup instead of reporting a split restore point.
3. It applies a Job in the backup's namespace — which is the cluster's — owned by the backup and labeled with its UID, so a Job found by name is adopted only when the UID matches: a `postgres:<version>` initContainer runs `pg_dump --format=custom` of the entire logical database into a scratch volume, then the `camunda-operator-cli upload` container streams the archive to the bucket. The CLI is its own image, shipped with every release; the manager renders the one it was started with (`--camunda-operator-cli-image`, defaulted by the chart's `manager.cliImage`). The pod runs under the cluster's ServiceAccount for workload identity, or with the bucket's static credentials — the source Secret when it lives in the cluster namespace, the local copy the cluster controller keeps otherwise. The pod settings come from the cluster's `spec.backup.dump`, or from this CR's `spec.dump` replacing them as a whole; the image always comes from the cluster block (or defaults to `postgres:<major>`), because the Job runs under the cluster's ServiceAccount and the executable is the cluster owner's choice.
4. When the Job succeeds, the operator records the result, releases the Job (a failed Job stays for inspection), verifies that the cluster's backup store is still the pinned bucket at the pinned location and that the cluster is still converged on its spec — a retarget or a rollout in between would send the Zeebe backup elsewhere and break the restore point — and requests one Zeebe backup through the cluster's management binding, without an id — the cluster generates it — recording it as `status.zeebeBackupId` and polling until it completes. Partitions register their parts asynchronously, so a backup the cluster does not report yet is polled for a short grace before it counts as failed.
5. A dependency that fails mid-run — a deleted reference, a broken binding, a management API that stops answering or rejects the call, a dump pod that cannot start (its image does not pull, its Secret is gone, its volume never binds) — holds the backup for a bounded grace, measured from when the dependency first failed and reset when it recovers, then fails it. The Job itself has a deadline too (`dump.activeDeadlineSeconds`, 24 hours by default). A `Running` backup either finishes or terminalizes; it never parks forever, because it would block every later backup of the cluster.
6. Deleting the CR deletes a still-running Job, waits until the Job and its pods are gone, then deletes the dump object in the pinned bucket — never the Zeebe backups, which belong to the continuous range that a point-in-time restore consumes. The delete runs where the identity lives: for a bucket on static credentials the operator deletes directly (it holds the Secret); for a bucket on workload identity — which binds the *cluster* ServiceAccount, not the operator's — a cleanup Job runs `camunda-operator-cli delete` in the backup's namespace under that ServiceAccount, on the same pod identity surface as the dump Job. A failed cleanup Job holds the deletion visibly, with an event naming the Job; inspect it, fix the cause, and delete the Job to retry. When the cluster, the pinned bucket, or its credentials are genuinely gone, or when the pinned bucket now points somewhere else (`status.bucketLocation` no longer matches), the finalizer leaves the object, releases, and an event records why.

## Serialization

The backups of one cluster run one at a time, across both backup kinds. The claim on a cluster is a `coordination.k8s.io` Lease in the namespace of the cluster, named `camunda-backup-<cluster>`, with the holder identity `<Kind>/<Name>/<UID>` of the backup. The API server creates the Lease atomically, so the first backup to claim wins under concurrent reconciles and across the two controllers. A backup that finds the Lease held waits as `Pending` with reason `BackupInProgress` and names the holder. Among the pending backups of one kind, a started sibling goes first, then the older one, then the smaller name; that order is a fairness pre-filter that decides who tries the claim first and never who holds the cluster, and a backup that already holds the claim goes first whatever it says. The claim is taken before the backup id is allocated. The Lease goes back when the holder reaches a terminal phase and when a deleted holder releases its finalizer, also for a holder that never got as far as an id. A Lease whose holder is gone, was recreated under the same name, or is terminal is taken over by the next claimant, so a crash between the claim and the release never blocks the cluster. A Lease that another actor wrote blocks until that actor removes it.

## API reference

```yaml
apiVersion: core.camunda.io/v1
kind: LogicalBackupRDBMS
metadata:
  name: my-cluster-backup
  namespace: my-cluster-ns
spec:
  # object. Required. Immutable. The CamundaCluster to back up. It lives in
  # this CR's namespace: the operator reads the cluster's Secrets, runs a Job
  # in its namespace, and calls its management API, so the reference stays
  # inside the RBAC boundary the CR itself lives in. Whoever may create a
  # backup in a namespace may back up the clusters of that namespace, and no
  # others.
  clusterRef:
    # string. Required. Name of the CamundaCluster, in this CR's namespace.
    name: my-cluster
  # object. Optional. Replaces the pod settings of the cluster's
  # spec.backup.dump block as a whole for this backup (resources, extraEnv,
  # extraEnvFrom, podLabels, podAnnotations, scheduling, scratchVolume,
  # activeDeadlineSeconds). The two never merge. The image that runs the dump
  # is not among them: the Job runs under the cluster's ServiceAccount, so
  # the executable is the cluster owner's choice and always comes from the
  # cluster block.
  dump: {}
```

The spec is immutable: a backup is one operation, and a retry is a new CR.

## Status

`status.phase` tracks the one-shot operation (`Pending | Running | Completed | Failed`); `status.step` is the resume marker (`Dumping | ZeebeBackup`). `status.backupId` identifies the dump object, `status.objectKey` its full key, `status.jobName` the Job while it exists (cleared once the dump is recorded and the Job released), and `status.zeebeBackupId` the cluster-generated Zeebe backup, requested at `status.zeebeBackupRequestedAt`. `status.bucketRef` pins the `ObjectStorageConfig` the dump was written through and `status.bucketLocation` where that contract pointed at the start (`status.bucketGeneration` records its generation for reference), so a later retarget of the cluster cannot orphan the object, and a retarget of the contract cannot make deletion hit a stranger's object. `status.workloadConfigHash` pins the configuration Zeebe ran at start. `status.firstFailedAt` is when a dependency of the running backup first stopped resolving; the mid-run grace is measured from it, and it clears on recovery. `status.failureMessage` states why a `Failed` backup failed. `status.storageSizes.zeebe` records the effective restore size of one broker volume, best effort.

| Type | Reason | Meaning |
| --- | --- | --- |
| `Ready` | `Progressing` | The backup runs, or waits on the management binding or on the cluster converging on its current spec. |
| `Ready` | `Completed` | The backup finished and is restorable. |
| `Ready` | `Failed` | The backup failed; the message names the failing step. |
| `Ready` | `ClusterSuspended` | The cluster is suspended; the backup waits. |
| `Ready` | `BackupInProgress` | Another backup of the cluster runs; this one waits. |
| `Ready` | `StorageTypeMismatch` | The cluster does not store its data in a relational database. |
| `Ready` | `InvalidReference` | A referenced object does not exist, the server has not been probed for its current spec, or the dump block sets a reserved environment variable. |
| `Ready` | `MissingSecret` | The database names no usable backup credentials. |
| `Ready` | `MissingCredentials` | The bucket's static credentials do not resolve. |
| `Ready` | `ConnectionFailed` | The management API of the cluster is not reachable, or rejects the call. |

## Relationships

- [CamundaCluster](camundacluster.md) — referenced via `clusterRef`; its management binding and `spec.backup.dump` drive the backup.
- [DatabaseConfig](databaseconfig.md) / [DatabaseServerConfig](databaseserverconfig.md) — locate the database and the credentials of the dump; the server's probed `status.serverVersion` picks the client tools.
- [ObjectStorageConfig](objectstorageconfig.md) — the bucket the dump is uploaded to, via the cluster's `backupStorageRef`.
