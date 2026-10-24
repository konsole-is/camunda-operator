import { describe, expect, it } from "vitest";
import {
  type BackupLike,
  type RestoreLike,
  clusterBackups,
  clusterRestores,
  runningBackup,
  runningRestore,
} from "./backups";

const cluster = { metadata: { namespace: "team-a", name: "orders" } };

function backup(
  name: string,
  phase: string | undefined,
  over: { kind?: string; namespace?: string; cluster?: string } = {},
): BackupLike {
  return {
    kind: over.kind ?? "LogicalBackupRDBMS",
    metadata: { namespace: over.namespace ?? "team-a", name },
    spec: { clusterRef: { name: over.cluster ?? "orders" } },
    status: phase === undefined ? {} : { phase },
  };
}

describe("runningBackup", () => {
  it("returns no backup when the last backup of the cluster finished", () => {
    expect(
      runningBackup(cluster, [
        backup("nightly-1", "Failed"),
        backup("nightly-2", "Completed"),
      ]),
    ).toBeUndefined();
  });

  it("returns a Running LogicalBackupRDBMS of the cluster", () => {
    expect(runningBackup(cluster, [backup("nightly-1", "Running")])).toEqual({
      kind: "LogicalBackupRDBMS",
      namespace: "team-a",
      name: "nightly-1",
    });
  });

  it("returns a running LogicalBackupElasticsearch of the cluster", () => {
    expect(
      runningBackup(cluster, [
        backup("es-1", "Running", { kind: "LogicalBackupElasticsearch" }),
      ]),
    ).toEqual({
      kind: "LogicalBackupElasticsearch",
      namespace: "team-a",
      name: "es-1",
    });
  });

  it.each(["Pending", undefined])("counts the phase %s as running", (phase) => {
    expect(runningBackup(cluster, [backup("b", phase)])?.name).toBe("b");
  });

  it.each(["Completed", "Failed"])("skips a %s backup", (phase) => {
    expect(runningBackup(cluster, [backup("b", phase)])).toBeUndefined();
  });

  it("skips a running backup of another cluster in the namespace", () => {
    expect(
      runningBackup(cluster, [backup("b", "Running", { cluster: "payments" })]),
    ).toBeUndefined();
  });

  it("skips a running backup of a cluster of the same name in another namespace", () => {
    expect(
      runningBackup(cluster, [backup("b", "Running", { namespace: "team-b" })]),
    ).toBeUndefined();
  });

  it("skips an object of another kind", () => {
    expect(
      runningBackup(cluster, [
        backup("b", "Running", { kind: "BackupSchedule" }),
      ]),
    ).toBeUndefined();
  });

  it("returns the first running backup by name", () => {
    expect(
      runningBackup(cluster, [backup("b", "Running"), backup("a", "Running")])
        ?.name,
    ).toBe("a");
  });

  it("has no result without backups", () => {
    expect(runningBackup(cluster, [])).toBeUndefined();
  });
});

function restore(
  kind: string,
  phase: string,
  name = "r",
  target = "orders",
): RestoreLike {
  const ref = { name: target };
  return {
    kind,
    metadata: { namespace: "team-a", name },
    spec:
      kind === "PointInTimeRestore"
        ? { clusterRef: ref }
        : { targetClusterRef: ref },
    status: { phase },
  };
}

describe("runningRestore", () => {
  it.each([
    "LogicalRestoreElasticsearch",
    "LogicalRestoreRDBMS",
    "PointInTimeRestore",
  ])("finds a running %s of the cluster by its cluster reference", (kind) => {
    expect(
      runningRestore(cluster, [restore(kind, "RestoringPrimaryStorage")]),
    ).toEqual({ kind, namespace: "team-a", name: "r" });
  });

  it.each(["Completed", "Failed"])("skips a %s restore", (phase) => {
    expect(
      runningRestore(cluster, [restore("LogicalRestoreRDBMS", phase)]),
    ).toBeUndefined();
  });

  it("skips a running restore of another cluster", () => {
    expect(
      runningRestore(cluster, [
        restore("PointInTimeRestore", "Pending", "r", "payments"),
      ]),
    ).toBeUndefined();
  });

  it("returns the first running restore by name", () => {
    expect(
      runningRestore(cluster, [
        restore("LogicalRestoreRDBMS", "Pending", "b"),
        restore("PointInTimeRestore", "Pending", "a"),
      ])?.name,
    ).toBe("a");
  });
});

describe("clusterBackups", () => {
  it("keeps the backups of the cluster in their order, in every phase", () => {
    const kept = [
      backup("b", "Completed"),
      backup("a", "Failed", { kind: "LogicalBackupElasticsearch" }),
      backup("c", undefined),
    ];
    expect(clusterBackups(cluster, kept)).toEqual(kept);
  });

  it("drops another cluster, another namespace, and another kind", () => {
    expect(
      clusterBackups(cluster, [
        backup("x", "Completed", { cluster: "payments" }),
        backup("y", "Completed", { namespace: "team-b" }),
        backup("z", "Completed", { kind: "PointInTimeRestore" }),
      ]),
    ).toEqual([]);
  });
});

describe("clusterRestores", () => {
  it("matches a logical restore by targetClusterRef and a PointInTimeRestore by clusterRef", () => {
    const logical = {
      kind: "LogicalRestoreRDBMS",
      metadata: { namespace: "team-a", name: "r1" },
      spec: {
        targetClusterRef: { name: "orders" },
        clusterRef: { name: "payments" },
      },
      status: { phase: "Completed" },
    };
    const pitr = {
      kind: "PointInTimeRestore",
      metadata: { namespace: "team-a", name: "p1" },
      spec: {
        clusterRef: { name: "orders" },
        targetClusterRef: { name: "payments" },
      },
      status: {},
    };
    expect(clusterRestores(cluster, [logical, pitr])).toEqual([logical, pitr]);
  });

  it("drops a restore of another cluster, namespace, or kind", () => {
    expect(
      clusterRestores(cluster, [
        restore("LogicalRestoreRDBMS", "Completed", "a", "payments"),
        {
          ...restore("PointInTimeRestore", "Failed", "b"),
          metadata: { namespace: "team-b", name: "b" },
        },
        restore("LogicalBackupRDBMS", "Completed", "c"),
      ]),
    ).toEqual([]);
  });
});
