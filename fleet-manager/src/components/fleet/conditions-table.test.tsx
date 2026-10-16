// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { ConditionsTable } from "./conditions-table";

afterEach(cleanup);

describe("ConditionsTable", () => {
  const now = "2026-10-15T12:00:00Z";

  it("shows the reason and the message of a False condition", () => {
    render(
      <ConditionsTable
        now={now}
        conditions={[
          {
            type: "AdminSecretReady",
            status: "False",
            reason: "SecretMissing",
            message: "The Secret orders-admin does not exist.",
            lastTransitionTime: "2026-10-15T11:48:00Z",
          },
        ]}
      />,
    );
    const row = screen.getByRole("row", { name: /AdminSecretReady/ });
    expect(row.textContent).toContain("False");
    expect(row.textContent).toContain("SecretMissing");
    expect(row.textContent).toContain(
      "The Secret orders-admin does not exist.",
    );
    expect(row.textContent).toContain("12m");
  });

  it("renders the ungrouped conditions first, then the groups in order", () => {
    render(
      <ConditionsTable
        now={now}
        conditions={[
          { type: "MirroredSecretsReady", status: "True", reason: "Healthy" },
        ]}
        groups={[
          {
            label: "Claim",
            conditions: [
              { type: "RequestsReady", status: "True", reason: "Healthy" },
              { type: "ClusterReady", status: "False", reason: "Converging" },
            ],
          },
          {
            label: "Requests",
            conditions: [
              { type: "OutputsReady", status: "Unknown", reason: "Waiting" },
            ],
          },
        ]}
      />,
    );
    const order = screen
      .getAllByRole("row")
      .slice(1)
      .map((row) => row.querySelector("th")?.textContent);
    expect(order).toEqual([
      "MirroredSecretsReady",
      "Claim",
      "RequestsReady",
      "ClusterReady",
      "Requests",
      "OutputsReady",
    ]);
  });

  it("says so when there is no condition", () => {
    render(<ConditionsTable now={now} conditions={[]} />);
    expect(screen.getByText("No conditions")).toBeTruthy();
  });
});
