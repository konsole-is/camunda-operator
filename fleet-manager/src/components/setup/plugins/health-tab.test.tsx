// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { condition, pluginRegistration, registrationEvent } from "./fixtures";
import { HealthTab } from "./health-tab";

afterEach(cleanup);

const NOW = "2026-11-13T09:00:00Z";
const notReady = pluginRegistration("aws-kms", {}, [
  condition("Accepted", "True", "Healthy"),
  condition(
    "Ready",
    "False",
    "CredentialsInvalid",
    "The role cannot be assumed",
  ),
  condition("Progressing", "True", "Working"),
]);
const warning = registrationEvent(
  "aws-kms",
  "Warning",
  "CredentialsInvalid",
  "The role cannot be assumed by the controller",
);

describe("HealthTab", () => {
  it("shows Accepted and Ready and the events of the registration", () => {
    render(
      <HealthTab
        registration={notReady}
        events={[warning]}
        unread={[]}
        now={NOW}
      />,
    );
    const table = screen.getByRole("region", { name: "Conditions" });
    expect(within(table).getByText("Accepted")).toBeTruthy();
    expect(within(table).getByText("Ready")).toBeTruthy();
    expect(within(table).queryByText("Progressing")).toBeNull();
    expect(within(table).getAllByRole("row")).toHaveLength(3);
    const events = screen.getByRole("region", { name: "Events" });
    expect(
      within(events).getByText("The role cannot be assumed by the controller"),
    ).toBeTruthy();
  });

  it("shows the message of each condition as text", () => {
    render(
      <HealthTab registration={notReady} events={[]} unread={[]} now={NOW} />,
    );
    const table = screen.getByRole("region", { name: "Conditions" });
    expect(within(table).getByText("The role cannot be assumed")).toBeTruthy();
  });

  it("says No events for a registration without events", () => {
    render(
      <HealthTab registration={notReady} events={[]} unread={[]} now={NOW} />,
    );
    expect(
      within(screen.getByRole("region", { name: "Events" })).getByText(
        "No events",
      ),
    ).toBeTruthy();
  });

  it("shows no condition while PluginRegistration is not read", () => {
    render(
      <HealthTab
        registration={notReady}
        events={[]}
        unread={["PluginRegistration"]}
        now={NOW}
      />,
    );
    const conditions = screen.getByRole("region", { name: "Health" });
    expect(within(conditions).queryByRole("table")).toBeNull();
    expect(
      within(conditions).getByText(
        "Not known: the fleet manager has no current list of PluginRegistration.",
      ),
    ).toBeTruthy();
  });

  it("does not say No events while Event is not read", () => {
    render(
      <HealthTab
        registration={notReady}
        events={[]}
        unread={["Event"]}
        now={NOW}
      />,
    );
    const events = screen.getByRole("region", { name: "Events" });
    expect(within(events).queryByText("No events")).toBeNull();
    expect(
      within(events).getByText(
        "Not known: the fleet manager has no current list of Event.",
      ),
    ).toBeTruthy();
  });

  it("keeps the events it has and says that the list can miss events while Event is not read", () => {
    render(
      <HealthTab
        registration={notReady}
        events={[warning]}
        unread={["Event"]}
        now={NOW}
      />,
    );
    const events = screen.getByRole("region", { name: "Events" });
    expect(
      within(events).getByText("The list can miss events: Event is not read."),
    ).toBeTruthy();
    expect(
      within(events).getByText("The role cannot be assumed by the controller"),
    ).toBeTruthy();
  });

  it("shows an error for conditions that are not valid", () => {
    render(
      <HealthTab
        registration={pluginRegistration("bad", {}, [
          {
            type: "Ready",
            status: "True",
            reason: "Healthy",
            message: { a: 1 },
          } as never,
        ])}
        events={[]}
        unread={[]}
        now={NOW}
      />,
    );
    const conditions = screen.getByRole("region", { name: "Health" });
    expect(within(conditions).getByRole("alert").textContent).toBe(
      "Condition 1 is not a valid condition.",
    );
  });
});
