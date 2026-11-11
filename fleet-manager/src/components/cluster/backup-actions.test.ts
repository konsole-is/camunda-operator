import { describe, expect, it } from "vitest";
import {
  type BackupContext,
  backUpNow,
  backupChange,
  backupKindOf,
  backupNameErrors,
  clusterSuspended,
  deleteBackupChange,
  deleteLines,
  manualBackupName,
  waitNote,
} from "./backup-actions";
import type { BackupRow, BackupStorage } from "./backups";

const es: BackupStorage = {
  kind: "elasticsearch",
  bucket: { name: "backups", location: "s3://acme-backups" },
  repository: "camunda-backups",
  schedules: [],
};
const rdbms: BackupStorage = {
  kind: "rdbms",
  bucket: { name: "backups", location: "s3://acme-backups" },
  schedules: [],
};

const context = (over: Partial<BackupContext> = {}): BackupContext => ({
  namespace: "team-a",
  cluster: "orders",
  storage: es,
  suspended: false,
  backupNames: [],
  now: "2026-11-10T14:30:05Z",
  ...over,
});

const row = (over: Partial<BackupRow> = {}): BackupRow => ({
  kind: "LogicalBackupElasticsearch",
  name: "orders-manual-1",
  origin: "Manual",
  started: "2026-11-10 14:30 UTC",
  finished: "2026-11-10 14:42 UTC",
  duration: "12m",
  phase: "Completed",
  tone: "ok",
  step: "-",
  version: "8.9.3",
  details: [],
  ...over,
});

describe("backup rules", () => {
  it("the backup kind follows the secondary storage, and none or unknown storage has no kind", () => {
    expect(backupKindOf(es)).toBe("LogicalBackupElasticsearch");
    expect(backupKindOf(rdbms)).toBe("LogicalBackupRDBMS");
    expect(backupKindOf({ kind: "none", schedules: [] })).toBeUndefined();
    expect(
      backupKindOf({
        kind: "unknown",
        missing: "SecondaryStorageConfig orders-es",
        bucket: { name: "backups" },
        schedules: [],
      }),
    ).toBeUndefined();
  });

  it("the manual name is the cluster, manual, and the UTC date and time", () => {
    expect(manualBackupName("orders", "2026-11-10T14:30:05Z")).toBe(
      "orders-manual-20261110-143005",
    );
    expect(manualBackupName("orders", "2026-11-10T23:59:59.999+02:00")).toBe(
      "orders-manual-20261110-215959",
    );
  });

  it("a name must be set, a DNS subdomain, and not taken in the namespace", () => {
    expect(backupNameErrors("", [])).toEqual(["Enter a name"]);
    expect(backupNameErrors("Orders_1", [])).toEqual([
      "Use only lowercase letters, digits, '-', and '.'. Start and end with a letter or a digit.",
    ]);
    expect(backupNameErrors("orders-1", ["orders-1"])).toEqual([
      "A backup named orders-1 exists. Choose another name.",
    ]);
    expect(backupNameErrors("orders-1", ["orders-2"])).toEqual([]);
  });

  it("a cluster is suspended by spec.suspend or by a suspension hold annotation", () => {
    expect(clusterSuspended({ spec: { suspend: true } })).toBe(true);
    expect(
      clusterSuspended({
        spec: { suspend: false },
        metadata: {
          annotations: { "suspension-hold.camunda.io/restore": "r1" },
        },
      }),
    ).toBe(true);
    expect(
      clusterSuspended({
        spec: {},
        metadata: { annotations: { "camunda.io/other": "x" } },
      }),
    ).toBe(false);
    expect(clusterSuspended({})).toBe(false);
  });

  it("back up now is disabled without backup storage, with unknown storage, and on a suspended cluster", () => {
    expect(
      backUpNow(context({ storage: { kind: "none", schedules: [] } })),
    ).toEqual({ disabled: "This cluster has no backup storage." });
    expect(
      backUpNow(
        context({
          storage: {
            kind: "unknown",
            missing: "SecondaryStorageConfig orders-es",
            bucket: { name: "backups" },
            schedules: [],
          },
        }),
      ),
    ).toEqual({
      disabled:
        "SecondaryStorageConfig orders-es not found, so the kind of backup is not known.",
    });
    const suspended = {
      disabled:
        "The cluster is suspended. A backup waits until it runs again, so resume the cluster first.",
    };
    expect(backUpNow(context({ suspended: true }))).toEqual(suspended);
    expect(
      backUpNow(
        context({
          suspended: true,
          running: { what: "Backup", name: "nightly-1" },
        }),
      ),
    ).toEqual(suspended);
  });

  it("back up now stays enabled with a wait note while a backup or a restore runs", () => {
    expect(backUpNow(context())).toEqual({
      kind: "LogicalBackupElasticsearch",
    });
    expect(
      backUpNow(context({ running: { what: "Backup", name: "nightly-1" } })),
    ).toEqual({
      kind: "LogicalBackupElasticsearch",
      waits: "Backup nightly-1 is running. The new backup waits until it ends.",
    });
    expect(
      backUpNow(
        context({
          storage: rdbms,
          suspended: true,
          running: { what: "Restore", name: "r1" },
        }),
      ),
    ).toEqual({
      kind: "LogicalBackupRDBMS",
      waits: "Restore r1 is running. The new backup waits until it ends.",
    });
  });

  it("the wait note names the running operation, and nothing runs without one", () => {
    expect(waitNote({ what: "Restore", name: "r1" }, "backup")).toBe(
      "Restore r1 is running. The new backup waits until it ends.",
    );
    expect(waitNote({ what: "Backup", name: "b1" }, "restore")).toBe(
      "Backup b1 is running. The new restore waits until it ends.",
    );
    expect(waitNote(undefined, "backup")).toBeUndefined();
  });

  it("back up now writes one new backup of the kind of the storage, with the cluster reference, straight to the cluster", () => {
    expect(
      backupChange(
        context(),
        "LogicalBackupElasticsearch",
        "orders-manual-20261110-143005",
      ),
    ).toEqual({
      intent: "Back up orders",
      apply: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "LogicalBackupElasticsearch",
          metadata: { name: "orders-manual-20261110-143005", namespace: "team-a" },
          spec: { clusterRef: { name: "orders" } },
        },
      ],
      create: ["LogicalBackupElasticsearch/team-a/orders-manual-20261110-143005"],
      directOnly: true,
    });
    expect(
      backupChange(context({ storage: rdbms }), "LogicalBackupRDBMS", "b1"),
    ).toEqual({
      intent: "Back up orders",
      apply: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "LogicalBackupRDBMS",
          metadata: { name: "b1", namespace: "team-a" },
          spec: { clusterRef: { name: "orders" } },
        },
      ],
      create: ["LogicalBackupRDBMS/team-a/b1"],
      directOnly: true,
    });
  });

  it("delete writes one delete of the backup, straight to the cluster", () => {
    expect(
      deleteBackupChange("team-a", row({ kind: "LogicalBackupRDBMS" })),
    ).toEqual({
      intent: "Delete backup orders-manual-1",
      apply: [],
      delete: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "LogicalBackupRDBMS",
          namespace: "team-a",
          name: "orders-manual-1",
        },
      ],
      directOnly: true,
    });
  });

  it("the delete confirm names the artifacts of each kind, and a scheduled backup says what its schedule keeps", () => {
    expect(deleteLines(row())).toEqual({
      goes: [
        "Backup orders-manual-1, with its Elasticsearch snapshots and its Zeebe partition backup in the bucket.",
      ],
      stays: [],
    });
    expect(deleteLines(row({ kind: "LogicalBackupRDBMS" }))).toEqual({
      goes: ["Backup orders-manual-1, with its database dump in the bucket."],
      stays: [
        "The Zeebe backups. Zeebe removes them after the retention window of the cluster.",
      ],
    });
    expect(
      deleteLines(row({ kind: "LogicalBackupRDBMS", origin: "Scheduled" })),
    ).toEqual({
      goes: ["Backup orders-manual-1, with its database dump in the bucket."],
      stays: [
        "The Zeebe backups. Zeebe removes them after the retention window of the cluster.",
        "The other backups of the schedule. The schedule deletes the oldest of them beyond the counts it keeps.",
      ],
    });
  });
});
