import { beforeEach, describe, expect, it, vi } from "vitest";
import { fakeWriter } from "../../../../../../test/fakes/writer";
import {
  applyBackupChange,
  applyClaimChange,
  previewClaimChange,
} from "./actions";

vi.mock(
  "@/lib/write/writer",
  () => import("../../../../../../test/fakes/writer"),
);

const change = (kind = "CamundaClusterClaim") => ({
  intent: "Scale the brokers of orders",
  apply: [
    {
      apiVersion: "fleet.camunda.io/v1alpha1",
      kind,
      metadata: { name: "orders", namespace: "team-a" },
      spec: { cluster: { zeebe: { replicas: 5 } } },
    },
  ],
  preconditions: { [`${kind}/team-a/orders` as const]: "41" },
});

beforeEach(() => fakeWriter.reset());

describe("applyClaimChange", () => {
  it("applyClaimChange writes the change through the writer", async () => {
    expect(await applyClaimChange(change())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([change()]);
  });

  it("applyClaimChange refuses a change of another kind and writes nothing", async () => {
    const result = await applyClaimChange(change("Environment"));
    expect(result.ok).toBe(false);
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("previewClaimChange", () => {
  it("previewClaimChange previews through the writer", async () => {
    const refused = {
      objects: [],
      errors: [{ code: "Invalid" as const, message: "CEL", path: "spec" }],
    };
    fakeWriter.previews = [refused];
    expect(await previewClaimChange(change())).toEqual(refused);
    expect(fakeWriter.previewed).toEqual([change()]);
  });

  it("previewClaimChange refuses a change of another kind and previews nothing", async () => {
    const result = await previewClaimChange(change("Environment"));
    expect(result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.previewed).toEqual([]);
  });
});

describe("applyBackupChange", () => {
  const orders = { namespace: "team-a", name: "orders" };
  const object = (kind: string, name: string, namespace = "team-a") => ({
    apiVersion: "core.camunda.io/v1",
    kind,
    metadata: { name, namespace },
  });
  const backup = (cluster = "orders") => ({
    intent: "Back up orders",
    apply: [
      {
        apiVersion: "core.camunda.io/v1",
        kind: "LogicalBackupElasticsearch",
        metadata: { name: "orders-manual-1", namespace: "team-a" },
        spec: { clusterRef: { name: cluster } },
      },
    ],
    create: ["LogicalBackupElasticsearch/team-a/orders-manual-1" as const],
    directOnly: true,
  });

  it("applyBackupChange writes a backup of the cluster straight to the cluster, also in Git mode", async () => {
    fakeWriter.reset("git");
    expect(await applyBackupChange(orders, backup())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([backup()]);
  });

  it.each(["LogicalBackupElasticsearch", "LogicalBackupRDBMS"])(
    "applyBackupChange refuses a name that a %s of the namespace has, and writes nothing",
    async (kind) => {
      fakeWriter.objects = [object(kind, "orders-manual-1")];
      expect(await applyBackupChange(orders, backup())).toEqual({
        ok: false,
        errors: [
          {
            code: "Stale",
            message: `The ${kind} orders-manual-1 exists already. Pick another name.`,
            ref: {
              apiVersion: "core.camunda.io/v1",
              kind,
              namespace: "team-a",
              name: "orders-manual-1",
            },
          },
        ],
      });
      expect(fakeWriter.applied).toEqual([]);
    },
  );

  const remove = (kind = "LogicalBackupRDBMS") => ({
    intent: "Delete backup orders-manual-1",
    apply: [],
    delete: [
      {
        apiVersion: "core.camunda.io/v1",
        kind,
        namespace: "team-a",
        name: "orders-manual-1",
      },
    ],
    directOnly: true,
  });
  const restore = (
    kind: string,
    name: string,
    phase: string,
    backup = "orders-manual-1",
    namespace = "team-a",
  ) => ({
    ...object(kind, name, namespace),
    spec: { backupRef: { name: backup }, targetClusterRef: { name: "orders" } },
    status: { phase },
  });

  it("applyBackupChange deletes a backup that no restore reads", async () => {
    fakeWriter.objects = [
      object("LogicalBackupRDBMS", "orders-manual-1"),
      restore("LogicalRestoreRDBMS", "done", "Completed"),
      restore("LogicalRestoreRDBMS", "failed", "Failed"),
      restore("LogicalRestoreRDBMS", "other", "Pending", "orders-manual-2"),
      restore("LogicalRestoreElasticsearch", "es", "Pending"),
      restore(
        "LogicalRestoreRDBMS",
        "elsewhere",
        "Pending",
        undefined,
        "team-b",
      ),
    ];
    expect(await applyBackupChange(orders, remove())).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([remove()]);
  });

  it.each([
    ["LogicalBackupElasticsearch", "LogicalRestoreElasticsearch"],
    ["LogicalBackupRDBMS", "LogicalRestoreRDBMS"],
  ])(
    "applyBackupChange refuses to delete a %s that a restore reads until the restore ends, and writes nothing",
    async (kind, restoreKind) => {
      fakeWriter.objects = [restore(restoreKind, "r1", "Pending")];
      expect(await applyBackupChange(orders, remove(kind))).toEqual({
        ok: false,
        errors: [
          {
            code: "Stale",
            message:
              "Restore r1 reads the backup orders-manual-1. Delete the backup after the restore ends.",
            ref: remove(kind).delete[0],
          },
        ],
      });
      expect(fakeWriter.applied).toEqual([]);
    },
  );

  it("applyBackupChange writes nothing when it cannot read the backups and restores", async () => {
    fakeWriter.readError = new Error("connect ECONNREFUSED");
    for (const change of [backup(), remove()]) {
      expect(await applyBackupChange(orders, change)).toEqual({
        ok: false,
        errors: [{ code: "Unavailable", message: "connect ECONNREFUSED" }],
      });
    }
    expect(fakeWriter.applied).toEqual([]);
  });

  it("applyBackupChange refuses a backup of another cluster and writes nothing", async () => {
    const result = await applyBackupChange(orders, backup("billing"));
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });

  it("applyBackupChange refuses a claim change and writes nothing", async () => {
    const result = await applyBackupChange(orders, change());
    expect(!result.ok && result.errors[0].code).toBe("Invalid");
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("applyBackupChange, restores", () => {
  const orders = { namespace: "team-a", name: "orders" };
  const core = "core.camunda.io/v1";
  const restore = (
    kind = "LogicalRestoreRDBMS",
    name = "orders-restore-1",
    backup = "orders-manual-1",
  ) => ({
    intent: "Restore orders",
    apply: [
      {
        apiVersion: core,
        kind,
        metadata: { name, namespace: "team-a" },
        spec:
          kind === "PointInTimeRestore"
            ? {
                clusterRef: { name: "orders" },
                timestamp: "2026-11-10T09:30:00Z",
              }
            : {
                backupRef: { name: backup },
                targetClusterRef: { name: "orders" },
              },
      },
    ],
    create: [`${kind}/team-a/${name}` as const],
    directOnly: true,
  });
  const backupOf = (
    kind: string,
    name: string,
    phase: string | undefined,
    cluster = "orders",
  ) => ({
    apiVersion: core,
    kind,
    metadata: { name, namespace: "team-a" },
    spec: { clusterRef: { name: cluster } },
    status: phase === undefined ? {} : { phase },
  });
  const restoreOf = (
    kind: string,
    name: string,
    phase: string,
    cluster = "orders",
    namespace = "team-a",
  ) => ({
    apiVersion: core,
    kind,
    metadata: { name, namespace },
    spec:
      kind === "PointInTimeRestore"
        ? { clusterRef: { name: cluster } }
        : { backupRef: { name: "b" }, targetClusterRef: { name: cluster } },
    status: { phase },
  });
  const refOf = (change: ReturnType<typeof restore>) => ({
    apiVersion: core,
    kind: change.apply[0].kind,
    namespace: "team-a",
    name: change.apply[0].metadata.name,
  });

  it.each([
    ["LogicalRestoreRDBMS", "LogicalBackupRDBMS"],
    ["LogicalRestoreElasticsearch", "LogicalBackupElasticsearch"],
  ])(
    "applyBackupChange writes a %s of a Completed backup of the cluster straight to the cluster, also in Git mode",
    async (kind, backupKind) => {
      fakeWriter.reset("git");
      fakeWriter.objects = [
        backupOf(backupKind, "orders-manual-1", "Completed"),
        restoreOf(kind, "done-1", "Completed"),
        restoreOf("PointInTimeRestore", "failed-1", "Failed"),
        restoreOf(kind, "billing-1", "Pending", "billing"),
        restoreOf(kind, "elsewhere-1", "Pending", "orders", "team-b"),
      ];
      expect(await applyBackupChange(orders, restore(kind))).toEqual({
        ok: true,
        mode: "direct",
      });
      expect(fakeWriter.applied).toEqual([restore(kind)]);
    },
  );

  it("applyBackupChange writes a point-in-time restore of the cluster", async () => {
    const change = restore("PointInTimeRestore", "orders-pitr-1");
    expect(await applyBackupChange(orders, change)).toEqual({
      ok: true,
      mode: "direct",
    });
    expect(fakeWriter.applied).toEqual([change]);
  });

  it.each([
    "LogicalRestoreElasticsearch",
    "LogicalRestoreRDBMS",
    "PointInTimeRestore",
  ])(
    "applyBackupChange refuses a restore name that a %s of the namespace has, and writes nothing",
    async (kind) => {
      fakeWriter.objects = [
        backupOf("LogicalBackupRDBMS", "orders-manual-1", "Completed"),
        restoreOf(kind, "orders-restore-1", "Completed", "billing"),
      ];
      expect(await applyBackupChange(orders, restore())).toEqual({
        ok: false,
        errors: [
          {
            code: "Stale",
            message: `The ${kind} orders-restore-1 exists already. Pick another name.`,
            ref: {
              apiVersion: core,
              kind,
              namespace: "team-a",
              name: "orders-restore-1",
            },
          },
        ],
      });
      expect(fakeWriter.applied).toEqual([]);
    },
  );

  it.each([
    "LogicalRestoreElasticsearch",
    "LogicalRestoreRDBMS",
    "PointInTimeRestore",
  ])(
    "applyBackupChange refuses a restore while a %s of the cluster has not ended, and writes nothing",
    async (kind) => {
      fakeWriter.objects = [
        backupOf("LogicalBackupRDBMS", "orders-manual-1", "Completed"),
        restoreOf(kind, "r0", "Pending"),
      ];
      for (const change of [
        restore(),
        restore("PointInTimeRestore", "orders-pitr-1"),
      ]) {
        expect(await applyBackupChange(orders, change)).toEqual({
          ok: false,
          errors: [
            {
              code: "Stale",
              message:
                "Restore r0 of this cluster has not ended. Start a new restore after it ends.",
              ref: refOf(change),
            },
          ],
        });
      }
      expect(fakeWriter.applied).toEqual([]);
    },
  );

  it.each([
    [[], "Backup orders-manual-1 does not exist."],
    [
      [backupOf("LogicalBackupElasticsearch", "orders-manual-1", "Completed")],
      "Backup orders-manual-1 does not exist.",
    ],
    [
      [backupOf("LogicalBackupRDBMS", "orders-manual-1", "Running")],
      "Backup orders-manual-1 is Running, and a restore reads only a Completed backup.",
    ],
    [
      [backupOf("LogicalBackupRDBMS", "orders-manual-1", undefined)],
      "Backup orders-manual-1 is Pending, and a restore reads only a Completed backup.",
    ],
    [
      [
        backupOf(
          "LogicalBackupRDBMS",
          "orders-manual-1",
          "Completed",
          "billing",
        ),
      ],
      "Backup orders-manual-1 is a backup of the cluster billing. A restore writes only into the cluster of its backup.",
    ],
  ])(
    "applyBackupChange refuses a logical restore of a backup that is missing, not Completed, or of another cluster: %#",
    async (objects, message) => {
      fakeWriter.objects = objects;
      expect(await applyBackupChange(orders, restore())).toEqual({
        ok: false,
        errors: [{ code: "Stale", message, ref: refOf(restore()) }],
      });
      expect(fakeWriter.applied).toEqual([]);
    },
  );

  it("applyBackupChange writes no restore when it cannot read the backups and restores", async () => {
    fakeWriter.readError = new Error("connect ECONNREFUSED");
    for (const change of [
      restore(),
      restore("PointInTimeRestore", "orders-pitr-1"),
    ]) {
      expect(await applyBackupChange(orders, change)).toEqual({
        ok: false,
        errors: [{ code: "Unavailable", message: "connect ECONNREFUSED" }],
      });
    }
    expect(fakeWriter.applied).toEqual([]);
  });
});
