// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ActivityBanner, UnmanagedBanner } from "./banners";

afterEach(cleanup);

describe("UnmanagedBanner", () => {
  it("says the cluster is read only here", () => {
    render(<UnmanagedBanner />);
    expect(screen.getByRole("status").textContent).toBe(
      "Not managed by the fleet manager. You can read this cluster but not change it here.",
    );
  });
});

describe("ActivityBanner", () => {
  it("names the running backup and links to the Backups tab", () => {
    render(
      <ActivityBanner
        backup={{
          kind: "LogicalBackupRDBMS",
          namespace: "team-a",
          name: "nightly-1",
        }}
        backupsHref="?tab=backups"
      />,
    );
    expect(screen.getByRole("status").textContent).toContain(
      "Backup nightly-1 is running.",
    );
    expect(
      screen
        .getByRole("link", { name: "Open the Backups tab" })
        .getAttribute("href"),
    ).toBe("?tab=backups");
  });

  it("names a restore", () => {
    render(
      <ActivityBanner
        restore={{
          kind: "LogicalRestoreRDBMS",
          namespace: "team-a",
          name: "r1",
        }}
        backupsHref="?tab=backups"
      />,
    );
    expect(screen.getByRole("status").textContent).toContain(
      "Restore r1 is running.",
    );
  });

  it("names a point-in-time restore", () => {
    render(
      <ActivityBanner
        restore={{
          kind: "PointInTimeRestore",
          namespace: "team-a",
          name: "pitr",
        }}
        backupsHref="?tab=backups"
      />,
    );
    expect(screen.getByRole("status").textContent).toContain(
      "Point-in-time restore pitr is running.",
    );
  });

  it("renders nothing without a running backup or restore", () => {
    const { container } = render(<ActivityBanner backupsHref="?tab=backups" />);
    expect(container.innerHTML).toBe("");
  });

  it("calls onOpenBackups on a click", async () => {
    const onOpenBackups = vi.fn();
    render(
      <ActivityBanner
        backup={{ kind: "LogicalBackupRDBMS", namespace: "team-a", name: "b" }}
        backupsHref="?tab=backups"
        onOpenBackups={onOpenBackups}
      />,
    );
    await userEvent.click(
      screen.getByRole("link", { name: "Open the Backups tab" }),
    );
    expect(onOpenBackups).toHaveBeenCalledOnce();
  });
});
