// @vitest-environment jsdom
import { cleanup, render, screen, within } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { platformFixtures } from "./fixtures";
import { PlatformList, PlatformListLive } from "./platform-list";

const failing = vi.hoisted(() => ({ kind: "", unanswered: "" }));
vi.mock("@/lib/k8s/hooks", () => ({
  useResources: (kind: string, _opts: unknown, initial: unknown[]) => ({
    items: initial,
    ...(kind === failing.kind && { error: `${kind} is forbidden` }),
  }),
  useWatchesFresh: (lists: { kind: string }[]) =>
    lists.map(({ kind }) => kind !== failing.unanswered),
}));

afterEach(() => {
  cleanup();
  failing.kind = "";
  failing.unanswered = "";
});

const cellsOf = (name: RegExp) =>
  within(screen.getByRole("row", { name }))
    .getAllByRole("cell")
    .map((c) => c.textContent);

describe("PlatformList", () => {
  it("shows each config with its auth method, its user count, and its status", () => {
    render(<PlatformListLive initial={platformFixtures()} />);

    expect(cellsOf(/oidc-prod/)).toEqual(["oidc-prod", "OIDC", "5", "Ready"]);
    expect(cellsOf(/basic-lab/)).toEqual(["basic-lab", "Basic", "0", "Ready"]);
    expect(cellsOf(/broken/)[3]).toContain("MissingSecret");
  });

  it("links each config to its detail", () => {
    render(<PlatformListLive initial={platformFixtures()} />);

    expect(
      screen.getByRole("link", { name: /oidc-prod/ }).getAttribute("href"),
    ).toBe("/setup/platform/oidc-prod");
  });

  it("offers no create", () => {
    render(<PlatformListLive initial={platformFixtures()} />);

    expect(screen.queryByRole("link", { name: /New/ })).toBeNull();
    expect(screen.queryByRole("button", { name: /New/ })).toBeNull();
  });

  it("shows a read error above the rows", () => {
    failing.kind = "DatabaseServer";
    render(<PlatformListLive initial={platformFixtures()} />);

    expect(screen.getByRole("alert").textContent).toBe(
      "Could not read DatabaseServer: DatabaseServer is forbidden",
    );
    expect(screen.getByRole("row", { name: /oidc-prod/ })).toBeDefined();
  });

  it("shows no user count and names the kind while a kind that can use a config cannot be read", () => {
    failing.kind = "DatabaseServer";
    render(<PlatformListLive initial={platformFixtures()} />);

    expect(cellsOf(/basic-lab/)[2]).toBe("Not known");
    expect(cellsOf(/oidc-prod/)[2]).toBe("Not known");
    expect(
      screen.getByText(
        "The users are not known while the fleet manager cannot read DatabaseServer.",
      ),
    ).toBeDefined();
  });

  it("shows no user count while the watch of a kind that can use a config has not answered", () => {
    failing.unanswered = "Environment";
    render(<PlatformListLive initial={platformFixtures()} />);

    expect(cellsOf(/basic-lab/)[2]).toBe("Not known");
    expect(
      screen.getByText(
        "The users are not known while the fleet manager cannot read Environment.",
      ),
    ).toBeDefined();
  });

  it("says what to do when no config exists", () => {
    render(<PlatformList rows={[]} errors={[]} />);

    expect(
      screen.getByText(
        "No platform config yet. A cluster needs one. The guide shows how to apply one.",
      ),
    ).toBeDefined();
  });
});
