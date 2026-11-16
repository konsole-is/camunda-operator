// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import { PlaneConditions } from "./plane-conditions";

afterEach(cleanup);

const now = "2026-11-15T12:00:00Z";
const row = (
  type: string,
  status: ConditionRow["status"] = "True",
  reason = "Healthy",
): ConditionRow => ({ type, status, reason, lastTransitionTime: now });

const planeRows = [
  row("Ready"),
  row("SecretsReady", "True", "Disabled"),
  row("MirroredSecretsReady"),
  row("ServiceAccountsReady"),
  row("ManagementAuthReady"),
  row("OptimizeCallbacksReady", "True", "Disabled"),
];
const claimRows = [
  row("Ready"),
  row("RequestsReady"),
  row("ManagementClusterReady"),
];

const table = () =>
  screen.getByRole("region", { name: "Conditions outside the components" });

describe("PlaneConditions", () => {
  it("shows the resource conditions of the plane and the claim group, and no component condition", () => {
    render(
      <PlaneConditions
        plane={[...planeRows, row("IdentityReady", "False", "Down")]}
        claim={claimRows}
        now={now}
      />,
    );
    for (const type of [
      "SecretsReady",
      "MirroredSecretsReady",
      "ServiceAccountsReady",
      "ManagementAuthReady",
      "OptimizeCallbacksReady",
      "RequestsReady",
      "ManagementClusterReady",
    ]) {
      expect(within(table()).getByText(type)).toBeTruthy();
    }
    expect(within(table()).queryByText("IdentityReady")).toBeNull();
    expect(within(table()).queryByText("Ready", { exact: true })).toBeNull();
    expect(within(table()).getByText("Claim", { exact: true })).toBeTruthy();
  });

  it("shows the reason Disabled of OptimizeCallbacksReady", () => {
    render(<PlaneConditions plane={planeRows} now={now} />);
    const cells = within(table())
      .getByText("OptimizeCallbacksReady")
      .closest("tr");
    expect(cells?.textContent).toContain("Disabled");
  });

  it("a claim-only plane shows the claim group alone", () => {
    render(<PlaneConditions claim={claimRows} now={now} />);
    const headers = within(table())
      .getAllByRole("rowheader")
      .map((h) => h.textContent);
    expect(headers).toEqual([
      "Claim",
      "RequestsReady",
      "ManagementClusterReady",
    ]);
  });
});
