// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  cleanup,
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
import type { WriteMode } from "@/lib/write/types";
import { FakeEventSource } from "../../../test/fakes/event-source";
import { fakeWriter } from "../../../test/fakes/writer";
import type { BackupContext } from "./backup-actions";
import { BackupHeaderActions, BackupRowActions } from "./backup-actions-view";
import type { BackupRow, BackupStorage } from "./backups";

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
  suspended: false,
  backupNames: [],
  readingRestores: [],
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
      context({ suspended: true, running: { what: "Restore", name: "r1" } }),
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
      context({ suspended: true }),
      "The cluster is suspended. A backup waits until it runs again, so resume the cluster first.",
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
