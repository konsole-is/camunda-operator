// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  applyBackupChange,
  applyClaimChange,
  previewClaimChange,
} from "@/app/fleet/clusters/[namespace]/[name]/actions";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import { FakeEventSource } from "../../../test/fakes/event-source";
import { cluster, NOW, slotContext } from "../../../test/fakes/slot-context";
import { fakeWriter } from "../../../test/fakes/writer";
import { backUpNow } from "./backup-actions";
import { backupContextOf, backupSlots } from "./backup-slots";
import type { BackupObject, BackupRow, RestoreObject } from "./backups";
import { CHANGES_OFF, ClusterEditProvider } from "./edit/cluster-edit";

vi.mock("@/lib/write/writer", () => import("../../../test/fakes/writer"));

const es = {
  kind: "elasticsearch" as const,
  bucket: { name: "backups", location: "s3://acme-backups" },
  repository: "camunda-backups",
  schedules: [],
};

function backup(
  kind: BackupObject["kind"],
  name: string,
  phase: string | undefined,
  namespace = "team-a",
  clusterName = "orders",
): BackupObject {
  return {
    kind,
    metadata: { namespace, name },
    spec: { clusterRef: { name: clusterName } },
    status: phase === undefined ? {} : { phase },
  } as unknown as BackupObject;
}

function restore(name: string, phase: string): RestoreObject {
  return {
    kind: "LogicalRestoreElasticsearch",
    metadata: { namespace: "team-a", name },
    spec: { targetClusterRef: { name: "orders" }, backupRef: { name: "b" } },
    status: { phase },
  } as unknown as RestoreObject;
}

beforeEach(() => {
  fakeWriter.reset();
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

describe("backupContextOf", () => {
  it("backupContextOf reads the storage, the running operation, and the backup names of the namespace", () => {
    const backups = [
      backup("LogicalBackupElasticsearch", "done-1", "Completed"),
      backup(
        "LogicalBackupRDBMS",
        "other-cluster-1",
        "Running",
        "team-a",
        "billing",
      ),
      backup("LogicalBackupElasticsearch", "elsewhere-1", "Running", "team-b"),
    ];
    const base = slotContext({ backupStorage: es, backups });
    expect(backupContextOf(base)).toEqual({
      namespace: "team-a",
      cluster: "orders",
      storage: es,
      backupNames: ["done-1", "other-cluster-1"],
      restoreNames: [],
      readingRestores: [],
      clusterVersion: "8.9.1",
      now: NOW,
    });

    const restoring = slotContext({
      cluster: cluster({ suspend: true }),
      backupStorage: es,
      backups,
      restores: [restore("r1", "Restoring")],
    });
    expect(backupContextOf(restoring)).toMatchObject({
      suspension: "requested",
      running: { what: "Restore", name: "r1" },
      restoring: "r1",
    });

    const both = slotContext({
      backupStorage: es,
      backups: [
        ...backups,
        backup("LogicalBackupElasticsearch", "now-1", "Running"),
      ],
      restores: [restore("r1", "Pending")],
    });
    expect(backupContextOf(both).running).toEqual({
      what: "Backup",
      name: "now-1",
    });
  });

  it("a restore that runs comes before a queued backup, so Back up now stays on while the restore suspends the cluster", () => {
    const context = slotContext({
      cluster: cluster({ suspend: true }),
      backupStorage: es,
      backups: [
        backup("LogicalBackupElasticsearch", "queued-1", "Pending"),
        backup("LogicalBackupElasticsearch", "queued-2", undefined),
      ],
      restores: [restore("r1", "RestoringSecondaryStorage")],
    });
    expect(backupContextOf(context).running).toEqual({
      what: "Restore",
      name: "r1",
    });
    expect(backUpNow(backupContextOf(context)).disabled).toBeUndefined();
  });

  it("a queued backup is the running operation when nothing else runs or holds the cluster", () => {
    const context = slotContext({
      backups: [backup("LogicalBackupElasticsearch", "queued-1", "Pending")],
    });
    expect(backupContextOf(context).running).toEqual({
      what: "Backup",
      name: "queued-1",
    });
  });
});

describe("backupContextOf, the facts of a restore", () => {
  it("reads the version of the merged spec on a cluster that reports none, and the retention window of the merged spec", () => {
    const suspended = {
      ...cluster({
        suspend: true,
        backup: { primaryStorage: { retention: { window: "P3D" } } },
      }),
      status: {},
    } as unknown as ReturnType<typeof cluster>;
    expect(backupContextOf(slotContext({ cluster: suspended }))).toMatchObject({
      clusterVersion: "8.9.1",
      retentionWindow: "P3D",
    });
  });

  it("names every restore of the namespace and the restore of the cluster that has not ended", () => {
    const of = (
      kind: string,
      name: string,
      phase: string,
      target = "orders",
      namespace = "team-a",
    ) =>
      ({
        kind,
        metadata: { namespace, name },
        spec:
          kind === "PointInTimeRestore"
            ? { clusterRef: { name: target }, timestamp: NOW }
            : { targetClusterRef: { name: target }, backupRef: { name: "b" } },
        status: { phase },
      }) as unknown as RestoreObject;
    const context = backupContextOf(
      slotContext({
        restores: [
          of("LogicalRestoreRDBMS", "done-1", "Completed"),
          of("PointInTimeRestore", "pitr-1", "RestoringDatabase"),
          of("LogicalRestoreElasticsearch", "billing-1", "Pending", "billing"),
          of(
            "LogicalRestoreRDBMS",
            "elsewhere-1",
            "Pending",
            "orders",
            "team-b",
          ),
        ],
      }),
    );
    expect(context.restoreNames).toEqual(["done-1", "pitr-1", "billing-1"]);
    expect(context.restoring).toBe("pitr-1");
    expect(
      backupContextOf(
        slotContext({ restores: [of("LogicalRestoreRDBMS", "d", "Failed")] }),
      ).restoring,
    ).toBeUndefined();
  });

  it("reads the suspension of the cluster, a Ready reason of the operator included", () => {
    const attached = {
      ...cluster(),
      status: {
        conditions: [{ type: "Ready", reason: "StorageAlreadyAttached" }],
      },
    } as unknown as ReturnType<typeof cluster>;
    expect(backupContextOf(slotContext({ cluster: attached })).suspension).toBe(
      "StorageAlreadyAttached",
    );
    expect(backupContextOf(slotContext()).suspension).toBeUndefined();
  });
});

describe("backupContextOf, the holders and readers", () => {
  function rdbmsRestore(name: string, phase: string, backupName: string) {
    return {
      kind: "LogicalRestoreRDBMS",
      metadata: { namespace: "team-a", name },
      spec: {
        targetClusterRef: { name: "orders" },
        backupRef: { name: backupName },
      },
      status: { phase },
    } as unknown as RestoreObject;
  }
  function pitr(name: string) {
    return {
      kind: "PointInTimeRestore",
      metadata: { namespace: "team-a", name },
      spec: { clusterRef: { name: "orders" }, timestamp: NOW },
      status: { phase: "Running" },
    } as unknown as RestoreObject;
  }

  it("lists the restores of both logical kinds that are not final, with the backup that each reads", () => {
    const context = slotContext({
      restores: [
        restore("r1", "Restoring"),
        rdbmsRestore("r2", "Pending", "dump-1"),
        rdbmsRestore("r3", "Completed", "dump-2"),
        pitr("p1"),
      ],
    });
    expect(backupContextOf(context).readingRestores).toEqual([
      { name: "r1", kind: "LogicalBackupElasticsearch", backup: "b" },
      { name: "r2", kind: "LogicalBackupRDBMS", backup: "dump-1" },
    ]);
  });

  it("an Elasticsearch backup that ended as ResumeFailed holds the cluster when nothing runs", () => {
    const held = {
      ...backup("LogicalBackupElasticsearch", "held-1", "Failed"),
      status: { phase: "Failed", terminalReason: "ResumeFailed" },
    } as unknown as BackupObject;
    expect(backupContextOf(slotContext({ backups: [held] })).running).toEqual({
      what: "Backup",
      name: "held-1",
      resumeFailed: true,
    });
    const running = backup("LogicalBackupElasticsearch", "now-1", "Running");
    expect(
      backupContextOf(slotContext({ backups: [held, running] })).running,
    ).toEqual({ what: "Backup", name: "now-1" });
  });

  it("a ResumeFailed holder comes before a queued backup, so the wait note names the holder", () => {
    const held = {
      ...backup("LogicalBackupElasticsearch", "held-1", "Failed"),
      status: { phase: "Failed", terminalReason: "ResumeFailed" },
    } as unknown as BackupObject;
    const queued = backup(
      "LogicalBackupElasticsearch",
      "a-queued-1",
      "Pending",
    );
    expect(
      backupContextOf(slotContext({ backups: [held, queued] })).running,
    ).toEqual({ what: "Backup", name: "held-1", resumeFailed: true });
  });

  it("a page without a write mode turns the changes off with the reason of the edit host", () => {
    expect(
      backupContextOf(slotContext({ writeMode: undefined })).changesOff,
    ).toBe(CHANGES_OFF);
    expect(backupContextOf(slotContext()).changesOff).toBeUndefined();
  });
});

describe("backupSlots", () => {
  it("the row slot of backupSlots mounts Restore and Delete on a completed backup row and the header slot keeps Back up now", async () => {
    const context = slotContext({ backupStorage: es });
    const slots = backupSlots(applyBackupChange);
    const row = {
      kind: "LogicalBackupElasticsearch",
      name: "nightly-1",
      origin: "Manual",
      phase: "Completed",
      version: "8.9.1",
    } as BackupRow;
    render(
      <QueryClientProvider client={new QueryClient()}>
        <GitPushesProvider>
          <ClusterEditProvider
            context={context}
            actions={{ preview: previewClaimChange, apply: applyClaimChange }}
          >
            {slots["cluster.backups.actions"](context)}
            {slots["cluster.backups.row"](context, row)}
          </ClusterEditProvider>
        </GitPushesProvider>
      </QueryClientProvider>,
    );
    expect(screen.getByRole("button", { name: "Back up now" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Restore" })).toHaveProperty(
      "disabled",
      false,
    );
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
    await userEvent.click(
      screen.getByRole("button", { name: "Delete backup" }),
    );
    await waitFor(() =>
      expect(fakeWriter.applied.map((c) => c.intent)).toEqual([
        "Delete backup nightly-1",
      ]),
    );
    expect((await screen.findByRole("status")).textContent).toContain(
      "Backup nightly-1 deleted.",
    );
  });
});
