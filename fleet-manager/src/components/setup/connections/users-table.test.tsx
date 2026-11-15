// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { ContractUser } from "@/lib/usage";
import { UsersTable } from "./users-table";

afterEach(cleanup);

const refused: ContractUser = {
  kind: "CamundaCluster",
  namespace: "team-a",
  name: "billing",
  fields: ["spec.storageRef"],
  ready: {
    status: "False",
    reason: "StorageAlreadyAttached",
    message: "Another cluster holds this storage backend: team-a/orders",
  },
  environment: "production",
};

function rowOf(name: string): HTMLElement {
  return screen.getByText(name).closest("tr")!;
}

describe("UsersTable", () => {
  it("shows a cluster refused by a secondary storage contract with its own Ready", () => {
    render(<UsersTable users={[refused]} />);
    const row = rowOf("team-a/billing");
    expect(
      within(row).getByText("Not ready · StorageAlreadyAttached"),
    ).toBeTruthy();
    expect(
      within(row).getByText(
        "Another cluster holds this storage backend: team-a/orders",
      ),
    ).toBeTruthy();
  });

  it("shows no message under the chip of a ready user", () => {
    render(
      <UsersTable
        users={[
          {
            ...refused,
            ready: {
              status: "True",
              reason: "Healthy",
              message: "All checks passed",
            },
          },
        ]}
      />,
    );
    expect(screen.queryByText("All checks passed")).toBeNull();
  });

  it("names the preset that sets the reference", () => {
    render(
      <UsersTable
        users={[
          {
            kind: "ElasticsearchCluster",
            namespace: "team-a",
            name: "orders-es",
            fields: ["spec.snapshotStorageRef"],
            preset: "es-small",
          },
        ]}
      />,
    );
    const row = rowOf("team-a/orders-es");
    expect(within(row).getByText("spec.snapshotStorageRef")).toBeTruthy();
    expect(within(row).getByText("from preset es-small")).toBeTruthy();
  });

  it("links a cluster to its page", () => {
    render(<UsersTable users={[refused]} />);
    expect(
      screen.getByRole("link", { name: "team-a/billing" }).getAttribute("href"),
    ).toBe("/fleet/clusters/team-a/billing");
  });

  it("links no other kind", () => {
    render(
      <UsersTable
        users={[{ ...refused, kind: "DatabaseConfig", name: "orders-db" }]}
      />,
    );
    expect(screen.queryByRole("link", { name: "team-a/orders-db" })).toBeNull();
  });

  it("shows the environment of each user", () => {
    render(
      <UsersTable
        users={[
          refused,
          { ...refused, name: "orders", environment: undefined },
        ]}
      />,
    );
    expect(
      within(rowOf("team-a/billing")).getByText("production"),
    ).toBeTruthy();
    expect(
      within(rowOf("team-a/orders")).getByText("No environment"),
    ).toBeTruthy();
  });

  it("says that no resource uses the contract", () => {
    render(<UsersTable users={[]} />);
    expect(screen.getByText("No resource uses this contract.")).toBeTruthy();
  });
});
