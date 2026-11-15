// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  ConnectionList,
  ConnectionListLive,
  type ContractRow,
  contractRows,
} from "./connection-list";
import { connectionFixtures } from "./fixtures";
import { CONNECTION_TABS, type ConnectionTab } from "./tabs";

const failing = vi.hoisted(() => ({ kind: "" }));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
    ...(kind === failing.kind && { error: `${kind} is forbidden` }),
  }),
}));

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(),
}));

afterEach(() => {
  cleanup();
  failing.kind = "";
});

function rowsOf(): Record<ConnectionTab, ContractRow[]> {
  const sources = connectionFixtures();
  return Object.fromEntries(
    CONNECTION_TABS.map((t) => [t.value, contractRows(sources, t.kind)]),
  ) as Record<ConnectionTab, ContractRow[]>;
}

function show(tab: ConnectionTab, errors = []) {
  render(
    <ConnectionList
      tab={tab}
      rows={rowsOf()}
      errors={errors}
      onTab={vi.fn()}
    />,
  );
}

function rowOf(name: string): HTMLElement {
  return screen.getByRole("link", { name }).closest("tr")!;
}

describe("ConnectionList", () => {
  it("has the five tabs, including Databases", () => {
    show("object-storage");
    expect(screen.getAllByRole("tab").map((t) => t.textContent)).toEqual([
      "Object storage",
      "Secondary storage",
      "Database servers",
      "Databases",
      "Management Identity",
    ]);
  });

  it("shows each writer of the Object storage tab", () => {
    show("object-storage");
    expect(within(rowOf("backups")).getByText("Created by hand")).toBeTruthy();
    expect(
      within(rowOf("orders-backup")).getByText("Registration aws-s3"),
    ).toBeTruthy();
    expect(
      within(rowOf("old-backup")).getByText("Kept from aws-s3"),
    ).toBeTruthy();
  });

  it("shows the owner of a secondary storage contract", () => {
    show("secondary-storage");
    expect(
      within(rowOf("orders-es")).getByText("ElasticsearchCluster orders-es"),
    ).toBeTruthy();
  });

  it("shows the plane of a Management Identity contract, without a Namespace column", () => {
    show("management-identity");
    expect(
      within(rowOf("plane-a-optimize")).getByText(
        "Management plane platform/plane-a",
      ),
    ).toBeTruthy();
    expect(
      screen.queryByRole("columnheader", { name: /Namespace/ }),
    ).toBeNull();
  });

  it("shows the key facts of the kind", () => {
    show("database-servers");
    expect(screen.getByRole("columnheader", { name: /Host/ })).toBeTruthy();
    expect(within(rowOf("postgres")).getByText("pg.team-a:5432")).toBeTruthy();
  });

  it("links a contract to its page", () => {
    show("object-storage");
    expect(
      screen.getByRole("link", { name: "backups" }).getAttribute("href"),
    ).toBe("/setup/connections/object-storage/team-a/backups");
  });

  it("counts the users of a contract", () => {
    show("database-servers");
    const cells = within(rowOf("postgres")).getAllByRole("cell");
    const users = screen
      .getAllByRole("columnheader")
      .findIndex((h) => h.textContent?.startsWith("Users"));
    expect(cells[users].textContent).toBe("2");
  });

  it("filters by writer", async () => {
    show("object-storage");
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Writer" }),
      "Created by hand",
    );
    expect(screen.getByRole("link", { name: "backups" })).toBeTruthy();
    expect(screen.queryByRole("link", { name: "orders-backup" })).toBeNull();
    expect(screen.queryByRole("link", { name: "old-backup" })).toBeNull();
  });

  it("shows the Ready of the contract", () => {
    show("database-servers");
    expect(
      within(rowOf("postgres")).getByText("Not ready · ConnectionFailed"),
    ).toBeTruthy();
    cleanup();
    show("object-storage");
    expect(within(rowOf("backups")).getByText("Ready")).toBeTruthy();
  });

  it("shows the read errors above the tabs", () => {
    render(
      <ConnectionList
        tab="object-storage"
        rows={rowsOf()}
        errors={[{ kind: "DatabaseServer", message: "forbidden" }]}
        onTab={vi.fn()}
      />,
    );
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read DatabaseServer: forbidden",
    );
  });

  it("says that a tab has no contract", () => {
    render(
      <ConnectionList
        tab="databases"
        rows={{ ...rowsOf(), databases: [] }}
        errors={[]}
        onTab={vi.fn()}
      />,
    );
    expect(screen.getByText("No databases contract yet.")).toBeTruthy();
  });
});

describe("contractRows", () => {
  it("treats a plugin output as managed while the requests are not known", () => {
    failing.kind = "PluginRequest";
    render(<ConnectionListLive initial={connectionFixtures()} />);
    expect(
      within(rowOf("old-backup")).getByText("Registration aws-s3"),
    ).toBeTruthy();
    expect(screen.getByRole("alert").textContent).toContain("PluginRequest");
  });

  it("gives each contract of the kind one row, with its writer", () => {
    expect(
      contractRows(connectionFixtures(), "ObjectStorageConfig").map((r) => [
        r.ref.name,
        r.writer.by,
        r.users,
      ]),
    ).toEqual([
      ["backups", "hand", 1],
      ["old-backup", "kept", 0],
      ["orders-backup", "registration", 0],
    ]);
  });
});
