// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import type { PresetImpact } from "./impact";
import { ImpactPreview } from "./impact-preview";

afterEach(cleanup);

const impact: PresetImpact = {
  rows: [
    {
      kind: "CamundaCluster",
      namespace: "team-billing",
      name: "billing",
      mark: "Pods roll",
      connectorsRestart: false,
    },
    {
      kind: "CamundaCluster",
      namespace: "team-orders",
      name: "orders",
      mark: "Unchanged",
      connectorsRestart: false,
    },
  ],
  notes: ["The brokers restart one at a time. The other processes restart."],
  unread: [],
};

describe("ImpactPreview", () => {
  it("shows each resource with its mark", () => {
    render(<ImpactPreview impact={impact} />);
    const rows = within(
      screen.getByRole("list", { name: "Inheriting resources" }),
    ).getAllByRole("listitem");
    expect(rows.map((r) => r.textContent)).toEqual([
      "team-billing/billingPods roll",
      "team-orders/ordersUnchanged",
    ]);
  });

  it("shows the notes after the rows", () => {
    render(<ImpactPreview impact={impact} />);
    const resources = screen.getByRole("list", { name: "Inheriting resources" });
    const notes = screen.getByRole("list", { name: "Notes" });
    expect(notes.textContent).toBe(impact.notes[0]);
    expect(
      resources.compareDocumentPosition(notes) &
        Node.DOCUMENT_POSITION_FOLLOWING,
    ).toBeTruthy();
  });

  it("says that no resource inherits the preset", () => {
    render(<ImpactPreview impact={{ rows: [], notes: [], unread: [] }} />);
    expect(screen.getByText("No resource inherits this preset.")).toBeTruthy();
  });

  it("names the lists that cannot be read and shows no resource as unaffected", () => {
    render(
      <ImpactPreview
        impact={{ rows: [], notes: [], unread: ["CamundaCluster", "CamundaRelease"] }}
      />,
    );
    expect(screen.getByRole("alert").textContent).toContain(
      "The impact is not known while the fleet manager cannot read CamundaCluster, CamundaRelease.",
    );
    expect(screen.queryByText("No resource inherits this preset.")).toBeNull();
    expect(screen.queryByText("Unchanged")).toBeNull();
  });
});
