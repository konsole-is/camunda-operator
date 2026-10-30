// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import { parse } from "yaml";
import type { Resource } from "@/lib/k8s/kinds";
import { CLAIM_LABEL } from "@/lib/labels";
import { mergeCluster } from "@/lib/merge";
import { claimView } from "./claim";
import { ClaimTab } from "./claim-tab";

afterEach(cleanup);

const now = "2026-10-30T12:00:00Z";

type Condition = NonNullable<
  NonNullable<Resource<"CamundaClusterClaim">["status"]>["conditions"]
>[number];

const condition = (
  type: string,
  status: Condition["status"],
  reason: string,
  message = "",
): Condition => ({ type, status, reason, message, lastTransitionTime: now });

const cluster = {
  kind: "CamundaCluster",
  metadata: {
    namespace: "team-a",
    name: "orders",
    labels: { [CLAIM_LABEL]: "orders" },
  },
  spec: { presetRef: "medium", releaseRef: "r-8.9" },
} as unknown as Resource<"CamundaCluster">;

function claimWith(
  conditions: Condition[],
  over: { deleting?: boolean } = {},
): Resource<"CamundaClusterClaim"> {
  return {
    apiVersion: "fleet.camunda.io/v1alpha1",
    kind: "CamundaClusterClaim",
    metadata: {
      namespace: "team-a",
      name: "orders",
      managedFields: [{ manager: "fleet-manager", operation: "Apply" }],
      ...(over.deleting && { deletionTimestamp: now }),
    },
    spec: {
      presetRef: "medium",
      plugins: [{ section: "access", registration: "gateway-access" }],
    },
    status: {
      environment: "prod",
      conditions,
      requests: [
        {
          name: "orders-access",
          section: "access",
          registration: "gateway-access",
          outputsReady: "False",
          ready: "False",
        },
      ],
      resolvedPolicy: {
        parameters: {
          "gateway-access": { hostname: { value: "orders.example.com" } },
        },
      },
    },
  } as Resource<"CamundaClusterClaim">;
}

const MISSING_INPUT = claimWith([
  condition(
    "Ready",
    "False",
    "MissingInput",
    "registration aws-kms needs the input keyRef, and no selected section gives it",
  ),
]);
const WAITING = claimWith([
  condition(
    "Ready",
    "False",
    "PrerequisiteNotMet",
    "waiting for the outputs of cloud-identity",
  ),
  condition(
    "RequestsReady",
    "False",
    "OperationPending",
    "waiting for the outputs of cloud-identity",
  ),
  condition(
    "ClusterReady",
    "False",
    "Creating",
    "CamundaCluster has not reported Ready yet",
  ),
]);
const DOWN = claimWith([
  condition(
    "Ready",
    "False",
    "Down",
    "ClusterReady: Down: 1 of 3 brokers are ready",
  ),
  condition("ClusterReady", "False", "Down", "Down: 1 of 3 brokers are ready"),
  condition("AttachmentsReady", "True", "Healthy"),
]);
const DELETING = claimWith(
  [condition("Ready", "False", "Deleting", "waiting for orders-access")],
  { deleting: true },
);
const FIXTURES = [MISSING_INPUT, WAITING, DOWN, DELETING];

// Every reason that a fixture of this file uses. Each one is set by the claim
// controller or by ocf for a claim condition: the precheck reasons and
// Deleting (api/fleet/v1alpha1/conditions.go), Healthy (api/v1), and the ocf
// reasons PrerequisiteNotMet, OperationPending, Blocked, Creating, Down.
const CLAIM_REASONS = new Set([
  "NoEnvironment",
  "UnknownEnvironment",
  "NotAllowed",
  "MissingInput",
  "InvalidReference",
  "SlotConflict",
  "WiringCycle",
  "Deleting",
  "Healthy",
  "PrerequisiteNotMet",
  "OperationPending",
  "Blocked",
  "Creating",
  "Down",
]);

function renderTab(claim: Resource<"CamundaClusterClaim">) {
  render(
    <ClaimTab
      view={claimView({
        claim,
        cluster,
        merged: mergeCluster(undefined, undefined, cluster.spec),
        environments: [
          {
            kind: "Environment",
            metadata: { name: "prod" },
            spec: { displayName: "Production" },
          } as unknown as Resource<"Environment">,
        ],
        sections: [
          {
            kind: "PluginSection",
            metadata: { name: "access" },
            spec: { label: "Access", appliesTo: ["CamundaClusterClaim"] },
          } as unknown as Resource<"PluginSection">,
        ],
        registrations: [
          {
            kind: "PluginRegistration",
            metadata: { name: "gateway-access" },
            spec: { label: "Gateway", deletionPolicies: ["Delete"] },
          } as unknown as Resource<"PluginRegistration">,
        ],
      })}
      stored={claim}
      now={now}
    />,
  );
}

const conditionRow = (type: string) =>
  within(screen.getByRole("region", { name: "Claim conditions" }))
    .getByRole("rowheader", { name: type })
    .closest("tr")!;

describe("ClaimTab", () => {
  it("uses only the reasons that the claim controller sets", () => {
    for (const c of FIXTURES.flatMap((f) => f.status?.conditions ?? [])) {
      expect(CLAIM_REASONS).toContain(c.reason);
    }
  });

  it("shows a MissingInput claim with its reason and its requests", () => {
    renderTab(MISSING_INPUT);
    const ready = conditionRow("Ready");
    expect(within(ready).getByText("False")).toBeTruthy();
    expect(within(ready).getByText("MissingInput")).toBeTruthy();
    expect(
      within(ready).getByText(
        "registration aws-kms needs the input keyRef, and no selected section gives it",
      ),
    ).toBeTruthy();
    const requests = screen.getByRole("table", { name: "Requests" });
    const row = within(requests).getByRole("row", { name: /^orders-access/ });
    expect(within(row).getByText("Access")).toBeTruthy();
    expect(within(row).getByText("Gateway")).toBeTruthy();
  });

  it("shows a claim that waits for outputs", () => {
    renderTab(WAITING);
    for (const [type, reason] of [
      ["Ready", "PrerequisiteNotMet"],
      ["RequestsReady", "OperationPending"],
    ]) {
      const row = conditionRow(type);
      expect(within(row).getByText(reason)).toBeTruthy();
      expect(
        within(row).getByText("waiting for the outputs of cloud-identity"),
      ).toBeTruthy();
    }
  });

  it("shows the ClusterReady message of the cluster as stored", () => {
    renderTab(WAITING);
    expect(
      within(conditionRow("ClusterReady")).getByText("Creating"),
    ).toBeTruthy();
    expect(
      within(conditionRow("ClusterReady")).getByText(
        "CamundaCluster has not reported Ready yet",
      ),
    ).toBeTruthy();
    cleanup();
    renderTab(DOWN);
    expect(within(conditionRow("ClusterReady")).getByText("Down")).toBeTruthy();
    expect(
      within(conditionRow("ClusterReady")).getByText(
        "Down: 1 of 3 brokers are ready",
      ),
    ).toBeTruthy();
  });

  it("shows a deleting claim", () => {
    renderTab(DELETING);
    expect(within(conditionRow("Ready")).getByText("Deleting")).toBeTruthy();
    expect(
      within(conditionRow("Ready")).getByText("waiting for orders-access"),
    ).toBeTruthy();
  });

  it("shows the YAML of the claim as stored", async () => {
    renderTab(MISSING_INPUT);
    const button = screen.getByRole("button", { name: "Show YAML" });
    expect(button.getAttribute("aria-expanded")).toBe("false");
    await userEvent.click(button);
    const yaml = screen.getByLabelText("Claim YAML").textContent ?? "";
    const metadata = { ...MISSING_INPUT.metadata };
    delete metadata.managedFields;
    expect(parse(yaml)).toEqual({ ...MISSING_INPUT, metadata });
    expect(yaml).not.toContain("managedFields");
    const hide = screen.getByRole("button", { name: "Hide YAML" });
    expect(hide.getAttribute("aria-expanded")).toBe("true");
    await userEvent.click(hide);
    expect(screen.queryByLabelText("Claim YAML")).toBeNull();
  });

  it("marks the basics as set at creation and each section as chosen at creation", () => {
    renderTab(MISSING_INPUT);
    const basics = screen.getByRole("region", { name: "Basics" });
    for (const value of ["orders", "team-a", "Production"]) {
      expect(within(basics).getByText(value)).toBeTruthy();
    }
    expect(within(basics).getAllByText("Set at creation")).toHaveLength(3);
    const access = screen.getByRole("region", { name: "Access" });
    expect(within(access).getByText("Gateway")).toBeTruthy();
    expect(within(access).getByText("Chosen at creation")).toBeTruthy();
  });

  it("marks a ref that the claim leaves to the environment", () => {
    renderTab(MISSING_INPUT);
    const refs = screen.getByRole("region", { name: "Release and preset" });
    expect(within(refs).getByText("r-8.9")).toBeTruthy();
    expect(within(refs).getByText("medium")).toBeTruthy();
    expect(
      within(refs).getAllByText("Default of the environment").length,
    ).toBeGreaterThan(0);
  });

  it("shows the state and lock of a parameter", () => {
    renderTab(MISSING_INPUT);
    const table = screen.getByRole("table", { name: "Access parameters" });
    const row = within(table).getByRole("row", { name: /^hostname/ });
    expect(within(row).getByText("orders.example.com")).toBeTruthy();
    expect(within(row).getAllByText("Set by your admin")).toHaveLength(2);
  });

  it("lets a keyboard user scroll a wide parameter table", () => {
    renderTab(MISSING_INPUT);
    expect(
      screen
        .getByRole("region", { name: "Access parameters table" })
        .getAttribute("tabindex"),
    ).toBe("0");
  });

  it("says when the claim has no attachments", () => {
    renderTab(MISSING_INPUT);
    const attachments = screen.getByRole("region", { name: "Attachments" });
    expect(within(attachments).getAllByText("Not attached")).toHaveLength(2);
  });
});
