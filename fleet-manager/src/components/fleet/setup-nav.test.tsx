// @vitest-environment jsdom
import { cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { SetupBreadcrumb } from "./setup-breadcrumb";
import { SetupNav } from "./setup-nav";

const pathname = vi.hoisted(() => ({ current: "/setup/presets" }));
vi.mock("next/navigation", () => ({ usePathname: () => pathname.current }));

afterEach(cleanup);

describe("SetupNav", () => {
  it("lists the Setup pages in order", () => {
    render(<SetupNav />);
    const nav = screen.getByRole("navigation", { name: "Setup" });
    expect(
      [...nav.querySelectorAll("a")].map((a) => a.textContent?.trim()),
    ).toEqual([
      "Environments",
      "Presets",
      "Releases",
      "Platform",
      "Connections",
      "Plugins",
      "Settings",
    ]);
  });

  it.each([
    ["/setup/presets", "Presets"],
    ["/setup/presets/gold", "Presets"],
    ["/setup/settings", "Settings"],
  ])("marks the current Setup page on %s", (path, label) => {
    pathname.current = path;
    render(<SetupNav />);
    const marked = screen
      .getByRole("navigation", { name: "Setup" })
      .querySelectorAll('[aria-current="page"]');
    expect([...marked].map((a) => a.textContent?.trim())).toEqual([label]);
  });
});

describe("SetupBreadcrumb", () => {
  it("shows Setup / <list> / <name> on a detail page", () => {
    pathname.current = "/setup/presets/gold";
    render(<SetupBreadcrumb />);
    const crumbs = screen.getByRole("navigation", { name: "Breadcrumb" });
    expect(crumbs.textContent?.replace(/\s+/g, " ").trim()).toBe(
      "Setup / Presets / gold",
    );
    expect(
      screen.getByRole("link", { name: "Presets" }).getAttribute("href"),
    ).toBe("/setup/presets");
    expect(crumbs.querySelector('[aria-current="page"]')?.textContent).toBe(
      "gold",
    );
  });

  it("shows nothing on a list page", () => {
    pathname.current = "/setup/presets";
    render(<SetupBreadcrumb />);
    expect(screen.queryByRole("navigation", { name: "Breadcrumb" })).toBeNull();
  });
});
