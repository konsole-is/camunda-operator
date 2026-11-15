// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { ConditionList } from "./condition-list";

afterEach(cleanup);

const now = "2026-10-15T12:00:00Z";

describe("ConditionList", () => {
  it("shows the type, the state, the reason, the message, and the time since the last change", () => {
    render(
      <ConditionList
        label="Admin conditions"
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
    const item = within(
      screen.getByRole("list", { name: "Admin conditions" }),
    ).getByRole("listitem");
    expect(within(item).getByText("AdminSecretReady")).toBeTruthy();
    expect(
      within(item).getByRole("img", {
        name: "AdminSecretReady: Not ready (SecretMissing)",
      }),
    ).toBeTruthy();
    expect(within(item).getByText("SecretMissing")).toBeTruthy();
    expect(
      within(item).getByText("The Secret orders-admin does not exist."),
    ).toBeTruthy();
    expect(within(item).getByText("Changed 12m ago")).toBeTruthy();
  });

  it("leaves out a message and a time that the condition does not have", () => {
    render(
      <ConditionList
        label="Conditions"
        now={now}
        conditions={[{ type: "Ready", status: "True", reason: "Healthy" }]}
      />,
    );
    expect(screen.getByRole("listitem").textContent).toBe("ReadyHealthy");
  });

  it("keeps the order of the conditions", () => {
    render(
      <ConditionList
        label="Conditions"
        now={now}
        conditions={[
          { type: "WebappReady", status: "True", reason: "Healthy" },
          { type: "Ready", status: "False", reason: "Degraded" },
        ]}
      />,
    );
    expect(
      screen.getAllByRole("listitem").map((item) => item.textContent),
    ).toEqual(["WebappReadyHealthy", "ReadyDegraded"]);
  });

  it("says so when there is no condition", () => {
    render(<ConditionList label="Conditions" now={now} conditions={[]} />);
    expect(screen.getByText("No conditions")).toBeTruthy();
    expect(screen.queryByRole("list")).toBeNull();
  });
});
