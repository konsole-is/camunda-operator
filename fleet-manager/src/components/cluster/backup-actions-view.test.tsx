// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { applyBackupChange } from "@/app/fleet/clusters/[namespace]/[name]/actions";
import { GitPushesProvider } from "@/components/fleet/git-pushes";
import { Toast, useWriteToast } from "@/components/fleet/toast";
import type { WriteMode, WriteResult } from "@/lib/write/types";
import { FakeEventSource } from "../../../test/fakes/event-source";
import { fakeWriter } from "../../../test/fakes/writer";
import type { BackupContext } from "./backup-actions";
import { BackupHeaderActions, BackupRowActions } from "./backup-actions-view";
import type { BackupRow, BackupStorage, RestoreInterval } from "./backups";

vi.mock("@/lib/write/writer", () => import("../../../test/fakes/writer"));

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
  restoreNames: [],
  readingRestores: [],
  clusterVersion: "8.9.3",
  now: "2026-11-10T14:30:05Z",
  ...over,
});

const row = (over: Partial<BackupRow> = {}): BackupRow => ({
  kind: "LogicalBackupElasticsearch",
  name: "nightly-1",
  origin: "Manual",
  started: "2026-11-10 02:00 UTC",
  finished: "2026-11-10 02:12 UTC",
  duration: "12m",
  phase: "Completed",
  tone: "ok",
  step: "-",
  version: "8.9.3",
  details: [],
  ...over,
});

// Page owns the toast of the page, as the edit host does.
function Page({ children }: { children: (show: Show) => ReactNode }) {
  const { toast, show, dismiss } = useWriteToast();
  return (
    <>
      {children(show)}
      {toast && <Toast result={toast} onDismiss={dismiss} />}
    </>
  );
}
type Show = ReturnType<typeof useWriteToast>["show"];

function renderPage(children: (show: Show) => ReactNode) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <GitPushesProvider>
        <Page>{children}</Page>
      </GitPushesProvider>
    </QueryClientProvider>,
  );
}

const header = (c: BackupContext = context()) =>
  renderPage((show) => (
    <BackupHeaderActions context={c} apply={applyBackupChange} show={show} />
  ));

const rowActions = (r: BackupRow, c: BackupContext = context()) =>
  renderPage((show) => (
    <BackupRowActions
      context={c}
      row={r}
      apply={applyBackupChange}
      show={show}
    />
  ));

const NAME = "orders-manual-20261110-143005";

beforeEach(() => {
  fakeWriter.reset();
  FakeEventSource.instances = [];
  vi.stubGlobal("EventSource", FakeEventSource);
});
afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
});

async function startBackup() {
  await userEvent.click(screen.getByRole("button", { name: "Back up now" }));
  expect(screen.getByLabelText("Name")).toHaveProperty("value", NAME);
  await userEvent.click(screen.getByRole("button", { name: "Start backup" }));
}

const esBackup = {
  intent: "Back up orders",
  apply: [
    {
      apiVersion: "core.camunda.io/v1",
      kind: "LogicalBackupElasticsearch",
      metadata: { name: NAME, namespace: "team-a" },
      spec: { clusterRef: { name: "orders" } },
    },
  ],
  create: [`LogicalBackupElasticsearch/team-a/${NAME}`],
  directOnly: true,
};

describe("BackupHeaderActions", () => {
  it("back up now writes the exact Elasticsearch backup in direct mode", async () => {
    header();
    await startBackup();
    await waitFor(() => expect(fakeWriter.applied).toEqual([esBackup]));
    expect((await screen.findByRole("status")).textContent).toContain(
      `Backup ${NAME} created.`,
    );
    expect(screen.queryByLabelText("Name")).toBeNull();
  });

  it("back up now writes the same backup in Git mode and does not wait for GitOps", async () => {
    fakeWriter.reset("git" satisfies WriteMode);
    header();
    await startBackup();
    await waitFor(() => expect(fakeWriter.applied).toEqual([esBackup]));
    const status = await screen.findByRole("status");
    expect(status.textContent).toContain(`Backup ${NAME} created.`);
    expect(status.textContent).not.toContain("Waiting for GitOps to apply");
  });

  it("back up now writes an RDBMS backup for relational storage", async () => {
    header(context({ storage: rdbms }));
    await startBackup();
    await waitFor(() =>
      expect(fakeWriter.applied).toEqual([
        {
          intent: "Back up orders",
          apply: [
            {
              apiVersion: "core.camunda.io/v1",
              kind: "LogicalBackupRDBMS",
              metadata: { name: NAME, namespace: "team-a" },
              spec: { clusterRef: { name: "orders" } },
            },
          ],
          create: [`LogicalBackupRDBMS/team-a/${NAME}`],
          directOnly: true,
        },
      ]),
    );
  });

  it.each([
    [
      context({ running: { what: "Backup", name: "nightly-1" } }),
      "Backup nightly-1 is running. The new backup waits until it ends.",
    ],
    [
      context({
        suspension: "held",
        running: { what: "Restore", name: "r1" },
      }),
      "Restore r1 is running. The new backup waits until it ends.",
    ],
  ])(
    "back up now stays enabled with its note while a backup or a restore runs",
    async (c, note) => {
      header(c);
      const button = screen.getByRole("button", { name: "Back up now" });
      expect(button).toHaveProperty("disabled", false);
      await userEvent.click(button);
      expect(screen.getByText(note)).toBeTruthy();
    },
  );

  it.each([
    [
      context({ storage: { kind: "none", schedules: [] } }),
      "This cluster has no backup storage.",
    ],
    [
      context({
        storage: {
          kind: "unknown",
          missing: "SecondaryStorageConfig orders-es",
          bucket: { name: "backups" },
          schedules: [],
        },
      }),
      "SecondaryStorageConfig orders-es not found, so the kind of backup is not known.",
    ],
    [
      context({ suspension: "requested" }),
      "The cluster is suspended. A backup waits until it runs again, so resume the cluster first.",
    ],
    [
      context({ suspension: "StorageAlreadyAttached" }),
      "Another cluster holds the secondary storage of this cluster, so the operator keeps this cluster suspended, and a backup waits until it runs again. Give this cluster storage of its own, or delete the cluster that holds the storage.",
    ],
  ])(
    "back up now is disabled with its reason without backup storage, with unknown storage, and on a suspended cluster",
    (c, reason) => {
      header(c);
      const button = screen.getByRole("button", { name: "Back up now" });
      expect(button).toHaveProperty("disabled", true);
      const described = button.getAttribute("aria-describedby") ?? "";
      expect(document.getElementById(described)?.textContent).toBe(reason);
    },
  );

  it.each([
    [
      context({ suspension: "requested" }),
      "The cluster is suspended. A backup waits until it runs again, so resume the cluster first.",
    ],
    [
      context({ storage: { kind: "none", schedules: [] } }),
      "This cluster has no backup storage.",
    ],
  ])(
    "start backup turns off with the reason when back up now turns off while the form is open",
    async (off, reason) => {
      const client = new QueryClient();
      const page = (c: BackupContext) => (
        <QueryClientProvider client={client}>
          <GitPushesProvider>
            <Page>
              {(show) => (
                <BackupHeaderActions
                  context={c}
                  apply={applyBackupChange}
                  show={show}
                />
              )}
            </Page>
          </GitPushesProvider>
        </QueryClientProvider>
      );
      const { rerender } = render(page(context()));
      await userEvent.click(
        screen.getByRole("button", { name: "Back up now" }),
      );
      rerender(page(off));
      const start = screen.getByRole("button", { name: "Start backup" });
      expect(start).toHaveProperty("disabled", true);
      const described = start.getAttribute("aria-describedby") ?? "";
      expect(document.getElementById(described)?.textContent).toBe(reason);
    },
  );

  it("a taken name blocks Start backup", async () => {
    header(context({ backupNames: [NAME] }));
    await userEvent.click(screen.getByRole("button", { name: "Back up now" }));
    expect(
      screen.getByText(`A backup named ${NAME} exists. Choose another name.`),
    ).toBeTruthy();
    const start = screen.getByRole("button", { name: "Start backup" });
    expect(start).toHaveProperty("disabled", true);

    const field = screen.getByLabelText("Name");
    await userEvent.clear(field);
    await userEvent.type(field, "orders-before-upgrade");
    expect(start).toHaveProperty("disabled", false);
  });

  it("a refused backup keeps the form open and shows the error", async () => {
    fakeWriter.results = [
      {
        ok: false,
        errors: [{ code: "Invalid", message: "spec is immutable" }],
      },
    ];
    header();
    await startBackup();
    expect((await screen.findByRole("alert")).textContent).toContain(
      "spec is immutable",
    );
    expect(screen.getByLabelText("Name")).toHaveProperty("value", NAME);
  });

  it("cancel closes the form and writes nothing", async () => {
    header();
    await userEvent.click(screen.getByRole("button", { name: "Back up now" }));
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));
    expect(screen.queryByLabelText("Name")).toBeNull();
    expect(fakeWriter.applied).toEqual([]);
  });

  it("change schedule links to the attachments of the Claim tab", () => {
    header();
    expect(
      screen
        .getByRole("link", { name: "Change schedule" })
        .getAttribute("href"),
    ).toBe("?tab=claim&section=attachments");
  });

  it("the header says that backups and restores skip Git", () => {
    header();
    expect(
      screen.getByText(
        "Backups and restores go straight to the cluster, also in Git mode.",
      ),
    ).toBeTruthy();
  });
});

describe("BackupRowActions", () => {
  it.each(["direct", "git"] as const)(
    "delete writes the exact delete of the backup in %s mode",
    async (mode) => {
      fakeWriter.reset(mode);
      rowActions(row({ kind: "LogicalBackupRDBMS" }));
      await userEvent.click(screen.getByRole("button", { name: "Delete" }));
      await userEvent.click(
        screen.getByRole("button", { name: "Delete backup" }),
      );
      await waitFor(() =>
        expect(fakeWriter.applied).toEqual([
          {
            intent: "Delete backup nightly-1",
            apply: [],
            delete: [
              {
                apiVersion: "core.camunda.io/v1",
                kind: "LogicalBackupRDBMS",
                namespace: "team-a",
                name: "nightly-1",
              },
            ],
            directOnly: true,
          },
        ]),
      );
      const status = await screen.findByRole("status");
      expect(status.textContent).toContain("Backup nightly-1 deleted.");
      expect(status.textContent).not.toContain("Waiting for GitOps to apply");
    },
  );

  it("the delete confirm lists the artifacts that go and what stays", async () => {
    rowActions(row({ kind: "LogicalBackupRDBMS", origin: "Scheduled" }));
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
    const confirm = screen.getByRole("group", { name: "Delete" });
    const items = within(confirm)
      .getAllByRole("listitem")
      .map((li) => li.textContent);
    expect(items).toEqual([
      "Backup nightly-1, with its database dump in the bucket.",
      "The Zeebe backups. Zeebe removes them after the retention window of the cluster.",
      "The other backups of the schedule. The schedule deletes the oldest of them beyond the counts it keeps.",
    ]);
    expect(fakeWriter.applied).toEqual([]);
  });
  it("delete is off with the name of the restore that reads the backup", async () => {
    rowActions(
      row(),
      context({
        readingRestores: [
          {
            name: "r1",
            kind: "LogicalBackupElasticsearch",
            backup: "nightly-1",
          },
        ],
      }),
    );
    const button = screen.getByRole("button", { name: "Delete" });
    expect(button).toHaveProperty("disabled", true);
    const described = button.getAttribute("aria-describedby") ?? "";
    expect(document.getElementById(described)?.textContent).toBe(
      "Restore r1 reads this backup. Delete the backup after the restore ends.",
    );
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("a page that cannot write", () => {
  const OFF = "Changes are off: the fleet manager could not read its settings.";

  it("back up now and delete are off with the reason of the page", () => {
    renderPage((show) => (
      <>
        <BackupHeaderActions
          context={context({ changesOff: OFF })}
          apply={applyBackupChange}
          show={show}
        />
        <BackupRowActions
          context={context({ changesOff: OFF })}
          row={row()}
          apply={applyBackupChange}
          show={show}
        />
      </>
    ));
    for (const name of ["Back up now", "Delete"]) {
      const button = screen.getByRole("button", { name });
      expect(button).toHaveProperty("disabled", true);
      const described = button.getAttribute("aria-describedby") ?? "";
      expect(document.getElementById(described)?.textContent).toBe(OFF);
    }
  });
});

describe("a request that rejects", () => {
  let calls = 0;
  const rejects = async (): Promise<WriteResult> => {
    calls++;
    throw new Error("fetch failed");
  };
  beforeEach(() => {
    calls = 0;
  });

  it("a rejected backup shows the error and keeps the form open", async () => {
    renderPage((show) => (
      <BackupHeaderActions context={context()} apply={rejects} show={show} />
    ));
    await startBackup();
    expect((await screen.findByRole("alert")).textContent).toContain(
      "fetch failed",
    );
    expect(screen.getByLabelText("Name")).toHaveProperty("value", NAME);
  });

  it("a rejected delete shows the error, and the next confirm sends the delete again", async () => {
    renderPage((show) => (
      <BackupRowActions
        context={context()}
        row={row()}
        apply={rejects}
        show={show}
      />
    ));
    await userEvent.click(screen.getByRole("button", { name: "Delete" }));
    const confirm = screen.getByRole("button", { name: "Delete backup" });
    await userEvent.click(confirm);
    expect((await screen.findByRole("alert")).textContent).toContain(
      "fetch failed",
    );
    await userEvent.click(confirm);
    await waitFor(() => expect(calls).toBe(2));
  });
});

describe("restore of a backup", () => {
  const RESTORE = "orders-restore-20261110-143005";
  const dump = (over: Partial<BackupRow> = {}) =>
    row({ kind: "LogicalBackupRDBMS", version: "8.9.3", ...over });
  // stored is the backup of dump() on the API server, which the server
  // action reads before it writes a restore.
  const stored = {
    apiVersion: "core.camunda.io/v1",
    kind: "LogicalBackupRDBMS",
    metadata: { name: "nightly-1", namespace: "team-a" },
    spec: { clusterRef: { name: "orders" } },
    status: { phase: "Completed", version: "8.9.3" },
  };

  async function openRestore() {
    await userEvent.click(screen.getByRole("button", { name: "Restore" }));
    return screen.getByRole("dialog", { name: "Restore orders" });
  }

  async function confirm(dialog: HTMLElement) {
    await userEvent.type(
      within(dialog).getByLabelText("Type orders to confirm"),
      "orders",
    );
    await userEvent.click(
      within(dialog).getByRole("button", { name: "Restore" }),
    );
  }

  it.each(["direct", "git"] as const)(
    "restore of a completed backup writes the exact logical restore in %s mode",
    async (mode) => {
      fakeWriter.reset(mode);
      fakeWriter.objects = [stored];
      rowActions(dump(), context({ storage: rdbms }));
      await confirm(await openRestore());
      await waitFor(() =>
        expect(fakeWriter.applied).toEqual([
          {
            intent: "Restore orders from backup nightly-1",
            apply: [
              {
                apiVersion: "core.camunda.io/v1",
                kind: "LogicalRestoreRDBMS",
                metadata: { name: RESTORE, namespace: "team-a" },
                spec: {
                  backupRef: { name: "nightly-1" },
                  targetClusterRef: { name: "orders" },
                },
              },
            ],
            create: [`LogicalRestoreRDBMS/team-a/${RESTORE}`],
            directOnly: true,
          },
        ]),
      );
      const status = await screen.findByRole("status");
      expect(status.textContent).toContain(`Restore ${RESTORE} created.`);
      expect(status.textContent).not.toContain("Waiting for GitOps to apply");
      expect(screen.queryByRole("dialog")).toBeNull();
    },
  );

  it("the restore confirm names the backup, both versions, and that the version changes", async () => {
    rowActions(
      dump({ version: "8.9.3" }),
      context({ storage: rdbms, clusterVersion: "8.10.1" }),
    );
    const dialog = await openRestore();
    const items = within(dialog)
      .getAllByRole("listitem")
      .map((li) => li.textContent);
    expect(items).toEqual([
      "Backup nightly-1 has Camunda 8.9.3. The cluster has 8.10.1.",
      "The restore replaces the data of the cluster and sets its version to 8.9.3.",
      "It suspends the cluster while it runs, and starts it again when it completes. A failed restore leaves the cluster suspended.",
    ]);
  });

  it("the confirm stays disabled until the cluster name is typed", async () => {
    rowActions(dump(), context({ storage: rdbms }));
    const dialog = await openRestore();
    const button = within(dialog).getByRole("button", { name: "Restore" });
    expect(button).toHaveProperty("disabled", true);
    await userEvent.type(
      within(dialog).getByLabelText("Type orders to confirm"),
      "Orders",
    );
    expect(button).toHaveProperty("disabled", true);
    expect(fakeWriter.applied).toEqual([]);
  });

  it("restore is disabled with its reason when the RDBMS backup is on another minor", () => {
    rowActions(
      dump({ version: "8.8.4" }),
      context({ storage: rdbms, clusterVersion: "8.10.1" }),
    );
    const button = screen.getByRole("button", { name: "Restore" });
    expect(button).toHaveProperty("disabled", true);
    const described = button.getAttribute("aria-describedby") ?? "";
    expect(document.getElementById(described)?.textContent).toBe(
      "Backup nightly-1 has Camunda 8.8.4. It restores only into Camunda 8.8 or 8.9, and the cluster has 8.10.1.",
    );
  });

  it.each(["Failed", "Running", "Pending"])(
    "restore is not offered on a %s backup",
    (phase) => {
      rowActions(dump({ phase }), context({ storage: rdbms }));
      expect(screen.queryByRole("button", { name: "Restore" })).toBeNull();
      expect(screen.getByRole("button", { name: "Delete" })).toBeTruthy();
    },
  );

  it("restore stays enabled with its note while a backup runs", async () => {
    rowActions(
      dump(),
      context({
        storage: rdbms,
        running: { what: "Backup", name: "nightly-2" },
      }),
    );
    const dialog = await openRestore();
    expect(
      within(dialog).getByText(
        "Backup nightly-2 is running. The new restore waits until it ends.",
      ),
    ).toBeTruthy();
  });

  it("a refused restore closes the confirm and shows the error", async () => {
    fakeWriter.objects = [
      stored,
      {
        apiVersion: "core.camunda.io/v1",
        kind: "LogicalRestoreRDBMS",
        metadata: { name: "r0", namespace: "team-a" },
        spec: {
          backupRef: { name: "nightly-0" },
          targetClusterRef: { name: "orders" },
        },
        status: { phase: "Pending" },
      },
    ];
    rowActions(dump(), context({ storage: rdbms }));
    await confirm(await openRestore());
    expect((await screen.findByRole("alert")).textContent).toContain(
      "Restore r0 of this cluster has not ended.",
    );
    expect(fakeWriter.applied).toEqual([]);
  });
});

describe("restore to a point in time", () => {
  const ago = (days: number) =>
    new Date(
      Date.parse("2026-11-10T14:30:05Z") - days * 86_400_000,
    ).toISOString();
  const archived = (window: RestoreInterval[]): BackupStorage => ({
    ...rdbms,
    database: {
      archive: {
        server: "orders-db",
        state: { tone: "ok", label: "Archiving" },
        window,
      },
    },
  });
  const window: RestoreInterval[] = [
    { from: "2026-11-05T00:00:00Z", to: "2026-11-08T00:00:00Z", failing: true },
    { from: "2026-11-09T00:00:00Z" },
  ];

  async function openPoint() {
    await userEvent.click(
      screen.getByRole("button", { name: "Restore to a point in time" }),
    );
    return screen.getByRole("dialog", {
      name: "Restore orders to a point in time",
    });
  }

  function pick(dialog: HTMLElement, value: string) {
    fireEvent.change(within(dialog).getByLabelText("Time (UTC)"), {
      target: { value },
    });
  }

  it("point in time is offered only on PostgreSQL with recovery by the operator", () => {
    header(context());
    header(context({ storage: rdbms }));
    expect(
      screen.queryByRole("button", { name: "Restore to a point in time" }),
    ).toBeNull();
    cleanup();
    header(context({ storage: archived(window) }));
    expect(
      screen.getByRole("button", { name: "Restore to a point in time" }),
    ).toHaveProperty("disabled", false);
  });

  it("point in time lists the window and refuses a point outside it", async () => {
    header(context({ storage: archived(window) }));
    const dialog = await openPoint();
    const items = within(dialog)
      .getAllByRole("listitem")
      .map((li) => li.textContent);
    expect(items).toEqual([
      "2026-11-05 00:00 UTC to 2026-11-08 00:00 UTC. Uploads are failing after this point.",
      "2026-11-09 00:00 UTC to now",
    ]);
    pick(dialog, "2026-11-08T12:00:00");
    expect(
      within(dialog).getByText("Choose a point inside the restore window."),
    ).toBeTruthy();
    expect(
      within(dialog).queryByLabelText("Type orders to confirm"),
    ).toBeNull();
    pick(dialog, "2026-11-11T12:00:00");
    expect(
      within(dialog).getByText("Choose a point in the past."),
    ).toBeTruthy();
  });

  it.each(["direct", "git"] as const)(
    "point in time writes the exact PointInTimeRestore in %s mode",
    async (mode) => {
      fakeWriter.reset(mode);
      header(context({ storage: archived(window) }));
      const dialog = await openPoint();
      pick(dialog, "2026-11-09T09:30:15");
      expect(
        within(dialog)
          .getAllByRole("listitem")
          .map((li) => li.textContent)
          .slice(2),
      ).toEqual([
        "The database server rolls back to 2026-11-09 09:30:15 UTC, and the Zeebe data goes back to the backup that covers it.",
        "The restore suspends the cluster while it runs, and starts it again when it completes.",
        "If no Zeebe backup covers the point, the restore fails after it erased the broker volumes.",
      ]);
      await userEvent.type(
        within(dialog).getByLabelText("Type orders to confirm"),
        "orders",
      );
      await userEvent.click(
        within(dialog).getByRole("button", { name: "Restore" }),
      );
      await waitFor(() =>
        expect(fakeWriter.applied).toEqual([
          {
            intent: "Restore orders to 2026-11-09T09:30:15Z",
            apply: [
              {
                apiVersion: "core.camunda.io/v1",
                kind: "PointInTimeRestore",
                metadata: {
                  name: "orders-pitr-20261110-143005",
                  namespace: "team-a",
                },
                spec: {
                  clusterRef: { name: "orders" },
                  timestamp: "2026-11-09T09:30:15Z",
                },
              },
            ],
            create: ["PointInTimeRestore/team-a/orders-pitr-20261110-143005"],
            directOnly: true,
          },
        ]),
      );
      const status = await screen.findByRole("status");
      expect(status.textContent).toContain(
        "Restore orders-pitr-20261110-143005 created.",
      );
      expect(status.textContent).not.toContain("Waiting for GitOps to apply");
    },
  );

  it("point in time is disabled when the cluster keeps its Zeebe backups for a window the fleet manager cannot read", () => {
    header(
      context({
        storage: archived([{ from: ago(1) }]),
        retentionWindow: "P1W",
      }),
    );
    const button = screen.getByRole("button", {
      name: "Restore to a point in time",
    });
    expect(button).toHaveProperty("disabled", true);
    const described = button.getAttribute("aria-describedby") ?? "";
    expect(document.getElementById(described)?.textContent).toBe(
      "The cluster keeps its Zeebe backups for P1W, which the fleet manager cannot read, so it cannot offer a point.",
    );
  });
});
