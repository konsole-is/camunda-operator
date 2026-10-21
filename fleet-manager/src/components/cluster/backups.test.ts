import { describe, expect, it } from "vitest";
import type { KindName, Resource } from "@/lib/k8s/kinds";
import { BACKUP_SCHEDULE_LABEL } from "@/lib/labels";
import {
  type BackupObject,
  type BackupStorageInput,
  type RestoreObject,
  backupRowsOf,
  backupStorageOf,
  bucketLocation,
  dateTime,
  duration,
  restoreRowsOf,
} from "./backups";

const now = "2026-10-18T12:00:00Z";
const cluster = { metadata: { namespace: "team-a", name: "orders" } };

type Over = {
  created?: string;
  labels?: Record<string, string>;
  cluster?: string;
};

function backupOf(
  kind: BackupObject["kind"],
  name: string,
  status: object,
  over: Over,
): BackupObject {
  return {
    kind,
    metadata: {
      namespace: "team-a",
      name,
      creationTimestamp: over.created,
      labels: over.labels,
    },
    spec: { clusterRef: { name: over.cluster ?? "orders" } },
    status,
  } as unknown as BackupObject;
}

function esBackup(name: string, status: object, over: Over = {}) {
  return backupOf("LogicalBackupElasticsearch", name, status, over);
}

function rdbmsBackup(name: string, status: object, over: Over = {}) {
  return backupOf("LogicalBackupRDBMS", name, status, over);
}

function restore(
  kind: RestoreObject["kind"],
  name: string,
  phase: string | undefined,
  over: Over & { backup?: string; timestamp?: string } = {},
): RestoreObject {
  const ref = { name: over.cluster ?? "orders" };
  return {
    kind,
    metadata: {
      namespace: "team-a",
      name,
      creationTimestamp: over.created,
    },
    spec:
      kind === "PointInTimeRestore"
        ? { clusterRef: ref, timestamp: over.timestamp }
        : { targetClusterRef: ref, backupRef: { name: over.backup ?? "b" } },
    status: phase === undefined ? {} : { phase },
  } as unknown as RestoreObject;
}

describe("duration", () => {
  it.each([
    ["2026-10-18T11:59:15Z", "2026-10-18T12:00:00Z", "45s"],
    ["2026-10-18T11:47:55Z", "2026-10-18T12:00:00Z", "12m 5s"],
    ["2026-10-18T08:40:00Z", "2026-10-18T12:00:00Z", "3h 20m"],
    ["2026-10-18T09:00:00Z", "2026-10-18T12:00:00Z", "3h"],
    ["2026-10-16T08:00:00Z", "2026-10-18T12:00:00Z", "2d 4h"],
    ["2026-10-18T12:00:10Z", "2026-10-18T12:00:00Z", "0s"],
    [undefined, "2026-10-18T12:00:00Z", "-"],
    ["2026-10-18T12:00:00Z", undefined, "-"],
    ["not a time", "2026-10-18T12:00:00Z", "-"],
  ])("duration(%s, %s) is %s", (from, to, want) => {
    expect(duration(from, to)).toBe(want);
  });
});

describe("dateTime", () => {
  it("writes the UTC date and minute", () => {
    expect(dateTime("2026-10-18T11:58:31Z")).toBe("2026-10-18 11:58 UTC");
  });

  it.each([undefined, "", "not a time"])("writes - for %j", (iso) => {
    expect(dateTime(iso)).toBe("-");
  });
});

describe("backupRowsOf", () => {
  it("a completed Elasticsearch backup of a schedule gives its row and details", () => {
    const [row] = backupRowsOf(
      cluster,
      [
        esBackup(
          "nightly-1",
          {
            phase: "Completed",
            step: "ResumeExporting",
            backupId: 1760781600,
            repository: "camunda-backups",
            partitionsCount: 3,
            version: "8.9.23",
            storageSizes: { zeebe: "10Gi", elasticsearch: 21474836480 },
            completionTime: "2026-10-18T02:14:00Z",
          },
          {
            created: "2026-10-18T02:00:00Z",
            labels: { [BACKUP_SCHEDULE_LABEL]: "nightly" },
          },
        ),
      ],
      now,
    );
    expect(row).toMatchObject({
      kind: "LogicalBackupElasticsearch",
      name: "nightly-1",
      origin: "Scheduled",
      phase: "Completed",
      tone: "ok",
      started: "2026-10-18 02:00 UTC",
      finished: "2026-10-18 02:14 UTC",
      duration: "14m",
      step: "ResumeExporting",
      version: "8.9.23",
    });
    expect(row.details).toEqual([
      { label: "Backup ID", value: "1760781600" },
      { label: "Repository", value: "camunda-backups" },
      { label: "Partitions", value: "3" },
      { label: "Restore sizes", value: "Zeebe 10Gi, Elasticsearch 20 GiB" },
    ]);
  });

  it("a failed RDBMS backup by hand shows its object key, Zeebe backup ID, and failure", () => {
    const [row] = backupRowsOf(
      cluster,
      [
        rdbmsBackup(
          "manual-1",
          {
            phase: "Failed",
            step: "ZeebeBackup",
            backupId: 7,
            objectKey: "b/team-a/orders/7/uid/camunda.dump",
            zeebeBackupId: 0,
            failureMessage: "Zeebe refused the backup",
          },
          { created: "2026-10-18T10:00:00Z" },
        ),
      ],
      now,
    );
    expect(row).toMatchObject({
      origin: "Manual",
      phase: "Failed",
      tone: "bad",
      duration: "-",
    });
    expect(row.details).toEqual([
      { label: "Backup ID", value: "7" },
      { label: "Object key", value: "b/team-a/orders/7/uid/camunda.dump" },
      { label: "Zeebe backup ID", value: "0" },
      { label: "Failure", value: "Zeebe refused the backup" },
    ]);
  });

  it("does not show a backup ID or partitions of 0", () => {
    const [row] = backupRowsOf(
      cluster,
      [esBackup("b", { backupId: 0, partitionsCount: 0 })],
      now,
    );
    expect(row.details).toEqual([]);
  });

  it("an empty schedule label is a backup by hand", () => {
    const [row] = backupRowsOf(
      cluster,
      [esBackup("b", {}, { labels: { [BACKUP_SCHEDULE_LABEL]: "" } })],
      now,
    );
    expect(row.origin).toBe("Manual");
  });

  it("a running backup counts its duration to now", () => {
    const [row] = backupRowsOf(
      cluster,
      [
        esBackup(
          "b",
          { phase: "Running" },
          { created: "2026-10-18T11:30:00Z" },
        ),
      ],
      now,
    );
    expect(row).toMatchObject({ duration: "30m", finished: "-", tone: "warn" });
  });

  it("a backup without times shows -", () => {
    const [row] = backupRowsOf(cluster, [esBackup("b", {})], now);
    expect(row).toMatchObject({
      phase: "Pending",
      tone: "warn",
      started: "-",
      finished: "-",
      duration: "-",
      step: "-",
      version: "-",
    });
  });

  it("sorts the backups newest first, then by name, and drops another cluster", () => {
    const rows = backupRowsOf(
      cluster,
      [
        esBackup("old", {}, { created: "2026-10-16T02:00:00Z" }),
        rdbmsBackup("b-new", {}, { created: "2026-10-18T02:00:00Z" }),
        esBackup("a-new", {}, { created: "2026-10-18T02:00:00Z" }),
        esBackup(
          "other",
          {},
          {
            created: "2026-10-18T03:00:00Z",
            cluster: "payments",
          },
        ),
      ],
      now,
    );
    expect(rows.map((r) => r.name)).toEqual(["a-new", "b-new", "old"]);
  });
});

describe("restoreRowsOf", () => {
  it("keeps every running restore and the five newest finished ones", () => {
    const finished = [1, 2, 3, 4, 5, 6, 7].map((day) =>
      restore(
        "LogicalRestoreRDBMS",
        `done-${day}`,
        day % 2 ? "Completed" : "Failed",
        {
          created: `2026-10-0${day}T00:00:00Z`,
        },
      ),
    );
    const rows = restoreRowsOf(cluster, [
      ...finished,
      restore("LogicalRestoreRDBMS", "running", "RestoringPrimaryStorage", {
        created: "2026-10-01T00:00:00Z",
      }),
      restore("LogicalRestoreRDBMS", "other", "Pending", {
        created: "2026-10-09T00:00:00Z",
        cluster: "payments",
      }),
    ]);
    expect(rows.map((r) => r.name)).toEqual([
      "running",
      "done-7",
      "done-6",
      "done-5",
      "done-4",
      "done-3",
    ]);
    expect(rows[0]).toMatchObject({ running: true, tone: "warn" });
    expect(rows[1]).toMatchObject({ running: false, tone: "ok" });
    expect(rows[2]).toMatchObject({ running: false, tone: "bad" });
  });

  it("a PointInTimeRestore names its point, a logical restore its backup", () => {
    const rows = restoreRowsOf(cluster, [
      restore("PointInTimeRestore", "pitr", "RestoringDatabase", {
        created: "2026-10-18T10:00:00Z",
        timestamp: "2026-10-18T09:30:00Z",
      }),
      restore("LogicalRestoreElasticsearch", "logical", "Pending", {
        created: "2026-10-18T09:00:00Z",
        backup: "nightly-1",
      }),
    ]);
    expect(rows).toMatchObject([
      {
        kind: "PointInTimeRestore",
        name: "pitr",
        label: "Point-in-time restore",
        source: "To 2026-10-18 09:30 UTC",
        started: "2026-10-18 10:00 UTC",
        phase: "RestoringDatabase",
      },
      {
        kind: "LogicalRestoreElasticsearch",
        name: "logical",
        label: "Restore",
        source: "Backup nightly-1",
        phase: "Pending",
      },
    ]);
  });

  it("a restore without a phase is Pending, and a failed one carries its message", () => {
    const failed = restore("LogicalRestoreRDBMS", "f", "Failed");
    (failed.status as { failureMessage?: string }).failureMessage =
      "The backup is gone";
    const rows = restoreRowsOf(cluster, [
      restore("LogicalRestoreRDBMS", "p", undefined),
      failed,
    ]);
    expect(rows).toMatchObject([
      { name: "p", phase: "Pending", running: true },
      { name: "f", failureMessage: "The backup is gone" },
    ]);
  });
});

function obj<K extends KindName>(
  kind: K,
  name: string,
  spec: object,
  status?: object,
  meta: object = {},
): Resource<K> {
  return {
    kind,
    metadata: { namespace: "team-a", name, ...meta },
    spec,
    status,
  } as unknown as Resource<K>;
}

function osc(name: string, spec: object) {
  return obj("ObjectStorageConfig", name, spec);
}

function schedule(name: string, clusterName: string, status?: object) {
  return obj(
    "BackupSchedule",
    name,
    {
      clusterRef: { name: clusterName },
      schedule: "0 2 * * *",
      retained: { completed: 7, failed: 3 },
    },
    status,
  );
}

function input(
  over: Partial<Omit<BackupStorageInput, "cluster">> & {
    backupRepository?: string;
  } = {},
): BackupStorageInput {
  const { backupRepository, ...rest } = over;
  return {
    cluster: obj(
      "CamundaCluster",
      "orders",
      {},
      backupRepository ? { management: { backupRepository } } : {},
    ),
    spec: {},
    storage: [],
    objectStorage: [],
    databaseConfigs: [],
    databaseServerConfigs: [],
    databaseServers: [],
    databaseServerPresets: [],
    schedules: [],
    now,
    ...rest,
  };
}

const esStorage = obj("SecondaryStorageConfig", "orders-es", {
  type: "elasticsearch",
  elasticsearch: {
    endpoint: "http://es:9200",
    snapshotRepository: "from-config",
  },
});

const s3Backups = osc("backups", {
  type: "S3",
  s3: { bucketName: "acme-backups", basePath: "orders", region: "eu-west-1" },
});

function rdbmsChain(
  over: {
    databaseConfig?: boolean;
    serverConfig?: boolean;
    server?: object | false;
    serverStatus?: object;
    owned?: boolean;
  } = {},
): BackupStorageInput {
  return input({
    spec: { storageRef: "orders-db-storage", backupStorageRef: "backups" },
    storage: [
      obj("SecondaryStorageConfig", "orders-db-storage", {
        type: "rdbms",
        rdbms: { databaseConfigRef: "orders-db" },
      }),
    ],
    objectStorage: [
      s3Backups,
      osc("wal", { type: "GCS", gcs: { bucketName: "acme-wal" } }),
    ],
    databaseConfigs:
      over.databaseConfig === false
        ? []
        : [obj("DatabaseConfig", "orders-db", { serverRef: "orders-pg" })],
    databaseServerConfigs:
      over.serverConfig === false
        ? []
        : [
            obj(
              "DatabaseServerConfig",
              "orders-pg",
              {
                host: "orders-pg-rw.team-a.svc",
                port: 5432,
                pitr: {
                  enabled: true,
                  recovery: "operator",
                  retentionPeriodDays: 30,
                },
              },
              undefined,
              over.owned === false ? {} : { ownerReferences: [publisher] },
            ),
          ],
    databaseServers:
      over.server === false
        ? []
        : [
            obj(
              "DatabaseServer",
              "orders-pg",
              over.server ?? {
                databaseServerConfig: "orders-pg",
                archive: { objectStorageRef: "wal" },
              },
              over.serverStatus ?? {
                conditions: [
                  { type: "ArchiveReady", status: "True", reason: "Archiving" },
                ],
                archive: {
                  history: [
                    {
                      serverName: "orders-pg",
                      from: "2026-09-01T00:00:00Z",
                      to: "2026-09-20T15:20:40Z",
                    },
                    { serverName: "orders-pg", from: "2026-09-20T15:20:40Z" },
                  ],
                  reachableFrom: "2026-09-25T09:58:00Z",
                },
              },
              { uid: "uid-orders-pg" },
            ),
          ],
  });
}

const publisher = {
  apiVersion: "core.camunda.io/v1",
  kind: "DatabaseServer",
  name: "orders-pg",
  uid: "uid-orders-pg",
  controller: true,
};

describe("backupStorageOf", () => {
  it("is none without backupStorageRef, and still lists the schedules of the cluster", () => {
    expect(
      backupStorageOf(input({ schedules: [schedule("nightly", "orders")] })),
    ).toEqual({
      kind: "none",
      schedules: [
        { name: "nightly", cron: "0 2 * * *", completed: 7, failed: 3 },
      ],
    });
  });

  it("an Elasticsearch cluster shows the repository of the cluster status and the S3 bucket", () => {
    expect(
      backupStorageOf(
        input({
          spec: { storageRef: "orders-es", backupStorageRef: "backups" },
          backupRepository: "camunda-backups",
          storage: [esStorage],
          objectStorage: [s3Backups],
        }),
      ),
    ).toEqual({
      kind: "elasticsearch",
      repository: "camunda-backups",
      bucket: {
        name: "backups",
        location: "s3://acme-backups/orders (region eu-west-1)",
      },
      schedules: [],
    });
  });

  it("falls back to the snapshotRepository of the SecondaryStorageConfig", () => {
    expect(
      backupStorageOf(
        input({
          spec: { storageRef: "orders-es", backupStorageRef: "backups" },
          storage: [esStorage],
          objectStorage: [s3Backups],
        }),
      ),
    ).toMatchObject({ kind: "elasticsearch", repository: "from-config" });
  });

  it("names a missing bucket", () => {
    const storage = backupStorageOf(
      input({
        spec: { storageRef: "orders-es", backupStorageRef: "gone" },
        storage: [esStorage],
        objectStorage: [s3Backups],
      }),
    );
    expect(storage.kind !== "none" && storage.bucket).toEqual({ name: "gone" });
  });

  it("is unknown when the SecondaryStorageConfig is missing", () => {
    expect(
      backupStorageOf(
        input({
          spec: { storageRef: "gone", backupStorageRef: "backups" },
          objectStorage: [s3Backups],
        }),
      ),
    ).toEqual({
      kind: "unknown",
      missing: "SecondaryStorageConfig gone",
      bucket: {
        name: "backups",
        location: "s3://acme-backups/orders (region eu-west-1)",
      },
      schedules: [],
    });
  });

  it("a PostgreSQL cluster follows the chain to its server, archive, bucket, and window", () => {
    expect(backupStorageOf(rdbmsChain())).toMatchObject({
      kind: "rdbms",
      database: {
        serverConfig: "orders-pg",
        endpoint: "orders-pg-rw.team-a.svc:5432",
        pitr: { enabled: true, recovery: "operator", retentionDays: 30 },
        archive: {
          server: "orders-pg",
          state: { tone: "ok", label: "Archiving" },
          bucket: { name: "wal", location: "gs://acme-wal" },
          window: { from: "2026-09-25T09:58:00Z" },
        },
      },
    });
  });

  it("starts the window at the open record when it is later than reachableFrom", () => {
    const storage = backupStorageOf(
      rdbmsChain({
        serverStatus: {
          archive: {
            history: [
              { serverName: "orders-pg", from: "2026-10-01T00:00:00Z" },
            ],
            reachableFrom: "2026-09-25T09:58:00Z",
          },
        },
      }),
    );
    expect(
      storage.kind === "rdbms" && storage.database?.archive?.window,
    ).toEqual({ from: "2026-10-01T00:00:00Z" });
  });

  it("ends the window at unverifiedFrom while uploads fail", () => {
    const storage = backupStorageOf(
      rdbmsChain({
        serverStatus: {
          conditions: [
            {
              type: "ArchiveReady",
              status: "False",
              reason: "ArchiveFailing",
              message: "The upload of WAL failed",
            },
          ],
          archive: {
            history: [
              {
                serverName: "orders-pg",
                from: "2026-09-20T15:20:40Z",
                unverifiedFrom: "2026-10-18T10:00:00Z",
              },
            ],
            reachableFrom: "2026-09-25T09:58:00Z",
          },
        },
      }),
    );
    expect(storage.kind === "rdbms" && storage.database?.archive).toMatchObject(
      {
        state: {
          tone: "bad",
          label: "ArchiveFailing",
          message: "The upload of WAL failed",
        },
        window: { from: "2026-09-25T09:58:00Z", to: "2026-10-18T10:00:00Z" },
      },
    );
  });

  it("has no window without an open record", () => {
    const storage = backupStorageOf(
      rdbmsChain({
        serverStatus: {
          archive: {
            history: [
              {
                serverName: "orders-pg",
                from: "2026-09-01T00:00:00Z",
                to: "2026-09-20T00:00:00Z",
              },
            ],
          },
        },
      }),
    );
    expect(
      storage.kind === "rdbms" && storage.database?.archive?.window,
    ).toBeUndefined();
  });

  it("starts the window at the retention cutoff when the open record is older", () => {
    const storage = backupStorageOf(
      rdbmsChain({
        server: {
          databaseServerConfig: "orders-pg",
          archive: { objectStorageRef: "wal", retentionPeriodDays: 7 },
        },
        serverStatus: {
          archive: {
            history: [
              { serverName: "orders-pg", from: "2026-01-01T00:00:00Z" },
            ],
          },
        },
      }),
    );
    expect(
      storage.kind === "rdbms" && storage.database?.archive?.window,
    ).toEqual({ from: "2026-10-11T12:00:00.000Z" });
  });

  it("takes the archive of the DatabaseServerPreset when the server sets none", () => {
    const data = rdbmsChain({
      server: { databaseServerConfig: "orders-pg", presetRef: "pg" },
    });
    data.databaseServerPresets = [
      {
        kind: "DatabaseServerPreset",
        metadata: { name: "pg" },
        spec: { server: { archive: { objectStorageRef: "wal" } } },
      } as unknown as Resource<"DatabaseServerPreset">,
    ];
    const storage = backupStorageOf(data);
    expect(storage.kind === "rdbms" && storage.database?.archive).toMatchObject(
      {
        state: { tone: "ok", label: "Archiving" },
        bucket: { name: "wal", location: "gs://acme-wal" },
        window: { from: "2026-09-25T09:58:00Z" },
      },
    );
  });

  it("an archive without ArchiveReady is Waiting", () => {
    const storage = backupStorageOf(rdbmsChain({ serverStatus: {} }));
    expect(
      storage.kind === "rdbms" && storage.database?.archive?.state,
    ).toEqual({ tone: "warn", label: "Waiting" });
  });

  it("a server without spec.archive shows the archive as Off and no window", () => {
    const storage = backupStorageOf(
      rdbmsChain({
        server: { databaseServerConfig: "orders-pg" },
        serverStatus: {},
      }),
    );
    expect(storage.kind === "rdbms" && storage.database?.archive).toEqual({
      server: "orders-pg",
      state: { tone: "muted", label: "Off" },
    });
  });

  it("ignores a DatabaseServer that requests the config but does not control it", () => {
    const data = rdbmsChain();
    data.databaseServers = [
      obj(
        "DatabaseServer",
        "intruder",
        {
          databaseServerConfig: "orders-pg",
          archive: { objectStorageRef: "backups" },
        },
        {},
        { uid: "uid-intruder" },
      ),
      ...data.databaseServers,
    ];
    const storage = backupStorageOf(data);
    expect(storage.kind === "rdbms" && storage.database?.archive).toMatchObject(
      {
        server: "orders-pg",
        bucket: { name: "wal" },
      },
    );
  });

  it("names a missing DatabaseServerPreset in place of the archive state", () => {
    const storage = backupStorageOf(
      rdbmsChain({
        server: { databaseServerConfig: "orders-pg", presetRef: "pg" },
      }),
    );
    expect(storage.kind === "rdbms" && storage.database?.archive).toEqual({
      server: "orders-pg",
      state: { tone: "bad", label: "DatabaseServerPreset pg not found" },
    });
  });

  it("has no window when the retention cutoff is after unverifiedFrom", () => {
    const storage = backupStorageOf(
      rdbmsChain({
        server: {
          databaseServerConfig: "orders-pg",
          archive: { objectStorageRef: "wal", retentionPeriodDays: 7 },
        },
        serverStatus: {
          archive: {
            history: [
              {
                serverName: "orders-pg",
                from: "2026-09-01T00:00:00Z",
                unverifiedFrom: "2026-10-01T00:00:00Z",
              },
            ],
          },
        },
      }),
    );
    expect(storage.kind === "rdbms" && storage.database?.archive).toBeDefined();
    expect(
      storage.kind === "rdbms" && storage.database?.archive?.window,
    ).toBeUndefined();
  });

  it("an external server shows the pitr of its DatabaseServerConfig and no archive", () => {
    const storage = backupStorageOf(rdbmsChain({ owned: false }));
    expect(storage.kind === "rdbms" && storage.database).toEqual({
      serverConfig: "orders-pg",
      endpoint: "orders-pg-rw.team-a.svc:5432",
      pitr: { enabled: true, recovery: "operator", retentionDays: 30 },
    });
  });

  it.each([
    ["DatabaseConfig orders-db", rdbmsChain({ databaseConfig: false })],
    ["DatabaseServerConfig orders-pg", rdbmsChain({ serverConfig: false })],
  ])(
    "names the first missing link of the PostgreSQL chain: %s",
    (missing, data) => {
      const storage = backupStorageOf(data);
      expect(storage.kind === "rdbms" && storage.database).toEqual({ missing });
    },
  );

  it("lists only the schedules of the cluster, by name", () => {
    expect(
      backupStorageOf(
        input({
          schedules: [
            schedule("weekly", "orders", { lastBackupName: "weekly-1" }),
            schedule("payments-nightly", "payments"),
            schedule("nightly", "orders"),
          ],
        }),
      ).schedules.map((s) => [s.name, s.lastBackup]),
    ).toEqual([
      ["nightly", undefined],
      ["weekly", "weekly-1"],
    ]);
  });
});

describe("bucketLocation", () => {
  it.each([
    [{ type: "S3", s3: { bucketName: "b" } }, "s3://b"],
    [
      { type: "S3", s3: { bucketName: "b", basePath: "p", region: "r" } },
      "s3://b/p (region r)",
    ],
    [{ type: "GCS", gcs: { bucketName: "b", basePath: "p" } }, "gs://b/p"],
    [
      { type: "AzureBlob", azureBlob: { accountName: "acct", container: "c" } },
      "azure://acct/c",
    ],
    [
      {
        type: "AzureBlob",
        azureBlob: { accountName: "acct", container: "c", basePath: "p" },
      },
      "azure://acct/c/p",
    ],
  ])("bucketLocation of %j is %s", (spec, want) => {
    expect(bucketLocation(osc("x", spec))).toBe(want);
  });
});
