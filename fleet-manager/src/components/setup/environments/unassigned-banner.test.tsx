// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import { UnassignedBanner } from "./unassigned-banner";

afterEach(cleanup);

describe("UnassignedBanner", () => {
  it("names the namespaces without an environment and those whose environment does not exist", () => {
    render(
      <UnassignedBanner
        namespaces={[
          { name: "legacy", reason: "NoEnvironment" },
          { name: "old-ns", reason: "UnknownEnvironment" },
          { name: "team-x", reason: "NoEnvironment" },
        ]}
      />,
    );
    const text = screen.getByRole("note").textContent;
    expect(text).toContain(
      "These namespaces hold clusters and have no environment: legacy, team-x. New clusters in these namespaces are refused (NoEnvironment).",
    );
    expect(text).toContain(
      "These namespaces name an environment that does not exist: old-ns. New clusters in them are refused (UnknownEnvironment).",
    );
  });

  it("shows only the line of the reason that a namespace has", () => {
    render(
      <UnassignedBanner
        namespaces={[{ name: "old-ns", reason: "UnknownEnvironment" }]}
      />,
    );
    expect(screen.getByRole("note").textContent).not.toContain(
      "have no environment",
    );
  });

  it("renders nothing for no namespace", () => {
    const { container } = render(<UnassignedBanner namespaces={[]} />);
    expect(container.innerHTML).toBe("");
  });
});
