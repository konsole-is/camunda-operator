// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { ConditionRow } from "@/components/fleet/conditions-table";
import { OverviewConditions } from "./overview-conditions";

afterEach(cleanup);

const now = "2026-10-18T12:00:00Z";
const row = (
  type: string,
  status: ConditionRow["status"],
  reason: string,
): ConditionRow => ({ type, status, reason, lastTransitionTime: now });

// headers lists the row headers and the row group headers of the section
// in their order.
const headers = () =>
  within(screen.getByRole("region", { name: "Conditions" }))
    .getAllByRole("rowheader")
    .map((h) => h.textContent);

describe("OverviewConditions", () => {
  it("lists the resource conditions, then Optimize, then the claim group", () => {
    render(
      <OverviewConditions
        cluster={[
          row("Ready", "True", "Healthy"),
          row("AdminSecretReady", "False", "MissingSecret"),
          row("MirroredSecretsReady", "True", "Healthy"),
          row("ZeebeReady", "False", "Degraded"),
        ]}
        optimize={[
          row("Ready", "True", "Healthy"),
          row("MirroredSecretsReady", "True", "Healthy"),
        ]}
        claim={[
          row("Ready", "True", "Healthy"),
          row("RequestsReady", "True", "Healthy"),
          row("ClusterReady", "True", "Healthy"),
          row("AttachmentsReady", "True", "Healthy"),
        ]}
        now={now}
      />,
    );
    expect(headers()).toEqual([
      "AdminSecretReady",
      "MirroredSecretsReady",
      "Optimize",
      "MirroredSecretsReady",
      "Claim",
      "RequestsReady",
      "ClusterReady",
      "AttachmentsReady",
    ]);
  });

  it("a cluster without claim and Optimize has no groups", () => {
    render(
      <OverviewConditions
        cluster={[row("AdminSecretReady", "True", "Healthy")]}
        now={now}
      />,
    );
    expect(headers()).toEqual(["AdminSecretReady"]);
  });

  it("leaves out a group without rows", () => {
    render(
      <OverviewConditions
        cluster={[row("AdminSecretReady", "True", "Healthy")]}
        optimize={[row("Ready", "True", "Healthy")]}
        claim={[]}
        now={now}
      />,
    );
    expect(headers()).toEqual(["AdminSecretReady"]);
  });

  it("shows a claim condition type that the app does not know in the Claim group", () => {
    render(
      <OverviewConditions
        cluster={[]}
        claim={[
          row("Ready", "True", "Healthy"),
          row("FutureReady", "False", "Blocked"),
        ]}
        now={now}
      />,
    );
    expect(headers()).toEqual(["Claim", "FutureReady"]);
  });
});
