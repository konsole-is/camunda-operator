// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import type { BackupRow, BackupStorage, RestoreRow } from "./backups";
import { BackupsTab } from "./backups-tab";

afterEach(cleanup);

const esStorage: BackupStorage = {
  kind: "elasticsearch",
  bucket: { name: "backups", location: "s3://acme-backups" },
  repository: "camunda-backups",
  schedules: [],
};

const failedRow: BackupRow = {
  kind: "LogicalBackupRDBMS",
  name: "manual-1",
  origin: "Manual",
  started: "2026-10-18 10:00 UTC",
  finished: "-",
  duration: "-",
  phase: "Failed",
  tone: "bad",
  step: "ZeebeBackup",
  version: "-",
  details: [
    { label: "Backup ID", value: "7" },
    { label: "Failure", value: "Zeebe refused the backup" },
  ],
};

const restoreRow: RestoreRow = {
  kind: "PointInTimeRestore",
  name: "pitr-1",
  label: "Point-in-time restore",
  source: "To 2026-10-18 09:30 UTC",
  started: "2026-10-18 10:00 UTC",
  phase: "Failed",
  tone: "bad",
  running: false,
  failureMessage: "No WAL after the point",
};

describe("BackupsTab", () => {
  it("a row expands to its details and closes again", async () => {
    render(
      <BackupsTab storage={esStorage} backups={[failedRow]} restores={[]} />,
    );
    const toggle = screen.getByRole("button", { name: "Details of manual-1" });
    expect(toggle.getAttribute("aria-expanded")).toBe("false");
    expect(screen.queryByText("Zeebe refused the backup")).toBeNull();

    await userEvent.click(toggle);
    expect(toggle.getAttribute("aria-expanded")).toBe("true");
    const details = document.getElementById(
      toggle.getAttribute("aria-controls") ?? "",
    );
    expect(details).not.toBeNull();
    expect(within(details!).getByText("Zeebe refused the backup")).toBeTruthy();

    await userEvent.click(toggle);
    expect(screen.queryByText("Zeebe refused the backup")).toBeNull();
  });

  it("shows the columns of a backup row", () => {
    render(
      <BackupsTab storage={esStorage} backups={[failedRow]} restores={[]} />,
    );
    const [, row] = within(
      screen.getByRole("region", { name: "Backups" }),
    ).getAllByRole("row");
    expect(
      within(row)
        .getAllByRole("cell")
        .map((c) => c.textContent),
    ).toEqual([
      "manual-1",
      "Manual",
      "2026-10-18 10:00 UTC",
      "-",
      "-",
      "Failed",
      "ZeebeBackup",
      "-",
    ]);
  });

  it("shows a restore with its type, source, phase, and failure", () => {
    render(
      <BackupsTab storage={esStorage} backups={[]} restores={[restoreRow]} />,
    );
    const restores = screen.getByRole("region", { name: "Restores" });
    for (const text of [
      "pitr-1",
      "Point-in-time restore",
      "To 2026-10-18 09:30 UTC",
      "Failed",
      "No WAL after the point",
    ]) {
      expect(within(restores).getByText(text)).toBeTruthy();
    }
  });

  it("says so when there are no backups and no restores", () => {
    render(<BackupsTab storage={esStorage} backups={[]} restores={[]} />);
    expect(screen.getByText("No backups yet.")).toBeTruthy();
    expect(screen.getByText("No restores.")).toBeTruthy();
    expect(
      screen.getByText(
        "No backup schedule. Backups run only when someone starts one.",
      ),
    ).toBeTruthy();
  });

  it("shows the bucket and the repository of an Elasticsearch cluster", () => {
    render(<BackupsTab storage={esStorage} backups={[]} restores={[]} />);
    const storage = screen.getByRole("region", { name: "Backup storage" });
    expect(within(storage).getByText("s3://acme-backups")).toBeTruthy();
    expect(within(storage).getByText("camunda-backups")).toBeTruthy();
  });

  it("names a missing bucket and an unregistered repository", () => {
    render(
      <BackupsTab
        storage={{
          kind: "elasticsearch",
          bucket: { name: "gone" },
          schedules: [],
        }}
        backups={[]}
        restores={[]}
      />,
    );
    const storage = screen.getByRole("region", { name: "Backup storage" });
    expect(within(storage).getByText("gone (not found)")).toBeTruthy();
    expect(within(storage).getByText("Not registered yet")).toBeTruthy();
  });

  it("names a missing SecondaryStorageConfig", () => {
    render(
      <BackupsTab
        storage={{
          kind: "unknown",
          missing: "SecondaryStorageConfig orders-es",
          bucket: { name: "backups", location: "s3://acme-backups" },
          schedules: [],
        }}
        backups={[]}
        restores={[]}
      />,
    );
    const storage = screen.getByRole("region", { name: "Backup storage" });
    expect(
      within(storage).getByText("SecondaryStorageConfig orders-es not found"),
    ).toBeTruthy();
  });

  it("names the missing link of the PostgreSQL chain", () => {
    render(
      <BackupsTab
        storage={{
          kind: "rdbms",
          bucket: { name: "backups", location: "s3://acme-backups" },
          database: { missing: "DatabaseConfig orders-db" },
          schedules: [],
        }}
        backups={[]}
        restores={[]}
      />,
    );
    expect(screen.getByText("DatabaseConfig orders-db not found")).toBeTruthy();
  });

  it("shows a failing archive with the end of its window", () => {
    render(
      <BackupsTab
        storage={{
          kind: "rdbms",
          bucket: { name: "backups", location: "s3://acme-backups" },
          database: {
            serverConfig: "orders-pg",
            endpoint: "orders-pg-rw.team-a.svc:5432",
            pitr: { enabled: true, recovery: "external", retentionDays: 7 },
            archive: {
              server: "orders-pg",
              state: {
                tone: "bad",
                label: "ArchiveFailing",
                message: "The upload of WAL failed",
              },
              bucket: { name: "wal", location: "gs://acme-wal" },
              window: {
                from: "2026-09-25T09:58:00Z",
                to: "2026-10-18T10:00:00Z",
              },
            },
          },
          schedules: [],
        }}
        backups={[]}
        restores={[]}
      />,
    );
    const storage = screen.getByRole("region", { name: "Backup storage" });
    expect(
      within(storage).getByText("ArchiveFailing").getAttribute("title"),
    ).toBe("The upload of WAL failed");
    expect(within(storage).getByText("gs://acme-wal")).toBeTruthy();
    expect(
      within(storage).getByText("On, 7 days, recovery outside the operator"),
    ).toBeTruthy();
    expect(
      within(storage).getByText(
        "2026-09-25 09:58 UTC to 2026-10-18 10:00 UTC (uploads are failing after this point)",
      ),
    ).toBeTruthy();
  });

  it("shows point-in-time restore Off and no restore point yet", () => {
    render(
      <BackupsTab
        storage={{
          kind: "rdbms",
          bucket: { name: "backups", location: "s3://acme-backups" },
          database: {
            serverConfig: "orders-pg",
            endpoint: "orders-pg-rw.team-a.svc:5432",
            pitr: { enabled: false },
            archive: {
              server: "orders-pg",
              state: { tone: "warn", label: "Waiting" },
              bucket: { name: "wal", location: "gs://acme-wal" },
            },
          },
          schedules: [],
        }}
        backups={[]}
        restores={[]}
      />,
    );
    expect(screen.getByText("Off")).toBeTruthy();
    expect(screen.getByText("No restore point yet")).toBeTruthy();
  });

  it("lists each schedule with its cron, retention, and last backup", () => {
    render(
      <BackupsTab
        storage={{
          kind: "none",
          schedules: [
            {
              name: "nightly",
              cron: "0 2 * * *",
              completed: 7,
              failed: 3,
              lastBackup: "nightly-1",
            },
          ],
        }}
        backups={[]}
        restores={[]}
      />,
    );
    const schedule = screen.getByRole("region", { name: "Backup schedule" });
    expect(schedule.textContent).toContain(
      "nightly: 0 2 * * *, keeps 7 completed and 3 failed",
    );
    expect(within(schedule).getByText("Last backup nightly-1")).toBeTruthy();
  });

  it("without backup storage shows only the sentence in the storage block", () => {
    render(
      <BackupsTab
        storage={{ kind: "none", schedules: [] }}
        backups={[]}
        restores={[]}
      />,
    );
    const storage = screen.getByRole("region", { name: "Backup storage" });
    expect(
      within(storage).getByText(
        "This cluster has no backup storage. Backups need a bucket from the Backup storage section.",
      ),
    ).toBeTruthy();
    expect(within(storage).queryByText("Bucket")).toBeNull();
  });

  it("mounts the actions node in the header", () => {
    render(
      <BackupsTab
        storage={esStorage}
        backups={[]}
        restores={[]}
        actions={<button>Back up now</button>}
      />,
    );
    expect(screen.getByRole("button", { name: "Back up now" })).toBeTruthy();
  });
});
