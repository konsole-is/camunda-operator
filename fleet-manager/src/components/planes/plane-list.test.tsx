// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import { CLAIM_LABEL } from "@/lib/labels";
import { type Plane, planesOf } from "./plane";
import { PlaneList } from "./plane-list";

afterEach(cleanup);

function cluster(
  namespace: string,
  name: string,
  {
    labels,
    spec = {},
    status,
  }: {
    labels?: Record<string, string>;
    spec?: Record<string, unknown>;
    status?: Record<string, unknown>;
  } = {},
) {
  return {
    apiVersion: "core.camunda.io/v1",
    kind: "CamundaManagementCluster",
    metadata: { namespace, name, labels },
    spec: { identityProvider: { keycloak: {} }, ...spec },
    status,
  } as never;
}

function claim(namespace: string, name: string, environments: string[]) {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaManagementClusterClaim",
    metadata: { namespace, name },
    spec: { environments, management: { identityProvider: { keycloak: {} } } },
  } as never;
}

const ready = (status: "True" | "False", reason: string) => ({
  conditions: [{ type: "Ready", status, reason }],
});

const keycloakPlane = planesOf({
  clusters: [
    cluster("platform", "plane-a", {
      labels: { [CLAIM_LABEL]: "plane-a" },
      spec: {
        identityProvider: { keycloak: {} },
        console: {},
        webModeler: {},
      },
      status: {
        ...ready("False", "UnsupportedVersion"),
        clusters: [
          { namespace: "team-a", name: "orders", attached: true },
          {
            namespace: "team-b",
            name: "billing",
            attached: false,
            reason: "ClaimedElsewhere",
            message: "plane-b holds it",
          },
        ],
        optimize: [
          { namespace: "team-a", name: "opt", externalUrl: "https://opt" },
        ],
        managementAuthConfig: "plane-a",
      },
    }),
  ],
  claims: [claim("platform", "plane-a", ["production", "staging"])],
  namespaces: [],
});

function threePlanes(): Plane[] {
  return planesOf({
    clusters: [
      cluster("platform", "keycloak", {
        labels: { [CLAIM_LABEL]: "keycloak" },
        status: {
          ...ready("True", "Healthy"),
          clusters: [{ namespace: "a", name: "x", attached: true }],
        },
      }),
      cluster("platform", "oidc", {
        labels: { [CLAIM_LABEL]: "oidc" },
        spec: { identityProvider: { oidc: {} } },
        status: {
          ...ready("True", "Healthy"),
          clusters: [
            { namespace: "a", name: "x", attached: true },
            { namespace: "a", name: "y", attached: true },
            { namespace: "a", name: "z", attached: true },
          ],
        },
      }),
      cluster("platform", "broken", {
        labels: { [CLAIM_LABEL]: "broken" },
        status: ready("False", "UnsupportedVersion"),
      }),
    ],
    claims: [
      claim("platform", "keycloak", ["production"]),
      claim("platform", "oidc", ["production"]),
      claim("platform", "broken", ["staging"]),
    ],
    namespaces: [],
  });
}

const table = () => screen.getByRole("region", { name: "Management planes" });
const rowNames = () =>
  within(table())
    .getAllByRole("row")
    .slice(1)
    .map((row) => within(row).getAllByRole("cell")[0].textContent);

describe("PlaneList", () => {
  it("shows every column of a plane", () => {
    render(<PlaneList planes={keycloakPlane} />);
    expect(
      within(table())
        .getAllByRole("columnheader")
        .map((h) => h.textContent),
    ).toEqual([
      "Management plane",
      "Identity",
      "Components",
      "Serves",
      "Clusters",
      "Optimize",
      "Contract",
      "Status",
      "Warnings",
    ]);
    const cells = within(screen.getByRole("row", { name: /plane-a/ }))
      .getAllByRole("cell")
      .map((c) => c.textContent);
    expect(cells.slice(0, 8)).toEqual([
      "plane-aplatform",
      "Keycloak",
      "Identity, Console, Web Modeler",
      "production, staging",
      "1 attached · 1 not attached",
      "1",
      "plane-a",
      "Not ready · UnsupportedVersion",
    ]);
  });

  it("reads 0 (OIDC) for the Optimize count in OIDC mode", () => {
    render(<PlaneList planes={threePlanes()} />);
    const cells = within(screen.getByRole("row", { name: /oidc/ }))
      .getAllByRole("cell")
      .map((c) => c.textContent);
    expect(cells[5]).toBe("0 (OIDC)");
  });

  it("shows the reason of a Ready plane, such as Suspended", () => {
    const suspended = planesOf({
      clusters: [
        cluster("platform", "stopped", {
          labels: { [CLAIM_LABEL]: "stopped" },
          status: ready("True", "Suspended"),
        }),
      ],
      claims: [claim("platform", "stopped", ["production"])],
      namespaces: [],
    });
    render(<PlaneList planes={suspended} />);
    const cells = within(screen.getByRole("row", { name: /stopped/ }))
      .getAllByRole("cell")
      .map((c) => c.textContent);
    expect(cells[7]).toBe("Ready · Suspended");
  });

  it("shows a not-attached warning chip with each cluster on hover", async () => {
    render(<PlaneList planes={keycloakPlane} />);
    await userEvent.hover(
      screen.getByRole("button", { name: "1 not attached" }),
    );
    const note = screen.getByRole("note");
    expect(note.textContent).toContain("team-b/billing");
    expect(note.textContent).toContain("ClaimedElsewhere: plane-b holds it");
  });

  it("finds a plane by search", async () => {
    render(<PlaneList planes={threePlanes()} />);
    await userEvent.type(screen.getByRole("searchbox"), "stag");
    expect(rowNames()).toEqual(["brokenplatform"]);
  });

  it("filters by identity mode and by status", async () => {
    render(<PlaneList planes={threePlanes()} />);
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Identity" }),
      "OIDC",
    );
    expect(rowNames()).toEqual(["oidcplatform"]);
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Identity" }),
      "",
    );
    await userEvent.selectOptions(
      screen.getByRole("combobox", { name: "Status" }),
      "Not ready",
    );
    expect(rowNames()).toEqual(["brokenplatform"]);
  });

  it("sorts by status and by clusters", async () => {
    render(<PlaneList planes={threePlanes()} />);
    const header = (name: string) =>
      within(table()).getByRole("button", { name });
    await userEvent.click(header("Clusters"));
    await userEvent.click(header("Clusters"));
    expect(rowNames()[0]).toBe("oidcplatform");
    await userEvent.click(header("Status"));
    expect(rowNames()[0]).toBe("brokenplatform");
  });

  it("pages after 25 planes", async () => {
    const planes = planesOf({
      clusters: Array.from({ length: 26 }, (_, i) =>
        cluster("platform", `plane-${String(i + 1).padStart(2, "0")}`),
      ),
      claims: [],
      namespaces: [],
    });
    render(<PlaneList planes={planes} />);
    expect(screen.getByText("Page 1 of 2")).toBeTruthy();
    expect(rowNames()).toHaveLength(25);
    await userEvent.click(screen.getByRole("button", { name: "Next page" }));
    expect(rowNames()).toEqual([
      "plane-26platformNot managed by the fleet manager",
    ]);
  });

  it("shows a plane without the claim label as read only with its raw selector", () => {
    const planes = planesOf({
      clusters: [
        cluster("platform", "legacy", {
          spec: { namespaceSelector: { matchLabels: { team: "payments" } } },
        }),
      ],
      claims: [],
      namespaces: [],
    });
    render(<PlaneList planes={planes} />);
    const row = screen.getByRole("row", { name: /legacy/ });
    expect(row.textContent).toContain("Not managed by the fleet manager");
    expect(row.textContent).toContain("team=payments");
    expect(within(row).queryAllByRole("button")).toEqual([]);
    expect(
      within(row)
        .getAllByRole("link")
        .map((l) => l.getAttribute("href")),
    ).toEqual(["/fleet/planes/platform/legacy"]);
  });

  it("links New management plane to the create flow", () => {
    render(<PlaneList planes={[]} />);
    expect(
      screen
        .getByRole("link", { name: "New management plane" })
        .getAttribute("href"),
    ).toBe("/fleet/planes/new");
  });

  it("says when there is no plane yet", () => {
    render(<PlaneList planes={[]} />);
    expect(screen.getByText("No management plane yet.")).toBeTruthy();
  });

  it("shows each read error with its kind above the table", () => {
    render(
      <PlaneList
        planes={keycloakPlane}
        errors={[
          { kind: "Namespace", message: "namespaces: forbidden" },
          { kind: "CamundaManagementClusterClaim", message: "timed out" },
        ]}
      />,
    );
    expect(screen.getAllByRole("alert").map((a) => a.textContent)).toEqual([
      "Could not read Namespace: namespaces: forbidden",
      "Could not read CamundaManagementClusterClaim: timed out",
    ]);
    expect(screen.getByRole("row", { name: /plane-a/ })).toBeTruthy();
  });
});
