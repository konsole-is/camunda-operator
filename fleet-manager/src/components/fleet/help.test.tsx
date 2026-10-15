// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it } from "vitest";
import { fieldDocFor } from "@/lib/fielddocs";
import { ConditionsTable } from "./conditions-table";
import { FieldHelp } from "./field-help";

afterEach(cleanup);

const partitions = fieldDocFor(
  "core.camunda.io",
  "CamundaCluster",
  "spec.zeebe.partitions",
);

describe("FieldHelp", () => {
  it("shows the description and the API reference link for a known path on hover", async () => {
    render(
      <FieldHelp label="Partitions" doc={partitions} operatorVersion="0.2.3" />,
    );
    expect(screen.queryByRole("note")).toBeNull();
    await userEvent.hover(
      screen.getByRole("button", { name: "About Partitions" }),
    );
    const note = screen.getByRole("note");
    expect(note.textContent).toContain(
      "Partitions is the number of partitions.",
    );
    expect(
      within(note)
        .getByRole("link", { name: "API reference" })
        .getAttribute("href"),
    ).toBe(
      "https://konsole-is.github.io/camunda-operator/0.2/crds/api-reference/#zeebespec",
    );
  });

  it("opens on keyboard focus and closes on Escape", async () => {
    render(
      <FieldHelp label="Partitions" doc={partitions} operatorVersion="0.2.3" />,
    );
    await userEvent.tab();
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "About Partitions" }),
    );
    expect(screen.getByRole("note")).toBeTruthy();
    await userEvent.tab();
    expect(document.activeElement).toBe(
      screen.getByRole("link", { name: "API reference" }),
    );
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("note")).toBeNull();
    expect(document.activeElement).toBe(
      screen.getByRole("button", { name: "About Partitions" }),
    );
  });

  it("stays open when the pointer leaves while the focus is inside", async () => {
    render(
      <FieldHelp label="Partitions" doc={partitions} operatorVersion="0.2.3" />,
    );
    const button = screen.getByRole("button", { name: "About Partitions" });
    await userEvent.hover(button);
    await userEvent.tab();
    await userEvent.tab();
    expect(document.activeElement).toBe(
      screen.getByRole("link", { name: "API reference" }),
    );
    await userEvent.unhover(button);
    expect(screen.getByRole("note")).toBeTruthy();
  });

  it("renders nothing for a path without a description", () => {
    const { container } = render(
      <FieldHelp
        label="Constructor"
        doc={fieldDocFor("core.camunda.io", "CamundaCluster", "constructor")}
        operatorVersion="0.2.3"
      />,
    );
    expect(container.innerHTML).toBe("");
  });
});

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
