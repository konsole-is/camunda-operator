// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import {
  condition,
  pluginGrant,
  pluginRequest,
  registrationSources,
} from "./fixtures";
import { usedByRows } from "./used-by";
import { UsedByTab } from "./used-by-tab";

afterEach(cleanup);

const esRequest = pluginRequest(
  "team-orders",
  "orders-secondary-storage",
  {
    registration: "elasticsearch",
    claimRef: { kind: "CamundaClusterClaim", name: "orders" },
    target: { kind: "CamundaCluster", name: "orders" },
    deletionPolicy: "Orphan",
  },
  {
    conditions: [
      condition("OutputsReady", "True", "Healthy"),
      condition("Ready", "True", "Healthy"),
    ] as never,
    summary: [{ label: "Health", value: "green", severity: "Info" }],
    resources: [
      {
        kind: "ElasticsearchCluster",
        name: "orders-es",
        role: "Search",
        onOrphan: "Kept",
      },
      {
        kind: "SecondaryStorageConfig",
        name: "orders-es",
        onOrphan: "Deleted",
      },
    ],
  },
);
const s3Request = pluginRequest(
  "team-orders",
  "orders-backup-storage",
  { registration: "aws-s3" },
  {
    conditions: [
      condition("OutputsReady", "True", "Healthy"),
      condition(
        "Ready",
        "False",
        "GrantRejected",
        "A Grant of the request is refused",
      ),
    ] as never,
  },
);
const refused = pluginGrant(
  "team-orders",
  "orders-bucket-access",
  { template: "bucket-access" },
  {
    "plugins.camunda.io/registration": "aws-s3",
    "plugins.camunda.io/request": "orders-backup-storage",
  },
  [condition("Ready", "False", "OverCeiling")],
);
const termOf = (panel: string, label: string) =>
  within(screen.getByRole("region", { name: panel })).getByText(label, {
    selector: "dt",
  }).nextElementSibling as HTMLElement;

describe("UsedByTab", () => {
  it("shows the claim, the headline, the resources, and the deletion policy of a request", () => {
    const rows = usedByRows(
      "elasticsearch",
      registrationSources({ requests: [esRequest] }),
      [],
    );
    render(<UsedByTab rows={rows} unread={[]} />);
    const panel = "team-orders/orders-secondary-storage";
    expect(
      within(termOf(panel, "Cluster"))
        .getByRole("link", { name: "orders" })
        .getAttribute("href"),
    ).toBe("/fleet/clusters/team-orders/orders");
    expect(termOf(panel, "Environment").textContent).toBe("Production");
    expect(termOf(panel, "Headline").textContent).toBe("Health: green");
    expect(termOf(panel, "Ready").textContent).toBe("Ready: True");
    const resources = termOf(panel, "Resources");
    expect(
      within(resources).getByText("ElasticsearchCluster orders-es"),
    ).toBeTruthy();
    expect(within(resources).getByText("On Orphan: Kept")).toBeTruthy();
    expect(within(resources).getByText("On Orphan: Deleted")).toBeTruthy();
    expect(termOf(panel, "Deletion policy").textContent).toBe("Orphan");
  });

  it("names the Grant with Ready False of a request whose Ready is GrantRejected", () => {
    const rows = usedByRows(
      "aws-s3",
      registrationSources({ requests: [s3Request] }),
      [refused],
    );
    render(<UsedByTab rows={rows} unread={[]} />);
    const ready = termOf("team-orders/orders-backup-storage", "Ready");
    expect(
      within(ready).getByText("Ready: False · GrantRejected"),
    ).toBeTruthy();
    expect(
      within(ready).getByText("A Grant of the request is refused"),
    ).toBeTruthy();
    expect(
      within(ready).getByText("Grants with Ready False: orders-bucket-access"),
    ).toBeTruthy();
  });

  it("links a management plane request to its plane", () => {
    const rows = usedByRows(
      "aws-s3",
      registrationSources({
        requests: [
          pluginRequest("team-orders", "ops-access", {
            registration: "aws-s3",
            claimRef: { kind: "CamundaManagementClusterClaim", name: "ops" },
            target: { kind: "CamundaManagementCluster", name: "ops" },
          }),
        ],
      }),
      [],
    );
    render(<UsedByTab rows={rows} unread={[]} />);
    expect(
      within(termOf("team-orders/ops-access", "Management plane"))
        .getByRole("link", { name: "ops" })
        .getAttribute("href"),
    ).toBe("/fleet/planes/team-orders/ops");
  });

  it("says when no claim uses the option", () => {
    render(<UsedByTab rows={[]} unread={[]} />);
    expect(screen.getByText("No claim uses this option.")).toBeTruthy();
  });

  it("does not say Not used while PluginRequest is not read", () => {
    render(<UsedByTab rows={[]} unread={["PluginRequest"]} />);
    expect(screen.queryByText("No claim uses this option.")).toBeNull();
    expect(
      screen.getByText("Not known: PluginRequest is not read."),
    ).toBeTruthy();
  });

  it("shows the conditions as not known while PluginRequest is not read", () => {
    const sources = registrationSources({ requests: [esRequest] });
    const rows = usedByRows(
      "elasticsearch",
      { ...sources, stale: ["PluginRequest"] },
      [],
    );
    render(<UsedByTab rows={rows} unread={["PluginRequest"]} />);
    expect(
      termOf("team-orders/orders-secondary-storage", "Ready").textContent,
    ).toBe("Ready: Not known");
    expect(
      termOf("team-orders/orders-secondary-storage", "Headline").textContent,
    ).toBe("Not known: PluginRequest is not read.");
    expect(
      termOf("team-orders/orders-secondary-storage", "Resources").textContent,
    ).toBe("Not known: PluginRequest is not read.");
    expect(
      screen.getByText(
        "The list can miss requests: PluginRequest is not read.",
      ),
    ).toBeTruthy();
  });

  it("shows the Grants and the environment as not known when their lists are not read", () => {
    const sources = registrationSources({ requests: [s3Request] });
    const rows = usedByRows(
      "aws-s3",
      { ...sources, stale: ["Grant", "Environment"] },
      [refused],
    );
    render(<UsedByTab rows={rows} unread={["Grant", "Environment"]} />);
    const panel = "team-orders/orders-backup-storage";
    expect(termOf(panel, "Grants").textContent).toBe(
      "Not known: Grant is not read.",
    );
    expect(termOf(panel, "Environment").textContent).toBe(
      "Not known: Environment is not read.",
    );
    expect(
      within(termOf(panel, "Ready")).queryByText(/Grants with Ready False/),
    ).toBeNull();
  });

  it("shows the error of a request whose conditions are not valid", () => {
    const rows = usedByRows(
      "aws-s3",
      registrationSources({
        requests: [
          pluginRequest("team-orders", "bad", { registration: "aws-s3" }, {
            conditions: { Ready: "True" },
          } as never),
        ],
      }),
      [],
    );
    render(<UsedByTab rows={rows} unread={[]} />);
    expect(
      within(screen.getByRole("region", { name: "team-orders/bad" })).getByRole(
        "alert",
      ).textContent,
    ).toBe("The conditions are not a list.");
  });
});
