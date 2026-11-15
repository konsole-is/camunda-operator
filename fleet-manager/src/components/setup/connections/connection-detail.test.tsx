// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { ContractRef } from "@/lib/usage";
import { ConnectionDetail } from "./connection-detail";
import { connectionFixtures } from "./fixtures";

afterEach(cleanup);

function show(target: ContractRef) {
  render(
    <ConnectionDetail
      target={target}
      sources={{ ...connectionFixtures(), errors: [] }}
    />,
  );
}

const inTeamA = (kind: ContractRef["kind"], name: string): ContractRef => ({
  kind,
  namespace: "team-a",
  name,
});

const panel = (name: string) => screen.getByRole("region", { name });

describe("ConnectionDetail", () => {
  it("shows the Ready reason and message of the contract", () => {
    show(inTeamA("DatabaseServerConfig", "postgres"));
    const status = panel("Status");
    expect(within(status).getByText("ConnectionFailed")).toBeTruthy();
    expect(
      within(status).getByText(
        "Connecting to pg.team-a:5432: connection refused",
      ),
    ).toBeTruthy();
  });

  it("says No status yet for a contract without a Ready", () => {
    const sources = connectionFixtures();
    sources.objectStorageConfigs[0] = {
      ...sources.objectStorageConfigs[0],
      status: undefined,
    };
    render(
      <ConnectionDetail
        target={inTeamA("ObjectStorageConfig", "backups")}
        sources={{ ...sources, errors: [] }}
      />,
    );
    expect(within(panel("Status")).getByText("No status yet")).toBeTruthy();
  });

  it("never claims a check of the bucket", () => {
    show(inTeamA("ObjectStorageConfig", "backups"));
    expect(screen.queryByText(/reachab|bucket access|tested/i)).toBeNull();
  });

  it.each<[ContractRef, string]>([
    [
      inTeamA("SecondaryStorageConfig", "orders-es"),
      "Managed by ElasticsearchCluster orders-es. Change it there.",
    ],
    [
      inTeamA("ObjectStorageConfig", "orders-backup"),
      "Managed by Registration aws-s3. Change it there.",
    ],
    [
      { kind: "ManagementAuthConfig", name: "plane-a-optimize" },
      "Managed by Management plane platform/plane-a. Change it there.",
    ],
  ])(
    "shows Managed by and Change it there for a contract not created by hand: %j",
    (target, sentence) => {
      show(target);
      expect(within(panel("Writer")).getByText(sentence)).toBeTruthy();
    },
  );

  it.each<[ContractRef, string]>([
    [inTeamA("ObjectStorageConfig", "backups"), "Created by hand"],
    [inTeamA("ObjectStorageConfig", "old-backup"), "Kept from aws-s3"],
  ])(
    "shows no Managed by line for a contract created by hand or kept from a registration: %j",
    (target, writer) => {
      show(target);
      expect(within(panel("Writer")).getByText(writer)).toBeTruthy();
      expect(screen.queryByText(/Managed by/)).toBeNull();
    },
  );

  it("shows the CA certificate as a name and a key", () => {
    show(inTeamA("SecondaryStorageConfig", "shared-es"));
    const settings = panel("Settings");
    const ca = within(settings).getByText("CA certificate").nextElementSibling!;
    expect(within(ca as HTMLElement).getByText("shared-es-ca")).toBeTruthy();
    expect(within(ca as HTMLElement).getByText("ca.crt")).toBeTruthy();
  });

  it("shows the admin credentials as a name, a username key, and a password key", () => {
    show(inTeamA("DatabaseServerConfig", "postgres"));
    const admin = within(panel("Settings")).getByText("Admin credentials")
      .nextElementSibling as HTMLElement;
    expect(within(admin).getByText("postgres-admin")).toBeTruthy();
    expect(within(admin).getByText("username")).toBeTruthy();
    expect(within(admin).getByText("password")).toBeTruthy();
  });

  it("shows the replica hint next to the node count", () => {
    show(inTeamA("SecondaryStorageConfig", "shared-es"));
    expect(
      within(panel("Settings")).getByText(
        "Sets the index replicas when the cluster sets no replica count.",
      ),
    ).toBeTruthy();
  });

  it("lists every user of the contract", () => {
    show(inTeamA("SecondaryStorageConfig", "shared-es"));
    const users = panel("Used by");
    expect(within(users).getByText("team-a/orders")).toBeTruthy();
    expect(within(users).getByText("team-a/billing")).toBeTruthy();
    expect(
      within(users).getByText("Not ready · StorageAlreadyAttached"),
    ).toBeTruthy();
  });

  it("names the contract and its namespace in the heading", () => {
    show(inTeamA("ObjectStorageConfig", "backups"));
    expect(
      screen.getByRole("heading", { level: 1, name: /backups/ }),
    ).toBeTruthy();
    expect(screen.getByText("team-a")).toBeTruthy();
  });

  it("says that a missing contract does not exist", () => {
    show(inTeamA("ObjectStorageConfig", "gone"));
    expect(screen.getByRole("alert").textContent).toBe(
      "The contract gone does not exist.",
    );
  });

  it("says that Used by can miss users of a kind that could not be read", () => {
    render(
      <ConnectionDetail
        target={inTeamA("ObjectStorageConfig", "backups")}
        sources={{
          ...connectionFixtures(),
          errors: [{ kind: "DatabaseServer", message: "forbidden" }],
        }}
      />,
    );
    expect(
      within(panel("Used by")).getByText(
        "This list can miss users. Could not read DatabaseServer.",
      ),
    ).toBeTruthy();
  });

  it("shows the read errors", () => {
    render(
      <ConnectionDetail
        target={inTeamA("ObjectStorageConfig", "backups")}
        sources={{
          ...connectionFixtures(),
          errors: [{ kind: "CamundaCluster", message: "forbidden" }],
        }}
      />,
    );
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read CamundaCluster: forbidden",
    );
  });
});
