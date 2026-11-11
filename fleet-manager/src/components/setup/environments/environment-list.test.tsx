// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { EnvironmentList, EnvironmentListLive } from "./environment-list";
import { environmentFixtures } from "./fixtures";
import { environmentRows, unassignedNamespaces } from "./rows";

const failing = vi.hoisted(() => ({ kind: "" }));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
    ...(kind === failing.kind && { error: `${kind} is forbidden` }),
  }),
  useWatchesFresh: (lists: unknown[]) => lists.map(() => true),
}));

afterEach(() => {
  cleanup();
  failing.kind = "";
});

describe("EnvironmentList", () => {
  it("shows each environment with its color, namespace count, cluster count, plane count, and status", () => {
    render(<EnvironmentListLive initial={environmentFixtures()} />);
    const row = screen.getByRole("row", { name: /Production/ });
    const cells = within(row)
      .getAllByRole("cell")
      .map((c) => c.textContent);
    expect(cells).toEqual(["Productionproduction", "2", "2", "0", "Ready"]);
    expect(row.querySelector('[data-color="#C62828"]')).not.toBeNull();
  });

  it("counts the labelled namespaces of an environment without a status", () => {
    render(<EnvironmentListLive initial={environmentFixtures()} />);
    const cells = within(screen.getByRole("row", { name: /Staging/ }))
      .getAllByRole("cell")
      .map((c) => c.textContent);
    expect(cells.slice(1)).toEqual(["1", "0", "0", "No status"]);
  });

  it("links each environment to its editor", () => {
    render(<EnvironmentListLive initial={environmentFixtures()} />);
    expect(
      screen.getByRole("link", { name: /Platform/ }).getAttribute("href"),
    ).toBe("/setup/environments/platform");
    expect(
      screen
        .getByRole("link", { name: "New environment" })
        .getAttribute("href"),
    ).toBe("/setup/environments/new");
  });

  it("shows the banner with the namespaces without an environment", () => {
    render(<EnvironmentListLive initial={environmentFixtures()} />);
    expect(screen.getByRole("note").textContent).toContain(
      "These namespaces hold clusters and have no environment: legacy.",
    );
  });

  it("shows no banner while no environment exists", () => {
    render(
      <EnvironmentListLive
        initial={{ ...environmentFixtures(), environments: [] }}
      />,
    );
    expect(screen.queryByRole("note")).toBeNull();
    expect(
      screen.getByText(
        "No environment yet. Without one, every namespace takes the policy of the registrations.",
      ),
    ).toBeTruthy();
  });

  it("shows the error of a failed read above the table", () => {
    failing.kind = "Environment";
    render(<EnvironmentListLive initial={environmentFixtures()} />);
    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read Environment: Environment is forbidden",
    );
    expect(screen.getByRole("region", { name: "Environments" })).toBeTruthy();
  });

  it("shows a color that is not a hex color as no swatch", () => {
    const s = environmentFixtures();
    s.environments[0].spec.color = "red";
    render(
      <EnvironmentList
        rows={environmentRows(s)}
        unassigned={unassignedNamespaces(s)}
        errors={[]}
      />,
    );
    expect(document.querySelector('[data-color="red"]')).toBeNull();
  });
});
