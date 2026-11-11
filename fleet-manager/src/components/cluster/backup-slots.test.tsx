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
import { backupContextOf, backupSlots } from "./backup-slots";
import type { BackupObject, BackupRow, RestoreObject } from "./backups";
import { ClusterEditProvider } from "./edit/cluster-edit";

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
      suspended: false,
      backupNames: ["done-1", "other-cluster-1"],
      now: NOW,
    });

    const restoring = slotContext({
      cluster: cluster({ suspend: true }),
      backupStorage: es,
      backups,
      restores: [restore("r1", "Restoring")],
    });
    expect(backupContextOf(restoring)).toMatchObject({
      suspended: true,
      running: { what: "Restore", name: "r1" },
    });

    const both = slotContext({
      backupStorage: es,
      backups: [
        ...backups,
        backup("LogicalBackupElasticsearch", "now-1", undefined),
      ],
      restores: [restore("r1", "Restoring")],
    });
    expect(backupContextOf(both).running).toEqual({
      what: "Backup",
      name: "now-1",
    });
  });
});

describe("backupSlots", () => {
  it("the row slot of backupSlots mounts Delete on a backup row and the header slot mounts Back up now", async () => {
    const context = slotContext({ backupStorage: es });
    const slots = backupSlots(applyBackupChange);
    const row = {
      kind: "LogicalBackupElasticsearch",
      name: "nightly-1",
      origin: "Manual",
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
