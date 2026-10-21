import { describe, expect, it } from "vitest";
import type { Resource } from "@/lib/k8s/kinds";
import { BACKUP_SCHEDULE_LABEL } from "@/lib/labels";
import {
  type BackupObject,
  type RestoreObject,
  backupRowsOf,
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
