import { describe, expect, it } from "vitest";
import {
  type BackupContext,
  backUpNow,
  backupChange,
  backupKindOf,
  backupNameErrors,
  clusterVersionOf,
  deleteBackupChange,
  deleteBlock,
  deleteLines,
  freeName,
  manualBackupName,
  pointInTime,
  pointInTimeChange,
  pointInTimeError,
  pointInTimeWindow,
  restoreBlock,
  restoreChange,
  restoreKindOf,
  restoreNameOf,
  retentionMs,
  suspensionOf,
  timestampOf,
  waitNote,
} from "./backup-actions";
import type { BackupRow, BackupStorage, RestoreInterval } from "./backups";

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
  backupNames: [],
  readingRestores: [],
  restoreNames: [],
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

  it("a cluster is suspended by spec.suspend or by a suspension hold annotation, and a hold comes first", () => {
    const hold = { "suspension-hold.camunda.io/restore": "r1" };
    expect(suspensionOf({ spec: { suspend: true } })).toBe("requested");
    expect(
      suspensionOf({
        spec: { suspend: false },
        metadata: { annotations: hold },
      }),
    ).toBe("held");
    expect(
      suspensionOf({
        spec: { suspend: true },
        metadata: { annotations: hold },
      }),
    ).toBe("held");
    expect(
      suspensionOf({
        spec: {},
        metadata: { annotations: { "camunda.io/other": "x" } },
      }),
    ).toBeUndefined();
    expect(suspensionOf({})).toBeUndefined();
  });

  it("a cluster is suspended, as CamundaCluster.Suspended of api/v1 says, while Ready reports StorageAlreadyAttached or WaitingForHandover", () => {
    const ready = (type: string, reason: string) => ({
      status: { conditions: [{ type, reason }] },
    });
    expect(suspensionOf(ready("Ready", "StorageAlreadyAttached"))).toBe(
      "StorageAlreadyAttached",
    );
    expect(suspensionOf(ready("Ready", "WaitingForHandover"))).toBe(
      "WaitingForHandover",
    );
    expect(
      suspensionOf(ready("Ready", "VersionDowngradeRefused")),
    ).toBeUndefined();
    expect(
      suspensionOf(ready("Progressing", "StorageAlreadyAttached")),
    ).toBeUndefined();
  });

  it.each([
    [
      "requested",
      "The cluster is suspended. A backup waits until it runs again, so resume the cluster first.",
    ],
    [
      "held",
      "A suspension hold keeps the cluster suspended, and a backup waits until the cluster runs again. A failed restore keeps its hold until you delete that restore.",
    ],
    [
      "StorageAlreadyAttached",
      "Another cluster holds the secondary storage of this cluster, so the operator keeps this cluster suspended, and a backup waits until it runs again. Give this cluster storage of its own, or delete the cluster that holds the storage.",
    ],
    [
      "WaitingForHandover",
      "Something else still writes the secondary storage of this cluster, so the operator keeps this cluster suspended, and a backup waits until it runs again. Wait, or remove what the Ready condition of the cluster names.",
    ],
  ] as const)(
    "back up now on a cluster suspended as %s gives the remedy of that case",
    (suspension, reason) => {
      expect(backUpNow(context({ suspension }))).toEqual({ disabled: reason });
    },
  );

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
    expect(backUpNow(context({ suspension: "requested" }))).toEqual(suspended);
    expect(
      backUpNow(
        context({
          suspension: "requested",
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
          suspension: "held",
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
          metadata: {
            name: "orders-manual-20261110-143005",
            namespace: "team-a",
          },
          spec: { clusterRef: { name: "orders" } },
        },
      ],
      create: [
        "LogicalBackupElasticsearch/team-a/orders-manual-20261110-143005",
      ],
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
  const OFF = "Changes are off: the fleet manager could not read its settings.";

  it("back up now and delete are off with the reason of the page when the page cannot write", () => {
    expect(backUpNow(context({ changesOff: OFF }))).toEqual({ disabled: OFF });
    expect(deleteBlock(context({ changesOff: OFF }), row())).toBe(OFF);
  });

  it("delete is off while a restore that is not final reads the backup", () => {
    const reading = context({
      readingRestores: [
        {
          name: "r1",
          kind: "LogicalBackupElasticsearch",
          backup: "orders-manual-1",
        },
      ],
    });
    expect(deleteBlock(reading, row())).toBe(
      "Restore r1 reads this backup. Delete the backup after the restore ends.",
    );
    expect(deleteBlock(reading, row({ name: "orders-manual-2" }))).toBe(
      undefined,
    );
    expect(deleteBlock(reading, row({ kind: "LogicalBackupRDBMS" }))).toBe(
      undefined,
    );
    expect(deleteBlock(context(), row())).toBeUndefined();
  });

  it("the wait note names a backup that ended as ResumeFailed and holds the cluster", () => {
    const held = {
      what: "Backup" as const,
      name: "b1",
      resumeFailed: true as const,
    };
    expect(waitNote(held, "backup")).toBe(
      "Backup b1 ended as ResumeFailed and still holds the cluster. The new backup waits until you delete that backup.",
    );
    expect(backUpNow(context({ running: held })).waits).toBe(
      waitNote(held, "backup"),
    );
  });

  it("the delete confirm of a backup that runs says that the run stops", () => {
    expect(deleteLines(row({ phase: "Pending" })).goes).toEqual([
      "Backup orders-manual-1, with its Elasticsearch snapshots and its Zeebe partition backup in the bucket.",
      "The run of the backup. If it paused exporting on the cluster, exporting resumes.",
    ]);
    expect(
      deleteLines(row({ kind: "LogicalBackupRDBMS", phase: "Running" })).goes,
    ).toEqual([
      "Backup orders-manual-1, with its database dump in the bucket.",
      "The run of the backup, with its dump Job.",
    ]);
    expect(deleteLines(row({ phase: "Failed" })).goes).toHaveLength(1);
  });
});

describe("restore rules", () => {
  const NOW = "2026-11-10T14:30:05Z";
  const ago = (days: number) =>
    new Date(Date.parse(NOW) - days * 86_400_000).toISOString();
  const OFF = "Changes are off: the fleet manager could not read its settings.";

  it("the restore kind follows the backup kind", () => {
    expect(restoreKindOf("LogicalBackupElasticsearch")).toBe(
      "LogicalRestoreElasticsearch",
    );
    expect(restoreKindOf("LogicalBackupRDBMS")).toBe("LogicalRestoreRDBMS");
  });

  it("the cluster version is the reported one, else the version of the merged spec", () => {
    expect(
      clusterVersionOf(
        { status: { management: { version: "8.9.2" } } },
        { version: "8.9.4" },
      ),
    ).toBe("8.9.2");
    expect(clusterVersionOf({ status: {} }, { version: "8.9.4" })).toBe(
      "8.9.4",
    );
    expect(
      clusterVersionOf({ status: { management: { version: "" } } }, {}),
    ).toBeUndefined();
  });

  it("an Elasticsearch backup restores when it recorded an x.y.z version", () => {
    const c = context({ clusterVersion: "8.10.1" });
    expect(restoreBlock(c, row({ version: "8.9.4" }))).toBeUndefined();
    for (const version of ["-", "8.9"]) {
      expect(restoreBlock(c, row({ version }))).toBe(
        "Backup orders-manual-1 recorded no Camunda version, so it cannot be restored.",
      );
    }
  });

  it("an RDBMS backup restores into the same minor or one minor newer", () => {
    const dump = row({ kind: "LogicalBackupRDBMS", version: "8.9.4" });
    const at = (clusterVersion?: string) =>
      restoreBlock(context({ storage: rdbms, clusterVersion }), dump);
    for (const version of ["8.9.0", "8.9.9", "8.10.2"]) {
      expect(at(version)).toBeUndefined();
    }
    for (const version of ["8.8.9", "8.11.0", "9.9.4"]) {
      expect(at(version)).toBe(
        `Backup orders-manual-1 has Camunda 8.9.4. It restores only into Camunda 8.9 or 8.10, and the cluster has ${version}.`,
      );
    }
    expect(at(undefined)).toBe(
      "The cluster has no Camunda version yet, so the version rule cannot be checked.",
    );
    expect(at("latest")).toBe(
      "The cluster has no Camunda version yet, so the version rule cannot be checked.",
    );
  });

  it("restore is off when the page cannot write and while a restore of the cluster has not ended", () => {
    expect(restoreBlock(context({ changesOff: OFF }), row())).toBe(OFF);
    expect(restoreBlock(context({ restoring: "r1" }), row())).toBe(
      "Restore r1 of this cluster has not ended. Start a new restore after it ends.",
    );
  });

  it("a restore name skips the names that are taken", () => {
    const base = "orders-restore-20261110-143005";
    expect(freeName(base, [])).toBe(base);
    expect(freeName(base, [base])).toBe(`${base}-2`);
    expect(freeName(base, [base, `${base}-2`, `${base}-4`])).toBe(`${base}-3`);
    expect(restoreNameOf(context({ restoreNames: [base] }), "restore")).toBe(
      `${base}-2`,
    );
    expect(restoreNameOf(context(), "pitr")).toBe(
      "orders-pitr-20261110-143005",
    );
  });

  it("a restore writes the exact logical restore of the backup kind, straight to the cluster", () => {
    expect(restoreChange(context(), row(), "r1")).toEqual({
      intent: "Restore orders from backup orders-manual-1",
      apply: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "LogicalRestoreElasticsearch",
          metadata: { name: "r1", namespace: "team-a" },
          spec: {
            backupRef: { name: "orders-manual-1" },
            targetClusterRef: { name: "orders" },
          },
        },
      ],
      create: ["LogicalRestoreElasticsearch/team-a/r1"],
      directOnly: true,
    });
    expect(
      restoreChange(context(), row({ kind: "LogicalBackupRDBMS" }), "r2"),
    ).toEqual({
      intent: "Restore orders from backup orders-manual-1",
      apply: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "LogicalRestoreRDBMS",
          metadata: { name: "r2", namespace: "team-a" },
          spec: {
            backupRef: { name: "orders-manual-1" },
            targetClusterRef: { name: "orders" },
          },
        },
      ],
      create: ["LogicalRestoreRDBMS/team-a/r2"],
      directOnly: true,
    });
  });

  it("the retention window reads days and time and refuses weeks", () => {
    expect(retentionMs("P7D")).toBe(604_800_000);
    expect(retentionMs("PT12H")).toBe(43_200_000);
    expect(retentionMs("P1DT1H30M15.5S")).toBe(
      86_400_000 + 3_600_000 + 1_800_000 + 15_500,
    );
    for (const window of ["P1W", "P1M", "P", "PT", "7d", ""]) {
      expect(retentionMs(window)).toBeUndefined();
    }
  });

  it("the point-in-time window is the archive cut to the Zeebe window and to now", () => {
    const archive: RestoreInterval[] = [
      { from: ago(10), to: ago(8) },
      { from: ago(9), to: ago(6) },
      { from: ago(5), to: ago(2), failing: true },
      { from: ago(1) },
    ];
    expect(pointInTimeWindow(archive, "P7D", NOW)).toEqual({
      intervals: [
        { from: ago(7), to: ago(6) },
        { from: ago(5), to: ago(2), failing: true },
        { from: ago(1) },
      ],
    });
    expect(pointInTimeWindow(archive, undefined, NOW)).toEqual(
      pointInTimeWindow(archive, "P7D", NOW),
    );
    expect(pointInTimeWindow([{ from: ago(1) }], "PT12H", NOW)).toEqual({
      intervals: [{ from: ago(0.5) }],
    });
  });

  it("the point-in-time window is unavailable without a restore point, outside the Zeebe window, and with a window the fleet manager cannot read", () => {
    expect(pointInTimeWindow([], "P7D", NOW)).toEqual({
      unavailable: "The archive has no restore point yet.",
    });
    expect(
      pointInTimeWindow([{ from: ago(10), to: ago(8) }], "P7D", NOW),
    ).toEqual({
      unavailable:
        "No point is inside both the archive window and the P7D in which Zeebe keeps its backups.",
    });
    expect(pointInTimeWindow([{ from: ago(1) }], "P1W", NOW)).toEqual({
      unavailable:
        "The cluster keeps its Zeebe backups for P1W, which the fleet manager cannot read, so it cannot offer a point.",
    });
  });

  it("a point must be a time, in the past, and inside an interval", () => {
    const intervals: RestoreInterval[] = [
      { from: ago(5), to: ago(2) },
      { from: ago(1) },
    ];
    const error = (at: string | undefined) =>
      pointInTimeError(at, intervals, NOW);
    expect(error(undefined)).toBe("Enter a date and time.");
    expect(error(ago(5))).toBeUndefined();
    expect(error(ago(3))).toBeUndefined();
    expect(error(NOW)).toBeUndefined();
    const outside = "Choose a point inside the restore window.";
    expect(error(ago(2))).toBe(outside);
    expect(error(ago(1.5))).toBe(outside);
    expect(error(ago(6))).toBe(outside);
    expect(error("2026-11-10T14:30:06Z")).toBe("Choose a point in the past.");
  });

  it("the time of the field is read as UTC to the second", () => {
    expect(timestampOf("2026-11-10T09:30")).toBe("2026-11-10T09:30:00Z");
    expect(timestampOf("2026-11-10T09:30:15")).toBe("2026-11-10T09:30:15Z");
    expect(timestampOf("")).toBeUndefined();
    expect(timestampOf("tomorrow")).toBeUndefined();
  });

  it("a point-in-time restore writes the exact PointInTimeRestore, straight to the cluster", () => {
    expect(pointInTimeChange(context(), "p1", "2026-11-10T09:30:00Z")).toEqual({
      intent: "Restore orders to 2026-11-10T09:30:00Z",
      apply: [
        {
          apiVersion: "core.camunda.io/v1",
          kind: "PointInTimeRestore",
          metadata: { name: "p1", namespace: "team-a" },
          spec: {
            clusterRef: { name: "orders" },
            timestamp: "2026-11-10T09:30:00Z",
          },
        },
      ],
      create: ["PointInTimeRestore/team-a/p1"],
      directOnly: true,
    });
  });

  const archived = (window?: RestoreInterval[]): BackupStorage => ({
    ...rdbms,
    database: {
      archive: {
        server: "orders-db",
        state: { tone: "ok", label: "Archiving" },
        ...(window ? { window } : {}),
      },
    },
  });

  it("point in time is offered only on PostgreSQL with recovery by the operator", () => {
    expect(pointInTime(context())).toBeUndefined();
    expect(pointInTime(context({ storage: rdbms }))).toBeUndefined();
    expect(pointInTime(context({ storage: archived() }))).toBeUndefined();
    expect(
      pointInTime(context({ storage: archived([{ from: ago(1) }]) })),
    ).toEqual({ intervals: [{ from: ago(1) }] });
  });

  it("point in time is off when the page cannot write, while a restore has not ended, and without a point", () => {
    const storage = archived([{ from: ago(1) }]);
    expect(pointInTime(context({ storage, changesOff: OFF }))).toEqual({
      disabled: OFF,
    });
    expect(pointInTime(context({ storage, restoring: "r1" }))).toEqual({
      disabled:
        "Restore r1 of this cluster has not ended. Start a new restore after it ends.",
    });
    expect(
      pointInTime(context({ storage: archived([]), retentionWindow: "P7D" })),
    ).toEqual({ disabled: "The archive has no restore point yet." });
  });
});
