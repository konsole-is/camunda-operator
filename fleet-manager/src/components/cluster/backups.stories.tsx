import type { Story } from "@/gallery/registry";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { BACKUP_SCHEDULE_LABEL } from "@/lib/labels";
import {
  type BackupObject,
  type BackupStorageInput,
  type RestoreObject,
  backupRowsOf,
  backupStorageOf,
  restoreRowsOf,
} from "./backups";
import { BackupsTab } from "./backups-tab";

const now = "2026-10-18T12:00:00Z";

function obj<K extends KindName>(
  kind: K,
  name: string,
  spec: object,
  status: object = {},
  meta: object = {},
): Resource<K> {
  return {
    kind,
    metadata: { namespace: "team-a", name, ...meta },
    spec,
    status,
  } as unknown as Resource<K>;
}

const cluster = (backupRepository?: string) =>
  obj(
    "CamundaCluster",
    "orders",
    {},
    backupRepository ? { management: { backupRepository } } : {},
  );

const nightly = obj(
  "BackupSchedule",
  "nightly",
  {
    clusterRef: { name: "orders" },
    schedule: "0 2 * * *",
    retained: { completed: 7, failed: 3 },
  },
  { lastBackupName: "nightly-20261018" },
);

const s3Backups = obj("ObjectStorageConfig", "backups", {
  type: "S3",
  s3: { bucketName: "acme-backups", basePath: "orders", region: "eu-west-1" },
});

function input(over: Partial<BackupStorageInput>): BackupStorageInput {
  return {
    cluster: cluster(),
    spec: {},
    storage: [],
    objectStorage: [],
    databaseConfigs: [],
    databaseServerConfigs: [],
    databaseServers: [],
    databaseServerPresets: [],
    schedules: [],
    now,
    ...over,
  };
}

const esBackups = [
  obj(
    "LogicalBackupElasticsearch",
    "nightly-20261018",
    { clusterRef: { name: "orders" } },
    {
      phase: "Completed",
      step: "ResumeExporting",
      backupId: 1760752800,
      repository: "camunda-backups",
      partitionsCount: 3,
      version: "8.9.23",
      storageSizes: { zeebe: "10Gi", elasticsearch: 21474836480 },
      completionTime: "2026-10-18T02:14:22Z",
    },
    {
      creationTimestamp: "2026-10-18T02:00:00Z",
      labels: { [BACKUP_SCHEDULE_LABEL]: "nightly" },
    },
  ),
  obj(
    "LogicalBackupElasticsearch",
    "before-upgrade",
    { clusterRef: { name: "orders" } },
    {
      phase: "Failed",
      step: "SnapshotRecords",
      backupId: 1760770000,
      repository: "camunda-backups",
      failureMessage:
        "Elasticsearch refused the snapshot: the repository is read-only",
    },
    { creationTimestamp: "2026-10-18T06:46:40Z" },
  ),
  obj(
    "LogicalBackupElasticsearch",
    "manual-now",
    { clusterRef: { name: "orders" } },
    { phase: "Running", step: "BackupRuntime", version: "8.9.23" },
    { creationTimestamp: "2026-10-18T11:48:00Z" },
  ),
] as BackupObject[];

const esRestores = [
  obj(
    "LogicalRestoreElasticsearch",
    "restore-staging",
    {
      targetClusterRef: { name: "orders" },
      backupRef: { name: "nightly-20261018" },
    },
    { phase: "RestoringPrimaryStorage" },
    { creationTimestamp: "2026-10-18T11:50:00Z" },
  ),
] as RestoreObject[];

const pgInput = input({
  spec: { storageRef: "orders-db-storage", backupStorageRef: "backups" },
  storage: [
    obj("SecondaryStorageConfig", "orders-db-storage", {
      type: "rdbms",
      rdbms: { databaseConfigRef: "orders-db" },
    }),
  ],
  objectStorage: [
    s3Backups,
    obj("ObjectStorageConfig", "wal", {
      type: "GCS",
      gcs: { bucketName: "acme-wal", basePath: "orders-pg" },
    }),
  ],
  databaseConfigs: [
    obj("DatabaseConfig", "orders-db", { serverRef: "orders-pg" }),
  ],
  databaseServerConfigs: [
    obj("DatabaseServerConfig", "orders-pg", {
      host: "orders-pg-rw.team-a.svc",
      port: 5432,
      pitr: { enabled: true, recovery: "operator", retentionPeriodDays: 30 },
    }),
  ],
  databaseServers: [
    obj(
      "DatabaseServer",
      "orders-pg",
      {
        databaseServerConfig: "orders-pg",
        archive: { objectStorageRef: "wal" },
      },
      {
        conditions: [
          { type: "ArchiveReady", status: "True", reason: "Archiving" },
        ],
        archive: {
          history: [{ serverName: "orders-pg", from: "2026-09-20T15:20:40Z" }],
          reachableFrom: "2026-09-25T09:58:00Z",
        },
      },
    ),
  ],
  schedules: [nightly],
});

const pgBackups = [
  obj(
    "LogicalBackupRDBMS",
    "nightly-20261018",
    { clusterRef: { name: "orders" } },
    {
      phase: "Completed",
      step: "ZeebeBackup",
      backupId: 1760752800,
      objectKey: "backups/team-a/orders/1760752800/camunda.dump",
      zeebeBackupId: 1760752800,
      version: "8.9.23",
      storageSizes: { zeebe: "10Gi" },
      completionTime: "2026-10-18T02:06:10Z",
    },
    {
      creationTimestamp: "2026-10-18T02:00:00Z",
      labels: { [BACKUP_SCHEDULE_LABEL]: "nightly" },
    },
  ),
] as BackupObject[];

const pgRestores = [
  obj(
    "PointInTimeRestore",
    "undo-bad-deploy",
    { clusterRef: { name: "orders" }, timestamp: "2026-10-17T15:30:00Z" },
    { phase: "Completed", completionTime: "2026-10-17T16:02:00Z" },
    { creationTimestamp: "2026-10-17T15:41:00Z" },
  ),
] as RestoreObject[];

export const backupsStories: Story[] = [
  {
    component: "BackupsTab",
    name: "Elasticsearch with backups and a running restore",
    render: () => (
      <BackupsTab
        storage={backupStorageOf(
          input({
            cluster: cluster("camunda-backups"),
            spec: { storageRef: "orders-es", backupStorageRef: "backups" },
            storage: [
              obj("SecondaryStorageConfig", "orders-es", {
                type: "elasticsearch",
                elasticsearch: { endpoint: "http://orders-es:9200" },
              }),
            ],
            objectStorage: [s3Backups],
            schedules: [nightly],
          }),
        )}
        backups={backupRowsOf(cluster(), esBackups, now)}
        restores={restoreRowsOf(cluster(), esRestores)}
      />
    ),
  },
  {
    component: "BackupsTab",
    name: "PostgreSQL with point-in-time restore",
    render: () => (
      <BackupsTab
        storage={backupStorageOf(pgInput)}
        backups={backupRowsOf(cluster(), pgBackups, now)}
        restores={restoreRowsOf(cluster(), pgRestores)}
      />
    ),
  },
  {
    component: "BackupsTab",
    name: "No backup storage",
    render: () => (
      <BackupsTab
        storage={backupStorageOf(input({}))}
        backups={[]}
        restores={[]}
      />
    ),
  },
];
