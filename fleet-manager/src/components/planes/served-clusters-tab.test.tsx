// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import type { ServedCluster } from "./served-clusters";
import { ServedClustersTab } from "./served-clusters-tab";

afterEach(cleanup);

const row = (
  name: string,
  attached: boolean,
  extra: Partial<ServedCluster> = {},
): ServedCluster => ({
  namespace: `team-${name}`,
  name,
  environment: "production",
  attached,
  href: `/fleet/clusters/team-${name}/${name}`,
  ...extra,
});

const rows: ServedCluster[] = [
  row("orders", true),
  row("modeler", true, {
    reason: "BasicAuthUserFailed",
    message: "The Web Modeler user is missing",
  }),
  row("billing", false, {
    reason: "ClaimedElsewhere",
    message: "The plane platform/sandbox already serves this cluster",
  }),
  row("search", false, {
    environment: undefined,
    reason: "NotReady",
    message: "The orchestration cluster is not ready",
  }),
];

const table = () => screen.getByRole("region", { name: "Clusters served" });
const names = () =>
  within(table())
    .queryAllByRole("link")
    .map((l) => l.textContent);
const rowOf = (name: string) =>
  within(table())
    .getAllByRole("row")
    .find((r) => within(r).queryByRole("link", { name })) as HTMLElement;

describe("ServedClustersTab", () => {
  it("shows cluster, environment, namespace, attached, reason, and message, and links the cluster page", () => {
    render(<ServedClustersTab rows={rows} />);
    for (const column of [
      "Cluster",
      "Environment",
      "Namespace",
      "Attached",
      "Reason",
      "Message",
    ]) {
      expect(
        within(table()).getByRole("columnheader", { name: column }),
      ).toBeTruthy();
    }
    expect(
      within(rowOf("billing"))
        .getByRole("link", { name: "billing" })
        .getAttribute("href"),
    ).toBe("/fleet/clusters/team-billing/billing");
    const cells = within(rowOf("billing"))
      .getAllByRole("cell")
      .map((c) => c.textContent);
    expect(cells).toEqual([
      "billing",
      "production",
      "team-billing",
      "No",
      "ClaimedElsewhere",
      "The plane platform/sandbox already serves this cluster",
    ]);
    expect(within(rowOf("orders")).getByText("Yes")).toBeTruthy();
  });

  it("filters by environment, with No environment for a row without one", async () => {
    const user = userEvent.setup();
    render(<ServedClustersTab rows={rows} />);
    await user.selectOptions(
      screen.getByRole("combobox", { name: "Environment" }),
      "No environment",
    );
    expect(names()).toEqual(["search"]);
    expect(within(rowOf("search")).getByText("No environment")).toBeTruthy();
  });

  it("filters by attached, and keeps an attached row with a reason under Attached", async () => {
    const user = userEvent.setup();
    render(<ServedClustersTab rows={rows} />);
    const attached = screen.getByRole("combobox", { name: "Attached" });
    await user.selectOptions(attached, "Attached");
    expect(names()).toEqual(["orders", "modeler"]);
    await user.selectOptions(attached, "Not attached");
    expect(names()).toEqual(["billing", "search"]);
  });

  it("searches the name, the namespace, the reason, and the message", async () => {
    const user = userEvent.setup();
    render(<ServedClustersTab rows={rows} />);
    const search = screen.getByRole("searchbox", { name: "Search" });
    await user.type(search, "ClaimedElsewhere");
    expect(names()).toEqual(["billing"]);
    await user.clear(search);
    await user.type(search, "team-search");
    expect(names()).toEqual(["search"]);
    await user.clear(search);
    await user.type(search, "user is missing");
    expect(names()).toEqual(["modeler"]);
  });

  it("says when the plane lists no clusters", () => {
    render(<ServedClustersTab rows={[]} />);
    expect(screen.getByText("This plane lists no clusters yet.")).toBeTruthy();
  });

  it("does not say that the plane lists no clusters while the plane is not read, and names the kind", () => {
    render(
      <ServedClustersTab rows={[]} unread={["CamundaManagementCluster"]} />,
    );
    expect(screen.queryByText("This plane lists no clusters yet.")).toBeNull();
    expect(
      screen.getAllByText(
        "The fleet manager has no current list of CamundaManagementCluster. This tab can be incomplete or out of date.",
      ),
    ).toHaveLength(1);
  });

  it("shows the environment as not known, not as none, while it cannot be read", async () => {
    const user = userEvent.setup();
    render(
      <ServedClustersTab
        rows={[
          row("orders", true, {
            environment: undefined,
            environmentUnknown: true,
          }),
        ]}
        unread={["Namespace"]}
      />,
    );
    expect(within(rowOf("orders")).getByText("Not known")).toBeTruthy();
    expect(within(rowOf("orders")).queryByText("No environment")).toBeNull();
    expect(
      screen.getByText(
        "The fleet manager has no current list of Namespace. This tab can be incomplete or out of date.",
      ),
    ).toBeTruthy();
    const options = within(
      screen.getByRole("combobox", { name: "Environment" }),
    )
      .getAllByRole("option")
      .map((o) => o.textContent);
    expect(options).not.toContain("No environment");
    await user.selectOptions(
      screen.getByRole("combobox", { name: "Environment" }),
      "Not known",
    );
    expect(names()).toEqual(["orders"]);
  });
});
