// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { AttentionPanel } from "./attention-panel";

afterEach(cleanup);

describe("AttentionPanel", () => {
  it("links each entry to its page", () => {
    render(
      <AttentionPanel
        items={[
          {
            id: "cluster/team-a/orders",
            what: "Cluster",
            name: "team-a/orders",
            reason: "Down",
            message: "No broker is ready",
            href: "/fleet/clusters/team-a/orders",
          },
          {
            id: "plane/platform/plane-a",
            what: "Management plane",
            name: "platform/plane-a",
            reason: "Down",
          },
        ]}
      />,
    );
    const panel = screen.getByRole("region", { name: "Needs attention" });
    expect(
      within(panel)
        .getByRole("link", { name: "Cluster team-a/orders" })
        .getAttribute("href"),
    ).toBe("/fleet/clusters/team-a/orders");
    expect(within(panel).getByText("Down: No broker is ready")).toBeTruthy();
    expect(within(panel).getAllByRole("link")).toHaveLength(1);
    expect(
      within(panel).getByText("Management plane platform/plane-a"),
    ).toBeTruthy();
  });

  it("says that nothing needs attention", () => {
    render(<AttentionPanel items={[]} />);
    expect(
      within(screen.getByRole("region", { name: "Needs attention" })).getByText(
        "Nothing needs attention.",
      ),
    ).toBeTruthy();
  });
});
